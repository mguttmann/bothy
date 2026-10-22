// SPDX-License-Identifier: Apache-2.0

package metrics_test

import (
	"context"
	"slices"
	"strings"
	"testing"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/metric"

	"github.com/OWNER/bothy/services/server/internal/platform/otel/metrics"
	"github.com/OWNER/bothy/services/server/internal/platform/otel/metrics/metricstest"
)

// fixture returns a valid counter spec with one attribute a.
func fixture(a metrics.Attr) metrics.Spec {
	return metrics.Spec{Name: "bothy.fixture", Kind: metrics.KindCounter, Unit: "{x}", Description: "fixture", Attrs: []metrics.Attr{a}}
}

// mentions reports whether one of errs contains every part.
func mentions(errs []error, parts ...string) bool {
	return slices.ContainsFunc(errs, func(err error) bool {
		for _, p := range parts {
			if !strings.Contains(err.Error(), p) {
				return false
			}
		}
		return true
	})
}

func TestCheck_OpenAttributeFails(t *testing.T) {
	if errs := metrics.Check([]metrics.Spec{fixture(metrics.Attr{Key: "bothy.reason", Values: []string{"a"}})}); len(errs) != 0 {
		t.Fatalf("a closed literal set fails: %v", errs)
	}
	for name, a := range map[string]metrics.Attr{
		"no set":         {Key: "bothy.reason"},
		"unknown source": {Key: "bothy.reason", Source: "user_input"},
	} {
		t.Run(name, func(t *testing.T) {
			if errs := metrics.Check([]metrics.Spec{fixture(a)}); !mentions(errs, "bothy.fixture", "bothy.reason") {
				t.Errorf("Check passed an open attribute: %v", errs)
			}
		})
	}
}

func TestCheck_PersonAttributeFails(t *testing.T) {
	for _, key := range []string{
		"bothy.thread.id", "bothy.webhook_subscription.id", "bothy.principal.pseudo_id", "bothy.correlation_id",
		"bothy.event.seq", "user.name", "enduser.id", "client.address", "session.id", "bothy.message_id",
	} {
		t.Run(key, func(t *testing.T) {
			errs := metrics.Check([]metrics.Spec{fixture(metrics.Attr{Key: key, Values: []string{"x"}})})
			if !mentions(errs, key, "person") {
				t.Errorf("Check passed %s: %v", key, errs)
			}
		})
	}
}

func TestCheck_HistogramWithoutBucketsFails(t *testing.T) {
	h := metrics.Spec{Name: "bothy.fixture.duration", Kind: metrics.KindHistogram, Unit: "s", Description: "fixture"}
	if errs := metrics.Check([]metrics.Spec{h}); !mentions(errs, "bothy.fixture.duration", "without bucket bounds") {
		t.Errorf("Check passed a histogram without bounds: %v", errs)
	}
	h.Buckets = []float64{1, 0.5}
	if errs := metrics.Check([]metrics.Spec{h}); !mentions(errs, "not ascending") {
		t.Errorf("Check passed descending bounds: %v", errs)
	}
	c := fixture(metrics.Attr{Key: "bothy.reason", Values: []string{"a"}})
	c.Buckets = []float64{1}
	if errs := metrics.Check([]metrics.Spec{c}); !mentions(errs, "bucket bounds on a Counter") {
		t.Errorf("Check passed bounds on a counter: %v", errs)
	}
	if errs := metrics.Check([]metrics.Spec{c, c}); !mentions(errs, "declared twice") {
		t.Errorf("Check passed a duplicate: %v", errs)
	}
}

// TestViews_BucketBounds: every histogram of the registry aggregates with the bounds of its 4.7
// row, also when the creating library advises other bounds (otelhttp, otelpgx-like advice).
func TestViews_BucketBounds(t *testing.T) {
	if metrics.LibraryOtelHTTP != otelhttp.ScopeName {
		t.Fatalf("LibraryOtelHTTP %q differs from otelhttp.ScopeName %q", metrics.LibraryOtelHTTP, otelhttp.ScopeName)
	}
	mp, reader := metricstest.Provider(t)
	for _, s := range metrics.All() {
		if s.Kind != metrics.KindHistogram {
			continue
		}
		scope := metrics.Scope
		if s.Library != "" {
			scope = s.Library
		}
		h, err := mp.Meter(scope).Float64Histogram(s.Name, metric.WithExplicitBucketBoundaries(0.001, 0.5, 1000))
		if err != nil {
			t.Fatal(err)
		}
		h.Record(context.Background(), 0.3)
	}
	got := metricstest.Collect(t, reader)
	for _, s := range metrics.All() {
		if s.Kind != metrics.KindHistogram {
			continue
		}
		m, ok := got[s.Name]
		if !ok {
			t.Errorf("%s: not collected", s.Name)
			continue
		}
		pts := metricstest.Points(m)
		if len(pts) != 1 || !slices.Equal(pts[0].Bounds, s.Buckets) {
			t.Errorf("%s: bounds %v, want %v", s.Name, pts, s.Buckets)
		}
	}
}

