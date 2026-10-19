// SPDX-License-Identifier: Apache-2.0

package httpserver_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
)

// reportHarness serves submitReport (POST /api/v1/reports) with the body kind BodyReport and
// records the raw JSON body the handler sees through RawJSONBody.
func reportHarness(t *testing.T, cfg *httpserver.ChainConfig) (*httpservertest.Harness, *string) {
	t.Helper()
	op := httpservertest.Op("submitReport", http.MethodPost, "/api/v1/reports", "console.csp_report")
	op.RequestBody, op.Public = httpserver.BodyReport, true
	var got string
	h := httpservertest.New(t, httpservertest.Options{Config: cfg, Routes: []httpservertest.Route{{Op: op,
		Handler: httpservertest.Authorized(op.Action, func(w http.ResponseWriter, r *http.Request) {
			b, _ := httpserver.RawJSONBody(r.Context())
			got = string(b)
			w.WriteHeader(http.StatusNoContent)
		})}}})
	return h, &got
}

// TestBody_ReportKind: the body kind of CSP violation reports takes application/reports+json and
// application/csp-report, passes the JSON guard of SEC-IN-2 and the JSON limit, hands the raw body
// to the handler and refuses every other media type with 415.
func TestBody_ReportKind(t *testing.T) {
	t.Parallel()
	h, got := reportHarness(t, nil)
	for _, c := range []struct{ media, body string }{
		{"application/reports+json", `[{"type":"csp-violation","body":{"effectiveDirective":"script-src-elem"}}]`},
		{"application/csp-report", `{"csp-report":{"effective-directive":"script-src-elem"}}`},
		{"application/csp-report; charset=utf-8", `{"csp-report":{}}`},
	} {
		*got = ""
		resp := h.Do(h.NewRequest(http.MethodPost, "/api/v1/reports", c.body, "Content-Type", c.media))
		if resp.StatusCode != http.StatusNoContent || *got != c.body {
			t.Errorf("%s: %d, handler got %q", c.media, resp.StatusCode, *got)
		}
	}
	wantProblem(t, h.Do(h.NewRequest(http.MethodPost, "/api/v1/reports", `{}`, "Content-Type", "application/json")), 415, "unsupported_media_type")
	wantProblem(t, h.Do(h.NewRequest(http.MethodPost, "/api/v1/reports", `{"a":1,"a":2}`, "Content-Type", "application/csp-report")), 400, "malformed_request")

	cfg := httpserver.DefaultChainConfig(httpservertest.PublicURL)
	cfg.MaxJSONBodyBytes = 64
	small, _ := reportHarness(t, &cfg)
	big := `[{"type":"csp-violation","body":{"sample":"` + strings.Repeat("x", 100) + `"}}]`
	wantProblem(t, small.Do(small.NewRequest(http.MethodPost, "/api/v1/reports", big, "Content-Type", "application/reports+json")), 413, "payload_too_large")
}

// TestConsoleGuard_SkipsCSPReports: a violation report that the browser sends with the console
// cookie (the legacy report-uri format carries cookies of the same origin) has no
// X-Console-Request header; the CSRF guard lets the action console.csp_report pass, because it
// changes nothing and is public (R-AZ-33, docs/DECISIONS.md D-M6-03-5).
func TestConsoleGuard_SkipsCSPReports(t *testing.T) {
	t.Parallel()
	auth := httpservertest.NewActors()
	auth.SetCookie("cookie-alex", actor.Actor{ID: ids.New(), Kind: actor.KindUser, SessionID: ids.New(), DeviceID: ids.New(), ClientComponent: "console"})
	op := httpservertest.Op("submitReport", http.MethodPost, "/api/v1/reports", "console.csp_report")
	op.RequestBody, op.Public = httpserver.BodyReport, true
	h := httpservertest.New(t, httpservertest.Options{Authenticator: auth, Routes: []httpservertest.Route{{Op: op,
		Handler: httpservertest.Authorized(op.Action, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })}}})
	resp := h.Do(h.NewRequest(http.MethodPost, "/api/v1/reports", `{"csp-report":{}}`,
		"Content-Type", "application/csp-report", "Cookie", httpserver.ConsoleCookieHeader("cookie-alex")))
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("report with the console cookie: %d %s", resp.StatusCode, resp.Body)
	}
}
