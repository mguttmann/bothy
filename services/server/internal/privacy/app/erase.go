// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"fmt"

	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/personaldata"
	"github.com/OWNER/bothy/services/server/internal/privacy/domain"
)

// RunErase is the job privacy.erase (20 section 4.4, brief M7-06 design note 3): it moves the
// approved deletion to running, then runs steps 1 to 6 in order, each step a call of every
// participant in the order of their registration (each in its own transactions), and records
// the step in progress.last_step after all participants finished it; a retry after a crash
// resumes at the next step. Step 7 writes the audit entry privacy.erase in the unit that
// completes the request. The content mode privacy.account_deletion_content is fixed in progress
// when the job starts, so a resumed deletion keeps it.
//
// The last active owner is refused in step 1 (errs.CodeLastOwner): the request fails with the
// code last_owner and the job ends without a retry.
//
// Errors: those of the participants and the storage; the job retries.
func (s *Service) RunErase(ctx context.Context, id ids.ID) error {
	r, done, err := s.begin(ctx, id, domain.KindDeletion, func(r *domain.Request) {
		if r.Progress.Content == "" {
			r.Progress.Content = string(s.cfg.DeletionContent)
		}
	})
	if err != nil || done {
		return err
	}
	e := personaldata.Erasure{
		Subject: r.SubjectUserID, RequestID: r.ID, DecidedBy: r.ApprovedBy, Content: personaldata.ContentMode(r.Progress.Content),
	}
	if !e.Content.Valid() {
		e.Content = s.cfg.DeletionContent
	}
	for _, step := range personaldata.ParticipantSteps() {
		if int(step) <= r.Progress.LastStep {
			continue
		}
		if done, err := s.runStep(ctx, id, step, e); err != nil || done {
			return err
		}
	}
	return s.finish(ctx, id, func(ctx context.Context, r *domain.Request) error {
		r.State, r.CompletedAt, r.Progress.LastStep = domain.StateCompleted, s.now(), int(personaldata.StepAudit)
		return s.d.Audit.Record(ctx, auditing.Entry{
			Action: actionErase, Decision: auditing.DecisionAllow, ResourceKind: resourceUser, ResourceID: r.SubjectUserID,
			Attrs: auditing.Attrs{attrRequestID: auditing.ID(r.ID), attrContent: auditing.Code(string(e.Content))},
		})
	})
}

// runStep runs step through every participant and records it as finished. done is true when the
// request ended (the last owner was refused); an error makes the job retry.
func (s *Service) runStep(ctx context.Context, id ids.ID, step personaldata.Step, e personaldata.Erasure) (bool, error) {
	if step == personaldata.StepDisable {
		if err := s.checkNotLastOwner(ctx, e.Subject); err != nil {
			return true, s.failIfLastOwner(ctx, id, err)
		}
	}
	for _, p := range s.d.Participants.All() {
		if err := p.Erase(ctx, step, e); err != nil {
			if step == personaldata.StepDisable {
				return true, s.failIfLastOwner(ctx, id, err)
			}
			return false, fmt.Errorf("privacy: step %s of module %s: %w", step, p.Module(), err)
		}
	}
	return false, s.markStep(ctx, id, step)
}

// failIfLastOwner ends the request as failed with last_owner when err is that refusal, and
// returns nil then (no retry: the state of the instance decides, not a fault); any other error
// is returned for a retry.
func (s *Service) failIfLastOwner(ctx context.Context, id ids.ID, err error) error {
	if e, ok := errs.As(err); ok && e.Code == errs.CodeLastOwner {
		return s.Fail(ctx, id, codeLastOwner)
	}
	return err
}

// markStep records step as the last finished step of the running request id.
func (s *Service) markStep(ctx context.Context, id ids.ID, step personaldata.Step) error {
	return s.d.UoW.Do(ctx, func(ctx context.Context) error {
		r, ok, err := s.d.Store.Lock(ctx, id)
		if err != nil || !ok {
			return err
		}
		if int(step) > r.Progress.LastStep {
			r.Progress.LastStep = int(step)
		}
		return s.d.Store.Update(ctx, r)
	})
}
