// SPDX-License-Identifier: Apache-2.0

// Package bothytest is a fake server for the auth paths of the import tool
// (docs/spec/22-import-migration.md 10, docs/spec/08-identity.md 5.3, 7.3 and 7.4). It runs
// plain HTTP on 127.0.0.1 with a port the system assigns and serves beginPairing,
// completePairing (202 until the test approves, then 201; 410 after a rejection or expiry),
// refreshToken (rotation with a family and reuse detection: a reuse revokes the session), logout,
// getMe and getInstance, the import interface of imports.go and the chat read routes of chat.go
// (getChannel, listChannelThreads, listThreadMessages). It verifies every DPoP proof
// (signature, typ, htm, htu, ath, iat, jti replay, nonce and the key binding), answers use_dpop_nonce to the first request and after
// RotateNonce, counts requests per method and path, injects 429 with Retry-After and redirects,
// and can echo the presented tokens into error bodies for the leak test.
//
// Only _test.go files import this package; a test in package bothyapi holds that it never
// reaches the binary bothy.
//
// Example:
//
//	fake := bothytest.New(t, bothytest.Options{Clock: clk})
//	go func() { fake.WaitPairing(); fake.Approve() }()
//	err := bothyapi.Login(ctx, bothyapi.LoginOptions{ServerURL: fake.URL, ...})
package bothytest

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/jose"
)

// The routes of the fake, below APIPrefix.
const (
	// APIPrefix is the path prefix of the API.
	APIPrefix = "/api/v1"
	// PathBeginPairing is beginPairing.
	PathBeginPairing = APIPrefix + "/auth/pairing/begin"
	// PathCompletePairing is completePairing.
	PathCompletePairing = APIPrefix + "/auth/pairing/complete"
	// PathRefresh is refreshToken.
	PathRefresh = APIPrefix + "/auth/token/refresh"
	// PathLogout is logout.
	PathLogout = APIPrefix + "/auth/logout"
	// PathMe is getMe, the protected GET route of the fake.
	PathMe = APIPrefix + "/me"
	// PathInstance is getInstance.
	PathInstance = APIPrefix + "/instance"
)

// UserID is the person every pairing of the fake logs in.
const UserID = "0192f4a0-7b1c-7f89-8d6b-6960453241bc"

// The token lifetimes and windows of the fake.
const (
	// AccessTTL is the lifetime of an access token (expires_in).
	AccessTTL = 15 * time.Minute
	// RefreshTTL is the lifetime of a refresh token.
	RefreshTTL = 24 * time.Hour
	// PairingTTL is the validity of a pairing code.
	PairingTTL = 10 * time.Minute
	// proofWindow is how far iat of a proof may lie from the clock of the fake.
	proofWindow = time.Minute
)

// Options configure the fake.
type Options struct {
	// Clock is the time of the fake (iat checks, expiry); nil means clock.System().
	Clock clock.Clock
}

// Request is one request the fake received.
type Request struct {
	// Method and Path of the request; Path without query.
	Method, Path string
	// Header is a copy of the request headers.
	Header http.Header
}

// Registration is the device registration of a beginPairing request.
type Registration struct {
	DeviceID   string          `json:"device_id"`
	Name       string          `json:"name"`
	Platform   string          `json:"platform"`
	AppVersion string          `json:"app_version"`
	PublicJWK  json.RawMessage `json:"public_jwk"`
	Alg        string          `json:"alg"`
	StepUpKey  json.RawMessage `json:"step_up_key,omitempty"`
}

// pairing is the state of one pairing.
type pairing struct {
	id, code string
	reg      Registration
	jkt      string
	expires  time.Time
	state    string // requested, approved, rejected, collected
}

// session is a logged-in session with its refresh family.
type session struct {
	id, deviceID, jkt string
	revoked           bool
	current           string          // current refresh token
	used              map[string]bool // rotated refresh tokens
}

// access is an issued access token.
type access struct {
	s       *session
	expires time.Time
}

