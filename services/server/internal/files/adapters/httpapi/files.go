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
	// The derived transcript ETag (R-DM-48) is the precondition of submitTranscript (M3-09).
	return rest.GetFile200JSONResponse{Body: FileREST(f), Headers: rest.GetFile200ResponseHeaders{ETag: &f.TranscriptETag}}, nil
}

// DeleteFile implements rest.StrictServerInterface (DELETE /files/{file_id}, M3-08): an own
// file that is not attached is deleted; its objects and row go with files.purge_objects. The
// answers of files.Files.Delete map to 204, 404, 403 and 409 through the error chain.
func (h *Files) DeleteFile(ctx context.Context, req rest.DeleteFileRequestObject) (rest.DeleteFileResponseObject, error) {
	if err := h.files.Delete(ctx, ids.ID(req.FileId)); err != nil {
		return nil, err
	}
	return rest.DeleteFile204Response{}, nil
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
