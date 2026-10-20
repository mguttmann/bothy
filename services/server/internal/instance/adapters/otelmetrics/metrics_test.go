// SPDX-License-Identifier: Apache-2.0

package otelmetrics_test

import (
	"slices"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/instance/adapters/otelmetrics"
	"github.com/OWNER/bothy/services/server/internal/instance/domain"
	"github.com/OWNER/bothy/services/server/internal/platform/otel/metrics"
	"github.com/OWNER/bothy/services/server/internal/platform/otel/metrics/metricstest"
)

// TestCSPViolations_DirectivesEqualDomain: the closed set of bothy.csp.directive is the list the
// report parser keeps, so no accepted directive records as other.
func TestCSPViolations_DirectivesEqualDomain(t *testing.T) {
	var got []string
	for _, a := range metrics.ConsoleCSPViolations.Attrs {
		if a.Key == "bothy.csp.directive" {
			got = slices.Sorted(slices.Values(a.Values))
		}
	}
	if want := slices.Sorted(slices.Values(domain.CSPDirectives())); !slices.Equal(got, want) {
		t.Fatalf("metric directives %v, domain %v", got, want)
	}
	metricstest.Strict(t)
	mp, reader := metricstest.Provider(t)
	c, err := otelmetrics.NewCSPViolations(mp)
	if err != nil {
		t.Fatal(err)
	}
	c.Count(t.Context(), "script-src-elem", "enforce", 1)
	m := metricstest.Find(t, reader, "bothy.console.csp_violations")
	if n := metricstest.Sum(m, map[string]string{"bothy.csp.directive": "script-src-elem", "bothy.csp.disposition": "enforce"}); n != 1 {
		t.Fatalf("count %v, want 1", n)
	}
}
