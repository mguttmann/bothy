// SPDX-License-Identifier: Apache-2.0

package bothytest_test

import (
	"bytes"
	"context"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/cmd/bothy/importtool/bothyapi"
	"github.com/OWNER/bothy/services/server/cmd/bothy/importtool/bothyapi/bothytest"
)

// TestFakeAuthPaths drives the fake through the client: pairing, protected routes, the counters,
// a rejected proof without a token, a 404 and a revoked logout.
func TestFakeAuthPaths(t *testing.T) {
	fake := bothytest.New(t, bothytest.Options{})
	path := filepath.Join(t.TempDir(), "bothy", "session.json")
	var out bytes.Buffer
	deps := bothyapi.Deps{Sleep: func(ctx context.Context, _ time.Duration) error { fake.Approve(); return ctx.Err() }}
	go fake.WaitPairing()
	if err := bothyapi.Login(t.Context(), bothyapi.LoginOptions{ServerURL: fake.URL, SessionFile: path,
		Limits: bothyapi.DefaultLimits(), Deps: deps, Stdout: &out}); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out.String()) != fake.Code() {
		t.Errorf("code = %q", out.String())
	}
	c, err := bothyapi.New(t.Context(), bothyapi.Options{SessionFile: path, Limits: bothyapi.DefaultLimits(), Deps: deps})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Limits(t.Context()); err != nil {
		t.Fatal(err)
	}
	if n := fake.Count(http.MethodGet, bothytest.PathInstance); n < 1 {
		t.Errorf("getInstance requests = %d", n)
	}
	resp, err := http.Get(fake.URL + bothytest.PathMe) //nolint:noctx // test of the fake only
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("getMe without token = %d", resp.StatusCode)
	}
	resp, err = http.Get(fake.URL + "/api/v1/unknown") //nolint:noctx // test of the fake only
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("unknown route = %d", resp.StatusCode)
	}
	if err := bothyapi.Logout(t.Context(), bothyapi.Options{SessionFile: path, Limits: bothyapi.DefaultLimits(), Deps: deps}); err != nil {
		t.Fatal(err)
	}
	if !fake.Revoked() || fake.Rotations() != 0 {
		t.Errorf("revoked = %v, rotations = %d", fake.Revoked(), fake.Rotations())
	}
}
