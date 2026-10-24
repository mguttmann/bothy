// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"time"

	"github.com/OWNER/bothy/services/server/internal/billing"
	"github.com/OWNER/bothy/services/server/internal/billing/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// NewEntry is one cost entry to insert (03 cost_entries); zero IDs are NULL.
type NewEntry struct {
	ID, RunID, AgentID, ThreadID, ChannelID ids.ID
	SpaceID, AgentSessionID                 ids.ID
	BillingMode, Provider, Model            string
	Tokens                                  domain.Tokens
	AmountMicros                            int64
	Currency, Source                        string
	OccurredAt                              time.Time
}

// TotalsFilter is the query of Repository.Totals: the level column and its value, the period and
// the channels. The repository never sees an empty channel filter (Totals returns nothing first).
type TotalsFilter struct {
	Level    billing.Level
	ID       ids.ID
	From, To time.Time
	// AllChannels lifts the channel restriction; otherwise ChannelIDs.
	AllChannels bool
	ChannelIDs  []ids.ID
}

// ErrPriceOverlap reports an exclusion or unique violation of price_table (SQLSTATE 23P01 or
// 23505); Create answers 409 conflict with reason price_overlap.
var ErrPriceOverlap = errors.New("billing: the validity overlaps a row of the same provider, model and unit")

// Repository is the outgoing port of price_table and cost_entries. Reads join the ambient
// transaction of ctx or use the pool; writes and locks need the ambient transaction (an internal
// error otherwise).
type Repository interface {
	// LockPriceSet returns the rows of provider and model (exact match) that cover at, every unit,
	// locked FOR SHARE in ascending id (03 section 6.3), so a concurrent closePrice waits for the
	// booking and a booking that waited for a close sees the new end.
	LockPriceSet(ctx context.Context, provider, model string, at time.Time) ([]domain.PriceRow, error)
	// InsertEntries inserts the entries with is_estimate true and no reconciliation columns.
	InsertEntries(ctx context.Context, es []NewEntry) error
	// ThreadEntries returns every entry with thread_id, superseded ones included, in ascending id.
	ThreadEntries(ctx context.Context, threadID ids.ID) ([]domain.EntryRow, error)
	// Totals returns the valid sums per billing mode and currency of f, ordered by billing mode,
	// then currency.
	Totals(ctx context.Context, f TotalsFilter) ([]domain.Total, error)
	// ListPrices returns every price row ordered by provider, model, unit, valid_from and id.
	ListPrices(ctx context.Context) ([]domain.PriceRow, error)
	// CurrencyMismatch reports a row of provider and model (any unit) whose validity overlaps
	// [from, to) (to zero: open) and whose currency is not currency.
	CurrencyMismatch(ctx context.Context, provider, model, currency string, from, to time.Time) (bool, error)
	// InsertPrice inserts a row. Errors: ErrPriceOverlap.
	InsertPrice(ctx context.Context, p domain.PriceRow, createdBy ids.ID) error
	// LockPrice returns the row id locked FOR UPDATE; found is false for an unknown id.
	LockPrice(ctx context.Context, id ids.ID) (row domain.PriceRow, found bool, err error)
	// PriceInUse reports a computed entry, valid or superseded, of the row's provider and model
	// with occurred_at at or after the row's valid_from and at or after validTo whose count for
	// the row's unit is above 0 (M2-22 design note 12 c).
	PriceInUse(ctx context.Context, row domain.PriceRow, validTo time.Time) (bool, error)
	// SetValidTo sets valid_to of the row id.
	SetValidTo(ctx context.Context, id ids.ID, validTo time.Time) error
	// LockReconcile takes the exclusive transaction-scoped advisory lock of each run (key
	// namespace billing.reconcile:<run_id>), one statement per ID in the given order, which the
	// caller sorts ascending (M2-28 design note 3 c).
	LockReconcile(ctx context.Context, runIDs []ids.ID) error
	// LockReconcileShared takes the shared form of the same locks (design note 6 a).
	LockReconcileShared(ctx context.Context, runIDs []ids.ID) error
	// SessionsOfRuns returns the agent sessions of the entries of the runs, ascending.
	SessionsOfRuns(ctx context.Context, runIDs []ids.ID) ([]ids.ID, error)
	// LockSessions takes the exclusive transaction-scoped advisory lock of each session (key
	// namespace billing.session:<session_id>), one statement per ID in the given order.
	LockSessions(ctx context.Context, sessionIDs []ids.ID) error
	// LockSessionShared takes the shared form of the lock of one session (a booking).
	LockSessionShared(ctx context.Context, sessionID ids.ID) error
	// ValidEntries returns the valid entries (no entry supersedes them) of the runs with
	// occurred_at in [from, to), every column, in ascending id. It reads in a new statement, so
	// after LockReconcile it sees what a concurrent reconciliation committed.
	ValidEntries(ctx context.Context, runIDs []ids.ID, from, to time.Time) ([]domain.Entry, error)
	// InsertReconciled inserts reconciled entries (is_estimate false, source reconciled,
	// reconciled_at and supersedes_id set); it never updates or deletes a row.
	InsertReconciled(ctx context.Context, es []domain.Entry) error
	// RunTotals returns the valid sums of the runs over all their valid entries, one row per run
	// and currency, ordered by run and currency.
	RunTotals(ctx context.Context, runIDs []ids.ID) ([]CurrencyTotal, error)
	// SessionTotals returns the valid sums of the agent sessions of the runs over all valid
	// entries of each session, one row per session and currency, ordered by session and currency.
	SessionTotals(ctx context.Context, runIDs []ids.ID) ([]CurrencyTotal, error)
	// Estimates returns at most limit estimates per run of f (M2-28 design note 12), by run id
	// ascending after f.After.
	Estimates(ctx context.Context, f EstimatesFilter, limit int) ([]billing.Estimate, error)
}

// CurrencyTotal is the valid sum of one run or session in one currency.
type CurrencyTotal struct {
	// ID is the run or the session.
	ID           ids.ID
	Currency     string
	AmountMicros int64
	// IsEstimate is true when any summed entry is an estimate.
	IsEstimate bool
}

// EstimatesFilter is the query of Repository.Estimates: the half-open range, the agents and the
// keyset position (the last run of the previous page; zero for the first page).
type EstimatesFilter struct {
	From, To time.Time
	AgentIDs []ids.ID
	After    ids.ID
}

// ReconcileScheduler inserts the job billing.reconcile with the run IDs (ascending) in the
// ambient unit of work (M2-28 design note 3 g); adapters/jobs implements it.
type ReconcileScheduler interface {
	Schedule(ctx context.Context, runIDs []ids.ID) error
}

// Cursors encodes and decodes the opaque page cursors of 06 "Pagination"; adapters/httpapi
// implements it with the codec of the HTTP layer.
type Cursors interface {
	// Encode returns the cursor of key for list and binding.
	Encode(list string, binding []byte, key any) (string, error)
	// Decode checks s against list and binding and fills key. Errors: cursor_invalid.
	Decode(list string, binding []byte, s string, key any) error
}
