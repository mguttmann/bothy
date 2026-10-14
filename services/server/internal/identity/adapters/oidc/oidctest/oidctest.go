// SPDX-License-Identifier: Apache-2.0

// Package oidctest is the fake identity provider of the OIDC tests (brief M1-41 "Fake IdP", the
// test double of docs/PLAN.md section 5): a TLS server on 127.0.0.1 with discovery, /authorize,
// /token and /jwks. /authorize records the server's PKCE challenge, state and nonce and redirects
// with a code; /token checks grant type, redirect URI, PKCE S256 and the client authentication
// (Basic secret or a private_key_jwt assertion against a registered public key with aud, exp and
// a unique jti) and issues an ID token signed by the provider key. Knobs change header and claims
// of the next ID tokens, rotate the key, omit the iss parameter, answer with an error, switch to
// the Microsoft Entra ID issuer template, and set a group claim or its Entra overage (M1-42).
// Only tests import it.
//
// Example:
//
//	idp := oidctest.New(t, oidctest.Options{ClientID: "chat", Secret: "EXAMPLE-secret"})
//	idp.NextLogin(oidctest.Login{Subject: "alex-sub"})
//	location := idp.Authorize(t, authorizationURL) // the redirect to the server's callback
package oidctest

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/jose"
)

// keyError is the error member and parameter of OAuth answers (RFC 6749 sections 4.1.2.1 and 5.2).
const keyError = "error"

// TenantTemplate is the issuer path of Entra mode; the issuer of a tenant replaces the
// placeholder with its tid.
const TenantTemplate = "/{tenantid}/v2.0"

// Options configure the fake provider.
type Options struct {
	// ClientID is the client id the provider knows.
	ClientID string
	// Secret is the client secret of client_secret_basic; "" refuses Basic authentication.
	Secret string
	// AssertionKey is the public key of private_key_jwt; nil refuses assertions.
	AssertionKey any
	// Entra switches to the issuer template https://127.0.0.1:<port>/{tenantid}/v2.0 with its
	// discovery document under /organizations/v2.0.
	Entra bool
	// SendIss makes /authorize add the iss parameter and discovery announce it (RFC 9207).
	SendIss bool
	// Alg is the algorithm of the ID tokens: RS256 (default), PS256 or ES256.
	Alg string
	// Now is the time of the ID tokens; nil means time.Now.
	Now func() time.Time
	// JWKSURI overrides jwks_uri in the discovery document (private target tests).
	JWKSURI string
}

// Login is the person who signs in at the next /authorize.
type Login struct {
	Subject, Name, PreferredUsername, Email, TenantID string
}

// grant is an issued authorization code.
type grant struct {
	login                       Login
	challenge, method, redirect string
	nonce                       string
	used                        bool
}

// Assertion is a client assertion the token endpoint accepted: its header and claims.
type Assertion struct {
	Header map[string]any
	Claims map[string]any
}

// IDP is a running fake provider. It is safe for concurrent use.
type IDP struct {
	t    *testing.T
	srv  *httptest.Server
	opts Options

	mu         sync.Mutex
	key        any
	kid        string
	oldKeys    map[string]any
	next       Login
	grants     map[string]*grant
	jtis       map[string]bool
	counts     map[string]int
	header     map[string]any
	claims     map[string]any
	drop       []string
	fail       string
	omitIss    bool
	seenVerif  []string
	seenMethod []string
	assertions []Assertion
	basic      int
}

