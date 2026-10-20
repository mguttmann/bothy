// SPDX-License-Identifier: Apache-2.0

package session

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"slices"
	"sync"
	"time"

	"github.com/OWNER/bothy/services/host/internal/clock"
	"github.com/OWNER/bothy/services/host/internal/config"
	"github.com/OWNER/bothy/services/host/internal/control"
	"github.com/OWNER/bothy/services/host/internal/doctor"
	"github.com/OWNER/bothy/services/host/internal/localchan"
	"github.com/OWNER/bothy/services/host/internal/noise"
	"github.com/OWNER/bothy/services/host/internal/state"
)

// Close reasons the host service sends (M2-43 accepts exactly these, 24 10.3).
const (
	ReasonClientClosed        = "client_closed"
	ReasonProcessExited       = "process_exited"
	ReasonIdleTimeout         = "idle_timeout"
	ReasonDetachExpired       = "detach_expired"
	ReasonOpenFailed          = "open_failed"
	ReasonHostPolicyViolation = "host_policy_violation"
)

// Confirmer asks the person at the machine (internal/confirm.Server.Ask) whether a PTY may start
// (kind open) or an attach may replay (kind attach); nil when host_service.require_local_confirm is
// false.
type Confirmer func(ctx context.Context, kind, profile, sessionUser string, timeout time.Duration) bool

// Deps are the parts of the manager; cmd/bothy-host wires them in run.
type Deps struct {
	// HostID is the own host ID; NoiseKey the static Noise key pair; AuthKey the login key.
	HostID   string
	NoiseKey noise.KeyPair
	AuthKey  ed25519.PrivateKey
	// HostVersion goes into message 2 and identify.
	HostVersion string
	// HTTP and DataURL come from the control client (control.Client.HTTPClient, DataURL).
	HTTP    func() *http.Client
	DataURL func(path string) (string, error)
	Config  *config.Config
	// Bindings store bindings.json; APIKey reads claude_api_key of the state directory for one
	// start.
	Bindings BindingStore
	APIKey   func() ([]byte, error)
	// Gate runs HD6, HD10 and HD11; Result returns the last result (lists after HD7, claude_bin);
	// Offered builds the offered profiles from it (profiles.Offered).
	Gate    *Gate
	Result  func() *doctor.Result
	Offered func(*doctor.Result) []doctor.Profile
	// ServerHost reports HD5 or-ed with control.Client.PeerLocal.
	ServerHost func() bool
	// Dial opens the session channel of a helper.
	Dial Dialer
	// Confirm is the local confirmation; nil disables it.
	Confirm Confirmer
	Clock   clock.Clock
	Log     *slog.Logger
	// Random is the source of the ephemeral Noise keys; nil means crypto/rand.
	Random io.Reader
}

// Manager implements control.Sessions and control.Resender. It is safe for concurrent use.
type Manager struct {
	d        Deps
	verifier *Verifier

	mu       sync.Mutex
	sessions map[string]*session // sessions whose PTY runs
	conns    map[string]*dataConn
	links    map[string]*helperLink
	pending  []control.SessionEnded // ends not reported yet
	// reported holds the sessions whose end this process reported or the server closed; an
	// attach for one of them closes with 1000 and reports nothing new.
	reported map[string]struct{}
	notifier control.Notifier
	closed   bool
	ctx      context.Context
	cancel   context.CancelFunc
}

// Compile-time checks: the ports of M2-43.
var (
	_ control.Sessions = (*Manager)(nil)
	_ control.Resender = (*Manager)(nil)
)

// New returns the manager and binds it to the gate.
func New(d Deps) *Manager {
	if d.Clock == nil {
		d.Clock = clock.Real()
	}
	if d.Log == nil {
		d.Log = slog.Default()
	}
	ctx, cancel := context.WithCancel(context.Background())
	m := &Manager{d: d, verifier: &Verifier{JTI: NewJTIMemory(), Bindings: NewBindings(d.Bindings)}, sessions: map[string]*session{},
		conns: map[string]*dataConn{}, links: map[string]*helperLink{}, reported: map[string]struct{}{}, ctx: ctx, cancel: cancel}
	if d.Gate != nil {
		d.Gate.SetEnforce(m.Enforce)
	}
	return m
}

// Log messages and attributes: codes, IDs and names only (24 10.1).
const (
	msgRefused  = "terminal handshake refused"
	msgEnded    = "terminal session ended"
	msgStarted  = "terminal session started"
	msgDetached = "terminal session detached"
	attrSession = "session_id"
	attrDetail  = "close_detail"
	attrReason  = "close_reason"
	attrProfile = "profile"
	attrUser    = "session_user"
	attrExit    = "exit_code"
	attrCode    = "code"
)

// Live implements control.Sessions: the sessions whose PTY runs and those whose end was not
// reported yet.
func (m *Manager) Live() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]string, 0, len(m.sessions)+len(m.pending))
	for id := range m.sessions {
		out = append(out, id)
	}
	for _, e := range m.pending {
		if !slices.Contains(out, e.SessionID) {
			out = append(out, e.SessionID)
		}
	}
	slices.Sort(out)
	return out
}

