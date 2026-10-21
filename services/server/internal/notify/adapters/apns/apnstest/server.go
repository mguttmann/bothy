// SPDX-License-Identifier: Apache-2.0

// Package apnstest is Fake-APNs: an in-process HTTP/2 server with TLS on 127.0.0.1 that answers
// like the Apple Push Notification service (docs/spec/13-push-benachrichtigungen.md 5.1 and 7.1;
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
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"errors"
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

// Options configure the JWT check of the Fake.
type Options struct {
	// TeamID is the expected claim iss; KeyID the expected header kid.
	TeamID, KeyID string
	// PublicKey verifies the ES256 signature of the bearer JWT.
	PublicKey *ecdsa.PublicKey
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
}

// Server is a running Fake-APNs. It is safe for concurrent use.
type Server struct {
	srv           *httptest.Server
	teamID, keyID string

	mu      sync.Mutex
	pub     *ecdsa.PublicKey
	scripts map[string][]Response
	reqs    []Request
	conns   int
	// hold, when set, makes requests wait until holdN of them wait (Hold).
	hold     chan struct{}
	holdN    int
	holdSeen int
}

// devicePrefix is the path of a send request before the token (13 5.1).
const devicePrefix = "/3/device/"

// Start starts a Fake on 127.0.0.1 on a free port with HTTP/2 and TLS, under a certificate that
// RootCAs trusts. The error reports a failed listen.
func Start(opts Options) (s *Server, err error) {
	defer func() {
		// httptest panics when it cannot listen; the caller gets an error instead.
		if r := recover(); r != nil {
			s, err = nil, errors.New("apnstest: cannot listen on 127.0.0.1")
		}
	}()
	s = &Server{teamID: opts.TeamID, keyID: opts.KeyID, pub: opts.PublicKey, scripts: map[string][]Response{}}
	s.srv = httptest.NewUnstartedServer(http.HandlerFunc(s.handle))
	s.srv.EnableHTTP2 = true
	s.srv.Config.ConnState = func(_ net.Conn, st http.ConnState) {
		if st == http.StateNew {
			s.mu.Lock()
			s.conns++
			s.mu.Unlock()
		}
	}
	s.srv.StartTLS()
	return s, nil
}

// URL returns the base URL, https://127.0.0.1:<port>.
func (s *Server) URL() string { return s.srv.URL }

// RootCAs returns a pool that trusts the certificate of the Fake, for egress.Deps.RootCAs.
func (s *Server) RootCAs() *x509.CertPool {
	pool := x509.NewCertPool()
	pool.AddCert(s.srv.Certificate())
	return pool
}

// Certificate returns the TLS certificate of the Fake, for a pool that trusts several Fakes.
func (s *Server) Certificate() *x509.Certificate { return s.srv.Certificate() }

// EgressRule returns the entry of egress.allowed_hosts that lets the egress client reach the
// Fake on loopback: "127.0.0.1:<port>;allow_private".
func (s *Server) EgressRule() string {
	return strings.TrimPrefix(s.srv.URL, "https://") + ";allow_private"
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

// Requests returns a copy of every recorded request in arrival order.
func (s *Server) Requests() []Request {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Request(nil), s.reqs...)
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
	s.reqs = append(s.reqs, rec)
	s.mu.Unlock()
	s.wait(r)
	switch {
	case r.Method != http.MethodPost:
		answer(w, r, Response{Status: http.StatusMethodNotAllowed, Reason: "MethodNotAllowed"})
	case !strings.HasPrefix(r.URL.Path, devicePrefix) || rec.Token == "" || strings.Contains(rec.Token, "/"):
		answer(w, r, Response{Status: http.StatusNotFound, Reason: "BadPath"})
	case rec.JWT == "":
		answer(w, r, Response{Status: http.StatusForbidden, Reason: "MissingProviderToken"})
	case !verified:
		answer(w, r, Response{Status: http.StatusForbidden, Reason: "InvalidProviderToken"})
	default:
		answer(w, r, s.next(rec.Token))
	}
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
