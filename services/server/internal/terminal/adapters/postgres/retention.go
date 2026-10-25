// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/OWNER/bothy/services/server/internal/kernel/retention"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/terminal/adapters/postgres/sqlcgen"
)

// Retention is the participant of terminal in the job retention.security (M7-05,
// docs/spec/20-privacy.md 2 and 4.1): terminal_sessions after closed_at, host_session_users and
// hosts after retired_at of the host. A row another row still holds by a foreign key (a session
// its session user, a grant its target host) stays until that row is gone.
type Retention struct {
	pool *pg.Pool
}

// Compile-time check: Retention is a participant of kernel/retention.
var _ retention.Participant = (*Retention)(nil)

// NewRetention returns the participant over pool.
func NewRetention(pool *pg.Pool) *Retention { return &Retention{pool: pool} }

// Class returns retention.ClassSecurity.
func (*Retention) Class() retention.Class { return retention.ClassSecurity }

// Tables returns the tables in the order Purge deletes from them.
func (*Retention) Tables() []string {
	return []string{"terminal_sessions", "host_session_users", "hosts"}
}

// Purge implements retention.Participant: one batch of at most batch rows per table, each in one
// short transaction. Errors: those of the storage.
func (r *Retention) Purge(ctx context.Context, cutoff time.Time, batch int) (int, bool, error) {
	c := pgtype.Timestamptz{Time: cutoff, Valid: true}
	limit := pg.Limit(batch)
	steps := []struct {
		candidates func(ctx context.Context, q *sqlcgen.Queries) ([]pgtype.UUID, error)
		del        func(ctx context.Context, q *sqlcgen.Queries, id pgtype.UUID) (int64, error)
	}{
		{
			func(ctx context.Context, q *sqlcgen.Queries) ([]pgtype.UUID, error) {
				return q.ExpiredTerminalSessions(ctx, sqlcgen.ExpiredTerminalSessionsParams{Cutoff: c, RowLimit: limit})
			},
			func(ctx context.Context, q *sqlcgen.Queries, id pgtype.UUID) (int64, error) {
				return q.DeleteTerminalSession(ctx, sqlcgen.DeleteTerminalSessionParams{ID: id, Cutoff: c})
			},
		},
		{
			func(ctx context.Context, q *sqlcgen.Queries) ([]pgtype.UUID, error) {
				return q.ExpiredHostSessionUsers(ctx, sqlcgen.ExpiredHostSessionUsersParams{Cutoff: c, RowLimit: limit})
			},
			func(ctx context.Context, q *sqlcgen.Queries, id pgtype.UUID) (int64, error) {
				return q.DeleteHostSessionUser(ctx, sqlcgen.DeleteHostSessionUserParams{ID: id, Cutoff: c})
			},
		},
		{
			func(ctx context.Context, q *sqlcgen.Queries) ([]pgtype.UUID, error) {
				return q.ExpiredHosts(ctx, sqlcgen.ExpiredHostsParams{Cutoff: c, RowLimit: limit})
			},
			func(ctx context.Context, q *sqlcgen.Queries, id pgtype.UUID) (int64, error) {
				return q.DeleteHost(ctx, sqlcgen.DeleteHostParams{ID: id, Cutoff: c})
			},
		},
	}
	total, more := 0, false
	for _, st := range steps {
		n, m, err := pg.PurgeStep(ctx, r.pool, batch,
			func(ctx context.Context, q pg.DBTX) ([]pgtype.UUID, error) { return st.candidates(ctx, sqlcgen.New(q)) },
			func(ctx context.Context, q pg.DBTX, id pgtype.UUID) (int64, error) {
				return st.del(ctx, sqlcgen.New(q), id)
			})
		if err != nil {
			return total, false, err
		}
		total, more = total+n, more || m
	}
	return total, more, nil
}
