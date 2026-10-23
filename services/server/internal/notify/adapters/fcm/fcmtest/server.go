// SPDX-License-Identifier: Apache-2.0

// Package fcmtest is Fake-FCM: an in-process HTTP/1.1 server with TLS on 127.0.0.1 that answers
// like the OAuth 2.0 token endpoint of Google and the FCM HTTP v1 send endpoint
// (docs/spec/13-push-notifications.md 5.2 and 7.2; PLAN 5 row "Test doubles"). It verifies the
// RS256 assertion of the service account, issues and checks bearer tokens, answers sends per
// device token from a script, and records every send request, so the tests of the adapter
// notify/adapters/fcm (M3-21) and later client tests (M5-05) never reach Google. The package
// imports no testing package, so a load test can use it too; the server binary never links it.
//
// Example:
//
//	srv, err := fcmtest.Start(fcmtest.Options{ProjectID: "example-chat", ClientEmail: "push@example-chat.example.com", KeyID: "EXAMPLEKEY", PublicKey: &key.PublicKey})
//	defer srv.Close()
//	account := srv.ServiceAccountJSON(key) // the secret file of push.fcm.service_account_ref
//	srv.Script(token, fcmtest.Response{Status: 404, RPCStatus: "NOT_FOUND", ErrorCode: "UNREGISTERED"})
//	// egress.allowed_hosts gets srv.EgressRule(), egress.Deps.RootCAs srv.RootCAs(),
//	// fcm.Deps.Endpoints gets srv.URL() and srv.TokenURL(); then:
//	reqs := srv.Requests()
package fcmtest

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"log"
	"mime"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/OWNER/bothy/services/server/internal/platform/egress"
)

// Scope is the OAuth scope the assertion must name (13 5.2).
const Scope = "https://www.googleapis.com/auth/firebase.messaging"

// GrantType is the grant_type of the token request (RFC 7523 2.1).
const GrantType = "urn:ietf:params:oauth:grant-type:jwt-bearer"

// maxAssertionLifetime is the longest exp - iat the Fake accepts, as Google does.
const maxAssertionLifetime = int64(time.Hour / time.Second)

// defaultErrorMessage is error.message of an error body when Options.ErrorMessage is empty.
const defaultErrorMessage = "Fake-FCM refused the request."

// Options configure the Fake.
type Options struct {
	// ProjectID is the project of the send path /v1/projects/<ProjectID>/messages:send.
	ProjectID string
	// ClientEmail is the expected claim iss of the assertion.
	ClientEmail string
	// KeyID is the expected header kid of the assertion.
	KeyID string
	// PublicKey verifies the RS256 signature of the assertion.
	PublicKey *rsa.PublicKey
	// Lifetime is expires_in of issued tokens; 0 means one hour.
	Lifetime time.Duration
	// ErrorMessage is error.message in every error body of the send endpoint, for example a
	// canary that must never reach a log; empty means a fixed default text.
	ErrorMessage string
}

// detailType is the key of the type of an entry of error.details.
const detailType = "@type"

// Response is one scripted answer of the send endpoint.
type Response struct {
	// Status is the HTTP status; 0 means 200. A 200 answers {"name": "projects/<id>/messages/<id>"},
	// every other status the FCM error body {"error": {"code", "status", "message", "details"}}.
	Status int
	// ErrorCode is errorCode of the FcmError detail; empty: no FcmError detail.
	ErrorCode string
	// RPCStatus is error.status, for example "UNAVAILABLE".
	RPCStatus string
	// TokenField adds a BadRequest detail with a violation on message.token.
	TokenField bool
	// OtherField adds a BadRequest detail with a violation on message.android.ttl, the shape
	// Google documents for a bad field value.
	OtherField bool
	// RetryAfter is the Retry-After header verbatim; empty: absent.
	RetryAfter string
}

// TokenResponse is one scripted answer of the token endpoint to a valid token request.
type TokenResponse struct {
	// Status is the HTTP status; 0 or 200 means 200 with a new token.
	Status int
	// Error is the OAuth error of a non-200 answer, the body {"error": Error}.
	Error string
}

// Request is one recorded send request.
type Request struct {
	// Proto is the protocol of the request, "HTTP/1.1" for the adapter; Method and Path are the
	// HTTP method and the URL path.
	Proto, Method, Path string
	// Header is a copy of the request header.
	Header http.Header
	// Body is the request body.
	Body []byte
	// Bearer is the access token of the Authorization header without "Bearer ".
	Bearer string
}

