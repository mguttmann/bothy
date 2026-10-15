// SPDX-License-Identifier: Apache-2.0

package httpapi_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os/exec"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/adapters/httpapi"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
)

func TestDPoPError_Rendering(t *testing.T) {
	t.Parallel()
	for _, code := range []string{identity.DPoPInvalidProof, identity.DPoPUseNonce} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "https://chat.example.com/api/v1/me", nil)
		httpserver.WriteProblem(rec, req, &identity.DPoPError{Code: code, Nonce: "nonce-1"})
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("%s: status %d", code, rec.Code)
		}
		if got, want := rec.Header().Get("WWW-Authenticate"), `DPoP error="`+code+`", algs="ES256 EdDSA"`; got != want {
			t.Errorf("%s: WWW-Authenticate %q, want %q", code, got, want)
		}
		if got := rec.Header().Get("DPoP-Nonce"); got != "nonce-1" {
			t.Errorf("%s: DPoP-Nonce %q", code, got)
		}
		var body map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if body["code"] != code {
			t.Errorf("%s: body code %v", code, body["code"])
		}
		for _, leak := range []string{"htu", "mismatch", "jti", "nonce_", "signature"} {
			if strings.Contains(rec.Body.String(), leak) {
				t.Errorf("%s: body names an internal reason (%q): %s", code, leak, rec.Body)
			}
		}
	}
}

func TestExpectedHTU(t *testing.T) {
	t.Parallel()
	pub, _ := url.Parse("https://chat.example.com")
	for target, want := range map[string]string{
		"http://evil.example.org/api/v1/me?x=1":      "https://chat.example.com/api/v1/me",
		"/api/v1/devices/0192f4a0-7b1c-7848-8501-17": "https://chat.example.com/api/v1/devices/0192f4a0-7b1c-7848-8501-17",
		"/api/v1/a%2Fb": "https://chat.example.com/api/v1/a%2Fb",
	} {
		r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, target, nil)
		r.Host = "evil.example.org"
		if got := httpapi.ExpectedHTU(pub, r); got != want {
			t.Errorf("ExpectedHTU(%q) = %q, want %q", target, got, want)
		}
	}
	withPort, _ := url.Parse("http://192.0.2.1:8443")
	if got := httpapi.ExpectedHTU(withPort, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/x", nil)); got != "http://192.0.2.1:8443/x" {
		t.Errorf("with port: %q", got)
	}
}

// TestIdentitytest_NotInBinary: the server binary never depends on the test helpers, the fake
// identity provider of M1-41 included.
func TestIdentitytest_NotInBinary(t *testing.T) {
	t.Parallel()
	out, err := exec.CommandContext(t.Context(), "go", "list", "-deps", "../../../../cmd/bothy").CombinedOutput()
	if err != nil {
		t.Fatalf("go list: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "/internal/identity/adapters/httpapi") {
		t.Fatalf("go list output does not look like the dependencies of cmd/bothy:\n%.400s", out)
	}
	for _, helper := range []string{"/internal/identity/adapters/identitytest", "/internal/identity/adapters/oidc/oidctest"} {
		if strings.Contains(string(out), helper) {
			t.Fatalf("cmd/bothy depends on %s", helper)
		}
	}
}
