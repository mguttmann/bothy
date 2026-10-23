// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/OWNER/bothy/services/server/internal/files/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/files/app"
	"github.com/OWNER/bothy/services/server/internal/files/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Compile-time check: Repository is the store of the purge, the cleanup and deleteFile (M3-08).
var _ app.CleanupStore = (*Repository)(nil)

// ExpiredOpenSessions implements app.CleanupStore.
func (r *Repository) ExpiredOpenSessions(ctx context.Context, now time.Time, after app.Candidate, limit int) ([]app.Candidate, error) {
	rows, err := r.read(ctx).ExpiredOpenSessions(ctx, sqlcgen.ExpiredOpenSessionsParams{Now: ts(now), AfterTime: ts(after.At),
		AfterID: uuid(after.FileID), BatchSize: int32(limit)}) //nolint:gosec // limit is file.gc_batch_size, a small positive int
	if err != nil {
		return nil, mapError(err, "reading expired sessions")
	}
	out := make([]app.Candidate, 0, len(rows))
	for _, row := range rows {
		out = append(out, app.Candidate{FileID: idOf(row.FileID), At: timeOf(row.ExpiresAt)})
	}
	return out, nil
}

// UnattachedCandidates implements app.CleanupStore.
func (r *Repository) UnattachedCandidates(ctx context.Context, cutoff time.Time, after app.Candidate, limit int) ([]app.Candidate, error) {
	rows, err := r.read(ctx).UnattachedCandidates(ctx, sqlcgen.UnattachedCandidatesParams{Cutoff: ts(cutoff), AfterTime: ts(after.At),
		AfterID: uuid(after.FileID), BatchSize: int32(limit)}) //nolint:gosec // limit is file.gc_batch_size, a small positive int
	if err != nil {
		return nil, mapError(err, "reading unattached files")
	}
	out := make([]app.Candidate, 0, len(rows))
	for _, row := range rows {
		out = append(out, app.Candidate{FileID: idOf(row.ID), At: timeOf(row.CreatedAt)})
	}
	return out, nil
}

// FailedRowCandidates implements app.CleanupStore.
func (r *Repository) FailedRowCandidates(ctx context.Context, cutoff time.Time, after app.Candidate, limit int) ([]app.Candidate, error) {
	rows, err := r.read(ctx).FailedRowCandidates(ctx, sqlcgen.FailedRowCandidatesParams{Cutoff: ts(cutoff), AfterTime: ts(after.At),
		AfterID: uuid(after.FileID), BatchSize: int32(limit)}) //nolint:gosec // limit is file.gc_batch_size, a small positive int
	if err != nil {
		return nil, mapError(err, "reading failed rows")
	}
	out := make([]app.Candidate, 0, len(rows))
	for _, row := range rows {
		out = append(out, app.Candidate{FileID: idOf(row.ID), At: timeOf(row.CreatedAt)})
	}
	return out, nil
}

// DeletedRows implements app.CleanupStore.
func (r *Repository) DeletedRows(ctx context.Context, after app.Candidate, limit int) ([]app.Candidate, error) {
	rows, err := r.read(ctx).DeletedRows(ctx, sqlcgen.DeletedRowsParams{AfterTime: ts(after.At), AfterID: uuid(after.FileID),
		BatchSize: int32(limit)}) //nolint:gosec // limit is file.gc_batch_size, a small positive int
	if err != nil {
		return nil, mapError(err, "reading deleted rows")
	}
	out := make([]app.Candidate, 0, len(rows))
	for _, row := range rows {
		out = append(out, app.Candidate{FileID: idOf(row.ID), At: timeOf(row.DeletedAt)})
	}
	return out, nil
}

// PurgeBacklog implements app.CleanupStore.
func (r *Repository) PurgeBacklog(ctx context.Context) (int64, time.Time, error) {
	row, err := r.read(ctx).PurgeBacklog(ctx)
	if err != nil {
		return 0, time.Time{}, mapError(err, "counting the purge backlog")
	}
	if row.Backlog == 0 {
		return 0, time.Time{}, nil
	}
	return row.Backlog, timeOf(row.Oldest), nil
}

