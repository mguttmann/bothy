// SPDX-License-Identifier: Apache-2.0

// Package postgres is the PostgreSQL adapter of the audit module: the sqlc queries in queries/,
// the generated code in sqlcgen/ and Repository, which implements app.Store over audit_events and
// audit_ip_keys (docs/spec/03-datenmodell.md section 4.12). Only this package runs SQL on those
// tables (03 Grundsatz 12).
//
// Example:
//
//	repo := postgres.NewRepository(pool)
//	chain := app.NewChain(counters, repo, reporter)
package postgres

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/OWNER/bothy/services/server/internal/audit/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/audit/app"
	"github.com/OWNER/bothy/services/server/internal/audit/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
)

// Repository implements app.Store with the generated queries of sqlcgen. It is safe for
// concurrent use.
type Repository struct {
	pool *pg.Pool
}

// Compile-time check: Repository is the store of the audit application layer.
var _ app.Store = (*Repository)(nil)

// NewRepository returns the repository over pool. Reads outside a unit of work use pool; reads
// and writes inside one use its transaction (pg.Querier, pg.Tx).
func NewRepository(pool *pg.Pool) *Repository {
	return &Repository{pool: pool}
}

// read returns the queries for a read: the ambient transaction of ctx or the pool.
func (r *Repository) read(ctx context.Context) *sqlcgen.Queries {
	return sqlcgen.New(pg.Querier(ctx, r.pool))
}

// InsertRows inserts rows in one statement in the ambient transaction (app.Store). Outside a
// unit of work it fails (kind internal), because the rows would skip the audit hook.
func (r *Repository) InsertRows(ctx context.Context, rows []domain.Row) error {
	tx, err := pg.Tx(ctx)
	if err != nil {
		return err
	}
	p := sqlcgen.InsertRowsParams{}
	for _, row := range rows {
		details, err := domain.AppendCanonical(nil, row.Details)
		if err != nil {
			return err
		}
		p.Seqs = append(p.Seqs, row.Seq)
		p.Ids = append(p.Ids, optionalUUID(row.ID))
		p.OccurredAts = append(p.OccurredAts, pgtype.Timestamptz{Time: row.OccurredAt, Valid: true})
		p.ActorPrincipalIds = append(p.ActorPrincipalIds, optionalUUID(row.ActorPrincipalID))
		p.ActorDeviceIds = append(p.ActorDeviceIds, optionalUUID(row.ActorDeviceID))
		p.ActorSessionIds = append(p.ActorSessionIds, optionalUUID(row.ActorSessionID))
		p.Actions = append(p.Actions, row.Action)
		p.Decisions = append(p.Decisions, row.Decision)
		p.DenyReasons = append(p.DenyReasons, row.DenyReason)
		p.ResourceKinds = append(p.ResourceKinds, row.ResourceKind)
		p.ResourceIds = append(p.ResourceIds, optionalUUID(row.ResourceID))
		p.Details = append(p.Details, string(details))
		p.CorrelationIds = append(p.CorrelationIds, optionalUUID(row.CorrelationID))
		p.PrevHashes = append(p.PrevHashes, bytes.Clone(row.PrevHash[:]))
		p.Hashes = append(p.Hashes, bytes.Clone(row.Hash[:]))
	}
	return sqlcgen.New(tx).InsertRows(ctx, p)
}

// HashAt returns the hash of row seq (app.Store).
func (r *Repository) HashAt(ctx context.Context, seq int64) ([domain.HashSize]byte, bool, error) {
	var out [domain.HashSize]byte
	h, err := r.read(ctx).HashAt(ctx, seq)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, false, nil
	}
	if err != nil {
		return out, false, err
	}
	if len(h) != domain.HashSize {
		return out, false, fmt.Errorf("audit: row %d has a hash of %d bytes", seq, len(h))
	}
	copy(out[:], h)
	return out, true, nil
}

// Rows returns the rows fromSeq .. toSeq in chain order, as stored: details decoded with
// UseNumber, so integers stay exact and CanonicalRow recomputes the hash (app.Store).
func (r *Repository) Rows(ctx context.Context, fromSeq, toSeq int64) ([]domain.Row, error) {
	got, err := r.read(ctx).Rows(ctx, sqlcgen.RowsParams{FromSeq: fromSeq, ToSeq: toSeq})
	if err != nil {
		return nil, err
	}
	return rowsOf(got), nil
}

// rowsOf converts stored rows.
func rowsOf(got []sqlcgen.AuditEvent) []domain.Row {
	out := make([]domain.Row, 0, len(got))
	for _, e := range got {
		out = append(out, rowOf(e))
	}
	return out
}

