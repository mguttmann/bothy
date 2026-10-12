// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"maps"
	"sync"
	"time"

	"github.com/OWNER/bothy/services/server/internal/audit/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
)

// Aggregator collects identical denials per audit.deny_aggregate_window (05 "2.4", R-AZ-04,
// SEC-AUD-6) and writes one row per key: when the window of the key ends (Run), at Flush and at
// shutdown, or at once for the oldest key when audit.deny_aggregate_max_keys keys are held. A
// write that fails keeps its buckets for the next attempt; only a bucket that leaves the cap and
// cannot be written is dropped, with Reporter.DenialsDropped. The buckets live in the memory of
// one process: several processes each write their own rows, and a hard crash loses the open
// buckets of at most one window (residual risk of SEC-AUD-6). It is safe for concurrent use.
type Aggregator struct {
	clock    clock.Clock
	window   time.Duration
	write    func(ctx context.Context, rows []domain.Row) error
	reporter Reporter
	wake     chan struct{}

	mu    sync.Mutex
	agg   *domain.Aggregation[domain.Row]
	retry time.Time // no timer write before this time after a failure
}

// newAggregator returns the aggregator that writes through write.
func newAggregator(window time.Duration, maxKeys int, clk clock.Clock, write func(context.Context, []domain.Row) error, rep Reporter) *Aggregator {
	return &Aggregator{
		clock: clk, window: window, write: write, reporter: rep, wake: make(chan struct{}, 1),
		agg: domain.NewAggregation[domain.Row](window, maxKeys),
	}
}

// add counts one denial; an evicted bucket is written at once and dropped with a report when that
// fails.
func (a *Aggregator) add(ctx context.Context, key domain.AggregateKey, at time.Time, row domain.Row) {
	a.mu.Lock()
	before := a.agg.Len()
	evicted, full := a.agg.Add(key, at, row)
	grew := a.agg.Len() > before
	a.mu.Unlock()
	if grew {
		select {
		case a.wake <- struct{}{}:
		default:
		}
	}
	if !full {
		return
	}
	// The caller's transaction rolled back or never existed; its cancellation must not lose the
	// row, the statement timeouts of the unit of work bound the write.
	if err := a.write(context.WithoutCancel(ctx), []domain.Row{bucketRow(evicted)}); err != nil {
		a.reporter.DenialsDropped(ctx, evicted.Key.Action, evicted.Key.Reason, evicted.Count)
	}
}

// bucketRow is the row of a bucket: the first denial with occurred_at = first_at and
// details.aggregate = {count, first_at, last_at}.
func bucketRow(b domain.Bucket[domain.Row]) domain.Row {
	r := b.First
	r.OccurredAt = b.FirstAt
	r.Details = maps.Clone(r.Details)
	r.Details[domain.DetailAggregate] = map[string]any{
		"count": b.Count, "first_at": domain.FormatTime(b.FirstAt), "last_at": domain.FormatTime(b.LastAt),
	}
	return r
}

// Run writes every bucket when its window ends until ctx ends, and returns nil then. The timer
// waits for the earliest deadline; after a failed write it waits one window before the next
// attempt. Run does not flush at its end: the shutdown step calls Flush after Run returned.
func (a *Aggregator) Run(ctx context.Context) error {
	for {
		a.mu.Lock()
		next, ok := a.agg.NextDeadline()
		if a.retry.After(next) {
			next = a.retry
		}
		a.mu.Unlock()
		var fire <-chan time.Time
		var timer *time.Timer
		if ok {
			timer = time.NewTimer(max(next.Sub(a.clock.Now()), 0))
			fire = timer.C
		}
		select {
		case <-ctx.Done():
			stopTimer(timer)
			return nil
		case <-a.wake:
			stopTimer(timer)
		case <-fire:
			_ = a.FlushDue(ctx) // reported inside; the buckets stay for the next round
		}
	}
}

// stopTimer stops t if it is set.
func stopTimer(t *time.Timer) {
	if t != nil {
		t.Stop()
	}
}

// FlushDue writes the buckets whose window has ended by the clock, in one own transaction. On
// failure the buckets go back (Reporter.WriteFailed) and the error is returned.
func (a *Aggregator) FlushDue(ctx context.Context) error {
	a.mu.Lock()
	due := a.agg.TakeDue(a.clock.Now())
	a.mu.Unlock()
	return a.writeBuckets(ctx, due)
}

// Flush writes every bucket, open or due, in one own transaction (shutdown, tests). On failure the
// buckets go back and the error is returned.
func (a *Aggregator) Flush(ctx context.Context) error {
	a.mu.Lock()
	all := a.agg.TakeAll()
	a.mu.Unlock()
	return a.writeBuckets(ctx, all)
}

// writeBuckets writes bs or puts them back.
func (a *Aggregator) writeBuckets(ctx context.Context, bs []domain.Bucket[domain.Row]) error {
	if len(bs) == 0 {
		return nil
	}
	rows := make([]domain.Row, len(bs))
	for i, b := range bs {
		rows[i] = bucketRow(b)
	}
	err := a.write(ctx, rows)
	if err == nil {
		return nil
	}
	a.reporter.WriteFailed(ctx, err)
	a.mu.Lock()
	dropped := a.agg.Restore(bs)
	a.retry = a.clock.Now().Add(a.window)
	a.mu.Unlock()
	for _, b := range dropped {
		a.reporter.DenialsDropped(ctx, b.Key.Action, b.Key.Reason, b.Count)
	}
	return errors.Join(errors.New("audit: write aggregated denials"), err)
}

// Close is the final flush at shutdown: Flush, and when that fails every bucket that was not
// written is dropped and reported with Reporter.DenialsDropped, because the process ends and
// keeps nothing. It returns the error of the flush.
func (a *Aggregator) Close(ctx context.Context) error {
	err := a.Flush(ctx)
	if err == nil {
		return nil
	}
	a.mu.Lock()
	lost := a.agg.TakeAll()
	a.mu.Unlock()
	for _, b := range lost {
		a.reporter.DenialsDropped(ctx, b.Key.Action, b.Key.Reason, b.Count)
	}
	return err
}