// PurgeRecord implements app.CleanupStore.
func (r *Repository) PurgeRecord(ctx context.Context, fileID ids.ID) (app.PurgeRecord, bool, error) {
	row, err := r.read(ctx).PurgeFacts(ctx, uuid(fileID))
	if errors.Is(err, pgx.ErrNoRows) {
		return app.PurgeRecord{}, false, nil
	}
	if err != nil {
		return app.PurgeRecord{}, false, mapError(err, "reading purge facts")
	}
	return app.PurgeRecord{
		Facts:           domain.PurgeFacts{Exists: true, Deleted: row.Deleted, UploadState: row.UploadState, ScanStatus: row.ScanStatus},
		ObjectKey:       row.ObjectKey,
		UploadedBy:      idOf(row.UploadedBy),
		HasSession:      row.SessionState.Valid,
		PartCount:       int(row.PartCount.Int32),
		StorageUploadID: row.StorageUploadID.String,
	}, true, nil
}

// LockFile implements app.CleanupStore: the files row FOR UPDATE, deleted or not.
func (r *Repository) LockFile(ctx context.Context, fileID ids.ID) (bool, error) {
	q, err := r.write(ctx)
	if err != nil {
		return false, err
	}
	_, err = q.LockFileForCleanup(ctx, uuid(fileID))
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return err == nil, mapError(err, "locking a file")
}

// LockFileForDelete implements app.CleanupStore: the files row FOR UPDATE after authorize of
// file.delete, deleted or not.
func (r *Repository) LockFileForDelete(ctx context.Context, fileID ids.ID) (bool, error) {
	q, err := r.write(ctx)
	if err != nil {
		return false, err
	}
	_, err = q.LockFileForDelete(ctx, uuid(fileID))
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return err == nil, mapError(err, "locking a file")
}

// CleanupRecord implements app.CleanupStore.
func (r *Repository) CleanupRecord(ctx context.Context, fileID ids.ID) (app.CleanupRecord, bool, error) {
	row, err := r.read(ctx).CleanupFileFacts(ctx, uuid(fileID))
	if errors.Is(err, pgx.ErrNoRows) {
		return app.CleanupRecord{}, false, nil
	}
	if err != nil {
		return app.CleanupRecord{}, false, mapError(err, "reading cleanup facts")
	}
	return app.CleanupRecord{
		UploadedBy: idOf(row.UploadedBy), Purpose: row.Purpose, UploadState: row.UploadState, ScanStatus: row.ScanStatus,
		Deleted: row.Deleted, Attached: row.Attached,
		Ages: domain.CleanupFacts{CreatedAt: timeOf(row.CreatedAt), ScannedAt: timeOf(row.ScannedAt),
			SessionState: domain.SessionState(row.SessionState.String), CompletedAt: timeOf(row.CompletedAt), ExpiresAt: timeOf(row.ExpiresAt)},
	}, true, nil
}

// FileForDelete implements app.CleanupStore: what fileResource needs, deleted rows included.
func (r *Repository) FileForDelete(ctx context.Context, fileID ids.ID) (app.FileRecord, bool, error) {
	row, err := r.read(ctx).FileForDelete(ctx, uuid(fileID))
	if errors.Is(err, pgx.ErrNoRows) {
		return app.FileRecord{}, false, nil
	}
	if err != nil {
		return app.FileRecord{}, false, mapError(err, "reading a file")
	}
	return app.FileRecord{ID: idOf(row.ID), UploadedBy: idOf(row.UploadedBy), Purpose: row.Purpose,
		ScopeChannelID: idOf(row.ScopeChannelID), Deleted: row.Deleted}, true, nil
}

// ExpireSession implements app.CleanupStore: the open session expired, its file failed and its
// part rows deleted, in the unit of work of ctx.
func (r *Repository) ExpireSession(ctx context.Context, fileID ids.ID) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	n, err := q.ExpireSession(ctx, uuid(fileID))
	if err != nil {
		return mapError(err, "expiring a session")
	}
	if n != 1 {
		return errs.New(errs.CodeInternalError, "files: expiring a session that is not open")
	}
	if err := q.FailFile(ctx, uuid(fileID)); err != nil {
		return mapError(err, "failing a file")
	}
	return mapError(q.DeleteParts(ctx, uuid(fileID)), "deleting parts")
}

// DeleteFileRow implements app.CleanupStore.
func (r *Repository) DeleteFileRow(ctx context.Context, fileID ids.ID) (bool, error) {
	q, err := r.write(ctx)
	if err != nil {
		return false, err
	}
	n, err := q.DeleteFileRow(ctx, uuid(fileID))
	return n == 1, mapError(err, "deleting a file row")
}

// AbortOpenSessions implements app.Repository (markDeleted, brief M3-08 design note 2 b).
func (r *Repository) AbortOpenSessions(ctx context.Context, fileIDs []ids.ID) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	return mapError(q.AbortOpenSessionsOfFiles(ctx, uuids(fileIDs)), "aborting sessions of deleted files")
}
