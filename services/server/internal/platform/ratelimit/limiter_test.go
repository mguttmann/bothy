// SPDX-License-Identifier: Apache-2.0

package ratelimit_test

import (
	"context"
	"fmt"
	"net/netip"
	"runtime"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/ratelimit"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
)

const keyA = "p:0192f4a0-7b1c-7577-8bf0-c4f4ee4f887e"

func TestBucket_StartsFull(t *testing.T) {
	t.Parallel()
	l, _ := newLimiter(t, config(nil))
	d := allow(t, l, "rl.write", keyA)
	if !d.Allowed || d.Remaining != 29 || d.Limits != (ratelimit.Limits{PerMinute: 120, Burst: 30}) {
		t.Fatalf("first request %+v, want allowed with 29 left", d)
	}
}

func TestBucket_BurstThenDeny(t *testing.T) {
	t.Parallel()
	for _, lim := range []ratelimit.Limits{{PerMinute: 120, Burst: 30}, {PerMinute: 7, Burst: 3}, {PerMinute: 2, Burst: 2}, {PerMinute: 1200, Burst: 200}} {
		t.Run(fmt.Sprintf("%d_%d", lim.PerMinute, lim.Burst), func(t *testing.T) {
			t.Parallel()
			l, _ := newLimiter(t, config(map[ratelimit.Class]ratelimit.Limits{"rl.write": lim}))
			for i := range lim.Burst {
				if d := allow(t, l, "rl.write", keyA); !d.Allowed || d.Remaining != lim.Burst-1-i {
					t.Fatalf("request %d: %+v", i+1, d)
				}
			}
			d := allow(t, l, "rl.write", keyA)
			want := (60 + lim.PerMinute - 1) / lim.PerMinute // ceil(60 / per_minute)
			if d.Allowed || d.RetryAfterSeconds != want || d.Remaining != 0 {
				t.Fatalf("request after burst: %+v, want denied with Retry-After %d", d, want)
			}
		})
	}
}

func TestBucket_Refill(t *testing.T) {
	t.Parallel()
	l, clk := newLimiter(t, config(map[ratelimit.Class]ratelimit.Limits{"rl.write": {PerMinute: 60, Burst: 2}}))
	allow(t, l, "rl.write", keyA)
	allow(t, l, "rl.write", keyA)
	if allow(t, l, "rl.write", keyA).Allowed {
		t.Fatal("empty bucket allowed")
	}
	clk.Advance(999 * time.Millisecond)
	if d := allow(t, l, "rl.write", keyA); d.Allowed || d.RetryAfterSeconds != 1 {
		t.Fatalf("0.999 tokens: %+v", d)
	}
	clk.Advance(time.Millisecond)
	if d := allow(t, l, "rl.write", keyA); !d.Allowed || d.Remaining != 0 || d.ResetSeconds != 2 {
		t.Fatalf("one token refilled: %+v", d)
	}
}

func TestBucket_NeverAboveBurst(t *testing.T) {
	t.Parallel()
	l, clk := newLimiter(t, config(map[ratelimit.Class]ratelimit.Limits{"rl.write": {PerMinute: 60, Burst: 5}}))
	allow(t, l, "rl.write", keyA)
	clk.Advance(1000 * time.Hour)
	if d := allow(t, l, "rl.write", keyA); d.Remaining != 4 {
		t.Fatalf("after a long pause %+v, want 4 left (burst 5 minus this request)", d)
	}
	// A clock that jumps back refills nothing and takes nothing.
	clk.Advance(-time.Hour)
	if d := allow(t, l, "rl.write", keyA); !d.Allowed || d.Remaining != 3 {
		t.Fatalf("after the clock jumped back %+v", d)
	}
}

func TestHeaders_Format(t *testing.T) {
	t.Parallel()
	l, _ := newLimiter(t, config(nil))
	policy, state := ratelimit.HeaderValues("rl.write", allow(t, l, "rl.write", keyA))
	if policy != `"rl.write";q=120;w=60` || state != `"rl.write";r=29;t=1` {
		t.Fatalf("policy %q, state %q", policy, state)
	}
	policy, state = ratelimit.HeaderValues("rl.read", allow(t, l, "rl.read", keyA))
	if policy != `"rl.read";q=600;w=60` || state != `"rl.read";r=99;t=1` {
		t.Fatalf("policy %q, state %q", policy, state)
	}
}

