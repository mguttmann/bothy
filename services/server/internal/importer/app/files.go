// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/files"
	"github.com/OWNER/bothy/services/server/internal/importer"
	"github.com/OWNER/bothy/services/server/internal/importer/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// filesDeps refuses the file operations of a service without the files port.
func (s *Service) filesDeps() error {
	if s.d.Files == nil {
		return errs.New(errs.CodeInternalError, "importer: the service has no port for the files of an import")
	}
	return nil
}

// checkFileCmd checks the body of createImportFile before any lookup: the origin key file:<id>
// and the hex SHA-256.
func checkFileCmd(cmd importer.FileCmd) ([32]byte, error) {
	var p domain.Problems
	if _, code := domain.ParseOriginKey(domain.KindFile, cmd.OriginKey); code != "" {
		p.Add("/origin_key", code, "file:<attachment_id>")
	}
	sum, ok := domain.ParseSHA256(cmd.SHA256)
	if !ok {
		p.Add("/sha256", domain.FieldInvalid, "a hex SHA-256 is required")
	}
	return sum, p.Err()
}

// running loads the source id for import.apply under its row lock and refuses a source that is
// not running (409 import_not_running, recorded as a denial).
func (s *Service) running(ctx context.Context, a actor.Actor, id ids.ID) (*batch, error) {
	src, err := s.load(ctx, a, authorize.ActionImportApply, id, true)
	if err != nil {
		return nil, err
	}
	if src.State != importer.StateRunning {
		s.deny(ctx, id, importer.ReasonImportNotRunning)
		return nil, conflict(importer.ReasonImportNotRunning, "the source is not running")
	}
	return &batch{src: src}, nil
}

// targetOf reports whether id is the target of a posted item of the source with one of kinds.
func (s *Service) targetOf(ctx context.Context, bt *batch, id ids.ID, kinds ...string) (bool, error) {
	for _, k := range kinds {
		row, found, err := s.d.Store.ItemOfTarget(ctx, bt.src.ID, k, id)
		if err != nil {
			return false, err
		}
		if found && row.State == importer.ItemPosted {
			return true, nil
		}
	}
	return false, nil
}

// CreateFile implements importer.Imports (22 6.5 and 7.2 item 3, M8-11 design note 6): the body
// checks, then one unit of work: import.apply on the running source, the channel of a posted
// channel item of the source (409 target_outside_source) and the author of a posted user item
// (409 author_not_imported), then the item. A new key writes the pending item with the file that
// files.ImportFiles.Reserve inserts; a pending key answers the same file ID with its session (a new
// one after an abort or expiry) and posts the item when its session completed meanwhile; a posted
// key answers the item without a session.
func (s *Service) CreateFile(ctx context.Context, a actor.Actor, id ids.ID, cmd importer.FileCmd) (importer.FileSlot, error) {
	ctx, err := withCaller(ctx, a)
	if err != nil {
		return importer.FileSlot{}, err
	}
	if err := s.filesDeps(); err != nil {
		return importer.FileSlot{}, err
	}
	sum, err := checkFileCmd(cmd)
	if err != nil {
		return importer.FileSlot{}, err
	}
	var out importer.FileSlot
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		bt, err := s.running(ctx, a, id)
		if err != nil {
			return err
		}
		if out, err = s.createFile(ctx, bt, cmd, sum); err != nil {
			return err
		}
		return s.count(ctx, bt)
	})
	if err != nil {
		return importer.FileSlot{}, err
	}
	return out, nil
}

// createFile is CreateFile inside its unit of work.
func (s *Service) createFile(ctx context.Context, bt *batch, cmd importer.FileCmd, sum [32]byte) (importer.FileSlot, error) {
	ok, err := s.targetOf(ctx, bt, cmd.ChannelID, targetChannel)
	if err != nil {
		return importer.FileSlot{}, err
	}
	if !ok {
		s.deny(ctx, bt.src.ID, importer.ReasonTargetOutsideSource)
		return importer.FileSlot{}, outside("the channel is not a channel of this source")
	}
	if ok, err = s.targetOf(ctx, bt, cmd.AuthorID, targetUser, targetAgent); err != nil {
		return importer.FileSlot{}, err
	}
	if !ok {
		return importer.FileSlot{}, conflict(importer.ReasonAuthorNotImported, "the author is not imported")
	}
	row, created, err := s.d.Store.InsertPendingItem(ctx, NewItem{ID: ids.New(), SourceID: bt.src.ID, OriginKey: cmd.OriginKey,
		ItemKind: domain.KindFile, At: s.now()})
	if err != nil {
		return importer.FileSlot{}, err
	}
	if !created {
		return s.fileAgain(ctx, bt, cmd.OriginKey)
	}
	slot, err := s.d.Files.Reserve(ctx, files.ImportReserve{ChannelID: cmd.ChannelID, UploadedBy: cmd.AuthorID, Name: cmd.Name,
		SizeBytes: cmd.SizeBytes, SHA256: sum, DeclaredMIMEType: cmd.DeclaredMIMEType})
	if err != nil {
		return importer.FileSlot{}, err
	}
	if row, err = s.d.Store.FinishItem(ctx, row.ID, importer.ItemPending, targetFile, slot.FileID, s.now()); err != nil {
		return importer.FileSlot{}, err
	}
	bt.counts.Add(domain.KindFile, importer.ItemPending)
	return importer.FileSlot{Item: itemDTO(row, false), Upload: plan(slot.Upload)}, nil
}

// plan converts the session of files into the port DTO.
func plan(u files.UploadSession) *importer.UploadPlan {
	return &importer.UploadPlan{FileID: u.FileID, Mode: u.Mode, PartSizeBytes: u.PartSizeBytes, PartCount: u.PartCount, ExpiresAt: u.ExpiresAt}
}

