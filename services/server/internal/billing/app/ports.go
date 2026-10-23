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
}