func TestHeaders_RoundingFloorAndCeil(t *testing.T) {
	t.Parallel()
	// 7 per minute: one token every 60/7 = 8.571... seconds.
	l, clk := newLimiter(t, config(map[ratelimit.Class]ratelimit.Limits{"rl.write": {PerMinute: 7, Burst: 3}}))
	for range 3 {
		allow(t, l, "rl.write", keyA)
	}
	d := allow(t, l, "rl.write", keyA)
	// tokens 0: Retry-After ceil(8.571) = 9, t = ceil(3 * 8.571) = ceil(25.71) = 26.
	if d.Allowed || d.Remaining != 0 || d.RetryAfterSeconds != 9 || d.ResetSeconds != 26 {
		t.Fatalf("empty: %+v", d)
	}
	clk.Advance(10 * time.Second) // 1.1666 tokens
	d = allow(t, l, "rl.write", keyA)
	// 0.1666 tokens after the request: r = floor = 0, t = ceil((3 - 0.1666) * 8.571) = ceil(24.28) = 25.
	if !d.Allowed || d.Remaining != 0 || d.ResetSeconds != 25 {
		t.Fatalf("after 10s: %+v", d)
	}
	_, state := ratelimit.HeaderValues("rl.write", d)
	if state != `"rl.write";r=0;t=25` {
		t.Fatalf("state %q", state)
	}
	d = allow(t, l, "rl.write", keyA)
	// 0.1666 tokens, denied: Retry-After = ceil(0.8333 * 8.571) = ceil(7.14) = 8.
	if d.Allowed || d.RetryAfterSeconds != 8 {
		t.Fatalf("denied at 0.1666 tokens: %+v", d)
	}
	// A full bucket has t = 0.
	clk.Advance(time.Hour)
	if d := mustPeek(t, l, "rl.write", keyA); d.ResetSeconds != 0 || d.Remaining != 3 {
		t.Fatalf("full bucket: %+v", d)
	}
}