// TestViews_DropsWhatTheCatalogueDoesNotName: instruments of otelpgx and instruments outside the
// catalogue never reach a reader.
func TestViews_DropsWhatTheCatalogueDoesNotName(t *testing.T) {
	mp, reader := metricstest.Provider(t)
	ctx := context.Background()
	pgx, _ := mp.Meter(metrics.LibraryOtelPGX).Float64Histogram("db.client.operation.duration")
	pgx.Record(ctx, 0.1)
	body, _ := mp.Meter(otelhttp.ScopeName).Int64Histogram("http.server.request.body.size")
	body.Record(ctx, 10)
	own, _ := mp.Meter(metrics.Scope).Int64Counter("bothy.not.in.catalogue")
	own.Add(ctx, 1)
	if got := metricstest.Collect(t, reader); len(got) != 0 {
		t.Errorf("collected %v, want nothing", got)
	}
}

func TestRecord_UnknownValueMapsToOther(t *testing.T) {
	mp, reader := metricstest.Provider(t)
	c, err := metrics.NewCounter(mp, metrics.GatewayResync, nil)
	if err != nil {
		t.Fatal(err)
	}
	c.Add(context.Background(), 1, "retention")
	c.Add(context.Background(), 2, "a reason nobody declared")
	m := metricstest.Find(t, reader, "bothy.gateway.resync")
	if got := metricstest.Sum(m, map[string]string{"bothy.reason": "other"}); got != 2 {
		t.Errorf("other = %v, want 2", got)
	}
	if got := metricstest.Sum(m, map[string]string{"bothy.reason": "retention"}); got != 1 {
		t.Errorf("retention = %v, want 1", got)
	}
	if n := len(metricstest.Points(m)); n != 2 {
		t.Errorf("%d series, want 2", n)
	}
}

func TestRecord_UnknownValueStrictFailsInTests(t *testing.T) {
	var reported []error
	restore := metrics.SetStrict(func(err error) { reported = append(reported, err) })
	defer restore()
	mp, _ := metricstest.Provider(t)
	c, err := metrics.NewCounter(mp, metrics.GatewayResync, nil)
	if err != nil {
		t.Fatal(err)
	}
	c.Add(context.Background(), 1, "retention")
	if len(reported) != 0 {
		t.Fatalf("a known value was reported: %v", reported)
	}
	c.Add(context.Background(), 1, "unknown")
	c.Add(context.Background(), 1)
	if !mentions(reported, "bothy.gateway.resync", `"unknown"`) || !mentions(reported, "takes 1 attribute values, got 0") {
		t.Errorf("strict switch reported %v", reported)
	}
}

func TestRecord_SourceValuesRequired(t *testing.T) {
	mp, _ := metricstest.Provider(t)
	if _, err := metrics.NewCounter(mp, metrics.RateLimitRejected, nil); err == nil {
		t.Error("NewCounter without the rate classes succeeded")
	}
	if _, err := metrics.NewHistogram(mp, metrics.RateLimitRejected, metrics.Values{metrics.SourceRateClasses: {"rl.read"}}); err == nil {
		t.Error("NewHistogram of a counter succeeded")
	}
	c, err := metrics.NewCounter(mp, metrics.RateLimitRejected, metrics.Values{metrics.SourceRateClasses: {"rl.read"}})
	if err != nil || c == nil {
		t.Fatalf("NewCounter with the classes: %v", err)
	}
}

func TestRecord_AgentHandlePattern(t *testing.T) {
	var reported []error
	restore := metrics.SetStrict(func(err error) { reported = append(reported, err) })
	defer restore()
	mp, reader := metricstest.Provider(t)
	// The handles are not passed: the pattern of 03 principals.handle is the closed set.
	c, err := metrics.NewCounter(mp, metrics.RunFinished, nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	c.Add(ctx, 1, "alex-bot", "completed", "")
	c.Add(ctx, 1, "sam.helper_2", "failed", "stale")
	if len(reported) != 0 {
		t.Fatalf("handles of the pattern were reported: %v", reported)
	}
	for _, bad := range []string{"Robin", "-robin", "robin bot", ""} {
		c.Add(ctx, 1, bad, "cancelled", "user_cancelled")
	}
	if len(reported) != 4 || !mentions(reported, "bothy.agent.handle", `"Robin"`) {
		t.Errorf("strict switch reported %v, want the four handles outside the pattern", reported)
	}
	m := metricstest.Find(t, reader, "bothy.run.finished")
	for handle, want := range map[string]float64{"alex-bot": 1, "sam.helper_2": 1, "other": 4} {
		if got := metricstest.Sum(m, map[string]string{"bothy.agent.handle": handle}); got != want {
			t.Errorf("%s = %v, want %v", handle, got, want)
		}
	}
	for _, p := range metricstest.Points(m) {
		if _, has := p.Attrs["bothy.reason"]; has == (p.Attrs["bothy.run.state"] == "completed") {
			t.Errorf("point %v: bothy.reason must be absent exactly for completed", p.Attrs)
		}
	}
}
