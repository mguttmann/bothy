// SPDX-License-Identifier: Apache-2.0

package computer

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"slices"
	"sync"
	"time"

	"github.com/OWNER/bothy/services/host/internal/clock"
	"github.com/OWNER/bothy/services/host/internal/control"
	"github.com/OWNER/bothy/services/host/internal/noise"
)

// Stream is the data connection of one computer session (channel hostComputerData): binary Noise
// messages in both directions. *session.Stream implements it.
type Stream interface {
	// In returns the messages of the server; it is closed when the connection ends.
	In() <-chan []byte
	// Done is closed when the connection ended.
	Done() <-chan struct{}
	// Send queues one message and waits while the queue is full; false when it was not queued.
	Send(ctx context.Context, m []byte) bool
	// Drain waits until every queued message was written.
	Drain(ctx context.Context)
	// Close closes the connection with code and reason.
	Close(code int, reason string)
	// CloseCode returns the close code of the server, -1 for a lost connection.
	CloseCode() int
	// Keys returns the jwks of the hello of the connection; Leeway its auth.clock_leeway.
	Keys() []map[string]any
	Leeway() time.Duration
}

// Config configures the Manager; cmd/bothy-host wires it with host_service.computer_use.
type Config struct {
	// HostID is the own host ID; NoiseKey the static Noise key pair of the host.
	HostID   string
	NoiseKey noise.KeyPair
	// HostVersion and Platform go into message 2.
	HostVersion, Platform string
	// Signature is the signature state of this build for the report: verified with a code signing
	// requirement, development without one.
	Signature string
	// Dial opens the data connection of dataPath for the session (control.Client.DataURL and
	// session.DialStream in production).
	Dial func(ctx context.Context, dataPath, sessionID string) (Stream, error)
	// Helper is the channel to the computer helper.
	Helper *Helper
	// OnReport runs when the object computer of the report changed (control.Client.ComputerChanged);
	// it must not block.
	OnReport func()
	// Clock drives every timer; nil means clock.Real(). Log receives codes only; nil means
	// slog.Default(). Random is the source of the ephemeral Noise keys; nil means crypto/rand.
	Clock  clock.Clock
	Log    *slog.Logger
	Random io.Reader
}

// Manager implements control.Computers: the computer sessions of this host, at most one live at
// a time (the controller lock, 27 8.5). It is safe for concurrent use.
type Manager struct {
	c        Config
	verifier *Verifier
	lock     controllerLock
	ctx      context.Context
	cancel   context.CancelFunc

	mu       sync.Mutex
	sessions map[string]*csession
	pending  []control.ComputerEnded
	notifier control.ComputerNotifier
	closed   bool
}

// Compile-time check: the computer port of the control connection.
var _ control.Computers = (*Manager)(nil)