func mustPeek(t *testing.T, l *ratelimit.Limiter, class ratelimit.Class, key string) ratelimit.Decision {
	t.Helper()
	d, err := l.Peek(class, key)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestPeek_TakesNothing(t *testing.T) {
	t.Parallel()
	l, _ := newLimiter(t, config(map[ratelimit.Class]ratelimit.Limits{"rl.public_auth": {PerMinute: 60, Burst: 1}}))
	if d := mustPeek(t, l, "rl.public_auth", "ip:192.0.2.1"); !d.Allowed || d.Remaining != 1 || l.Len() != 0 {
		t.Fatalf("peek on a missing bucket %+v, %d tracked", d, l.Len())
	}
	allow(t, l, "rl.public_auth", "ip:192.0.2.1")
	if d := mustPeek(t, l, "rl.public_auth", "ip:192.0.2.1"); d.Allowed || d.RetryAfterSeconds != 1 {
		t.Fatalf("peek on an empty bucket %+v", d)
	}
	if _, err := l.Peek("rl.nope", "ip:192.0.2.1"); err == nil {
		t.Fatal("Peek with an unknown class gave no error")
	}
}

func TestKeys_ClassesIndependent(t *testing.T) {
	t.Parallel()
	l, _ := newLimiter(t, config(nil))
	for allow(t, l, "rl.write", keyA).Allowed {
	}
	if d := allow(t, l, "rl.read", keyA); !d.Allowed || d.Remaining != 99 {
		t.Fatalf("rl.read after draining rl.write: %+v", d)
	}
	if d := allow(t, l, "rl.write", "p:0192f4a0-7b1c-7577-8bf0-c4f4ee4f8870"); !d.Allowed {
		t.Fatalf("another principal after draining rl.write: %+v", d)
	}
}

func TestKeys_IPv6ByPrefix(t *testing.T) {
	t.Parallel()
	cases := []struct{ a, b string }{
		{"2001:db8:1:2::1", "2001:db8:1:2:ffff:ffff:ffff:ffff"},
		{"::ffff:192.0.2.7", "192.0.2.7"},
		{"fe80::1%eth0", "fe80::2"},
	}
	for _, c := range cases {
		if ka, kb := ratelimit.KeyForIP(netip.MustParseAddr(c.a)), ratelimit.KeyForIP(netip.MustParseAddr(c.b)); ka != kb {
			t.Errorf("%s -> %s, %s -> %s, want one key", c.a, ka, c.b, kb)
		}
	}
	for _, c := range [][2]string{{"2001:db8:1:2::1", "2001:db8:1:3::1"}, {"192.0.2.1", "192.0.2.2"}} {
		if ratelimit.KeyForIP(netip.MustParseAddr(c[0])) == ratelimit.KeyForIP(netip.MustParseAddr(c[1])) {
			t.Errorf("%s and %s share a key", c[0], c[1])
		}
	}
	if got := ratelimit.KeyForIP(netip.MustParseAddr("2001:db8:1:2:3:4:5:6")); got != "ip:2001:db8:1:2::/64" {
		t.Errorf("KeyForIP = %q", got)
	}
	if got := ratelimit.KeyForIP(netip.Addr{}); got != "ip:invalid" {
		t.Errorf("KeyForIP(invalid) = %q", got)
	}
}

func TestKeyKinds_MatchChapter05(t *testing.T) {
	t.Parallel()
	// The column "Key" of 05 names the key in words; each word maps to one kind.
	words := []struct {
		prefix string
		kind   ratelimit.KeyKind
	}{
		{"client IP", ratelimit.KeyClientIP},
		{"device or token", ratelimit.KeyDeviceOrToken},
		{"principal", ratelimit.KeyPrincipal},
		{"connection", ratelimit.KeyConnection},
		{"agent", ratelimit.KeyAgent},
		{"incoming webhook", ratelimit.KeyIncomingWebhook},
		{"host", ratelimit.KeyHost},
	}
	var fromChapter []ratelimit.Class
	for _, row := range chapterClasses(t) {
		fromChapter = append(fromChapter, row.class)
		i := slices.IndexFunc(words, func(w struct {
			prefix string
			kind   ratelimit.KeyKind
		}) bool {
			return strings.HasPrefix(row.key, w.prefix)
		})
		if i < 0 {
			t.Errorf("%s: key %q of 05 has no kind", row.class, row.key)
			continue
		}
		if kind, ok := ratelimit.KindOf(row.class); !ok || kind != words[i].kind {
			t.Errorf("%s: KindOf = %v (%v), 05 says %q", row.class, kind, ok, row.key)
		}
	}
	got := ratelimit.Classes()
	slices.Sort(got)
	slices.Sort(fromChapter)
	if !slices.Equal(got, fromChapter) {
		t.Errorf("Classes() = %v, 05 lists %v", got, fromChapter)
	}
	if _, ok := ratelimit.KindOf("rl.nope"); ok {
		t.Error("KindOf knows an unknown class")
	}
}

func TestConfig_RejectsZeroValues(t *testing.T) {
	t.Parallel()
	cases := map[string]func(*ratelimit.Config){
		"per_minute 0":       func(c *ratelimit.Config) { c.Classes["rl.read"] = ratelimit.Limits{PerMinute: 0, Burst: 1} },
		"burst 0":            func(c *ratelimit.Config) { c.Classes["rl.read"] = ratelimit.Limits{PerMinute: 1, Burst: 0} },
		"negative":           func(c *ratelimit.Config) { c.Classes["rl.read"] = ratelimit.Limits{PerMinute: -1, Burst: 1} },
		"per_minute too big": func(c *ratelimit.Config) { c.Classes["rl.read"] = ratelimit.Limits{PerMinute: 1000001, Burst: 1} },
		"burst too big":      func(c *ratelimit.Config) { c.Classes["rl.read"] = ratelimit.Limits{PerMinute: 1, Burst: 1000001} },
		"max_tracked_keys 0": func(c *ratelimit.Config) { c.MaxTrackedKeys = 0 },
		"sweep_interval 0":   func(c *ratelimit.Config) { c.SweepInterval = 0 },
		"unknown class":      func(c *ratelimit.Config) { c.Classes["rl.nope"] = ratelimit.Limits{PerMinute: 1, Burst: 1} },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			cfg := config(nil)
			mutate(&cfg)
			if _, err := ratelimit.New(cfg, clock.NewFake(t0), nil); err == nil {
				t.Fatal("New accepted the configuration")
			}
		})
	}
}