// New starts the provider for the life of t.
func New(t *testing.T, opts Options) *IDP {
	t.Helper()
	if opts.Alg == "" {
		opts.Alg = jose.AlgRS256
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	p := &IDP{
		t: t, opts: opts, oldKeys: map[string]any{}, grants: map[string]*grant{}, jtis: map[string]bool{}, counts: map[string]int{},
		header: map[string]any{}, claims: map[string]any{},
	}
	p.key, p.kid = newKey(t, opts.Alg), "k1"
	p.srv = httptest.NewTLSServer(http.HandlerFunc(p.serve))
	t.Cleanup(p.srv.Close)
	return p
}

// serve dispatches the four endpoints of the fake provider (the project router is for the
// server's own operations only).
func (p *IDP) serve(w http.ResponseWriter, r *http.Request) {
	switch r.Method + " " + r.URL.Path {
	case "GET /.well-known/openid-configuration", "GET /organizations/v2.0/.well-known/openid-configuration":
		p.discovery(w, r)
	case "GET /authorize":
		p.authorize(w, r)
	case "POST /token":
		p.token(w, r)
	case "GET /jwks":
		p.jwks(w, r)
	default:
		http.NotFound(w, r)
	}
}

// newKey creates a signing key for alg.
func newKey(t *testing.T, alg string) any {
	t.Helper()
	if alg == jose.AlgES256 {
		k, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		return k
	}
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	return k
}

// URL is the origin of the provider, https://127.0.0.1:<port>.
func (p *IDP) URL() string { return p.srv.URL }

// Port is the port of the provider.
func (p *IDP) Port() string {
	u, _ := url.Parse(p.srv.URL)
	return u.Port()
}

// Issuer is the configured issuer: the origin, or the tenant template in Entra mode.
func (p *IDP) Issuer() string {
	if p.opts.Entra {
		return p.srv.URL + TenantTemplate
	}
	return p.srv.URL
}

// TenantIssuer is the issuer of tenant tid in Entra mode.
func (p *IDP) TenantIssuer(tid string) string {
	return strings.Replace(p.Issuer(), "{tenantid}", tid, 1)
}

// RootCAs is the pool with the provider's certificate.
func (p *IDP) RootCAs() *x509.CertPool {
	pool := x509.NewCertPool()
	pool.AddCert(p.srv.Certificate())
	return pool
}

// Browser is an HTTP client that trusts the provider and does not follow redirects.
func (p *IDP) Browser() *http.Client {
	c := p.srv.Client()
	c.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return c
}

// NextLogin sets the person of the following /authorize requests.
func (p *IDP) NextLogin(l Login) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.next = l
}

// SetHeader overrides a member of the ID token header (alg "none" or "HS256" included; then the
// token is built unsigned or with a fake MAC as the header says).
func (p *IDP) SetHeader(name string, v any) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.header[name] = v
}

// SetClaim overrides a claim of the next ID tokens.
func (p *IDP) SetClaim(name string, v any) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.claims[name] = v
}

// SetGroups puts the group claim name with values into the next ID tokens (M1-42); Reset or
// DropClaim(name) takes it out again.
func (p *IDP) SetGroups(name string, values ...string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.claims[name] = append([]string{}, values...)
	delete(p.claims, "_claim_names")
	delete(p.claims, "_claim_sources")
}

// SetGroupsOverage replaces the group claim name of the next ID tokens with the Microsoft Entra ID
// overage: name in _claim_names, pointing to a source in _claim_sources (08 section 2.3).
func (p *IDP) SetGroupsOverage(name string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.claims, name)
	p.claims["_claim_names"] = map[string]any{name: "src1"}
	p.claims["_claim_sources"] = map[string]any{"src1": map[string]any{"endpoint": "https://graph.example.com/v1.0/users/me/getMemberObjects"}}
}

// DropClaim removes a claim from the next ID tokens.
func (p *IDP) DropClaim(name string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.drop = append(p.drop, name)
}

// Reset clears every header and claim override.
func (p *IDP) Reset() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.header, p.claims, p.drop, p.fail, p.omitIss = map[string]any{}, map[string]any{}, nil, "", false
}

// FailAuthorize makes /authorize redirect with error=code.
func (p *IDP) FailAuthorize(code string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.fail = code
}

// OmitIss makes /authorize leave out the iss parameter even when discovery announces it.
func (p *IDP) OmitIss() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.omitIss = true
}

// RotateKey signs the next ID tokens with a new key under kid; the JWKS serves the new key and
// drops the old ones unless keepOld.
func (p *IDP) RotateKey(kid string, keepOld bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if keepOld {
		p.oldKeys[p.kid] = p.key
	} else {
		p.oldKeys = map[string]any{}
	}
	p.key, p.kid = newKey(p.t, p.opts.Alg), kid
}

// Count is the number of requests one endpoint ("discovery", "authorize", "token", "jwks") got.
func (p *IDP) Count(endpoint string) int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.counts[endpoint]
}

// Seen returns the code_challenge_method of every /authorize and the code_verifier of every
// /token request.
func (p *IDP) Seen() (methods, verifiers []string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string(nil), p.seenMethod...), append([]string(nil), p.seenVerif...)
}

