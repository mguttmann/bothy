// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"context"
	"strconv"
	"time"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/importer"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// slotREST converts a file slot (schema ImportFileSlot): the parts of the session name
// uploadImportFilePart of the source id (M8-11 design note 6 b2).
func slotREST(id ids.ID, s importer.FileSlot) rest.ImportFileSlot {
	out := rest.ImportFileSlot{Item: itemREST(s.Item)}
	if s.Upload == nil {
		return out
	}
	u := s.Upload
	prefix := "/api/v1/imports/" + id.String() + "/files/" + u.FileID.String() + "/parts/"
	parts := make([]rest.UploadPart, 0, u.PartCount)
	for n := 1; n <= u.PartCount; n++ {
		parts = append(parts, rest.UploadPart{
			PartNumber: n, Url: prefix + strconv.Itoa(n), Headers: []rest.HeaderPair{{Name: "Content-Type", Value: "application/octet-stream"}},
		})
	}
	out.Upload = &rest.UploadSession{FileId: rest.Uuid(u.FileID), Mode: rest.UploadSessionMode(u.Mode), PartSizeBytes: u.PartSizeBytes,
		Parts: parts, ExpiresAt: u.ExpiresAt.UTC().Truncate(time.Millisecond)}
	return out
}

// CreateImportFile implements rest.StrictServerInterface (POST /imports/{import_id}/files).
func (h *Imports) CreateImportFile(ctx context.Context, req rest.CreateImportFileRequestObject) (rest.CreateImportFileResponseObject, error) {
	b := req.Body
	cmd := importer.FileCmd{OriginKey: b.OriginKey, ChannelID: ids.ID(b.ChannelId), AuthorID: ids.ID(b.AuthorId), Name: b.Name,
		SizeBytes: b.SizeBytes, SHA256: b.Sha256}
	if b.DeclaredMimeType != nil {
		cmd.DeclaredMIMEType = *b.DeclaredMimeType
	}
	s, err := h.imports.CreateFile(ctx, principal(ctx), ids.ID(req.ImportId), cmd)
	if err != nil {
		return nil, err
	}
	return rest.CreateImportFile200JSONResponse{Body: slotREST(ids.ID(req.ImportId), s)}, nil
}

// UploadImportFilePart implements rest.StrictServerInterface (PUT
// /imports/{import_id}/files/{file_id}/parts/{part_number}); the length is checked while reading,
// as uploadPart does.
func (h *Imports) UploadImportFilePart(ctx context.Context, req rest.UploadImportFilePartRequestObject) (rest.UploadImportFilePartResponseObject, error) {
	ref, err := h.imports.UploadFilePart(ctx, principal(ctx), ids.ID(req.ImportId), importer.FilePartCmd{FileID: ids.ID(req.FileId),
		PartNumber: req.PartNumber, ContentDigest: req.Params.ContentDigest, Body: req.Body})
	if err != nil {
		return nil, err
	}
	return rest.UploadImportFilePart200JSONResponse{Body: rest.UploadPartResult{PartNumber: ref.PartNumber, Etag: ref.ETag}}, nil
}

// CompleteImportFile implements rest.StrictServerInterface (POST
// /imports/{import_id}/files/{file_id}/complete): 200 with the posted item.
func (h *Imports) CompleteImportFile(ctx context.Context, req rest.CompleteImportFileRequestObject) (rest.CompleteImportFileResponseObject, error) {
	parts := make([]importer.PartRef, 0, len(req.Body.Parts))
	for _, p := range req.Body.Parts {
		parts = append(parts, importer.PartRef{PartNumber: p.PartNumber, ETag: p.Etag})
	}
	s, err := h.imports.CompleteFile(ctx, principal(ctx), ids.ID(req.ImportId), importer.FileCompleteCmd{FileID: ids.ID(req.FileId), Parts: parts})
	if err != nil {
		return nil, err
	}
	return rest.CompleteImportFile200JSONResponse{Body: slotREST(ids.ID(req.ImportId), s)}, nil
}
