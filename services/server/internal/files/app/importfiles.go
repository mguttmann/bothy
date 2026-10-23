// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/files"
	"github.com/OWNER/bothy/services/server/internal/files/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// ImportRepository is the outgoing port of the import files (M8-11) over file_upload_sessions and
// files; adapters/postgres implements it. It needs a unit of work.
type ImportRepository interface {
	// ReopenSession opens the aborted or expired session of fileID again with created_at and
	// expires_at, without a multipart upload id and without parts, and sets its file pending.
	ReopenSession(ctx context.Context, fileID ids.ID, createdAt, expiresAt time.Time) error
}

// ImportFilesPort implements files.ImportFiles over the service (M8-11 design note 6); a type of
// its own, because Service implements files.Uploads, whose Complete has the same signature and
// authorizes file.upload.
type ImportFilesPort struct{ s *Service }

// Compile-time check: the import port of files.
var _ files.ImportFiles = ImportFilesPort{}

// ImportFiles returns the port files.ImportFiles over s.
func (s *Service) ImportFiles() ImportFilesPort { return ImportFilesPort{s: s} }

// needImportUnit is the internal error of an import method outside the caller's unit of work.
func needImportUnit(ctx context.Context, method string) error {
	if txn.InUnit(ctx) {
		return nil
	}
	return errs.New(errs.CodeInternalError, "files: ImportFiles."+method+" needs the caller's unit of work")
}

// sessionView is the plan of an open session (schema UploadSession).
func sessionView(rec SessionRecord) files.UploadSession {
	return files.UploadSession{FileID: rec.FileID, Mode: modeDirect, PartSizeBytes: rec.PartSizeBytes, PartCount: rec.PartCount, ExpiresAt: rec.ExpiresAt}
}

// Reserve implements files.ImportFiles (12 11, 22 6.5): the checks of createUpload without
// file.upload and without the quota, the open sessions and the free space of the author under its
// advisory lock in the caller's unit of work, then the file and its session.
func (p ImportFilesPort) Reserve(ctx context.Context, r files.ImportReserve) (files.ImportSlot, error) {
	s := p.s
	if err := needImportUnit(ctx, "Reserve"); err != nil {
		return files.ImportSlot{}, err
	}
	if r.SizeBytes > s.cfg.MaxUploadBytes {
		return files.ImportSlot{}, errs.New(errs.CodePayloadTooLarge, "size_bytes is above file.max_upload_bytes")
	}
	name := domain.CleanName(s.d.Names.NFC(r.Name), s.cfg.NameMaxBytes)
	declared, err := s.checkType(name, r.DeclaredMIMEType)
	if err != nil {
		return files.ImportSlot{}, err
	}
	now := s.now()
	id := ids.New()
	key, err := domain.OriginalKey(id, 1)
	if err != nil {
		return files.ImportSlot{}, err
	}
	count, _ := domain.PlanParts(r.SizeBytes, s.cfg.PartBytes)
	up := NewUpload{
		File: FileRecord{
			ID: id, ObjectKey: key, SHA256: r.SHA256, SizeBytes: r.SizeBytes, MIMEType: octetStream, DeclaredMIMEType: declared,
			OriginalName: name, UploadedBy: r.UploadedBy, Purpose: purposeAttachment, ScopeChannelID: r.ChannelID,
			UploadState: stateOpenFile, ScanStatus: stateOpenFile, TranscriptStatus: transcriptNone, CreatedAt: now,
		},
		Session: SessionRecord{
			FileID: id, Mode: modeDirect, State: domain.SessionOpen, DeclaredSizeBytes: r.SizeBytes, DeclaredSHA256: r.SHA256,
			PartSizeBytes: s.cfg.PartBytes, PartCount: count, CreatedAt: now, ExpiresAt: now.Add(s.cfg.SessionTTL),
		},
	}
	if err := s.checkSessionsAndSpace(ctx, r.UploadedBy, r.SizeBytes, now); err != nil {
		return files.ImportSlot{}, err
	}
	if err := s.d.Repo.InsertUpload(ctx, up); err != nil {
		return files.ImportSlot{}, err
	}
	return files.ImportSlot{FileID: id, Upload: sessionView(up.Session)}, nil
}

