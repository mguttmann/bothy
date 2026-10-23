// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// The reasons of the 409 conflict of deleteFile (brief M3-08 design note 9 d,
// docs/decisions.d/M3-08.md).
const (
	// reasonFileAttached: an attached file goes with message.delete (05 4.8).
	reasonFileAttached = "file_attached"
	// reasonPurposeNotDeletable: avatar and logo are replaced or removed through their own
	// operations (M3-10), export files are M7-06's.
	reasonPurposeNotDeletable = "purpose_not_deletable"
)

// Delete implements files.Files (deleteFile, 05 4.8 file.delete, 12 section 12 row "file.delete";
// brief M3-08 design note 9):
//
//   - no row (never existed or purged): 404 not_found, the answer of an unknown id (06 7);
//   - deleted already: the same 404 for everyone but its uploader, without asking authorize
//     (AZ-P4); for the uploader file.delete on the descriptor of the dead row, which the HTTP
//     chain needs as proof of the check (02 "Proof of the check"), and nil (204) when it allows,
//     the 404 otherwise;
//   - otherwise authorize file.delete on fileResource: not_found answers 404, a denial with read
//     403 forbidden;
//   - after authorize an attached file answers 409 conflict file_attached and a file whose purpose
//     is not attachment 409 conflict purpose_not_deletable;
//   - one unit of work locks the row, reads deletion and attachment again (a concurrent delete or
//     attach wins and gives its answer) and calls markDeleted. No event and no audit (05 4.8).
func (s *Service) Delete(ctx context.Context, fileID ids.ID) error {
	a := caller(ctx)
	f, found, err := s.d.Cleanup.FileForDelete(ctx, fileID)
	if err != nil {
		return err
	}
	if !found {
		return notFound()
	}
	if f.Deleted {
		return s.deletedAgain(ctx, a, f)
	}
	ref, err := s.attachmentRef(ctx, fileID)
	if err != nil {
		return err
	}
	d, err := s.decide(ctx, a, authorize.ActionFileDelete, fileResource(f, ref))
	if err != nil {
		return err
	}
	if err := d.Err(); err != nil {
		return err
	}
	if ref != nil {
		return conflict(reasonFileAttached, "an attached file is deleted with its message")
	}
	if f.Purpose != purposeAttachment {
		return conflict(reasonPurposeNotDeletable, "this file is replaced or removed through its own operation")
	}
	return s.d.UoW.Do(ctx, func(ctx context.Context) error { return s.deleteLocked(ctx, a, fileID) })
}

// deleteLocked is the unit of work of deleteFile (design note 9 e): the row FOR UPDATE, deletion
// and attachment read again in a new statement (a concurrent delete or attach wins and gives its
// answer), then markDeleted.
func (s *Service) deleteLocked(ctx context.Context, a actor.Actor, fileID ids.ID) error {
	found, err := s.d.Cleanup.LockFileForDelete(ctx, fileID)
	if err != nil {
		return err
	}
	if !found {
		return notFound()
	}
	cur, found, err := s.d.Cleanup.CleanupRecord(ctx, fileID)
	if err != nil {
		return err
	}
	switch {
	case !found:
		return notFound()
	case cur.Deleted:
		return deletedAnswer(a, cur.UploadedBy)
	case cur.Attached:
		return conflict(reasonFileAttached, "an attached file is deleted with its message")
	}
	return s.markDeleted(ctx, Deletion{FileIDs: []ids.ID{fileID}})
}

// deletedAgain answers deleteFile for a file that is deleted already (design note 9 b): the 404 of
// an unknown id for everyone but the uploader, without authorize; for the uploader the check of
// file.delete on the unattached descriptor of the row, then nil (06 7) or the same 404.
func (s *Service) deletedAgain(ctx context.Context, a actor.Actor, f FileRecord) error {
	if err := deletedAnswer(a, f.UploadedBy); err != nil {
		return err
	}
	d, err := s.decide(ctx, a, authorize.ActionFileDelete, fileResource(f, nil))
	if err != nil {
		return err
	}
	if !d.Allowed() {
		return notFound()
	}
	return nil
}

// deletedAnswer is the answer of deleteFile for a file that is deleted already: success for its
// uploader (06 7, DELETE of a deleted resource), the 404 of an unknown id for everyone else.
func deletedAnswer(a actor.Actor, uploadedBy ids.ID) error {
	if a.ID == uploadedBy && !uploadedBy.IsZero() {
		return nil
	}
	return notFound()
}
