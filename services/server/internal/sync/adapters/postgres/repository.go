// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/otel/attr"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/sync"
	"github.com/OWNER/bothy/services/server/internal/sync/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/sync/app"
)

// sqlstateNoPartition is check_violation, which PostgreSQL raises for a row no partition takes.
const sqlstateNoPartition = "23514"

// msgNoPartition is the constant log line of an insert without a partition.
const msgNoPartition = "event_log has no partition for the new events; run the partition maintenance"

// Repository implements app.Writer and app.Partitions over a pool and reads the log for the
// tail. Inside a unit of work every statement uses the ambient transaction (pg.Querier). It is
// safe for concurrent use.
type Repository struct {
	pool *pg.Pool
	log  *slog.Logger
}

// Compile-time checks: Repository is the outgoing port of app.
var (
	_ app.Writer     = (*Repository)(nil)
	_ app.Partitions = (*Repository)(nil)
)

// NewRepository returns the repository over pool; log receives the error line of an insert
// without a partition (nil discards it).
func NewRepository(pool *pg.Pool, log *slog.Logger) *Repository {
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return &Repository{pool: pool, log: log}
}

// queries returns the sqlc queries on the ambient transaction of ctx or on the pool.
func (r *Repository) queries(ctx context.Context) *sqlcgen.Queries {
	return sqlcgen.New(pg.Querier(ctx, r.pool))
}

// Write inserts rows with seq first.. and notifies the log tails with the last seq, in one
// statement (app.Writer). It needs a unit of work. An insert without a partition for the rows (SQLSTATE 23514) is logged at error level with the
// first seq, never with payloads, and returned as kind unavailable: the partition maintenance is
// behind, which is an operational fault, not a bug of the caller.
func (r *Repository) Write(ctx context.Context, first, pv int64, rows []app.PendingEvent) error {
	tx, err := pg.Tx(ctx)
	if err != nil {
		return err
	}
	p := sqlcgen.InsertEventsParams{PermissionVersion: pv}
	for i, row := range rows {
		e := row.Event
		if e.SchemaVersion > math.MaxInt16 || e.SchemaVersion < 1 {
			return errs.New(errs.CodeInternalError, "sync: schema_version outside smallint")
		}
		p.Seqs = append(p.Seqs, first+int64(i))
		p.Ids = append(p.Ids, uuid(e.ID))
		p.Types = append(p.Types, e.Type)
		p.SchemaVersions = append(p.SchemaVersions, int16(e.SchemaVersion)) // #nosec G115 -- checked above
		p.ActorKinds = append(p.ActorKinds, string(e.ActorKind))
		p.ActorPrincipalIds = append(p.ActorPrincipalIds, uuid(e.ActorPrincipalID))
		p.SpaceIds = append(p.SpaceIds, uuid(e.SpaceID))
		p.ChannelIds = append(p.ChannelIds, uuid(e.ChannelID))
		p.ThreadIds = append(p.ThreadIds, uuid(e.ThreadID))
		p.ResourceKinds = append(p.ResourceKinds, e.ResourceKind)
		p.ResourceIds = append(p.ResourceIds, uuid(e.ResourceID))
		p.ResourceVersions = append(p.ResourceVersions, e.ResourceVersion)
		p.AudiencePrincipalIds = append(p.AudiencePrincipalIds, uuid(e.AudiencePrincipalID))
		p.ReceiveActions = append(p.ReceiveActions, e.ReceiveAction)
		p.Payloads = append(p.Payloads, string(e.Payload))
		p.Truncated = append(p.Truncated, e.Truncated)
		p.CorrelationIds = append(p.CorrelationIds, uuid(row.CorrelationID))
		p.TraceContexts = append(p.TraceContexts, row.TraceContext)
		p.ClientMsgIds = append(p.ClientMsgIds, uuid(e.ClientMsgID))
	}
	res, err := sqlcgen.New(tx).InsertEvents(ctx, p)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == sqlstateNoPartition {
		r.log.ErrorContext(ctx, msgNoPartition, slog.Int64(attr.EventSeq, first), slog.String(attr.ErrorType, pgErr.Code))
		last := first + int64(len(rows)) - 1
		return errs.Wrap(errs.CodeUnavailable, err, fmt.Sprintf("sync: event_log has no partition for seq %d to %d", first, last))
	}
	if err != nil {
		return err
	}
	if res.Inserted != int64(len(rows)) {
		return errs.New(errs.CodeInternalError, fmt.Sprintf("sync: inserted %d of %d events", res.Inserted, len(rows)))
	}
	return nil
}

