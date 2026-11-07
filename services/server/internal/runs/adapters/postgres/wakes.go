// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/runs/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/runs/app"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// Compile-time check: the repository serves the wake-ups (AR-04).
var _ app.WakeRepository = (*Repository)(nil)

// InsertWake implements app.WakeRepository: no returned row means the idempotency key of the
// wake-up exists (ON CONFLICT DO NOTHING), which gives inserted false and no error.
func (r *Repository) InsertWake(ctx context.Context, w app.NewWake) (app.WakeRow, bool, error) {
	q, err := r.write(ctx)
	if err != nil {
		return app.WakeRow{}, false, err
	}
	row, err := q.InsertWake(ctx, sqlcgen.InsertWakeParams{
		ID: uuid(w.ID), AgentID: uuid(w.AgentID), ThreadID: uuid(w.ThreadID), SourceRunID: uuid(w.SourceRunID),
		TriggeredBy: uuid(w.TriggeredBy), Kind: string(w.Kind), Condition: w.Condition, Label: w.Label, Reason: string(w.Reason),
		Outage: optionalInt4(w.Outage), NotBefore: ts(w.NotBefore), Deadline: ts(w.Deadline), State: string(w.State),
		StateReason: optionalText(w.StateReason), CreatedAt: ts(w.CreatedAt),
	})
	return oneWake(row, err, "inserting a wake-up")
}

// LockWake implements app.WakeRepository.
func (r *Repository) LockWake(ctx context.Context, id ids.ID) (app.WakeRow, bool, error) {
	q, err := r.write(ctx)
	if err != nil {
		return app.WakeRow{}, false, err
	}
	row, err := q.LockWake(ctx, uuid(id))
	return oneWake(row, err, "locking a wake-up")
}

// WakeByOutage implements app.WakeRepository.
func (r *Repository) WakeByOutage(ctx context.Context, sourceRunID ids.ID, outage int) (app.WakeRow, bool, error) {
	q, err := r.write(ctx)
	if err != nil {
		return app.WakeRow{}, false, err
	}
	row, err := q.WakeByOutage(ctx, sqlcgen.WakeByOutageParams{SourceRunID: uuid(sourceRunID), Outage: optionalInt4(outage)})
	return oneWake(row, err, "reading the wake-up of an outage")
}

// WakeBySourceReason implements app.WakeRepository.
func (r *Repository) WakeBySourceReason(ctx context.Context, sourceRunID ids.ID, reason domain.WakeReason) (app.WakeRow, bool, error) {
	q, err := r.write(ctx)
	if err != nil {
		return app.WakeRow{}, false, err
	}
	row, err := q.WakeBySourceReason(ctx, sqlcgen.WakeBySourceReasonParams{SourceRunID: uuid(sourceRunID), Reason: string(reason)})
	return oneWake(row, err, "reading the recovery wake-up of a run")
}

// UpdateWake implements app.WakeRepository.
func (r *Repository) UpdateWake(ctx context.Context, w app.WakeWrite) (app.WakeRow, bool, error) {
	q, err := r.write(ctx)
	if err != nil {
		return app.WakeRow{}, false, err
	}
	p := sqlcgen.UpdateWakeParams{
		State: string(w.State), StateReason: optionalText(w.StateReason), At: optionalTS(w.At), NotBefore: optionalTS(w.NotBefore),
		Deadline: optionalTS(w.Deadline), ClearFired: w.ClearFired, FiredRunID: optionalUUID(w.FiredRunID),
		FiredIntoRunID: optionalUUID(w.FiredIntoRunID), ID: uuid(w.ID), Version: w.Version,
	}
	if w.ResultText != "" {
		p.ResultText = pgtype.Text{String: w.ResultText, Valid: true}
	}
	row, err := q.UpdateWake(ctx, p)
	return oneWake(row, err, "changing a wake-up")
}

// CountPendingWakes implements app.WakeRepository.
func (r *Repository) CountPendingWakes(ctx context.Context, threadID ids.ID) (int, error) {
	n, err := r.read(ctx).CountPendingWakes(ctx, uuid(threadID))
	return int(n), mapError(err, "counting the pending wake-ups of a thread")
}

// CountUsageWakes implements app.WakeRepository.
func (r *Repository) CountUsageWakes(ctx context.Context, sourceRunID ids.ID) (int, error) {
	n, err := r.read(ctx).CountUsageWakes(ctx, uuid(sourceRunID))
	return int(n), mapError(err, "counting the usage-limit wake-ups of a run")
}

// PendingWakesOfAgent implements app.WakeRepository.
func (r *Repository) PendingWakesOfAgent(ctx context.Context, agentID ids.ID) ([]app.WakeRow, error) {
	rows, err := r.read(ctx).PendingWakesOfAgent(ctx, uuid(agentID))
	return wakes(rows, err, "reading the pending wake-ups of an agent")
}

// ThreadWakes implements app.WakeRepository.
func (r *Repository) ThreadWakes(ctx context.Context, threadID ids.ID, after *app.WakeKey, limit int) ([]app.WakeRow, error) {
	p := sqlcgen.ThreadWakesParams{ThreadID: uuid(threadID), PageLimit: int32(min(limit, maxPageRows))} // #nosec G115 -- bounded by maxPageRows
	if after != nil {
		p.HasAfter, p.AfterCreatedAt, p.AfterID = true, ts(after.CreatedAt), uuid(after.ID)
	} else {
		p.AfterCreatedAt, p.AfterID = ts(time.Time{}), uuid(ids.ID{})
	}
	rows, err := r.read(ctx).ThreadWakes(ctx, p)
	return wakes(rows, err, "reading the wake-ups of a thread")
}

