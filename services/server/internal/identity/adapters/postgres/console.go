// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"errors"
	"net/netip"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/identity/app"
	"github.com/OWNER/bothy/services/server/internal/identity/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Compile-time check: Repository is the console store of the identity application layer.
var _ app.ConsoleStore = (*Repository)(nil)

// InsertConsoleSession implements app.ConsoleStore.
func (r *Repository) InsertConsoleSession(ctx context.Context, s app.SessionRecord, cookieHash []byte) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	err = q.InsertConsoleSession(ctx, sqlcgen.InsertConsoleSessionParams{
		ID: uuid(s.ID), UserID: uuid(s.UserID), DeviceID: uuid(s.DeviceID), IdentityID: optionalUUID(s.IdentityID),
		AuthMethod: string(s.AuthMethod), CookieHash: cookieHash, CreatedAt: ts(s.CreatedAt), ExpiresAt: ts(s.ExpiresAt),
		LastIp: optionalAddr(s.LastIP),
	})
	return mapError(err, "inserting a console session")
}

// ConsoleAuthState implements app.ConsoleStore.
func (r *Repository) ConsoleAuthState(ctx context.Context, cookieHash []byte) (app.ConsoleAuthState, bool, error) {
	row, err := r.read(ctx).GetConsoleAuthState(ctx, cookieHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return app.ConsoleAuthState{}, false, nil
	}
	if err != nil {
		return app.ConsoleAuthState{}, false, mapError(err, "reading the console session of a request")
	}
	s := toSession(row.AuthSession)
	s.StepUpUserVerified = row.StepUpUserVerified
	return app.ConsoleAuthState{
		Session: s, DeviceState: identity.DeviceState(row.DeviceState),
		UserActive: domain.Status(row.UserStatus) == domain.StatusActive,
	}, true, nil
}

// SetConsoleCookieHash implements app.ConsoleStore.
func (r *Repository) SetConsoleCookieHash(ctx context.Context, sessionID ids.ID, hash []byte) (bool, error) {
	q, err := r.write(ctx)
	if err != nil {
		return false, err
	}
	n, err := q.SetConsoleCookieHash(ctx, sqlcgen.SetConsoleCookieHashParams{CookieHash: hash, ID: uuid(sessionID)})
	return n == 1, mapError(err, "rotating a console cookie")
}

// StoreConsoleView implements app.ConsoleStore.
func (r *Repository) StoreConsoleView(ctx context.Context, sessionID, userID ids.ID, at time.Time) (bool, error) {
	q, err := r.write(ctx)
	if err != nil {
		return false, err
	}
	n, err := q.StoreConsoleView(ctx, sqlcgen.StoreConsoleViewParams{ViewAsUserID: uuid(userID), StartedAt: ts(at), ID: uuid(sessionID)})
	return n == 1, mapError(err, "storing a console view")
}

// ClearConsoleView implements app.ConsoleStore.
func (r *Repository) ClearConsoleView(ctx context.Context, sessionID ids.ID, startedAt time.Time) (app.ConsoleView, bool, error) {
	q, err := r.write(ctx)
	if err != nil {
		return app.ConsoleView{}, false, err
	}
	row, err := q.ClearConsoleView(ctx, sqlcgen.ClearConsoleViewParams{
		StartedAt: pgtype.Timestamptz{Time: startedAt, Valid: !startedAt.IsZero()}, SessionID: uuid(sessionID),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return app.ConsoleView{}, false, nil
	}
	if err != nil {
		return app.ConsoleView{}, false, mapError(err, "ending a console view")
	}
	return app.ConsoleView{UserID: idOf(row.ViewAsUserID), StartedAt: timeOf(row.ViewAsStartedAt)}, true, nil
}

// TouchConsole implements app.ConsoleStore. It runs on the pool, never in the request's unit of
// work.
func (r *Repository) TouchConsole(ctx context.Context, sessionID, deviceID ids.ID, ip netip.Addr, at, threshold time.Time) error {
	err := sqlcgen.New(r.pool).TouchConsoleSession(ctx, sqlcgen.TouchConsoleSessionParams{
		SeenAt: ts(at), DeviceID: uuid(deviceID), Threshold: ts(threshold), ClientIp: optionalAddr(ip), SessionID: uuid(sessionID),
	})
	return mapError(err, "recording console activity")
}

// DueConsoleSessions implements app.ConsoleStore.
func (r *Repository) DueConsoleSessions(ctx context.Context, now, idleCutoff time.Time, after ids.ID, limit int) ([]ids.ID, error) {
	rows, err := r.read(ctx).ListDueConsoleSessions(ctx, sqlcgen.ListDueConsoleSessionsParams{
		Now: ts(now), IdleCutoff: ts(idleCutoff), After: optionalUUID(after), RowLimit: int32(min(limit, 1<<30)), //nolint:gosec // G115: capped above
	})
	if err != nil {
		return nil, mapError(err, "listing due console sessions")
	}
	out := make([]ids.ID, 0, len(rows))
	for _, u := range rows {
		out = append(out, idOf(u))
	}
	return out, nil
}
