// SPDX-License-Identifier: Apache-2.0

package httpserver_test

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/platform/health"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
)

// Header sets of 19-sicherheit.md "HTTP-Sicherheitskopfzeilen". Later tickets add their sets
// here (files in M3, console in M6) and their rows to responseKinds.
var (
	// hdr1 is SEC-HDR-1 without HSTS, which depends on the scheme of server.public_url.
	hdr1 = map[string]string{"X-Content-Type-Options": "nosniff", "Referrer-Policy": "no-referrer"}
	// hdr2 is SEC-HDR-2 without Content-Type, which each kind names itself.
	hdr2 = map[string]string{"Cache-Control": "no-store", "Cross-Origin-Resource-Policy": "same-origin"}
	// hdr3 is SEC-HDR-3 without the CSP, whose nonce differs per response (TestCSPHeader); the
	// console rows of SEC-T03 are TestConsoleIsolationHeaders in csp_test.go (M6-03).
	hdr3 = map[string]string{
		"Cross-Origin-Opener-Policy":   "same-origin",
		"Cross-Origin-Embedder-Policy": "require-corp",
		"Cross-Origin-Resource-Policy": "same-origin",
		"X-Frame-Options":              "DENY",
		"Permissions-Policy":           "camera=(), microphone=(), geolocation=(), payment=(), usb=()",
		"Reporting-Endpoints":          `csp="/api/v1/csp-reports"`,
	}
)

const (
	mediaJSON    = "application/json"
	mediaProblem = "application/problem+json"
	// testHSTSMaxAge is http.hsts_max_age of the header tests.
	testHSTSMaxAge = 3600 * time.Second
)

// responseKind is one row of SEC-T03: how to produce the response and what it must carry.
type responseKind struct {
	name     string
	internal bool                                          // served by the internal listener
	request  func(h *httpservertest.Harness) *http.Request // public kinds
	path     string                                        // internal kinds: method and path
	method   string
	started  bool   // internal kinds: the start finished (readiness)
	status   int    // expected status
	media    string // expected Content-Type; "" means no body
	api      bool   // SEC-HDR-2 applies
}

// responseKinds are the kinds of design note 7 of brief M1-21.
var responseKinds = []responseKind{
	{name: "200 json", status: 200, media: mediaJSON, api: true, request: func(h *httpservertest.Harness) *http.Request {
		return h.NewRequest(http.MethodGet, "/api/v1/things/"+thingID, "", "X-Mode", "fast")
	}},
	{name: "204 empty", status: 204, api: true, request: func(h *httpservertest.Harness) *http.Request {
		return h.NewRequest(http.MethodDelete, "/api/v1/things/"+thingID, "")
	}},
	{name: "400 malformed json", status: 400, media: mediaProblem, api: true, request: func(h *httpservertest.Harness) *http.Request {
		return h.NewRequest(http.MethodPost, "/api/v1/things", `{"name":`)
	}},
	{name: "400 validation", status: 400, media: mediaProblem, api: true, request: func(h *httpservertest.Harness) *http.Request {
		return h.NewRequest(http.MethodGet, "/api/v1/things/"+thingID, "")
	}},
	{name: "401", status: 401, media: mediaProblem, api: true, request: func(h *httpservertest.Harness) *http.Request {
		return h.NewRequest(http.MethodGet, "/api/v1/things/"+thingID, "", "X-Mode", "fast", "Authorization", "Bearer bad")
	}},
	{name: "404 unknown path", status: 404, media: mediaProblem, api: true, request: func(h *httpservertest.Harness) *http.Request {
		return h.NewRequest(http.MethodGet, "/nope", "")
	}},
	{name: "405 wrong method", status: 405, media: mediaProblem, api: true, request: func(h *httpservertest.Harness) *http.Request {
		return h.NewRequest(http.MethodPut, "/api/v1/things/"+thingID, `{}`)
	}},
	{name: "413 body too large", status: 413, media: mediaProblem, api: true, request: func(h *httpservertest.Harness) *http.Request {
		return h.NewRequest(http.MethodPost, "/api/v1/things", `{"name":"`+strings.Repeat("a", 200)+`","color":"red"}`)
	}},
	{name: "500 panic", status: 500, media: mediaProblem, api: true, request: func(h *httpservertest.Harness) *http.Request {
		return h.NewRequest(http.MethodPost, "/api/v1/things", `{"name":"ab","color":"red"}`)
	}},
	{name: "500 authz_not_checked", status: 500, media: mediaProblem, api: true, request: func(h *httpservertest.Harness) *http.Request {
		return h.NewRequest(http.MethodGet, "/api/v1/things/"+thingID, "", "X-Mode", "slow")
	}},
	{name: "503 unavailable", status: 503, media: mediaProblem, api: true, request: func(h *httpservertest.Harness) *http.Request {
		return h.NewRequest(http.MethodGet, "/api/v1/things/"+thingID+"?limit=3", "", "X-Mode", "fast")
	}},
	{name: "internal healthz", internal: true, method: http.MethodGet, path: health.PathLiveness, status: 200, media: mediaJSON, api: true},
	{name: "internal readyz ready", internal: true, method: http.MethodGet, path: health.PathReadiness, started: true, status: 200, media: mediaJSON, api: true},
	{name: "internal readyz starting", internal: true, method: http.MethodGet, path: health.PathReadiness, status: 503, media: mediaJSON, api: true},
	{name: "internal head readyz", internal: true, method: http.MethodHead, path: health.PathReadiness, started: true, status: 200, media: mediaJSON, api: true},
	{name: "internal unknown path", internal: true, method: http.MethodGet, path: "/metrics", status: 404, media: mediaProblem, api: true},
	{name: "internal wrong method", internal: true, method: http.MethodPost, path: health.PathLiveness, status: 405, media: mediaProblem, api: true},
}