// Active implements control.Sessions: the running PTYs.
func (m *Manager) Active() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.sessions)
}

// Open implements control.Sessions: one data connection per session.open, served in its own
// goroutine; a newer session.open replaces the older data connection of the same session.
func (m *Manager) Open(_ context.Context, o control.SessionOpen, n control.Notifier) {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return
	}
	m.notifier = n
	old := m.conns[o.SessionID]
	delete(m.conns, o.SessionID)
	m.mu.Unlock()
	if old != nil {
		old.close(CodeNormal, "")
	}
	arrival := m.d.Clock.Now()
	go m.serve(m.ctx, o, n, arrival)
}

// Close implements control.Sessions: session.close ends the PTY with its tree at once, or abandons
// an opening; no session.ended, the server closed the session already.
func (m *Manager) Close(_ context.Context, c control.SessionClose) {
	m.mu.Lock()
	s := m.sessions[c.SessionID]
	dc := m.conns[c.SessionID]
	delete(m.conns, c.SessionID)
	m.mu.Unlock()
	if s != nil {
		s.end(endSpec{reason: c.CloseReason, silent: true})
	}
	if dc != nil {
		dc.close(CodeNormal, "")
	}
}

// EndAll implements control.Sessions: every PTY ends (4110, 4111, 4112, 4114); the server ends the
// sessions itself, so nothing is reported.
func (m *Manager) EndAll(context.Context) {
	for _, s := range m.snapshot() {
		s.end(endSpec{reason: ReasonHostPolicyViolation, silent: true})
	}
}

// Resend implements control.Resender: the ends that could not be reported go out on n.
func (m *Manager) Resend(ctx context.Context, n control.Notifier) {
	m.mu.Lock()
	m.notifier = n
	pend := m.pending
	m.pending = nil
	m.mu.Unlock()
	for _, e := range pend {
		m.report(ctx, e)
	}
}

// Shutdown ends every session and stops the manager (the host service stops).
func (m *Manager) Shutdown() {
	m.mu.Lock()
	m.closed = true
	links := make([]*helperLink, 0, len(m.links))
	for _, l := range m.links {
		links = append(links, l)
	}
	m.mu.Unlock()
	m.EndAll(context.Background())
	m.cancel()
	for _, l := range links {
		_ = l.c.Close()
	}
}

// Enforce ends the running sessions that p ends (the gate calls it after every run).
func (m *Manager) Enforce(_ context.Context, p Policy) {
	for _, s := range m.snapshot() {
		detail := ""
		switch {
		case p.Host != "" && p.Ends:
			detail = p.Host
		case p.Users[s.osu] != "":
			detail = p.Users[s.osu]
		}
		if detail != "" {
			s.end(endSpec{reason: ReasonHostPolicyViolation, detail: detail})
		}
	}
}

func (m *Manager) snapshot() []*session {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]*session, 0, len(m.sessions))
	for _, s := range m.sessions {
		out = append(out, s)
	}
	return out
}

// report sends session.ended, or keeps it for the next ready when the control connection is down.
func (m *Manager) report(ctx context.Context, e control.SessionEnded) {
	m.mu.Lock()
	n := m.notifier
	m.mu.Unlock()
	if n == nil || n.Ended(ctx, e) != nil {
		m.mu.Lock()
		m.pending = append(m.pending, e)
		m.mu.Unlock()
	}
}

// started sends session.started.
func (m *Manager) started(ctx context.Context, id string) {
	m.mu.Lock()
	n := m.notifier
	m.mu.Unlock()
	if n != nil {
		_ = n.Started(ctx, id)
	}
}

// link returns the session channel of the helper of osu, dialling it when needed. A failed peer
// check is the HD10 event of the gate.
func (m *Manager) link(ctx context.Context, osu string) (*helperLink, string) {
	m.mu.Lock()
	l := m.links[osu]
	m.mu.Unlock()
	if l != nil && !l.isBroken() {
		return l, ""
	}
	c, err := m.d.Dial(ctx, osu)
	if errors.Is(err, doctor.ErrPeerMismatch) {
		if m.d.Gate != nil {
			m.d.Gate.PeerMismatch(ctx)
		}
		return nil, DetailHelperUntrusted
	}
	if err != nil {
		return nil, DetailPTYFailed
	}
	l = newHelperLink(m, osu, c)
	m.mu.Lock()
	m.links[osu] = l
	m.mu.Unlock()
	return l, ""
}

// linkBroken ends every session of l: the helper ended their PTYs when the channel broke (D-W-90).
// The end is reported as process_exited without an exit code (D-M2-45 ruling).
func (m *Manager) linkBroken(l *helperLink) {
	m.mu.Lock()
	if m.links[l.user] == l {
		delete(m.links, l.user)
	}
	m.mu.Unlock()
	for _, s := range m.snapshot() {
		if s.link == l {
			s.end(endSpec{reason: ReasonProcessExited})
		}
	}
}

