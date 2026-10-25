// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/OWNER/bothy/services/server/internal/kernel/retention"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/status/adapters/postgres/sqlcgen"
)

// Retention is the participant of status in the job retention.ops (M7-09, docs/spec/20-privacy.md
// 4.1): incidents resolved before the cutoff (retention.ops before now) with their updates and
// component links, and backup runs started before it, except the latest succeeded run of each
// kind, which bothy migrate up and the restore test read (D-M7-09-6). The components are the live
// configuration of the page and carry no end time; the owner, the job status.probe and the agents
// remove them.
type Retention struct {
	pool *pg.Pool
}

// Compile-time check: Retention is a participant of kernel/retention.
var _ retention.Participant = (*Retention)(nil)

// NewRetention returns the participant over pool.
func NewRetention(pool *pg.Pool) *Retention { return &Retention{pool: pool} }

// Class returns retention.ClassOps.
func (*Retention) Class() retention.Class { return retention.ClassOps }

// Tables returns the tables whose rows Purge removes: the incidents with the rows that go with
// them (ON DELETE CASCADE), then backup_runs.
func (*Retention) Tables() []string {
	return []string{"status_incidents", "status_incident_updates", "status_incident_components", "backup_runs"}
}

// Purge implements retention.Participant: one batch of at most batch rows per table, each in one
// short transaction. Errors: those of the storage.
func (r *Retention) Purge(ctx context.Context, cutoff time.Time, batch int) (int, bool, error) {
	c := pgtype.Timestamptz{Time: cutoff, Valid: true}
	limit := pg.Limit(batch)
	n1, m1, err := pg.PurgeStep(ctx, r.pool, batch,
		func(ctx context.Context, q pg.DBTX) ([]pgtype.UUID, error) {
			return sqlcgen.New(q).ExpiredIncidents(ctx, sqlcgen.ExpiredIncidentsParams{Cutoff: c, RowLimit: limit})
		},
		func(ctx context.Context, q pg.DBTX, id pgtype.UUID) (int64, error) {
			return sqlcgen.New(q).DeleteIncident(ctx, sqlcgen.DeleteIncidentParams{ID: id, Cutoff: c})
		})
	if err != nil {
		return n1, false, err
	}
	n2, m2, err := pg.PurgeStep(ctx, r.pool, batch,
		func(ctx context.Context, q pg.DBTX) ([]pgtype.UUID, error) {
			return sqlcgen.New(q).ExpiredBackupRuns(ctx, sqlcgen.ExpiredBackupRunsParams{Cutoff: c, RowLimit: limit})
		},
		func(ctx context.Context, q pg.DBTX, id pgtype.UUID) (int64, error) {
			return sqlcgen.New(q).DeleteBackupRun(ctx, sqlcgen.DeleteBackupRunParams{ID: id, Cutoff: c})
		})
	return n1 + n2, m1 || m2, err
}
