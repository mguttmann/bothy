// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/OWNER/bothy/services/server/internal/files/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/files/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Compile-time check: Repository is the store of the personal data participant and of the export
// packages (M7-06).
var _ app.PersonalDataStore = (*Repository)(nil)

// PersonalFiles implements app.PersonalDataStore.
func (r *Repository) PersonalFiles(ctx context.Context, uploader ids.ID) ([]app.PersonalFile, error) {
	rows, err := r.read(ctx).ListPersonalFiles(ctx, uuid(uploader))
	if err != nil {
		return nil, mapError(err, "listing the files of a data subject")
	}
	out := make([]app.PersonalFile, 0, len(rows))
	for _, f := range rows {
		out = append(out, app.PersonalFile{
			ID: idOf(f.ID), ObjectKey: f.ObjectKey, Name: f.OriginalName, MIMEType: f.MimeType, Purpose: f.Purpose,
			CreatedAt: timeOf(f.CreatedAt), ChannelID: idOf(f.ChannelID), TranscriptStatus: f.TranscriptStatus,
			TranscriptText: f.TranscriptText.String, TranscriptLanguage: f.TranscriptLanguage.String,
		})
	}
	return out, nil
}

// OpenUploadsOf implements app.PersonalDataStore.
func (r *Repository) OpenUploadsOf(ctx context.Context, uploader ids.ID) ([]ids.ID, error) {
	rows, err := r.read(ctx).ListOpenUploadsOfUploader(ctx, uuid(uploader))
	return idList(rows), mapError(err, "listing the open uploads of a data subject")
}

// UnattachedFilesOf implements app.PersonalDataStore.
func (r *Repository) UnattachedFilesOf(ctx context.Context, uploader ids.ID, purpose string) ([]ids.ID, error) {
	rows, err := r.read(ctx).ListUnattachedFilesOfUploader(ctx, sqlcgen.ListUnattachedFilesOfUploaderParams{UploadedBy: uuid(uploader), Purpose: purpose})
	return idList(rows), mapError(err, "listing the unattached files of a data subject")
}

// InsertExportFile implements app.PersonalDataStore.
func (r *Repository) InsertExportFile(ctx context.Context, f app.FileRecord) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	return mapError(q.InsertFile(ctx, sqlcgen.InsertFileParams{
		ID: uuid(f.ID), ObjectKey: f.ObjectKey, Sha256: f.SHA256[:], SizeBytes: f.SizeBytes, MimeType: f.MIMEType,
		DeclaredMimeType: text(f.DeclaredMIMEType), OriginalName: f.OriginalName, UploadedBy: uuid(f.UploadedBy), Purpose: f.Purpose,
		ScopeChannelID: optionalUUID(f.ScopeChannelID), UploadState: f.UploadState, ScanStatus: f.ScanStatus,
		TranscriptStatus: f.TranscriptStatus, CreatedAt: ts(f.CreatedAt),
	}), "inserting an export package")
}

// idList converts id columns.
func idList(rows []pgtype.UUID) []ids.ID {
	out := make([]ids.ID, 0, len(rows))
	for _, u := range rows {
		out = append(out, idOf(u))
	}
	return out
}
