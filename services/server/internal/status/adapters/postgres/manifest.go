// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/status/app"
)

// ManifestSource implements app.ManifestSource over a pool of migrate.bootstrap_user, the
// superuser bothy-backupd also hands to pgBackRest and pg_dump (16 7.1). The facts span the tables
// of every module, which no module role may read, and pg_stat_archiver. The queries are written
// here and not in queries/: the table list is dynamic, and the other tables belong to their
// modules, so they get no generated code in this module. It is safe for concurrent use.
type ManifestSource struct {
	pool *pg.Pool
}

// Compile-time check: ManifestSource is the manifest source of the backup use cases.
var _ app.ManifestSource = (*ManifestSource)(nil)

// NewManifestSource returns the source over pool, which must connect as a role that can read
// every table of schema public (migrate.bootstrap_user).
func NewManifestSource(pool *pg.Pool) *ManifestSource { return &ManifestSource{pool: pool} }

// rowCountsQuery returns every table of the domain schema with its exact row count: ordinary and
// partitioned tables of schema public that are not partitions, so the partitions of event_log
// are counted through their parent. The count runs inside the database with format('%I'), so no
// statement is built here (ADR 0007); inside the snapshot of ReadSnapshot the nested queries of
// query_to_xml see the same snapshot as every other statement.
const rowCountsQuery = `SELECT c.relname::text,
	(xpath('/row/n/text()', query_to_xml(format('SELECT count(*) AS n FROM %I.%I', n.nspname, c.relname), false, true, '')))[1]::text::bigint
	FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
	WHERE n.nspname = 'public' AND c.relkind IN ('r', 'p') AND NOT c.relispartition ORDER BY c.relname`

// Snapshot implements app.ManifestSource: every fact in one READ ONLY transaction with
// REPEATABLE READ (pg.ReadSnapshot), so the counts, heads and totals belong to one snapshot
// (brief M7-02 design note 3). Row counts are exact count(*), not pg_class.reltuples, because the
// restore test compares them.
//
// It fails when a query fails or the instance row is missing (a database that bothy migrate up
// has not set up).
func (s *ManifestSource) Snapshot(ctx context.Context) (app.ManifestFacts, error) {
	facts := app.ManifestFacts{RowCounts: map[string]int64{}}
	err := pg.ReadSnapshot(ctx, s.pool, func(ctx context.Context, q pg.DBTX) error {
		return readFacts(ctx, q, &facts)
	})
	if err != nil {
		return app.ManifestFacts{}, err
	}
	return facts, nil
}

// tableCount is one row of rowCountsQuery.
type tableCount struct {
	name string
	n    int64
}

// readFacts reads the facts of Snapshot with q into facts.
func readFacts(ctx context.Context, q pg.DBTX, facts *app.ManifestFacts) error {
	rows, err := q.Query(ctx, rowCountsQuery)
	if err != nil {
		return fmt.Errorf("status: counting the tables: %w", err)
	}
	counts, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (tableCount, error) {
		var c tableCount
		err := r.Scan(&c.name, &c.n)
		return c, err
	})
	if err != nil {
		return fmt.Errorf("status: counting the tables: %w", err)
	}
	for _, c := range counts {
		facts.RowCounts[c.name] = c.n
	}
	// The bounds as internal/sync reads them: 0 for an empty log.
	if err := q.QueryRow(ctx, `SELECT COALESCE(max(seq), 0)::bigint, COALESCE(min(seq), 0)::bigint FROM public.event_log`).
		Scan(&facts.HeadSeq, &facts.MinRetainedSeq); err != nil {
		return fmt.Errorf("status: reading the bounds of event_log: %w", err)
	}
	err = q.QueryRow(ctx, `SELECT seq, hash FROM public.audit_events ORDER BY seq DESC LIMIT 1`).
		Scan(&facts.AuditLastSeq, &facts.AuditLastHash)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("status: reading the head of audit_events: %w", err)
	}
	if err := q.QueryRow(ctx, `SELECT instance_id::text, sync_epoch::text, schema_version FROM public.instance WHERE id = 1`).
		Scan(&facts.InstanceID, &facts.SyncEpoch, &facts.SchemaVersion); err != nil {
		return fmt.Errorf("status: reading the instance row: %w", err)
	}
	if err := q.QueryRow(ctx, `SELECT count(*), COALESCE(sum(size_bytes), 0)::bigint FROM public.files WHERE deleted_at IS NULL`).
		Scan(&facts.FileCount, &facts.FileBytes); err != nil {
		return fmt.Errorf("status: reading the file totals: %w", err)
	}
	return nil
}

// LastArchived implements app.ManifestSource with pg_stat_archiver.last_archived_time:
// pgbackrest info reports no archive time. It returns false when no WAL segment
// was archived since the statistics were reset.
func (s *ManifestSource) LastArchived(ctx context.Context) (time.Time, bool, error) {
	var at pgtype.Timestamptz
	if err := s.pool.QueryRow(ctx, `SELECT last_archived_time FROM pg_stat_archiver`).Scan(&at); err != nil {
		return time.Time{}, false, fmt.Errorf("status: reading pg_stat_archiver: %w", err)
	}
	if !at.Valid {
		return time.Time{}, false, nil
	}
	return at.Time.UTC(), true, nil
}
