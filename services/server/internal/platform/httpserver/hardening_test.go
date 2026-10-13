// SPDX-License-Identifier: Apache-2.0

package httpserver_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
)

// TestUnmatched_UncleanPathIsProblem: a path that net/http would redirect to its clean form never
// gets the mux's own 307 with an HTML body; it is a 404 problem (every request through the chain).
func TestUnmatched_UncleanPathIsProblem(t *testing.T) {
	t.Parallel()
	disc := discoveryOp()
	h := httpservertest.New(t, httpservertest.Options{Routes: []httpservertest.Route{{Op: disc, Handler: okHandler(disc.Action)}}})
	for _, path := range []string{"/api/v1//discovery", "/api/v1/./discovery", "/api/v1/x/../discovery"} {
		req := h.NewRequest(http.MethodGet, "/", "")
		req.URL.Path = path
		req.URL.RawPath = ""
		resp := h.Do(req)
		if resp.StatusCode != 404 || resp.Problem(t)["code"] != "not_found" || resp.Header.Get("Cache-Control") != "no-store" {
			t.Errorf("%s: %d %v", path, resp.StatusCode, resp.Header)
		}
	}
}

// TestDeadline_SlowJSONBodyIsCut: the JSON guard reads the body under the read deadline of
// http.request_timeout; a client that never sends its declared body does not hold the request.
func TestDeadline_SlowJSONBodyIsCut(t *testing.T) {
	t.Parallel()
	cfg := httpserver.DefaultChainConfig(httpservertest.PublicURL)
	cfg.RequestTimeout = 200 * time.Millisecond
	op := httpservertest.Op("createThing", http.MethodPost, "/api/v1/things", "thing.create")
	h := httpservertest.New(t, httpservertest.Options{Config: &cfg, Routes: []httpservertest.Route{{Op: op, Handler: okHandler(op.Action)}}})
	pr, pw := io.Pipe()
	defer func() { _ = pw.Close() }()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, h.Server.URL+"/api/v1/things", pr)
	if err != nil {
		t.Fatal(err)
	}
	req.ContentLength = 100
	req.Header.Set("Content-Type", "application/json")
	go func() { _, _ = pw.Write([]byte(`{"name":`)) }()
	start := time.Now()
	resp, err := h.Server.Client().Do(req)
	if err == nil {
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusServiceUnavailable {
			t.Errorf("status %d, want 503", resp.StatusCode)
		}
	}
	if d := time.Since(start); d > 5*time.Second {
		t.Fatalf("the slow body held the request for %v", d)
	}
}

// TestNoNull_EmptyJSONBodyPasses: a JSON content type without a body (204, an empty 200) is not
// refused as invalid JSON.
func TestNoNull_EmptyJSONBodyPasses(t *testing.T) {
	t.Parallel()
	op := getThing()
	h := httpservertest.New(t, httpservertest.Options{Routes: []httpservertest.Route{{Op: op,
		Handler: httpservertest.Authorized(op.Action, func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNoContent)
		})}}})
	if resp := getOne(h); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status %d", resp.StatusCode)
	}
}

// TestAuth_SecondAuthorizationLineIsACredential: an empty first Authorization line does not make a
// request with a second line anonymous.
func TestAuth_SecondAuthorizationLineIsACredential(t *testing.T) {
	t.Parallel()
	op := getThing()
	h := httpservertest.New(t, httpservertest.Options{Authenticator: actor.AnonymousOnly{}, Routes: []httpservertest.Route{{Op: op, Handler: okHandler(op.Action)}}})
	req := h.NewRequest(http.MethodGet, "/api/v1/things/"+thingID, "", "X-Mode", "fast")
	req.Header["Authorization"] = []string{"", "Bearer smuggled"}
	if resp := h.Do(req); resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status %d, want 401", resp.StatusCode)
	}
}

// TestCORS_PreflightNamesItsRoute: a preflight records its route for span and access log.
func TestCORS_PreflightNamesItsRoute(t *testing.T) {
	t.Parallel()
	httpservertest.Spans()
	c := newChain(t)
	mux := http.NewServeMux()
	mux.Handle("OPTIONS /api/v1/things/{thing_id}", c.Preflight("/api/v1/things/{thing_id}", []string{"GET"}))
	req := httptest.NewRequestWithContext(context.Background(), http.MethodOptions, "/api/v1/things/"+thingID, nil)
	rec := httptest.NewRecorder()
	c.Public(mux).ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status %d", rec.Code)
	}
	found := false
	for _, s := range httpservertest.Spans().Ended() {
		if strings.HasSuffix(s.Name(), "OPTIONS /api/v1/things/{thing_id}") {
			found = true
		}
	}
	if !found {
		t.Fatal("no span named after the preflight route")
	}
}