func TestConfig_EveryCatalogueClassConfigured(t *testing.T) {
	t.Parallel()
	for _, row := range chapterClasses(t) {
		cfg := config(nil)
		delete(cfg.Classes, row.class)
		_, err := ratelimit.New(cfg, clock.NewFake(t0), nil)
		if err == nil || !strings.Contains(err.Error(), string(row.class)) {
			t.Errorf("New without %s: %v", row.class, err)
		}
	}
}

func TestAllow_UnknownClassErrors(t *testing.T) {
	t.Parallel()
	l, _ := newLimiter(t, config(nil))
	if _, err := l.Allow("rl.nope", keyA); err == nil {
		t.Error("unknown class: no error")
	}
	if _, err := l.Allow("rl.read", ""); err == nil {
		t.Error("empty key: no error")
	}
	if l.Len() != 0 {
		t.Errorf("%d buckets after failed calls", l.Len())
	}
}

func TestMemory_BoundedUnderManyKeys(t *testing.T) {
	t.Parallel()
	for _, limit := range []int{1, 7, 100, 1000} {
		t.Run(fmt.Sprint(limit), func(t *testing.T) {
			t.Parallel()
			cfg := config(nil)
			cfg.MaxTrackedKeys = limit
			l, _ := newLimiter(t, cfg)
			for i := range 3 * limit {
				d := allow(t, l, "rl.public_auth", fmt.Sprintf("ip:192.0.2.%d/%d", i%256, i))
				if !d.Allowed {
					t.Fatalf("key %d denied: a full table must never deny", i)
				}
				if n := l.Len(); n > limit {
					t.Fatalf("Len = %d after %d keys, cap %d", n, i+1, limit)
				}
			}
		})
	}
}

func TestSweep_RemovesOnlyFullBuckets(t *testing.T) {
	t.Parallel()
	l, clk := newLimiter(t, config(map[ratelimit.Class]ratelimit.Limits{
		"rl.write": {PerMinute: 60, Burst: 2}, "rl.read": {PerMinute: 6, Burst: 2},
	}))
	allow(t, l, "rl.write", keyA) // full again after 1 s
	allow(t, l, "rl.read", keyA)  // full again after 10 s
	clk.Advance(time.Second)
	l.Sweep()
	if l.Len() != 1 || !l.Tracked("rl.read", keyA) || l.Tracked("rl.write", keyA) {
		t.Fatalf("after 1 s: %d tracked, read %v, write %v", l.Len(), l.Tracked("rl.read", keyA), l.Tracked("rl.write", keyA))
	}
	clk.Advance(9 * time.Second)
	l.Sweep()
	if l.Len() != 0 {
		t.Fatalf("after 10 s: %d tracked", l.Len())
	}
}

