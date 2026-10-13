// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"errors"
	"net/netip"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/identity/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Compile-time check: Repository is the session store of the identity application layer.
var _ app.SessionStore = (*Repository)(nil)

// InsertSession implements app.SessionStore.
func (r *Repository) InsertSession(ctx context.Context, s app.SessionRecord) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	err = q.InsertSession(ctx, sqlcgen.InsertSessionParams{
		ID: uuid(s.ID), UserID: uuid(s.UserID), DeviceID: uuid(s.DeviceID), ClientComponent: string(s.Component),
		ParentSessionID: optionalUUID(s.ParentID), IdentityID: optionalUUID(s.IdentityID), AuthMethod: string(s.AuthMethod),
		CreatedAt: ts(s.CreatedAt), ExpiresAt: ts(s.ExpiresAt),
	})
	return mapError(err, "inserting a session")
}

// InsertFamily implements app.SessionStore.
func (r *Repository) InsertFamily(ctx context.Context, f app.FamilyRecord) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	err = q.InsertFamily(ctx, sqlcgen.InsertFamilyParams{
		ID: uuid(f.ID), SessionID: uuid(f.SessionID), DpopJkt: f.DPoPJKT, CurrentGeneration: int32(f.CurrentGeneration), //nolint:gosec // G115: generations stay far below 2^31
		CreatedAt: ts(f.CreatedAt),
	})
	return mapError(err, "inserting a refresh family")
}

// InsertRefreshToken implements app.SessionStore.
func (r *Repository) InsertRefreshToken(ctx context.Context, t app.RefreshRecord) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	err = q.InsertRefreshToken(ctx, sqlcgen.InsertRefreshTokenParams{
		ID: uuid(t.ID), FamilyID: uuid(t.FamilyID), Generation: int32(t.Generation), TokenHash: t.Hash, //nolint:gosec // G115: generations stay far below 2^31
		IssuedAt: ts(t.IssuedAt), ExpiresAt: ts(t.ExpiresAt),
	})
	return mapError(err, "inserting a refresh token")
}

// Session implements app.SessionStore.
func (r *Repository) Session(ctx context.Context, id ids.ID) (app.SessionRecord, bool, error) {
	row, err := r.read(ctx).GetSession(ctx, uuid(id))
	if errors.Is(err, pgx.ErrNoRows) {
		return app.SessionRecord{}, false, nil
	}
	if err != nil {
		return app.SessionRecord{}, false, mapError(err, "reading a session")
	}
	return toSession(row), true, nil
}

