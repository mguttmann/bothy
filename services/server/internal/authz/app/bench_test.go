// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
)

// BenchmarkAuthorizeCached measures Authorize from a warm cache with in-memory fakes:
// message.read on a visible channel (02 section 17, perf.authorize_cached_p99). It reports the
// 99th percentile of the iterations as p99-ns and asserts nothing; M1-72 compares the value with
// the target.
func BenchmarkAuthorizeCached(b *testing.B) {
	w := newWorld(b)
	p := w.user(w.alex)
	r := w.msg(w.allgemein, w.sam)
	ctx := context.Background()
	if d := w.decide(b, ctx, p, authorize.ActionMessageRead, r); !d.Allowed() {
		b.Fatalf("not allowed: %+v", d)
	}
	durations := make([]time.Duration, 0, b.N)
	b.ResetTimer()
	for b.Loop() {
		start := time.Now()
		_, _ = w.az.Authorize(ctx, p, authorize.ActionMessageRead, r)
		durations = append(durations, time.Since(start))
	}
	b.StopTimer()
	if len(durations) == 0 {
		return
	}
	slices.Sort(durations)
	b.ReportMetric(float64(durations[(len(durations)*99)/100]), "p99-ns")
}

// BenchmarkAuthorizeCachedActingFor is BenchmarkAuthorizeCached for Lotse acting in a run of Alex
// (SR9, brief M2-14 design note 11): the intersection costs one more snapshot lookup in the cache.
// It reports p99-ns and asserts nothing.
func BenchmarkAuthorizeCachedActingFor(b *testing.B) {
	w := newWorld(b)
	p := w.agentFor(w.lotse, w.alex, w.newActingRun(), actor.ActingForBound)
	r := w.msg(w.eskalation, w.sam)
	ctx := context.Background()
	if d := w.decide(b, ctx, p, authorize.ActionMessageRead, r); !d.Allowed() {
		b.Fatalf("not allowed: %+v", d)
	}
	durations := make([]time.Duration, 0, b.N)
	b.ResetTimer()
	for b.Loop() {
		start := time.Now()
		_, _ = w.az.Authorize(ctx, p, authorize.ActionMessageRead, r)
		durations = append(durations, time.Since(start))
	}
	b.StopTimer()
	if len(durations) == 0 {
		return
	}
	slices.Sort(durations)
	b.ReportMetric(float64(durations[(len(durations)*99)/100]), "p99-ns")
}
