// SPDX-License-Identifier: Apache-2.0

package sessiontest

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/host/internal/fingerprint"
	"github.com/OWNER/bothy/services/host/internal/noise"
)

// PairPath is the pairing endpoint of the fake server (openapi pairHost).
const PairPath = "/api/v1/hosts/pair"

// keyHostID is the member host_id of the pairing answer and of ready.
const keyHostID = "host_id"

// Errors of the scripted control connection.
var (
	// ErrNoControl is an op the test sends while no control connection is logged in.
	ErrNoControl = errors.New("sessiontest: no control connection")
	// ErrNotReported is a session.open before the host sent its report after the last ready: the
	// host would close the connection with 4002 (24 2.2 point 5), so the fake refuses to send it.
	ErrNotReported = errors.New("sessiontest: no report after the last ready")
)

// HostOptions configure NewHostServer.
type HostOptions struct {
	// Code is the pairing code the fake server accepts.
	Code string
	// Declaration is llm_credentials of every ready until SetDeclaration changes it.
	Declaration string
	// Values are the server values of every hello; the zero value means DefaultValues.
	Values Values
}

// ReportProfile is one item of HostReport.profiles.
type ReportProfile struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Kind        string `json:"kind"`
}

// ReportUser is one item of HostReport.session_users.
type ReportUser struct {
	OSUser   string   `json:"os_user"`
	Workdirs []string `json:"workdirs"`
}

// ReportCheck is one item of HostReport.doctor (HostDoctorCheck).
type ReportCheck struct {
	Check       string `json:"check"`
	OK          bool   `json:"ok"`
	Code        string `json:"code"`
	SessionUser string `json:"session_user"`
}

// Report is one report of the host service as the fake server received it.
type Report struct {
	Profiles            []ReportProfile `json:"profiles"`
	SessionUsers        []ReportUser    `json:"session_users"`
	Doctor              []ReportCheck   `json:"doctor"`
	IsServerHost        bool            `json:"is_server_host"`
	RequireLocalConfirm bool            `json:"require_local_confirm"`
	OSVersion           string          `json:"os_version"`
	// Raw is the body as sent.
	Raw json.RawMessage `json:"-"`
}

// HasProfile reports whether the report offers the profile name.
func (r Report) HasProfile(name string) bool {
	return slices.ContainsFunc(r.Profiles, func(p ReportProfile) bool { return p.Name == name })
}

// Failed reports a failed entry of check with code for sessionUser ("" for an entry without a
// session user name).
func (r Report) Failed(check, code, sessionUser string) bool {
	return slices.ContainsFunc(r.Doctor, func(c ReportCheck) bool {
		return !c.OK && c.Check == check && c.Code == code && c.SessionUser == sessionUser
	})
}

// Has reports any entry of check with code, failed or warning, for any session user.
func (r Report) Has(check, code string) bool {
	return slices.ContainsFunc(r.Doctor, func(c ReportCheck) bool { return c.Check == check && c.Code == code })
}

// Entries returns the entries of check with code.
func (r Report) Entries(check, code string) []ReportCheck {
	var out []ReportCheck
	for _, c := range r.Doctor {
		if c.Check == check && c.Code == code {
			out = append(out, c)
		}
	}
	return out
}

// Names reports whether any doctor entry names sessionUser.
func (r Report) Names(sessionUser string) bool {
	return slices.ContainsFunc(r.Doctor, func(c ReportCheck) bool { return c.SessionUser == sessionUser })
}

// Lists reports whether sessionUser is in session_users.
func (r Report) Lists(sessionUser string) bool {
	return slices.ContainsFunc(r.SessionUsers, func(u ReportUser) bool { return u.OSUser == sessionUser })
}

// Workdirs returns the reported working directories (display names) of sessionUser.
func (r Report) Workdirs(sessionUser string) []string {
	for _, u := range r.SessionUsers {
		if u.OSUser == sessionUser {
			return u.Workdirs
		}
	}
	return nil
}

// Identified is one identify of a control connection.
type Identified struct {
	ProtocolVersion int      `json:"protocol_version"`
	HostID          string   `json:"host_id"`
	HostVersion     string   `json:"host_version"`
	LiveSessions    []string `json:"live_sessions"`
	// ProofOK is set when the proof verified with the login key of the pairing over the
	// challenge of this connection.
	ProofOK bool `json:"-"`
}

// Ended is one session.ended.
type Ended struct {
	SessionID   string `json:"session_id"`
	CloseReason string `json:"close_reason"`
	CloseDetail string `json:"close_detail"`
	ExitCode    *int   `json:"exit_code"`
}

