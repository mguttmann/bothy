// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/OWNER/bothy/services/server/internal/instance/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/kernel/retention"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
)

// Retention returns the participant of instance in the job retention.security (M7-05,
// docs/spec/20-privacy.md 2 and 4.1) over the pool of r.
func (r *Repository) Retention() retention.Participant { return signingKeyRetention{pool: r.pool} }

// signingKeyRetention deletes retired signing keys whose not_after (the end of signing; the
// retirement follows one verification window later) lies before the cutoff. Keys of purpose
// audit_checkpoint stay, because audit.verify checks every checkpoint of the sink with the key of
// its kid in any state; a key a webhook subscription or a stored checkpoint still names stays
// through its foreign key.
type signingKeyRetention struct {
	pool *pg.Pool
}

// Class returns retention.ClassSecurity.
func (signingKeyRetention) Class() retention.Class { return retention.ClassSecurity }

// Tables returns server_signing_keys.
func (signingKeyRetention) Tables() []string { return []string{"server_signing_keys"} }

// Purge deletes at most batch expired keys in one short transaction. Errors: those of the storage.
func (s signingKeyRetention) Purge(ctx context.Context, cutoff time.Time, batch int) (int, bool, error) {
	before := pgtype.Timestamptz{Time: cutoff, Valid: true}
	return pg.PurgeStep(ctx, s.pool, batch,
		func(ctx context.Context, q pg.DBTX) ([]string, error) {
			return sqlcgen.New(q).ExpiredSigningKeys(ctx, sqlcgen.ExpiredSigningKeysParams{Cutoff: before, RowLimit: pg.Limit(batch)})
		},
		func(ctx context.Context, q pg.DBTX, kid string) (int64, error) {
			return sqlcgen.New(q).DeleteSigningKey(ctx, sqlcgen.DeleteSigningKeyParams{Kid: kid, Cutoff: before})
		})
}
