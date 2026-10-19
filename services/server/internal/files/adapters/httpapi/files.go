// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/files"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// GetFile implements rest.StrictServerInterface (GET /files/{file_id}): the metadata of a file
// the caller may read, in every state.
func (h *Files) GetFile(ctx context.Context, req rest.GetFileRequestObject) (rest.GetFileResponseObject, error) {
	f, err := h.files.Get(ctx, ids.ID(req.FileId))
	if err != nil {
		return nil, err
	}
	return rest.GetFile200JSONResponse{Body: FileREST(f)}, nil
}

// FileREST converts a file into the schema File: zero values are absent members, never JSON null
// (ADR 0053), and thumbnails is an empty list until M3-06.
func FileREST(f files.File) rest.File {
	out := rest.File{
		Id: rest.Uuid(f.ID), SizeBytes: f.SizeBytes, MimeType: f.MIMEType, OriginalName: f.OriginalName,
		UploadedBy: rest.Uuid(f.UploadedBy), Purpose: rest.FilePurpose(f.Purpose), UploadState: rest.FileUploadState(f.UploadState),
		ScanStatus: rest.FileScanStatus(f.ScanStatus), TranscriptStatus: rest.TranscriptStatus(f.TranscriptStatus),
		Thumbnails: make([]rest.Thumbnail, 0, len(f.Thumbnails)), CreatedAt: f.CreatedAt.UTC().Truncate(time.Millisecond),
	}
	if f.SHA256 != "" {
		v := f.SHA256
		out.Sha256 = &v
	}
	if !f.ScopeChannelID.IsZero() {
		v := rest.Uuid(f.ScopeChannelID)
		out.ScopeChannelId = &v
	}
	if f.Width != 0 {
		v := f.Width
		out.Width = &v
	}
	if f.Height != 0 {
		v := f.Height
		out.Height = &v
	}
	if f.DurationMS != 0 {
		v := f.DurationMS
		out.DurationMs = &v
	}
	if f.TranscriptLanguage != "" {
		v := f.TranscriptLanguage
		out.TranscriptLanguage = &v
	}
	if f.TranscriptSource != "" {
		v := rest.TranscriptSource(f.TranscriptSource)
		out.TranscriptSource = &v
	}
	for _, t := range f.Thumbnails {
		out.Thumbnails = append(out.Thumbnails, rest.Thumbnail{Variant: rest.ThumbnailVariant(t.Variant), MimeType: t.MIMEType,
			Width: t.Width, Height: t.Height})
	}
	return out
}