// The states of a pairing and the member of its expiry.
const (
	stateRequested = "requested"
	stateApproved  = "approved"
	stateRejected  = "rejected"
	stateCollected = "collected"
	memberExpires  = "expires_at"
)

// fault is an injected answer for a route.
type fault struct {
	status     int
	retryAfter string
	location   string
	times      int
}

// Server is a running fake. All methods are safe for concurrent use.
type Server struct {
	// URL is the origin of the fake, http://127.0.0.1:<port>.
	URL string

	srv *httptest.Server
	clk clock.Clock

	mu         sync.Mutex
	nonce      int
	jtis       map[string]bool
	pairings   []*pairing
	sessions   []*session
	access     map[string]*access
	requests   []Request
	faults     map[string]*fault
	echo       bool
	nonceEvery bool
	rotations  int
	onProtect  func(token string)
	registered chan struct{}
	// imp is the import interface of imports.go.
	imp *imports
}

// New starts a fake on 127.0.0.1 and closes it in tb.Cleanup.
func New(tb testing.TB, opts Options) *Server {
	tb.Helper()
	s := &Server{clk: opts.Clock, jtis: map[string]bool{}, access: map[string]*access{},
		faults: map[string]*fault{}, registered: make(chan struct{}, 16), nonce: 1}
	if s.clk == nil {
		s.clk = clock.System()
	}
	s.imp = newImports(s.clk.Now())
	s.srv = httptest.NewServer(http.HandlerFunc(s.serve))
	s.URL = s.srv.URL
	tb.Cleanup(s.srv.Close)
	return s
}

// Requests returns every request so far, in order.
func (s *Server) Requests() []Request {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Request(nil), s.requests...)
}

// Count returns how many requests with method and path arrived.
func (s *Server) Count(method, path string) int {
	n := 0
	for _, r := range s.Requests() {
		if r.Method == method && r.Path == path {
			n++
		}
	}
	return n
}

// RotateNonce makes the current nonce stale: the next proof gets use_dpop_nonce.
func (s *Server) RotateNonce() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nonce++
}

// NewNonceEveryRequest makes every request start a new nonce window, so every proof gets
// use_dpop_nonce.
func (s *Server) NewNonceEveryRequest(on bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nonceEvery = on
}

// Fail answers the next times requests to path with status; retryAfter is the Retry-After
// header ("" for none), location the Location header of a redirect ("" for none).
func (s *Server) Fail(path string, status int, retryAfter, location string, times int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.faults[path] = &fault{status: status, retryAfter: retryAfter, location: location, times: times}
}

// Echo makes every error body carry the presented access token, refresh token and proof.
func (s *Server) Echo(on bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.echo = on
}

// OnProtected calls f with the access token of every accepted request to a protected route,
// before the fake answers it.
func (s *Server) OnProtected(f func(token string)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onProtect = f
}

// WaitPairing blocks until a beginPairing was accepted.
func (s *Server) WaitPairing() { <-s.registered }

// Registration returns the device registration of the last pairing; ok is false without one.
func (s *Server) Registration() (Registration, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.pairings) == 0 {
		return Registration{}, false
	}
	return s.pairings[len(s.pairings)-1].reg, true
}

// Code returns the pairing code of the last pairing.
func (s *Server) Code() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.pairings) == 0 {
		return ""
	}
	return s.pairings[len(s.pairings)-1].code
}

// Approve approves the last pairing, as approvePairing with step-up would.
func (s *Server) Approve() { s.setPairing(stateApproved) }

// Reject rejects the last pairing: completePairing answers 410.
func (s *Server) Reject() { s.setPairing(stateRejected) }

func (s *Server) setPairing(state string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.pairings) > 0 {
		s.pairings[len(s.pairings)-1].state = state
	}
}

// ExpireAccess makes every issued access token expired: a protected request answers 401
// token_expired with the invalid_token challenge.
func (s *Server) ExpireAccess() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, a := range s.access {
		a.expires = s.clk.Now().Add(-time.Second)
	}
}

