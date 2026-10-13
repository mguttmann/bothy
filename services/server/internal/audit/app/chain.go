// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"fmt"
	"slices"
	"sync"

	"github.com/OWNER/bothy/services/server/internal/audit/domain"
	"github.com/OWNER/bothy/services/server/internal/instance"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// pending is the per-unit buffer of rows that Record and the own transactions of
// RecordDetached and the aggregator fill and the audit hook writes. kernel/txn drops it with a
// rollback and starts every attempt empty.
var pending = txn.NewKey[[]domain.Row]("audit rows")

// pendingMu makes the read and write of the buffer one step, so two goroutines that record in
// the same unit of work never lose an entry; units are short, one lock for all is enough.
var pendingMu sync.Mutex

// appendPending adds rows to the buffer of the unit of ctx.
func appendPending(ctx context.Context, rows ...domain.Row) error {
	pendingMu.Lock()
	defer pendingMu.Unlock()
	have, _ := txn.Load(ctx, pending)
	return txn.Store(ctx, pending, append(slices.Clip(have), rows...))
}

// Chain is the audit phase hook (02 "Flow" phase 3.3): it numbers the buffered rows of a unit
// with audit_seq, chains them and inserts them before the commit. internal/wire registers
// Chain.Hook as the only hook of txn.PhaseAudit. It is safe for concurrent use.
type Chain struct {
	counters instance.Counters
	store    Store
	reporter Reporter
}

// NewChain returns the hook over the audit_seq counter, the store and the reporter. internal/wire
// passes instance.Counters only to the constructors of the phase hooks
// (TestCountersOnlyUsedByPhaseHooks).
func NewChain(counters instance.Counters, store Store, reporter Reporter) *Chain {
	return &Chain{counters: counters, store: store, reporter: reporter}
}

// Hook writes the buffered rows of the unit of ctx. Without rows it does nothing, so a unit
// without audit entries never locks audit_seq. Otherwise it reserves n numbers with
// Counters.Advance (the row lock of audit_seq holds until the commit, last in the lock order of
// 03 "6.3"), reads the hash of row first - 1 (ZeroHash for row 1), computes the hash of each row
// in order and inserts all rows in one statement. If it fails, the unit fails ("if the audit
// fails, the change fails", docs/observability/audit-vs-telemetry.md).
//
// Errors are kind internal: a missing predecessor ("audit chain broken", reported through
// Reporter.ChainBroken), a row outside the canonical form, and the database errors, wrapped so
// that the unit of work still sees a deadlock or lock timeout and retries.
func (c *Chain) Hook(ctx context.Context) error {
	rows, _ := txn.Load(ctx, pending)
	if len(rows) == 0 {
		return nil
	}
	n := int64(len(rows))
	last, err := c.counters.Advance(ctx, instance.CounterAuditSeq, n)
	if err != nil {
		return errs.Wrap(errs.CodeInternalError, err, "audit: reserve audit_seq")
	}
	first := last - n + 1
	prev := domain.ZeroHash
	if first > 1 {
		h, ok, err := c.store.HashAt(ctx, first-1)
		if err != nil {
			return errs.Wrap(errs.CodeInternalError, err, "audit: read the hash of the previous row")
		}
		if !ok {
			c.reporter.ChainBroken(ctx, first-1)
			return errs.New(errs.CodeInternalError, fmt.Sprintf("audit chain broken: row %d is missing", first-1))
		}
		prev = h
	}
	out := make([]domain.Row, len(rows))
	for i, r := range rows {
		r.Seq = first + int64(i)
		r.PrevHash = prev
		canonical, err := domain.CanonicalRow(r)
		if err != nil {
			return errs.Wrap(errs.CodeInternalError, err, "audit: canonical form")
		}
		r.Hash = domain.ChainHash(prev, canonical)
		prev = r.Hash
		out[i] = r
	}
	if err := c.store.InsertRows(ctx, out); err != nil {
		return errs.Wrap(errs.CodeInternalError, err, "audit: insert rows")
	}
	return nil
}
