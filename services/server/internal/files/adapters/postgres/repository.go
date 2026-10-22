// SPDX-License-Identifier: Apache-2.0

// Package postgres is the PostgreSQL adapter of the files module (M3-03): the sqlc queries in
// queries/, the generated code in sqlcgen/ and Repository, which implements app.Repository over
// the tables files, file_upload_sessions and file_upload_parts (docs/spec/03-data-model.md 4.7).
// Only this package runs SQL on them (03 principle 12). Every read is keyed by a file id or by the
// uploader (07 7.4 rule 1).
//
// Example:
//
//	repo := postgres.NewRepository(pool)
//	svc, err := app.New(cfg, app.Deps{Repo: repo, ...})
package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/OWNER/bothy/services/server/internal/files/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/files/app"
	"github.com/OWNER/bothy/services/server/internal/files/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
)

// Repository implements app.Repository with the generated queries of sqlcgen. It is safe for
// concurrent use.
type Repository struct {
	pool *pg.Pool
}

// Compile-time check: Repository is the repository of the files application layer.
var _ app.Repository = (*Repository)(nil)

// NewRepository returns the repository over pool. Reads outside a unit of work use pool; reads
// and writes inside one use its transaction.
func NewRepository(pool *pg.Pool) *Repository { return &Repository{pool: pool} }

// read returns the queries for a read: the ambient transaction of ctx or the pool.
func (r *Repository) read(ctx context.Context) *sqlcgen.Queries {
	return sqlcgen.New(pg.Querier(ctx, r.pool))
}

// write returns the queries for a write: only the ambient transaction of ctx.
func (r *Repository) write(ctx context.Context) (*sqlcgen.Queries, error) {
	tx, err := pg.Tx(ctx)
	if err != nil {
		return nil, err
	}
	return sqlcgen.New(tx), nil
}

// LockPrincipalUploads implements app.Repository.
func (r *Repository) LockPrincipalUploads(ctx context.Context, principal ids.ID) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	return mapError(q.LockPrincipalUploads(ctx, uuid(principal)), "locking the uploads of a principal")
}

// QuotaUsedBytes implements app.Repository.
func (r *Repository) QuotaUsedBytes(ctx context.Context, principal ids.ID) (int64, error) {
	n, err := r.read(ctx).QuotaUsedBytes(ctx, uuid(principal))
	return n, mapError(err, "summing the quota")
}

// OpenSessions implements app.Repository.
func (r *Repository) OpenSessions(ctx context.Context, principal ids.ID, now time.Time) (int, time.Time, error) {
	row, err := r.read(ctx).CountOpenSessions(ctx, sqlcgen.CountOpenSessionsParams{UploadedBy: uuid(principal), Now: ts(now)})
	if err != nil {
		return 0, time.Time{}, mapError(err, "counting open sessions")
	}
	if row.Open == 0 {
		return 0, time.Time{}, nil
	}
	return int(row.Open), timeOf(row.Earliest), nil
}

// InsertUpload implements app.Repository.
func (r *Repository) InsertUpload(ctx context.Context, u app.NewUpload) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	f, s := u.File, u.Session
	if err := q.InsertFile(ctx, sqlcgen.InsertFileParams{
		ID: uuid(f.ID), ObjectKey: f.ObjectKey, Sha256: f.SHA256[:], SizeBytes: f.SizeBytes, MimeType: f.MIMEType,
		DeclaredMimeType: text(f.DeclaredMIMEType), OriginalName: f.OriginalName, UploadedBy: uuid(f.UploadedBy), Purpose: f.Purpose,
		ScopeChannelID: optionalUUID(f.ScopeChannelID), UploadState: f.UploadState, ScanStatus: f.ScanStatus,
		TranscriptStatus: f.TranscriptStatus, CreatedAt: ts(f.CreatedAt),
	}); err != nil {
		return mapError(err, "inserting a file")
	}
	return mapError(q.InsertSession(ctx, sqlcgen.InsertSessionParams{
		FileID: uuid(s.FileID), Mode: s.Mode, State: string(s.State), DeclaredSizeBytes: s.DeclaredSizeBytes,
		DeclaredSha256: s.DeclaredSHA256[:], PartSizeBytes: s.PartSizeBytes, PartCount: int32(s.PartCount), //nolint:gosec // G115: at most the S3 part limit (upload_part_plan)
		CreatedAt: ts(s.CreatedAt), ExpiresAt: ts(s.ExpiresAt),
	}), "inserting an upload session")
}

// Session implements app.Repository.
func (r *Repository) Session(ctx context.Context, fileID, principal ids.ID) (app.SessionRecord, bool, error) {
	row, err := r.read(ctx).GetSession(ctx, sqlcgen.GetSessionParams{FileID: uuid(fileID), UploadedBy: uuid(principal)})
	return oneSession(row, err)
}

