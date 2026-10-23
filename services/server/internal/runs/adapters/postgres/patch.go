// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/runs/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/runs/app"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// PatchSteps implements app.Repository: every step of the run by ordinal, the detail only for the
// summary step. Errors: internal outside a unit of work (the caller holds the run row lock).
func (r *Repository) PatchSteps(ctx context.Context, runID ids.ID) ([]app.StepRow, error) {
	q, err := r.write(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := q.PatchSteps(ctx, uuid(runID))
	if err != nil {
		return nil, mapError(err, "reading the steps of a patch")
	}
	out := make([]app.StepRow, len(rows))
	for i, row := range rows {
		out[i] = app.StepRow{
			ID: idOf(row.ID), RunID: idOf(row.RunID), Ordinal: int(row.Ordinal), ParentStepID: idOf(row.ParentStepID),
			Kind: row.Kind, Name: row.Name, Title: row.Title, Status: row.Status, Detail: row.Detail,
			StartedAt: timeOf(row.StartedAt), FinishedAt: timeOf(row.FinishedAt),
		}
	}
	return out, nil
}

// WritePatch implements app.Repository: the run row first (version + 1, stall_warned_at NULL),
// then the deleted, updated and inserted steps (03 "Lock order": the runs row, then run_steps).
// Deletes come before the updates and inserts, because the summary step takes the ordinal of a
// removed step. Errors: not_found for an unknown run, internal outside a unit of work.
func (r *Repository) WritePatch(ctx context.Context, w app.PatchWrite) (app.RunRow, error) {
	q, err := r.write(ctx)
	if err != nil {
		return app.RunRow{}, err
	}
	ch := w.Change
	p := sqlcgen.PatchRunParams{
		SetStatusLine: ch.StatusLine.Set, StatusLine: ch.StatusLine.Value,
		SetLastTextExcerpt: ch.LastTextExcerpt.Set, LastTextExcerpt: ch.LastTextExcerpt.Value,
		SetModel: ch.Model.Set, Model: ch.Model.Value, Metrics: ch.Metrics, Plan: ch.Plan, ID: uuid(w.ID),
	}
	if ch.Cost != nil {
		p.SetCost, p.CostMicros, p.Currency, p.CostIsEstimate = true, ch.Cost.AmountMicros, ch.Cost.Currency, ch.Cost.IsEstimate
	}
	stored, err := q.PatchRun(ctx, p)
	row, found, err := one(stored, err, "storing a patch")
	if err != nil {
		return app.RunRow{}, err
	}
	if !found {
		return app.RunRow{}, errs.New(errs.CodeNotFound, "runs: the run of a patch is gone")
	}
	if err := writeSteps(ctx, q, w.ID, ch); err != nil {
		return app.RunRow{}, err
	}
	return row, nil
}

// writeSteps deletes, updates and inserts the steps of ch for run, in that order.
func writeSteps(ctx context.Context, q *sqlcgen.Queries, run ids.ID, ch domain.Change) error {
	if len(ch.DeleteSteps) > 0 {
		in := make([]pgtype.UUID, 0, len(ch.DeleteSteps))
		for _, id := range ch.DeleteSteps {
			in = append(in, uuid(id))
		}
		if err := q.DeletePatchSteps(ctx, sqlcgen.DeletePatchStepsParams{RunID: uuid(run), Ids: in}); err != nil {
			return mapError(err, "deleting summarized steps")
		}
	}
	for _, u := range ch.UpdateSteps {
		if err := q.UpdatePatchStep(ctx, stepUpdate(run, u)); err != nil {
			return mapError(err, "updating a step")
		}
	}
	for _, s := range ch.InsertSteps {
		if s.Ordinal < 1 || s.Ordinal > maxInt32 {
			return errs.New(errs.CodeInternalError, "runs: a step without a valid ordinal")
		}
		err := q.InsertPatchStep(ctx, sqlcgen.InsertPatchStepParams{
			ID: uuid(s.ID), RunID: uuid(run), Ordinal: int32(s.Ordinal), ParentStepID: optionalUUID(s.ParentStepID),
			Kind: s.Kind, Name: s.Name, Title: s.Title, Status: s.Status, Detail: detailOf(s.Detail),
			StartedAt: ts(s.StartedAt), FinishedAt: optionalTS(s.FinishedAt),
		})
		if err != nil {
			return mapError(err, "inserting a step")
		}
	}
	return nil
}

// stepUpdate converts the change of one step; unset members are NULL and keep their column.
func stepUpdate(runID ids.ID, u domain.StepUpdate) sqlcgen.UpdatePatchStepParams {
	p := sqlcgen.UpdatePatchStepParams{RunID: uuid(runID), ID: uuid(u.ID), Detail: u.Detail}
	if u.Status != nil {
		p.Status = pgtype.Text{String: *u.Status, Valid: true}
	}
	if u.Title != nil {
		p.Title = pgtype.Text{String: *u.Title, Valid: true}
	}
	if u.FinishedAt != nil {
		p.FinishedAt = optionalTS(*u.FinishedAt)
	}
	if u.Ordinal != nil && *u.Ordinal >= 1 && *u.Ordinal <= maxInt32 {
		p.Ordinal = pgtype.Int4{Int32: int32(*u.Ordinal), Valid: true}
	}
	return p
}

// detailOf returns the stored detail, the empty object for none.
func detailOf(d []byte) []byte {
	if len(d) == 0 {
		return []byte("{}")
	}
	return d
}

// ActiveRunsOfThreads implements app.Repository.
func (r *Repository) ActiveRunsOfThreads(ctx context.Context, threadIDs []ids.ID) ([]app.RunRow, error) {
	in := make([]pgtype.UUID, 0, len(threadIDs))
	for _, id := range threadIDs {
		in = append(in, uuid(id))
	}
	rows, err := r.read(ctx).ActiveRunsOfThreads(ctx, in)
	return runs(rows, err, "reading the active runs of threads")
}
