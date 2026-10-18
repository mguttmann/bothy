// SPDX-License-Identifier: Apache-2.0

package harness

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/jose"
)

// proofParts parses a DPoP proof and verifies its signature with the key of its header.
func proofParts(t *testing.T, proof string) (jose.Header, map[string]any) {
	t.Helper()
	j, err := jose.ParseCompactJWS(proof, []string{jose.AlgES256}, "dpop+jwt")
	if err != nil {
		t.Fatalf("parse proof: %v", err)
	}
	k, err := jose.ParseJWK(j.Header.JWK)
	if err != nil {
		t.Fatalf("header jwk: %v", err)
	}
	if err := j.Verify(k); err != nil {
		t.Fatalf("signature: %v", err)
	}
	var claims map[string]any
	if err := json.Unmarshal(j.Payload, &claims); err != nil {
		t.Fatal(err)
	}
	return j.Header, claims
}

// TestDPoPProofClaims: a proof carries typ, alg and the public jwk in the header; jti, htm, htu
// without query and fragment, iat, ath for a token and nonce when known.
func TestDPoPProofClaims(t *testing.T) {
	t.Parallel()
	k := newDeviceKey()
	now := time.Unix(1_790_000_000, 0)
	proof := k.proof(http.MethodPost, "http://127.0.0.1:1/api/v1/threads?x=1#f", "n-1", "tok", now)
	h, c := proofParts(t, proof)
	if h.Alg != jose.AlgES256 || h.Typ != "dpop+jwt" || len(h.JWK) == 0 || strings.Contains(string(h.JWK), `"d"`) {
		t.Fatalf("header %+v", h)
	}
	sum := sha256.Sum256([]byte("tok"))
	want := map[string]any{
		"htm": "POST", "htu": "http://127.0.0.1:1/api/v1/threads", "iat": float64(now.Unix()),
		"nonce": "n-1", "ath": base64.RawURLEncoding.EncodeToString(sum[:]),
	}
	for name, v := range want {
		if c[name] != v {
			t.Errorf("%s = %v, want %v", name, c[name], v)
		}
	}
	if _, err := ids.Parse(c["jti"].(string)); err != nil {
		t.Errorf("jti %v: %v", c["jti"], err)
	}
	_, c2 := proofParts(t, k.proof(http.MethodGet, "http://127.0.0.1:1/api/v1/me", "", "", now))
	if c2["jti"] == c["jti"] {
		t.Error("jti repeats")
	}
	if _, ok := c2["ath"]; ok {
		t.Error("ath without a token")
	}
	if _, ok := c2["nonce"]; ok {
		t.Error("nonce without a known nonce")
	}
}

// stub is a fake server: it demands the nonce "n-2", answers token_expired once for the first
// access token, and records every proof it receives.
type stub struct {
	mu      sync.Mutex
	proofs  []map[string]any
	expired bool
}

func (s *stub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	parts := strings.Split(r.Header.Get("DPoP"), ".")
	raw, _ := base64.RawURLEncoding.DecodeString(parts[1])
	var c map[string]any
	_ = json.Unmarshal(raw, &c)
	s.proofs = append(s.proofs, c)
	problem := func(code string) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.Header().Set("DPoP-Nonce", "n-2")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, `{"code":"`+code+`"}`)
	}
	switch {
	case c["nonce"] != "n-2":
		problem("use_dpop_nonce")
	case r.URL.Path == "/api/v1/auth/token/refresh":
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"access_token":"tok-2","refresh_token":"rt-2","session":{"id":"0192f4a0-7b1c-7f89-8d6b-6960453241bc"}}`)
	case r.Header.Get("Authorization") == "DPoP tok-1" && !s.expired:
		s.expired = true
		problem("token_expired")
	default:
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"ok":true}`)
	}
}

