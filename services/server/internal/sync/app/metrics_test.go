// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"maps"
	gosync "sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/instance"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/sync/app"
)

// fakeMetrics records what the phase hook measures.
type fakeMetrics struct {
	mu       gosync.Mutex
	appended map[string]int
	waits    []time.Duration
}

func (m *fakeMetrics) EventsAppended(_ context.Context, eventType string, n int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.appended == nil {
		m.appended = map[string]int{}
	}
	m.appended[eventType] += n
}

func (m *fakeMetrics) EventSeqLockWait(_ context.Context, d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.waits = append(m.waits, d)
}

// controlledUnit returns a context inside a new unit in the work phase and its control.
func controlledUnit() (context.Context, txn.UnitControl) {
	reg := txn.NewRegistry()
	reg.Seal()
	u, ctrl := txn.NewControlledUnit(reg, txn.Options{})
	ctrl.SetPhase(txn.PhaseWork)
	return txn.WithUnit(context.Background(), u), ctrl
}

// TestSyncMetrics_EventAppendedAfterCommitOnly: bothy.event.appended counts the events of a unit
// per durable type only once the unit committed; a unit that rolls back counts nothing.
func TestSyncMetrics_EventAppendedAfterCommitOnly(t *testing.T) {
	t.Parallel()
	m := &fakeMetrics{}
	hook := app.NewPhaseHook(&fakeCounters{}, &fakeWriter{}).WithMetrics(m, nil)
	a := newAppender(t, fixedCorrelation{})
	run := func(commit bool) {
		ctx, ctrl := controlledUnit()
		other := event()
		other.Type = "thread.reopened"
		if err := a.Append(ctx, event(), event(), other); err != nil {
			t.Fatal(err)
		}
		ctrl.SetPhase(txn.PhaseSync)
		if err := hook.Run(ctx); err != nil {
			t.Fatal(err)
		}
		if len(m.appended) != 0 {
			t.Fatalf("counted %v before the commit", m.appended)
		}
		if commit {
			ctrl.Committed()
		}
	}
	run(false)
	run(true)
	if want := map[string]int{"thread.closed": 2, "thread.reopened": 1}; !maps.Equal(m.appended, want) {
		t.Errorf("appended = %v, want %v (the rolled-back unit counts nothing)", m.appended, want)
	}
}

// slowCounters advances the fake clock while the event_seq row is locked and advanced.
type slowCounters struct {
	fakeCounters
	clk  *clock.Fake
	wait time.Duration
}

func (c *slowCounters) Advance(ctx context.Context, name instance.CounterName, n int64) (int64, error) {
	c.clk.Advance(c.wait)
	return c.fakeCounters.Advance(ctx, name, n)
}

// TestSyncMetrics_LockWaitRecorded: bothy.db.event_seq.lock_wait is the duration of the statement
// that locks and advances the event_seq row; a unit without events takes no lock and records
// nothing.
func TestSyncMetrics_LockWaitRecorded(t *testing.T) {
	t.Parallel()
	clk := clock.NewFake(time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC))
	m := &fakeMetrics{}
	hook := app.NewPhaseHook(&slowCounters{clk: clk, wait: 30 * time.Millisecond}, &fakeWriter{}).WithMetrics(m, clk)
	empty, _ := controlledUnit()
	if err := hook.Run(empty); err != nil {
		t.Fatal(err)
	}
	ctx, ctrl := controlledUnit()
	if err := newAppender(t, fixedCorrelation{}).Append(ctx, event()); err != nil {
		t.Fatal(err)
	}
	ctrl.SetPhase(txn.PhaseSync)
	if err := hook.Run(ctx); err != nil {
		t.Fatal(err)
	}
	if len(m.waits) != 1 || m.waits[0] != 30*time.Millisecond {
		t.Errorf("lock waits = %v, want [30ms]", m.waits)
	}
}