// headerHarness mounts the routes that produce the public kinds, with publicURL and a JSON body
// limit small enough for the 413 row.
func headerHarness(t *testing.T, publicURL string) *httpservertest.Harness {
	t.Helper()
	get := getThing()
	del := httpservertest.Op("deleteThing", http.MethodDelete, thingPath, "thing.delete")
	create := httpservertest.Op("createThing", http.MethodPost, "/api/v1/things", "thing.create")
	contract := strings.Replace(string(httpservertest.Contract), "    patch:\n      operationId: updateThing",
		"    delete:\n      operationId: deleteThing\n      responses:\n        '204':\n          description: ok\n    patch:\n      operationId: updateThing", 1)
	cfg := httpserver.DefaultChainConfig(publicURL)
	cfg.MaxJSONBodyBytes = 128
	cfg.HSTSMaxAge = testHSTSMaxAge
	return httpservertest.New(t, httpservertest.Options{Config: &cfg, Contract: []byte(contract), Routes: []httpservertest.Route{
		{Op: get, Handler: func(w http.ResponseWriter, r *http.Request) {
			switch {
			case r.Header.Get("X-Mode") == "slow":
				// No authorize check: the response writer answers 500 internal_error.
				httpservertest.JSON(w, http.StatusOK, map[string]any{"ok": true})
			case r.URL.Query().Get("limit") == "3":
				httpserver.WriteProblem(w, r, errs.New(errs.CodeUnavailable, ""))
			default:
				okHandler(get.Action)(w, r)
			}
		}},
		{Op: del, Handler: httpservertest.Authorized(del.Action, func(w http.ResponseWriter, _ *http.Request) {
			// A handler that tries to send a version and to weaken the base headers.
			w.Header().Set("Server", "leak/1.0")
			w.Header().Del("X-Content-Type-Options")
			w.Header().Set("Strict-Transport-Security", "max-age=1; includeSubDomains")
			w.WriteHeader(http.StatusNoContent)
		})},
		{Op: create, Handler: func(http.ResponseWriter, *http.Request) { panic("x") }},
	}})
}