func TestEvict_PrefersFullThenOldest(t *testing.T) {
	t.Parallel()
	cfg := config(map[ratelimit.Class]ratelimit.Limits{
		"rl.write": {PerMinute: 1, Burst: 2},   // slow refill: never full within this test
		"rl.read":  {PerMinute: 600, Burst: 2}, // full again after 0.1 s
	})
	cfg.MaxTrackedKeys = 3
	l, clk := newLimiter(t, cfg)
	allow(t, l, "rl.write", "p:b") // oldest use
	clk.Advance(time.Second)
	allow(t, l, "rl.write", "p:c")
	clk.Advance(time.Second)
	allow(t, l, "rl.read", "p:a") // newest use, full after 0.1 s
	clk.Advance(time.Second)
	allow(t, l, "rl.write", "p:d") // table full: the full bucket goes, not the oldest
	if l.Tracked("rl.read", "p:a") || !l.Tracked("rl.write", "p:b") || l.Len() != 3 {
		t.Fatalf("full bucket kept or oldest evicted: a %v, b %v, len %d", l.Tracked("rl.read", "p:a"), l.Tracked("rl.write", "p:b"), l.Len())
	}
	allow(t, l, "rl.write", "p:e") // no full bucket: the oldest use goes
	if l.Tracked("rl.write", "p:b") || !l.Tracked("rl.write", "p:c") || !l.Tracked("rl.write", "p:d") || l.Len() != 3 {
		t.Fatalf("oldest not evicted: b %v, c %v, d %v, len %d", l.Tracked("rl.write", "p:b"), l.Tracked("rl.write", "p:c"), l.Tracked("rl.write", "p:d"), l.Len())
	}
}

func TestRun_StopsOnCancel(t *testing.T) {
	t.Parallel()
	cfg := config(map[ratelimit.Class]ratelimit.Limits{"rl.write": {PerMinute: 600000, Burst: 1}})
	cfg.SweepInterval = time.Millisecond
	l, err := ratelimit.New(cfg, clock.System(), nil)
	if err != nil {
		t.Fatal(err)
	}
	allow(t, l, "rl.write", keyA) // full again after 0.1 ms of real time
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		l.Run(ctx)
		close(done)
	}()
	deadline := testkit.Deadline()
	for l.Len() != 0 {
		if time.Now().After(deadline) {
			t.Fatal("Run never swept the full bucket")
		}
		runtime.Gosched()
		time.Sleep(time.Millisecond)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(testkit.Patience):
		t.Fatal("Run did not return after the context ended")
	}
}

func TestConcurrent_NoOverAdmission(t *testing.T) {
	t.Parallel()
	l, _ := newLimiter(t, config(nil)) // rl.read burst 100, frozen clock
	var allowed atomic.Int64
	var wg sync.WaitGroup
	for range 64 {
		wg.Go(func() {
			for range 20 {
				if d, err := l.Allow("rl.read", keyA); err == nil && d.Allowed {
					allowed.Add(1)
				}
			}
		})
	}
	wg.Wait()
	if allowed.Load() != 100 {
		t.Fatalf("allowed %d of 1280 requests, want exactly the burst 100", allowed.Load())
	}
}

func TestMetric_RejectedCounter(t *testing.T) {
	t.Parallel()
	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	l, err := ratelimit.New(config(map[ratelimit.Class]ratelimit.Limits{"rl.write": {PerMinute: 60, Burst: 1}}),
		clock.NewFake(t0), mp.Meter("test"))
	if err != nil {
		t.Fatal(err)
	}
	allow(t, l, "rl.write", keyA)
	allow(t, l, "rl.write", keyA)
	allow(t, l, "rl.write", keyA)
	allow(t, l, "rl.read", keyA)
	var rm metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &rm); err != nil {
		t.Fatal(err)
	}
	got := map[string]int64{}
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name != "bothy.ratelimit.rejected" {
				continue
			}
			if m.Unit != "{request}" {
				t.Errorf("unit %q", m.Unit)
			}
			for _, dp := range m.Data.(metricdata.Sum[int64]).DataPoints {
				v, _ := dp.Attributes.Value(attribute.Key("bothy.ratelimit.class"))
				if dp.Attributes.Len() != 1 {
					t.Errorf("attributes %v", dp.Attributes.ToSlice())
				}
				got[v.AsString()] += dp.Value
			}
		}
	}
	if len(got) != 1 || got["rl.write"] != 2 {
		t.Fatalf("bothy.ratelimit.rejected = %v, want rl.write: 2", got)
	}
}

