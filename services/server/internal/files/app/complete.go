// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/files"
	"github.com/OWNER/bothy/services/server/internal/files/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// Complete implements files.Uploads (brief M3-03 design note 12, docs/DECISIONS.md D-M3-03-11 and
// -12): in one unit of work (the ambient one of a keyed request) the session FOR UPDATE, then
// authorize, the state, the list of the client against the confirmed parts and the state change.
// No object storage call, no event and no job (M3-04 adds files.scan here). The size metric is
// recorded after the commit.
func (s *Service) Complete(ctx context.Context, cmd files.CompleteUpload) (files.File, error) {
	a := caller(ctx)
	var out files.File
	var size int64
	err := s.d.UoW.Do(ctx, func(ctx context.Context) error {
		rec, found, err := s.d.Repo.LockSession(ctx, cmd.FileID, a.ID, true)
		if err != nil {
			return err
		}
		if !found {
			return notFound()
		}
		d, err := s.decide(ctx, a, authorize.ActionFileUpload, authorize.Resource{Kind: authorize.ResourceChannel, ID: rec.File.ScopeChannelID})
		if err != nil {
			return err
		}
		if d.Outcome == authorize.OutcomeNotFound {
			// The idempotency stage rolls back a final error, so the session is not aborted here
			// (design note 14); the next request without key or the expiry ends it.
			return notFound()
		}
		if err := d.Err(); err != nil {
			return err
		}
		now := s.now()
		if rec.State == domain.SessionCompleted {
			out = FileView(rec.File)
			return nil
		}
		if s.expired(rec, now) {
			return gone()
		}
		if err := s.checkParts(ctx, rec, cmd.Parts); err != nil {
			return err
		}
		if err := s.d.Repo.CompleteSession(ctx, cmd.FileID, now); err != nil {
			return err
		}
		out, size = FileView(rec.File), rec.DeclaredSizeBytes
		if !txn.AfterCommit(ctx, func() { s.d.Metrics.UploadCompleted(ctx, size) }) {
			return errs.New(errs.CodeInternalError, "files: Complete outside a unit of work")
		}
		return nil
	})
	if err != nil {
		return files.File{}, err
	}
	return out, nil
}

// checkParts checks the list of the client against the confirmed parts of rec: a duplicate or
// out-of-range number is 400, a number of the plan without a confirmed part 409 upload_incomplete
// with the missing numbers as ranges in detail, a listed ETag that differs from the confirmed one
// 409 part_conflict. The list may name fewer parts than exist; in mode direct the confirmed rows
// are the truth. The sum of the confirmed sizes equals the declared size by construction.
func (s *Service) checkParts(ctx context.Context, rec SessionRecord, listed []files.PartRef) error {
	seen := make(map[int]bool, len(listed))
	for _, p := range listed {
		switch {
		case seen[p.PartNumber]:
			return fieldError(pointerParts, codeDuplicate, "a part number is listed twice")
		case p.PartNumber < 1 || p.PartNumber > rec.PartCount:
			return fieldError(pointerParts, codeOutOfRange, "a part number is outside the part plan of the upload")
		}
		seen[p.PartNumber] = true
	}
	parts, err := s.d.Repo.Parts(ctx, rec.FileID)
	if err != nil {
		return err
	}
	byNumber := make(map[int]PartRecord, len(parts))
	numbers := make([]int, 0, len(parts))
	var sum int64
	for _, p := range parts {
		byNumber[p.PartNumber] = p
		numbers = append(numbers, p.PartNumber)
		sum += p.SizeBytes
	}
	if missing := domain.MissingParts(rec.PartCount, numbers); len(missing) > 0 {
		return conflict(reasonUploadIncomplete, domain.Ranges(missing))
	}
	for _, p := range listed {
		if p.ETag != partETag(byNumber[p.PartNumber]) {
			return conflict(reasonPartConflict, "a listed etag differs from the confirmed part")
		}
	}
	if sum != rec.DeclaredSizeBytes {
		return errs.New(errs.CodeInternalError, "files: the confirmed parts do not add up to the declared size")
	}
	return nil
}