// LockSession implements app.Repository.
func (r *Repository) LockSession(ctx context.Context, fileID, principal ids.ID, update bool) (app.SessionRecord, bool, error) {
	q, err := r.write(ctx)
	if err != nil {
		return app.SessionRecord{}, false, err
	}
	if update {
		row, err := q.LockSessionForUpdate(ctx, sqlcgen.LockSessionForUpdateParams{FileID: uuid(fileID), UploadedBy: uuid(principal)})
		return oneSession(sqlcgen.GetSessionRow(row), err)
	}
	row, err := q.LockSessionForShare(ctx, sqlcgen.LockSessionForShareParams{FileID: uuid(fileID), UploadedBy: uuid(principal)})
	return oneSession(sqlcgen.GetSessionRow(row), err)
}

// SetStorageUploadID implements app.Repository.
func (r *Repository) SetStorageUploadID(ctx context.Context, fileID ids.ID, uploadID string) (bool, error) {
	q, err := r.write(ctx)
	if err != nil {
		return false, err
	}
	n, err := q.SetStorageUploadID(ctx, sqlcgen.SetStorageUploadIDParams{FileID: uuid(fileID), StorageUploadID: text(uploadID)})
	return n == 1, mapError(err, "storing the multipart upload id")
}

// Parts implements app.Repository.
func (r *Repository) Parts(ctx context.Context, fileID ids.ID) ([]app.PartRecord, error) {
	rows, err := r.read(ctx).ListParts(ctx, uuid(fileID))
	if err != nil {
		return nil, mapError(err, "listing parts")
	}
	out := make([]app.PartRecord, 0, len(rows))
	for _, p := range rows {
		out = append(out, partOf(p))
	}
	return out, nil
}

// Part implements app.Repository.
func (r *Repository) Part(ctx context.Context, fileID ids.ID, n int) (app.PartRecord, bool, error) {
	row, err := r.read(ctx).GetPart(ctx, sqlcgen.GetPartParams{FileID: uuid(fileID), PartNumber: int32(n)}) //nolint:gosec // G115: checked against part_count
	if errors.Is(err, pgx.ErrNoRows) {
		return app.PartRecord{}, false, nil
	}
	if err != nil {
		return app.PartRecord{}, false, mapError(err, "reading a part")
	}
	return partOf(row), true, nil
}

// InsertPart implements app.Repository.
func (r *Repository) InsertPart(ctx context.Context, p app.PartRecord) (bool, error) {
	q, err := r.write(ctx)
	if err != nil {
		return false, err
	}
	n, err := q.InsertPart(ctx, sqlcgen.InsertPartParams{
		FileID: uuid(p.FileID), PartNumber: int32(p.PartNumber), SizeBytes: p.SizeBytes, Sha256: p.SHA256[:], //nolint:gosec // G115: checked against part_count
		Etag: text(p.ETag), ReceivedAt: ts(p.ReceivedAt),
	})
	return n == 1, mapError(err, "inserting a part")
}

// CompleteSession implements app.Repository.
func (r *Repository) CompleteSession(ctx context.Context, fileID ids.ID, at time.Time) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	n, err := q.CompleteSession(ctx, sqlcgen.CompleteSessionParams{FileID: uuid(fileID), CompletedAt: ts(at)})
	if err == nil && n != 1 {
		err = errs.New(errs.CodeInternalError, "files: completing a session that is not open")
	}
	return mapError(err, "completing a session")
}

// AbortSession implements app.Repository: the session aborted, the file failed and the part rows
// deleted, in the unit of work of ctx.
func (r *Repository) AbortSession(ctx context.Context, fileID ids.ID) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	n, err := q.AbortSession(ctx, uuid(fileID))
	if err != nil {
		return mapError(err, "aborting a session")
	}
	if n != 1 {
		return errs.New(errs.CodeInternalError, "files: aborting a session that is not open")
	}
	if err := q.FailFile(ctx, uuid(fileID)); err != nil {
		return mapError(err, "failing a file")
	}
	return mapError(q.DeleteParts(ctx, uuid(fileID)), "deleting parts")
}

// File implements app.Repository: the file with its thumbnails (M3-06).
func (r *Repository) File(ctx context.Context, fileID ids.ID) (app.FileRecord, bool, error) {
	q := r.read(ctx)
	row, err := q.GetFile(ctx, uuid(fileID))
	if errors.Is(err, pgx.ErrNoRows) {
		return app.FileRecord{}, false, nil
	}
	if err != nil {
		return app.FileRecord{}, false, mapError(err, "reading a file")
	}
	f := fileRecord(row)
	if f.Thumbnails, err = thumbnails(ctx, q, row.ID); err != nil {
		return app.FileRecord{}, false, err
	}
	return f, true, nil
}

