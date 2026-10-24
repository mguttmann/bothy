// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"math"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/OWNER/bothy/services/server/internal/billing"
	"github.com/OWNER/bothy/services/server/internal/billing/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/billing/app"
	"github.com/OWNER/bothy/services/server/internal/billing/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// LockReconcile implements app.Repository: one statement per run in the given order.
func (r *Repository) LockReconcile(ctx context.Context, runIDs []ids.ID) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	for _, id := range runIDs {
		if err := q.LockReconcileRun(ctx, uuid(id)); err != nil {
			return mapError(err, "locking a run for the reconciliation")
		}
	}
	return nil
}

// LockReconcileShared implements app.Repository.
func (r *Repository) LockReconcileShared(ctx context.Context, runIDs []ids.ID) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	for _, id := range runIDs {
		if err := q.LockReconcileRunShared(ctx, uuid(id)); err != nil {
			return mapError(err, "locking a run for the reconciliation job")
		}
	}
	return nil
}

// SessionsOfRuns implements app.Repository.
func (r *Repository) SessionsOfRuns(ctx context.Context, runIDs []ids.ID) ([]ids.ID, error) {
	rows, err := r.read(ctx).ReconcileSessionsOfRuns(ctx, uuids(runIDs))
	if err != nil {
		return nil, mapError(err, "reading the sessions of runs")
	}
	out := make([]ids.ID, len(rows))
	for i, id := range rows {
		out[i] = idOf(id)
	}
	return out, nil
}

// LockSessions implements app.Repository: one statement per session in the given order.
func (r *Repository) LockSessions(ctx context.Context, sessionIDs []ids.ID) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	for _, id := range sessionIDs {
		if err := q.LockReconcileSession(ctx, uuid(id)); err != nil {
			return mapError(err, "locking a session for the reconciliation job")
		}
	}
	return nil
}

// LockSessionShared implements app.Repository.
func (r *Repository) LockSessionShared(ctx context.Context, sessionID ids.ID) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	return mapError(q.LockReconcileSessionShared(ctx, uuid(sessionID)), "locking a session for a booking")
}

// ValidEntries implements app.Repository.
func (r *Repository) ValidEntries(ctx context.Context, runIDs []ids.ID, from, to time.Time) ([]domain.Entry, error) {
	rows, err := r.read(ctx).ValidEntriesInPeriod(ctx, sqlcgen.ValidEntriesInPeriodParams{RunIds: uuids(runIDs), FromAt: ts(from), ToAt: ts(to)})
	if err != nil {
		return nil, mapError(err, "reading the valid entries of runs")
	}
	out := make([]domain.Entry, len(rows))
	for i, e := range rows {
		out[i] = domain.Entry{ID: idOf(e.ID), RunID: idOf(e.RunID), AgentID: idOf(e.AgentID), ThreadID: idOf(e.ThreadID),
			ChannelID: idOf(e.ChannelID), SpaceID: idOf(e.SpaceID), AgentSessionID: idOf(e.AgentSessionID), BillingMode: e.BillingMode,
			Provider: e.Provider, Model: e.Model, Tokens: domain.Tokens{In: e.TokensIn, Out: e.TokensOut, CacheRead: e.TokensCacheRead,
				CacheWrite: e.TokensCacheWrite, CacheWrite1h: e.TokensCacheWrite1h}, AmountMicros: e.AmountMicros, Currency: e.Currency,
			IsEstimate: e.IsEstimate, Source: e.Source, OccurredAt: timeOf(e.OccurredAt), ReconciledAt: timeOf(e.ReconciledAt),
			SupersedesID: idOf(e.SupersedesID)}
	}
	return out, nil
}

// InsertReconciled implements app.Repository.
func (r *Repository) InsertReconciled(ctx context.Context, es []domain.Entry) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	for _, e := range es {
		if err := q.InsertReconciledEntry(ctx, sqlcgen.InsertReconciledEntryParams{
			ID: uuid(e.ID), RunID: uuid(e.RunID), AgentID: uuid(e.AgentID), ThreadID: uuid(e.ThreadID), ChannelID: uuid(e.ChannelID),
			SpaceID: optionalUUID(e.SpaceID), AgentSessionID: optionalUUID(e.AgentSessionID), BillingMode: e.BillingMode,
			Provider: e.Provider, Model: e.Model, TokensIn: e.Tokens.In, TokensOut: e.Tokens.Out, TokensCacheRead: e.Tokens.CacheRead,
			TokensCacheWrite: e.Tokens.CacheWrite, TokensCacheWrite1h: e.Tokens.CacheWrite1h, AmountMicros: e.AmountMicros,
			Currency: e.Currency, OccurredAt: ts(e.OccurredAt), ReconciledAt: ts(e.ReconciledAt), SupersedesID: uuid(e.SupersedesID),
		}); err != nil {
			return mapError(err, "inserting a reconciled entry")
		}
	}
	return nil
}

// RunTotals implements app.Repository.
func (r *Repository) RunTotals(ctx context.Context, runIDs []ids.ID) ([]app.CurrencyTotal, error) {
	rows, err := r.read(ctx).ReconcileRunTotals(ctx, uuids(runIDs))
	if err != nil {
		return nil, mapError(err, "summing the valid entries of runs")
	}
	out := make([]app.CurrencyTotal, len(rows))
	for i, t := range rows {
		out[i] = app.CurrencyTotal{ID: idOf(t.RunID), Currency: t.Currency, AmountMicros: t.AmountMicros, IsEstimate: t.IsEstimate}
	}
	return out, nil
}

// SessionTotals implements app.Repository.
func (r *Repository) SessionTotals(ctx context.Context, runIDs []ids.ID) ([]app.CurrencyTotal, error) {
	rows, err := r.read(ctx).ReconcileSessionTotals(ctx, uuids(runIDs))
	if err != nil {
		return nil, mapError(err, "summing the valid entries of sessions")
	}
	out := make([]app.CurrencyTotal, len(rows))
	for i, t := range rows {
		out[i] = app.CurrencyTotal{ID: idOf(t.AgentSessionID), Currency: t.Currency, AmountMicros: t.AmountMicros, IsEstimate: t.IsEstimate}
	}
	return out, nil
}

// Estimates implements app.Repository.
func (r *Repository) Estimates(ctx context.Context, f app.EstimatesFilter, limit int) ([]billing.Estimate, error) {
	rows, err := r.read(ctx).CostEstimates(ctx, sqlcgen.CostEstimatesParams{AgentIds: uuids(f.AgentIDs), FromAt: ts(f.From), ToAt: ts(f.To),
		AfterRun: optionalUUID(f.After), RowLimit: int32(min(limit, math.MaxInt32))}) //nolint:gosec // G115: bounded above
	if err != nil {
		return nil, mapError(err, "reading the cost estimates")
	}
	out := make([]billing.Estimate, len(rows))
	for i, e := range rows {
		out[i] = billing.Estimate{RunID: idOf(e.RunID), AgentID: idOf(e.AgentID), Provider: e.Provider, Currency: e.Currency,
			EstimateMicros: e.EstimateMicros, Reconciled: e.Reconciled}
	}
	return out, nil
}

// uuids converts ids for an array parameter.
func uuids(in []ids.ID) []pgtype.UUID {
	out := make([]pgtype.UUID, len(in))
	for i, id := range in {
		out[i] = uuid(id)
	}
	return out
}