// HostServer is the fake server of the platform suite (brief M2-45 Design note 17): pairHost, the
// scripted control connection (hello, identify, ready, heartbeat_ack, every report, session.started
// and session.ended recorded) and the data endpoint of Server, on 127.0.0.1 port 0 with TLS. The
// test drives session.open, session.close and a further ready. It is safe for concurrent use.
type HostServer struct {
	*Server
	// Signer signs the session tokens; its JWK is in every hello.
	Signer *Signer
	// HostID and InstanceID are the answer of pairHost.
	HostID, InstanceID string

	code   string
	values Values

	mu       sync.Mutex
	change   chan struct{}
	decl     string
	noisePub []byte
	authPub  ed25519.PublicKey
	cur      *Conn
	reported bool
	logins   int
	reports  []Report
	ids      []Identified
	started  []string
	ended    []Ended
	received [][]byte
	stop     chan struct{}
}

// NewHostServer starts the fake server; it stops with the test.
func NewHostServer(t testing.TB, o HostOptions) *HostServer {
	if o.Values == (Values{}) {
		o.Values = DefaultValues
	}
	h := &HostServer{Signer: NewSigner("terminal-key-1"), HostID: NewID(), InstanceID: NewID(), code: o.Code, values: o.Values,
		decl: o.Declaration, change: make(chan struct{}), stop: make(chan struct{})}
	s := &Server{t: t, ctrl: make(chan *Conn, 16), data: map[string][]*Conn{}, hello: h.hello}
	s.cond = sync.NewCond(&s.mu)
	mux := http.NewServeMux()
	mux.HandleFunc("POST "+PairPath, h.servePair)
	mux.HandleFunc("GET "+ControlPath, s.serveControl)
	mux.HandleFunc("GET "+DataPrefix+"{id}/stream", s.serveData)
	s.srv = httptest.NewTLSServer(mux)
	s.URL = s.srv.URL
	s.Pool = x509.NewCertPool()
	s.Pool.AddCert(s.srv.Certificate())
	h.Server = s
	go h.accept()
	t.Cleanup(func() {
		close(h.stop)
		s.srv.Close()
	})
	return h
}

// CAPEM returns the certificate of the server as PEM, the file of bothy-host pair --ca-file.
func (h *HostServer) CAPEM() []byte {
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: h.srv.Certificate().Raw})
}

// NoisePublic returns the static Noise key of the paired host, the key the device needs.
func (h *HostServer) NoisePublic() ([noise.KeyLen]byte, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	var k [noise.KeyLen]byte
	if len(h.noisePub) != noise.KeyLen {
		return k, false
	}
	copy(k[:], h.noisePub)
	return k, true
}

// hello returns the body of every hello: the JWK of Signer and the values.
func (h *HostServer) hello() map[string]any {
	return Hello([]map[string]any{h.Signer.JWK()}, h.values)
}

// problem writes a problem answer with code.
func problem(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"type": "about:blank", "title": code, "status": status, "code": code})
}

// servePair is pairHost: the code must match; the answer carries the fingerprint the server
// computes from both public keys (24 3.2).
func (h *HostServer) servePair(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PairingCode    string            `json:"pairing_code"`
		NoisePublicKey string            `json:"noise_public_key"`
		AuthPublicJWK  map[string]string `json:"auth_public_jwk"`
	}
	body, err := readBody(r)
	if err != nil || json.Unmarshal(body, &req) != nil {
		problem(w, http.StatusBadRequest, "validation_failed")
		return
	}
	h.record(body)
	if req.PairingCode != h.code {
		problem(w, http.StatusUnauthorized, "pairing_code_invalid")
		return
	}
	np, err1 := b64.DecodeString(req.NoisePublicKey)
	ap, err2 := b64.DecodeString(req.AuthPublicJWK["x"])
	fp, err3 := fingerprint.Compute(np, ap)
	if err1 != nil || err2 != nil || err3 != nil {
		problem(w, http.StatusBadRequest, "validation_failed")
		return
	}
	h.mu.Lock()
	h.noisePub, h.authPub = np, ed25519.PublicKey(ap)
	h.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]string{keyHostID: h.HostID, "key_fingerprint": fp, "instance_id": h.InstanceID})
}

// maxBody bounds a request body of the fake server.
const maxBody = 1 << 20

func readBody(r *http.Request) ([]byte, error) {
	defer func() { _ = r.Body.Close() }()
	return io.ReadAll(io.LimitReader(r.Body, maxBody))
}

// record keeps a body the host service sent (the canary checks of the suite read them).
func (h *HostServer) record(b []byte) {
	h.mu.Lock()
	h.received = append(h.received, append([]byte(nil), b...))
	h.mu.Unlock()
}

