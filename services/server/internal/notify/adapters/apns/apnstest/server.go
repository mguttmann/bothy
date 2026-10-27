// SPDX-License-Identifier: Apache-2.0

// Package apnstest is Fake-APNs: an in-process HTTP/2 server with TLS on 127.0.0.1 that answers
// like the Apple Push Notification service (docs/spec/13-push-notifications.md 5.1 and 7.1;
// PLAN 5 row "Test doubles"). It checks the bearer JWT as APNs does, answers per device token
// from a script, and records every request, so the tests of the adapter notify/adapters/apns
// (M3-20) and the push handoff measurement (M3-24) never reach Apple. The package imports no
// testing package, so a load test can use it too; the server binary never links it.
//
// Example:
//
//	srv, err := apnstest.Start(apnstest.Options{TeamID: "EXAMPLETEAM", KeyID: "EXAMPLEKEY", PublicKey: &key.PublicKey})
//	defer srv.Close()
//	srv.Script(token, apnstest.Response{Status: 410, Reason: "Unregistered", Timestamp: at})
//	// egress.allowed_hosts gets srv.EgressRule(), egress.Deps.RootCAs srv.RootCAs(),
//	// apns.Deps.Endpoints maps the environment to srv.URL(); then:
//	reqs := srv.Requests()
package apnstest

import (
	"context"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/jose"
)

// Options configure the JWT check of the Fake and, for a Fake in its own container (fakepush of
// M5-11), where it listens, under which host name and with which certificate.
type Options struct {
	// TeamID is the expected claim iss; KeyID the expected header kid.
	TeamID, KeyID string
	// PublicKey verifies the ES256 signature of the bearer JWT.
	PublicKey *ecdsa.PublicKey
	// Addr is the listen address, for example ":8443" in a container; empty means 127.0.0.1 on a
	// free port.
	Addr string
	// Host is the host name of URL and EgressRule, for example "fakepush" on a Compose network;
	// empty means the listen address. The certificate must name it.
	Host string
	// Certificate is the TLS certificate of the Fake, for example a leaf of a per-run CA for
	// Host; nil means the certificate of httptest for 127.0.0.1.
	Certificate *tls.Certificate
	// OnAccept, when set, receives a copy of every request the Fake answers with 200, right
	// before the answer is written, so a capture can number the sends of several Fakes in one
	// order. It runs on the goroutine of the request and must not block.
	OnAccept func(Request)
}

// Response is one scripted answer.
type Response struct {
	// Status is the HTTP status; 0 means 200.
	Status int
	// Reason is the body {"reason": ...}; empty means no body.
	Reason string
	// Timestamp is the body member "timestamp" in Unix milliseconds (APNs sends it with 410);
	// zero means absent.
	Timestamp time.Time
}

// Request is one recorded request.
type Request struct {
	// Proto is the protocol of the request, "HTTP/2.0" when the client spoke HTTP/2.
	Proto string
	// Method is the HTTP method; Token the device token of the path /3/device/<token>.
	Method, Token string
	// Header is a copy of the request header.
	Header http.Header
	// Body is the request body.
	Body []byte
	// JWT is the bearer token without the prefix; empty when absent.
	JWT string
	// KeyID and Issuer are the header kid and the claim iss of JWT when it parsed.
	KeyID, Issuer string
	// IssuedAt is the claim iat of JWT when it parsed.
	IssuedAt time.Time
	// AcceptedAt is the time of the test process at which the Fake accepted the request: after it
	// read the whole request and verified the JWT, immediately before it wrote the status 200
	// ("the provider has accepted", t1 of 13 section 9; the push handoff measurement of M3-24).
	// It is zero for every other answer.
	AcceptedAt time.Time
}

// Server is a running Fake-APNs. It is safe for concurrent use.
type Server struct {
	srv           *httptest.Server
	teamID, keyID string
	// url is the base URL under Options.Host; onAccept is Options.OnAccept.
	url      string
	onAccept func(Request)

	mu      sync.Mutex
	pub     *ecdsa.PublicKey
	scripts map[string][]Response
	reqs    []Request
	// answered marks the entries of reqs whose answer is decided; Requests returns only those,
	// so no reader sees a request before its AcceptedAt is final.
	answered []bool
	conns    int
	// hold, when set, makes requests wait until holdN of them wait (Hold).
	hold     chan struct{}
	holdN    int
	holdSeen int
}

// devicePrefix is the path of a send request before the token (13 5.1).
const devicePrefix = "/3/device/"

