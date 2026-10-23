// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/messaging"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// The answer of a refused run_id (M2-19 design note 4). runMismatchDetail equals
// messaging/domain.RunMismatchDetail, so a person's run_id and every refused run of an agent
// answer with the same body; the integration test TestAnswer_RunIDMismatch holds both equal.
const (
	runMismatchPointer = "/run_id"
	runMismatchCode    = "run_mismatch"
	runMismatchDetail  = "run_id does not name an active run of the calling agent in this thread"
	reasonRunNotActive = "run_not_active"
)

// checkAnswer checks the run an agent's message names in run_id (M2-19 design note 4) inside the
// unit of work of the message: the run row under a shared lock (LockRunForAnswer, design note 7),
// then domain.AnswerVerdict. It changes nothing on the run and writes no audit entry; the
// authorization of the message itself (message.create) already ran. Errors: 400
// validation_failed on /run_id with run_mismatch for an unknown run, a run of another thread or
// of another agent; 409 conflict with reason run_not_active for an ended own run; the error of
// the repository otherwise. Each error vetoes the unit, so nothing of the message is stored.
func (s *Service) checkAnswer(ctx context.Context, ev messaging.MessagePosted) error {
	run, found, err := s.d.Repo.LockRunForAnswer(ctx, *ev.RunID)
	if err != nil {
		return err
	}
	return answerError(domain.AnswerVerdict(run, found, ev.AuthorID, ev.ThreadID))
}

// answerError maps the verdict of domain.AnswerVerdict to the answer of run_id; the card check
// of AttachCard (M2-15) uses it too, so a card and a text answer refuse a run_id alike.
func answerError(verdict error) error {
	switch err := verdict; {
	case err == nil:
		return nil
	case errors.Is(err, domain.ErrAnswerRunMismatch):
		return errs.New(errs.CodeValidationFailed, runMismatchDetail).
			WithFields(errs.FieldError{Pointer: runMismatchPointer, Code: runMismatchCode, Detail: runMismatchDetail})
	case errors.Is(err, domain.ErrAnswerRunNotActive):
		return errs.New(errs.CodeConflict, "the run of run_id has ended").WithReason(reasonRunNotActive)
	default:
		return errs.New(errs.CodeInternalError, "runs: an unknown verdict of the answer check")
	}
}