// Received returns every body the host service sent on pairHost and the control connections.
func (h *HostServer) Received() [][]byte {
	h.mu.Lock()
	defer h.mu.Unlock()
	return slices.Clone(h.received)
}

// notifyLocked wakes every waiter; h.mu is held.
func (h *HostServer) notifyLocked() {
	close(h.change)
	h.change = make(chan struct{})
}

// wait blocks until cond (called with h.mu held) holds or ctx ends.
func (h *HostServer) wait(ctx context.Context, cond func() bool) error {
	for {
		h.mu.Lock()
		ok, ch := cond(), h.change
		h.mu.Unlock()
		if ok {
			return nil
		}
		select {
		case <-ch:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// accept serves every control connection of the host service in turn.
func (h *HostServer) accept() {
	for {
		select {
		case c := <-h.ctrl:
			go h.serveLogin(c)
		case <-h.stop:
			return
		}
	}
}

// loginWait bounds the wait for identify (a protocol bound of the fake, not a product window).
const loginWait = 30 * time.Second

// serveLogin runs hello, identify and ready on c, then answers and records its frames until c ends.
func (h *HostServer) serveLogin(c *Conn) {
	ctx := context.Background()
	var nonce [32]byte
	_, _ = rand.Read(nonce[:])
	challenge := b64.EncodeToString(nonce[:])
	hello := h.hello()
	hello["challenge"] = challenge
	if c.Send(ctx, "hello", hello) != nil {
		return
	}
	wctx, cancel := context.WithTimeout(ctx, loginWait)
	f, err := c.Next(wctx)
	cancel()
	if err != nil || f.Err != nil || f.Op != "identify" {
		c.Close(4002, "")
		return
	}
	h.record(f.Raw)
	var id Identified
	if json.Unmarshal(f.D, &id) != nil {
		c.Close(4002, "")
		return
	}
	var proof struct {
		Proof string `json:"proof"`
	}
	_ = json.Unmarshal(f.D, &proof)
	h.mu.Lock()
	id.ProofOK = id.HostID == h.HostID && verifyProof(h.authPub, proof.Proof, challenge, h.HostID)
	h.ids = append(h.ids, id)
	h.cur, h.reported = c, false
	h.logins++
	decl := h.decl
	h.notifyLocked()
	h.mu.Unlock()
	if c.Send(ctx, "ready", map[string]any{keyHostID: h.HostID, "state": "active", "llm_credentials": decl}) != nil {
		return
	}
	h.serveOps(c)
}

// serveOps records the frames of c until it ends.
func (h *HostServer) serveOps(c *Conn) {
	for {
		f, err := c.Next(context.Background())
		if err != nil || f.Err != nil {
			break
		}
		if !f.Binary {
			h.handle(c, f)
		}
	}
	h.mu.Lock()
	if h.cur == c {
		h.cur, h.reported = nil, false
	}
	h.notifyLocked()
	h.mu.Unlock()
}

// handle records one text frame of c and answers a heartbeat.
func (h *HostServer) handle(c *Conn, f Frame) {
	h.record(f.Raw)
	if f.Op == "heartbeat" {
		_ = c.Send(context.Background(), "heartbeat_ack", map[string]any{"server_time": time.Now().UTC().Format(time.RFC3339Nano)})
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	defer h.notifyLocked()
	switch f.Op {
	case "report":
		var r Report
		if json.Unmarshal(f.D, &r) == nil {
			r.Raw = append(json.RawMessage(nil), f.D...)
			h.reports = append(h.reports, r)
			h.reported = h.reported || h.cur == c
		}
	case "session.started":
		var s struct {
			SessionID string `json:"session_id"`
		}
		if json.Unmarshal(f.D, &s) == nil {
			h.started = append(h.started, s.SessionID)
		}
	case "session.ended":
		var e Ended
		if json.Unmarshal(f.D, &e) == nil {
			h.ended = append(h.ended, e)
		}
	}
}

// verifyProof checks the identify proof (24 3.5): an EdDSA JWS of the login key over the challenge
// of this connection and the host ID.
func verifyProof(pub ed25519.PublicKey, jws, challenge, hostID string) bool {
	parts := strings.Split(jws, ".")
	if len(pub) != ed25519.PublicKeySize || len(parts) != 3 {
		return false
	}
	sig, err := b64.DecodeString(parts[2])
	if err != nil || !ed25519.Verify(pub, []byte(parts[0]+"."+parts[1]), sig) {
		return false
	}
	cb, err := b64.DecodeString(parts[1])
	if err != nil {
		return false
	}
	var cl struct {
		Challenge string `json:"challenge"`
		HostID    string `json:"host_id"`
	}
	return json.Unmarshal(cb, &cl) == nil && cl.Challenge == challenge && cl.HostID == hostID
}

// live returns the logged-in control connection and whether it reported after the last ready.
func (h *HostServer) live() (*Conn, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.cur, h.reported
}

// WaitReady waits until a control connection is logged in and the host sent its report after the
// last ready, the point from which session.open is allowed.
func (h *HostServer) WaitReady(ctx context.Context) error {
	return h.wait(ctx, func() bool { return h.cur != nil && h.reported })
}

// WaitLogins waits until n control connections logged in since the start.
func (h *HostServer) WaitLogins(ctx context.Context, n int) error {
	return h.wait(ctx, func() bool { return h.logins >= n })
}

// Logins returns the number of control connections that sent identify.
func (h *HostServer) Logins() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.logins
}

// Send sends the frame {op, d} on the logged-in control connection.
func (h *HostServer) Send(ctx context.Context, op string, d any) error {
	c, _ := h.live()
	if c == nil {
		return ErrNoControl
	}
	return c.Send(ctx, op, d)
}

// Open sends session.open for id with profile; expires_at is now plus window. Errors:
// ErrNoControl, ErrNotReported, or the write error.
func (h *HostServer) Open(ctx context.Context, id, profile string, window time.Duration) error {
	c, reported := h.live()
	switch {
	case c == nil:
		return ErrNoControl
	case !reported:
		return ErrNotReported
	}
	return c.Send(ctx, "session.open", map[string]any{"session_id": id, "data_path": DataPrefix + id + "/stream", "profile": profile,
		"expires_at": time.Now().Add(window).UTC().Format(time.RFC3339Nano)})
}

// CloseSession sends session.close for id with reason.
func (h *HostServer) CloseSession(ctx context.Context, id, reason string) error {
	return h.Send(ctx, "session.close", map[string]any{"session_id": id, "close_reason": reason})
}

// SetDeclaration changes llm_credentials and sends a further ready on the logged-in connection,
// as the server does after a committed change of the declaration (M2-45 Design note 2b).
func (h *HostServer) SetDeclaration(ctx context.Context, decl string) error {
	h.mu.Lock()
	h.decl = decl
	c := h.cur
	h.reported = false
	h.mu.Unlock()
	if c == nil {
		return ErrNoControl
	}
	return c.Send(ctx, "ready", map[string]any{keyHostID: h.HostID, "state": "active", "llm_credentials": decl})
}

// ReportCount returns the number of reports so far; WaitReport takes it as the start.
func (h *HostServer) ReportCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.reports)
}

// Reports returns every report so far.
func (h *HostServer) Reports() []Report {
	h.mu.Lock()
	defer h.mu.Unlock()
	return slices.Clone(h.reports)
}

// WaitReport returns the first report from index since on for which cond holds.
func (h *HostServer) WaitReport(ctx context.Context, since int, cond func(Report) bool) (Report, error) {
	var out Report
	err := h.wait(ctx, func() bool {
		for i := since; i < len(h.reports); i++ {
			if cond(h.reports[i]) {
				out = h.reports[i]
				return true
			}
		}
		return false
	})
	return out, err
}

// Identifies returns every identify of the control connections.
func (h *HostServer) Identifies() []Identified {
	h.mu.Lock()
	defer h.mu.Unlock()
	return slices.Clone(h.ids)
}

// StartedCount returns how often session.started came for id.
func (h *HostServer) StartedCount(id string) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	n := 0
	for _, s := range h.started {
		if s == id {
			n++
		}
	}
	return n
}

// EndedOf returns the session.ended of id, if one came.
func (h *HostServer) EndedOf(id string) (Ended, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, e := range h.ended {
		if e.SessionID == id {
			return e, true
		}
	}
	return Ended{}, false
}

// EndedCount returns the number of session.ended of id.
func (h *HostServer) EndedCount(id string) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	n := 0
	for _, e := range h.ended {
		if e.SessionID == id {
			n++
		}
	}
	return n
}

// WaitEnded waits for the session.ended of id.
func (h *HostServer) WaitEnded(ctx context.Context, id string) (Ended, error) {
	var out Ended
	err := h.wait(ctx, func() bool {
		for _, e := range h.ended {
			if e.SessionID == id {
				out = e
				return true
			}
		}
		return false
	})
	return out, err
}

// WaitStarted waits for a session.started of id.
func (h *HostServer) WaitStarted(ctx context.Context, id string) error {
	return h.wait(ctx, func() bool { return slices.Contains(h.started, id) })
}