// TestSECT03_HeadersPerResponseKind holds SEC-HDR-1 and SEC-HDR-2 on every response kind of
// design note 7 of brief M1-21, on both listeners, once with an https and once with an http
// server.public_url: HSTS is present exactly with https, no response carries Server.
func TestSECT03_HeadersPerResponseKind(t *testing.T) {
	t.Parallel()
	for _, publicURL := range []string{"https://chat.example.com", "http://127.0.0.1:8080"} {
		h := headerHarness(t, publicURL)
		sec := httpserver.SecurityHeadersFor(mustURL(t, publicURL), testHSTSMaxAge)
		for _, k := range responseKinds {
			var (
				status int
				hd     http.Header
			)
			if k.internal {
				reg := health.NewRegistry()
				if k.started {
					reg.SetStarted()
				}
				rec := httptest.NewRecorder()
				req := httptest.NewRequestWithContext(t.Context(), k.method, k.path, nil)
				httpserver.InternalHandler(httpserver.Internal(reg), sec).ServeHTTP(rec, req)
				status, hd = rec.Code, rec.Header()
			} else {
				resp := h.Do(k.request(h))
				status, hd = resp.StatusCode, resp.Header
			}
			name := k.name + " " + publicURL
			if status != k.status {
				t.Errorf("%s: status %d, want %d", name, status, k.status)
			}
			want := map[string]string{}
			for n, v := range hdr1 {
				want[n] = v
			}
			if k.api {
				for n, v := range hdr2 {
					want[n] = v
				}
			}
			for n, v := range want {
				if got := hd.Get(n); got != v {
					t.Errorf("%s: %s = %q, want %q", name, n, got, v)
				}
			}
			hsts := hd.Values("Strict-Transport-Security")
			if strings.HasPrefix(publicURL, "https:") {
				if len(hsts) != 1 || hsts[0] != "max-age="+strconv.Itoa(int(testHSTSMaxAge/time.Second)) {
					t.Errorf("%s: Strict-Transport-Security %q, want one max-age=3600", name, hsts)
				}
			} else if len(hsts) != 0 {
				t.Errorf("%s: Strict-Transport-Security %q with an http public URL", name, hsts)
			}
			if _, ok := hd["Server"]; ok {
				t.Errorf("%s: Server %q", name, hd.Get("Server"))
			}
			if k.media != "" && hd.Get("Content-Type") != k.media {
				t.Errorf("%s: Content-Type %q, want %q", name, hd.Get("Content-Type"), k.media)
			}
			if !k.internal && hd.Get("Correlation-Id") == "" {
				t.Errorf("%s: no Correlation-Id", name)
			}
			for n := range hd {
				if strings.HasPrefix(n, "Access-Control-") {
					t.Errorf("%s: CORS header %s without an allowed origin", name, n)
				}
			}
		}
	}
}

// TestHSTS_ValueAndCondition holds SEC-TLS-3: max-age is http.hsts_max_age in whole seconds,
// without includeSubDomains and preload, also 0; with an http public URL there is no header.
func TestHSTS_ValueAndCondition(t *testing.T) {
	t.Parallel()
	cases := []struct {
		url    string
		maxAge time.Duration
		want   string
	}{
		{"https://chat.example.com", 31536000 * time.Second, "max-age=31536000"},
		{"https://chat.example.com:8443", 0, "max-age=0"},
		{"https://[2001:db8::1]", 90 * time.Second, "max-age=90"},
		{"http://127.0.0.1:8080", time.Hour, ""},
		{"http://chat.example.com", time.Hour, ""},
	}
	for _, c := range cases {
		h := http.Header{}
		h.Set("Server", "bothy/1.2.3")
		httpserver.SecurityHeadersFor(mustURL(t, c.url), c.maxAge).Apply(h)
		got := h.Values("Strict-Transport-Security")
		switch {
		case c.want == "" && len(got) != 0:
			t.Errorf("%s: Strict-Transport-Security %q, want none", c.url, got)
		case c.want != "" && (len(got) != 1 || got[0] != c.want):
			t.Errorf("%s: Strict-Transport-Security %q, want %q", c.url, got, c.want)
		}
		if h.Get("X-Content-Type-Options") != "nosniff" || h.Get("Referrer-Policy") != "no-referrer" || h.Get("Server") != "" {
			t.Errorf("%s: base headers %v", c.url, h)
		}
	}
	var zero httpserver.SecurityHeaders
	h := http.Header{}
	zero.Apply(h)
	if h.Get("Strict-Transport-Security") != "" || h.Get("X-Content-Type-Options") != "nosniff" {
		t.Errorf("zero SecurityHeaders: %v", h)
	}
}

// TestHSTS_ChainUsesConfig holds that the chain takes HSTS from ChainConfig: server.public_url
// and http.hsts_max_age, also on a problem of a recovered panic, whose headers are rebuilt.
func TestHSTS_ChainUsesConfig(t *testing.T) {
	t.Parallel()
	h := headerHarness(t, "https://chat.example.com")
	resp := h.Do(h.NewRequest(http.MethodPost, "/api/v1/things", `{"name":"ab","color":"red"}`))
	if got := resp.Header.Get("Strict-Transport-Security"); got != "max-age=3600" {
		t.Fatalf("panic problem: Strict-Transport-Security %q", got)
	}
	cfg := httpserver.DefaultChainConfig("https://chat.example.com")
	if cfg.HSTSMaxAge != 31536000*time.Second {
		t.Fatalf("DefaultChainConfig HSTSMaxAge %s, want the chapter default 31536000s", cfg.HSTSMaxAge)
	}
}