// rowOf converts one stored row; details keep their integers exact (UseNumber).
func rowOf(e sqlcgen.AuditEvent) domain.Row {
	row := domain.Row{
		Seq: e.Seq, ID: idOf(e.ID), OccurredAt: e.OccurredAt.Time.UTC(),
		ActorPrincipalID: idOf(e.ActorPrincipalID), ActorDeviceID: idOf(e.ActorDeviceID),
		ActorSessionID: idOf(e.ActorSessionID), Action: e.Action, Decision: e.Decision,
		DenyReason: e.DenyReason.String, ResourceKind: e.ResourceKind.String, ResourceID: idOf(e.ResourceID),
		CorrelationID: idOf(e.CorrelationID),
	}
	// A row the recorder never wrote (details that are no object, a hash of another size) can
	// only come from a change outside the server: it reads with nil details or a zero hash, which
	// the chain walk reports as hash_mismatch or link_mismatch, instead of failing the read.
	dec := json.NewDecoder(bytes.NewReader(e.Details))
	dec.UseNumber()
	if err := dec.Decode(&row.Details); err != nil {
		row.Details = nil
	}
	if len(e.PrevHash) == domain.HashSize {
		copy(row.PrevHash[:], e.PrevHash)
	}
	if len(e.Hash) == domain.HashSize {
		copy(row.Hash[:], e.Hash)
	}
	return row
}

// Bounds returns the smallest retained and the largest seq (app.Store).
func (r *Repository) Bounds(ctx context.Context) (int64, int64, bool, error) {
	b, err := r.read(ctx).Bounds(ctx)
	if err != nil {
		return 0, 0, false, err
	}
	return b.FirstSeq, b.HeadSeq, b.HeadSeq > 0, nil
}

// ListEvents returns one page of rows matching f, newest first by seq (app.Store).
func (r *Repository) ListEvents(ctx context.Context, f app.EventFilter) ([]domain.Row, error) {
	p := sqlcgen.ListEventsParams{
		ActorID: optionalUUID(f.ActorID), ResourceID: optionalUUID(f.ResourceID),
		Action: optionalText(f.Action), Decision: optionalText(f.Decision), ResourceKind: optionalText(f.ResourceKind),
		FromAt: optionalTime(f.From), ToAt: optionalTime(f.To),
		BeforeSeq: pgtype.Int8{Int64: f.BeforeSeq, Valid: f.BeforeSeq > 0},
		RowLimit:  int32(min(max(f.Limit, 0), math.MaxInt32)),
	}
	got, err := r.read(ctx).ListEvents(ctx, p)
	if err != nil {
		return nil, err
	}
	return rowsOf(got), nil
}

// ExportSpan returns the smallest and the largest seq with occurred_at in [from, to) (app.Store).
func (r *Repository) ExportSpan(ctx context.Context, from, to time.Time) (int64, int64, bool, error) {
	s, err := r.read(ctx).ExportSpan(ctx, sqlcgen.ExportSpanParams{FromAt: optionalTime(from), ToAt: optionalTime(to)})
	if err != nil {
		return 0, 0, false, err
	}
	return s.FirstSeq, s.LastSeq, s.LastSeq > 0, nil
}

// LastCheckpoint returns the checkpoint with the largest seq (app.Store).
func (r *Repository) LastCheckpoint(ctx context.Context) (app.StoredCheckpoint, bool, error) {
	c, err := r.read(ctx).LastCheckpoint(ctx)
	if errors.Is(err, pgx.ErrNoRows) {
		return app.StoredCheckpoint{}, false, nil
	}
	if err != nil {
		return app.StoredCheckpoint{}, false, err
	}
	return checkpointOf(c.Seq, c.Hash, c.PreviousCheckpointHash, c.Signature, c.Kid), true, nil
}

// LockCheckpoints takes the checkpoint lock in the ambient transaction (app.Store).
func (r *Repository) LockCheckpoints(ctx context.Context) error {
	tx, err := pg.Tx(ctx)
	if err != nil {
		return err
	}
	return sqlcgen.New(tx).LockCheckpoints(ctx)
}

// InsertCheckpoint inserts c in the ambient transaction (app.Store).
func (r *Repository) InsertCheckpoint(ctx context.Context, c app.StoredCheckpoint) error {
	tx, err := pg.Tx(ctx)
	if err != nil {
		return err
	}
	p := sqlcgen.InsertCheckpointParams{Seq: c.Seq, Hash: bytes.Clone(c.Hash[:]), Signature: bytes.Clone(c.Signature), Kid: c.KID}
	if c.HasPrevious {
		p.PreviousCheckpointHash = bytes.Clone(c.Previous[:])
	}
	return sqlcgen.New(tx).InsertCheckpoint(ctx, p)
}

