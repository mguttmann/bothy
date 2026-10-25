// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"io"
	"log/slog"

	"github.com/OWNER/bothy/services/server/internal/files"
	"github.com/OWNER/bothy/services/server/internal/files/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// mediaTypeZip is files.mime_type of an export package (20 section 5.2: a ZIP archive).
const mediaTypeZip = "application/zip"

// msgExportCleanup is the Warn line of an object of a refused export package that could not be
// removed; the next store never reuses its key, so it only costs space.
const msgExportCleanup = "export package object could not be removed after a failed store"

// ExportPackages implements files.ExportPackages over the files use cases (M7-06): the export
// packages of 20 section 5.2 are files of purpose gdpr_export that the server writes itself, so
// they have no upload session, need no scan and never appear in the file operations (authz
// refuses file.read on them; their path is gdpr.export.download). It is safe for concurrent use.
type ExportPackages struct {
	s     *Service
	store PersonalDataStore
}

// Compile-time check: ExportPackages is the port privacy uses.
var _ files.ExportPackages = (*ExportPackages)(nil)

// NewExportPackages returns the port over the files use cases s, which must hold the object store
// (Deps.Store), and store. Errors: a missing collaborator.
//
// Example:
//
//	pkgs, err := app.NewExportPackages(svc, postgres.NewRepository(pool))
func NewExportPackages(s *Service, store PersonalDataStore) (*ExportPackages, error) {
	if s == nil || store == nil || s.d.Store == nil {
		return nil, errors.New("files: NewExportPackages needs the use cases with their object store and the store")
	}
	return &ExportPackages{s: s, store: store}, nil
}

// Store implements files.ExportPackages: outside any unit of work it checks the free space of the
// file system driver, writes the object under the original key of a new file id while it counts
// and hashes the body, and only when length and SHA-256 match cmd inserts the row: purpose
// gdpr_export, uploaded_by the subject, no scope channel, upload_state complete, scan_status
// skipped (the server wrote the bytes, nobody uploaded them), application/zip, no event. A refused
// package leaves no row, and its object is removed again.
//
// Errors: internal for a call inside a unit of work, a missing subject, name or body, a malformed
// digest, a length or digest that differs; 503 storage_full; 503 for the storage; those of the
// insert.
func (x *ExportPackages) Store(ctx context.Context, cmd files.StoreExport) (ids.ID, error) {
	if txn.InUnit(ctx) {
		return ids.ID{}, errs.New(errs.CodeInternalError, "files: storing an export package runs outside a unit of work")
	}
	want, err := hex.DecodeString(cmd.SHA256)
	if err != nil || len(want) != sha256.Size || cmd.SubjectID.IsZero() || cmd.Name == "" || cmd.Body == nil || cmd.SizeBytes < 0 {
		return ids.ID{}, errs.New(errs.CodeInternalError, "files: an export package needs its subject, name, body, length and SHA-256")
	}
	if err := x.s.checkSpace(ctx, cmd.SizeBytes); err != nil {
		return ids.ID{}, err
	}
	id := ids.New()
	key, err := domain.OriginalKey(id, 1)
	if err != nil {
		return ids.ID{}, err
	}
	h := sha256.New()
	err = x.s.d.Store.Put(ctx, key, io.TeeReader(cmd.Body, h), cmd.SizeBytes)
	switch {
	case errors.Is(err, ErrSizeMismatch):
		return ids.ID{}, errs.Wrap(errs.CodeInternalError, err, "files: the export package has another length")
	case err != nil:
		return ids.ID{}, imageStorageError(err)
	}
	var sum [sha256.Size]byte
	copy(sum[:], h.Sum(nil))
	if subtle.ConstantTimeCompare(sum[:], want) != 1 {
		x.removeObject(ctx, id, key)
		return ids.ID{}, errs.New(errs.CodeInternalError, "files: the export package has another SHA-256")
	}
	rec := FileRecord{
		ID: id, ObjectKey: key, SHA256: sum, SizeBytes: cmd.SizeBytes, MIMEType: mediaTypeZip, OriginalName: cmd.Name,
		UploadedBy: cmd.SubjectID, Purpose: string(domain.PurposeGDPRExport), UploadState: domain.UploadComplete,
		ScanStatus: domain.ScanSkipped, TranscriptStatus: transcriptNone, CreatedAt: x.s.now(),
	}
	if err := x.s.d.UoW.Do(ctx, func(ctx context.Context) error { return x.store.InsertExportFile(ctx, rec) }); err != nil {
		x.removeObject(ctx, id, key)
		return ids.ID{}, err
	}
	return id, nil
}

// removeObject deletes the object of a refused package; a failure is logged with the file id only.
func (x *ExportPackages) removeObject(ctx context.Context, id ids.ID, key string) {
	if err := x.s.d.Store.Delete(context.WithoutCancel(ctx), key); err != nil {
		x.s.d.Log.LogAttrs(ctx, slog.LevelWarn, msgExportCleanup, slog.String(logKeyFileID, id.String()),
			slog.String(logKeyErrorType, errorType(err)))
	}
}

// Open implements files.ExportPackages: the bytes of the export package id with its name, length
// and SHA-256. Errors: not_found for an unknown or deleted file, a file of another purpose and a
// missing object; 503 for the storage.
func (x *ExportPackages) Open(ctx context.Context, id ids.ID) (files.ExportContent, error) {
	f, found, err := x.s.d.Repo.File(ctx, id)
	if err != nil {
		return files.ExportContent{}, err
	}
	if !found || f.Purpose != string(domain.PurposeGDPRExport) {
		return files.ExportContent{}, notFound()
	}
	body, err := x.s.d.Store.Get(ctx, f.ObjectKey, nil)
	switch {
	case errors.Is(err, ErrNotFound):
		return files.ExportContent{}, notFound()
	case err != nil:
		return files.ExportContent{}, imageStorageError(err)
	}
	return files.ExportContent{Name: f.OriginalName, SizeBytes: f.SizeBytes, SHA256: hex.EncodeToString(f.SHA256[:]), Body: body}, nil
}

// Delete implements files.ExportPackages inside the caller's unit of work: the row FOR UPDATE, a
// new read, then markDeleted (redaction and files.purge_objects as for every deletion, 20 4.3).
// An unknown or deleted file and a file of another purpose are left alone. Errors: internal
// outside a unit of work; the storage.
func (x *ExportPackages) Delete(ctx context.Context, id ids.ID) error {
	if !txn.InUnit(ctx) {
		return errs.New(errs.CodeInternalError, "files: deleting an export package needs a unit of work")
	}
	found, err := x.s.d.Cleanup.LockFile(ctx, id)
	if err != nil || !found {
		return err
	}
	cur, found, err := x.s.d.Cleanup.CleanupRecord(ctx, id)
	if err != nil || !found || cur.Deleted || cur.Purpose != string(domain.PurposeGDPRExport) {
		return err
	}
	return x.s.markDeleted(ctx, Deletion{FileIDs: []ids.ID{id}, Purpose: string(domain.PurposeGDPRExport)})
}
