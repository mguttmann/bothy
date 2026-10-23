// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/runs/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/runs/app"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// Compile-time check: Repository is the outgoing port of the follow-up rows as well (M2-11).
var _ app.FollowupRepository = (*Repository)(nil)

// followupRow is the shape every follow-up query returns.
type followupRow struct {
	RunID, MessageID pgtype.UUID
	State            string
}

// toFollowup converts a row of run_followups.
func toFollowup(r followupRow) app.FollowupRow {
	return app.FollowupRow{RunID: idOf(r.RunID), MessageID: idOf(r.MessageID), State: domain.FollowupState(r.State)}
}

// uuids converts a list of ids for an array parameter.
func uuids(list []ids.ID) []pgtype.UUID {
	out := make([]pgtype.UUID, len(list))
	for i, id := range list {
		out[i] = uuid(id)
	}
	return out
}

// InsertFollowup implements app.FollowupRepository. Errors: internal outside a unit of work.
func (r *Repository) InsertFollowup(ctx context.Context, runID, messageID ids.ID) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	return mapError(q.InsertFollowup(ctx, sqlcgen.InsertFollowupParams{RunID: uuid(runID), MessageID: uuid(messageID)}),
		"inserting a follow-up")
}

// LockOpenFollowups implements app.FollowupRepository. Errors: internal outside a unit of work.
func (r *Repository) LockOpenFollowups(ctx context.Context, runID ids.ID) ([]app.FollowupRow, error) {
	q, err := r.write(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := q.LockOpenFollowups(ctx, uuid(runID))
	if err != nil {
		return nil, mapError(err, "locking the open follow-ups of a run")
	}
	out := make([]app.FollowupRow, len(rows))
	for i, row := range rows {
		out[i] = toFollowup(followupRow(row))
	}
	return out, nil
}

// RunFollowups implements app.FollowupRepository.
func (r *Repository) RunFollowups(ctx context.Context, runID ids.ID) ([]app.FollowupRow, error) {
	rows, err := r.read(ctx).RunFollowups(ctx, uuid(runID))
	if err != nil {
		return nil, mapError(err, "reading the follow-ups of a run")
	}
	out := make([]app.FollowupRow, len(rows))
	for i, row := range rows {
		out[i] = toFollowup(followupRow(row))
	}
	return out, nil
}

// FollowupStatesOf implements app.FollowupRepository; an empty list reads nothing.
func (r *Repository) FollowupStatesOf(ctx context.Context, runID ids.ID, messageIDs []ids.ID) ([]app.FollowupRow, error) {
	if len(messageIDs) == 0 {
		return nil, nil
	}
	rows, err := r.read(ctx).FollowupStatesOf(ctx, sqlcgen.FollowupStatesOfParams{RunID: uuid(runID), MessageIds: uuids(messageIDs)})
	if err != nil {
		return nil, mapError(err, "reading follow-up states")
	}
	out := make([]app.FollowupRow, len(rows))
	for i, row := range rows {
		out[i] = toFollowup(followupRow(row))
	}
	return out, nil
}

// LockFollowup implements app.FollowupRepository. Errors: internal outside a unit of work.
func (r *Repository) LockFollowup(ctx context.Context, runID, messageID ids.ID) (app.FollowupRow, bool, error) {
	q, err := r.write(ctx)
	if err != nil {
		return app.FollowupRow{}, false, err
	}
	row, err := q.LockFollowup(ctx, sqlcgen.LockFollowupParams{RunID: uuid(runID), MessageID: uuid(messageID)})
	if isNoRows(err) {
		return app.FollowupRow{}, false, nil
	}
	if err != nil {
		return app.FollowupRow{}, false, mapError(err, "locking a follow-up")
	}
	return toFollowup(followupRow(row)), true, nil
}

// SetFollowupStates implements app.FollowupRepository; an empty list writes nothing. Errors:
// internal outside a unit of work.
func (r *Repository) SetFollowupStates(ctx context.Context, runID ids.ID, messageIDs []ids.ID, state domain.FollowupState) error {
	if len(messageIDs) == 0 {
		return nil
	}
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	return mapError(q.SetFollowupStates(ctx, sqlcgen.SetFollowupStatesParams{State: string(state), RunID: uuid(runID), MessageIds: uuids(messageIDs)}),
		"storing follow-up states")
}

// MoveFollowups implements app.FollowupRepository; an empty list writes nothing. Errors: internal
// outside a unit of work.
func (r *Repository) MoveFollowups(ctx context.Context, fromRunID, toRunID ids.ID, messageIDs []ids.ID) error {
	if len(messageIDs) == 0 {
		return nil
	}
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	return mapError(q.MoveFollowups(ctx, sqlcgen.MoveFollowupsParams{ToRunID: uuid(toRunID), FromRunID: uuid(fromRunID), MessageIds: uuids(messageIDs)}),
		"moving follow-ups")
}

// FollowupOfMessage implements app.FollowupRepository.
func (r *Repository) FollowupOfMessage(ctx context.Context, messageID ids.ID) (app.FollowupRow, bool, error) {
	row, err := r.read(ctx).FollowupOfMessage(ctx, uuid(messageID))
	if isNoRows(err) {
		return app.FollowupRow{}, false, nil
	}
	if err != nil {
		return app.FollowupRow{}, false, mapError(err, "reading the follow-up of a message")
	}
	return toFollowup(followupRow(row)), true, nil
}

// RaiseRunVersion implements app.Repository: version + 1 of the run. Errors: not_found for an
// unknown run, internal outside a unit of work.
func (r *Repository) RaiseRunVersion(ctx context.Context, id ids.ID) (int64, error) {
	q, err := r.write(ctx)
	if err != nil {
		return 0, err
	}
	v, err := q.RaiseRunVersion(ctx, uuid(id))
	if isNoRows(err) {
		return 0, errs.New(errs.CodeNotFound, "runs: the run of a follow-up is gone")
	}
	return v, mapError(err, "raising the version of a run")
}

// LockActiveRunOfThread implements app.Repository. Errors: internal outside a unit of work.
func (r *Repository) LockActiveRunOfThread(ctx context.Context, threadID ids.ID) (app.RunRow, bool, error) {
	q, err := r.write(ctx)
	if err != nil {
		return app.RunRow{}, false, err
	}
	row, err := q.LockActiveRunOfThread(ctx, uuid(threadID))
	return one(row, err, "locking the active run of a thread")
}

// isNoRows reports the error of a :one query without a row.
func isNoRows(err error) bool { return errors.Is(err, pgx.ErrNoRows) }