// Start starts a Fake with HTTP/2 and TLS, under a certificate that RootCAs trusts: by default on
// 127.0.0.1 on a free port, else on Options.Addr under Options.Host and Options.Certificate. The
// error reports a failed listen.
func Start(opts Options) (s *Server, err error) {
	defer func() {
		// httptest panics when it cannot listen; the caller gets an error instead.
		if r := recover(); r != nil {
			s, err = nil, errors.New("apnstest: cannot listen on 127.0.0.1")
		}
	}()
	s = &Server{teamID: opts.TeamID, keyID: opts.KeyID, pub: opts.PublicKey, scripts: map[string][]Response{}, onAccept: opts.OnAccept}
	s.srv = httptest.NewUnstartedServer(http.HandlerFunc(s.handle))
	if err := listen(s.srv, opts.Addr, opts.Certificate); err != nil {
		return nil, err
	}
	s.srv.EnableHTTP2 = true
	s.srv.Config.ConnState = func(_ net.Conn, st http.ConnState) {
		if st == http.StateNew {
			s.mu.Lock()
			s.conns++
			s.mu.Unlock()
		}
	}
	s.srv.StartTLS()
	s.url = s.srv.URL
	if opts.Host != "" {
		_, port, _ := net.SplitHostPort(s.srv.Listener.Addr().String())
		s.url = "https://" + net.JoinHostPort(opts.Host, port)
	}
	return s, nil
}

// listen replaces the listener of the unstarted srv with one on addr and sets cert as its TLS
// certificate; an empty addr and a nil cert keep what httptest chose. The error reports a failed
// listen on addr.
func listen(srv *httptest.Server, addr string, cert *tls.Certificate) error {
	if addr != "" {
		var lc net.ListenConfig
		ln, err := lc.Listen(context.Background(), "tcp", addr)
		if err != nil {
			_ = srv.Listener.Close()
			return fmt.Errorf("apnstest: listen on %s: %w", addr, err)
		}
		_ = srv.Listener.Close()
		srv.Listener = ln
	}
	if cert != nil {
		srv.TLS = &tls.Config{Certificates: []tls.Certificate{*cert}, MinVersion: tls.VersionTLS12}
	}
	return nil
}

// URL returns the base URL: https://127.0.0.1:<port>, or https://<Options.Host>:<port>.
func (s *Server) URL() string { return s.url }

// RootCAs returns a pool that trusts the certificate of the Fake, for egress.Deps.RootCAs.
func (s *Server) RootCAs() *x509.CertPool {
	pool := x509.NewCertPool()
	pool.AddCert(s.srv.Certificate())
	return pool
}

// Certificate returns the TLS certificate of the Fake, for a pool that trusts several Fakes.
func (s *Server) Certificate() *x509.Certificate { return s.srv.Certificate() }

// EgressRule returns the entry of egress.allowed_hosts that lets the egress client reach the
// Fake on a private address: "127.0.0.1:<port>;allow_private", or "<Options.Host>:<port>;allow_private".
func (s *Server) EgressRule() string {
	return strings.TrimPrefix(s.url, "https://") + ";allow_private"
}

// Script sets the answers for token: the requests for it get rs in order, then 200.
func (s *Server) Script(token string, rs ...Response) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.scripts[token] = append([]Response(nil), rs...)
}

// Hold makes the next n requests wait until all n have arrived, then answers them; a test uses
// it to have n parallel sends in flight with the same JWT at once. A request whose client gives
// up stops waiting.
func (s *Server) Hold(n int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.hold, s.holdN, s.holdSeen = make(chan struct{}), n, 0
}

// wait blocks a held request until the last one of Hold arrives.
func (s *Server) wait(r *http.Request) {
	s.mu.Lock()
	ch := s.hold
	if ch == nil {
		s.mu.Unlock()
		return
	}
	s.holdSeen++
	if s.holdSeen == s.holdN {
		close(ch)
		s.hold = nil
	}
	s.mu.Unlock()
	select {
	case <-ch:
	case <-r.Context().Done():
	}
}

// Requests returns a copy of every recorded request whose answer is decided, in arrival order;
// a request still on its way (held, or between its JWT check and its answer) is left out, so its
// AcceptedAt is final in every copy.
func (s *Server) Requests() []Request {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Request, 0, len(s.reqs))
	for i, r := range s.reqs {
		if s.answered[i] {
			out = append(out, r)
		}
	}
	return out
}