// fileAgain answers a key that exists (M8-11 design note 6 d): posted without a session; pending
// with the session of files.ImportFiles.Slot, posted when it completed meanwhile.
func (s *Service) fileAgain(ctx context.Context, bt *batch, key string) (importer.FileSlot, error) {
	row, found, err := s.d.Store.LockItem(ctx, bt.src.ID, key)
	if err != nil {
		return importer.FileSlot{}, err
	}
	if !found || row.ItemKind != domain.KindFile {
		return importer.FileSlot{}, errs.New(errs.CodeInternalError, "importer: an existing file key vanished or changed its kind")
	}
	if row.State != importer.ItemPending || row.TargetID.IsZero() {
		return importer.FileSlot{Item: itemDTO(row, false)}, nil
	}
	slot, err := s.d.Files.Slot(ctx, row.TargetID)
	if err != nil {
		return importer.FileSlot{}, err
	}
	if !slot.Completed {
		return importer.FileSlot{Item: itemDTO(row, false), Upload: plan(slot.Upload)}, nil
	}
	row, err = s.postFile(ctx, bt, row)
	return importer.FileSlot{Item: itemDTO(row, false)}, err
}

// postFile sets the pending file item row posted (22 7.2 item 3: posted when the bytes are
// complete).
func (s *Service) postFile(ctx context.Context, bt *batch, row ItemRow) (ItemRow, error) {
	row, err := s.d.Store.FinishItem(ctx, row.ID, importer.ItemPosted, targetFile, row.TargetID, s.now())
	if err != nil {
		return ItemRow{}, err
	}
	bt.counts.Add(domain.KindFile, importer.ItemPosted)
	return row, nil
}

// fileItem returns the locked item of the source whose target is the file id; 404 for a file that
// is no file item of the source.
func (s *Service) fileItem(ctx context.Context, bt *batch, fileID ids.ID) (ItemRow, error) {
	row, found, err := s.d.Store.ItemOfTarget(ctx, bt.src.ID, targetFile, fileID)
	if err != nil {
		return ItemRow{}, err
	}
	if !found {
		return ItemRow{}, notFound()
	}
	row, found, err = s.d.Store.LockItem(ctx, bt.src.ID, row.OriginKey)
	if err != nil {
		return ItemRow{}, err
	}
	if !found {
		return ItemRow{}, notFound()
	}
	return row, nil
}

// UploadFilePart implements importer.Imports (M8-11 design note 6 b2): import.apply on the running
// source and the file of a file item of the source (404 otherwise) in a short unit of work, then
// the part through files.ImportFiles.UploadPart outside any unit of work, because the body streams
// into the object store (02 8.5).
func (s *Service) UploadFilePart(ctx context.Context, a actor.Actor, id ids.ID, cmd importer.FilePartCmd) (importer.PartRef, error) {
	ctx, err := withCaller(ctx, a)
	if err != nil {
		return importer.PartRef{}, err
	}
	if err := s.filesDeps(); err != nil {
		return importer.PartRef{}, err
	}
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		bt, err := s.running(ctx, a, id)
		if err != nil {
			return err
		}
		_, err = s.fileItem(ctx, bt, cmd.FileID)
		return err
	})
	if err != nil {
		return importer.PartRef{}, err
	}
	ref, err := s.d.Files.UploadPart(ctx, files.PutPart{FileID: cmd.FileID, PartNumber: cmd.PartNumber, ContentDigest: cmd.ContentDigest,
		ContentLength: -1, Body: cmd.Body})
	return importer.PartRef{PartNumber: ref.PartNumber, ETag: ref.ETag}, err
}

// CompleteFile implements importer.Imports (M8-11 design note 6 b2 and d): one unit of work with
// import.apply on the running source, the file of a file item of the source (404 otherwise), the
// completion through files.ImportFiles.Complete (files.scan with the lowest priority) and the item
// posted with its counter and import.progress. A posted item answers again without a change.
func (s *Service) CompleteFile(ctx context.Context, a actor.Actor, id ids.ID, cmd importer.FileCompleteCmd) (importer.FileSlot, error) {
	ctx, err := withCaller(ctx, a)
	if err != nil {
		return importer.FileSlot{}, err
	}
	if err := s.filesDeps(); err != nil {
		return importer.FileSlot{}, err
	}
	var out importer.FileSlot
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		out, err = s.completeFile(ctx, a, id, cmd)
		return err
	})
	if err != nil {
		return importer.FileSlot{}, err
	}
	return out, nil
}

// completeFile is CompleteFile inside its unit of work.
func (s *Service) completeFile(ctx context.Context, a actor.Actor, id ids.ID, cmd importer.FileCompleteCmd) (importer.FileSlot, error) {
	bt, err := s.running(ctx, a, id)
	if err != nil {
		return importer.FileSlot{}, err
	}
	row, err := s.fileItem(ctx, bt, cmd.FileID)
	if err != nil {
		return importer.FileSlot{}, err
	}
	if row.State == importer.ItemPending {
		parts := make([]files.PartRef, 0, len(cmd.Parts))
		for _, p := range cmd.Parts {
			parts = append(parts, files.PartRef{PartNumber: p.PartNumber, ETag: p.ETag})
		}
		if _, err := s.d.Files.Complete(ctx, files.CompleteUpload{FileID: cmd.FileID, Parts: parts}); err != nil {
			return importer.FileSlot{}, err
		}
		if row, err = s.postFile(ctx, bt, row); err != nil {
			return importer.FileSlot{}, err
		}
	}
	return importer.FileSlot{Item: itemDTO(row, false)}, s.count(ctx, bt)
}