// ReadyWakesOfThread implements app.WakeRepository.
func (r *Repository) ReadyWakesOfThread(ctx context.Context, threadID ids.ID, now time.Time) ([]ids.ID, error) {
	rows, err := r.read(ctx).ReadyWakesOfThread(ctx, sqlcgen.ReadyWakesOfThreadParams{ThreadID: uuid(threadID), Now: ts(now)})
	if err != nil {
		return nil, mapError(err, "reading the ready wake-ups of a thread")
	}
	out := make([]ids.ID, len(rows))
	for i, id := range rows {
		out[i] = idOf(id)
	}
	return out, nil
}

// PendingWakeCounts implements app.WakeRepository.
func (r *Repository) PendingWakeCounts(ctx context.Context) (map[domain.WakeKind]int64, error) {
	rows, err := r.read(ctx).PendingWakeCounts(ctx)
	if err != nil {
		return nil, mapError(err, "counting the pending wake-ups")
	}
	out := make(map[domain.WakeKind]int64, len(rows))
	for _, row := range rows {
		out[domain.WakeKind(row.Kind)] = row.N
	}
	return out, nil
}

// LockAutonomy implements app.WakeRepository.
func (r *Repository) LockAutonomy(ctx context.Context, threadID, personID ids.ID) (domain.Autonomy, bool, error) {
	q, err := r.write(ctx)
	if err != nil {
		return domain.Autonomy{}, false, err
	}
	row, err := q.LockAutonomy(ctx, sqlcgen.LockAutonomyParams{ThreadID: uuid(threadID), PersonID: uuid(personID)})
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Autonomy{}, false, nil
	}
	if err != nil {
		return domain.Autonomy{}, false, mapError(err, "locking the autonomy counts")
	}
	return domain.Autonomy{Chain: int(row.ChainCount), Idle: int(row.IdleCount), EndedReason: row.EndedReason.String}, true, nil
}

// SaveAutonomy implements app.WakeRepository.
func (r *Repository) SaveAutonomy(ctx context.Context, threadID, personID ids.ID, a domain.Autonomy) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	err = q.SaveAutonomy(ctx, sqlcgen.SaveAutonomyParams{
		ThreadID: uuid(threadID), PersonID: uuid(personID), ChainCount: int32(min(a.Chain, maxCount)), // #nosec G115 -- bounded by maxCount
		IdleCount: int32(min(a.Idle, maxCount)), EndedReason: optionalText(a.EndedReason), UpdatedAt: ts(time.Now()), // #nosec G115 -- bounded by maxCount
	})
	return mapError(err, "saving the autonomy counts")
}

// ResetAutonomy implements app.WakeRepository.
func (r *Repository) ResetAutonomy(ctx context.Context, threadID, personID ids.ID) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	err = q.ResetAutonomy(ctx, sqlcgen.ResetAutonomyParams{ThreadID: uuid(threadID), PersonID: uuid(personID), UpdatedAt: ts(time.Now())})
	return mapError(err, "resetting the autonomy counts")
}

// The bounds of the integer conversions: a page is never larger, a count never higher (integer
// columns of 03 run_autonomy).
const (
	maxPageRows = 1 << 20
	maxCount    = 1<<31 - 1
)

// optionalInt4 is an integer column, NULL for 0.
func optionalInt4(v int) pgtype.Int4 {
	return pgtype.Int4{Int32: int32(min(v, maxCount)), Valid: v != 0} // #nosec G115 -- bounded by maxCount
}

// oneWake converts the result of a single-row query; no rows is found false.
func oneWake(row sqlcgen.RunWake, err error, what string) (app.WakeRow, bool, error) {
	if errors.Is(err, pgx.ErrNoRows) {
		return app.WakeRow{}, false, nil
	}
	if err != nil {
		return app.WakeRow{}, false, mapError(err, what)
	}
	return toWake(row), true, nil
}

// wakes converts a list result.
func wakes(rows []sqlcgen.RunWake, err error, what string) ([]app.WakeRow, error) {
	if err != nil {
		return nil, mapError(err, what)
	}
	out := make([]app.WakeRow, len(rows))
	for i, row := range rows {
		out[i] = toWake(row)
	}
	return out, nil
}

// toWake converts a row of run_wakes.
func toWake(row sqlcgen.RunWake) app.WakeRow {
	return app.WakeRow{
		ID: idOf(row.ID), AgentID: idOf(row.AgentID), ThreadID: idOf(row.ThreadID), SourceRunID: idOf(row.SourceRunID),
		TriggeredBy: idOf(row.TriggeredBy), Kind: domain.WakeKind(row.Kind), Reason: domain.WakeReason(row.Reason),
		State: domain.WakeState(row.State), StateReason: row.StateReason.String, Condition: row.Condition, Label: row.Label,
		Outage: int(row.Outage.Int32), NotBefore: timeOf(row.NotBefore), Deadline: timeOf(row.Deadline), CreatedAt: timeOf(row.CreatedAt),
		EndedAt: timeOf(row.EndedAt), ResultText: row.ResultText, FiredRunID: idOf(row.FiredRunID), FiredIntoRunID: idOf(row.FiredIntoRunID),
		Version: row.Version,
	}
}