// fileRecord converts a files row.
func fileRecord(row sqlcgen.GetFileRow) app.FileRecord {
	return app.FileRecord{
		ID: idOf(row.ID), ObjectKey: row.ObjectKey, SHA256: digest(row.Sha256), SizeBytes: row.SizeBytes, MIMEType: row.MimeType,
		DeclaredMIMEType: row.DeclaredMimeType.String, OriginalName: row.OriginalName, UploadedBy: idOf(row.UploadedBy),
		Purpose: row.Purpose, ScopeChannelID: idOf(row.ScopeChannelID), UploadState: row.UploadState, ScanStatus: row.ScanStatus,
		Width: int(row.Width.Int32), Height: int(row.Height.Int32), DurationMS: row.DurationMs.Int64,
		TranscriptStatus: row.TranscriptStatus, TranscriptLanguage: row.TranscriptLanguage.String,
		TranscriptSource: row.TranscriptSource.String, CreatedAt: timeOf(row.CreatedAt),
	}
}

// oneSession converts the result of a session read; no row is found = false.
func oneSession(row sqlcgen.GetSessionRow, err error) (app.SessionRecord, bool, error) {
	if errors.Is(err, pgx.ErrNoRows) {
		return app.SessionRecord{}, false, nil
	}
	if err != nil {
		return app.SessionRecord{}, false, mapError(err, "reading a session")
	}
	return app.SessionRecord{
		FileID: idOf(row.FileID), Mode: row.Mode, State: domain.SessionState(row.State), DeclaredSizeBytes: row.DeclaredSizeBytes,
		DeclaredSHA256: digest(row.DeclaredSha256), PartSizeBytes: row.PartSizeBytes, PartCount: int(row.PartCount),
		StorageUploadID: row.StorageUploadID.String, CreatedAt: timeOf(row.CreatedAt), CompletedAt: timeOf(row.CompletedAt),
		ExpiresAt: timeOf(row.ExpiresAt),
		File: app.FileRecord{
			ID: idOf(row.FileID), ObjectKey: row.ObjectKey, SHA256: digest(row.Sha256), SizeBytes: row.SizeBytes, MIMEType: row.MimeType,
			DeclaredMIMEType: row.DeclaredMimeType.String, OriginalName: row.OriginalName, UploadedBy: idOf(row.UploadedBy),
			Purpose: row.Purpose, ScopeChannelID: idOf(row.ScopeChannelID), UploadState: row.UploadState, ScanStatus: row.ScanStatus,
			Width: int(row.Width.Int32), Height: int(row.Height.Int32), DurationMS: row.DurationMs.Int64,
			TranscriptStatus: row.TranscriptStatus, TranscriptLanguage: row.TranscriptLanguage.String,
			TranscriptSource: row.TranscriptSource.String, CreatedAt: timeOf(row.FileCreatedAt),
		},
	}, true, nil
}

// partOf converts a part row.
func partOf(p sqlcgen.FileUploadPart) app.PartRecord {
	return app.PartRecord{FileID: idOf(p.FileID), PartNumber: int(p.PartNumber), SizeBytes: p.SizeBytes, SHA256: digest(p.Sha256),
		ETag: p.Etag.String, ReceivedAt: timeOf(p.ReceivedAt)}
}

// mapError wraps a database error; nil and errs errors stay.
func mapError(err error, what string) error {
	if err == nil {
		return nil
	}
	if _, ok := errs.As(err); ok {
		return err
	}
	return errs.Wrap(errs.CodeInternalError, err, "files: "+what+" failed")
}

// digest converts a bytea column of 32 bytes; another length gives the zero digest.
func digest(b []byte) [32]byte {
	var out [32]byte
	if len(b) == len(out) {
		copy(out[:], b)
	}
	return out
}

// uuid converts an id for a parameter.
func uuid(id ids.ID) pgtype.UUID { return pgtype.UUID{Bytes: id, Valid: true} }

// optionalUUID converts an optional id: the zero ID becomes NULL.
func optionalUUID(id ids.ID) pgtype.UUID { return pgtype.UUID{Bytes: id, Valid: !id.IsZero()} }

// idOf converts a column; NULL becomes the zero ID.
func idOf(u pgtype.UUID) ids.ID {
	if !u.Valid {
		return ids.ID{}
	}
	return ids.ID(u.Bytes)
}

// text converts an optional text: "" becomes NULL.
func text(s string) pgtype.Text { return pgtype.Text{String: s, Valid: s != ""} }

// ts converts a time for a parameter.
func ts(t time.Time) pgtype.Timestamptz { return pgtype.Timestamptz{Time: t, Valid: true} }

// timeOf converts a column; NULL becomes the zero time.
func timeOf(t pgtype.Timestamptz) time.Time {
	if !t.Valid {
		return time.Time{}
	}
	return t.Time.UTC()
}
