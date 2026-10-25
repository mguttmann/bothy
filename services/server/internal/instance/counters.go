// SPDX-License-Identifier: Apache-2.0

package instance

import "context"

// CounterName names a row of sequence_counters (docs/spec/03-data-model.md).
type CounterName string

// The three counters; the migration that creates sequence_counters inserts their rows with
// value 0.
const (
	// CounterEventSeq numbers the durable events; advanced only in the sync hook.
	CounterEventSeq CounterName = "event_seq"
	// CounterAuditSeq numbers the audit chain; advanced only in the audit hook.
	CounterAuditSeq CounterName = "audit_seq"
	// CounterPermissionVersion is the rights version; advanced only in the rights begin hook.
	CounterPermissionVersion CounterName = "permission_version"
)

// Counters is the port of the sequence counters, the serialization points that make event
// sequence, audit chain and rights version gapless and commit-ordered (B8, ADR 0009, R-DM-05,
// docs/spec/04-sync-protocol.md "Pattern with a counter row").
//
// Only the phase hooks of authz, sync and audit use it (02 "Lock order"): internal/wire
// constructs it and passes it only to their constructors, TestCountersOnlyUsedByPhaseHooks in
// test/countersscan limits the packages that name it, and the implementation checks the phase of the
// unit of work at run time (kernel/txn.CurrentPhase). Both methods need a context inside a unit of
// work of platform/pg.
type Counters interface {
	// Advance locks the counter row until the commit and raises its value by n
	// (UPDATE ... SET value = value + n ... RETURNING value). It returns the new value last; the
	// caller's numbers are last-n+1 .. last. A transaction that waits at the row lock reads the
	// committed value after the holder commits (READ COMMITTED), so numbers follow the commit
	// order; a rollback returns the numbers with the row.
	//
	// Allowed phases: permission_version only in rights_begin, event_seq only in sync,
	// audit_seq only in audit. Errors: kind internal for another phase, an unknown name or
	// n < 1, before any SQL; the database error otherwise (a lock timeout is retried by the unit
	// of work).
	//
	// Example (in the sync hook):
	//
	//	last, err := counters.Advance(ctx, instance.CounterEventSeq, int64(len(events)))
	Advance(ctx context.Context, name CounterName, n int64) (last int64, err error)
	// Current reads the value of the counter without a lock: inside the hook that already holds
	// the lock it is that hook's own value, otherwise the committed value. Allowed only in the
	// four hook phases (rights_begin, rights_check, sync, audit). Errors: kind internal for
	// another phase or an unknown name; the database error otherwise.
	Current(ctx context.Context, name CounterName) (int64, error)
}