// LockSessionTree implements app.SessionStore in two statements: the session row first, then its
// children. One statement would read the children with the snapshot it took before it waited for
// the parent lock and miss a child that the holder of the lock inserted (READ COMMITTED re-checks
// only the rows it found; red team M1-34).
func (r *Repository) LockSessionTree(ctx context.Context, id ids.ID) ([]app.SessionRecord, error) {
	q, err := r.write(ctx)
	if err != nil {
		return nil, err
	}
	root, err := q.LockSession(ctx, uuid(id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, mapError(err, "locking a session")
	}
	rows, err := q.LockChildren(ctx, uuid(id))
	if err != nil {
		return nil, mapError(err, "locking child sessions")
	}
	out := make([]app.SessionRecord, 0, 1+len(rows))
	out = append(out, toSession(root))
	for _, row := range rows {
		out = append(out, toSession(row))
	}
	return out, nil
}

// RevokeSessions implements app.SessionStore.
func (r *Repository) RevokeSessions(ctx context.Context, list []ids.ID, reason identity.SessionRevokeReason, at time.Time) ([]ids.ID, error) {
	q, err := r.write(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := q.RevokeSessions(ctx, sqlcgen.RevokeSessionsParams{
		RevokedAt: ts(at), RevokeReason: pgtype.Text{String: string(reason), Valid: true}, Ids: uuids(list),
	})
	if err != nil {
		return nil, mapError(err, "revoking sessions")
	}
	out := make([]ids.ID, 0, len(rows))
	for _, u := range rows {
		out = append(out, idOf(u))
	}
	slices.SortFunc(out, compareIDs)
	return out, nil
}

// ActiveSessionsOfUser implements app.SessionStore.
func (r *Repository) ActiveSessionsOfUser(ctx context.Context, userID, before ids.ID, now time.Time, limit int) ([]app.SessionRecord, error) {
	rows, err := r.read(ctx).ListActiveSessionsOfUser(ctx, sqlcgen.ListActiveSessionsOfUserParams{
		UserID: uuid(userID), Now: ts(now), Before: optionalUUID(before), RowLimit: int32(min(limit, 1<<30)), //nolint:gosec // G115: capped above
	})
	if err != nil {
		return nil, mapError(err, "listing sessions")
	}
	out := make([]app.SessionRecord, 0, len(rows))
	for _, row := range rows {
		out = append(out, toSession(row))
	}
	return out, nil
}

// AuthState implements app.SessionStore.
func (r *Repository) AuthState(ctx context.Context, id ids.ID) (app.AuthState, bool, error) {
	row, err := r.read(ctx).GetAuthState(ctx, uuid(id))
	if errors.Is(err, pgx.ErrNoRows) {
		return app.AuthState{}, false, nil
	}
	if err != nil {
		return app.AuthState{}, false, mapError(err, "reading the session of a request")
	}
	return app.AuthState{
		Session: toSession(row.AuthSession), DeviceState: identity.DeviceState(row.DeviceState),
		DeviceLastSeenAt: timeOf(row.DeviceLastSeenAt), DeviceKeyJKT: row.DpopJkt.String,
	}, true, nil
}

// Touch implements app.SessionStore. It runs on the pool, never in the request's unit of work.
func (r *Repository) Touch(ctx context.Context, sessionID, deviceID ids.ID, ip netip.Addr, at, threshold time.Time) error {
	var addr *netip.Addr
	if ip.IsValid() {
		addr = &ip
	}
	err := sqlcgen.New(r.pool).TouchSession(ctx, sqlcgen.TouchSessionParams{
		SeenAt: ts(at), DeviceID: uuid(deviceID), Threshold: ts(threshold), ClientIp: addr, SessionID: uuid(sessionID),
	})
	return mapError(err, "recording activity")
}

// ActiveFamily implements app.SessionStore.
func (r *Repository) ActiveFamily(ctx context.Context, sessionID ids.ID) (app.FamilyRecord, bool, error) {
	row, err := r.read(ctx).GetActiveFamily(ctx, uuid(sessionID))
	if errors.Is(err, pgx.ErrNoRows) {
		return app.FamilyRecord{}, false, nil
	}
	if err != nil {
		return app.FamilyRecord{}, false, mapError(err, "reading a refresh family")
	}
	return toFamily(row), true, nil
}

// LockFamily implements app.SessionStore.
func (r *Repository) LockFamily(ctx context.Context, id ids.ID) (app.FamilyRecord, bool, error) {
	q, err := r.write(ctx)
	if err != nil {
		return app.FamilyRecord{}, false, err
	}
	row, err := q.LockFamily(ctx, uuid(id))
	if errors.Is(err, pgx.ErrNoRows) {
		return app.FamilyRecord{}, false, nil
	}
	if err != nil {
		return app.FamilyRecord{}, false, mapError(err, "locking a refresh family")
	}
	return toFamily(row), true, nil
}

// RefreshTokenByHash implements app.SessionStore.
func (r *Repository) RefreshTokenByHash(ctx context.Context, hash []byte) (app.RefreshRecord, ids.ID, bool, error) {
	row, err := r.read(ctx).GetRefreshTokenByHash(ctx, hash)
	if errors.Is(err, pgx.ErrNoRows) {
		return app.RefreshRecord{}, ids.ID{}, false, nil
	}
	if err != nil {
		return app.RefreshRecord{}, ids.ID{}, false, mapError(err, "reading a refresh token")
	}
	t := row.RefreshToken
	return app.RefreshRecord{
		ID: idOf(t.ID), FamilyID: idOf(t.FamilyID), Generation: int(t.Generation), Hash: t.TokenHash,
		IssuedAt: timeOf(t.IssuedAt), ExpiresAt: timeOf(t.ExpiresAt), UsedAt: timeOf(t.UsedAt),
	}, idOf(row.SessionID), true, nil
}

// MarkRefreshUsed implements app.SessionStore.
func (r *Repository) MarkRefreshUsed(ctx context.Context, familyID ids.ID, generation int, at time.Time) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	err = q.MarkRefreshTokenUsed(ctx, sqlcgen.MarkRefreshTokenUsedParams{
		UsedAt: ts(at), FamilyID: uuid(familyID), Generation: int32(generation), //nolint:gosec // G115: generations stay far below 2^31
	})
	return mapError(err, "marking a refresh token used")
}

// AdvanceFamily implements app.SessionStore.
func (r *Repository) AdvanceFamily(ctx context.Context, familyID ids.ID, generation int, at time.Time) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	err = q.AdvanceFamily(ctx, sqlcgen.AdvanceFamilyParams{
		CurrentGeneration: int32(generation), RotatedAt: ts(at), ID: uuid(familyID), //nolint:gosec // G115: generations stay far below 2^31
	})
	return mapError(err, "rotating a refresh family")
}

