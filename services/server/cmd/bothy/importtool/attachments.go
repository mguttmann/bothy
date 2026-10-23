// SPDX-License-Identifier: Apache-2.0

package importtool

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/OWNER/bothy/services/server/api/restclient"
	"github.com/OWNER/bothy/services/server/cmd/bothy/importtool/discord"
)

// The reasons of an attachment line (22 6.5 item 6).
const (
	reasonTooLarge = "too large"
	reasonFailed   = "failed"
)

// fetcher loads one attachment from the CDN; *discord.Fetcher satisfies it.
type fetcher interface {
	Fetch(ctx context.Context, ref discord.AttachmentRef) (*discord.Download, error)
}

// uploader takes attachments over (22 6.5): the CDN fetch of M8-02 into memory, bounded by the
// effective import.attachment_max_bytes plus one byte and never written to disk, then
// createImportFile with the SHA-256, the parts of the upload session and completeImportFile.
type uploader struct {
	api    bothyAPI
	src    uuid.UUID
	fetch  fetcher
	max    int64
	posted map[string]uuid.UUID // file origin keys posted by an earlier run, with their file
}

// attachmentResult is the outcome of one attachment: a posted file, or the reason of its line.
type attachmentResult struct {
	file   uuid.UUID
	reason string
}

// abortsRun reports a fetch error that ends the run (exit 1, 22 10.3) instead of giving the
// attachment a line: a rate limit above the limits or a cancelled run.
func abortsRun(err error) bool {
	return errors.Is(err, discord.ErrRateWaitTooLong) || errors.Is(err, discord.ErrRateLimited) ||
		errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

// take takes one attachment of message msg (read from the channel or thread readID) over into the
// channel target with the author author. A posted file of an earlier run is reused without a
// fetch. Errors end the run; an attachment that cannot be taken over returns its reason.
func (u *uploader) take(ctx context.Context, readID string, msg discord.Message, a discord.Attachment, channel, author uuid.UUID) (attachmentResult, error) {
	key := keyFile + a.ID
	if id, ok := u.posted[key]; ok {
		return attachmentResult{file: id}, nil
	}
	if a.Size > u.max {
		return attachmentResult{reason: reasonTooLarge}, nil
	}
	data, contentType, err := u.download(ctx, readID, msg.ID, a)
	switch {
	case errors.Is(err, discord.ErrTooLarge):
		return attachmentResult{reason: reasonTooLarge}, nil
	case err != nil && abortsRun(err):
		return attachmentResult{}, redactFetch(err)
	case err != nil || len(data) == 0:
		return attachmentResult{reason: reasonFailed}, nil
	}
	id, err := u.store(ctx, key, a, data, contentType, channel, author)
	if err != nil {
		var ae *APIError
		if errors.As(err, &ae) && ae.Status >= 400 && ae.Status < 500 && ae.Status != http.StatusTooManyRequests && ae.Reason != reasonNotRunning {
			// The server refused this file (its type, its size, a scan rule): the message keeps
			// the line, the run goes on.
			return attachmentResult{reason: reasonFailed}, nil
		}
		return attachmentResult{}, err
	}
	u.posted[key] = id
	return attachmentResult{file: id}, nil
}

// download fetches the bytes of an attachment into memory.
func (u *uploader) download(ctx context.Context, readID, messageID string, a discord.Attachment) ([]byte, string, error) {
	d, err := u.fetch.Fetch(ctx, discord.AttachmentRef{ChannelID: readID, MessageID: messageID, AttachmentID: a.ID, URL: a.URL, Size: a.Size})
	if err != nil {
		return nil, "", err
	}
	defer func() { _ = d.Body.Close() }()
	data, err := io.ReadAll(io.LimitReader(d.Body, u.max+1))
	if err != nil {
		return nil, "", err
	}
	if int64(len(data)) > u.max {
		return nil, "", discord.ErrTooLarge
	}
	ct := a.ContentType
	if ct == "" {
		ct = d.ContentType
	}
	return data, ct, nil
}

// store creates the file item and uploads its parts (M8-11 note 6 b2); a slot that is already
// posted is answered without an upload.
func (u *uploader) store(ctx context.Context, key string, a discord.Attachment, data []byte, contentType string, channel, author uuid.UUID) (uuid.UUID, error) {
	sum := sha256.Sum256(data)
	body := restclient.CreateImportFileJSONRequestBody{OriginKey: key, ChannelId: channel, AuthorId: author, Name: a.Filename,
		SizeBytes: int64(len(data)), Sha256: hex.EncodeToString(sum[:])}
	if ct := strings.TrimSpace(contentType); ct != "" {
		body.DeclaredMimeType = &ct
	}
	resp, err := u.api.CreateImportFileWithResponse(ctx, u.src, body)
	if err != nil {
		return uuid.UUID{}, err
	}
	if resp.JSON200 == nil {
		return uuid.UUID{}, apiError("createImportFile", resp.HTTPResponse, resp.Body)
	}
	slot := resp.JSON200
	if slot.Upload == nil {
		if slot.Item.TargetId == nil {
			return uuid.UUID{}, &APIError{Op: "createImportFile", Status: http.StatusOK, Code: "no_target"}
		}
		return *slot.Item.TargetId, nil
	}
	return u.upload(ctx, slot.Upload, data)
}

// upload sends every part of the session with its Content-Digest and completes the file.
func (u *uploader) upload(ctx context.Context, s *restclient.UploadSession, data []byte) (uuid.UUID, error) {
	parts := make([]restclient.UploadPartResult, 0, len(s.Parts))
	size := s.PartSizeBytes
	if size <= 0 {
		size = int64(len(data))
	}
	for _, p := range s.Parts {
		from := int64(p.PartNumber-1) * size
		to := min(from+size, int64(len(data)))
		if from < 0 || from >= to {
			return uuid.UUID{}, &APIError{Op: "uploadImportFilePart", Status: http.StatusBadRequest, Code: "part_range"}
		}
		chunk := data[from:to]
		sum := sha256.Sum256(chunk)
		digest := "sha-256=:" + base64.StdEncoding.EncodeToString(sum[:]) + ":"
		extra := partHeaders(p.Headers)
		resp, err := u.api.UploadImportFilePartWithBodyWithResponse(ctx, u.src, s.FileId, p.PartNumber,
			&restclient.UploadImportFilePartParams{ContentDigest: digest}, "application/octet-stream", bytes.NewReader(chunk), extra)
		if err != nil {
			return uuid.UUID{}, err
		}
		if resp.JSON200 == nil {
			return uuid.UUID{}, apiError("uploadImportFilePart", resp.HTTPResponse, resp.Body)
		}
		parts = append(parts, restclient.UploadPartResult{Etag: resp.JSON200.Etag, PartNumber: p.PartNumber})
	}
	resp, err := u.api.CompleteImportFileWithResponse(ctx, u.src, s.FileId, restclient.CompleteImportFileJSONRequestBody{Parts: parts})
	if err != nil {
		return uuid.UUID{}, err
	}
	if resp.JSON200 == nil {
		return uuid.UUID{}, apiError("completeImportFile", resp.HTTPResponse, resp.Body)
	}
	return s.FileId, nil
}

// partHeaders sets the headers the upload session prescribes for a part, except those the
// generated client sets itself.
func partHeaders(hs []restclient.HeaderPair) restclient.RequestEditorFn {
	return func(_ context.Context, req *http.Request) error {
		for _, h := range hs {
			switch http.CanonicalHeaderKey(h.Name) {
			case "Content-Digest", "Content-Type", "Content-Length", "Authorization", "Dpop", "Host":
				continue
			}
			req.Header.Set(h.Name, h.Value)
		}
		return nil
	}
}