// fromHelper hands PTY output of the helper to its session.
func (m *Manager) fromHelper(sid [localchan.SessionIDLen]byte, f []byte) {
	if s := m.byID(sid); s != nil {
		s.output(f)
	}
}

// exited ends the session whose process exited.
func (m *Manager) exited(id string, code int) {
	m.mu.Lock()
	s := m.sessions[id]
	m.mu.Unlock()
	if s != nil {
		s.exit(code)
	}
}

func (m *Manager) byID(sid [localchan.SessionIDLen]byte) *session {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, s := range m.sessions {
		if s.idb == sid {
			return s
		}
	}
	return nil
}

// payload2 is the payload of message 2 (24 5.2).
type payload2 struct {
	V           int    `json:"v"`
	HostVersion string `json:"host_version"`
	Profile     string `json:"profile"`
}

// serve runs one data connection from the dial to the end of the handshake, then hands it to the
// opening or the attach (Design notes 3 and 5 to 7).
func (m *Manager) serve(ctx context.Context, o control.SessionOpen, n control.Notifier, arrival time.Time) {
	expires, err := time.Parse(time.RFC3339Nano, o.ExpiresAt)
	if err != nil || o.DataPath != control.DataPathPrefix+o.SessionID+"/stream" {
		return
	}
	sidb, err := noise.ParseID(o.SessionID)
	hostb, herr := noise.ParseID(m.d.HostID)
	if err != nil || herr != nil {
		return
	}
	url, err := m.d.DataURL(o.DataPath)
	if err != nil {
		return
	}
	dctx, cancel := context.WithDeadline(ctx, expires.Add(time.Second))
	dc, err := dialData(dctx, m.d.HTTP(), url, identifyData{HostID: m.d.HostID, HostVersion: m.d.HostVersion, SessionID: o.SessionID},
		func(ch string) string { return control.Proof(m.d.AuthKey, m.d.HostID, ch, m.d.Clock.Now()) })
	cancel()
	if err != nil {
		return
	}
	m.mu.Lock()
	if old := m.conns[o.SessionID]; old != nil {
		old.close(CodeNormal, "")
	}
	m.conns[o.SessionID] = dc
	m.mu.Unlock()
	h := &handshake{m: m, o: o, n: n, dc: dc, sidb: sidb, hostb: hostb, expires: expires, window: expires.Sub(arrival)}
	h.run(ctx)
}

// dropConn forgets dc as the data connection of id.
func (m *Manager) dropConn(id string, dc *dataConn) {
	m.mu.Lock()
	if m.conns[id] == dc {
		delete(m.conns, id)
	}
	m.mu.Unlock()
}

// liveOf returns the running session of id for step 8.
func (m *Manager) liveOf(id string) (*session, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[id]
	return s, ok
}

// offered returns the offered profiles and the last result.
func (m *Manager) offered() ([]doctor.Profile, *doctor.Result) {
	res := m.d.Result()
	return m.d.Offered(res), res
}

// apiKey reads claude_api_key for one start; the value is never logged.
func (m *Manager) apiKey() (string, bool) {
	if m.d.APIKey == nil {
		return "", false
	}
	b, err := m.d.APIKey()
	if err != nil {
		return "", false
	}
	key := string(trimSpace(b))
	clear(b)
	return key, key != ""
}

func trimSpace(b []byte) []byte {
	for len(b) > 0 && (b[len(b)-1] == '\n' || b[len(b)-1] == '\r' || b[len(b)-1] == ' ') {
		b = b[:len(b)-1]
	}
	return b
}

// startRequest builds pty.start for claims and the offered profile kind.
func (m *Manager) startRequest(cl Claims, kind string, res *doctor.Result, cols, rows uint16) (localchan.PTYStart, string) {
	req := localchan.PTYStart{SessionID: cl.Sid, Profile: cl.Prf, Kind: kind, Declaration: res.Declaration, Cols: cols, Rows: rows}
	switch kind {
	case config.KindCommand:
		i := slices.IndexFunc(m.d.Config.Profiles, func(p config.Profile) bool { return p.Name == cl.Prf })
		if i < 0 {
			return req, DetailProfileUnknown
		}
		req.Program, req.Args = m.d.Config.Profiles[i].Program, m.d.Config.Profiles[i].Args
	case config.KindClaude:
		list := res.Workdirs[cl.Osu]
		if cl.Wdi == nil || *cl.Wdi >= len(list) {
			return req, DetailProfileUnknown
		}
		req.WorkdirIndex, req.Workdir = cl.Wdi, list[*cl.Wdi].Path
		if res.Declaration == state.DeclOrganization {
			key, ok := m.apiKey()
			if !ok {
				return req, DetailPTYFailed
			}
			req.APIKey = key
		}
	}
	return req, ""
}

// encodeJSON is json.Marshal for the payload of message 2 (strings only: never fails).
func encodeJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}
