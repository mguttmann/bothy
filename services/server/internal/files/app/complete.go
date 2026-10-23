// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/files"
	"github.com/OWNER/bothy/services/server/internal/files/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// Complete implements files.Uploads (brief M3-03 design note 12, docs/DECISIONS.md D-M3-03-11 and
// -12): in one unit of work (the ambient one of a keyed request) the session FOR UPDATE, then
// authorize, the state, the list of the client against the confirmed parts, the state change and
// the job files.scan with round 0 (M3-04), so the job exists exactly when the session is
// completed. No object storage call and no event. The size metric is recorded after the commit.
func (s *Service) Complete(ctx context.Context, cmd files.CompleteUpload) (files.File, error) {
	a := caller(ctx)
	var out files.File
	err := s.d.UoW.Do(ctx, func(ctx context.Context) error {
		f, size, done, err := s.complete(ctx, a, cmd)
		if err != nil || !done {
			out = f
			return err
		}
		out = f
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

// complete is the work of Complete inside its unit of work: it returns the file, the declared size
// and whether this call completed the session (false for a session completed before).
func (s *Service) complete(ctx context.Context, a actor.Actor, cmd files.CompleteUpload) (files.File, int64, bool, error) {
	if err := actingPrincipal(a); err != nil {
		return files.File{}, 0, false, err
	}
	rec, found, err := s.d.Repo.LockSession(ctx, cmd.FileID, a.ID, true)
	if err != nil {
		return files.File{}, 0, false, err
	}
	if !found || rec.File.Purpose != purposeAttachment {
		// An avatar or logo is completed by its own operation (CompleteStagedImage, M3-10).
		return files.File{}, 0, false, notFound()
	}
	d, err := s.decide(ctx, a, authorize.ActionFileUpload, authorize.Resource{Kind: authorize.ResourceChannel, ID: rec.File.ScopeChannelID})
	if err != nil {
		return files.File{}, 0, false, err
	}
	if d.Outcome == authorize.OutcomeNotFound {
		// The idempotency stage rolls back a final error, so the session is not aborted here
		// (design note 14); the next request without key or the expiry ends it.
		return files.File{}, 0, false, notFound()
	}
	if err := d.Err(); err != nil {
		return files.File{}, 0, false, err
	}
	now := s.now()
	switch {
	case rec.State == domain.SessionCompleted:
		return FileView(rec.File), 0, false, nil
	case s.expired(rec, now):
		return files.File{}, 0, false, gone()
	}
	if err := s.checkParts(ctx, rec, cmd.Parts); err != nil {
		return files.File{}, 0, false, err
	}
	if err := s.d.Repo.CompleteSession(ctx, cmd.FileID, now); err != nil {
		return files.File{}, 0, false, err
	}
	if err := s.d.ScanQueue.EnqueueScan(ctx, cmd.FileID, 0, time.Time{}); err != nil {
		return files.File{}, 0, false, err
	}
	return FileView(rec.File), rec.DeclaredSizeBytes, true, nil
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
