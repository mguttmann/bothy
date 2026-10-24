// SPDX-License-Identifier: Apache-2.0

// Package billingtest holds test helpers of the billing module (M2-22) that no production path
// has: InsertEntry writes a cost entry of any source, also a reconciled correction with
// supersedes_id, for the tests of the valid sum before M2-28 builds the reconciliation; and
// RecomputeComputed recomputes every computed entry from its price set with the function of the
// booking and reports differences (R-DM-23). Only tests import this package; it is kept out of
// the binary like identitytest.
//
// Example:
//
//	h := billingtest.New(ownerPool)
//	diffs, err := h.RecomputeComputed(ctx)
package billingtest

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/OWNER/bothy/services/server/internal/billing/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
)

// Helper runs the helpers over a pool; a pool of the database owner or of bothy_app (both may
// insert and read the billing tables).
type Helper struct {
	pool *pg.Pool
}

// New returns the helper over pool.
func New(pool *pg.Pool) *Helper { return &Helper{pool: pool} }

// Entry is a cost entry to insert; zero IDs and times are NULL, a zero ID a fresh UUIDv7.
type Entry struct {
	ID, RunID, AgentID, ThreadID, ChannelID ids.ID
	SpaceID, AgentSessionID                 ids.ID
	BillingMode, Provider, Model            string
	Tokens                                  domain.Tokens
	AmountMicros                            int64
	Currency                                string
	IsEstimate                              bool
	Source                                  string
	OccurredAt, ReconciledAt                time.Time
	SupersedesID                            ids.ID
}

// InsertEntry inserts e and returns its id. Errors: those of the database.
func (h *Helper) InsertEntry(ctx context.Context, e Entry) (ids.ID, error) {
	if e.ID.IsZero() {
		e.ID = ids.New()
	}
	_, err := h.pool.Exec(ctx, `INSERT INTO cost_entries (id, run_id, agent_id, thread_id, channel_id, space_id, agent_session_id,
	    billing_mode, provider, model, tokens_in, tokens_out, tokens_cache_read, tokens_cache_write, tokens_cache_write_1h,
	    amount_micros, currency, is_estimate, source, occurred_at, reconciled_at, supersedes_id)
	  VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22)`,
		e.ID.String(), e.RunID.String(), e.AgentID.String(), e.ThreadID.String(), e.ChannelID.String(), nullID(e.SpaceID), nullID(e.AgentSessionID), e.BillingMode, e.Provider,
		e.Model, e.Tokens.In, e.Tokens.Out, e.Tokens.CacheRead, e.Tokens.CacheWrite, e.Tokens.CacheWrite1h, e.AmountMicros, e.Currency,
		e.IsEstimate, e.Source, e.OccurredAt, nullTime(e.ReconciledAt), nullID(e.SupersedesID))
	if err != nil {
		return ids.ID{}, fmt.Errorf("billingtest: insert entry: %w", err)
	}
	return e.ID, nil
}

// Mismatch is a computed entry whose amount or currency differs from a recomputation.
type Mismatch struct {
	EntryID ids.ID
	// Stored is the booked amount and currency; Recomputed the result of domain.Compute now, with
	// its source ("unpriced" or "adapter_reported" when the price set is no longer complete).
	Stored, Recomputed domain.Priced
}

// RecomputeComputed recomputes every computed entry from S(provider, model, occurred_at) of the
// current price table with domain.Compute and returns the entries whose source, amount or
// currency differ. Errors: those of the database and of domain.Compute.
func (h *Helper) RecomputeComputed(ctx context.Context) ([]Mismatch, error) {
	prices, err := h.prices(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := h.pool.Query(ctx, `SELECT id::text, provider, model, tokens_in, tokens_out, tokens_cache_read, tokens_cache_write,
	    tokens_cache_write_1h, amount_micros, currency, occurred_at FROM cost_entries WHERE source = 'computed' ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("billingtest: read entries: %w", err)
	}
	type entry struct {
		id     ids.ID
		r      domain.Report
		stored domain.Priced
		at     time.Time
	}
	es, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (entry, error) {
		var e entry
		var id string
		err := r.Scan(&id, &e.r.Provider, &e.r.Model, &e.r.Tokens.In, &e.r.Tokens.Out, &e.r.Tokens.CacheRead, &e.r.Tokens.CacheWrite,
			&e.r.Tokens.CacheWrite1h, &e.stored.AmountMicros, &e.stored.Currency, &e.at)
		e.stored.Source = domain.SourceComputed
		if err == nil {
			e.id, err = ids.Parse(id)
		}
		return e, err
	})
	if err != nil {
		return nil, fmt.Errorf("billingtest: read entries: %w", err)
	}
	var out []Mismatch
	for _, e := range es {
		got, err := domain.Compute(e.r, domain.PriceSet(prices, e.r.Provider, e.r.Model, e.at))
		if err != nil {
			return nil, err
		}
		if got != e.stored {
			out = append(out, Mismatch{EntryID: e.id, Stored: e.stored, Recomputed: got})
		}
	}
	return out, nil
}

// prices reads every row of price_table.
func (h *Helper) prices(ctx context.Context) ([]domain.PriceRow, error) {
	rows, err := h.pool.Query(ctx, `SELECT id::text, provider, model, unit, price_micros, currency, valid_from, valid_to FROM price_table`)
	if err != nil {
		return nil, fmt.Errorf("billingtest: read prices: %w", err)
	}
	out, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (domain.PriceRow, error) {
		var p domain.PriceRow
		var id, unit string
		var to *time.Time
		err := r.Scan(&id, &p.Provider, &p.Model, &unit, &p.PriceMicros, &p.Currency, &p.ValidFrom, &to)
		p.Unit = domain.Unit(unit)
		if to != nil {
			p.ValidTo = *to
		}
		if err == nil {
			p.ID, err = ids.Parse(id)
		}
		return p, err
	})
	if err != nil {
		return nil, fmt.Errorf("billingtest: read prices: %w", err)
	}
	return out, nil
}

// nullID is nil for the zero ID.
func nullID(id ids.ID) any {
	if id.IsZero() {
		return nil
	}
	return id.String()
}

// nullTime is nil for the zero time.
func nullTime(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t
}