// CurrentRefresh returns the current refresh token of the last session ("" without one).
func (s *Server) CurrentRefresh() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.sessions) == 0 {
		return ""
	}
	return s.sessions[len(s.sessions)-1].current
}

// Rotations returns how many refreshToken calls rotated a token pair.
func (s *Server) Rotations() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.rotations
}

// Revoked reports whether the last session is revoked.
func (s *Server) Revoked() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.sessions) > 0 && s.sessions[len(s.sessions)-1].revoked
}

// serve dispatches one request.
func (s *Server) serve(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.requests = append(s.requests, Request{Method: r.Method, Path: r.URL.Path, Header: r.Header.Clone()})
	if s.nonceEvery {
		s.nonce++
	}
	if f := s.faults[r.URL.Path]; f != nil && f.times > 0 {
		f.times--
		if f.retryAfter != "" {
			w.Header().Set("Retry-After", f.retryAfter)
		}
		if f.location != "" {
			w.Header().Set("Location", f.location)
		}
		s.problem(w, r, f.status, "fault", body)
		return
	}
	if f := s.afterStoreFault(r); f != nil {
		s.serveAfterStore(w, r, body, f)
		return
	}
	s.route(w, r, body)
}

// route answers one request after the injected faults.
func (s *Server) route(w http.ResponseWriter, r *http.Request, body []byte) {
	switch r.Method + " " + r.URL.Path {
	case http.MethodPost + " " + PathBeginPairing:
		s.beginPairing(w, r, body)
	case http.MethodPost + " " + PathCompletePairing:
		s.completePairing(w, r, body)
	case http.MethodPost + " " + PathRefresh:
		s.refresh(w, r, body)
	case http.MethodPost + " " + PathLogout:
		if a := s.protect(w, r, body); a != nil {
			a.s.revoked = true
			w.WriteHeader(http.StatusNoContent)
		}
	case http.MethodGet + " " + PathMe:
		if a := s.protect(w, r, body); a != nil {
			writeJSON(w, http.StatusOK, map[string]any{
				"principal": map[string]any{"id": UserID, "kind": "user", "handle": "alex", "display_name": "Alex"},
				"is_owner":  s.imp.me.owner, "permission_version": 1, "device_id": a.s.deviceID, "session_id": a.s.id,
			})
		}
	case http.MethodGet + " " + PathInstance:
		if s.protect(w, r, body) != nil {
			writeJSON(w, http.StatusOK, map[string]any{
				"instance_id": "0192f4a0-7b1c-7db7-89b9-2fe4c48dcb3e", "locale_default": "en", "spec_version": "1.0.0",
				"branding": map[string]any{"product_name": "Team Chat", "accent_color": "loch", "version": 1},
				"features": []any{},
				"limits": map[string]any{"message_max_content_bytes": 262144, "message_max_attachments": 10,
					"file_max_upload_bytes": 104857600, "thread_title_max_chars": 200, "voice_max_duration_ms": 900000},
			})
		}
	default:
		if !s.serveImports(w, r, body) {
			s.problem(w, r, http.StatusNotFound, "not_found", body)
		}
	}
}

// beginPairing stores a pairing after a proof of the registered key.
func (s *Server) beginPairing(w http.ResponseWriter, r *http.Request, body []byte) {
	var in struct {
		Device Registration `json:"device"`
	}
	if json.Unmarshal(body, &in) != nil || in.Device.DeviceID == "" || len(in.Device.PublicJWK) == 0 {
		s.problem(w, r, http.StatusBadRequest, "validation_failed", body)
		return
	}
	if in.Device.Platform == "cli" && len(in.Device.StepUpKey) > 0 {
		s.problem(w, r, http.StatusBadRequest, "validation_failed", body)
		return
	}
	k, err := jose.ParseJWK(in.Device.PublicJWK)
	if err != nil {
		s.problem(w, r, http.StatusBadRequest, "validation_failed", body)
		return
	}
	if !s.proof(w, r, "", k.Thumbprint(), body) {
		return
	}
	p := &pairing{id: "0192f4a0-7b1c-7e21-8f3a-5b6c7d8e9f10", code: "7KQ2-M9XD", reg: in.Device, jkt: k.Thumbprint(),
		expires: s.clk.Now().Add(PairingTTL), state: stateRequested}
	s.pairings = append(s.pairings, p)
	writeJSON(w, http.StatusCreated, map[string]any{"pairing_id": p.id, "pairing_code": p.code, memberExpires: stamp(p.expires)})
	select {
	case s.registered <- struct{}{}:
	default:
	}
}

