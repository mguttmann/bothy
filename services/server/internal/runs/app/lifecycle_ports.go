// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/agents"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// TokenVerifier verifies an invocation token presented at startRun, resumeRun and
// renewInvocationToken (InvocationTokens.Verify, M2-06). The tokens bound with BindTokens
// implement it.
type TokenVerifier interface {
	Verify(ctx context.Context, token string, v VerifyInvocation) (domain.InvocationClaims, error)
}

// CancelTimeoutScheduler inserts the job runs.cancel_timeout of a run in the unit of work of ctx,
// to run at or after at (M2-09 design note 7 h); adapters/jobs implements it over
// jobs.Inserter.InsertTx.
type CancelTimeoutScheduler interface {
	Schedule(ctx context.Context, runID ids.ID, at time.Time) error
}

// HeartbeatReader reads the persisted heartbeat of agents (agents.Heartbeats, M2-09): the last
// stored heartbeat and its active runs, ascending by agent; unknown agents are absent.
type HeartbeatReader interface {
	HeartbeatStates(ctx context.Context, agentIDs []ids.ID) ([]agents.HeartbeatState, error)
}

// MismatchCounter counts bothy.agent.billing_mode_mismatch (signals.md 4.3): one per run that an
// I3 mismatch failed, found by the server at the start or reported by the adapter at the end.
type MismatchCounter interface {
	BillingModeMismatch(ctx context.Context, agentHandle string)
}

// noMismatches counts nothing.
type noMismatches struct{}

// BillingModeMismatch implements MismatchCounter without effect.
func (noMismatches) BillingModeMismatch(context.Context, string) {}

// verifier returns the bound token verifier. Errors: internal before BindTokens or for an issuer
// that cannot verify.
func (s *Service) verifier() (TokenVerifier, error) {
	box := s.tokens.Load()
	if box == nil {
		return nil, errs.New(errs.CodeInternalError, "runs: the invocation tokens are not bound")
	}
	v, ok := box.issuer.(TokenVerifier)
	if !ok {
		return nil, errs.New(errs.CodeInternalError, "runs: the bound invocation tokens cannot verify")
	}
	return v, nil
}

// fieldError is 400 validation_failed with one field error.
func fieldError(pointer, code, detail string) error {
	return errs.New(errs.CodeValidationFailed, detail).WithFields(errs.FieldError{Pointer: pointer, Code: code, Detail: detail})
}