func TestKeyForActorIDs(t *testing.T) {
	t.Parallel()
	id := ids.MustParse("0192f4a0-7b1c-7577-8bf0-c4f4ee4f887e")
	if ratelimit.KeyForPrincipal(id) != "p:"+id.String() || ratelimit.KeyForAgent(id) != "a:"+id.String() ||
		ratelimit.KeyForHost(id) != "h:"+id.String() || ratelimit.KeyForDevice(id) != "d:"+id.String() ||
		ratelimit.KeyForToken(id) != "t:"+id.String() || ratelimit.KeyForConnection("c1") != "c:c1" ||
		ratelimit.KeyForWebhook("w1") != "w:w1" {
		t.Fatal("key prefixes changed")
	}
}

func BenchmarkLimiter_Allow(b *testing.B) {
	cfg := config(nil)
	l, err := ratelimit.New(cfg, clock.System(), nil)
	if err != nil {
		b.Fatal(err)
	}
	keys := make([]string, 1024)
	for i := range keys {
		keys[i] = fmt.Sprintf("p:%04d", i)
	}
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			_, _ = l.Allow("rl.agent_runtime", keys[i&1023])
			i++
		}
	})
}

// BenchmarkLimiter_BytesPerKey reports the heap bytes one tracked bucket costs, with keys of the
// length of a principal key ("p:" plus a UUID); the README derives the memory of
// ratelimit.max_tracked_keys from it.
func BenchmarkLimiter_BytesPerKey(b *testing.B) {
	const n = 100000
	keys := make([]string, n)
	for i := range keys {
		keys[i] = fmt.Sprintf("p:0192f4a0-7b1c-7577-8bf0-%012d", i)
	}
	var perKey float64
	for b.Loop() {
		cfg := config(nil)
		cfg.MaxTrackedKeys = n
		runtime.GC()
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		l, err := ratelimit.New(cfg, clock.NewFake(t0), nil)
		if err != nil {
			b.Fatal(err)
		}
		for _, k := range keys {
			_, _ = l.Allow("rl.read", k)
		}
		runtime.GC()
		runtime.ReadMemStats(&after)
		perKey = float64(after.HeapAlloc-before.HeapAlloc) / n
		runtime.KeepAlive(l)
	}
	b.ReportMetric(perKey, "bytes/key")
}

// TestRefund_ReturnsOneTokenUpToBurst: Refund gives back a token Allow took, never more than the
// burst, and creates no bucket.
func TestRefund_ReturnsOneTokenUpToBurst(t *testing.T) {
	t.Parallel()
	l, _ := newLimiter(t, config(map[ratelimit.Class]ratelimit.Limits{"rl.write": {PerMinute: 60, Burst: 2}}))
	allow(t, l, "rl.write", keyA)
	allow(t, l, "rl.write", keyA)
	if err := l.Refund("rl.write", keyA); err != nil {
		t.Fatal(err)
	}
	if d, _ := l.Peek("rl.write", keyA); d.Remaining != 1 {
		t.Fatalf("after one refund: %d tokens, want 1", d.Remaining)
	}
	for range 3 {
		_ = l.Refund("rl.write", keyA)
	}
	if d, _ := l.Peek("rl.write", keyA); d.Remaining != 2 || d.ResetSeconds != 0 {
		t.Fatalf("refunds above the burst: %+v", d)
	}
	if err := l.Refund("rl.read", "p:none"); err != nil || l.Tracked("rl.read", "p:none") {
		t.Fatalf("refund on a missing bucket: %v, tracked %v", err, l.Tracked("rl.read", "p:none"))
	}
	if err := l.Refund("rl.nope", keyA); err == nil {
		t.Fatal("unknown class: no error")
	}
}

// TestShard_SeedPerLimiter: the shard of a key depends on a random seed of the limiter, so a
// client cannot aim its keys at the shard of another bucket (red team M1-19, finding 2).
func TestShard_SeedPerLimiter(t *testing.T) {
	t.Parallel()
	a, _ := newLimiter(t, config(nil))
	b, _ := newLimiter(t, config(nil))
	same := 0
	for i := range 100 {
		k := fmt.Sprintf("p:%d", i)
		if a.ShardOf("rl.write", k) == b.ShardOf("rl.write", k) {
			same++
		}
	}
	if same == 100 {
		t.Fatal("two limiters place 100 keys in the same shards: the shard choice has no seed")
	}
}
