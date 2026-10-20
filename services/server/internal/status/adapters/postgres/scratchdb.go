// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"iter"
	"math"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/OWNER/bothy/services/server/internal/audit"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/status/app"
)

// auditBatch is the number of audit rows one query of the chain iterators reads.
const auditBatch = 1000

// ScratchDB implements app.ScratchDB over a pool of the restored copy of the restore test
// (brief M7-03): a PostgreSQL in backup-scratch reached over its Unix socket as
// migrate.bootstrap_user. Every query only reads, and the pool never reaches the production
// database. The queries span the tables of several modules in a copy no module serves, so they
// are written here, like those of ManifestSource, and not as generated code of their modules.
// It is safe for concurrent use.
type ScratchDB struct {
	pool *pg.Pool
}

// Compile-time check: ScratchDB is the copy of the restore test.
var _ app.ScratchDB = (*ScratchDB)(nil)

// NewScratchDB returns the copy over pool, which must connect to the scratch PostgreSQL as a role
// that can read every table of schema public.
func NewScratchDB(pool *pg.Pool) *ScratchDB { return &ScratchDB{pool: pool} }

// Facts implements app.ScratchDB with the snapshot of ManifestSource.Snapshot.
func (s *ScratchDB) Facts(ctx context.Context) (app.ManifestFacts, error) {
	return NewManifestSource(s.pool).Snapshot(ctx)
}

// partitionsQuery returns count, smallest and largest seq of every leaf of the partition tree of
// event_log that holds rows, through query_to_xml as rowCountsQuery (ADR 0007: no statement built
// in Go). A table that is not partitioned is its own only leaf.
const partitionsQuery = `SELECT name, n, lo, hi FROM (
	SELECT c.relname::text AS name,
		(xpath('/row/n/text()', x))[1]::text::bigint AS n,
		COALESCE((xpath('/row/lo/text()', x))[1]::text::bigint, 0) AS lo,
		COALESCE((xpath('/row/hi/text()', x))[1]::text::bigint, 0) AS hi
	FROM pg_partition_tree('public.event_log') t
	JOIN pg_class c ON c.oid = t.relid
	JOIN pg_namespace ns ON ns.oid = c.relnamespace,
	LATERAL query_to_xml(format('SELECT count(*) AS n, min(seq) AS lo, max(seq) AS hi FROM %I.%I', ns.nspname, c.relname), false, true, '') AS x
	WHERE t.isleaf) s
	WHERE n > 0 ORDER BY lo`

// EventLog implements app.ScratchDB in one read-only snapshot: the partitions of event_log with
// rows, sequence_counters and the head of audit_events.
func (s *ScratchDB) EventLog(ctx context.Context) (app.EventLogFacts, error) {
	out := app.EventLogFacts{Counters: map[string]int64{}}
	err := pg.ReadSnapshot(ctx, s.pool, func(ctx context.Context, q pg.DBTX) error {
		rows, err := q.Query(ctx, partitionsQuery)
		if err != nil {
			return fmt.Errorf("status: reading the partitions of event_log: %w", err)
		}
		out.Partitions, err = pgx.CollectRows(rows, func(r pgx.CollectableRow) (app.EventPartition, error) {
			var p app.EventPartition
			err := r.Scan(&p.Name, &p.Count, &p.MinSeq, &p.MaxSeq)
			return p, err
		})
		if err != nil {
			return fmt.Errorf("status: reading the partitions of event_log: %w", err)
		}
		if out.Counters, err = readCounters(ctx, q); err != nil {
			return err
		}
		if err := q.QueryRow(ctx, `SELECT COALESCE(max(seq), 0)::bigint FROM public.audit_events`).Scan(&out.AuditHead); err != nil {
			return fmt.Errorf("status: reading the head of audit_events: %w", err)
		}
		return nil
	})
	if err != nil {
		return app.EventLogFacts{}, err
	}
	return out, nil
}

// counter is one row of sequence_counters.
type counter struct {
	name  string
	value int64
}

