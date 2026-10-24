// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/OWNER/bothy/services/server/internal/files/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/files/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// TranscriptFile implements app.Repository: one file by id with its transcript, deleted or not,
// in the ambient transaction when there is one and on the pool otherwise (M3-09).
func (r *Repository) TranscriptFile(ctx context.Context, fileID ids.ID) (app.FileRecord, bool, error) {
	row, err := r.read(ctx).GetTranscriptFile(ctx, uuid(fileID))
	return transcriptRecord(row, err)
}

// LockTranscriptFile implements app.Repository: TranscriptFile with the files row FOR UPDATE.
func (r *Repository) LockTranscriptFile(ctx context.Context, fileID ids.ID) (app.FileRecord, bool, error) {
	q, err := r.write(ctx)
	if err != nil {
		return app.FileRecord{}, false, err
	}
	row, err := q.LockTranscriptFile(ctx, uuid(fileID))
	return transcriptRecord(sqlcgen.GetTranscriptFileRow(row), err)
}

// LockVoiceFile implements app.Repository: the file of requestTranscription or submitTranscript
// with the files row FOR UPDATE, after authorize.
func (r *Repository) LockVoiceFile(ctx context.Context, fileID ids.ID) (app.FileRecord, bool, error) {
	q, err := r.write(ctx)
	if err != nil {
		return app.FileRecord{}, false, err
	}
	row, err := q.LockVoiceFile(ctx, uuid(fileID))
	return transcriptRecord(sqlcgen.GetTranscriptFileRow(row), err)
}

// SetTranscript implements app.Repository: the four transcript columns; "" writes NULL for text,
// language and source.
func (r *Repository) SetTranscript(ctx context.Context, fileID ids.ID, t app.TranscriptWrite) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	_, err = q.SetTranscript(ctx, sqlcgen.SetTranscriptParams{
		TranscriptStatus: t.Status, TranscriptText: optionalText(t.Text, t.Status == transcriptDone),
		TranscriptLanguage: optionalText(t.Language, t.Language != ""), TranscriptSource: optionalText(t.Source, t.Source != ""),
		ID: uuid(fileID),
	})
	return mapError(err, "writing a transcript")
}

// transcriptDone is the status whose text is stored, also when it is empty (a silent recording).
const transcriptDone = "done"

// optionalText converts a column value: set reports a non-NULL value.
func optionalText(s string, set bool) pgtype.Text {
	return pgtype.Text{String: s, Valid: set}
}

// transcriptRecord converts a transcript read; no row is found = false.
func transcriptRecord(row sqlcgen.GetTranscriptFileRow, err error) (app.FileRecord, bool, error) {
	if errors.Is(err, pgx.ErrNoRows) {
		return app.FileRecord{}, false, nil
	}
	if err != nil {
		return app.FileRecord{}, false, mapError(err, "reading a transcript file")
	}
	f := fileRecord(sqlcgen.GetFileRow{
		ID: row.ID, ObjectKey: row.ObjectKey, Sha256: row.Sha256, SizeBytes: row.SizeBytes, MimeType: row.MimeType,
		DeclaredMimeType: row.DeclaredMimeType, OriginalName: row.OriginalName, UploadedBy: row.UploadedBy, Purpose: row.Purpose,
		ScopeChannelID: row.ScopeChannelID, UploadState: row.UploadState, ScanStatus: row.ScanStatus, Width: row.Width,
		Height: row.Height, DurationMs: row.DurationMs, TranscriptStatus: row.TranscriptStatus,
		TranscriptLanguage: row.TranscriptLanguage, TranscriptSource: row.TranscriptSource, CreatedAt: row.CreatedAt,
		TranscriptText: row.TranscriptText,
	})
	f.Deleted = row.Deleted
	return f, true, nil
}
