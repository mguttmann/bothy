// SPDX-License-Identifier: Apache-2.0

package oidctest_test

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/identity/adapters/oidc/oidctest"
	"github.com/OWNER/bothy/services/server/internal/kernel/jose"
)

// s256 is the PKCE challenge of verifier.
func s256(v string) string {
	sum := sha256.Sum256([]byte(v))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// token posts form to the token endpoint with Basic authentication of the client chat and secret.
func token(t *testing.T, idp *oidctest.IDP, form url.Values, secret string) (int, map[string]any) {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, idp.URL()+"/token", strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth("chat", secret)
	resp, err := idp.Browser().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	var m map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&m)
	return resp.StatusCode, m
}

// TestFakeProvider: the fake provider issues a code for the server's PKCE challenge, refuses a
// wrong verifier, client or reused code, signs ID tokens with the key its JWKS publishes, and
// redirects with an error or in Entra mode with the tenant issuer as configured.
func TestFakeProvider(t *testing.T) {
	t.Parallel()
	const verifier = "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	for _, alg := range []string{jose.AlgRS256, jose.AlgPS256, jose.AlgES256} {
		idp := oidctest.New(t, oidctest.Options{ClientID: "chat", Secret: "EXAMPLE", Alg: alg, SendIss: true, Entra: alg == jose.AlgES256})
		idp.NextLogin(oidctest.Login{Subject: "s", Name: "Alex", PreferredUsername: "alex@example.com", Email: "alex@example.com", TenantID: "t1"})
		q := url.Values{"client_id": {"chat"}, "response_type": {"code"}, "redirect_uri": {"https://chat.example.com/cb"}, "state": {"st"},
			"nonce": {"n"}, "code_challenge": {s256(verifier)}, "code_challenge_method": {"S256"}}
		loc := idp.Authorize(t, idp.URL()+"/authorize?"+q.Encode())
		if loc.Query().Get("state") != "st" || loc.Query().Get("iss") != idp.TenantIssuer("t1") {
			t.Fatalf("%s: redirect %s", alg, loc)
		}
		form := url.Values{"grant_type": {"authorization_code"}, "code": {loc.Query().Get("code")}, "redirect_uri": {"https://chat.example.com/cb"},
			"code_verifier": {verifier[:42] + "A"}}
		if st, _ := token(t, idp, form, "EXAMPLE"); st != http.StatusBadRequest {
			t.Fatalf("%s: wrong verifier %d", alg, st)
		}
		form.Set("code_verifier", verifier)
		if st, _ := token(t, idp, form, "wrong"); st != http.StatusUnauthorized {
			t.Fatalf("%s: wrong secret %d", alg, st)
		}
		st, m := token(t, idp, form, "EXAMPLE")
		if st != http.StatusOK || idp.BasicLogins() != 1 {
			t.Fatalf("%s: token %d %v", alg, st, m)
		}
		j, err := jose.ParseCompactJWS(m["id_token"].(string), []string{alg}, "JWT")
		if err != nil {
			t.Fatalf("%s: %v", alg, err)
		}
		if !strings.Contains(string(j.Payload), `"nonce":"n"`) || !strings.Contains(string(j.Payload), `"tid":"t1"`) {
			t.Fatalf("%s: payload %s", alg, j.Payload)
		}
		if st, _ := token(t, idp, form, "EXAMPLE"); st != http.StatusBadRequest {
			t.Fatalf("%s: code reused %d", alg, st)
		}
		resp, err := idp.Browser().Get(idp.URL() + "/jwks") //nolint:noctx // test of the fake
		if err != nil {
			t.Fatal(err)
		}
		var set struct {
			Keys []json.RawMessage `json:"keys"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&set)
		_ = resp.Body.Close()
		k, err := jose.ParseVerificationJWK(set.Keys[0])
		if err != nil || j.Verify(k) != nil {
			t.Fatalf("%s: the JWKS does not verify the token: %v", alg, err)
		}
		methods, verifiers := idp.Seen()
		if methods[0] != "S256" || len(verifiers) != 4 || idp.Count("authorize") != 1 || idp.Count("jwks") != 1 {
			t.Fatalf("%s: seen %v %d", alg, methods, len(verifiers))
		}
	}
	idp := oidctest.New(t, oidctest.Options{ClientID: "chat"})
	idp.FailAuthorize("access_denied")
	idp.RotateKey("k2", true)
	loc := idp.Authorize(t, idp.URL()+"/authorize?client_id=chat&response_type=code&redirect_uri=https%3A%2F%2Fchat.example.com%2Fcb&state=s")
	if loc.Query().Get("error") != "access_denied" || loc.Query().Get("code") != "" {
		t.Fatalf("error redirect %s", loc)
	}
	idp.Reset()
	idp.SetHeader("alg", "none")
	idp.SetClaim("aud", "other")
	idp.DropClaim("nonce")
	idp.OmitIss()
	if len(idp.Assertions()) != 0 || idp.Issuer() != idp.URL() || idp.RootCAs() == nil {
		t.Fatal("fresh provider state")
	}
}
