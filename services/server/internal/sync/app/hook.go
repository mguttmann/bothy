// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/instance"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// PhaseHook is the phase hook of sync (02 "Ablauf" step 3.2): it numbers the buffered events of
// a unit with event_seq, writes them to event_log and wakes the log tails. internal/wire
// registers Run as the only hook of txn.PhaseSync. It is safe for concurrent use.
type PhaseHook struct {
	counters instance.Counters
	writer   Writer
	metrics  Metrics
	clock    clock.Clock
}

// NewPhaseHook returns the hook over the counters of instance and the writer. internal/wire
// passes instance.Counters only to the constructors of the phase hooks
// (TestCountersOnlyUsedByPhaseHooks).
func NewPhaseHook(counters instance.Counters, writer Writer) *PhaseHook {
	return &PhaseHook{counters: counters, writer: writer, metrics: noMetrics{}, clock: clock.System()}
}

// WithMetrics returns h measuring through m (nil measures nothing) with the clock clk (nil means
// clock.System()): the duration of the event_seq lock and, after the commit of each unit, its
// events per type (signals.md 4.2).
func (h *PhaseHook) WithMetrics(m Metrics, clk clock.Clock) *PhaseHook {
	if m == nil {
		m = noMetrics{}
	}
	if clk == nil {
		clk = clock.System()
	}
	h.metrics, h.clock = m, clk
	return h
}

// Run writes the buffered events of the unit of ctx. Without events it returns at once, so a
// transaction without events takes neither the counter lock nor NOTIFY (02 "Kosten der
// Serialisierung"). Otherwise, as the last domain step before the audit hook: lock and advance
// event_seq by n (04 "Muster mit Zähler-Zeile"), read permission_version after the lock so the
// value matches the commit order, then insert the rows in append order with seq last-n+1 .. last
// and pg_notify('event_log', last) in one statement (Writer.Write).
//
// The duration of the lock statement is recorded in bothy.db.event_seq.lock_wait; the events
// are counted per type in bothy.event.appended once the unit commits (txn.AfterCommit), so a
// rolled-back unit counts nothing.
//
// Errors are returned unchanged, so the unit of work still sees a deadlock or lock timeout and
// retries; a missing partition is kind unavailable (Writer.Insert).
func (h *PhaseHook) Run(ctx context.Context) error {
	rows, _ := txn.Load(ctx, pending)
	if len(rows) == 0 {
		return nil
	}
	n := int64(len(rows))
	start := h.clock.Now()
	last, err := h.counters.Advance(ctx, instance.CounterEventSeq, n)
	h.metrics.EventSeqLockWait(ctx, h.clock.Now().Sub(start))
	if err != nil {
		return err
	}
	h.countAfterCommit(ctx, rows)
	pv, err := h.counters.Current(ctx, instance.CounterPermissionVersion)
	if err != nil {
		return err
	}
	return h.writer.Write(ctx, last-n+1, pv, rows)
}

// countAfterCommit registers the count of rows per event type to run after the commit.
func (h *PhaseHook) countAfterCommit(ctx context.Context, rows []PendingEvent) {
	byType := map[string]int{}
	var order []string
	for _, r := range rows {
		if byType[r.Event.Type] == 0 {
			order = append(order, r.Event.Type)
		}
		byType[r.Event.Type]++
	}
	m := h.metrics
	// The commit happens after the hook returned; the context of the unit may be done by then,
	// and recording needs no deadline.
	mctx := context.WithoutCancel(ctx)
	txn.AfterCommit(ctx, func() {
		for _, t := range order {
			m.EventsAppended(mctx, t, byType[t])
		}
	})
}