// NewManager returns the manager; the caller sets c.Helper's OnEvent to Manager.HelperEvent.
func NewManager(c Config) *Manager {
	if c.Clock == nil {
		c.Clock = clock.Real()
	}
	if c.Log == nil {
		c.Log = slog.Default()
	}
	if c.Signature == "" {
		c.Signature = control.SignatureDevelopment
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Manager{c: c, verifier: NewVerifier(), ctx: ctx, cancel: cancel, sessions: map[string]*csession{}}
}

// Log messages and attributes: codes and IDs only (27 13.1).
const (
	msgStarted   = "computer session started"
	msgEnded     = "computer session ended"
	msgRefused   = "computer handshake refused"
	msgNotOpened = "computer data connection not opened"
	attrSession  = "session_id"
	attrReason   = "close_reason"
	attrDetail   = "close_detail"
	attrCode     = "code"
	attrState    = "state"
)

// Live implements control.Computers: the sessions that run and those whose end was not reported.
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

// Report implements control.Computers: the helper state as the object computer of 27 12.3.
func (m *Manager) Report() control.ComputerReport {
	st := m.c.Helper.State()
	if !st.Connected {
		r := control.AbsentReport(m.c.Signature)
		if st.Refused {
			r.Helper = control.HelperRefused
		}
		return r
	}
	return control.ComputerReport{Helper: control.HelperConnected, Enabled: st.Enabled, Generation: st.Generation, OSAccount: st.OSAccount,
		Permissions:   control.ComputerPermissions{Capture: permission(st.Permissions.Capture), Input: permission(st.Permissions.Input)},
		HelperVersion: st.HelperVersion, Signature: m.c.Signature, Locked: st.Locked}
}

// permission maps a permission of the helper to granted, denied or unknown.
func permission(p string) string {
	if p == "granted" || p == "denied" {
		return p
	}
	return control.PermUnknown
}

// Open implements control.Computers: the checks of 27 5.1 step 7 (helper, enablement, controller
// lock), the indicator, then the data connection and the handshake in their own goroutine.
func (m *Manager) Open(ctx context.Context, o control.ComputerOpen, n control.ComputerNotifier) {
	m.mu.Lock()
	m.notifier = n
	_, dup := m.sessions[o.SessionID]
	closed := m.closed
	m.mu.Unlock()
	if closed || dup {
		return
	}
	expires, err := time.Parse(time.RFC3339Nano, o.ExpiresAt)
	sidb, serr := noise.ParseID(o.SessionID)
	hostb, herr := noise.ParseID(m.c.HostID)
	if err != nil || serr != nil || herr != nil || o.DataPath != control.ComputerDataPathPrefix+o.SessionID+"/stream" || !validSettings(o.Settings) {
		m.c.Log.LogAttrs(ctx, slog.LevelWarn, msgNotOpened, slog.String(attrSession, o.SessionID))
		return
	}
	if d := m.admit(ctx, o.SessionID, -1); d != "" {
		m.report(ctx, endedOf(o.SessionID, ReasonOpenFailed, d, nil))
		m.c.Log.LogAttrs(ctx, slog.LevelInfo, msgEnded, slog.String(attrSession, o.SessionID), slog.String(attrReason, ReasonOpenFailed), slog.String(attrDetail, d))
		return
	}
	s := newSession(m, o, n, expires, sidb, hostb)
	m.mu.Lock()
	m.sessions[o.SessionID] = s
	m.mu.Unlock()
	s.show()
	go s.serve()
}

// admit runs the steps 9 to 11 of 27 5.3: the helper connected and trusted (the peer rule again),
// enabled (with generation gen when gen is not negative), and the controller lock free for id or
// held by it. It takes the lock and returns "", or the close detail of the failing step.
func (m *Manager) admit(ctx context.Context, id string, gen int64) string {
	h := m.c.Helper
	st := h.State()
	switch {
	case !st.Connected && st.Refused:
		return DetailHelperUntrusted
	case !st.Connected:
		return DetailHelperAbsent
	}
	if err := h.Recheck(ctx); err != nil {
		if errors.Is(err, ErrPeerRefused) {
			return DetailHelperUntrusted
		}
		return DetailHelperAbsent
	}
	st = h.State()
	if !st.Enabled || (gen >= 0 && st.Generation != gen) {
		return DetailNotEnabled
	}
	if !m.lock.acquire(id) {
		return DetailBusy
	}
	return ""
}

// validSettings checks the server values of computer.open.
func validSettings(s control.ComputerSettings) bool {
	return s.ImageMaxEdge > 0 && s.JPEGQuality >= 1 && s.JPEGQuality <= 100 && s.SettleMs >= 0 && s.WaitMaxMs >= 0 &&
		s.KeyRepeatMax >= 1 && s.ScrollAmountMax >= 1 && s.AuditAppsMax >= 0 && s.StatusIntervalMs > 0
}

// Close implements control.Computers: the server ended the session (27 8.4 item 1); the host
// releases, hides and closes without a computer.ended of its own (D-CU-03-5).
func (m *Manager) Close(_ context.Context, c control.ComputerClose) {
	if s := m.session(c.SessionID); s != nil {
		s.end(endSpec{reason: c.Reason, silent: true})
	}
}

// Disable implements control.Computers: the live session ends with device_disabled, then the
// helper switches its enablement off (27 4 item 4).
func (m *Manager) Disable(ctx context.Context, d control.ComputerDisable) {
	for _, s := range m.snapshot() {
		s.end(endSpec{reason: ReasonDeviceDisabled})
	}
	if err := m.c.Helper.Send(Disable{Op: OpDisable, Generation: d.Generation}); err != nil {
		m.c.Log.LogAttrs(ctx, slog.LevelWarn, "computer disable not delivered", slog.String(attrCode, DetailHelperAbsent))
	}
}

// EndAll implements control.Computers: every session ends without a report (4110, 4111, 4112,
// 4114: the server ends them itself).
func (m *Manager) EndAll(context.Context) {
	for _, s := range m.snapshot() {
		s.end(endSpec{reason: ReasonConnectionLost, silent: true})
	}
}

// Resend implements control.Computers: the computer.ended that could not be sent go out on n.
func (m *Manager) Resend(ctx context.Context, n control.ComputerNotifier) {
	m.mu.Lock()
	m.notifier = n
	pend := m.pending
	m.pending = nil
	m.mu.Unlock()
	for _, e := range pend {
		m.report(ctx, e)
	}
}

// Shutdown ends every session without a report and stops the manager (the host service stops).
func (m *Manager) Shutdown() {
	m.mu.Lock()
	m.closed = true
	m.mu.Unlock()
	m.EndAll(context.Background())
	m.cancel()
}

// HelperEvent handles a change of the helper (HelperOptions.OnEvent): a stop at the machine, a
// switch-off, a lock or unlock, a crash; every change of the state also refreshes the report.
func (m *Manager) HelperEvent(e Event) {
	switch e.Kind {
	case EventStop:
		m.helperStop(e.Stop)
		return
	case EventGone:
		for _, s := range m.snapshot() {
			s.end(endSpec{reason: ReasonError, detail: DetailHelperCrashed})
		}
	case EventState:
		for _, s := range m.snapshot() {
			m.helperState(s, e.Before, e.After)
		}
	}
	if m.c.OnReport != nil {
		go m.c.OnReport()
	}
}

// helperStop ends the session the helper names with stopped_by_user (Esc or the indicator).
func (m *Manager) helperStop(st Stop) {
	if st.Detail != DetailLocalEscape && st.Detail != DetailIndicatorStop {
		return
	}
	if s := m.session(st.SessionID); s != nil {
		s.end(endSpec{reason: ReasonStoppedByUser, detail: st.Detail})
	}
}

// helperState applies a state change to s: a local switch-off ends it, a lock pauses it, the
// unlock resumes it.
func (m *Manager) helperState(s *csession, before, after HelperState) {
	switch {
	case before.Enabled && !after.Enabled:
		s.end(endSpec{reason: ReasonDeviceDisabled})
	case after.Locked && !before.Locked:
		s.pause()
	case !after.Locked && before.Locked:
		s.resume()
	}
}

func (m *Manager) session(id string) *csession {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.sessions[id]
}

func (m *Manager) snapshot() []*csession {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]*csession, 0, len(m.sessions))
	for _, s := range m.sessions {
		out = append(out, s)
	}
	return out
}