// UnexportedCheckpoints returns at most limit checkpoints with exported_at NULL in seq order
// (app.Store).
func (r *Repository) UnexportedCheckpoints(ctx context.Context, limit int) ([]app.StoredCheckpoint, error) {
	got, err := r.read(ctx).UnexportedCheckpoints(ctx, int32(min(max(limit, 0), math.MaxInt32)))
	if err != nil {
		return nil, err
	}
	out := make([]app.StoredCheckpoint, 0, len(got))
	for _, c := range got {
		out = append(out, checkpointOf(c.Seq, c.Hash, c.PreviousCheckpointHash, c.Signature, c.Kid))
	}
	return out, nil
}

// MarkExported sets exported_at of checkpoint seq once (app.Store).
func (r *Repository) MarkExported(ctx context.Context, seq int64, at time.Time) error {
	return r.read(ctx).MarkExported(ctx, sqlcgen.MarkExportedParams{Seq: seq, ExportedAt: pgtype.Timestamptz{Time: at, Valid: true}})
}

// checkpointOf converts the columns of a row of audit_checkpoints. A hash of another size reads
// as zero, which never matches a row.
func checkpointOf(seq int64, hash, previous, signature []byte, kid string) app.StoredCheckpoint {
	c := app.StoredCheckpoint{Seq: seq, Signature: signature, KID: kid}
	if len(hash) == domain.HashSize {
		copy(c.Hash[:], hash)
	}
	if previous != nil {
		c.HasPrevious = true
		if len(previous) == domain.HashSize {
			copy(c.Previous[:], previous)
		}
	}
	return c
}

// optionalText converts a filter; empty is NULL.
func optionalText(s string) pgtype.Text { return pgtype.Text{String: s, Valid: s != ""} }

// optionalTime converts a bound; the zero time is NULL.
func optionalTime(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: !t.IsZero()}
}

// CurrentIPKey returns the IP key whose period contains at (app.Store).
func (r *Repository) CurrentIPKey(ctx context.Context, at time.Time) (domain.IPKey, bool, error) {
	k, err := r.read(ctx).CurrentIPKey(ctx, pgtype.Timestamptz{Time: at, Valid: true})
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.IPKey{}, false, nil
	}
	if err != nil {
		return domain.IPKey{}, false, err
	}
	return ipKey(k.ID, k.ValidFrom, k.ValidUntil, k.KeyCiphertext, k.KeyRef), true, nil
}

// InsertIPKey inserts k unless its period exists (app.Store); it needs a unit of work.
func (r *Repository) InsertIPKey(ctx context.Context, k domain.IPKey) error {
	tx, err := pg.Tx(ctx)
	if err != nil {
		return err
	}
	return sqlcgen.New(tx).InsertIPKey(ctx, sqlcgen.InsertIPKeyParams{
		ID: optionalUUID(k.ID), ValidFrom: pgtype.Timestamptz{Time: k.ValidFrom, Valid: true},
		ValidUntil: pgtype.Timestamptz{Time: k.ValidUntil, Valid: true}, KeyCiphertext: k.Ciphertext, KeyRef: k.KeyRef,
	})
}

// IPKeyByID returns one IP key (app.Store).
func (r *Repository) IPKeyByID(ctx context.Context, id ids.ID) (domain.IPKey, bool, error) {
	k, err := r.read(ctx).IPKeyByID(ctx, optionalUUID(id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.IPKey{}, false, nil
	}
	if err != nil {
		return domain.IPKey{}, false, err
	}
	return ipKey(k.ID, k.ValidFrom, k.ValidUntil, k.KeyCiphertext, k.KeyRef), true, nil
}

// ipKey converts the columns of a row of audit_ip_keys.
func ipKey(id pgtype.UUID, from, until pgtype.Timestamptz, ciphertext []byte, ref string) domain.IPKey {
	return domain.IPKey{ID: idOf(id), ValidFrom: from.Time.UTC(), ValidUntil: until.Time.UTC(), Ciphertext: ciphertext, KeyRef: ref}
}

// optionalUUID converts an ID; the zero ID is NULL.
func optionalUUID(id ids.ID) pgtype.UUID { return pgtype.UUID{Bytes: id, Valid: !id.IsZero()} }

// idOf converts a nullable UUID; NULL is the zero ID.
func idOf(u pgtype.UUID) ids.ID {
	if !u.Valid {
		return ids.ID{}
	}
	return ids.ID(u.Bytes)
}