// Head returns max(seq), 0 for an empty log (app.Partitions).
func (r *Repository) Head(ctx context.Context) (int64, error) {
	return r.queries(ctx).HeadSeq(ctx)
}

// MinRetainedSeq returns min(seq), 0 for an empty log: min_retained_seq of 04 "resync_required"
// (M1-63 reads it).
func (r *Repository) MinRetainedSeq(ctx context.Context) (int64, error) {
	return r.queries(ctx).MinRetainedSeq(ctx)
}

// Ensure calls sync_event_log_ensure_partitions (app.Partitions).
func (r *Repository) Ensure(ctx context.Context, size int64, ahead int, head int64) (int, error) {
	if ahead < 0 || ahead > math.MaxInt32 {
		return 0, errs.New(errs.CodeInternalError, "sync: partitions ahead outside int")
	}
	n, err := r.queries(ctx).EnsurePartitions(ctx, sqlcgen.EnsurePartitionsParams{Size: size, Ahead: int32(ahead), Head: head}) // #nosec G115 -- checked above
	return int(n), err
}

// DropExpired calls sync_event_log_drop_expired (app.Partitions).
func (r *Repository) DropExpired(ctx context.Context, cutoff time.Time, head int64, lockTimeout time.Duration) (int, error) {
	n, err := r.queries(ctx).DropExpiredPartitions(ctx, sqlcgen.DropExpiredPartitionsParams{
		Cutoff: pgtype.Timestamptz{Time: cutoff, Valid: true}, Head: head,
		LockTimeout: pgtype.Interval{Microseconds: lockTimeout.Microseconds(), Valid: true},
	})
	return int(n), err
}

// EventsAfter returns up to limit events with seq above after, in seq order, as the tail and the
// catch-up read them. Outside a unit of work it reads on the pool.
func (r *Repository) EventsAfter(ctx context.Context, after int64, limit int) ([]sync.Event, error) {
	if limit < 1 || limit > math.MaxInt32 {
		return nil, errs.New(errs.CodeInternalError, "sync: batch size outside int")
	}
	rows, err := r.queries(ctx).EventsAfter(ctx, sqlcgen.EventsAfterParams{AfterSeq: after, MaxRows: int32(limit)}) // #nosec G115 -- checked above
	if err != nil {
		return nil, err
	}
	out := make([]sync.Event, len(rows))
	for i, row := range rows {
		out[i] = toEvent(row)
	}
	return out, nil
}

// toEvent maps a row to the port type; NULL becomes the zero value. The payload is compacted:
// the text form of jsonb carries a space after every colon and comma, and the compact form of
// the normalised value is never larger than the payload the appender checked.
func toEvent(row sqlcgen.EventLog) sync.Event {
	payload := row.Payload
	var buf bytes.Buffer
	if err := json.Compact(&buf, payload); err == nil {
		payload = buf.Bytes()
	}
	return sync.Event{
		Seq: row.Seq, ID: idOf(row.ID), Type: row.Type, SchemaVersion: int(row.SchemaVersion),
		OccurredAt: row.OccurredAt.Time.UTC(), ActorKind: row.ActorKind.String,
		ActorPrincipalID: idOf(row.ActorPrincipalID), SpaceID: idOf(row.SpaceID),
		ChannelID: idOf(row.ChannelID), ThreadID: idOf(row.ThreadID), ResourceKind: row.ResourceKind,
		ResourceID: idOf(row.ResourceID), ResourceVersion: row.ResourceVersion.Int64,
		AudiencePrincipalID: idOf(row.AudiencePrincipalID), ReceiveAction: row.ReceiveAction,
		Payload: payload, Truncated: row.Truncated, Redacted: row.RedactedAt.Valid,
		CorrelationID: idOf(row.CorrelationID), TraceContext: row.TraceContext.String,
		ClientMsgID: idOf(row.ClientMsgID), PermissionVersion: row.PermissionVersion,
	}
}

// uuid converts an ID to the pgx type; the zero ID is NULL.
func uuid(id ids.ID) pgtype.UUID { return pgtype.UUID{Bytes: id, Valid: !id.IsZero()} }

// idOf converts a pgx UUID to an ID; NULL is the zero ID.
func idOf(u pgtype.UUID) ids.ID {
	if !u.Valid {
		return ids.ID{}
	}
	return ids.ID(u.Bytes)
}
