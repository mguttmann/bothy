// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/OWNER/bothy/services/server/internal/files/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/files/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Compile-time check: Repository is the read port of downloads (M3-05).
var _ app.DeliveryReader = (*Repository)(nil)

// FileForDelivery implements app.DeliveryReader: one file by id, deleted or not.
func (r *Repository) FileForDelivery(ctx context.Context, fileID ids.ID) (app.FileRecord, bool, error) {
	row, err := r.read(ctx).GetFileForDelivery(ctx, uuid(fileID))
	if errors.Is(err, pgx.ErrNoRows) {
		return app.FileRecord{}, false, nil
	}
	if err != nil {
		return app.FileRecord{}, false, mapError(err, "reading a file for a delivery")
	}
	return app.FileRecord{
		ID: idOf(row.ID), ObjectKey: row.ObjectKey, SHA256: digest(row.Sha256), SizeBytes: row.SizeBytes, MIMEType: row.MimeType,
		OriginalName: row.OriginalName, UploadedBy: idOf(row.UploadedBy), Purpose: row.Purpose, ScopeChannelID: idOf(row.ScopeChannelID),
		UploadState: row.UploadState, ScanStatus: row.ScanStatus, TranscriptStatus: row.TranscriptStatus,
		TranscriptText: row.TranscriptText.String, TranscriptLanguage: row.TranscriptLanguage.String,
		TranscriptSource: row.TranscriptSource.String, CreatedAt: timeOf(row.CreatedAt), Deleted: row.DeletedAt.Valid,
	}, true, nil
}

// Thumbnail implements app.DeliveryReader: one row of file_thumbnails.
func (r *Repository) Thumbnail(ctx context.Context, fileID ids.ID, variant string) (app.ThumbnailRecord, bool, error) {
	row, err := r.read(ctx).GetThumbnail(ctx, sqlcgen.GetThumbnailParams{FileID: uuid(fileID), Variant: variant})
	if errors.Is(err, pgx.ErrNoRows) {
		return app.ThumbnailRecord{}, false, nil
	}
	if err != nil {
		return app.ThumbnailRecord{}, false, mapError(err, "reading a thumbnail")
	}
	return app.ThumbnailRecord{FileID: idOf(row.FileID), Variant: row.Variant, ObjectKey: row.ObjectKey, MIMEType: row.MimeType,
		Width: int(row.Width), Height: int(row.Height), SizeBytes: row.SizeBytes}, true, nil
}
