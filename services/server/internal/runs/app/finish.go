// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/textcheck"
	"github.com/OWNER/bothy/services/server/internal/runs"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// The audit attributes of an I3 mismatch the adapter reports (10 section 14 row "I3 mismatch").
const (
	attrBillingModeExpected = "billing_mode_expected"
	attrToolClass           = "tool_class"
)

// AwaitInput implements runs.Runs (awaitRunInput, run.finish; M2-09 design note 6 a): running ->
// waiting_input with the status line; waiting_input again answers the run without change (06
// section 2 "No effect"); every other state gives 409 invalid_state_transition.
func (s *Service) AwaitInput(ctx context.Context, cmd runs.AwaitInput) (runs.Run, error) {
	if textcheck.DisplayText(cmd.StatusLine) != nil {
		return runs.Run{}, fieldError("/status_line", "invalid", "the status line contains characters that are not allowed")
	}
	a := caller(ctx)
	var doc runs.Run
	err := s.d.UoW.Do(ctx, func(ctx context.Context) error {
		row, err := s.lockAgentRun(ctx, a, cmd.RunID)
		if err != nil {
			return err
		}
		//exhaustive:ignore // Every other state cannot wait for input (default).
		switch row.State {
		case domain.StateWaitingInput:
			doc, err = s.documentOf(ctx, row)
			return err
		case domain.StateRunning:
		default:
			return errs.New(errs.CodeInvalidStateTransition, "only a running run can wait for input")
		}
		t, err := s.Transition(ctx, TransitionCmd{
			RunID: row.ID, From: row.State, Version: row.Version, To: domain.StateWaitingInput, SetBy: domain.SetterAdapter,
			Store: TransitionStore{StatusLine: cmd.StatusLine},
		})
		if err != nil {
			return err
		}
		doc, err = s.documentByID(ctx, t.Run.ID)
		return err
	})
	if err != nil {
		return runs.Run{}, err
	}
	return doc, nil
}

// Finish implements runs.Runs (finishRun, run.finish; M2-09 design note 6 b to e): the reason
// rules of FinishDecision, the same end state again without change, otherwise the end of the run
// through endRun with the metrics and the summary. An adapter that reports an I3 mismatch writes
// one audit entry in the same unit of work and counts bothy.agent.billing_mode_mismatch.
func (s *Service) Finish(ctx context.Context, cmd runs.FinishRun) (runs.Run, error) {
	req := domain.FinishRequest{Target: domain.State(cmd.State), Reason: cmd.Reason, CostsPresent: cmd.CostsPresent}
	// The request rules come before the run is read: a bad body is 400 for every run.
	if _, err := domain.FinishDecision(domain.StateRunning, req); err != nil && errs.KindOf(err) != errs.KindConflict {
		return runs.Run{}, finishError(err)
	}
	if textcheck.DisplayText(cmd.Summary) != nil {
		return runs.Run{}, fieldError("/summary", "invalid", "the summary contains characters that are not allowed")
	}
	metrics, err := metricsJSON(cmd.Metrics)
	if err != nil {
		return runs.Run{}, err
	}
	a := caller(ctx)
	var f finished
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		f, err = s.finishLocked(ctx, a, cmd, req, metrics)
		return err
	})
	if err != nil {
		return runs.Run{}, err
	}
	if f.mismatch {
		s.d.Mismatches.BillingModeMismatch(context.WithoutCancel(ctx), f.mismatchHandle)
	}
	return f.doc, nil
}

// finished is the result of finishLocked: the run document and, for an adapter-reported I3
// mismatch, the handle of the agent whose counter Finish raises after the commit.
type finished struct {
	doc            runs.Run
	mismatch       bool
	mismatchHandle string
}

// finishLocked is the unit of work of Finish: it locks and authorizes the run, applies
// FinishDecision and ends the run, with the audit entry of a reported I3 mismatch.
func (s *Service) finishLocked(ctx context.Context, a actor.Actor, cmd runs.FinishRun, req domain.FinishRequest,
	metrics []byte) (finished, error) {
	row, err := s.lockAgentRun(ctx, a, cmd.RunID)
	if err != nil {
		return finished{}, err
	}
	out, err := domain.FinishDecision(row.State, req)
	if err != nil {
		return finished{}, finishError(err)
	}
	if out.Kind == domain.FinishNoChange {
		doc, err := s.documentOf(ctx, row)
		return finished{doc: doc}, err
	}
	reason := cmd.Reason
	if req.Target == domain.StateCompleted {
		reason = ""
	}
	t, err := s.endRun(ctx, row, req.Target, reason, domain.SetterAdapter, TransitionStore{Metrics: metrics, Summary: cmd.Summary})
	if err != nil {
		return finished{}, err
	}
	var f finished
	if out.Reason == domain.RunReasonBillingModeMismatch || out.Reason == domain.RunReasonToolClassMismatch {
		if err := s.auditReportedMismatch(ctx, a, row, out.Reason); err != nil {
			return finished{}, err
		}
		f.mismatch, f.mismatchHandle = true, t.AgentHandle
	}
	f.doc, err = s.documentByID(ctx, row.ID)
	return f, err
}

