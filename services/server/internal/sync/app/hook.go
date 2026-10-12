// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/instance"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// PhaseHook is the phase hook of sync (02 "Flow" step 3.2): it numbers the buffered events of
// a unit with event_seq, writes them to event_log and wakes the log tails. internal/wire
// registers Run as the only hook of txn.PhaseSync. It is safe for concurrent use.
type PhaseHook struct {
	counters instance.Counters
	writer   Writer
}

// NewPhaseHook returns the hook over the counters of instance and the writer. internal/wire
// passes instance.Counters only to the constructors of the phase hooks
// (TestCountersOnlyUsedByPhaseHooks).
func NewPhaseHook(counters instance.Counters, writer Writer) *PhaseHook {
	return &PhaseHook{counters: counters, writer: writer}
}

// Run writes the buffered events of the unit of ctx. Without events it returns at once, so a
// transaction without events takes neither the counter lock nor NOTIFY (02 "Cost of
// serialization"). Otherwise, as the last domain step before the audit hook: lock and advance
// event_seq by n (04 "Pattern with a counter row"), read permission_version after the lock so the
// value matches the commit order, then insert the rows in append order with seq last-n+1 .. last
// and pg_notify('event_log', last) in one statement (Writer.Write).
//
// Errors are returned unchanged, so the unit of work still sees a deadlock or lock timeout and
// retries; a missing partition is kind unavailable (Writer.Insert).
func (h *PhaseHook) Run(ctx context.Context) error {
	rows, _ := txn.Load(ctx, pending)
	if len(rows) == 0 {
		return nil
	}
	n := int64(len(rows))
	last, err := h.counters.Advance(ctx, instance.CounterEventSeq, n)
	if err != nil {
		return err
	}
	pv, err := h.counters.Current(ctx, instance.CounterPermissionVersion)
	if err != nil {
		return err
	}
	return h.writer.Write(ctx, last-n+1, pv, rows)
}