// readCounters reads sequence_counters by name.
func readCounters(ctx context.Context, q pg.DBTX) (map[string]int64, error) {
	rows, err := q.Query(ctx, `SELECT name, value FROM public.sequence_counters`)
	if err != nil {
		return nil, fmt.Errorf("status: reading sequence_counters: %w", err)
	}
	all, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (counter, error) {
		var c counter
		err := r.Scan(&c.name, &c.value)
		return c, err
	})
	if err != nil {
		return nil, fmt.Errorf("status: reading sequence_counters: %w", err)
	}
	out := make(map[string]int64, len(all))
	for _, c := range all {
		out[c.name] = c.value
	}
	return out, nil
}

// AuditChain implements app.ScratchDB: instance_id, the public keys of purpose audit_checkpoint
// and iterators that read audit_events and audit_checkpoints in seq order in batches of
// auditBatch rows, so a long log is never held whole.
func (s *ScratchDB) AuditChain(ctx context.Context) (audit.RowsInput, error) {
	var inst pgtype.UUID
	if err := s.pool.QueryRow(ctx, `SELECT instance_id FROM public.instance WHERE id = 1`).Scan(&inst); err != nil {
		return audit.RowsInput{}, fmt.Errorf("status: reading the instance of the copy: %w", err)
	}
	rows, err := s.pool.Query(ctx, `SELECT kid, purpose, alg, public_jwk FROM public.server_signing_keys
		WHERE purpose = $1 ORDER BY kid`, audit.PurposeAuditCheckpoint)
	if err != nil {
		return audit.RowsInput{}, fmt.Errorf("status: reading the signing keys of the copy: %w", err)
	}
	keys, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (audit.VerificationKey, error) {
		var k audit.VerificationKey
		err := r.Scan(&k.KID, &k.Purpose, &k.Alg, &k.PublicJWK)
		return k, err
	})
	if err != nil {
		return audit.RowsInput{}, fmt.Errorf("status: reading the signing keys of the copy: %w", err)
	}
	return audit.RowsInput{
		InstanceID: ids.ID(inst.Bytes), Keys: keys,
		Events: pages(ctx, s.pool, `SELECT seq, id, occurred_at, actor_principal_id, actor_device_id, actor_session_id,
			action, decision, deny_reason, resource_kind, resource_id, details, correlation_id, prev_hash, hash
			FROM public.audit_events WHERE seq > $1 ORDER BY seq LIMIT $2`, scanEvent, func(e audit.Event) int64 { return e.Seq }),
		Checkpoints: pages(ctx, s.pool, `SELECT seq, hash, signature FROM public.audit_checkpoints
			WHERE seq > $1 ORDER BY seq LIMIT $2`, scanCheckpoint, func(c audit.CheckpointRow) int64 { return c.Seq }),
	}, nil
}

// pages yields the rows of query, a keyset page over seq with the parameters (after, limit), in
// pages of auditBatch rows; the first error ends the sequence.
func pages[T any](ctx context.Context, pool *pg.Pool, query string, scan func(pgx.CollectableRow) (T, error), seq func(T) int64) iter.Seq2[T, error] {
	return func(yield func(T, error) bool) {
		var zero T
		after := int64(math.MinInt64)
		for {
			page, err := readPage(ctx, pool, query, after, scan)
			if err != nil {
				yield(zero, err)
				return
			}
			for _, v := range page {
				if !yield(v, nil) {
					return
				}
			}
			if len(page) < auditBatch {
				return
			}
			after = seq(page[len(page)-1])
		}
	}
}

// readPage reads one page of query after the seq after.
func readPage[T any](ctx context.Context, pool *pg.Pool, query string, after int64, scan func(pgx.CollectableRow) (T, error)) ([]T, error) {
	rows, err := pool.Query(ctx, query, after, auditBatch)
	if err != nil {
		return nil, fmt.Errorf("status: reading the audit log of the copy: %w", err)
	}
	page, err := pgx.CollectRows(rows, scan)
	if err != nil {
		return nil, fmt.Errorf("status: reading the audit log of the copy: %w", err)
	}
	return page, nil
}