// Server is a running Fake-FCM. It is safe for concurrent use.
type Server struct {
	srv  *httptest.Server
	opts Options

	mu          sync.Mutex
	pub         *rsa.PublicKey
	scripts     map[string][]Response
	tokenScript []TokenResponse
	tokens      map[string]bool
	reqs        []Request
	tokenReqs   int
}

// tokenPath is the path of the token endpoint.
const tokenPath = "/token"

// Start starts a Fake on 127.0.0.1 on a free port with HTTP/1.1 and TLS, under a certificate
// that RootCAs trusts. The error reports a failed listen.
func Start(opts Options) (s *Server, err error) {
	defer func() {
		// httptest panics when it cannot listen; the caller gets an error instead.
		if r := recover(); r != nil {
			s, err = nil, errors.New("fcmtest: cannot listen on 127.0.0.1")
		}
	}()
	if opts.Lifetime <= 0 {
		opts.Lifetime = time.Hour
	}
	if opts.ErrorMessage == "" {
		opts.ErrorMessage = defaultErrorMessage
	}
	s = &Server{opts: opts, pub: opts.PublicKey, scripts: map[string][]Response{}, tokens: map[string]bool{}}
	s.srv = httptest.NewUnstartedServer(http.HandlerFunc(s.handle))
	// Connections that a client closes during the handshake at Close are no finding of a test.
	s.srv.Config.ErrorLog = log.New(io.Discard, "", 0)
	s.srv.StartTLS()
	return s, nil
}

// URL returns the base URL, https://127.0.0.1:<port>, for Endpoints.Send.
func (s *Server) URL() string { return s.srv.URL }

// TokenURL returns URL() + "/token", for Endpoints.Token and the claim aud.
func (s *Server) TokenURL() string { return s.srv.URL + tokenPath }

// RootCAs returns a pool that trusts the certificate of the Fake, for egress.Deps.RootCAs.
func (s *Server) RootCAs() *x509.CertPool {
	pool := x509.NewCertPool()
	pool.AddCert(s.srv.Certificate())
	return pool
}

// EgressRule returns the entry of egress.allowed_hosts that lets the egress client reach the
// Fake on loopback: "127.0.0.1:<port>;allow_private".
func (s *Server) EgressRule() string {
	return strings.TrimPrefix(s.srv.URL, "https://") + ";allow_private"
}

// EgressConfig returns the egress configuration of a test against the Fake: EgressRule as the
// only allowed host, an answer limit of 64 KiB and timeout as the request deadline. Tests pass
// testkit.Patience as timeout, so many parallel TLS requests on a loaded machine never hit a
// fixed deadline (the class of a wall-clock bound, docs/DECISIONS.md D-M3-21-16).
func (s *Server) EgressConfig(timeout time.Duration) (egress.Config, error) {
	r, err := egress.ParseRule(s.EgressRule())
	if err != nil {
		return egress.Config{}, err
	}
	return egress.Config{AllowedHosts: []egress.Rule{r}, Timeout: timeout, MaxResponseBytes: 1 << 16}, nil
}

// ServiceAccountJSON returns a service account JSON for key with type "service_account",
// client_email = Options.ClientEmail, private_key_id = Options.KeyID, private_key = key as
// PKCS #8 PEM ("PRIVATE KEY") and token_uri = TokenURL(). It returns nil when key cannot be
// encoded (nil or invalid).
func (s *Server) ServiceAccountJSON(key *rsa.PrivateKey) []byte {
	if key == nil {
		return nil
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return nil
	}
	b, err := json.Marshal(struct { // #nosec G117 -- the service account of a per-run test key, the secret file a test writes
		Type         string `json:"type"`
		ClientEmail  string `json:"client_email"`
		PrivateKeyID string `json:"private_key_id"`
		PrivateKey   string `json:"private_key"`
		TokenURI     string `json:"token_uri"`
	}{"service_account", s.opts.ClientEmail, s.opts.KeyID, string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})), s.TokenURL()})
	if err != nil {
		return nil
	}
	return b
}

// Script sets the send answers for the device token: the requests whose message.token is token
// get rs in order, then 200. A later call replaces the rest of the earlier script.
func (s *Server) Script(token string, rs ...Response) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.scripts[token] = append([]Response(nil), rs...)
}

// ScriptToken sets the answers to valid token requests: rs in order, then 200 with a new token.
// A request with a bad form or assertion gets 400 invalid_grant and uses no scripted answer.
func (s *Server) ScriptToken(rs ...TokenResponse) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tokenScript = append([]TokenResponse(nil), rs...)
}

// Requests returns a copy of every recorded send request in arrival order: the requests that
// passed method, path, bearer and body checks.
func (s *Server) Requests() []Request {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Request(nil), s.reqs...)
}

