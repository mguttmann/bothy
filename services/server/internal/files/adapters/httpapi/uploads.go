// SPDX-License-Identifier: Apache-2.0

// Package httpapi holds the REST handlers of the files module as part of
// rest.StrictServerInterface: createUpload, uploadPart, getUpload, completeUpload and abortUpload
// (uploads.go) and getFile (files.go) of docs/api/openapi.yaml (M3-03). The use cases authorize;
// the handlers only convert and pass errors to the problem writer of the chain unchanged.
//
// Example:
//
//	h := httpapi.New(svc, svc)
//	built, err := routes.Build(wire.Handlers{Files: h}, wire.RouteOptions(), cat)
package httpapi

import (
	"context"
	"encoding/hex"
	"strconv"
	"time"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/files"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Files implements the REST operations of the files module.
type Files struct {
	uploads files.Uploads
	files   files.Files
}

// New returns the handlers over the upload and file ports.
func New(u files.Uploads, f files.Files) *Files {
	return &Files{uploads: u, files: f}
}

// partURLPrefix is the path of the parts of an upload in mode direct (12 3.2).
const partURLPrefix = "/api/v1/uploads/"

// CreateUpload implements rest.StrictServerInterface (POST /channels/{channel_id}/uploads): 201
// with the plan of the session in mode direct.
func (h *Files) CreateUpload(ctx context.Context, req rest.CreateUploadRequestObject) (rest.CreateUploadResponseObject, error) {
	b := req.Body
	var sum [32]byte
	if n, err := hex.Decode(sum[:], []byte(b.Sha256)); err != nil || n != len(sum) {
		// The schema pattern refuses it first; this keeps the handler safe on its own.
		return nil, errs.New(errs.CodeValidationFailed, "sha256 is not a lowercase hex SHA-256").
			WithFields(errs.FieldError{Pointer: "/sha256", Code: "pattern", Detail: "sha256 is not a lowercase hex SHA-256"})
	}
	cmd := files.CreateUpload{ChannelID: ids.ID(req.ChannelId), Name: b.Name, SizeBytes: b.SizeBytes, SHA256: sum}
	if b.DeclaredMimeType != nil {
		cmd.DeclaredMIMEType = *b.DeclaredMimeType
	}
	s, err := h.uploads.Create(ctx, cmd)
	if err != nil {
		return nil, err
	}
	parts := make([]rest.UploadPart, 0, s.PartCount)
	for n := 1; n <= s.PartCount; n++ {
		parts = append(parts, rest.UploadPart{
			PartNumber: n, Url: partURLPrefix + s.FileID.String() + "/parts/" + strconv.Itoa(n),
			Headers: []rest.HeaderPair{{Name: "Content-Type", Value: "application/octet-stream"}},
		})
	}
	return rest.CreateUpload201JSONResponse{Body: rest.UploadSession{
		FileId: rest.Uuid(s.FileID), Mode: rest.UploadSessionMode(s.Mode), PartSizeBytes: s.PartSizeBytes, Parts: parts,
		ExpiresAt: s.ExpiresAt.UTC().Truncate(time.Millisecond),
	}}, nil
}

// UploadPart implements rest.StrictServerInterface (PUT /uploads/{file_id}/parts/{part_number}):
// 200 with the part and its ETag. The Content-Length of the request is not visible to a strict
// handler, so the length is checked while reading (docs/DECISIONS.md D-M3-03-22).
func (h *Files) UploadPart(ctx context.Context, req rest.UploadPartRequestObject) (rest.UploadPartResponseObject, error) {
	ref, err := h.uploads.PutPart(ctx, files.PutPart{FileID: ids.ID(req.FileId), PartNumber: req.PartNumber,
		ContentDigest: req.Params.ContentDigest, ContentLength: -1, Body: req.Body})
	if err != nil {
		return nil, err
	}
	return rest.UploadPart200JSONResponse{Body: rest.UploadPartResult{PartNumber: ref.PartNumber, Etag: ref.ETag}}, nil
}

// GetUpload implements rest.StrictServerInterface (GET /uploads/{file_id}): the state of an own
// upload with its confirmed parts.
func (h *Files) GetUpload(ctx context.Context, req rest.GetUploadRequestObject) (rest.GetUploadResponseObject, error) {
	st, err := h.uploads.State(ctx, ids.ID(req.FileId))
	if err != nil {
		return nil, err
	}
	out := rest.UploadState{FileId: rest.Uuid(st.FileID), UploadState: rest.UploadStateUploadState(st.View),
		PartSizeBytes: st.PartSizeBytes, ExpiresAt: st.ExpiresAt.UTC().Truncate(time.Millisecond)}
	out.CompletedParts = make([]struct {
		Etag       string `json:"etag"`
		PartNumber int    `json:"part_number"`
	}, 0, len(st.Confirmed))
	for _, p := range st.Confirmed {
		out.CompletedParts = append(out.CompletedParts, struct {
			Etag       string `json:"etag"`
			PartNumber int    `json:"part_number"`
		}{Etag: p.ETag, PartNumber: p.PartNumber})
	}
	return rest.GetUpload200JSONResponse{Body: out}, nil
}

// CompleteUpload implements rest.StrictServerInterface (POST /uploads/{file_id}/complete): 200
// with the file, which stays pending until the verify pass.
func (h *Files) CompleteUpload(ctx context.Context, req rest.CompleteUploadRequestObject) (rest.CompleteUploadResponseObject, error) {
	parts := make([]files.PartRef, 0, len(req.Body.Parts))
	for _, p := range req.Body.Parts {
		parts = append(parts, files.PartRef{PartNumber: p.PartNumber, ETag: p.Etag})
	}
	f, err := h.uploads.Complete(ctx, files.CompleteUpload{FileID: ids.ID(req.FileId), Parts: parts})
	if err != nil {
		return nil, err
	}
	return rest.CompleteUpload200JSONResponse{Body: FileREST(f)}, nil
}

// AbortUpload implements rest.StrictServerInterface (DELETE /uploads/{file_id}): 204.
func (h *Files) AbortUpload(ctx context.Context, req rest.AbortUploadRequestObject) (rest.AbortUploadResponseObject, error) {
	if err := h.uploads.Abort(ctx, ids.ID(req.FileId)); err != nil {
		return nil, err
	}
	return rest.AbortUpload204Response{}, nil
}
