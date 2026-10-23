// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"time"

	"github.com/OWNER/bothy/services/server/internal/agents"
	"github.com/OWNER/bothy/services/server/internal/billing"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/textcheck"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
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

// Finish implements runs.Runs (finishRun, run.finish; M2-09 design note 6 b to e, M2-22 design
// note 5): the reason rules of FinishDecision and the cost reports are checked before the run is
// read (billing.Costs.ValidateReports, pointers under /costs); the same end state again answers
// without change and books nothing (06 section 2 "No effect"), otherwise one unit of work ends the
// run through endRun with the metrics, the summary and, for a non-empty costs, the booking: one
// clock reading is finished_at and occurred_at of every entry, billing books one entry per report
// with the copies of the run, the cost columns of the run take the sum in the currency of the
// first entry in the same update as the transition, and the session of the run adds the sums
// (agents.Sessions.AddCost). A run without billing_mode_used (it never started) and a non-empty
// costs is 400 validation_failed (invalid, /costs). An adapter that reports an I3 mismatch writes
// one audit entry in the same unit of work and counts bothy.agent.billing_mode_mismatch. After the
// commit every entry counts bothy.run.cost.
func (s *Service) Finish(ctx context.Context, cmd runs.FinishRun) (runs.Run, error) {
	req := domain.FinishRequest{Target: domain.State(cmd.State), Reason: cmd.Reason}
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
	reports := costReports(cmd.Costs)
	if len(reports) > 0 {
		if s.d.Costs == nil {
			return runs.Run{}, errs.New(errs.CodeInternalError, "runs: a finish with costs needs the billing module")
		}
		if err := s.d.Costs.ValidateReports(reports); err != nil {
			return runs.Run{}, underCosts(err)
		}
	}
	a := caller(ctx)
	var res finishResult
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		res, err = s.finishInUnit(ctx, a, cmd, req, TransitionStore{Metrics: metrics, Summary: cmd.Summary}, reports)
		return err
	})
	if err != nil {
		return runs.Run{}, err
	}
	if res.mismatch {
		s.d.Mismatches.BillingModeMismatch(context.WithoutCancel(ctx), res.handle)
	}
	return res.doc, nil
}

// finishResult is the outcome of finishInUnit: the run document and, for an I3 mismatch the
// adapter reported, the agent handle of the counter.
type finishResult struct {
	doc      runs.Run
	mismatch bool
	handle   string
}

// finishInUnit is the unit of work of Finish: lock and decide, then book, end the run with the
// cost columns, add the session sums, register the cost metric and audit a reported mismatch.
func (s *Service) finishInUnit(ctx context.Context, a actor.Actor, cmd runs.FinishRun, req domain.FinishRequest, store TransitionStore,
	reports []billing.CostReport) (finishResult, error) {
	row, err := s.lockAgentRun(ctx, a, cmd.RunID)
	if err != nil {
		return finishResult{}, err
	}
	out, err := domain.FinishDecision(row.State, req)
	if err != nil {
		return finishResult{}, finishError(err)
	}
	if out.Kind == domain.FinishNoChange {
		doc, err := s.documentOf(ctx, row)
		return finishResult{doc: doc}, err
	}
	reason := cmd.Reason
	if req.Target == domain.StateCompleted {
		reason = ""
	}
	store.At = s.now()
	booking, err := s.book(ctx, row, store.At, reports)
	if err != nil {
		return finishResult{}, err
	}
	if len(booking.Sums) > 0 {
		first := booking.Sums[0]
		store.Cost = &CostColumns{Micros: first.AmountMicros, Currency: first.Currency, IsEstimate: true}
	}
	t, err := s.endRun(ctx, row, req.Target, reason, domain.SetterAdapter, store)
	if err != nil {
		return finishResult{}, err
	}
	if err := s.addSessionCosts(ctx, row, booking); err != nil {
		return finishResult{}, err
	}
	s.countCosts(ctx, t.AgentHandle, row.BillingModeUsed, booking)
	var res finishResult
	if out.Reason == domain.RunReasonBillingModeMismatch || out.Reason == domain.RunReasonToolClassMismatch {
		if err := s.auditReportedMismatch(ctx, a, row, out.Reason); err != nil {
			return finishResult{}, err
		}
		res.mismatch, res.handle = true, t.AgentHandle
	}
	res.doc, err = s.documentByID(ctx, row.ID)
	return res, err
}