// RevokeFamily implements app.SessionStore.
func (r *Repository) RevokeFamily(ctx context.Context, familyID ids.ID, reason app.FamilyRevokeReason, at time.Time) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	err = q.RevokeFamily(ctx, sqlcgen.RevokeFamilyParams{
		RevokedAt: ts(at), RevokeReason: pgtype.Text{String: string(reason), Valid: true}, ID: uuid(familyID),
	})
	return mapError(err, "revoking a refresh family")
}

// RevokeActiveFamilies implements app.SessionStore.
func (r *Repository) RevokeActiveFamilies(ctx context.Context, list []ids.ID, reason app.FamilyRevokeReason, at time.Time) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	err = q.RevokeActiveFamiliesOfSessions(ctx, sqlcgen.RevokeActiveFamiliesOfSessionsParams{
		RevokedAt: ts(at), RevokeReason: pgtype.Text{String: string(reason), Valid: true}, SessionIds: uuids(list),
	})
	return mapError(err, "revoking refresh families")
}

func toSession(s sqlcgen.AuthSession) app.SessionRecord {
	return app.SessionRecord{
		ID: idOf(s.ID), UserID: idOf(s.UserID), DeviceID: idOf(s.DeviceID), Component: identity.ClientComponent(s.ClientComponent),
		ParentID: idOf(s.ParentSessionID), IdentityID: idOf(s.IdentityID), AuthMethod: identity.AuthMethod(s.AuthMethod),
		CreatedAt: timeOf(s.CreatedAt), ExpiresAt: timeOf(s.ExpiresAt), StepUpAt: timeOf(s.StepUpAt),
		StepUpMethod: s.StepUpMethod.String, LastSeenAt: timeOf(s.LastSeenAt), RevokedAt: timeOf(s.RevokedAt),
		RevokeReason: identity.SessionRevokeReason(s.RevokeReason.String),
	}
}

func toFamily(f sqlcgen.RefreshTokenFamily) app.FamilyRecord {
	return app.FamilyRecord{
		ID: idOf(f.ID), SessionID: idOf(f.SessionID), DPoPJKT: f.DpopJkt, CurrentGeneration: int(f.CurrentGeneration),
		RotatedAt: timeOf(f.RotatedAt), CreatedAt: timeOf(f.CreatedAt), RevokedAt: timeOf(f.RevokedAt),
	}
}

// ts converts a time for a parameter.
func ts(t time.Time) pgtype.Timestamptz { return pgtype.Timestamptz{Time: t, Valid: true} }

// uuids converts ids for an array parameter.
func uuids(list []ids.ID) []pgtype.UUID {
	out := make([]pgtype.UUID, 0, len(list))
	for _, id := range list {
		out = append(out, uuid(id))
	}
	return out
}
