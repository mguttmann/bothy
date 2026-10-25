// SPDX-License-Identifier: Apache-2.0

package smoke

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/OWNER/bothy/services/server/api/rest"
)

// nonceServer answers with the next entry of script per request and records the claims of each
// DPoP proof and the request headers.
type nonceServer struct {
	mu      sync.Mutex
	script  []func(w http.ResponseWriter)
	proofs  []map[string]any
	headers []http.Header
}

func (s *nonceServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var claims map[string]any
	if p := r.Header.Get("DPoP"); p != "" {
		claims = claimsOf(p)
	}
	s.proofs = append(s.proofs, claims)
	s.headers = append(s.headers, r.Header.Clone())
	i := len(s.proofs) - 1
	if i >= len(s.script) {
		w.WriteHeader(http.StatusTeapot)
		return
	}
	s.script[i](w)
}

// useNonce answers 401 use_dpop_nonce with nonce as the new DPoP-Nonce.
func useNonce(nonce string) func(w http.ResponseWriter) {
	const status, code = http.StatusUnauthorized, "use_dpop_nonce"
	return func(w http.ResponseWriter) {
		if nonce != "" {
			w.Header().Set("DPoP-Nonce", nonce)
		}
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(map[string]any{"type": "/api/v1/problems/" + code, "title": code, "status": status, "code": code,
			"correlation_id": "4bf92f35-77b3-4da6-a3ce-929d0e0e4736"})
	}
}

func okMe(nonce string) func(w http.ResponseWriter) {
	return func(w http.ResponseWriter) {
		if nonce != "" {
			w.Header().Set("DPoP-Nonce", nonce)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"is_owner":true,"permission_version":1,"principal":{"display_name":"Alex","handle":"alex",` +
			`"id":"0192f000-0000-7000-8000-000000000001","kind":"user"}}`))
	}
}

func newTestClient(t *testing.T, s *nonceServer) *HTTPClient {
	t.Helper()
	srv := httptest.NewServer(s)
	t.Cleanup(srv.Close)
	c, err := NewHTTPClient(srv.URL, "https://chat.example.com")
	if err != nil {
		t.Fatal(err)
	}
	c.SetCredentials(newTestKey(t), "tok")
	return c
}

// TestHTTPClientRetriesOnceOnUseDPoPNonce: a 401 use_dpop_nonce is repeated once with the nonce
// of its answer; the proof names the public URL, not the dialled address, and carries ath.
func TestHTTPClientRetriesOnceOnUseDPoPNonce(t *testing.T) {
	t.Parallel()
	s := &nonceServer{script: []func(http.ResponseWriter){useNonce("n-1"), okMe("")}}
	c := newTestClient(t, s)
	me, err := c.GetMe(t.Context())
	if err != nil || me.Principal.Handle != "alex" {
		t.Fatalf("GetMe = %+v, %v", me, err)
	}
	if len(s.proofs) != 2 {
		t.Fatalf("%d requests, want 2", len(s.proofs))
	}
	if _, ok := s.proofs[0]["nonce"]; ok {
		t.Error("first proof has a nonce before the server named one")
	}
	if s.proofs[1]["nonce"] != "n-1" || s.proofs[1]["htu"] != "https://chat.example.com/api/v1/me" || s.proofs[1]["ath"] == nil {
		t.Errorf("retry proof %v", s.proofs[1])
	}
	if s.proofs[0]["jti"] == s.proofs[1]["jti"] {
		t.Error("the retry reused the proof")
	}
	if got := s.headers[1].Get("Authorization"); got != "DPoP tok" {
		t.Errorf("Authorization %q", got)
	}
}

// TestHTTPClientStopsAfterSecondNonceError: a second use_dpop_nonce is the error of the call,
// with status and code and without a third request.
func TestHTTPClientStopsAfterSecondNonceError(t *testing.T) {
	t.Parallel()
	s := &nonceServer{script: []func(http.ResponseWriter){
		useNonce("n-1"),
		useNonce("n-2"),
		okMe(""),
	}}
	c := newTestClient(t, s)
	_, err := c.GetMe(t.Context())
	var se *StatusError
	if !errors.As(err, &se) || se.Status != http.StatusUnauthorized || se.Code != "use_dpop_nonce" {
		t.Fatalf("GetMe error %v, want a StatusError 401 use_dpop_nonce", err)
	}
	if len(s.proofs) != 2 {
		t.Errorf("%d requests, want 2", len(s.proofs))
	}
}

// TestHTTPClientTracksLatestNonce: the DPoP-Nonce of every answer replaces the previous one.
func TestHTTPClientTracksLatestNonce(t *testing.T) {
	t.Parallel()
	s := &nonceServer{script: []func(http.ResponseWriter){okMe("n-1"), okMe("n-2"), okMe("")}}
	c := newTestClient(t, s)
	for range 3 {
		if _, err := c.GetMe(t.Context()); err != nil {
			t.Fatal(err)
		}
	}
	if _, ok := s.proofs[0]["nonce"]; ok || s.proofs[1]["nonce"] != "n-1" || s.proofs[2]["nonce"] != "n-2" {
		t.Errorf("nonces %v %v %v", s.proofs[0]["nonce"], s.proofs[1]["nonce"], s.proofs[2]["nonce"])
	}
}

// TestHTTPClientIdempotencyKey: createSpace carries an Idempotency-Key, the same on the nonce
// retry; createMessage (exempt, client_msg_id) and anonymous calls without a token carry no
// Authorization.
func TestHTTPClientIdempotencyKey(t *testing.T) {
	t.Parallel()
	space := func(w http.ResponseWriter) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"created_at":"2026-10-19T00:00:00.000Z","id":"0192f000-0000-7000-8000-000000000002","name":"x","sort_key":"a","version":1}`))
	}
	s := &nonceServer{script: []func(http.ResponseWriter){useNonce("n-1"), space}}
	c := newTestClient(t, s)
	if _, err := c.CreateSpace(t.Context(), rest.SpaceCreate{Name: "x"}); err != nil {
		t.Fatal(err)
	}
	k0, k1 := s.headers[0].Get("Idempotency-Key"), s.headers[1].Get("Idempotency-Key")
	if len(k0) != 36 || k0 != k1 || k0[14] != '7' {
		t.Errorf("Idempotency-Key %q then %q, want one UUIDv7 on both", k0, k1)
	}
}

