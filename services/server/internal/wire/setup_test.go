// SPDX-License-Identifier: Apache-2.0

package wire_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	identityapp "github.com/OWNER/bothy/services/server/internal/identity/app"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

// call sends method path with body and header pairs to the listener on port.
func call(t *testing.T, port int, method, path, body string, headers ...string) (int, string, http.Header) {
	t.Helper()
	var rd io.Reader
	if body != "" {
		rd = strings.NewReader(body)
	}
	req, err := http.NewRequestWithContext(context.Background(), method, "http://127.0.0.1:"+strconv.Itoa(port)+path, rd)
	if err != nil {
		t.Fatal(err)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b), resp.Header
}

// TestIDT06_RequestsBeforeSetupGet503: a fresh instance answers every public request except the
// discovery reads with 503 setup_required (with and without a token, the redemption, an unknown
// path, the gateway path), the internal listener is unaffected, and after bothy setup
// bootstrap-owner on the host (wire.Setup, the code path of the command) the same request reaches
// the chain without a restart (08 "Erstinstallation", ID-T06).
func TestIDT06_RequestsBeforeSetupGet503(t *testing.T) {
	t.Parallel()
	db := newServeDB(t)
	env := serveEnv(t, db)
	p := start(t, wire.RoleAPI, env, wire.Deps{})
	internal := p.waitInternal(t)
	waitReadyz(t, internal, http.StatusOK)
	public := p.logs.port("public listener started")
	token := []string{"Authorization", "DPoP eyJhbGciOiJFZERTQSJ9.e30.EXAMPLE", "DPoP", "eyJhbGciOiJFUzI1NiJ9.e30.EXAMPLE"}
	for _, c := range []struct {
		method, path, body string
		headers            []string
	}{
		{http.MethodGet, "/api/v1/me", "", nil},
		{http.MethodGet, "/api/v1/me", "", token},
		{http.MethodPost, "/api/v1/auth/enroll/redeem", `{"enrollment_code":"en_EXAMPLE","device":{}}`, nil},
		{http.MethodGet, "/api/v1/unknown", "", nil},
		{http.MethodGet, "/api/v1/gateway", "", token},
	} {
		code, body, hdr := call(t, public, c.method, c.path, c.body, c.headers...)
		if code != http.StatusServiceUnavailable || !strings.Contains(body, `"code":"setup_required"`) || hdr.Get("Retry-After") != "" {
			t.Fatalf("%s %s = %d %s (Retry-After %q), want 503 setup_required", c.method, c.path, code, body, hdr.Get("Retry-After"))
		}
	}
	if code, body, _ := call(t, public, http.MethodGet, "/api/v1/instance/jwks", ""); code != http.StatusOK {
		t.Fatalf("jwks before setup = %d %s", code, body)
	}
	if code, _ := get(t, internal, "/readyz"); code != http.StatusOK {
		t.Fatalf("/readyz = %d", code)
	}

	s, err := wire.NewSetup(context.Background(), loadEnv(t, env), env, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	res, err := s.BootstrapOwner(context.Background(), identityapp.BootstrapOwnerCmd{Handle: "alex", DisplayName: "Alex"})
	s.Close(context.Background())
	if err != nil || res.Code.Code == "" {
		t.Fatalf("BootstrapOwner = %+v, %v", res, err)
	}
	if code, body, _ := call(t, public, http.MethodGet, "/api/v1/me", ""); code != http.StatusUnauthorized {
		t.Fatalf("after setup without a restart: %d %s, want 401", code, body)
	}
}
