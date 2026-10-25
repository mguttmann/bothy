// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/OWNER/bothy/services/server/internal/identity/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/kernel/retention"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
)

// Retention is the participant of identity in the job retention.security (M7-05,
// docs/spec/20-privacy.md 4.1): it deletes the rows of the security tables of identity whose end
// time lies before the cutoff. End time per table (README "Retention"): refresh_tokens
// expires_at; refresh_token_families revoked_at; auth_sessions and api_tokens the earlier of
// expires_at and revoked_at; device_keys, devices and passkey_credentials revoked_at;
// enrollment_codes the earlier of expires_at and used_at. local_credentials has no end time; it
// goes with the account (M7-06). A row that a row of another module still references (a terminal
// session of terminal holds its login session and device) stays until that row is gone.
type Retention struct {
	pool *pg.Pool
}

// Compile-time check: Retention is a participant of kernel/retention.
var _ retention.Participant = (*Retention)(nil)

// NewRetention returns the participant over pool.
func NewRetention(pool *pg.Pool) *Retention { return &Retention{pool: pool} }

// Class returns retention.ClassSecurity.
func (*Retention) Class() retention.Class { return retention.ClassSecurity }

// Tables returns the tables Purge deletes from, in the order it deletes them.
func (*Retention) Tables() []string {
	return []string{
		"refresh_tokens", "refresh_token_families", "auth_sessions", "device_keys", "devices",
		"passkey_credentials", "api_tokens", "enrollment_codes",
	}
}

// identityStep is the candidate query and the delete of one table.
type identityStep struct {
	candidates func(q *sqlcgen.Queries, ctx context.Context, cutoff pgtype.Timestamptz, limit int32) ([]pgtype.UUID, error)
	del        func(q *sqlcgen.Queries, ctx context.Context, id pgtype.UUID, cutoff pgtype.Timestamptz) (int64, error)
}

