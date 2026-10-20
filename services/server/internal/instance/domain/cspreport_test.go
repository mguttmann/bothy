// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"reflect"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/instance/domain"
)

// TestParseCSPReports: both formats yield only the directive, the disposition and the origin of
// the blocked URL; paths, queries, the document URL and the sample never come through.
func TestParseCSPReports(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		format domain.CSPReportFormat
		body   string
		want   []domain.CSPViolation
	}{
		{"reporting api", domain.CSPReportingAPI, `[
			{"type":"csp-violation","url":"https://chat.example.com/console/secret-path","body":{"documentURL":"https://chat.example.com/console/secret-path","blockedURL":"https://cdn.example.com:8443/lib/x.js?token=abc","effectiveDirective":"script-src-elem","disposition":"enforce","sample":"alert(1)"}},
			{"type":"deprecation","body":{"id":"x"}},
			{"type":"csp-violation","body":{"blockedURL":"inline","effectiveDirective":"style-src-attr","disposition":"report"}},
			{"type":"csp-violation","body":{"blockedURL":"trusted-types-policy","effectiveDirective":"trusted-types","disposition":"enforce"}},
			"not an object",
			{"type":"csp-violation","body":"not an object"}
		]`, []domain.CSPViolation{
			{Directive: "script-src-elem", Disposition: "enforce", BlockedOrigin: "https://cdn.example.com:8443"},
			{Directive: "style-src-attr", Disposition: "report", BlockedOrigin: "inline"},
			{Directive: "trusted-types", Disposition: "enforce", BlockedOrigin: "trusted-types-policy"},
		}},
		{"legacy", domain.CSPLegacy, `{"csp-report":{"document-uri":"https://chat.example.com/console/x","blocked-uri":"eval","violated-directive":"script-src 'self'","disposition":"enforce","script-sample":"x"}}`,
			[]domain.CSPViolation{{Directive: "script-src", Disposition: "enforce", BlockedOrigin: "eval"}}},
		{"legacy effective wins", domain.CSPLegacy, `{"csp-report":{"blocked-uri":"wss://chat.example.com/api/v1/gateway","effective-directive":"connect-src","violated-directive":"default-src"}}`,
			[]domain.CSPViolation{{Directive: "connect-src", Disposition: "other", BlockedOrigin: "wss://chat.example.com"}}},
		{"unknown values become other", domain.CSPReportingAPI, `[{"type":"csp-violation","body":{"blockedURL":"javascript:alert(1)","effectiveDirective":"evil directive","disposition":"maybe"}},
			{"type":"csp-violation","body":{"blockedURL":"https://alex:pw@example.com/","effectiveDirective":"img-src"}},
			{"type":"csp-violation","body":{"blockedURL":"data:image/png;base64,AAAA","effectiveDirective":"img-src"}},
			{"type":"csp-violation","body":{"blockedURL":"blob:https://chat.example.com/1234","effectiveDirective":"worker-src"}},
			{"type":"csp-violation","body":{"effectiveDirective":"font-src"}}]`,
			[]domain.CSPViolation{
				{Directive: "other", Disposition: "other", BlockedOrigin: "other"},
				{Directive: "img-src", Disposition: "other", BlockedOrigin: "https://example.com"},
				{Directive: "img-src", Disposition: "other", BlockedOrigin: "data"},
				{Directive: "worker-src", Disposition: "other", BlockedOrigin: "blob"},
				{Directive: "font-src", Disposition: "other", BlockedOrigin: "other"},
			}},
	}
	for _, c := range cases {
		got, err := domain.ParseCSPReports(c.format, []byte(c.body))
		if err != nil {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s:\n got %+v\nwant %+v", c.name, got, c.want)
		}
	}
	for _, bad := range []struct {
		format domain.CSPReportFormat
		body   string
	}{{domain.CSPReportingAPI, `{"type":"csp-violation"}`}, {domain.CSPLegacy, `[]`}, {domain.CSPLegacy, `{"csp-report":1}`}, {domain.CSPReportingAPI, `nope`}} {
		if _, err := domain.ParseCSPReports(bad.format, []byte(bad.body)); err == nil {
			t.Errorf("%s accepted", bad.body)
		}
	}
}

// TestSummarizeCSPViolations: identical violations become one entry with their count, in the
// order they first appear, so a large report body costs one log record per distinct violation.
func TestSummarizeCSPViolations(t *testing.T) {
	t.Parallel()
	a := domain.CSPViolation{Directive: "script-src-elem", Disposition: "enforce", BlockedOrigin: "inline"}
	b := domain.CSPViolation{Directive: "img-src", Disposition: "report", BlockedOrigin: "https://example.com"}
	got := domain.SummarizeCSPViolations([]domain.CSPViolation{a, b, a, a, b})
	want := []domain.CSPViolationCount{{CSPViolation: a, Count: 3}, {CSPViolation: b, Count: 2}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
	if got := domain.SummarizeCSPViolations(nil); len(got) != 0 {
		t.Fatalf("empty input gave %+v", got)
	}
}