// completePairing answers 202, 201 or 410.
func (s *Server) completePairing(w http.ResponseWriter, r *http.Request, body []byte) {
	if len(s.pairings) == 0 {
		s.problem(w, r, http.StatusNotFound, "not_found", body)
		return
	}
	p := s.pairings[len(s.pairings)-1]
	if !s.proof(w, r, "", p.jkt, body) {
		return
	}
	switch {
	case p.state == stateRejected || p.state == stateCollected || s.clk.Now().After(p.expires):
		s.problem(w, r, http.StatusGone, "gone", body)
	case p.state == stateRequested:
		writeJSON(w, http.StatusAccepted, map[string]any{"pairing_id": p.id, "status": stateRequested, memberExpires: stamp(p.expires)})
	default:
		p.state = stateCollected
		ss := &session{id: "0192f4a0-7b1c-75c3-8a09-bf22e12411b6", deviceID: p.reg.DeviceID, jkt: p.jkt, used: map[string]bool{}}
		s.sessions = append(s.sessions, ss)
		s.issue(w, ss, http.StatusCreated)
	}
}

// refresh rotates the refresh token of a family; a reuse revokes the session.
func (s *Server) refresh(w http.ResponseWriter, r *http.Request, body []byte) {
	var in struct {
		RefreshToken string `json:"refresh_token"`
	}
	_ = json.Unmarshal(body, &in)
	var ss *session
	for _, c := range s.sessions {
		if c.current == in.RefreshToken || c.used[in.RefreshToken] {
			ss = c
		}
	}
	if ss == nil {
		s.problem(w, r, http.StatusUnauthorized, "unauthenticated", body)
		return
	}
	if !s.proof(w, r, "", ss.jkt, body) {
		return
	}
	if ss.revoked || ss.used[in.RefreshToken] {
		ss.revoked = true
		s.problem(w, r, http.StatusUnauthorized, "session_revoked", body)
		return
	}
	ss.used[ss.current] = true
	s.rotations++
	s.issue(w, ss, http.StatusOK)
}

// issue writes a fresh token pair of ss with status.
func (s *Server) issue(w http.ResponseWriter, ss *session, status int) {
	at, rt := "at-"+random(), "rt-"+random()
	ss.current = rt
	now := s.clk.Now()
	s.access[at] = &access{s: ss, expires: now.Add(AccessTTL)}
	writeJSON(w, status, map[string]any{
		"access_token": at, "token_type": "DPoP", "expires_in": int(AccessTTL / time.Second),
		"refresh_token": rt, "refresh_expires_at": stamp(now.Add(RefreshTTL)),
		"session": map[string]any{"id": ss.id, "user_id": UserID, "device_id": ss.deviceID, "auth_method": "device_pairing",
			"created_at": stamp(now), memberExpires: stamp(now.Add(RefreshTTL))},
		"device": map[string]any{"id": ss.deviceID, "user_id": UserID, "name": "bothy import", "platform": "cli",
			"state": "active", "key_thumbprint": ss.jkt, "created_at": stamp(now)},
	})
}

