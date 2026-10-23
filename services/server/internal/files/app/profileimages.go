// SPDX-License-Identifier: Apache-2.0

package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"mime"
	"slices"
	"strings"
	"time"

	"github.com/OWNER/bothy/services/server/internal/files/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// The use cases of avatar and logo files (brief M3-10, docs/spec/12-files.md 3.7): the module
// identity (setMyAvatar) and the module instance (uploadBrandingLogo) authorize their own action
// and reach these use cases through their source ports AvatarFiles and LogoFiles, which
// adapters/profileimages implements by plain delegation. No use case here calls authorize: the
// caller authorized, as every caller of a port does.

// ImageUpload is the input of StageImage: the image part of a multipart request.
type ImageUpload struct {
	// Purpose is avatar or branding.
	Purpose domain.Purpose
	// UploadedBy is the principal that uploads, files.uploaded_by.
	UploadedBy ids.ID
	// DeclaredType is the Content-Type of the part "file".
	DeclaredType string
	// Body is the part; it ends with io.EOF only when no further part follows.
	Body io.Reader
}

// ImageVariant is an opened variant of an image file.
type ImageVariant struct {
	// Body streams the variant; the caller closes it.
	Body io.ReadCloser
	// MIMEType is file_thumbnails.mime_type.
	MIMEType string
	// SizeBytes is file_thumbnails.size_bytes.
	SizeBytes int64
}

// pointerFile is the pointer of the part "file" in a field error.
const pointerFile = "/file"

// codeRequired is the field code of a missing or empty part "file".
const codeRequired = "required"

// StageImage is design note 2 of the brief M3-10: it checks the declared type, reads the part into
// memory up to domain.ImageCap plus one byte while counting and hashing, checks the open sessions
// and the free space (no quota, docs/decisions.d/M3-10.md D-M3-10-4), then inserts the file and its upload
// session of one part with the counted size and digest and stores the part. It returns the file
// id; the session stays open until CompleteStagedImage. It runs outside any unit of work, because
// it calls the object store (02 8.5). No type detection and no image parser run here (SEC-UP-5).
//
// Errors: 415 unsupported_media_type (with reason file_type_denied for a type of
// file.denied_mime_types), 413 payload_too_large naming the key of the cap, 400 validation_failed
// at /file for an empty part, the 400 of a further part from the body, 429 with Retry-After for
// file.upload_max_open_sessions, 503 storage_full, 503 for a storage failure. Nothing is stored on
// any of them but the last, which leaves an open session to expire.
//
// Example:
//
//	id, err := svc.StageImage(ctx, app.ImageUpload{Purpose: domain.PurposeAvatar, UploadedBy: alex, DeclaredType: "image/png", Body: part})
func (s *Service) StageImage(ctx context.Context, in ImageUpload) (ids.ID, error) {
	if txn.InUnit(ctx) {
		return ids.ID{}, errs.New(errs.CodeInternalError, "files: StageImage runs inside a unit of work")
	}
	declared, err := s.checkImageType(in.Purpose, in.DeclaredType)
	if err != nil {
		return ids.ID{}, err
	}
	body, err := s.readImage(in.Body)
	if err != nil {
		return ids.ID{}, err
	}
	rec, err := s.insertImage(ctx, in, declared, body)
	if err != nil {
		return ids.ID{}, err
	}
	sum := sha256.Sum256(body)
	part := PartRecord{FileID: rec.FileID, PartNumber: 1, SizeBytes: int64(len(body)), SHA256: sum}
	etag, err := s.d.Parts.Put(ctx, rec, 1, bytes.NewReader(body), int64(len(body)))
	if err != nil {
		return ids.ID{}, imageStorageError(err)
	}
	part.ETag, part.ReceivedAt = etag, s.now()
	if _, err := s.confirmPart(ctx, actor.Actor{ID: in.UploadedBy}, rec, part); err != nil {
		return ids.ID{}, err
	}
	return rec.FileID, nil
}

