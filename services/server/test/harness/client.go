// SPDX-License-Identifier: Apache-2.0

package harness

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"go.yaml.in/yaml/v3"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/jose"
)

// apiPrefix is the server base of every operation (06 "Basic rules", servers of openapi.yaml).
const apiPrefix = "/api/v1"

// Request is one API call of a Client.
type Request struct {
	// Method and Path name the operation; Path is relative to /api/v1 and may carry a query.
	Method, Path string
	// Body is marshalled as JSON when not nil; a []byte or an io.Reader is sent as is.
	Body any
	// Header are extra request headers (If-Match, a fixed Idempotency-Key).
	Header http.Header
	// ContentLength, when positive, is the declared length of a streamed Body; otherwise a stream
	// is sent chunked.
	ContentLength int64
}

// Response is the answer to a Request.
type Response struct {
	// Status is the HTTP status.
	Status int
	// Header are the response headers.
	Header http.Header
	// Body is the full response body.
	Body []byte
	// Elapsed is the time from sending to the end of the body, of the last attempt.
	Elapsed time.Duration
}

// JSON decodes the body into v and fails tb when that does not work.
func (r Response) JSON(tb TB, v any) {
	tb.Helper()
	if err := json.Unmarshal(r.Body, v); err != nil {
		tb.Fatalf("harness: decode %d %s: %v", r.Status, r.Body, err)
	}
}

// TB is the part of testing.TB the response helpers need.
type TB interface {
	Helper()
	Fatalf(format string, args ...any)
}

// Client is a signed-in principal of a harness server: a person with a device key, a DPoP-bound
// token pair and a step-up key. It is safe for concurrent use.
type Client struct {
	// PrincipalID and DeviceID name the person and its device.
	PrincipalID, DeviceID string
	// SessionID is the session of the token pair.
	SessionID string
	// Observer, when set, sees every request and its final response.
	Observer func(Request, Response)
	// ClientIP, when set, is sent as X-Forwarded-For on every request and on the gateway upgrade,
	// so the server (which trusts the loopback peer as its reverse proxy, server.trusted_proxies)
	// counts the client by that address, as a deployment behind its proxy sees distinct clients.
	// Load tests give each simulated client its own documentation address.
	ClientIP string

	publicURL string
	http      *http.Client
	key       *deviceKey
	stepKey   *deviceKey
	// apiToken marks the client of an agent: access is an API token, there is no device.
	apiToken bool
	// proofs and tokens are Options.ProofObserver and Options.TokenObserver of the server; nil
	// sees nothing.
	proofs func(string)
	tokens func(access, refresh string)
	// tb is the test of the server: gateway connections report their frame violations to it (G9).
	tb testing.TB

	mu      sync.Mutex
	access  string
	refresh string
	nonce   string
}

// Key returns the private DPoP key of the client as a JWK (credential broker of G7).
func (c *Client) Key() json.RawMessage { return c.key.privateJWK() }

// Proof returns a DPoP proof of the device key of the client for htm and htu with nonce, bound to
// its current access token (RFC 9449): the identify of the device stream of the terminal (M2-44)
// sends it with the access token.
func (c *Client) Proof(htm, htu, nonce string) string {
	access, _ := c.Tokens()
	return c.key.proof(htm, htu, nonce, access, time.Now())
}

// Tokens returns the current access and refresh token.
func (c *Client) Tokens() (access, refresh string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.access, c.refresh
}

// Do sends r with a fresh DPoP proof (RFC 9449) built with the newest DPoP-Nonce. It retries once
// on use_dpop_nonce and refreshes the token pair once on token_expired. POST and PATCH carry a fresh
// UUIDv7 Idempotency-Key unless r sets one or the operation is exempt (06 "Idempotency"). It never
// fails the test; a transport error is a Response with status 0 and the error as body.
func (c *Client) Do(ctx context.Context, r Request) Response {
	if (r.Method == http.MethodPost || r.Method == http.MethodPatch) && r.Header.Get("Idempotency-Key") == "" && !exempt(r.Method, r.Path) {
		r.Header = r.Header.Clone()
		if r.Header == nil {
			r.Header = http.Header{}
		}
		r.Header.Set("Idempotency-Key", ids.New().String())
	}
	resp := c.send(ctx, r, true)
	if resp.Status == http.StatusUnauthorized && problemCode(resp.Body) == "token_expired" {
		if c.renew(ctx) {
			resp = c.send(ctx, r, true)
		}
	}
	if c.Observer != nil {
		c.Observer(r, resp)
	}
	return resp
}

// DoAnonymous sends r like Do but without the access token: the DPoP proof of the client's key
// alone, with one retry on use_dpop_nonce (refreshToken, a replayed refresh token, the login and
// pairing requests of a device). The Observer sees it.
func (c *Client) DoAnonymous(ctx context.Context, r Request) Response {
	resp := c.send(ctx, r, false)
	if c.Observer != nil {
		c.Observer(r, resp)
	}
	return resp
}