// Connections returns the number of TLS connections accepted so far.
func (s *Server) Connections() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.conns
}

// SetPublicKey replaces the key that verifies the JWT, after a key rotation.
func (s *Server) SetPublicKey(k *ecdsa.PublicKey) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pub = k
}

// Close stops the server and closes its connections.
func (s *Server) Close() { s.srv.Close() }

// handle records the request, checks method, path and JWT like APNs and answers the script.
func (s *Server) handle(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	rec := Request{Proto: r.Proto, Method: r.Method, Header: r.Header.Clone(), Body: body}
	if tok, ok := strings.CutPrefix(r.URL.Path, devicePrefix); ok {
		rec.Token = tok
	}
	if bearer, ok := strings.CutPrefix(r.Header.Get("Authorization"), "bearer "); ok {
		rec.JWT = bearer
	}
	verified := s.parseJWT(&rec)
	s.mu.Lock()
	idx := len(s.reqs)
	s.reqs = append(s.reqs, rec)
	s.answered = append(s.answered, false)
	s.mu.Unlock()
	s.wait(r)
	var resp Response
	switch {
	case r.Method != http.MethodPost:
		resp = Response{Status: http.StatusMethodNotAllowed, Reason: "MethodNotAllowed"}
	case !strings.HasPrefix(r.URL.Path, devicePrefix) || rec.Token == "" || strings.Contains(rec.Token, "/"):
		resp = Response{Status: http.StatusNotFound, Reason: "BadPath"}
	case rec.JWT == "":
		resp = Response{Status: http.StatusForbidden, Reason: "MissingProviderToken"}
	case !verified:
		resp = Response{Status: http.StatusForbidden, Reason: "InvalidProviderToken"}
	default:
		resp = s.next(rec.Token)
	}
	accepted := resp.Status == 0 || resp.Status == http.StatusOK
	s.mu.Lock()
	if accepted {
		s.reqs[idx].AcceptedAt = time.Now()
	}
	s.answered[idx] = true
	rec = s.reqs[idx]
	s.mu.Unlock()
	if accepted && s.onAccept != nil {
		s.onAccept(rec)
	}
	answer(w, r, resp)
}

// parseJWT fills KeyID, Issuer and IssuedAt of rec and reports whether the JWT verifies with the
// current key and carries the expected kid and iss.
func (s *Server) parseJWT(rec *Request) bool {
	if rec.JWT == "" {
		return false
	}
	j, err := jose.ParseCompactJWS(rec.JWT, []string{jose.AlgES256}, "")
	if err != nil {
		return false
	}
	var claims struct {
		Iss string `json:"iss"`
		Iat int64  `json:"iat"`
	}
	if json.Unmarshal(j.Payload, &claims) != nil {
		return false
	}
	rec.KeyID, rec.Issuer, rec.IssuedAt = j.Header.Kid, claims.Iss, time.Unix(claims.Iat, 0).UTC()
	s.mu.Lock()
	pub := s.pub
	s.mu.Unlock()
	if pub == nil {
		return false
	}
	k, err := jose.JWKFromPublicKey(pub)
	if err != nil || j.Verify(k) != nil {
		return false
	}
	return j.Header.Kid == s.keyID && claims.Iss == s.teamID
}

// next pops the next scripted answer of token, or 200.
func (s *Server) next(token string) Response {
	s.mu.Lock()
	defer s.mu.Unlock()
	rs := s.scripts[token]
	if len(rs) == 0 {
		return Response{}
	}
	s.scripts[token] = rs[1:]
	return rs[0]
}

// answer writes resp with the header apns-id: the one of the request, or a new UUID.
func answer(w http.ResponseWriter, r *http.Request, resp Response) {
	id := r.Header.Get("Apns-Id")
	if id == "" {
		id = newUUID()
	}
	w.Header().Set("Apns-Id", id)
	status := resp.Status
	if status == 0 {
		status = http.StatusOK
	}
	if resp.Reason == "" {
		w.WriteHeader(status)
		return
	}
	b := `{"reason":` + strconv.Quote(resp.Reason)
	if !resp.Timestamp.IsZero() {
		b += `,"timestamp":` + strconv.FormatInt(resp.Timestamp.UnixMilli(), 10)
	}
	b += "}"
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(b))
}

// newUUID returns a random UUID in canonical form, like the apns-id APNs generates.
func newUUID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	h := hex.EncodeToString(b[:])
	return h[:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:]
}
