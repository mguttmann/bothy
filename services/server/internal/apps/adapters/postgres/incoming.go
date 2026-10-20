// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/OWNER/bothy/services/server/internal/apps/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/apps/app"
	"github.com/OWNER/bothy/services/server/internal/apps/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Compile-time check: Repository is the store of the incoming webhooks.
var _ app.IncomingStore = (*Repository)(nil)

// ListActiveIncoming implements app.IncomingStore.
func (r *Repository) ListActiveIncoming(ctx context.Context, channelID ids.ID) ([]app.IncomingRecord, error) {
	rows, err := r.read(ctx).ListActiveIncomingByChannel(ctx, uuid(channelID))
	if err != nil {
		return nil, mapError(err, "listing incoming webhooks")
	}
	out := make([]app.IncomingRecord, 0, len(rows))
	for _, row := range rows {
		out = append(out, toIncoming(row))
	}
	return out, nil
}

// InsertIncoming implements app.IncomingStore.
func (r *Repository) InsertIncoming(ctx context.Context, w app.IncomingRecord) (app.IncomingRecord, error) {
	q, err := r.write(ctx)
	if err != nil {
		return app.IncomingRecord{}, err
	}
	row, err := q.InsertIncoming(ctx, sqlcgen.InsertIncomingParams{
		ID: uuid(w.ID), AppID: uuid(w.AppID), ChannelID: uuid(w.ChannelID), ThreadID: optionalUUID(w.ThreadID), Name: w.Name,
		TokenHash: w.TokenHash, CreatedBy: uuid(w.CreatedBy),
	})
	if err != nil {
		return app.IncomingRecord{}, mapError(err, "inserting an incoming webhook")
	}
	return toIncoming(row), nil
}

// Incoming implements app.IncomingStore.
func (r *Repository) Incoming(ctx context.Context, id ids.ID) (app.IncomingRecord, bool, error) {
	row, err := r.read(ctx).GetIncoming(ctx, uuid(id))
	return one(row, err, toIncoming, "reading an incoming webhook")
}

// IncomingForAuth implements app.IncomingStore.
func (r *Repository) IncomingForAuth(ctx context.Context, id ids.ID) (app.IncomingAuth, bool, error) {
	row, err := r.read(ctx).GetIncomingForAuth(ctx, uuid(id))
	return one(row, err, func(x sqlcgen.GetIncomingForAuthRow) app.IncomingAuth {
		return app.IncomingAuth{
			Webhook: toIncoming(x.IncomingWebhook), InstallationID: ids.ID(x.InstallationID.Bytes),
			InstallationState: domain.State(x.InstallationState), GrantedScopes: nonNil(x.GrantedScopes),
		}
	}, "reading an incoming webhook for its token")
}

// LockIncomingForPost implements app.IncomingStore.
func (r *Repository) LockIncomingForPost(ctx context.Context, id ids.ID) (app.IncomingRecord, bool, error) {
	q, err := r.write(ctx)
	if err != nil {
		return app.IncomingRecord{}, false, err
	}
	row, err := q.LockIncomingForPost(ctx, uuid(id))
	return one(row, err, toIncoming, "locking an incoming webhook")
}

// RevokeIncoming implements app.IncomingStore.
func (r *Repository) RevokeIncoming(ctx context.Context, id ids.ID, at time.Time) (bool, error) {
	q, err := r.write(ctx)
	if err != nil {
		return false, err
	}
	n, err := q.RevokeIncoming(ctx, sqlcgen.RevokeIncomingParams{ID: uuid(id), RevokedAt: timestamp(at)})
	if err != nil {
		return false, mapError(err, "revoking an incoming webhook")
	}
	return n > 0, nil
}

// RevokeIncomingByApp implements app.IncomingStore.
func (r *Repository) RevokeIncomingByApp(ctx context.Context, appID ids.ID, at time.Time) (int, error) {
	q, err := r.write(ctx)
	if err != nil {
		return 0, err
	}
	n, err := q.RevokeIncomingByApp(ctx, sqlcgen.RevokeIncomingByAppParams{AppID: uuid(appID), RevokedAt: timestamp(at)})
	if err != nil {
		return 0, mapError(err, "revoking the incoming webhooks of an app")
	}
	return int(n), nil
}

// RevokeIncomingByChannel implements app.IncomingStore.
func (r *Repository) RevokeIncomingByChannel(ctx context.Context, channelID ids.ID, at time.Time) (int, error) {
	q, err := r.write(ctx)
	if err != nil {
		return 0, err
	}
	n, err := q.RevokeIncomingByChannel(ctx, sqlcgen.RevokeIncomingByChannelParams{ChannelID: uuid(channelID), RevokedAt: timestamp(at)})
	if err != nil {
		return 0, mapError(err, "revoking the incoming webhooks of a channel")
	}
	return int(n), nil
}

// TouchIncomingLastUsed implements app.IncomingStore. It runs on the pool, outside the unit of
// work of the post, so concurrent posts never upgrade their share lock (design note 6 f).
func (r *Repository) TouchIncomingLastUsed(ctx context.Context, id ids.ID, at, olderThan time.Time) error {
	_, err := r.read(ctx).TouchIncomingLastUsed(ctx, sqlcgen.TouchIncomingLastUsedParams{
		ID: uuid(id), UsedAt: timestamp(at), OlderThan: timestamp(olderThan),
	})
	if err != nil {
		return mapError(err, "touching an incoming webhook")
	}
	return nil
}

// toIncoming converts a row; NULL becomes the zero value.
func toIncoming(w sqlcgen.IncomingWebhook) app.IncomingRecord {
	out := app.IncomingRecord{
		ID: ids.ID(w.ID.Bytes), AppID: ids.ID(w.AppID.Bytes), ChannelID: ids.ID(w.ChannelID.Bytes), Name: w.Name, TokenHash: w.TokenHash,
		CreatedBy: ids.ID(w.CreatedBy.Bytes), CreatedAt: w.CreatedAt.Time, LastUsedAt: timeOf(w.LastUsedAt), RevokedAt: timeOf(w.RevokedAt),
	}
	if w.ThreadID.Valid {
		out.ThreadID = ids.ID(w.ThreadID.Bytes)
	}
	return out
}

// optionalUUID is NULL for the zero id.
func optionalUUID(id ids.ID) pgtype.UUID { return pgtype.UUID{Bytes: id, Valid: !id.IsZero()} }