// scanEvent reads one row of audit_events as the module audit stores it: details decoded with
// UseNumber, so integers stay exact; details that are no object and a hash of another size read
// as nil and zero, which the walk reports, instead of failing the read.
func scanEvent(r pgx.CollectableRow) (audit.Event, error) {
	var (
		e                                         audit.Event
		id, principal, device, session, res, corr pgtype.UUID
		at                                        pgtype.Timestamptz
		deny, kind                                pgtype.Text
		details, prev, hash                       []byte
	)
	if err := r.Scan(&e.Seq, &id, &at, &principal, &device, &session, &e.Action, &e.Decision, &deny, &kind,
		&res, &details, &corr, &prev, &hash); err != nil {
		return audit.Event{}, err
	}
	e.ID, e.OccurredAt = idOf(id), at.Time.UTC()
	e.ActorPrincipalID, e.ActorDeviceID, e.ActorSessionID = idOf(principal), idOf(device), idOf(session)
	e.DenyReason, e.ResourceKind, e.ResourceID, e.CorrelationID = deny.String, kind.String, idOf(res), idOf(corr)
	dec := json.NewDecoder(bytes.NewReader(details))
	dec.UseNumber()
	if err := dec.Decode(&e.Details); err != nil {
		e.Details = nil
	}
	if len(prev) == audit.HashSize {
		copy(e.PrevHash[:], prev)
	}
	if len(hash) == audit.HashSize {
		copy(e.Hash[:], hash)
	}
	return e, nil
}

// scanCheckpoint reads one row of audit_checkpoints.
func scanCheckpoint(r pgx.CollectableRow) (audit.CheckpointRow, error) {
	var c audit.CheckpointRow
	var hash []byte
	if err := r.Scan(&c.Seq, &hash, &c.Signature); err != nil {
		return audit.CheckpointRow{}, err
	}
	if len(hash) == audit.HashSize {
		copy(c.Hash[:], hash)
	}
	return c, nil
}

// idOf converts a nullable UUID column; NULL is the zero id.
func idOf(u pgtype.UUID) ids.ID {
	if !u.Valid {
		return ids.ID{}
	}
	return ids.ID(u.Bytes)
}

// ThreadSample implements app.ScratchDB: at most n threads by random(), each with count(*), min
// and max of thread_seq of its messages and last_message_seq. With fewer threads every thread is
// returned.
func (s *ScratchDB) ThreadSample(ctx context.Context, n int) ([]app.ThreadSeqs, error) {
	rows, err := s.pool.Query(ctx, `SELECT t.id::text, count(m.thread_seq), COALESCE(min(m.thread_seq), 0)::bigint,
		COALESCE(max(m.thread_seq), 0)::bigint, t.last_message_seq
		FROM (SELECT id, last_message_seq FROM public.threads ORDER BY random() LIMIT $1) t
		LEFT JOIN public.messages m ON m.thread_id = t.id
		GROUP BY t.id, t.last_message_seq`, n)
	if err != nil {
		return nil, fmt.Errorf("status: sampling threads: %w", err)
	}
	out, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (app.ThreadSeqs, error) {
		var th app.ThreadSeqs
		err := r.Scan(&th.ThreadID, &th.Messages, &th.MinSeq, &th.MaxSeq, &th.LastMessageSeq)
		return th, err
	})
	if err != nil {
		return nil, fmt.Errorf("status: sampling threads: %w", err)
	}
	return out, nil
}

// FileSample implements app.ScratchDB: at most n files by random() among those with deleted_at
// IS NULL, upload_state complete and scan_status clean or skipped: a pending upload has no
// object yet, and the scan pass deletes the object of an infected or unscannable file
// (docs/DECISIONS.md D-M7-03-4).
func (s *ScratchDB) FileSample(ctx context.Context, n int) ([]app.SampleFile, error) {
	rows, err := s.pool.Query(ctx, `SELECT id::text, object_key, sha256 FROM public.files
		WHERE deleted_at IS NULL AND upload_state = 'complete' AND scan_status IN ('clean', 'skipped')
		ORDER BY random() LIMIT $1`, n)
	if err != nil {
		return nil, fmt.Errorf("status: sampling files: %w", err)
	}
	out, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (app.SampleFile, error) {
		var f app.SampleFile
		err := r.Scan(&f.ID, &f.ObjectKey, &f.SHA256)
		return f, err
	})
	if err != nil {
		return nil, fmt.Errorf("status: sampling files: %w", err)
	}
	return out, nil
}