// checkImageType is step b of design note 2: a declared type of file.denied_mime_types is 415 with
// reason file_type_denied (12 6.1, first step), any type outside ImageUpload 415 without reason.
// It returns the media type to store in declared_mime_type.
func (s *Service) checkImageType(p domain.Purpose, declared string) (string, error) {
	mt, _, err := mime.ParseMediaType(declared)
	mt = strings.ToLower(mt)
	if err == nil {
		for _, denied := range s.cfg.DeniedMIMETypes {
			if strings.EqualFold(strings.TrimSpace(denied), mt) {
				return "", errs.New(errs.CodeUnsupportedMediaType, "the file type is denied by file.denied_mime_types").WithReason(reasonFileTypeDenied)
			}
		}
	}
	if err != nil || !domain.DeclaredImageTypeAllowed(p, mt) {
		return "", errs.New(errs.CodeUnsupportedMediaType, "an image must be image/jpeg, image/png or image/webp")
	}
	return mt, nil
}

// readImage is step c and d of design note 2: at most the cap plus one byte, so a larger part is
// recognised without reading it; reading to the end of a smaller part also checks that no further
// part follows (the body reports it).
func (s *Service) readImage(r io.Reader) ([]byte, error) {
	limit := domain.ImageCap(s.cfg.MaxUploadBytes, s.cfg.PartBytes)
	if r == nil {
		return nil, fieldError(pointerFile, codeRequired, "the part file is empty")
	}
	b, err := io.ReadAll(io.LimitReader(r, limit+1))
	switch {
	case int64(len(b)) > limit:
		return nil, errs.New(errs.CodePayloadTooLarge, "the image is above "+domain.ImageCapSetting(s.cfg.MaxUploadBytes, s.cfg.PartBytes))
	case err != nil:
		if _, ok := errs.As(err); ok {
			return nil, err
		}
		// A body that breaks off is the client's error, never a server error.
		return nil, errs.Wrap(errs.CodeValidationFailed, err, "the part file could not be read").
			WithFields(errs.FieldError{Pointer: pointerFile, Code: codeInvalid, Detail: "the part file could not be read"})
	case len(b) == 0:
		return nil, fieldError(pointerFile, codeRequired, "the part file is empty")
	}
	return b, nil
}

// insertImage is steps e and f of design note 2 up to the rows: in one unit of work under the
// advisory lock of the principal the open sessions and the free space, then the file and its
// session in mode direct with the counted size and digest as the declared values.
func (s *Service) insertImage(ctx context.Context, in ImageUpload, declared string, body []byte) (SessionRecord, error) {
	now := s.now()
	id := ids.New()
	key, err := domain.OriginalKey(id, 1)
	if err != nil {
		return SessionRecord{}, err
	}
	size := int64(len(body))
	sum := sha256.Sum256(body)
	up := NewUpload{
		File: FileRecord{
			ID: id, ObjectKey: key, SHA256: sum, SizeBytes: size, MIMEType: octetStream, DeclaredMIMEType: declared,
			OriginalName: domain.FixedOriginalName(in.Purpose), UploadedBy: in.UploadedBy, Purpose: string(in.Purpose),
			UploadState: stateOpenFile, ScanStatus: stateOpenFile, TranscriptStatus: transcriptNone, CreatedAt: now,
		},
		Session: SessionRecord{
			FileID: id, Mode: modeDirect, State: domain.SessionOpen, DeclaredSizeBytes: size, DeclaredSHA256: sum,
			PartSizeBytes: s.cfg.PartBytes, PartCount: 1, CreatedAt: now, ExpiresAt: now.Add(s.cfg.SessionTTL),
		},
	}
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		if err := s.checkSessionsAndSpace(ctx, in.UploadedBy, size, now); err != nil {
			return err
		}
		return s.d.Repo.InsertUpload(ctx, up)
	})
	if err != nil {
		return SessionRecord{}, err
	}
	rec := up.Session
	rec.File = up.File
	return rec, nil
}

// imageStorageError keeps a kernel/errs error and answers any other failure of the part store with 503.
func imageStorageError(err error) error {
	if _, ok := errs.As(err); ok {
		return err
	}
	return errs.Wrap(errs.CodeUnavailable, err, "files: the image could not be stored").WithCause(errs.CauseDependencyUnavailable)
}

