// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/terminal/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/terminal/domain"
)

// The queries of the control connection (M2-43, queries/control.sql).

// UpdateHostLogin implements app.Store.
func (r *Repository) UpdateHostLogin(ctx context.Context, id ids.ID, hostVersion string, seen time.Time, version int64) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	n, err := q.UpdateHostLogin(ctx, sqlcgen.UpdateHostLoginParams{
		ID: uuid(id), ReportedHostVersion: optionalText(hostVersion), LastSeenAt: optionalTime(seen), Version: version,
	})
	return oneRow(n, err, "writing the login of a host")
}

// TouchLastSeen implements app.Store.
func (r *Repository) TouchLastSeen(ctx context.Context, id ids.ID, at, olderThan time.Time) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	_, err = q.TouchHostLastSeen(ctx, sqlcgen.TouchHostLastSeenParams{ID: uuid(id), LastSeenAt: optionalTime(at), OlderThan: optionalTime(olderThan)})
	return mapError(err, "writing last_seen_at of a host")
}

// UpdateHostReport implements app.Store: the lists in the forms of host_profiles.v1 and
// host_doctor.v1, in the reported order.
func (r *Repository) UpdateHostReport(ctx context.Context, id ids.ID, rep domain.Report, version int64) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	profiles, err := json.Marshal(nonNilList(rep.Profiles))
	if err != nil {
		return errs.Wrap(errs.CodeInternalError, err, "terminal: encoding the reported profiles")
	}
	doctor, err := json.Marshal(nonNilList(rep.Doctor))
	if err != nil {
		return errs.Wrap(errs.CodeInternalError, err, "terminal: encoding the reported checks")
	}
	n, err := q.UpdateHostReport(ctx, sqlcgen.UpdateHostReportParams{
		ID: uuid(id), ReportedProfiles: profiles, ReportedDoctor: doctor,
		ReportedIsServerHost:        pgtype.Bool{Bool: rep.IsServerHost, Valid: true},
		ReportedRequireLocalConfirm: pgtype.Bool{Bool: rep.RequireLocalConfirm, Valid: true},
		ReportedOsVersion:           optionalText(rep.OSVersion), Version: version,
	})
	return oneRow(n, err, "writing the report of a host")
}

// nonNilList returns list, or the empty list for nil (the reported columns are never NULL).
func nonNilList[T any](list []T) []T {
	if list == nil {
		return []T{}
	}
	return list
}

// LockSession implements app.Store.
func (r *Repository) LockSession(ctx context.Context, id ids.ID) (domain.Session, bool, error) {
	q, err := r.write(ctx)
	if err != nil {
		return domain.Session{}, false, err
	}
	row, err := q.LockSession(ctx, uuid(id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Session{}, false, nil
	}
	if err != nil {
		return domain.Session{}, false, mapError(err, "locking a session")
	}
	return session(row), true, nil
}

// SessionsByIDs implements app.Store.
func (r *Repository) SessionsByIDs(ctx context.Context, list []ids.ID) ([]domain.Session, error) {
	if len(list) == 0 {
		return nil, nil
	}
	rows, err := r.read(ctx).SessionsByIDs(ctx, uuids(list))
	if err != nil {
		return nil, mapError(err, "reading listed sessions")
	}
	return sessions(rows), nil
}

// StartSession implements app.Store.
func (r *Repository) StartSession(ctx context.Context, id ids.ID, at time.Time) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	n, err := q.StartSession(ctx, sqlcgen.StartSessionParams{ID: uuid(id), StartedAt: optionalTime(at)})
	return oneRow(n, err, "starting a session")
}

// DetachSession implements app.Store.
func (r *Repository) DetachSession(ctx context.Context, id ids.ID, at time.Time) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	n, err := q.DetachSession(ctx, sqlcgen.DetachSessionParams{ID: uuid(id), DetachedAt: optionalTime(at)})
	return oneRow(n, err, "detaching a session")
}

// CloseSessionReported implements app.Store.
func (r *Repository) CloseSessionReported(ctx context.Context, id ids.ID, end domain.HostSessionEnd, at time.Time) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	code := pgtype.Int4{}
	if end.ExitCode != nil {
		code = pgtype.Int4{Int32: int32(min(max(*end.ExitCode, math.MinInt32), math.MaxInt32)), Valid: true}
	}
	n, err := q.CloseSessionReported(ctx, sqlcgen.CloseSessionReportedParams{
		ID: uuid(id), ClosedAt: optionalTime(at), CloseReason: optionalText(string(end.Reason)), CloseDetail: optionalText(string(end.Detail)),
		ExitCode: code,
	})
	return oneRow(n, err, "closing a session the host ended")
}

// sessions converts rows of terminal_sessions.
func sessions(rows []sqlcgen.TerminalSession) []domain.Session {
	out := make([]domain.Session, 0, len(rows))
	for _, row := range rows {
		out = append(out, session(row))
	}
	return out
}

// session converts one row of terminal_sessions (metadata only).
func session(row sqlcgen.TerminalSession) domain.Session {
	s := domain.Session{
		ID: idOf(row.ID), HostID: idOf(row.HostID), UserID: idOf(row.UserID), State: row.State, OpenedAt: timeOf(row.OpenedAt),
		BytesIn: row.BytesIn, BytesOut: row.BytesOut, DeviceID: idOf(row.DeviceID), Profile: row.Profile,
		StartedAt: timeOf(row.StartedAt), DetachedAt: timeOf(row.DetachedAt), ClosedAt: timeOf(row.ClosedAt),
		CloseReason: domain.CloseReason(row.CloseReason.String), CloseDetail: domain.CloseDetail(row.CloseDetail.String), Version: row.Version,
	}
	if row.ExitCode.Valid {
		c := int(row.ExitCode.Int32)
		s.ExitCode = &c
	}
	sessionGrantColumns(&s, row)
	return s
}