// importSession locks the session of the import file fileID FOR UPDATE by its uploader; an
// unknown or deleted file and one that is no attachment are 404.
func (s *Service) importSession(ctx context.Context, fileID ids.ID) (SessionRecord, error) {
	f, found, err := s.d.Repo.File(ctx, fileID)
	if err != nil {
		return SessionRecord{}, err
	}
	if !found || f.Purpose != purposeAttachment {
		return SessionRecord{}, notFound()
	}
	rec, found, err := s.d.Repo.LockSession(ctx, fileID, f.UploadedBy, true)
	if err != nil {
		return SessionRecord{}, err
	}
	if !found {
		return SessionRecord{}, notFound()
	}
	return rec, nil
}

// Slot implements files.ImportFiles (22 7.2 item 3, M8-11 design note 6 d): a completed session is
// Completed; an open one that has not expired is answered; an aborted or expired one, also an open
// one past expires_at, opens again for the same file ID with a new expiry.
func (p ImportFilesPort) Slot(ctx context.Context, fileID ids.ID) (files.ImportSlot, error) {
	s := p.s
	if err := needImportUnit(ctx, "Slot"); err != nil {
		return files.ImportSlot{}, err
	}
	rec, err := s.importSession(ctx, fileID)
	if err != nil {
		return files.ImportSlot{}, err
	}
	now := s.now()
	switch {
	case rec.State == domain.SessionCompleted:
		return files.ImportSlot{FileID: fileID, Completed: true}, nil
	case !s.expired(rec, now):
		return files.ImportSlot{FileID: fileID, Upload: sessionView(rec)}, nil
	}
	re, ok := s.d.Repo.(ImportRepository)
	if !ok {
		return files.ImportSlot{}, errs.New(errs.CodeInternalError, "files: the repository cannot reopen an import session")
	}
	rec.State, rec.CreatedAt, rec.ExpiresAt = domain.SessionOpen, now, now.Add(s.cfg.SessionTTL)
	if err := re.ReopenSession(ctx, fileID, rec.CreatedAt, rec.ExpiresAt); err != nil {
		return files.ImportSlot{}, err
	}
	return files.ImportSlot{FileID: fileID, Upload: sessionView(rec)}, nil
}

// UploadPart implements files.ImportFiles: the session of the file by its uploader without
// file.upload, then the part rules of uploadPart (storePart), outside any unit of work.
func (p ImportFilesPort) UploadPart(ctx context.Context, cmd files.PutPart) (files.PartRef, error) {
	s := p.s
	if txn.InUnit(ctx) {
		return files.PartRef{}, errs.New(errs.CodeInternalError, "files: ImportFiles.UploadPart runs inside a unit of work")
	}
	f, found, err := s.d.Repo.File(ctx, cmd.FileID)
	if err != nil {
		return files.PartRef{}, err
	}
	if !found || f.Purpose != purposeAttachment {
		return files.PartRef{}, notFound()
	}
	rec, found, err := s.d.Repo.Session(ctx, cmd.FileID, f.UploadedBy)
	if err != nil {
		return files.PartRef{}, err
	}
	if !found {
		return files.PartRef{}, notFound()
	}
	return s.storePart(ctx, actor.Actor{ID: f.UploadedBy}, rec, cmd)
}

// Complete implements files.ImportFiles: the session FOR UPDATE by its uploader, then the rules of
// completeUpload (closeSession) with files.scan at the lowest priority (WithBulkJobs).
func (p ImportFilesPort) Complete(ctx context.Context, cmd files.CompleteUpload) (files.File, error) {
	s := p.s
	if err := needImportUnit(ctx, "Complete"); err != nil {
		return files.File{}, err
	}
	rec, err := s.importSession(ctx, cmd.FileID)
	if err != nil {
		return files.File{}, err
	}
	f, size, done, err := s.closeSession(WithBulkJobs(ctx), rec, cmd)
	if err != nil || !done {
		return f, err
	}
	if !txn.AfterCommit(ctx, func() { s.d.Metrics.UploadCompleted(ctx, size) }) {
		return files.File{}, errs.New(errs.CodeInternalError, "files: ImportFiles.Complete outside a unit of work")
	}
	return f, nil
}