// errNotStaged is the internal error of a CompleteStagedImage whose file is not an open staged
// session of the purpose and principal: the callers pass the id StageImage returned, so this is a
// programming error, not a client error.
var errNotStaged = errors.New("files: the file is not an open staged image of the purpose and principal")

// CompleteStagedImage is the link step of design note 3 inside the caller's unit of work: the
// session of fileID FOR UPDATE, it must be open with its part confirmed and the file of purpose p
// uploaded by uploadedBy; then the session completed and the job files.scan (M3-04), as
// completeUpload does. Errors: an internal error otherwise or outside a unit of work.
func (s *Service) CompleteStagedImage(ctx context.Context, fileID, uploadedBy ids.ID, p domain.Purpose) error {
	if !txn.InUnit(ctx) {
		return errs.New(errs.CodeInternalError, "files: CompleteStagedImage needs a unit of work")
	}
	rec, found, err := s.d.Repo.LockSession(ctx, fileID, uploadedBy, true)
	if err != nil {
		return err
	}
	if !found || rec.State != domain.SessionOpen || rec.File.Purpose != string(p) || !p.Image() {
		return errs.Wrap(errs.CodeInternalError, errNotStaged, "")
	}
	if err := s.checkParts(ctx, rec, nil); err != nil {
		return errs.Wrap(errs.CodeInternalError, err, "files: the staged image has no confirmed part")
	}
	if err := s.d.Repo.CompleteSession(ctx, fileID, s.now()); err != nil {
		return err
	}
	if err := s.d.ScanQueue.EnqueueScan(ctx, fileID, 0, time.Time{}); err != nil {
		return err
	}
	size := rec.DeclaredSizeBytes
	if !txn.AfterCommit(ctx, func() { s.d.Metrics.UploadCompleted(ctx, size) }) {
		return errs.New(errs.CodeInternalError, "files: CompleteStagedImage outside a unit of work")
	}
	return nil
}

// ReleaseImage is the release step of design note 3 inside the caller's unit of work: the deleted
// mark of fileID when it is a file of purpose p (and of uploadedBy unless it is zero) that is not
// deleted yet; anything else is left alone. The objects stay until files.purge_objects (M3-08).
func (s *Service) ReleaseImage(ctx context.Context, fileID, uploadedBy ids.ID, p domain.Purpose) error {
	if !txn.InUnit(ctx) {
		return errs.New(errs.CodeInternalError, "files: ReleaseImage needs a unit of work")
	}
	if !p.Image() {
		return errs.New(errs.CodeInternalError, "files: ReleaseImage of a purpose without image operations")
	}
	_, err := s.d.Repo.ReleaseFile(ctx, fileID, string(p), uploadedBy, s.now())
	return err
}

// OpenImageVariant opens variant v of the deliverable file fileID of purpose p (design note 8):
// a file that is unknown, deleted, of another purpose, not deliverable (pending, failed, blocked)
// or without the variant is 404 not_found, the same for every case. It runs outside a unit of
// work. Errors: not_found, 503 for a storage failure.
func (s *Service) OpenImageVariant(ctx context.Context, fileID ids.ID, p domain.Purpose, v domain.Variant) (ImageVariant, error) {
	if s.d.Store == nil {
		return ImageVariant{}, errs.New(errs.CodeInternalError, "files: OpenImageVariant without an object store")
	}
	f, found, err := s.d.Repo.File(ctx, fileID)
	if err != nil {
		return ImageVariant{}, err
	}
	if !found || f.Purpose != string(p) || !domain.IsDeliverable(f.UploadState, f.ScanStatus, f.Deleted) {
		return ImageVariant{}, notFound()
	}
	i := slices.IndexFunc(f.Thumbnails, func(t ThumbnailRecord) bool { return t.Variant == v })
	if i < 0 {
		return ImageVariant{}, notFound()
	}
	t := f.Thumbnails[i]
	body, err := s.d.Store.Get(ctx, t.ObjectKey, nil)
	switch {
	case errors.Is(err, ErrNotFound):
		return ImageVariant{}, notFound()
	case err != nil:
		return ImageVariant{}, imageStorageError(err)
	}
	return ImageVariant{Body: body, MIMEType: t.MIMEType, SizeBytes: t.SizeBytes}, nil
}
