// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"errors"
	"math"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/terminal/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/terminal/domain"
)

// The queries of the terminal sessions (M2-44, queries/sessions.sql).

// InsertSession implements app.Store.
func (r *Repository) InsertSession(ctx context.Context, s domain.Session) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	return mapError(q.InsertTerminalSession(ctx, sqlcgen.InsertTerminalSessionParams{
		ID: uuid(s.ID), HostID: uuid(s.HostID), UserID: uuid(s.UserID), HostSessionUserID: uuid(s.HostSessionUserID),
		DeviceID: uuid(s.DeviceID), AuthSessionID: uuid(s.AuthSessionID), Profile: s.Profile, TokenJti: uuid(s.TokenJTI),
		KeyGeneration: int32(min(max(s.KeyGeneration, math.MinInt32), math.MaxInt32)), OpenedAt: optionalTime(s.OpenedAt),
	}), "inserting a terminal session")
}

// Session implements app.Store.
func (r *Repository) Session(ctx context.Context, id ids.ID) (domain.Session, bool, error) {
	row, err := r.read(ctx).GetTerminalSession(ctx, uuid(id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Session{}, false, nil
	}
	if err != nil {
		return domain.Session{}, false, mapError(err, "reading a terminal session")
	}
	return session(row), true, nil
}

// AttachSession implements app.Store.
func (r *Repository) AttachSession(ctx context.Context, id, deviceID, authSessionID, jti ids.ID) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	n, err := q.AttachTerminalSession(ctx, sqlcgen.AttachTerminalSessionParams{
		ID: uuid(id), DeviceID: uuid(deviceID), AuthSessionID: uuid(authSessionID), TokenJti: uuid(jti),
	})
	return oneRow(n, err, "attaching a terminal session")
}

// ActivateSession implements app.Store.
func (r *Repository) ActivateSession(ctx context.Context, id ids.ID) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	n, err := q.ActivateTerminalSession(ctx, uuid(id))
	return oneRow(n, err, "activating a terminal session")
}

// AddCounters implements app.Store.
func (r *Repository) AddCounters(ctx context.Context, id ids.ID, in, out int64, lastInput time.Time) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	_, err = q.AddTerminalSessionCounters(ctx, sqlcgen.AddTerminalSessionCountersParams{
		ID: uuid(id), BytesIn: in, BytesOut: out, LastInputAt: optionalTime(lastInput),
	})
	return mapError(err, "writing the counters of a terminal session")
}

// ListSessions implements app.Store.
func (r *Repository) ListSessions(ctx context.Context, userID, hostID ids.ID, state string, beforeID ids.ID, limit int) ([]domain.Session, error) {
	rows, err := r.read(ctx).ListTerminalSessions(ctx, sqlcgen.ListTerminalSessionsParams{
		UserID: uuid(userID), HostID: uuid(hostID), State: state, BeforeID: uuid(beforeID), PageLimit: int32(min(max(limit, 0), math.MaxInt32)),
	})
	if err != nil {
		return nil, mapError(err, "listing terminal sessions")
	}
	return sessions(rows), nil
}

// UnfinishedSessions implements app.Store.
func (r *Repository) UnfinishedSessions(ctx context.Context) ([]ids.ID, error) {
	rows, err := r.read(ctx).UnfinishedTerminalSessions(ctx)
	if err != nil {
		return nil, mapError(err, "reading the unfinished terminal sessions")
	}
	out := make([]ids.ID, 0, len(rows))
	for _, id := range rows {
		out = append(out, idOf(id))
	}
	return out, nil
}

// sessionGrantColumns fills the columns of the grant of row into s (M2-44).
func sessionGrantColumns(s *domain.Session, row sqlcgen.TerminalSession) {
	s.HostSessionUserID, s.OpenedByDeviceID, s.AuthSessionID = idOf(row.HostSessionUserID), idOf(row.OpenedByDeviceID), idOf(row.AuthSessionID)
	s.TokenJTI, s.ClosedBy, s.KeyGeneration = idOf(row.TokenJti), idOf(row.ClosedBy), int64(row.KeyGeneration)
	s.LastInputAt = timeOf(row.LastInputAt)
}