// send sends r with the access token (when auth) and repeats it once on use_dpop_nonce.
func (c *Client) send(ctx context.Context, r Request, auth bool) Response {
	resp := c.attempt(ctx, r, auth)
	if resp.Status == http.StatusUnauthorized && problemCode(resp.Body) == "use_dpop_nonce" {
		resp = c.attempt(ctx, r, auth)
	}
	return resp
}

// attempt sends r once and keeps the newest DPoP-Nonce.
func (c *Client) attempt(ctx context.Context, r Request, auth bool) Response {
	var body io.Reader
	switch b := r.Body.(type) {
	case nil:
	case []byte:
		body = bytes.NewReader(b)
	case io.Reader:
		// A stream is sent as it is, once: a retry after use_dpop_nonce finds it consumed, so a
		// caller that streams sends a request with the nonce first.
		body = b
	default:
		raw, err := json.Marshal(b)
		if err != nil {
			return Response{Body: []byte(err.Error())}
		}
		body = bytes.NewReader(raw)
	}
	target := c.publicURL + apiPrefix + r.Path
	req, err := http.NewRequestWithContext(ctx, r.Method, target, body)
	if err != nil {
		return Response{Body: []byte(err.Error())}
	}
	for name, vs := range r.Header {
		req.Header[name] = vs
	}
	if r.ContentLength > 0 {
		req.ContentLength = r.ContentLength
	}
	if c.ClientIP != "" {
		req.Header.Set("X-Forwarded-For", c.ClientIP)
	}
	if body != nil && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}
	c.mu.Lock()
	access, nonce := c.access, c.nonce
	c.mu.Unlock()
	if !auth {
		access = ""
	}
	if access != "" {
		req.Header.Set("Authorization", "DPoP "+access)
	}
	proof := c.key.proof(r.Method, target, nonce, access, time.Now())
	if c.proofs != nil {
		c.proofs(proof)
	}
	req.Header.Set("DPoP", proof)
	start := time.Now()
	resp, err := c.http.Do(req)
	if err != nil {
		return Response{Body: []byte(err.Error())}
	}
	defer func() { _ = resp.Body.Close() }()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return Response{Body: []byte(err.Error())}
	}
	if n := resp.Header.Get("DPoP-Nonce"); n != "" {
		c.mu.Lock()
		c.nonce = n
		c.mu.Unlock()
	}
	return Response{Status: resp.StatusCode, Header: resp.Header, Body: b, Elapsed: time.Since(start)}
}

// renew rotates the token pair with refreshToken and reports whether it worked.
func (c *Client) renew(ctx context.Context) bool {
	_, refresh := c.Tokens()
	resp := c.send(ctx, Request{Method: http.MethodPost, Path: "/auth/token/refresh", Body: map[string]any{"refresh_token": refresh}}, false)
	if resp.Status != http.StatusOK {
		return false
	}
	var pair tokenPair
	if json.Unmarshal(resp.Body, &pair) != nil || pair.AccessToken == "" {
		return false
	}
	c.adopt(pair)
	return true
}

// tokenPair is the part of TokenResponse the client keeps.
type tokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	Session      struct {
		ID string `json:"id"`
	} `json:"session"`
	Device struct {
		ID          string `json:"id"`
		PrincipalID string `json:"principal_id"`
	} `json:"device"`
}

// Adopt takes over the token pair of a TokenResponse body (a login, pairing or enrollment the
// test drove itself with a client of Server.NewDevice) and sets PrincipalID and SessionID from
// it. The error reports a body without access token.
func (c *Client) Adopt(body []byte) error {
	var pair tokenPair
	if err := json.Unmarshal(body, &pair); err != nil {
		return err
	}
	if pair.AccessToken == "" {
		return fmt.Errorf("harness: no access_token in %d bytes", len(body))
	}
	c.adopt(pair)
	if pair.Device.PrincipalID != "" {
		c.PrincipalID = pair.Device.PrincipalID
	}
	return nil
}

// Registration returns the DeviceRegistration of the client (its DPoP and step-up keys) with
// name, for the device member of login, pairing and enrollment requests.
func (c *Client) Registration(name string) map[string]any { return c.registration(name) }

