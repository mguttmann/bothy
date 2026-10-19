// SPDX-License-Identifier: Apache-2.0

package metrics_test

import (
	"context"
	"math"
	"slices"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/platform/otel/metrics"
	"github.com/OWNER/bothy/services/server/internal/platform/otel/metrics/metricstest"
)

// TestClient_SpecsPassCheck: the app metrics carry only closed sets and no person or ID key (17
// rule 8, the catalogue test of M1-68 extended to the new instruments), the histograms the 4.7 row
// "durations up to seconds", and app majors 0 to 99.
func TestClient_SpecsPassCheck(t *testing.T) {
	if problems := metrics.Check(append(metrics.Client(), metrics.ClientMetricsDiscarded)); len(problems) > 0 {
		t.Fatalf("Check: %v", problems)
	}
	for _, s := range metrics.Client() {
		for _, a := range s.Attrs {
			if metrics.PersonOrIDKey(a.Key) {
				t.Errorf("%s: %s names a person or an ID", s.Name, a.Key)
			}
		}
		if s.Kind == metrics.KindHistogram && !slices.Equal(s.Buckets, metrics.BucketsSeconds) {
			t.Errorf("%s: bounds %v", s.Name, s.Buckets)
		}
	}
	if m := metrics.ClientAppMajors(); len(m) != 100 || m[0] != "0" || m[99] != "99" {
		t.Errorf("ClientAppMajors = %v", m)
	}
}

// TestCounter_AddFloat: a float counter adds non-negative finite values and ignores the rest.
func TestCounter_AddFloat(t *testing.T) {
	metricstest.Strict(t)
	mp, reader := metricstest.Provider(t)
	c, err := metrics.NewCounter(mp, metrics.ClientFrames, nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	for _, v := range []float64{120, 0.5, -3, math.NaN(), math.Inf(1)} {
		c.AddFloat(ctx, v, "macos", "comfortable", "light", "1")
	}
	points := metricstest.Points(metricstest.Find(t, reader, "bothy.client.frames"))
	if len(points) != 1 || points[0].Value != 120.5 {
		t.Errorf("points %v, want one with 120.5", points)
	}
}
