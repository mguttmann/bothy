// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"log/slog"

	"github.com/OWNER/bothy/services/server/internal/files/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// msgPurgeAttached is the Error line of a deleted file that still has an attachment row.
const msgPurgeAttached = "file purge found an attachment row of a deleted file; the row stays"

// PurgeDeps are the collaborators of the purge.
type PurgeDeps struct {
	// Store reads the purge facts and deletes the row.
	Store CleanupStore
	// Objects deletes the originals and variants.
	Objects ObjectStore
	// Parts discards the parts or the multipart upload of the session.
	Parts PartStore
	// UoW runs the row delete.
	UoW txn.UnitOfWork
	// Log receives the Error of an attached deleted file; nil means slog.Default().
	Log *slog.Logger
}

// Purge is the use case of the job files.purge_objects (docs/spec/12-files.md section 12, brief
// M3-08 design note 4): it deletes every object the key schema allows for a deleted, failed or
// blocked file and then, for a deleted file, its row. It never calls the object store inside a
// unit of work (02 8.5) and never lists the store (D-FI-11). It is safe for concurrent use; two
// runs for the same file delete the row once.
type Purge struct {
	d PurgeDeps
}

// NewPurge returns the use case. Errors: a missing collaborator.
func NewPurge(d PurgeDeps) (*Purge, error) {
	if d.Store == nil || d.Objects == nil || d.Parts == nil || d.UoW == nil {
		return nil, errors.New("files: NewPurge needs every collaborator")
	}
	if d.Log == nil {
		d.Log = slog.Default()
	}
	return &Purge{d: d}, nil
}

// Run purges fileID (design note 4 c): (1) the facts, the stored key and the session without a
// transaction, and domain.PlanPurge; PurgeNothing returns nil. (2) Outside every unit of work the
// part store discards the parts or aborts the multipart upload, then every key of
// domain.ObjectKeysOf is deleted; a missing object counts as deleted (12 4.1). (3) For a deleted
// file only: one unit of work locks the row, checks that it still exists, is deleted and has no
// attachment row, and deletes it; sessions, parts, thumbnails and text extracts follow by their
// ON DELETE CASCADE. No event and no audit (05 4.8).
//
// Errors: a cancelled ctx gives ctx.Err(); the first storage error is returned unchanged, so River
// retries the job; a key outside the key schema and an attachment row of a deleted file are
// internal errors, and nothing more is deleted.
func (p *Purge) Run(ctx context.Context, fileID ids.ID) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	rec, found, err := p.d.Store.PurgeRecord(ctx, fileID)
	if err != nil {
		return err
	}
	if !found {
		return nil
	}
	plan := domain.PlanPurge(rec.Facts)
	if plan == domain.PurgeNothing {
		return nil
	}
	keys, err := domain.ObjectKeysOf(fileID, rec.ObjectKey)
	if err != nil {
		return errs.Wrap(errs.CodeInternalError, err, "files: the stored original key is outside the key schema")
	}
	if rec.HasSession {
		s := SessionRecord{FileID: fileID, PartCount: rec.PartCount, StorageUploadID: rec.StorageUploadID,
			File: FileRecord{ID: fileID, ObjectKey: rec.ObjectKey, UploadedBy: rec.UploadedBy}}
		if err := p.d.Parts.Discard(ctx, s); err != nil {
			return err
		}
	}
	for _, k := range keys {
		if err := p.d.Objects.Delete(ctx, k); err != nil {
			return err
		}
	}
	if plan != domain.PurgeObjectsAndRow {
		return nil
	}
	return p.d.UoW.Do(ctx, func(ctx context.Context) error { return p.deleteRow(ctx, fileID) })
}

// deleteRow is the unit of work of step 3: the row of a deleted file without attachment row goes;
// a row that another run removed meanwhile is no error.
func (p *Purge) deleteRow(ctx context.Context, fileID ids.ID) error {
	found, err := p.d.Store.LockFile(ctx, fileID)
	if err != nil || !found {
		return err
	}
	cur, found, err := p.d.Store.CleanupRecord(ctx, fileID)
	if err != nil || !found || !cur.Deleted {
		return err
	}
	if cur.Attached {
		p.d.Log.LogAttrs(ctx, slog.LevelError, msgPurgeAttached, slog.String(logKeyFileID, fileID.String()))
		return errs.New(errs.CodeInternalError, "files: a deleted file still has an attachment row")
	}
	_, err = p.d.Store.DeleteFileRow(ctx, fileID)
	return err
}