// book books the reports of a finishing run at time at inside its unit of work (M2-22 design note
// 5 b, d and e): nothing without reports; a run without billing_mode_used is 400 invalid on
// /costs, because a runtime starts only after run.start and cost_entries.billing_mode needs the
// actual mode (ruling D-M2-22-5); otherwise billing books with the copies of the run and the
// channel and space of its thread now.
func (s *Service) book(ctx context.Context, row RunRow, at time.Time, reports []billing.CostReport) (billing.Booking, error) {
	if len(reports) == 0 {
		return billing.Booking{}, nil
	}
	if row.BillingModeUsed == "" {
		return billing.Booking{}, fieldError("/costs", "invalid", "a run that never started reports no costs")
	}
	info, err := s.d.Threads.Info(ctx, row.ThreadID)
	if err != nil {
		return billing.Booking{}, err
	}
	b, err := s.d.Costs.BookRun(ctx, billing.BookRun{
		RunID: row.ID, AgentID: row.AgentID, ThreadID: row.ThreadID, ChannelID: info.ChannelID, SpaceID: info.SpaceID,
		AgentSessionID: row.AgentSessionID, BillingMode: row.BillingModeUsed, OccurredAt: at, Reports: reports,
	})
	if err != nil {
		return billing.Booking{}, underCosts(err)
	}
	return b, nil
}

// addSessionCosts adds the sums of a booking to the session of the run, one call per currency,
// in the unit of work after the run row (03 section 6.3; M2-22 design note 5 g).
func (s *Service) addSessionCosts(ctx context.Context, row RunRow, b billing.Booking) error {
	if row.AgentSessionID.IsZero() || len(b.Sums) == 0 {
		return nil
	}
	if s.d.Sessions == nil {
		return errs.New(errs.CodeInternalError, "runs: a run with a session books without the sessions of agents")
	}
	for _, sum := range b.Sums {
		if err := s.d.Sessions.AddCost(ctx, agents.AddSessionCost{SessionID: row.AgentSessionID, AmountMicros: sum.AmountMicros,
			Currency: sum.Currency}); err != nil {
			return err
		}
	}
	return nil
}

// countCosts registers bothy.run.cost per booked entry for after the commit, never inside: a
// retried unit would count twice (M2-22 design note 5 h).
func (s *Service) countCosts(ctx context.Context, handle, mode string, b billing.Booking) {
	if len(b.Entries) == 0 {
		return
	}
	metrics := s.d.Metrics
	costs := make([]BookedCost, len(b.Entries))
	for i, e := range b.Entries {
		costs[i] = BookedCost{AgentHandle: handle, BillingMode: mode, Currency: e.Currency, AmountMicros: e.AmountMicros, IsEstimate: e.IsEstimate}
	}
	txn.AfterCommit(ctx, func() {
		for _, c := range costs {
			metrics.CostBooked(context.WithoutCancel(ctx), c)
		}
	})
}

// costReports maps the reports of the runs port to billing.
func costReports(cs []runs.CostReport) []billing.CostReport {
	if len(cs) == 0 {
		return nil
	}
	out := make([]billing.CostReport, len(cs))
	for i, c := range cs {
		out[i] = billing.CostReport{Provider: c.Provider, Model: c.Model, AmountMicros: c.AmountMicros, Currency: c.Currency,
			Tokens: billing.TokenCounts{In: c.TokensIn, Out: c.TokensOut, CacheRead: c.TokensCacheRead, CacheWrite: c.TokensCacheWrite,
				CacheWrite1h: c.TokensCacheWrite1h}}
	}
	return out
}

// underCosts prefixes the pointers of a validation error of billing with /costs, the member of
// RunFinish; other errors pass unchanged.
func underCosts(err error) error {
	e, ok := errs.As(err)
	if !ok || e.Code != errs.CodeValidationFailed || len(e.Fields) == 0 {
		return err
	}
	fields := make([]errs.FieldError, len(e.Fields))
	for i, f := range e.Fields {
		f.Pointer = "/costs" + f.Pointer
		fields[i] = f
	}
	return errs.New(errs.CodeValidationFailed, e.Detail).WithFields(fields...)
}

// finishError maps the request errors of FinishDecision to 400 validation_failed with the field
// of 06 section 5; the 409 of a transition passes unchanged.
func finishError(err error) error {
	switch {
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
// unit of work, so they see the ended run and commit with it: M2-15 disables the open components
// of the run at every end state (DisableRunComponents, ruling D-M2-15-21), then the follow-ups are
// settled by Followups.OnRunEnded (M2-11 design note 7); the delivery's denial of a queued
// invocation and runs.queued_expiry end their runs here as well.
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
	if s.fu == nil {
		return t, nil
	}
	return t, s.fu.OnRunEnded(ctx, t)
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
