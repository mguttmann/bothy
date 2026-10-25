// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/OWNER/bothy/services/server/internal/kernel/retention"
	"github.com/OWNER/bothy/services/server/internal/notify/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
)

// Compile-time check: Registrations is the participant of notify in the job retention.security
// (M7-05, docs/spec/20-privacy.md 2 and 4.1).
var _ retention.Participant = (*Registrations)(nil)

// Class returns retention.ClassSecurity: push registrations are security data.
func (*Registrations) Class() retention.Class { return retention.ClassSecurity }

// Tables returns push_registrations.
func (*Registrations) Tables() []string { return []string{"push_registrations"} }

// Purge implements retention.Participant: it deletes at most batch push registrations whose
// disabled_at lies before cutoff (retention.security_after_end before now) in one short
// transaction; an enabled registration has no end time, and one of a deleted device or session
// goes with it by its foreign key. Errors: those of the storage.
func (a *Registrations) Purge(ctx context.Context, cutoff time.Time, batch int) (int, bool, error) {
	at := pgtype.Timestamptz{Time: cutoff, Valid: true}
	expired := func(ctx context.Context, q pg.DBTX) ([]pgtype.UUID, error) {
		return sqlcgen.New(q).ExpiredPushRegistrations(ctx, sqlcgen.ExpiredPushRegistrationsParams{Cutoff: at, RowLimit: pg.Limit(batch)})
	}
	remove := func(ctx context.Context, q pg.DBTX, id pgtype.UUID) (int64, error) {
		return sqlcgen.New(q).DeletePushRegistration(ctx, sqlcgen.DeletePushRegistrationParams{ID: id, Cutoff: at})
	}
	return pg.PurgeStep(ctx, a.pool, batch, expired, remove)
}
