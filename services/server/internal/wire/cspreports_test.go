// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"context"
	"log/slog"
	"maps"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/api/routes"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
	"github.com/OWNER/bothy/services/server/internal/platform/otel/metrics/metricstest"
)

// cspCanary stands in for content a report may carry: a path of the document URL, a query of the
// blocked URL and the script sample. It must never reach a log record.
const cspCanary = "cspcanary7f3a"

// publicAllow is an Authorizer that allows every action and records the check, as the authz
// module does for the public action console.csp_report (05 2.7).
type publicAllow struct{ authorize.Authorizer }

func (publicAllow) Authorize(ctx context.Context, _ authorize.Principal, a authorize.Action, _ authorize.Resource) (authorize.Decision, error) {
	authorize.RecordCheck(ctx, authorize.Check{Action: string(a), Kind: authorize.CheckAuthorize, Allowed: true})
	return authorize.Decision{Outcome: authorize.OutcomeAllow}, nil
}

// TestCspReport serves submitCspReport over the real contract, chain and rate limits (M6-03,
// R-AZ-33): both formats are accepted with 204 without credentials, an oversize body is 413, the
// class rl.public_auth refuses after its burst, and the log records carry only the directive, the
// disposition and the blocked origin; the canary of the document URL, the query and the sample
// never appears in the log capture.
func TestCspReport(t *testing.T) {
	logs := &captureHandler{}
	log := slog.New(logs)
	cfg := httpserver.DefaultChainConfig("https://chat.example.com")
	cfg.MaxJSONBodyBytes = 2048
	chain, err := NewChain(cfg, nil, log)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := InstallRateLimits(chain, loadValues(t, "BOTHY_RATELIMIT__RL__PUBLIC_AUTH__BURST=4"), routes.Enabled(), clock.NewFake(time.Unix(0, 0))); err != nil {
		t.Fatal(err)
	}
	metricstest.Strict(t)
	mp, reader := metricstest.Provider(t)
	reports, err := NewCSPReports(publicAllow{}, log, mp, 20)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	if err := MountRoutes(RoleAPI, mux, Handlers{CSPReports: reports}, routes.SpecCatalog(), chain, log); err != nil {
		t.Fatal(err)
	}
	h := chain.Public(mux)
	post := func(media, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, httpserver.CSPReportPath, strings.NewReader(body))
		req.Header.Set("Content-Type", media)
		req.RemoteAddr = "192.0.2.1:4321"
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}
	modern := `[{"type":"csp-violation","url":"https://chat.example.com/console/` + cspCanary + `","body":{"documentURL":"https://chat.example.com/console/` + cspCanary +
		`","blockedURL":"https://cdn.example.com/x.js?q=` + cspCanary + `","effectiveDirective":"script-src-elem","disposition":"enforce","sample":"` + cspCanary + `","sourceFile":"https://chat.example.com/` + cspCanary + `.js","lineNumber":7}}]`
	legacy := `{"csp-report":{"document-uri":"https://chat.example.com/console/` + cspCanary + `","blocked-uri":"inline","effective-directive":"style-src-elem","disposition":"report","script-sample":"` + cspCanary + `"}}`
	if rec := post("application/reports+json", modern); rec.Code != http.StatusNoContent {
		t.Fatalf("Reporting API: %d %s", rec.Code, rec.Body)
	}
	if rec := post("application/csp-report", legacy); rec.Code != http.StatusNoContent {
		t.Fatalf("legacy: %d %s", rec.Code, rec.Body)
	}
	if rec := post("application/reports+json", `[{"type":"csp-violation","body":{"sample":"`+strings.Repeat("x", 3000)+`"}}]`); rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversize: %d %s", rec.Code, rec.Body)
	}
	if rec := post("application/json", `[]`); rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("other media type: %d %s", rec.Code, rec.Body)
	}
	// The burst is 4 per client IP; the refused 413 and 415 may or may not have spent a token, so
	// at most two more reports pass before the class refuses.
	limited := false
	for range 3 {
		if rec := post("application/csp-report", `{"csp-report":{}}`); rec.Code == http.StatusTooManyRequests {
			limited = true
			break
		}
	}
	if !limited {
		t.Fatal("rl.public_auth never refused beyond its burst")
	}

	var got []map[string]string
	logs.mu.Lock()
	for _, r := range logs.records {
		for k, v := range r.attrs {
			if strings.Contains(v, cspCanary) || strings.Contains(k, cspCanary) || strings.Contains(r.msg, cspCanary) {
				t.Errorf("log record %q carries content: %s=%s", r.msg, k, v)
			}
		}
		if r.msg == "csp violation reported" {
			if r.level != slog.LevelInfo {
				t.Errorf("level %v, want info", r.level)
			}
			got = append(got, r.attrs)
		}
	}
	logs.mu.Unlock()
	want := []map[string]string{
		{"bothy.csp.directive": "script-src-elem", "bothy.csp.disposition": "enforce", "bothy.csp.blocked_origin": "https://cdn.example.com"},
		{"bothy.csp.directive": "style-src-elem", "bothy.csp.disposition": "report", "bothy.csp.blocked_origin": "inline"},
	}
	if len(got) > len(want) {
		got = got[:len(want)] // the empty legacy reports of the rate limit loop
	}
	if !slices.EqualFunc(got, want, maps.Equal) {
		t.Fatalf("records %v, want %v", got, want)
	}
	m := metricstest.Find(t, reader, "bothy.console.csp_violations")
	if n := metricstest.Sum(m, map[string]string{"bothy.csp.directive": "script-src-elem", "bothy.csp.disposition": "enforce"}); n != 1 {
		t.Errorf("counter %v, want 1", n)
	}
}

