// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/OWNER/bothy/services/server/internal/files/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/files/app"
	"github.com/OWNER/bothy/services/server/internal/files/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Compile-time check: Repository is the port of the subscriber of chat.ThreadMoved (M3-07).
var _ app.ThreadAttachments = (*Repository)(nil)

// fileIDKey is the unique constraint on message_attachments(file_id) (migration 00064).
const fileIDKey = "message_attachments_file_id_key"

// uniqueViolation is the SQLSTATE of a unique violation.
const uniqueViolation = "23505"

// LockAttachTargets implements app.Repository: the files rows FOR UPDATE OF files in ascending id
// with the state of their session, then their thumbnails in one more statement.
func (r *Repository) LockAttachTargets(ctx context.Context, fileIDs []ids.ID) ([]app.AttachTarget, error) {
	q, err := r.write(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := q.LockAttachTargets(ctx, uuids(fileIDs))
	if err != nil {
		return nil, mapError(err, "locking the files of an attach")
	}
	out := make([]app.AttachTarget, 0, len(rows))
	found := make([]ids.ID, 0, len(rows))
	for _, row := range rows {
		f := fileRecord(sqlcgen.GetFileRow{
			ID: row.ID, ObjectKey: row.ObjectKey, Sha256: row.Sha256, SizeBytes: row.SizeBytes, MimeType: row.MimeType,
			DeclaredMimeType: row.DeclaredMimeType, OriginalName: row.OriginalName, UploadedBy: row.UploadedBy, Purpose: row.Purpose,
			ScopeChannelID: row.ScopeChannelID, UploadState: row.UploadState, ScanStatus: row.ScanStatus, Width: row.Width,
			Height: row.Height, DurationMs: row.DurationMs, TranscriptStatus: row.TranscriptStatus,
			TranscriptLanguage: row.TranscriptLanguage, TranscriptSource: row.TranscriptSource, CreatedAt: row.CreatedAt,
		})
		f.Deleted = row.Deleted
		out = append(out, app.AttachTarget{File: f, SessionState: domain.SessionState(row.SessionState.String)})
		found = append(found, f.ID)
	}
	thumbs, err := thumbnailsOf(ctx, q, found)
	if err != nil {
		return nil, err
	}
	for i := range out {
		out[i].File.Thumbnails = thumbs[out[i].File.ID]
	}
	return out, nil
}

// AttachedFileIDs implements app.Repository.
func (r *Repository) AttachedFileIDs(ctx context.Context, fileIDs []ids.ID) ([]ids.ID, error) {
	rows, err := r.read(ctx).AttachedFileIDs(ctx, uuids(fileIDs))
	if err != nil {
		return nil, mapError(err, "reading the attachments of files")
	}
	out := make([]ids.ID, 0, len(rows))
	for _, u := range rows {
		out = append(out, idOf(u))
	}
	return out, nil
}

// InsertAttachments implements app.Repository: the rows of one message in one statement. Rows of
// several messages, threads or channels are an internal error; the unique violation on file_id is
// app.ErrAlreadyAttached.
func (r *Repository) InsertAttachments(ctx context.Context, rows []app.AttachmentRecord) error {
	q, err := r.write(ctx)
	if err != nil || len(rows) == 0 {
		return err
	}
	first := rows[0]
	arg := sqlcgen.InsertAttachmentsParams{MessageID: uuid(first.MessageID), ThreadID: uuid(first.ThreadID), ChannelID: uuid(first.ChannelID),
		FileIds: make([]pgtype.UUID, 0, len(rows)), Positions: make([]int16, 0, len(rows))}
	for _, row := range rows {
		if row.MessageID != first.MessageID || row.ThreadID != first.ThreadID || row.ChannelID != first.ChannelID {
			return errs.New(errs.CodeInternalError, "files: InsertAttachments with rows of several messages")
		}
		arg.FileIds = append(arg.FileIds, uuid(row.FileID))
		arg.Positions = append(arg.Positions, int16(row.Position)) //nolint:gosec // G115: at most message.max_attachments
	}
	err = q.InsertAttachments(ctx, arg)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == uniqueViolation && pgErr.ConstraintName == fileIDKey {
		return app.ErrAlreadyAttached
	}
	return mapError(err, "inserting attachments")
}

// MessageAttachments implements app.Repository: the rows with their files in one statement, the
// thumbnails of all files in one more.
func (r *Repository) MessageAttachments(ctx context.Context, messageIDs []ids.ID) ([]app.AttachedFile, error) {
	q := r.read(ctx)
	rows, err := q.ListMessageAttachments(ctx, uuids(messageIDs))
	if err != nil {
		return nil, mapError(err, "listing the attachments of messages")
	}
	out := make([]app.AttachedFile, 0, len(rows))
	fileIDs := make([]ids.ID, 0, len(rows))
	for _, row := range rows {
		f := fileRecord(sqlcgen.GetFileRow{
			ID: row.ID, ObjectKey: row.ObjectKey, Sha256: row.Sha256, SizeBytes: row.SizeBytes, MimeType: row.MimeType,
			DeclaredMimeType: row.DeclaredMimeType, OriginalName: row.OriginalName, UploadedBy: row.UploadedBy, Purpose: row.Purpose,
			ScopeChannelID: row.ScopeChannelID, UploadState: row.UploadState, ScanStatus: row.ScanStatus, Width: row.Width,
			Height: row.Height, DurationMs: row.DurationMs, TranscriptStatus: row.TranscriptStatus,
			TranscriptLanguage: row.TranscriptLanguage, TranscriptSource: row.TranscriptSource, CreatedAt: row.CreatedAt,
		})
		f.Deleted = row.Deleted
		out = append(out, app.AttachedFile{File: f, Row: app.AttachmentRecord{MessageID: idOf(row.MessageID), FileID: f.ID,
			ThreadID: idOf(row.ThreadID), ChannelID: idOf(row.ChannelID), Position: int(row.Position)}})
		fileIDs = append(fileIDs, f.ID)
	}
	thumbs, err := thumbnailsOf(ctx, q, fileIDs)
	if err != nil {
		return nil, err
	}
	for i := range out {
		out[i].File.Thumbnails = thumbs[out[i].File.ID]
	}
	return out, nil
}

// AttachmentOf implements app.Repository.
func (r *Repository) AttachmentOf(ctx context.Context, fileID ids.ID) (app.AttachmentRecord, bool, error) {
	row, err := r.read(ctx).GetAttachmentByFile(ctx, uuid(fileID))
	if errors.Is(err, pgx.ErrNoRows) {
		return app.AttachmentRecord{}, false, nil
	}
	if err != nil {
		return app.AttachmentRecord{}, false, mapError(err, "reading the attachment of a file")
	}
	return app.AttachmentRecord{MessageID: idOf(row.MessageID), FileID: idOf(row.FileID), ThreadID: idOf(row.ThreadID),
		ChannelID: idOf(row.ChannelID), Position: int(row.Position)}, true, nil
}

// LockMessageFiles implements app.Repository.
func (r *Repository) LockMessageFiles(ctx context.Context, messageID ids.ID) ([]ids.ID, error) {
	q, err := r.write(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := q.LockMessageAttachmentFiles(ctx, uuid(messageID))
	if err != nil {
		return nil, mapError(err, "locking the files of a message")
	}
	out := make([]ids.ID, 0, len(rows))
	for _, u := range rows {
		out = append(out, idOf(u))
	}
	return out, nil
}

// DeleteMessageAttachments implements app.Repository.
func (r *Repository) DeleteMessageAttachments(ctx context.Context, messageID ids.ID) (int, error) {
	q, err := r.write(ctx)
	if err != nil {
		return 0, err
	}
	n, err := q.DeleteMessageAttachments(ctx, uuid(messageID))
	return int(n), mapError(err, "deleting the attachments of a message")
}

// AttachmentStates implements app.Repository; it reads in the transaction of the unit of work.
func (r *Repository) AttachmentStates(ctx context.Context, messageID ids.ID) ([]domain.StateFacts, error) {
	q, err := r.write(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := q.ListAttachmentStates(ctx, uuid(messageID))
	if err != nil {
		return nil, mapError(err, "reading the attachment states of a message")
	}
	out := make([]domain.StateFacts, 0, len(rows))
	for _, row := range rows {
		out = append(out, domain.StateFacts{UploadState: row.UploadState, ScanStatus: row.ScanStatus, Deleted: row.Deleted})
	}
	return out, nil
}

// MoveThreadAttachments implements app.ThreadAttachments; it needs the transaction of the move.
func (r *Repository) MoveThreadAttachments(ctx context.Context, threadID, channelID ids.ID) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	return mapError(q.MoveThreadAttachments(ctx, sqlcgen.MoveThreadAttachmentsParams{ThreadID: uuid(threadID), ChannelID: uuid(channelID)}),
		"moving the attachments of a thread")
}

// thumbnailsOf reads the thumbnails of fileIDs in one statement, grouped by file; no file costs
// no statement.
func thumbnailsOf(ctx context.Context, q *sqlcgen.Queries, fileIDs []ids.ID) (map[ids.ID][]app.ThumbnailRecord, error) {
	out := make(map[ids.ID][]app.ThumbnailRecord, len(fileIDs))
	if len(fileIDs) == 0 {
		return out, nil
	}
	rows, err := q.ListThumbnailsOfFiles(ctx, uuids(fileIDs))
	if err != nil {
		return nil, mapError(err, "listing the thumbnails of files")
	}
	for _, t := range rows {
		id := idOf(t.FileID)
		out[id] = append(out[id], app.ThumbnailRecord{Variant: domain.Variant(t.Variant), ObjectKey: t.ObjectKey, MIMEType: t.MimeType,
			Width: int(t.Width), Height: int(t.Height), SizeBytes: t.SizeBytes})
	}
	return out, nil
}

// uuids converts ids for an array parameter.
func uuids(list []ids.ID) []pgtype.UUID {
	out := make([]pgtype.UUID, 0, len(list))
	for _, id := range list {
		out = append(out, uuid(id))
	}
	return out
}
