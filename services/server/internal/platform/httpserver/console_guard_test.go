// SPDX-License-Identifier: Apache-2.0

package httpserver_test

import (
	"net/http"
	"sync/atomic"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
)

// TestConsoleGuard: a request authenticated by the console cookie with a method other than GET,
// HEAD and OPTIONS needs X-Console-Request: 1 and the Origin of server.public_url; without them it
// is 403 forbidden with reason csrf_check_failed and the handler never runs (SEC-CORS-4,
// D-M6-01-5). A GET passes without the header, and a request with an Authorization header is not
// checked at all.
func TestConsoleGuard(t *testing.T) {
	t.Parallel()
	console := actor.Actor{ID: ids.New(), Kind: actor.KindUser, SessionID: ids.New(), DeviceID: ids.New(), ClientComponent: "console"}
	app := actor.Actor{ID: ids.New(), Kind: actor.KindUser, SessionID: ids.New(), DeviceID: ids.New(), ClientComponent: "app"}
	auth := httpservertest.NewActors()
	auth.SetCookie("cookie-alex", console)
	auth.Set("app-alex", app)
	var calls atomic.Int64
	post := httpservertest.Op("createThing", http.MethodPost, "/api/v1/things", "thing.create")
	get := getThing()
	h := httpservertest.New(t, httpservertest.Options{Authenticator: auth, Routes: []httpservertest.Route{
		{Op: post, Handler: httpservertest.Authorized(post.Action, func(w http.ResponseWriter, _ *http.Request) {
			calls.Add(1)
			w.WriteHeader(http.StatusNoContent)
		})},
		{Op: get, Handler: okHandler(get.Action)},
	}})
	cookie := "Cookie"
	value := httpserver.ConsoleCookieHeader("cookie-alex")
	for _, tc := range []struct {
		name    string
		headers []string
	}{
		{"no header", []string{cookie, value, "Origin", httpservertest.PublicURL}},
		{"wrong value", []string{cookie, value, httpserver.HeaderConsoleRequest, "2", "Origin", httpservertest.PublicURL}},
		{"foreign origin", []string{cookie, value, httpserver.HeaderConsoleRequest, "1", "Origin", "https://evil.example.com"}},
		{"other port", []string{cookie, value, httpserver.HeaderConsoleRequest, "1", "Origin", "https://chat.example.com:8443"}},
		{"no origin", []string{cookie, value, httpserver.HeaderConsoleRequest, "1"}},
		{"null origin", []string{cookie, value, httpserver.HeaderConsoleRequest, "1", "Origin", "null"}},
	} {
		resp := h.Do(h.NewRequest(http.MethodPost, "/api/v1/things", `{"name":"ab","color":"red"}`, tc.headers...))
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("%s: status %d: %s", tc.name, resp.StatusCode, resp.Body)
			continue
		}
		if p := resp.Problem(t); p["code"] != "forbidden" || p["reason"] != "csrf_check_failed" {
			t.Errorf("%s: problem %v", tc.name, p)
		}
	}
	if n := calls.Load(); n != 0 {
		t.Fatalf("the handler ran %d times for refused requests", n)
	}
	for _, origin := range []string{httpservertest.PublicURL, "https://CHAT.example.com:443"} {
		resp := h.Do(h.NewRequest(http.MethodPost, "/api/v1/things", `{"name":"ab","color":"red"}`, cookie, value,
			httpserver.HeaderConsoleRequest, "1", "Origin", origin))
		if resp.StatusCode != http.StatusNoContent {
			t.Fatalf("origin %s: status %d: %s", origin, resp.StatusCode, resp.Body)
		}
	}
	if resp := h.Do(h.NewRequest(http.MethodGet, "/api/v1/things/"+thingID, "", cookie, value, "X-Mode", "fast")); resp.StatusCode != http.StatusOK {
		t.Fatalf("GET with the cookie: status %d: %s", resp.StatusCode, resp.Body)
	}
	if resp := h.Do(h.NewRequest(http.MethodPost, "/api/v1/things", `{"name":"ab","color":"red"}`, "Authorization", "Bearer app-alex")); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("app request: status %d: %s", resp.StatusCode, resp.Body)
	}
	if n := calls.Load(); n != 3 {
		t.Fatalf("handler calls %d, want 3", n)
	}
	if h.Logs.Contains("cookie-alex") {
		t.Fatal("a log record carries the cookie value")
	}
}
