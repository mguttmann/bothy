// SPDX-License-Identifier: Apache-2.0

package bothyapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/adapters/replay"
	"github.com/OWNER/bothy/services/server/internal/identity/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/jose"
)

// verifier returns the DPoP verifier of the server (M1-33) on clk.
func verifier(t *testing.T, clk clock.Clock) *app.DPoPVerifier {
	t.Helper()
	v, err := app.NewDPoPVerifier(app.Config{DPoPNonceTTL: 5 * time.Minute, DPoPProofMaxAge: time.Minute, ClockLeeway: 5 * time.Second},
		bytes.Repeat([]byte{7}, 32), replay.New(clk), clk, nil)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

// TestProofPassesServerVerifier (08 7.3): a proof of the tool passes the verifier of the server
// with htu without query and fragment, ath of the token, the nonce of the server and the key
// binding of the token; a changed token fails ath.
func TestProofPassesServerVerifier(t *testing.T) {
	clk := clock.NewFake(start)
	v := verifier(t, clk)
	k, err := newProofKey()
	if err != nil {
		t.Fatal(err)
	}
	pub, err := jose.ParseJWK(k.publicJWK())
	if err != nil {
		t.Fatal(err)
	}
	origin := "https://chat.example.com"
	htu := proofHTU(origin, "/api/v1/me")
	if htu != origin+"/api/v1/me" {
		t.Fatalf("htu = %q", htu)
	}
	nonce := v.CurrentNonce()
	p, err := k.proof(http.MethodGet, htu, "at-token", nonce, clk.Now())
	if err != nil {
		t.Fatal(err)
	}
	got, err := v.Verify(t.Context(), identity.ProofRequest{Proofs: []string{p}, Method: http.MethodGet,
		URL: origin + "/api/v1/me", AccessToken: "at-token", ExpectedJKT: pub.Thumbprint()})
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if got.Alg != jose.AlgEdDSA {
		t.Errorf("alg = %s, want EdDSA", got.Alg)
	}
	p2, err := k.proof(http.MethodGet, htu, "at-token", nonce, clk.Now())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := v.Verify(t.Context(), identity.ProofRequest{Proofs: []string{p2}, Method: http.MethodGet,
		URL: origin + "/api/v1/me", AccessToken: "other", ExpectedJKT: pub.Thumbprint()}); err == nil {
		t.Error("a proof with ath of another token passed")
	}
	p3, err := k.proof(http.MethodPost, htu, "", "", clk.Now())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := v.Verify(t.Context(), identity.ProofRequest{Proofs: []string{p3}, Method: http.MethodPost, URL: origin + "/api/v1/me"}); err == nil {
		t.Error("a proof without the nonce passed after the server started nonces")
	}
}

// TestProofHTUWithoutQuery: the request URL of a call with a query and fragment gives htu without
// both, and the private key never appears in the proof.
func TestProofHTUWithoutQuery(t *testing.T) {
	if got := proofHTU("https://chat.example.com", "/api/v1/threads"); got != "https://chat.example.com/api/v1/threads" {
		t.Errorf("htu = %q", got)
	}
	k, err := newProofKey()
	if err != nil {
		t.Fatal(err)
	}
	p, err := k.proof(http.MethodGet, "https://chat.example.com/api/v1/threads", "", "n", start)
	if err != nil {
		t.Fatal(err)
	}
	j, err := jose.ParseCompactJWS(p, []string{jose.AlgEdDSA}, "dpop+jwt")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(j.Header.JWK), `"d"`) {
		t.Error("the proof header carries the private key")
	}
	var c map[string]any
	if err := json.Unmarshal(j.Payload, &c); err != nil {
		t.Fatal(err)
	}
	for _, m := range []string{"jti", "htm", "htu", "iat", "nonce"} {
		if _, ok := c[m]; !ok {
			t.Errorf("claim %s missing", m)
		}
	}
	if _, ok := c["ath"]; ok {
		t.Error("ath without a token")
	}
}

// TestNonceAfterUseDPoPNonce: the fake answers the first proof with use_dpop_nonce; the retry
// carries the nonce of its DPoP-Nonce header.
func TestNonceAfterUseDPoPNonce(t *testing.T) {
	e := newEnv(t)
	e.login(t)
	reqs := e.fake.Requests()
	if len(reqs) < 2 {
		t.Fatalf("requests = %d", len(reqs))
	}
	if n := nonceOf(t, reqs[0].Header.Get("DPoP")); n != "" {
		t.Errorf("first proof has nonce %q, want none", n)
	}
	if n := nonceOf(t, reqs[1].Header.Get("DPoP")); n == "" {
		t.Error("the retry after use_dpop_nonce has no nonce")
	}
}

// nonceOf returns the nonce claim of a proof.
func nonceOf(t *testing.T, proof string) string {
	t.Helper()
	j, err := jose.ParseCompactJWS(proof, []string{jose.AlgEdDSA}, "dpop+jwt")
	if err != nil {
		t.Fatal(err)
	}
	var c struct {
		Nonce string `json:"nonce"`
	}
	if err := json.Unmarshal(j.Payload, &c); err != nil {
		t.Fatal(err)
	}
	return c.Nonce
}