// TokenRequests returns the number of POST requests to the token endpoint so far, refused ones
// included.
func (s *Server) TokenRequests() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.tokenReqs
}

// RevokeTokens revokes every token issued so far: sends with them get 401 UNAUTHENTICATED.
// Tokens issued later are valid.
func (s *Server) RevokeTokens() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tokens = map[string]bool{}
}

// SetPublicKey replaces the key that verifies the assertion, after a key rotation.
func (s *Server) SetPublicKey(k *rsa.PublicKey) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pub = k
}

// Close stops the server and closes its connections.
func (s *Server) Close() { s.srv.Close() }

// handle routes to the token endpoint or the send endpoint.
func (s *Server) handle(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == tokenPath {
		s.handleToken(w, r)
		return
	}
	s.handleSend(w, r)
}

// handleToken answers a token request (RFC 7523 2.1) like Google's token endpoint.
func (s *Server) handleToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		oauthError(w, http.StatusMethodNotAllowed, "invalid_request", "")
		return
	}
	s.mu.Lock()
	s.tokenReqs++
	s.mu.Unlock()
	body, _ := io.ReadAll(r.Body)
	assertion, ok := tokenForm(r.Header.Get("Content-Type"), body)
	if !ok {
		oauthError(w, http.StatusBadRequest, "invalid_grant", "bad token request form")
		return
	}
	if !s.verifyAssertion(assertion) {
		oauthError(w, http.StatusBadRequest, "invalid_grant", "invalid JWT assertion")
		return
	}
	s.mu.Lock()
	var resp TokenResponse
	if len(s.tokenScript) > 0 {
		resp, s.tokenScript = s.tokenScript[0], s.tokenScript[1:]
	}
	if resp.Status != 0 && resp.Status != http.StatusOK {
		s.mu.Unlock()
		oauthError(w, resp.Status, resp.Error, "")
		return
	}
	tok := "ya29.fake-" + randomHex(24)
	s.tokens[tok] = true
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]any{
		"access_token": tok,
		"token_type":   "Bearer",
		"expires_in":   int64(s.opts.Lifetime / time.Second),
	})
}

// tokenForm returns the assertion of a form with content type application/x-www-form-urlencoded
// and exactly the fields grant_type (= GrantType) and assertion, each once.
func tokenForm(contentType string, body []byte) (string, bool) {
	mt, _, err := mime.ParseMediaType(contentType)
	if err != nil || mt != "application/x-www-form-urlencoded" {
		return "", false
	}
	form, err := url.ParseQuery(string(body))
	if err != nil || len(form) != 2 || len(form["grant_type"]) != 1 || len(form["assertion"]) != 1 {
		return "", false
	}
	if form.Get("grant_type") != GrantType || form.Get("assertion") == "" {
		return "", false
	}
	return form.Get("assertion"), true
}

// verifyAssertion reports whether a is a compact JWS with a header of exactly alg = RS256,
// typ = JWT, kid = KeyID, a payload of exactly iss = ClientEmail, scope = Scope,
// aud = TokenURL(), iat and exp with exp > iat and exp - iat at most one hour, and an RS256
// signature that the current public key verifies.
func (s *Server) verifyAssertion(a string) bool {
	parts := strings.Split(a, ".")
	if len(parts) != 3 {
		return false
	}
	head, err1 := base64.RawURLEncoding.DecodeString(parts[0])
	payload, err2 := base64.RawURLEncoding.DecodeString(parts[1])
	sig, err3 := base64.RawURLEncoding.DecodeString(parts[2])
	if err1 != nil || err2 != nil || err3 != nil {
		return false
	}
	var h map[string]any
	if json.Unmarshal(head, &h) != nil || len(h) != 3 || h["alg"] != "RS256" || h["typ"] != "JWT" || h["kid"] != s.opts.KeyID {
		return false
	}
	var p map[string]any
	dec := json.NewDecoder(bytes.NewReader(payload))
	dec.UseNumber()
	if dec.Decode(&p) != nil || len(p) != 5 || p["iss"] != s.opts.ClientEmail || p["scope"] != Scope || p["aud"] != s.TokenURL() {
		return false
	}
	iat, ok1 := intClaim(p["iat"])
	exp, ok2 := intClaim(p["exp"])
	if !ok1 || !ok2 || exp <= iat || exp-iat > maxAssertionLifetime {
		return false
	}
	s.mu.Lock()
	pub := s.pub
	s.mu.Unlock()
	if pub == nil {
		return false
	}
	sum := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	return rsa.VerifyPKCS1v15(pub, crypto.SHA256, sum[:], sig) == nil // NOSONAR: RS256 signature check of the assertion, not encryption (as kernel/jose, M1-41/17)
}