// TestClientNonceRetryAndRefresh: the client retries once with the nonce of a use_dpop_nonce
// answer, refreshes on token_expired and repeats the request with the new token.
func TestClientNonceRetryAndRefresh(t *testing.T) {
	t.Parallel()
	s := &stub{}
	srv := httptest.NewServer(s)
	t.Cleanup(srv.Close)
	c := &Client{publicURL: srv.URL, http: srv.Client(), key: newDeviceKey(), access: "tok-1", refresh: "rt-1"}
	r := c.Do(context.Background(), Request{Method: http.MethodGet, Path: "/me"})
	if r.Status != http.StatusOK {
		t.Fatalf("status %d %s", r.Status, r.Body)
	}
	// 1 without nonce -> use_dpop_nonce, 2 with nonce -> token_expired, 3 refresh, 4 repeat.
	if len(s.proofs) != 4 {
		t.Fatalf("%d requests, want 4: %v", len(s.proofs), s.proofs)
	}
	if _, ok := s.proofs[0]["nonce"]; ok {
		t.Error("first proof carries a nonce")
	}
	if c.access != "tok-2" || c.refresh != "rt-2" {
		t.Errorf("tokens after refresh: %q %q", c.access, c.refresh)
	}
	if _, ok := s.proofs[2]["ath"]; ok {
		t.Error("refresh proof carries ath")
	}
	sum := sha256.Sum256([]byte("tok-2"))
	if s.proofs[3]["ath"] != base64.RawURLEncoding.EncodeToString(sum[:]) {
		t.Errorf("repeat ath %v", s.proofs[3]["ath"])
	}
}

// TestClientIdempotencyKey: POST and PATCH carry a UUIDv7 Idempotency-Key unless the operation is
// exempt; GET never does.
func TestClientIdempotencyKey(t *testing.T) {
	t.Parallel()
	var got []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = append(got, r.Method+" "+r.URL.Path+" "+r.Header.Get("Idempotency-Key"))
		w.Header().Set("DPoP-Nonce", "n")
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(srv.Close)
	c := &Client{publicURL: srv.URL, http: srv.Client(), key: newDeviceKey(), access: "a", nonce: "n"}
	for _, r := range []Request{
		{Method: http.MethodPost, Path: "/spaces", Body: map[string]any{"name": "x"}},
		{Method: http.MethodPost, Path: "/threads/0192f4a0-7b1c-7f89-8d6b-6960453241bc/messages", Body: map[string]any{}},
		{Method: http.MethodGet, Path: "/spaces"},
	} {
		c.Do(context.Background(), r)
	}
	if len(got) != 3 {
		t.Fatalf("requests %v", got)
	}
	key := strings.Fields(got[0])
	if len(key) != 3 {
		t.Fatalf("no key on POST /spaces: %v", got)
	}
	if _, err := ids.Parse(key[2]); err != nil || key[2][14] != '7' {
		t.Errorf("key %q is no UUIDv7", key[2])
	}
	if len(strings.Fields(got[1])) != 2 || len(strings.Fields(got[2])) != 2 {
		t.Errorf("key on an exempt or GET request: %v", got)
	}
}

// TestClientDoAnonymous: DoAnonymous sends a DPoP proof of the client's key without the access
// token and hands the exchange to the Observer.
func TestClientDoAnonymous(t *testing.T) {
	t.Parallel()
	var auth, proof string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth, proof = r.Header.Get("Authorization"), r.Header.Get("DPoP")
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(srv.Close)
	seen := 0
	c := &Client{publicURL: srv.URL, http: srv.Client(), key: newDeviceKey(), access: "tok", nonce: "n",
		Observer: func(Request, Response) { seen++ }}
	r := c.DoAnonymous(context.Background(), Request{Method: http.MethodPost, Path: "/auth/token/refresh"})
	if r.Status != http.StatusNoContent || auth != "" || proof == "" || seen != 1 {
		t.Fatalf("status %d, Authorization %q, proof %v, observed %d", r.Status, auth, proof != "", seen)
	}
}