// protect checks the access token and its proof of a protected request and returns the token, or
// writes the 401 and returns nil.
func (s *Server) protect(w http.ResponseWriter, r *http.Request, body []byte) *access {
	tok, ok := strings.CutPrefix(r.Header.Get("Authorization"), "DPoP ")
	a := s.access[tok]
	if !ok || a == nil || a.s.revoked {
		w.Header().Set("WWW-Authenticate", `DPoP error="invalid_token", algs="ES256 EdDSA"`)
		s.problem(w, r, http.StatusUnauthorized, "unauthenticated", body)
		return nil
	}
	if !s.proof(w, r, tok, a.s.jkt, body) {
		return nil
	}
	if !s.clk.Now().Before(a.expires) {
		w.Header().Set("WWW-Authenticate", `DPoP error="invalid_token", algs="ES256 EdDSA"`)
		s.problem(w, r, http.StatusUnauthorized, "token_expired", body)
		return nil
	}
	if s.onProtect != nil {
		f := s.onProtect
		s.mu.Unlock()
		f(tok)
		s.mu.Lock()
	}
	return a
}

// proof verifies the DPoP proof of r (RFC 9449 section 4.3): one proof, typ dpop+jwt, EdDSA or
// ES256, a public jwk that signs it with thumbprint wantJKT, htm, htu without query, iat within
// the window, a new jti, the current nonce and ath of token. A failure writes the 401 and
// returns false.
func (s *Server) proof(w http.ResponseWriter, r *http.Request, token, wantJKT string, body []byte) bool {
	vals := r.Header.Values("DPoP")
	invalid := func() bool {
		w.Header().Set("WWW-Authenticate", `DPoP error="invalid_dpop_proof", algs="ES256 EdDSA"`)
		s.problem(w, r, http.StatusUnauthorized, "invalid_dpop_proof", body)
		return false
	}
	if len(vals) != 1 {
		return invalid()
	}
	j, err := jose.ParseCompactJWS(vals[0], []string{jose.AlgEdDSA, jose.AlgES256}, "dpop+jwt")
	if err != nil {
		return invalid()
	}
	k, err := jose.ParseJWK(j.Header.JWK)
	if err != nil || j.Verify(k) != nil || k.Thumbprint() != wantJKT {
		return invalid()
	}
	var c struct {
		JTI   string `json:"jti"`
		HTM   string `json:"htm"`
		HTU   string `json:"htu"`
		IAT   int64  `json:"iat"`
		Nonce string `json:"nonce"`
		ATH   string `json:"ath"`
	}
	if json.Unmarshal(j.Payload, &c) != nil || c.JTI == "" || s.jtis[c.JTI] {
		return invalid()
	}
	iat := time.Unix(c.IAT, 0)
	now := s.clk.Now()
	if c.HTM != r.Method || c.HTU != s.URL+r.URL.Path || iat.Before(now.Add(-proofWindow)) || iat.After(now.Add(proofWindow)) {
		return invalid()
	}
	wantATH := ""
	if token != "" {
		sum := sha256.Sum256([]byte(token))
		wantATH = base64.RawURLEncoding.EncodeToString(sum[:])
	}
	if c.ATH != wantATH {
		return invalid()
	}
	s.jtis[c.JTI] = true
	if c.Nonce != s.currentNonce() {
		w.Header().Set("WWW-Authenticate", `DPoP error="use_dpop_nonce", algs="ES256 EdDSA"`)
		s.problem(w, r, http.StatusUnauthorized, "use_dpop_nonce", body)
		return false
	}
	return true
}

func (s *Server) currentNonce() string { return "nonce-" + strconv.Itoa(s.nonce) }

// problem writes a problem with code; with echo on, the body also carries the request's
// credentials and body.
func (s *Server) problem(w http.ResponseWriter, r *http.Request, status int, code string, body []byte) {
	w.Header().Set("DPoP-Nonce", s.currentNonce())
	p := map[string]any{"type": APIPrefix + "/problems/" + code, "title": code, "status": status, "code": code,
		"correlation_id": "0192f4a0-7b1c-7577-8bf0-c4f4ee4f887e"}
	if s.echo {
		p["detail"] = r.Header.Get("Authorization") + " " + r.Header.Get("DPoP") + " " + string(body)
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(p)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func stamp(t time.Time) string { return t.UTC().Format("2006-01-02T15:04:05.000Z") }

func random() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}
