// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"math"
	"slices"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/platform/otel/metrics"
	"github.com/OWNER/bothy/services/server/internal/stats/domain"
)

func ptr[T any](v T) *T { return &v }

// closeCodes are the close codes of 04 6.4 the tests pass (the gateway's list in production).
var closeCodes = map[int]bool{1000: true, 1001: true, 4009: true, 4013: true}

// batch is a batch of an app with the entries.
func batch(entries ...domain.Entry) domain.Batch {
	return domain.Batch{Platform: "ios", AppVersion: "1.4.0", Density: "touch", Theme: "dark", Entries: entries}
}

// TestFilter_DiscardsEachBadEntryAlone: an unknown name, a wrong kind, a bound outside 4.7, a
// repeated bound, a negative count, a negative or missing counter value and a non-finite gauge are
// each discarded with their reason, and the rest of the batch is accepted (signals.md 4.6a,
// brief M4-31 Tests "Filter").
func TestFilter_DiscardsEachBadEntryAlone(t *testing.T) {
	good := []domain.Entry{
		{Name: "client.thread_open.duration", Kind: "histogram", Buckets: []domain.Bucket{{LE: 0.1, Count: 12}, {LE: 0.25, Count: 3}}},
		{Name: "client.frames", Kind: "counter", Value: ptr(1200.0)},
		{Name: "client.outbox.depth", Kind: "gauge", Value: ptr(math.Copysign(0, -1))},
		{Name: "client.push.placeholder", Kind: "counter", Value: ptr(0.0)},
	}
	bad := []struct {
		e      domain.Entry
		reason domain.Reason
	}{
		{domain.Entry{Name: "client.screen_views", Kind: "counter", Value: ptr(1.0)}, domain.ReasonUnknownName},
		{domain.Entry{Name: "client.frames", Kind: "gauge", Value: ptr(1.0)}, domain.ReasonKindMismatch},
		{domain.Entry{Name: "client.cold_start.duration", Kind: "counter", Value: ptr(1.0)}, domain.ReasonKindMismatch},
		{domain.Entry{Name: "client.cold_start.duration", Kind: "histogram", Buckets: []domain.Bucket{{LE: 0.3, Count: 1}}}, domain.ReasonInvalidBounds},
		{domain.Entry{Name: "client.cold_start.duration", Kind: "histogram", Buckets: []domain.Bucket{{LE: 1, Count: 1}, {LE: 1, Count: 2}}}, domain.ReasonInvalidBounds},
		{domain.Entry{Name: "client.cold_start.duration", Kind: "histogram"}, domain.ReasonInvalidBounds},
		{domain.Entry{Name: "client.catchup.duration", Kind: "histogram", Buckets: []domain.Bucket{{LE: 1, Count: -1}}}, domain.ReasonInvalidValue},
		{domain.Entry{Name: "client.resync", Kind: "counter", Value: ptr(-1.0)}, domain.ReasonInvalidValue},
		{domain.Entry{Name: "client.resync", Kind: "counter"}, domain.ReasonInvalidValue},
		{domain.Entry{Name: "client.resync", Kind: "counter", Value: ptr(math.Inf(1))}, domain.ReasonInvalidValue},
		{domain.Entry{Name: "client.outbox.depth", Kind: "gauge", Value: ptr(math.NaN())}, domain.ReasonInvalidValue},
		{domain.Entry{Name: "client.outbox.depth", Kind: "gauge"}, domain.ReasonInvalidValue},
	}
	entries := slices.Clone(good)
	want := map[domain.Reason]int{}
	for _, b := range bad {
		entries = append(entries, b.e)
		want[b.reason]++
	}
	r := domain.Filter(batch(entries...), closeCodes)
	if len(r.Accepted) != len(good) {
		t.Fatalf("accepted %d entries, want %d: %+v", len(r.Accepted), len(good), r.Accepted)
	}
	for i, m := range r.Accepted {
		if m.Name != good[i].Name || string(m.Kind) != good[i].Kind {
			t.Errorf("accepted[%d] = %s %s, want %s %s", i, m.Name, m.Kind, good[i].Name, good[i].Kind)
		}
		if m.Attrs != (domain.Attrs{Platform: "ios", Density: "touch", Theme: "dark", AppMajor: "1"}) {
			t.Errorf("accepted[%d] attributes %+v", i, m.Attrs)
		}
	}
	if !maps(r.Discarded, want) {
		t.Errorf("discarded %v, want %v", r.Discarded, want)
	}
	if got := r.Accepted[0].Buckets; len(got) != 2 || got[0] != (domain.Bucket{LE: 0.1, Count: 12}) {
		t.Errorf("histogram buckets %v", got)
	}
	if r.Accepted[1].Value != 1200 {
		t.Errorf("counter value %v", r.Accepted[1].Value)
	}
}