// forget removes s and frees the controller lock.
func (m *Manager) forget(s *csession) {
	m.mu.Lock()
	if m.sessions[s.o.SessionID] == s {
		delete(m.sessions, s.o.SessionID)
	}
	m.mu.Unlock()
	m.lock.release(s.o.SessionID)
}

// report sends computer.ended, or keeps it for the next ready when the control connection is
// down.
func (m *Manager) report(ctx context.Context, e control.ComputerEnded) {
	m.mu.Lock()
	n := m.notifier
	m.mu.Unlock()
	if n == nil || n.ComputerEnded(ctx, e) != nil {
		m.mu.Lock()
		m.pending = append(m.pending, e)
		m.mu.Unlock()
	}
}

// endedOf builds computer.ended; the lists are never null.
func endedOf(id, reason, detail string, s *csession) control.ComputerEnded {
	e := control.ComputerEnded{SessionID: id, CloseReason: reason, CloseDetail: detail, ActionCounts: map[string]int64{}, Apps: []control.AppIdentity{}}
	if s != nil {
		s.mu.Lock()
		for k, v := range s.counts {
			e.ActionCounts[k] = v
		}
		e.Apps = append(e.Apps, s.apps...)
		e.PausedMs = s.pausedTotal.Milliseconds()
		s.mu.Unlock()
	}
	return e
}

// controllerLock is the lock per host held for the life of a computer session (27 8.5).
type controllerLock struct {
	mu     sync.Mutex
	holder string
}

// acquire takes the lock for id; true also when id holds it already.
func (l *controllerLock) acquire(id string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.holder != "" && l.holder != id {
		return false
	}
	l.holder = id
	return true
}

// release frees the lock when id holds it.
func (l *controllerLock) release(id string) {
	l.mu.Lock()
	if l.holder == id {
		l.holder = ""
	}
	l.mu.Unlock()
}
