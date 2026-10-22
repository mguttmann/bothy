// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"errors"
	"math"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/terminal/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/terminal/app"
	"github.com/OWNER/bothy/services/server/internal/terminal/domain"
)

// The queries of the end of sessions (M2-46, queries/ending.sql).

// EndSession implements app.Store: the one closing statement of the module.
func (r *Repository) EndSession(ctx context.Context, id ids.ID, end domain.End, at time.Time) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	p := sqlcgen.EndTerminalSessionParams{
		ID: uuid(id), ClosedAt: optionalTime(at), CloseReason: optionalText(string(end.Reason)), CloseDetail: optionalText(string(end.Detail)),
		ClosedBy: optionalUUID(end.ClosedBy),
	}
	if end.ExitCode != nil {
		p.ExitCode = pgtype.Int4{Int32: int32(min(max(*end.ExitCode, math.MinInt32), math.MaxInt32)), Valid: true}
	}
	if pr := end.Progress; pr != nil {
		p.BytesIn, p.BytesOut, p.LastInputAt = max(pr.BytesIn, 0), max(pr.BytesOut, 0), optionalTime(pr.LastInputAt)
	}
	n, err := q.EndTerminalSession(ctx, p)
	return oneRow(n, err, "ending a terminal session")
}

// LockOpenSessionsOfUsers implements app.Store.
func (r *Repository) LockOpenSessionsOfUsers(ctx context.Context, userIDs []ids.ID) ([]domain.Session, error) {
	if len(userIDs) == 0 {
		return nil, nil
	}
	q, err := r.write(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := q.LockOpenSessionsOfUsers(ctx, uuids(userIDs))
	if err != nil {
		return nil, mapError(err, "locking the open sessions of people")
	}
	return sessions(rows), nil
}

// LockOpenSessionsOfDevice implements app.Store.
func (r *Repository) LockOpenSessionsOfDevice(ctx context.Context, deviceID ids.ID) ([]domain.Session, error) {
	q, err := r.write(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := q.LockOpenSessionsOfDevice(ctx, uuid(deviceID))
	if err != nil {
		return nil, mapError(err, "locking the open sessions of a device")
	}
	return sessions(rows), nil
}

// LockOpenSessionsOfAuthSessions implements app.Store.
func (r *Repository) LockOpenSessionsOfAuthSessions(ctx context.Context, authSessionIDs []ids.ID) ([]domain.Session, error) {
	if len(authSessionIDs) == 0 {
		return nil, nil
	}
	q, err := r.write(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := q.LockOpenSessionsOfAuthSessions(ctx, uuids(authSessionIDs))
	if err != nil {
		return nil, mapError(err, "locking the open sessions of login sessions")
	}
	return sessions(rows), nil
}

// SweepCandidates implements app.Store.
func (r *Repository) SweepCandidates(ctx context.Context) ([]app.SweepCandidate, error) {
	rows, err := r.read(ctx).SweepCandidates(ctx)
	if err != nil {
		return nil, mapError(err, "reading the open terminal sessions")
	}
	out := make([]app.SweepCandidate, 0, len(rows))
	for _, row := range rows {
		out = append(out, app.SweepCandidate{SessionID: idOf(row.ID), AuthSessionID: idOf(row.AuthSessionID), HostState: row.HostState,
			LocalConfirm: row.ReportedRequireLocalConfirm.Valid && row.ReportedRequireLocalConfirm.Bool})
	}
	return out, nil
}

// LockPersonForOpen implements app.Store.
func (r *Repository) LockPersonForOpen(ctx context.Context, userID ids.ID) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	return mapError(q.LockPersonForOpen(ctx, uuid(userID)), "locking the person of an opening")
}

// CountOpenSessions implements app.Store.
func (r *Repository) CountOpenSessions(ctx context.Context, userID, hostID ids.ID) (ofPerson, ofHost int, err error) {
	row, err := r.read(ctx).CountOpenSessions(ctx, sqlcgen.CountOpenSessionsParams{PersonID: uuid(userID), OfHostID: uuid(hostID)})
	if err != nil {
		return 0, 0, mapError(err, "counting the open terminal sessions")
	}
	if row.OfPerson > math.MaxInt32 || row.OfHost > math.MaxInt32 {
		return 0, 0, errs.Wrap(errs.CodeInternalError, errors.New("count out of range"), "terminal: counting the open sessions")
	}
	return int(row.OfPerson), int(row.OfHost), nil
}