func maps(a, b map[domain.Reason]int) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

// TestFilter_CloseCode: close_code is kept only for client.reconnects, and a code outside 04 6.4
// or a missing one becomes "other"; on another metric it is dropped.
func TestFilter_CloseCode(t *testing.T) {
	r := domain.Filter(batch(
		domain.Entry{Name: "client.reconnects", Kind: "counter", CloseCode: ptr(4009), Value: ptr(2.0)},
		domain.Entry{Name: "client.reconnects", Kind: "counter", CloseCode: ptr(4999), Value: ptr(1.0)},
		domain.Entry{Name: "client.reconnects", Kind: "counter", Value: ptr(1.0)},
		domain.Entry{Name: "client.resync", Kind: "counter", CloseCode: ptr(4009), Value: ptr(1.0)},
	), closeCodes)
	got := []string{}
	for _, m := range r.Accepted {
		got = append(got, m.CloseCode)
	}
	if !slices.Equal(got, []string{"4009", "other", "other", ""}) {
		t.Errorf("close codes %q", got)
	}
}

// TestAppMajor: the major version of app_version, "other" outside ^[0-9]{1,2}$ (signals.md
// 4.6a); a leading zero is dropped so the value lies in the closed set 0 to 99.
func TestAppMajor(t *testing.T) {
	for in, want := range map[string]string{
		"1.4.0": "1", "12.0.3": "12", "07.1": "7", "0.0.0": "0", "99": "99", "100.0.0": "other",
		"": "other", "v1.2": "other", "1a.0": "other", "cnryabc.1": "other", "-1.0": "other", "\x201.0": "other",
	} {
		if got := domain.AppMajor(in); got != want {
			t.Errorf("AppMajor(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestFilter_AttributesOnlyFromTheClosedLists: platform, density and theme outside their lists
// become "other" or absent; nothing of the batch besides them reaches a measurement.
func TestFilter_AttributesOnlyFromTheClosedLists(t *testing.T) {
	b := domain.Batch{Platform: "beos", AppVersion: "cnry0123456789abcdef0123.0", Density: "", Theme: "sepia",
		Entries: []domain.Entry{{Name: "client.frames", Kind: "counter", Value: ptr(1.0)}}}
	r := domain.Filter(b, closeCodes)
	if len(r.Accepted) != 1 || r.Accepted[0].Attrs != (domain.Attrs{Platform: "other", Density: "", Theme: "other", AppMajor: "other"}) {
		t.Fatalf("attributes %+v", r.Accepted)
	}
}

// TestCatalogue_EqualsTheRegistry: the names and kinds of the closed list are the app metrics of
// the metric registry (which TestCatalogue_RegistryMatchesSignalsDoc holds equal to signals.md
// 4.6a), and the bounds are the registry's row "durations up to seconds" (4.7).
func TestCatalogue_EqualsTheRegistry(t *testing.T) {
	kinds := map[metrics.Kind]domain.Kind{metrics.KindHistogram: domain.KindHistogram, metrics.KindCounter: domain.KindCounter, metrics.KindGauge: domain.KindGauge}
	specs := metrics.Client()
	if len(domain.Names()) != len(specs) {
		t.Fatalf("names %v, registry has %d", domain.Names(), len(specs))
	}
	for _, s := range specs {
		k, ok := domain.KindOf(s.Name[len("bothy."):])
		if !ok || k != kinds[s.Kind] {
			t.Errorf("%s: kind %q %v, registry %s", s.Name, k, ok, s.Kind)
		}
	}
	if !slices.Equal(domain.HistogramBounds(), metrics.BucketsSeconds) {
		t.Errorf("bounds %v, registry %v", domain.HistogramBounds(), metrics.BucketsSeconds)
	}
}