// Assertions returns the accepted client assertions.
func (p *IDP) Assertions() []Assertion {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]Assertion(nil), p.assertions...)
}

// BasicLogins is the number of token requests accepted with client_secret_basic.
func (p *IDP) BasicLogins() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.basic
}

// Authorize plays the system browser: it opens authorizationURL at the provider and returns the
// Location of the provider's redirect to the server.
func (p *IDP) Authorize(t *testing.T, authorizationURL string) *url.URL {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, authorizationURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := p.Browser().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("authorize answered %d", resp.StatusCode)
	}
	loc, err := url.Parse(resp.Header.Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	return loc
}

func (p *IDP) count(endpoint string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.counts[endpoint]++
}

func (p *IDP) discovery(w http.ResponseWriter, _ *http.Request) {
	p.count("discovery")
	jwksURI := p.opts.JWKSURI
	if jwksURI == "" {
		jwksURI = p.srv.URL + "/jwks"
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"issuer": p.Issuer(), "authorization_endpoint": p.srv.URL + "/authorize", "token_endpoint": p.srv.URL + "/token",
		"jwks_uri": jwksURI, "response_types_supported": []string{"code"}, "code_challenge_methods_supported": []string{"S256"},
		"id_token_signing_alg_values_supported":          []string{"RS256", "PS256", "ES256"},
		"authorization_response_iss_parameter_supported": p.opts.SendIss,
	})
}

func (p *IDP) authorize(w http.ResponseWriter, r *http.Request) {
	p.count("authorize")
	q := r.URL.Query()
	p.mu.Lock()
	p.seenMethod = append(p.seenMethod, q.Get("code_challenge_method"))
	redirect, err := url.Parse(q.Get("redirect_uri"))
	if err != nil || q.Get("client_id") != p.opts.ClientID || q.Get("response_type") != "code" {
		p.mu.Unlock()
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	out := url.Values{"state": {q.Get("state")}}
	if p.fail != "" {
		out.Set(keyError, p.fail)
	} else {
		code := randomText()
		p.grants[code] = &grant{login: p.next, challenge: q.Get("code_challenge"), method: q.Get("code_challenge_method"),
			redirect: q.Get("redirect_uri"), nonce: q.Get("nonce")}
		out.Set("code", code)
	}
	if p.opts.SendIss && !p.omitIss {
		out.Set("iss", p.issuerOf(p.next))
	}
	p.mu.Unlock()
	redirect.RawQuery = out.Encode()
	// The fake provider redirects to the redirect_uri of the request on purpose: it is the
	// server under test.
	http.Redirect(w, r, redirect.String(), http.StatusFound) //nolint:gosec // G710: test double, the target is the server under test

}

// issuerOf is the issuer of a login: the tenant issuer in Entra mode.
func (p *IDP) issuerOf(l Login) string {
	if p.opts.Entra {
		return p.TenantIssuer(l.TenantID)
	}
	return p.srv.URL
}

func (p *IDP) token(w http.ResponseWriter, r *http.Request) {
	p.count("token")
	if err := r.ParseForm(); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{keyError: "invalid_request"})
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	f := r.PostForm
	p.seenVerif = append(p.seenVerif, f.Get("code_verifier"))
	g := p.grants[f.Get("code")]
	switch {
	case f.Get("grant_type") != "authorization_code" || g == nil || g.used:
		writeJSON(w, http.StatusBadRequest, map[string]string{keyError: "invalid_grant"})
		return
	case g.redirect != f.Get("redirect_uri") || g.method != "S256" || s256(f.Get("code_verifier")) != g.challenge:
		writeJSON(w, http.StatusBadRequest, map[string]string{keyError: "invalid_grant"})
		return
	}
	if !p.clientOK(r, f) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{keyError: "invalid_client"})
		return
	}
	g.used = true
	writeJSON(w, http.StatusOK, map[string]any{
		"access_token": "provider-access-" + randomText(), "refresh_token": "provider-refresh-" + randomText(), "token_type": "Bearer",
		"id_token": p.idToken(g),
	})
}