// TestHTTPClientDialsOnlyBaseURL: the request goes to the base URL while the proof names the
// public URL; a client without a token sends no Authorization header.
func TestHTTPClientDialsOnlyBaseURL(t *testing.T) {
	t.Parallel()
	s := &nonceServer{script: []func(http.ResponseWriter){okMe("")}}
	srv := httptest.NewServer(s)
	t.Cleanup(srv.Close)
	c, err := NewHTTPClient(srv.URL, "https://chat.example.com")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.GetMe(t.Context()); err != nil {
		t.Fatal(err)
	}
	if s.headers[0].Get("Authorization") != "" || s.headers[0].Get("DPoP") == "" || s.headers[0].Get("Origin") != "" {
		t.Errorf("headers %v", s.headers[0])
	}
}

// claimsOf decodes the claims of a compact JWS without checking it; nil when it is malformed.
func claimsOf(jws string) map[string]any {
	parts := strings.Split(jws, ".")
	if len(parts) != 3 {
		return nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil
	}
	var claims map[string]any
	if json.Unmarshal(raw, &claims) != nil {
		return nil
	}
	return claims
}

// TestHTTPClient_RetriesAfterRateLimit: a 429 with Retry-After is sent again with the same
// Idempotency-Key; a 429 without it ends the call.
func TestHTTPClient_RetriesAfterRateLimit(t *testing.T) {
	t.Parallel()
	var mu sync.Mutex
	var keys []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		keys = append(keys, r.Header.Get("Idempotency-Key"))
		n := len(keys)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/spaces") && n == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = io.WriteString(w, `{"code":"rate_limited"}`)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/me") {
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = io.WriteString(w, `{"code":"rate_limited"}`)
			return
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `{}`)
	}))
	defer srv.Close()
	c, err := NewHTTPClient(srv.URL, "https://chat.example.com")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.CreateSpace(t.Context(), rest.SpaceCreate{Name: "x"}); err != nil {
		t.Fatalf("CreateSpace after one 429: %v", err)
	}
	mu.Lock()
	if len(keys) != 2 || keys[0] == "" || keys[0] != keys[1] {
		t.Errorf("Idempotency-Keys %q, want the same key twice", keys)
	}
	mu.Unlock()
	var se *StatusError
	if _, err := c.GetMe(t.Context()); !errors.As(err, &se) || se.Status != http.StatusTooManyRequests {
		t.Errorf("GetMe = %v, want the 429 without Retry-After", err)
	}
}
