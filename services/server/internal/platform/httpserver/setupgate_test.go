// SPDX-License-Identifier: Apache-2.0

package httpserver_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
)

// fakeSetup is a SetupState that counts its calls.
type fakeSetup struct {
	done  atomic.Bool
	err   error
	calls atomic.Int32
}

func (f *fakeSetup) SetupCompleted(context.Context) (bool, error) {
	f.calls.Add(1)
	return f.done.Load(), f.err
}

// fakeRoutes maps "METHOD path" to an action.
type fakeRoutes map[string]string

func (f fakeRoutes) Action(r *http.Request) (string, bool) {
	a, ok := f[r.Method+" "+r.URL.Path]
	return a, ok
}

// gateRoutes is a route table with the discovery reads and two other operations.
var gateRoutes = fakeRoutes{
	"GET /api/v1/instance/jwks":       "instance.discovery.read",
	"GET /api/v1/discovery":           "instance.discovery.read",
	"GET /api/v1/me":                  "self.read",
	"POST /api/v1/auth/enroll/redeem": "auth.login.complete",
}

// gateServe runs one request through the gate in front of a handler that answers 204.
func gateServe(t *testing.T, gate func(http.Handler) http.Handler, method, path string) *httptest.ResponseRecorder {
	t.Helper()
	h := gate(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), method, "http://example.com"+path, nil)
	req.Header.Set("Authorization", "DPoP token-EXAMPLE")
	h.ServeHTTP(rec, req)
	return rec
}

// problemCode returns the code member of a problem body.
func problemCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var p map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatalf("problem body %q: %v", rec.Body, err)
	}
	code, _ := p["code"].(string)
	return code
}

func TestSetupGateAllowsOnlyDiscoveryActions(t *testing.T) {
	t.Parallel()
	gate := httpserver.SetupGate(&fakeSetup{}, gateRoutes)
	for _, c := range []struct {
		method, path string
		pass         bool
	}{
		{http.MethodGet, "/api/v1/instance/jwks", true},
		{http.MethodGet, "/api/v1/discovery", true},
		{http.MethodGet, "/api/v1/me", false},
		{http.MethodPost, "/api/v1/auth/enroll/redeem", false},
		{http.MethodGet, "/api/v1/unknown", false},
		{http.MethodGet, "/api/v1/gateway", false},
		{http.MethodPost, "/api/v1/instance/jwks", false},
	} {
		rec := gateServe(t, gate, c.method, c.path)
		if c.pass {
			if rec.Code != http.StatusNoContent {
				t.Errorf("%s %s: status %d, want it passed", c.method, c.path, rec.Code)
			}
			continue
		}
		if rec.Code != http.StatusServiceUnavailable || problemCode(t, rec) != "setup_required" {
			t.Errorf("%s %s: %d %s, want 503 setup_required", c.method, c.path, rec.Code, rec.Body)
		}
		if rec.Header().Get("Retry-After") != "" {
			t.Errorf("%s %s: Retry-After %q, want none", c.method, c.path, rec.Header().Get("Retry-After"))
		}
	}
}

func TestSetupGateCachesCompletion(t *testing.T) {
	t.Parallel()
	state := &fakeSetup{}
	gate := httpserver.SetupGate(state, gateRoutes)
	if rec := gateServe(t, gate, http.MethodGet, "/api/v1/me"); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("before setup: %d", rec.Code)
	}
	// The bootstrap on the host completes the setup; the next request passes without a restart.
	state.done.Store(true)
	if rec := gateServe(t, gate, http.MethodGet, "/api/v1/me"); rec.Code != http.StatusNoContent {
		t.Fatalf("after setup: %d %s", rec.Code, rec.Body)
	}
	calls := state.calls.Load()
	for range 3 {
		if rec := gateServe(t, gate, http.MethodGet, "/api/v1/unknown"); rec.Code != http.StatusNoContent {
			t.Fatalf("cached: %d", rec.Code)
		}
	}
	if got := state.calls.Load(); got != calls || calls != 2 {
		t.Fatalf("SetupState called %d times after the first true (%d before), want no more calls", got-calls, calls)
	}
}

func TestSetupGateFailsClosed(t *testing.T) {
	t.Parallel()
	state := &fakeSetup{err: errors.New("database down")}
	gate := httpserver.SetupGate(state, gateRoutes)
	rec := gateServe(t, gate, http.MethodGet, "/api/v1/me")
	if rec.Code != http.StatusServiceUnavailable || problemCode(t, rec) != "unavailable" {
		t.Fatalf("%d %s, want 503 unavailable", rec.Code, rec.Body)
	}
	// Discovery stays readable, it needs no setup state.
	if rec := gateServe(t, gate, http.MethodGet, "/api/v1/instance/jwks"); rec.Code != http.StatusNoContent {
		t.Fatalf("discovery with a failing state: %d", rec.Code)
	}
}

func TestMuxLookup(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	mux.Handle("GET /api/v1/users/{user_id}", http.NotFoundHandler())
	mux.Handle("GET /api/v1/instance/jwks", http.NotFoundHandler())
	l := httpserver.MuxLookup(mux, map[string]string{"GET /api/v1/users/{user_id}": "user.read", "GET /api/v1/instance/jwks": "instance.discovery.read"})
	for _, c := range []struct {
		method, path, action string
		ok                   bool
	}{
		{http.MethodGet, "/api/v1/users/0192f4a0-7b1c-7f89-8d6b-6960453241bc", "user.read", true},
		{http.MethodGet, "/api/v1/instance/jwks", "instance.discovery.read", true},
		{http.MethodPost, "/api/v1/instance/jwks", "", false},
		{http.MethodGet, "/api/v1/nothing", "", false},
	} {
		a, ok := l.Action(httptest.NewRequestWithContext(context.Background(), c.method, "http://example.com"+c.path, nil))
		if a != c.action || ok != c.ok {
			t.Errorf("%s %s = %q, %v; want %q, %v", c.method, c.path, a, ok, c.action, c.ok)
		}
	}
}