// finishError maps the request errors of FinishDecision to 400 validation_failed with the field
// of 06 section 5; the 409 of a transition passes unchanged.
func finishError(err error) error {
	switch {
	case errors.Is(err, domain.ErrCostsNotEnabled):
		// Costs are booked into cost_entries from M2-22 (ruling D-M2-09-14).
		return notEnabled("/costs")
	case errors.Is(err, domain.ErrReasonRequired):
		return fieldError("/reason", "required", "failed and cancelled need a reason")
	case errors.Is(err, domain.ErrCancelReason):
		return fieldError("/reason", "invalid_enum", "cancelled takes only the reason of a cancel request")
	case errors.Is(err, domain.ErrFinishTarget):
		return fieldError("/state", "invalid_enum", "the state is not an end state")
	}
	return err
}

// lockAgentRun locks the run id and authorizes the agent-self action run.finish on it (design
// note 1); an unknown or invisible run gives the same not_found.
func (s *Service) lockAgentRun(ctx context.Context, a actor.Actor, id ids.ID) (RunRow, error) {
	row, found, err := s.d.Repo.LockRun(ctx, id)
	if err != nil {
		return RunRow{}, err
	}
	d, err := s.authorizeAgent(ctx, a, authorize.ActionRunFinish, row, found, id, nil)
	if err != nil {
		return RunRow{}, err
	}
	if !d.Allowed() {
		return RunRow{}, d.Err()
	}
	return row, nil
}

// endRun is the one end-of-run function (design note 6 f): every end state of a run goes through
// it, from the adapter (finishRun), the server (I3 at the start, a refused renewal, a cancel
// request that ends at once, the jobs runs.cancel_timeout and runs.stall_check). Transition stamps
// finished_at and duration_ms, sets trigger_state done, failed or cancelled with the reason,
// appends run.state_changed and records the run metrics after the commit.
//
// Extension point: M2-11 (follow-ups deferred or discarded at the end of a run) and M2-15
// (components disabled at the end) add their steps here, after the transition and in the same
// unit of work, so they see the ended run and commit with it. M2-15: DisableRunComponents at
// every end state, not only after a stop (ruling D-M2-15-21).
func (s *Service) endRun(ctx context.Context, row RunRow, to domain.State, reason string, setBy domain.Setter,
	store TransitionStore) (Transitioned, error) {
	if !to.Ended() {
		return Transitioned{}, errs.New(errs.CodeInternalError, "runs: endRun needs an end state")
	}
	t, err := s.Transition(ctx, TransitionCmd{
		RunID: row.ID, From: row.State, Version: row.Version, To: to, Reason: reason, SetBy: setBy, Store: store,
	})
	if err != nil {
		return Transitioned{}, err
	}
	if err := s.DisableRunComponents(ctx, row.ID); err != nil {
		return Transitioned{}, err
	}
	return t, nil
}

// auditReportedMismatch writes the audit entry of an I3 mismatch the adapter reported (design note
// 6 e, 10 section 14): action run.finish, decision deny with the mismatch as reason, the configured
// mode of the run and the configured tool class of the agent as codes; never a token or a jti.
func (s *Service) auditReportedMismatch(ctx context.Context, a actor.Actor, row RunRow, reason string) error {
	if s.d.Audit == nil {
		return errs.New(errs.CodeInternalError, "runs: the end of a run has no audit recorder")
	}
	facts, err := s.d.Agents.RunFacts(ctx, []ids.ID{row.AgentID})
	if err != nil {
		return err
	}
	attrs := auditing.Attrs{attrRunID: auditing.ID(row.ID), attrBillingModeExpected: auditing.Code(row.BillingModeExpected)}
	if tc := facts[row.AgentID].ToolClass; tc != "" {
		attrs[attrToolClass] = auditing.Code(tc)
	}
	return s.d.Audit.Record(actor.WithActor(ctx, a), auditing.Entry{
		Action: string(authorize.ActionRunFinish), Decision: auditing.DecisionDeny, Reason: auditing.Reason(reason),
		ResourceKind: resourceRun, ResourceID: row.ID, Attrs: attrs,
	})
}

// metricsJSON encodes run_metrics.v1; nil keeps the stored object.
func metricsJSON(m map[string]any) ([]byte, error) {
	if m == nil {
		return nil, nil
	}
	return encode(m)
}
