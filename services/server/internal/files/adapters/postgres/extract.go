// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/OWNER/bothy/services/server/internal/files/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/files/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Compile-time check: Repository is the repository of the text extraction as well (M3-15).
var _ app.ExtractRepository = (*Repository)(nil)

// programLimitExceeded is SQLSTATE 54000, which PostgreSQL raises when a tsvector exceeds its
// size limit (brief M3-15 design note 8).
const programLimitExceeded = "54000"

// InsertTextExtract implements app.ExtractRepository.
func (r *Repository) InsertTextExtract(ctx context.Context, fileID ids.ID, lang string) (bool, error) {
	q, err := r.write(ctx)
	if err != nil {
		return false, err
	}
	n, err := q.InsertTextExtract(ctx, sqlcgen.InsertTextExtractParams{FileID: uuid(fileID), SearchLanguage: lang})
	return n == 1, mapError(err, "inserting a text extract")
}

// ExtractTarget implements app.ExtractRepository.
func (r *Repository) ExtractTarget(ctx context.Context, fileID ids.ID) (app.ExtractTarget, bool, error) {
	row, err := r.read(ctx).GetExtractTarget(ctx, uuid(fileID))
	if errors.Is(err, pgx.ErrNoRows) {
		return app.ExtractTarget{}, false, nil
	}
	if err != nil {
		return app.ExtractTarget{}, false, mapError(err, "reading a text extract")
	}
	return app.ExtractTarget{State: row.State, File: app.FileRecord{
		ID: fileID, Purpose: row.Purpose, MIMEType: row.MimeType, SizeBytes: row.SizeBytes, ObjectKey: row.ObjectKey,
		UploadState: row.UploadState, ScanStatus: row.ScanStatus, Deleted: row.Deleted,
	}}, true, nil
}

// WriteExtract implements app.ExtractRepository: SQLSTATE 54000 (the tsvector size limit) becomes
// app.ErrVectorTooLarge.
func (r *Repository) WriteExtract(ctx context.Context, fileID ids.ID, w app.ExtractWrite) (bool, error) {
	q, err := r.write(ctx)
	if err != nil {
		return false, err
	}
	arg := sqlcgen.WriteTextExtractParams{State: w.State, Truncated: w.Truncated, Extractor: text(w.Version),
		ExtractedAt: ts(w.At), Error: text(w.Code), FileID: uuid(fileID)}
	if w.State == "done" {
		// The empty text is stored as '', not NULL.
		arg.TextContent.String, arg.TextContent.Valid = w.Text, true
	}
	n, err := q.WriteTextExtract(ctx, arg)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == programLimitExceeded {
		return false, app.ErrVectorTooLarge
	}
	return n == 1, mapError(err, "writing a text extract")
}

// DeleteTextExtract implements app.ExtractRepository.
func (r *Repository) DeleteTextExtract(ctx context.Context, fileID ids.ID) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	return mapError(q.DeleteTextExtract(ctx, uuid(fileID)), "deleting a text extract")
}

// DeleteTextExtractsOfDeleted implements app.Repository.
func (r *Repository) DeleteTextExtractsOfDeleted(ctx context.Context, fileIDs []ids.ID) error {
	if len(fileIDs) == 0 {
		return nil
	}
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	return mapError(q.DeleteTextExtractsOfDeletedFiles(ctx, uuids(fileIDs)), "deleting the text extracts of deleted files")
}

// RefreshCandidates implements app.ExtractRepository.
func (r *Repository) RefreshCandidates(ctx context.Context, after ids.ID, limit int) ([]app.RefreshCandidate, error) {
	q, err := r.write(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := q.ListRefreshCandidates(ctx, sqlcgen.ListRefreshCandidatesParams{After: uuid(after), BatchSize: int32(min(limit, 1<<30))}) //nolint:gosec // G115: bounded above
	if err != nil {
		return nil, mapError(err, "listing the files of the extraction walk")
	}
	out := make([]app.RefreshCandidate, 0, len(rows))
	for _, row := range rows {
		out = append(out, app.RefreshCandidate{FileID: idOf(row.ID), MIMEType: row.MimeType, SizeBytes: row.SizeBytes,
			HasRow: row.HasRow, State: row.State, Extractor: row.Extractor, Error: row.Error})
	}
	return out, nil
}

// ResetTextExtract implements app.ExtractRepository.
func (r *Repository) ResetTextExtract(ctx context.Context, fileID ids.ID, seenState, seenExtractor string) (bool, error) {
	q, err := r.write(ctx)
	if err != nil {
		return false, err
	}
	n, err := q.ResetTextExtract(ctx, sqlcgen.ResetTextExtractParams{FileID: uuid(fileID), SeenState: seenState, SeenExtractor: text(seenExtractor)})
	return n == 1, mapError(err, "resetting a text extract")
}

// DeleteOrphanTextExtracts implements app.ExtractRepository.
func (r *Repository) DeleteOrphanTextExtracts(ctx context.Context, limit int) (int, error) {
	q, err := r.write(ctx)
	if err != nil {
		return 0, err
	}
	n, err := q.DeleteOrphanTextExtracts(ctx, int32(min(limit, 1<<30))) //nolint:gosec // G115: bounded above
	return int(n), mapError(err, "deleting the text extracts of deleted files")
}
