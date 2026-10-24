// SPDX-License-Identifier: Apache-2.0

// Package postgres is the PostgreSQL adapter of the own measurement of the stats module (M6-13):
// the sqlc queries of stats_minutes in queries/, the generated code in sqlcgen/ and Repository,
// which implements app.MinuteStore and app.PartitionStore (docs/spec/03-data-model.md 4.15). Only
// this package runs SQL on the table (03 principle 12).
//
// Example:
//
//	repo := postgres.NewRepository(pool)
//	err := repo.Upsert(ctx, rows) // one statement, its own transaction
package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/stats/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/stats/app"
)

// sqlstateCheckViolation is the SQLSTATE of an insert into a partitioned table without a
// partition for the row ("no partition of relation ... found for row").
const sqlstateCheckViolation = "23514"

// Repository implements the stores of the own measurement over a pool of bothy_app. Every call
// runs on the pool, never in the transaction of a unit of work: a flush is its own short
// transaction (25 3.4). It is safe for concurrent use.
type Repository struct {
	pool *pg.Pool
}

// Compile-time checks: Repository is the outgoing port of the flush and of the retention job.
var (
	_ app.MinuteStore    = (*Repository)(nil)
	_ app.PartitionStore = (*Repository)(nil)
)

// NewRepository returns the repository over pool, a pool of bothy_app.
func NewRepository(pool *pg.Pool) *Repository { return &Repository{pool: pool} }

// Upsert implements app.MinuteStore: one INSERT ... ON CONFLICT DO UPDATE over the rows, which
// commits on its own. A row in a UTC day without a partition gives app.ErrNoPartition.
func (r *Repository) Upsert(ctx context.Context, rows []app.MinuteRow) error {
	if len(rows) == 0 {
		return nil
	}
	p := sqlcgen.UpsertMinutesParams{}
	for _, row := range rows {
		p.BucketStarts = append(p.BucketStarts, pgtype.Timestamptz{Time: row.BucketStart, Valid: true})
		p.Metrics = append(p.Metrics, row.Metric)
		p.ChannelIds = append(p.ChannelIds, uuid(row.ChannelID))
		p.AgentIds = append(p.AgentIds, uuid(row.AgentID))
		p.Dimensions = append(p.Dimensions, row.Dimension)
		p.Currencies = append(p.Currencies, row.Currency)
		p.Counts = append(p.Counts, row.Count)
		p.Sums = append(p.Sums, row.Sum)
		p.Goods = append(p.Goods, row.Good)
	}
	err := sqlcgen.New(r.pool).UpsertMinutes(ctx, p)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == sqlstateCheckViolation && pgErr.ConstraintName == "" {
		// A violated CHECK names its constraint; a missing partition names none.
		return errors.Join(app.ErrNoPartition, err)
	}
	return err
}

// EnsurePartition implements app.MinuteStore and app.PartitionStore through
// stats_minutes_ensure_partition.
func (r *Repository) EnsurePartition(ctx context.Context, day time.Time) error {
	_, err := sqlcgen.New(r.pool).EnsurePartition(ctx, pgtype.Date{Time: day.UTC(), Valid: true})
	return err
}

// DropExpired implements app.PartitionStore through stats_minutes_drop_expired.
func (r *Repository) DropExpired(ctx context.Context, cutoff time.Time, lockTimeout time.Duration) (int, error) {
	n, err := sqlcgen.New(r.pool).DropExpiredPartitions(ctx, sqlcgen.DropExpiredPartitionsParams{
		Cutoff:      pgtype.Timestamptz{Time: cutoff, Valid: true},
		LockTimeout: pgtype.Interval{Microseconds: lockTimeout.Microseconds(), Valid: true},
	})
	return int(n), err
}

// uuid converts id to a nullable UUID: the zero ID is NULL.
func uuid(id ids.ID) pgtype.UUID {
	return pgtype.UUID{Bytes: id, Valid: !id.IsZero()}
}
