// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/OWNER/bothy/services/server/internal/files/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/files/app"
	"github.com/OWNER/bothy/services/server/internal/files/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// ImageFile implements app.Repository (M3-06): the file by id with its thumbnails, deleted or not.
func (r *Repository) ImageFile(ctx context.Context, fileID ids.ID) (app.FileRecord, bool, error) {
	q := r.read(ctx)
	row, err := q.GetImageFile(ctx, uuid(fileID))
	return r.withThumbnails(ctx, q, row, err)
}

// LockImageFile implements app.Repository: as ImageFile with the files row locked FOR UPDATE.
func (r *Repository) LockImageFile(ctx context.Context, fileID ids.ID) (app.FileRecord, bool, error) {
	q, err := r.write(ctx)
	if err != nil {
		return app.FileRecord{}, false, err
	}
	row, err := q.LockImageFile(ctx, uuid(fileID))
	return r.withThumbnails(ctx, q, sqlcgen.GetImageFileRow(row), err)
}

// withThumbnails converts an image file row and reads its thumbnails; no row is found = false.
func (r *Repository) withThumbnails(ctx context.Context, q *sqlcgen.Queries, row sqlcgen.GetImageFileRow, err error) (app.FileRecord, bool, error) {
	if errors.Is(err, pgx.ErrNoRows) {
		return app.FileRecord{}, false, nil
	}
	if err != nil {
		return app.FileRecord{}, false, mapError(err, "reading an image file")
	}
	f := fileRecord(sqlcgen.GetFileRow{
		ID: row.ID, ObjectKey: row.ObjectKey, Sha256: row.Sha256, SizeBytes: row.SizeBytes, MimeType: row.MimeType,
		DeclaredMimeType: row.DeclaredMimeType, OriginalName: row.OriginalName, UploadedBy: row.UploadedBy, Purpose: row.Purpose,
		ScopeChannelID: row.ScopeChannelID, UploadState: row.UploadState, ScanStatus: row.ScanStatus, Width: row.Width,
		Height: row.Height, DurationMs: row.DurationMs, TranscriptStatus: row.TranscriptStatus,
		TranscriptLanguage: row.TranscriptLanguage, TranscriptSource: row.TranscriptSource, CreatedAt: row.CreatedAt,
	})
	f.Deleted = row.Deleted
	if f.Thumbnails, err = thumbnails(ctx, q, row.ID); err != nil {
		return app.FileRecord{}, false, err
	}
	return f, true, nil
}

// thumbnails reads the file_thumbnails rows of a file.
func thumbnails(ctx context.Context, q *sqlcgen.Queries, fileID pgtype.UUID) ([]app.ThumbnailRecord, error) {
	rows, err := q.ListFileThumbnails(ctx, fileID)
	if err != nil {
		return nil, mapError(err, "listing thumbnails")
	}
	out := make([]app.ThumbnailRecord, 0, len(rows))
	for _, t := range rows {
		out = append(out, app.ThumbnailRecord{Variant: domain.Variant(t.Variant), ObjectKey: t.ObjectKey, MIMEType: t.MimeType,
			Width: int(t.Width), Height: int(t.Height), SizeBytes: t.SizeBytes})
	}
	return out, nil
}

// StoreImageOriginal implements app.Repository: T1 of the job, guarded by the pending state.
func (r *Repository) StoreImageOriginal(ctx context.Context, fileID ids.ID, o app.StoredOriginal) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	n, err := q.StoreImageOriginal(ctx, sqlcgen.StoreImageOriginalParams{
		ObjectKey: o.ObjectKey, Sha256: o.SHA256[:], SizeBytes: o.SizeBytes, MimeType: o.MIMEType, Width: optionalInt(o.Width),
		Height: optionalInt(o.Height), ID: uuid(fileID),
	})
	if err != nil {
		return mapError(err, "storing the cleaned original")
	}
	if n != 1 {
		return errs.New(errs.CodeInternalError, "files: the image file is no longer pending")
	}
	return nil
}

// FailImage implements app.Repository: the file failed and the reason in its session.
func (r *Repository) FailImage(ctx context.Context, fileID ids.ID, reason string) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	n, err := q.FailImageFile(ctx, uuid(fileID))
	if err != nil {
		return mapError(err, "failing an image file")
	}
	if n != 1 {
		return errs.New(errs.CodeInternalError, "files: the image file is no longer pending")
	}
	return mapError(q.SetSessionFailureReason(ctx, sqlcgen.SetSessionFailureReasonParams{FileID: uuid(fileID), FailureReason: text(reason)}),
		"writing the failure reason")
}

// InsertThumbnails implements app.Repository: ON CONFLICT DO NOTHING per row.
func (r *Repository) InsertThumbnails(ctx context.Context, fileID ids.ID, rows []app.ThumbnailRecord) (int, error) {
	q, err := r.write(ctx)
	if err != nil {
		return 0, err
	}
	inserted := 0
	for _, t := range rows {
		n, err := q.InsertFileThumbnail(ctx, sqlcgen.InsertFileThumbnailParams{
			FileID: uuid(fileID), Variant: string(t.Variant), ObjectKey: t.ObjectKey, MimeType: t.MIMEType,
			Width: int32(t.Width), Height: int32(t.Height), SizeBytes: t.SizeBytes, //nolint:gosec // G115: at most file.thumbnail.large_px
		})
		if err != nil {
			return inserted, mapError(err, "inserting a thumbnail")
		}
		inserted += int(n)
	}
	return inserted, nil
}

// SetMediaInfo implements app.Repository: the non-zero values; zero keeps the column.
func (r *Repository) SetMediaInfo(ctx context.Context, fileID ids.ID, m app.MediaInfo) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	_, err = q.SetFileMediaInfo(ctx, sqlcgen.SetFileMediaInfoParams{
		Width: optionalInt(m.Width), Height: optionalInt(m.Height), DurationMs: pgtype.Int8{Int64: m.DurationMS, Valid: m.DurationMS != 0},
		ID: uuid(fileID),
	})
	return mapError(err, "writing the media values")
}

// CompleteContainerFile implements app.Repository: the non-zero values of m and upload_state
// complete for a pending, clean or skipped file (M3-09 design note 5).
func (r *Repository) CompleteContainerFile(ctx context.Context, fileID ids.ID, m app.MediaInfo) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	_, err = q.CompleteContainerFile(ctx, sqlcgen.CompleteContainerFileParams{
		Width: optionalInt(m.Width), Height: optionalInt(m.Height), DurationMs: pgtype.Int8{Int64: m.DurationMS, Valid: m.DurationMS != 0},
		ID: uuid(fileID),
	})
	return mapError(err, "completing a container file")
}

// optionalInt converts a size: 0 becomes NULL; values beyond int32 cannot come from a decoder
// that respects file.image.max_pixels and are stored as NULL.
func optionalInt(v int) pgtype.Int4 {
	if v <= 0 || v > 1<<31-1 {
		return pgtype.Int4{}
	}
	return pgtype.Int4{Int32: int32(v), Valid: true}
}