// clientOK checks the client authentication of a token request; the caller holds mu.
func (p *IDP) clientOK(r *http.Request, f url.Values) bool {
	if id, secret, ok := r.BasicAuth(); ok {
		uid, err1 := url.QueryUnescape(id)
		usec, err2 := url.QueryUnescape(secret)
		if p.opts.Secret == "" || err1 != nil || err2 != nil || uid != p.opts.ClientID || usec != p.opts.Secret {
			return false
		}
		p.basic++
		return true
	}
	if p.opts.AssertionKey == nil || f.Get("client_assertion_type") != "urn:ietf:params:oauth:client-assertion-type:jwt-bearer" {
		return false
	}
	j, err := jose.ParseCompactJWS(f.Get("client_assertion"), []string{jose.AlgPS256, jose.AlgRS256, jose.AlgES256}, "JWT")
	if err != nil {
		return false
	}
	k, err := jose.JWKFromPublicKey(p.opts.AssertionKey)
	if err != nil || j.Verify(k) != nil {
		return false
	}
	var claims map[string]any
	if json.Unmarshal(j.Payload, &claims) != nil {
		return false
	}
	exp, _ := claims["exp"].(float64)
	jti, _ := claims["jti"].(string)
	if claims["iss"] != p.opts.ClientID || claims["sub"] != p.opts.ClientID || claims["aud"] != p.srv.URL+"/token" ||
		int64(exp) <= p.opts.Now().Unix() || jti == "" || p.jtis[jti] {
		return false
	}
	p.jtis[jti] = true
	head := map[string]any{}
	parts := strings.Split(f.Get("client_assertion"), ".")
	if b, err := base64.RawURLEncoding.DecodeString(parts[0]); err == nil {
		_ = json.Unmarshal(b, &head)
	}
	p.assertions = append(p.assertions, Assertion{Header: head, Claims: claims})
	return true
}

// idToken builds the ID token of g with the overrides; the caller holds mu.
func (p *IDP) idToken(g *grant) string {
	now := p.opts.Now()
	claims := map[string]any{
		"iss": p.issuerOf(g.login), "sub": g.login.Subject, "aud": p.opts.ClientID, "exp": now.Add(5 * time.Minute).Unix(),
		"iat": now.Unix(), "nonce": g.nonce, "auth_time": now.Unix(), "amr": []string{"pwd"},
	}
	for name, v := range map[string]string{"name": g.login.Name, "preferred_username": g.login.PreferredUsername, "email": g.login.Email,
		"tid": g.login.TenantID} {
		if v != "" {
			claims[name] = v
		}
	}
	maps.Copy(claims, p.claims)
	for _, d := range p.drop {
		delete(claims, d)
	}
	payload, _ := json.Marshal(claims)
	head := map[string]any{"alg": p.opts.Alg, "typ": "JWT", "kid": p.kid}
	maps.Copy(head, p.header)
	hb, _ := json.Marshal(head)
	input := seg(hb) + "." + seg(payload)
	switch head["alg"] {
	case "none":
		return input + "."
	case "HS256":
		return input + "." + seg([]byte("fake-mac-EXAMPLE"))
	}
	var signer jose.Signer
	switch k := p.key.(type) {
	case *ecdsa.PrivateKey:
		signer = jose.ES256Signer{Key: k}
	case *rsa.PrivateKey:
		if head["alg"] == jose.AlgPS256 {
			signer = jose.PS256Signer{Key: k}
		} else {
			signer = jose.RS256Signer{Key: k}
		}
	}
	sig, err := signer.Sign([]byte(input))
	if err != nil {
		p.t.Error(err)
	}
	return input + "." + seg(sig)
}

func (p *IDP) jwks(w http.ResponseWriter, _ *http.Request) {
	p.count("jwks")
	p.mu.Lock()
	keys := []map[string]any{publicJWK(p.t, p.key, p.kid)}
	for kid, k := range p.oldKeys {
		keys = append(keys, publicJWK(p.t, k, kid))
	}
	p.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]any{"keys": keys})
}

// publicJWK is the JWKS entry of the private key k.
func publicJWK(t *testing.T, k any, kid string) map[string]any {
	var pub any
	switch key := k.(type) {
	case *rsa.PrivateKey:
		pub = &key.PublicKey
	case *ecdsa.PrivateKey:
		pub = &key.PublicKey
	}
	j, err := jose.JWKFromPublicKey(pub)
	if err != nil {
		t.Error(err)
	}
	b, _ := json.Marshal(j)
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	m["kid"], m["use"] = kid, "sig"
	return m
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func seg(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

func s256(v string) string {
	sum := sha256.Sum256([]byte(v))
	return seg(sum[:])
}

func randomText() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x", b)
}