// identitySteps are the tables in the order of Tables: tokens before their family, sessions
// before devices (auth_sessions holds its device with ON DELETE RESTRICT).
var identitySteps = []identityStep{
	{
		candidates: func(q *sqlcgen.Queries, ctx context.Context, c pgtype.Timestamptz, l int32) ([]pgtype.UUID, error) {
			return q.ExpiredRefreshTokens(ctx, sqlcgen.ExpiredRefreshTokensParams{Cutoff: c, RowLimit: l})
		},
		del: func(q *sqlcgen.Queries, ctx context.Context, id pgtype.UUID, c pgtype.Timestamptz) (int64, error) {
			return q.DeleteRefreshToken(ctx, sqlcgen.DeleteRefreshTokenParams{ID: id, Cutoff: c})
		},
	},
	{
		candidates: func(q *sqlcgen.Queries, ctx context.Context, c pgtype.Timestamptz, l int32) ([]pgtype.UUID, error) {
			return q.ExpiredRefreshFamilies(ctx, sqlcgen.ExpiredRefreshFamiliesParams{Cutoff: c, RowLimit: l})
		},
		del: func(q *sqlcgen.Queries, ctx context.Context, id pgtype.UUID, c pgtype.Timestamptz) (int64, error) {
			return q.DeleteRefreshFamily(ctx, sqlcgen.DeleteRefreshFamilyParams{ID: id, Cutoff: c})
		},
	},
	{
		candidates: func(q *sqlcgen.Queries, ctx context.Context, c pgtype.Timestamptz, l int32) ([]pgtype.UUID, error) {
			return q.ExpiredAuthSessions(ctx, sqlcgen.ExpiredAuthSessionsParams{Cutoff: c, RowLimit: l})
		},
		del: func(q *sqlcgen.Queries, ctx context.Context, id pgtype.UUID, c pgtype.Timestamptz) (int64, error) {
			return q.DeleteAuthSession(ctx, sqlcgen.DeleteAuthSessionParams{ID: id, Cutoff: c})
		},
	},
	{
		candidates: func(q *sqlcgen.Queries, ctx context.Context, c pgtype.Timestamptz, l int32) ([]pgtype.UUID, error) {
			return q.ExpiredDeviceKeys(ctx, sqlcgen.ExpiredDeviceKeysParams{Cutoff: c, RowLimit: l})
		},
		del: func(q *sqlcgen.Queries, ctx context.Context, id pgtype.UUID, c pgtype.Timestamptz) (int64, error) {
			return q.DeleteDeviceKey(ctx, sqlcgen.DeleteDeviceKeyParams{ID: id, Cutoff: c})
		},
	},
	{
		candidates: func(q *sqlcgen.Queries, ctx context.Context, c pgtype.Timestamptz, l int32) ([]pgtype.UUID, error) {
			return q.ExpiredDevices(ctx, sqlcgen.ExpiredDevicesParams{Cutoff: c, RowLimit: l})
		},
		del: func(q *sqlcgen.Queries, ctx context.Context, id pgtype.UUID, c pgtype.Timestamptz) (int64, error) {
			return q.DeleteDevice(ctx, sqlcgen.DeleteDeviceParams{ID: id, Cutoff: c})
		},
	},
	{
		candidates: func(q *sqlcgen.Queries, ctx context.Context, c pgtype.Timestamptz, l int32) ([]pgtype.UUID, error) {
			return q.ExpiredPasskeys(ctx, sqlcgen.ExpiredPasskeysParams{Cutoff: c, RowLimit: l})
		},
		del: func(q *sqlcgen.Queries, ctx context.Context, id pgtype.UUID, c pgtype.Timestamptz) (int64, error) {
			return q.DeletePasskey(ctx, sqlcgen.DeletePasskeyParams{ID: id, Cutoff: c})
		},
	},
	{
		candidates: func(q *sqlcgen.Queries, ctx context.Context, c pgtype.Timestamptz, l int32) ([]pgtype.UUID, error) {
			return q.ExpiredAPITokens(ctx, sqlcgen.ExpiredAPITokensParams{Cutoff: c, RowLimit: l})
		},
		del: func(q *sqlcgen.Queries, ctx context.Context, id pgtype.UUID, c pgtype.Timestamptz) (int64, error) {
			return q.DeleteAPIToken(ctx, sqlcgen.DeleteAPITokenParams{ID: id, Cutoff: c})
		},
	},
	{
		candidates: func(q *sqlcgen.Queries, ctx context.Context, c pgtype.Timestamptz, l int32) ([]pgtype.UUID, error) {
			return q.ExpiredEnrollmentCodes(ctx, sqlcgen.ExpiredEnrollmentCodesParams{Cutoff: c, RowLimit: l})
		},
		del: func(q *sqlcgen.Queries, ctx context.Context, id pgtype.UUID, c pgtype.Timestamptz) (int64, error) {
			return q.DeleteEnrollmentCode(ctx, sqlcgen.DeleteEnrollmentCodeParams{ID: id, Cutoff: c})
		},
	},
}

// Purge implements retention.Participant: one batch of at most batch rows per table, each table
// in its own short transaction. Errors: those of the storage.
func (r *Retention) Purge(ctx context.Context, cutoff time.Time, batch int) (int, bool, error) {
	c := pgtype.Timestamptz{Time: cutoff, Valid: true}
	limit := pg.Limit(batch)
	total, more := 0, false
	for _, st := range identitySteps {
		n, m, err := pg.PurgeStep(ctx, r.pool, batch,
			func(ctx context.Context, q pg.DBTX) ([]pgtype.UUID, error) {
				return st.candidates(sqlcgen.New(q), ctx, c, limit)
			},
			func(ctx context.Context, q pg.DBTX, id pgtype.UUID) (int64, error) {
				return st.del(sqlcgen.New(q), ctx, id, c)
			})
		if err != nil {
			return total, false, mapError(err, "deleting expired security rows")
		}
		total, more = total+n, more || m
	}
	return total, more, nil
}