// intClaim returns v as an integer when it is a JSON integer.
func intClaim(v any) (int64, bool) {
	n, ok := v.(json.Number)
	if !ok {
		return 0, false
	}
	i, err := strconv.ParseInt(n.String(), 10, 64)
	return i, err == nil
}

// handleSend answers a send request like the FCM HTTP v1 API.
func (s *Server) handleSend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if r.URL.Path != "/v1/projects/"+s.opts.ProjectID+"/messages:send" {
		s.answer(w, Response{Status: http.StatusNotFound, RPCStatus: "NOT_FOUND"})
		return
	}
	bearer, ok := cutBearer(r.Header.Get("Authorization"))
	s.mu.Lock()
	known := ok && s.tokens[bearer]
	s.mu.Unlock()
	if !known {
		s.answer(w, Response{Status: http.StatusUnauthorized, RPCStatus: "UNAUTHENTICATED"})
		return
	}
	body, _ := io.ReadAll(r.Body)
	var envelope struct {
		Message json.RawMessage `json:"message"`
	}
	var msg map[string]json.RawMessage
	if json.Unmarshal(body, &envelope) != nil || json.Unmarshal(envelope.Message, &msg) != nil || msg == nil {
		s.answer(w, Response{Status: http.StatusBadRequest, RPCStatus: "INVALID_ARGUMENT"})
		return
	}
	var token string
	_ = json.Unmarshal(msg["token"], &token)
	s.mu.Lock()
	s.reqs = append(s.reqs, Request{Proto: r.Proto, Method: r.Method, Path: r.URL.Path, Header: r.Header.Clone(), Body: body, Bearer: bearer})
	var resp Response
	if rs := s.scripts[token]; len(rs) > 0 {
		resp, s.scripts[token] = rs[0], rs[1:]
	}
	s.mu.Unlock()
	s.answer(w, resp)
}

// cutBearer returns the token of an Authorization header "Bearer <token>" (scheme without regard
// to ASCII case).
func cutBearer(h string) (string, bool) {
	const prefix = "Bearer "
	if len(h) <= len(prefix) || !strings.EqualFold(h[:len(prefix)], prefix) {
		return "", false
	}
	return h[len(prefix):], true
}

// fcmError is the error body of the FCM HTTP v1 API (13 7.2).
type fcmError struct {
	Error struct {
		Code    int    `json:"code"`
		Status  string `json:"status"`
		Message string `json:"message"`
		Details []any  `json:"details,omitempty"`
	} `json:"error"`
}

// answer writes resp: 200 with a message name, else the FCM error body.
func (s *Server) answer(w http.ResponseWriter, resp Response) {
	if resp.RetryAfter != "" {
		w.Header().Set("Retry-After", resp.RetryAfter)
	}
	status := resp.Status
	if status == 0 {
		status = http.StatusOK
	}
	if status == http.StatusOK {
		writeJSON(w, status, map[string]string{"name": "projects/" + s.opts.ProjectID + "/messages/" + randomHex(8)})
		return
	}
	var e fcmError
	e.Error.Code, e.Error.Status, e.Error.Message = status, resp.RPCStatus, s.opts.ErrorMessage
	if resp.ErrorCode != "" {
		e.Error.Details = append(e.Error.Details, map[string]string{
			detailType:  "type.googleapis.com/google.firebase.fcm.v1.FcmError",
			"errorCode": resp.ErrorCode,
		})
	}
	if resp.TokenField {
		e.Error.Details = append(e.Error.Details, map[string]any{
			detailType: "type.googleapis.com/google.rpc.BadRequest",
			"fieldViolations": []map[string]string{{
				"field":       "message.token",
				"description": "The registration token is not a valid FCM registration token",
			}},
		})
	}
	if resp.OtherField {
		e.Error.Details = append(e.Error.Details, map[string]any{
			detailType: "type.googleapis.com/google.rpc.BadRequest",
			"fieldViolations": []map[string]string{{
				"field":       "message.android.ttl",
				"description": "Invalid value",
			}},
		})
	}
	writeJSON(w, status, e)
}

// oauthError writes the OAuth error body {"error": code} with status, plus error_description
// when desc is not empty.
func oauthError(w http.ResponseWriter, status int, code, desc string) {
	body := map[string]string{"error": code}
	if desc != "" {
		body["error_description"] = desc
	}
	writeJSON(w, status, body)
}

// writeJSON writes v as JSON with status.
func writeJSON(w http.ResponseWriter, status int, v any) {
	b, _ := json.Marshal(v)
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(status)
	_, _ = w.Write(b)
}

// randomHex returns n random bytes in hex.
func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