// TestCspReport_LogRecordsAreBounded: a report body of many violations writes one log record per
// distinct violation and at most console.csp_report_max_records of them, while the counter counts
// every violation; abuse within the rate limit raises a counter, not the log volume (brief M6-03
// Design note 4).
func TestCspReport_LogRecordsAreBounded(t *testing.T) {
	logs := &captureHandler{}
	log := slog.New(logs)
	chain, err := NewChain(httpserver.DefaultChainConfig("https://chat.example.com"), nil, log)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := InstallRateLimits(chain, loadValues(t), routes.Enabled(), clock.NewFake(time.Unix(0, 0))); err != nil {
		t.Fatal(err)
	}
	metricstest.Strict(t)
	mp, reader := metricstest.Provider(t)
	reports, err := NewCSPReports(publicAllow{}, log, mp, 5)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	if err := MountRoutes(RoleAPI, mux, Handlers{CSPReports: reports}, routes.SpecCatalog(), chain, log); err != nil {
		t.Fatal(err)
	}
	h := chain.Public(mux)
	var items []string
	for i := range 2000 {
		items = append(items, `{"type":"csp-violation","body":{"effectiveDirective":"img-src","disposition":"enforce","blockedURL":"https://h`+strconv.Itoa(i%20)+`.example.com/x"}}`)
	}
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, httpserver.CSPReportPath, strings.NewReader("["+strings.Join(items, ",")+"]"))
	req.Header.Set("Content-Type", "application/reports+json")
	req.RemoteAddr = "192.0.2.1:4321"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status %d %s", rec.Code, rec.Body)
	}
	n := 0
	logs.mu.Lock()
	for _, r := range logs.records {
		if r.msg == "csp violation reported" {
			n++
		}
	}
	logs.mu.Unlock()
	if n != 5 {
		t.Errorf("%d log records, want 5 (console.csp_report_max_records)", n)
	}
	m := metricstest.Find(t, reader, "bothy.console.csp_violations")
	if c := metricstest.Sum(m, map[string]string{"bothy.csp.directive": "img-src", "bothy.csp.disposition": "enforce"}); c != 2000 {
		t.Errorf("counter %v, want 2000", c)
	}
}
