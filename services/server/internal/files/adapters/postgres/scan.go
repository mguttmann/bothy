// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/OWNER/bothy/services/server/internal/files/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/files/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Compile-time check: Repository is the repository of the scan pass as well (M3-04).
var _ app.ScanRepository = (*Repository)(nil)

// ScanTarget implements app.ScanRepository.
func (r *Repository) ScanTarget(ctx context.Context, fileID ids.ID) (app.SessionRecord, bool, error) {
	row, err := r.read(ctx).GetScanTarget(ctx, uuid(fileID))
	return scanTarget(row, err)
}

// LockScanTarget implements app.ScanRepository: the session row and then the files row FOR UPDATE.
func (r *Repository) LockScanTarget(ctx context.Context, fileID ids.ID) (app.SessionRecord, bool, error) {
	q, err := r.write(ctx)
	if err != nil {
		return app.SessionRecord{}, false, err
	}
	row, err := q.LockScanTarget(ctx, uuid(fileID))
	return scanTarget(sqlcgen.GetScanTargetRow(row), err)
}

// FinishScan implements app.ScanRepository: the file, guarded by its pending state, then the
// failure reason of the session. A file that no longer awaits the pass is an internal error: the
// caller locked and checked it in the same unit of work.
func (r *Repository) FinishScan(ctx context.Context, fileID ids.ID, res app.ScanResult) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	n, err := q.FinishScanFile(ctx, sqlcgen.FinishScanFileParams{
		UploadState: res.UploadState, ScanStatus: res.ScanStatus, MimeType: res.MIMEType, Sha256: res.SHA256[:],
		SizeBytes: res.SizeBytes, ScannedAt: optionalTS(res.ScannedAt), ScanEngine: text(res.ScanEngine), ID: uuid(fileID),
	})
	if err != nil {
		return mapError(err, "writing the scan result")
	}
	if n != 1 {
		return errs.New(errs.CodeInternalError, "files: the scanned file no longer awaits the pass")
	}
	return mapError(q.SetSessionFailureReason(ctx, sqlcgen.SetSessionFailureReasonParams{
		FileID: uuid(fileID), FailureReason: text(res.FailureReason),
	}), "writing the failure reason")
}

// CountAwaitingScan implements app.ScanRepository.
func (r *Repository) CountAwaitingScan(ctx context.Context) (int64, error) {
	n, err := r.read(ctx).CountAwaitingScan(ctx)
	return n, mapError(err, "counting the files awaiting the scan")
}

// scanTarget converts a scan target row; no row is found = false.
func scanTarget(row sqlcgen.GetScanTargetRow, err error) (app.SessionRecord, bool, error) {
	if errors.Is(err, pgx.ErrNoRows) {
		return app.SessionRecord{}, false, nil
	}
	if err != nil {
		return app.SessionRecord{}, false, mapError(err, "reading a scan target")
	}
	s, _, err := oneSession(sqlcgen.GetSessionRow{
		FileID: row.FileID, Mode: row.Mode, State: row.State, DeclaredSizeBytes: row.DeclaredSizeBytes,
		DeclaredSha256: row.DeclaredSha256, PartSizeBytes: row.PartSizeBytes, PartCount: row.PartCount,
		StorageUploadID: row.StorageUploadID, CreatedAt: row.CreatedAt, CompletedAt: row.CompletedAt, ExpiresAt: row.ExpiresAt,
		ObjectKey: row.ObjectKey, Sha256: row.Sha256, SizeBytes: row.SizeBytes, MimeType: row.MimeType,
		DeclaredMimeType: row.DeclaredMimeType, OriginalName: row.OriginalName, UploadedBy: row.UploadedBy, Purpose: row.Purpose,
		ScopeChannelID: row.ScopeChannelID, UploadState: row.UploadState, ScanStatus: row.ScanStatus, Width: row.Width,
		Height: row.Height, DurationMs: row.DurationMs, TranscriptStatus: row.TranscriptStatus,
		TranscriptLanguage: row.TranscriptLanguage, TranscriptSource: row.TranscriptSource, FileCreatedAt: row.FileCreatedAt,
	}, nil)
	if err != nil {
		return app.SessionRecord{}, false, err
	}
	s.File.Deleted = row.Deleted
	return s, true, nil
}

// optionalTS converts an optional time: the zero time becomes NULL.
func optionalTS(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: !t.IsZero()}
}