// adopt takes over a token pair.
func (c *Client) adopt(p tokenPair) {
	if c.tokens != nil {
		c.tokens(p.AccessToken, p.RefreshToken)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.access, c.refresh = p.AccessToken, p.RefreshToken
	if p.Session.ID != "" {
		c.SessionID = p.Session.ID
	}
}

// StepUp performs a fresh step-up with the step-up device key of the session
// (createStepUpChallenge, completeStepUpWithDeviceKey; 08 "Step-up flow") and returns an error
// when the server refuses.
func (c *Client) StepUp(ctx context.Context) error {
	ch := c.Do(ctx, Request{Method: http.MethodPost, Path: "/auth/step-up/challenge"})
	if ch.Status != http.StatusOK {
		return fmt.Errorf("harness: step-up challenge: %d %s", ch.Status, ch.Body)
	}
	var challenge struct {
		ID        string `json:"challenge_id"`
		Challenge string `json:"challenge"`
	}
	if err := json.Unmarshal(ch.Body, &challenge); err != nil {
		return fmt.Errorf("harness: step-up challenge: %w", err)
	}
	payload, err := json.Marshal(map[string]any{"challenge": challenge.Challenge, "sid": c.SessionID, "iat": time.Now().Unix()})
	if err != nil {
		return err
	}
	sig, err := jose.SignCompact(jose.ES256Signer{Key: c.stepKey.priv}, jose.Header{}, payload)
	if err != nil {
		return err
	}
	done := c.Do(ctx, Request{Method: http.MethodPost, Path: "/auth/step-up/device-key", Body: map[string]any{"challenge_id": challenge.ID, "signature": sig}})
	if done.Status != http.StatusOK {
		return fmt.Errorf("harness: step-up: %d %s", done.Status, done.Body)
	}
	return nil
}

// problemCode returns the member code of a problem body, or "".
func problemCode(body []byte) string {
	var p struct {
		Code string `json:"code"`
	}
	_ = json.Unmarshal(body, &p)
	return p.Code
}

// deviceKey is an ES256 key of a test device.
type deviceKey struct {
	priv *ecdsa.PrivateKey
	jwk  json.RawMessage
}

// newDeviceKey returns a fresh P-256 key. It panics when the system has no randomness.
func newDeviceKey() *deviceKey {
	k, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		panic(err)
	}
	jwk, err := jose.JWKFromPublicKey(&k.PublicKey)
	if err != nil {
		panic(err)
	}
	raw, err := json.Marshal(jwk)
	if err != nil {
		panic(err)
	}
	return &deviceKey{priv: k, jwk: raw}
}

// privateJWK returns the key as a private JWK (RFC 7518 6.2.2) for the Python side of G7.
func (k *deviceKey) privateJWK() json.RawMessage {
	b := func(n []byte) string { return base64.RawURLEncoding.EncodeToString(n) }
	ecdh, err := k.priv.Bytes()
	if err != nil {
		panic(err)
	}
	pub, err := k.priv.PublicKey.Bytes() // 0x04 || X || Y
	if err != nil {
		panic(err)
	}
	raw, err := json.Marshal(map[string]string{"kty": "EC", "crv": "P-256", "x": b(pub[1:33]), "y": b(pub[33:]), "d": b(ecdh)})
	if err != nil {
		panic(err)
	}
	return raw
}

// proof returns a DPoP proof for htm and htu (query and fragment removed) at now, with nonce and
// ath of accessToken when they are not empty.
func (k *deviceKey) proof(htm, htu, nonce, accessToken string, now time.Time) string {
	if u, err := url.Parse(htu); err == nil {
		u.RawQuery, u.Fragment, u.RawFragment = "", "", ""
		htu = u.String()
	}
	claims := map[string]any{"jti": ids.New().String(), "htm": htm, "htu": htu, "iat": json.Number(strconv.FormatInt(now.Unix(), 10))}
	if nonce != "" {
		claims["nonce"] = nonce
	}
	if accessToken != "" {
		sum := sha256.Sum256([]byte(accessToken))
		claims["ath"] = base64.RawURLEncoding.EncodeToString(sum[:])
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		panic(err)
	}
	s, err := jose.SignCompact(jose.ES256Signer{Key: k.priv}, jose.Header{Typ: "dpop+jwt", JWK: k.jwk}, payload)
	if err != nil {
		panic(err)
	}
	return s
}

// exemptOps maps "METHOD /path-template" of every operation with x-idempotency-exempt in the
// contract; matching uses a ServeMux, so templates match like the server routes.
var exemptOps = sync.OnceValue(func() *http.ServeMux {
	var doc struct {
		Paths map[string]map[string]struct {
			Exempt string `yaml:"x-idempotency-exempt"`
		} `yaml:"paths"`
	}
	if err := yaml.Unmarshal(rest.Spec, &doc); err != nil {
		panic(err)
	}
	mux := http.NewServeMux()
	for path, item := range doc.Paths {
		for method, op := range item {
			if op.Exempt != "" {
				mux.Handle(strings.ToUpper(method)+" "+path, http.NotFoundHandler())
			}
		}
	}
	return mux
})

// exempt reports whether the operation of method and path is exempt from Idempotency-Key.
func exempt(method, path string) bool {
	p, _, _ := strings.Cut(path, "?")
	req := &http.Request{Method: method, URL: &url.URL{Path: p}}
	_, pattern := exemptOps().Handler(req)
	return pattern != ""
}
