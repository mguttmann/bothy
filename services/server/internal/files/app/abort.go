// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/files/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// uploadStateFailed is files.upload_state of an aborted or failed upload.
const uploadStateFailed = "failed"

// Abort implements files.Uploads (brief M3-03 design note 13, docs/DECISIONS.md D-M3-03-11 and
// -20): the own session (404 otherwise), file.delete on the file (a lost read aborts the open
// session and answers 404), an aborted or expired session is a success without change, a
// completed one 409 invalid_state_transition; an open one, also past expires_at, is aborted.
func (s *Service) Abort(ctx context.Context, fileID ids.ID) error {
	a := caller(ctx)
	rec, err := s.ownSession(ctx, a, fileID, authorize.ActionFileDelete)
	if err != nil {
		return err
	}
	switch rec.State {
	case domain.SessionAborted, domain.SessionExpired:
		return nil
	case domain.SessionCompleted:
		return completedNoAbort()
	case domain.SessionOpen:
	}
	return s.abort(ctx, a, fileID)
}

// completedNoAbort is the 409 of an abort of a completed session; deleting the file is
// deleteFile (M3-08).
func completedNoAbort() error {
	return errs.New(errs.CodeInvalidStateTransition, "a completed upload cannot be aborted")
}

// abort ends the open session of fileID of a in its own unit of work: the session row FOR
// UPDATE (it serializes with parts and completion), aborted, the file failed, its part rows gone
// and recordFileChange (file.updated for the uploader and FileFinalized, because failed is final);
// after the commit the part store removes the objects. A
// failure of that removal is logged with the file id only and does not fail the abort (the
// cleanup of failed files removes what is left, M3-08). A session that changed state meanwhile
// answers like Abort does.
func (s *Service) abort(ctx context.Context, a actor.Actor, fileID ids.ID) error {
	var locked SessionRecord
	err := s.d.UoW.Do(ctx, func(ctx context.Context) error {
		cur, found, err := s.d.Repo.LockSession(ctx, fileID, a.ID, true)
		if err != nil {
			return err
		}
		if !found {
			return notFound()
		}
		locked = cur
		switch cur.State {
		case domain.SessionAborted, domain.SessionExpired:
			return nil
		case domain.SessionCompleted:
			return completedNoAbort()
		case domain.SessionOpen:
		}
		if err := s.d.Repo.AbortSession(ctx, fileID); err != nil {
			return err
		}
		f := cur.File
		f.UploadState = uploadStateFailed
		return s.changes().recordFileChange(ctx, a, cur.File, f)
	})
	if err != nil || locked.State != domain.SessionOpen {
		return err
	}
	if err := s.d.Parts.Discard(ctx, locked); err != nil {
		s.logDiscard(ctx, locked, err)
	}
	return nil
}
