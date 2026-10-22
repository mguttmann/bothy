// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"unicode/utf8"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/textcheck"
	"github.com/OWNER/bothy/services/server/internal/runs"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// The audit attributes of cancelRun (M2-09 design note 7 e).
const (
	attrStateBefore  = "state_before"
	attrImmediate    = "immediate"
	attrCancelReason = "cancel_reason"
	attrNoChange     = "no_change"
)

// cancelRequestedJSON is the payload RunCancelRequestedPayload; requested_by only for
// user_cancelled (ruling D-M2-09-13), never null.
type cancelRequestedJSON struct {
	RunID       string `json:"run_id"`
	ThreadID    string `json:"thread_id"`
	Reason      string `json:"reason"`
	RequestedBy string `json:"requested_by,omitempty"`
}

// Cancel implements runs.Runs (cancelRun, run.cancel; M2-09 design note 7): the optional reason
// is checked and stored nowhere (ruling D-M2-09-11); the run is locked, run.cancel decided with the
// starter as owner; a queued run ends at once with user_cancelled, a running or waiting run gets a
// cancel request with the job runs.cancel_timeout, a cancelled run answers without change, and a
// completed or failed run gives 409. One audit entry of the allow is written in the unit of work.
func (s *Service) Cancel(ctx context.Context, cmd runs.CancelRun) (runs.RunSummary, error) {
	if cmd.Reason != "" && (textcheck.DisplayText(cmd.Reason) != nil || utf8.RuneCountInString(cmd.Reason) > s.cfg.ReasonMaxChars) {
		return runs.RunSummary{}, fieldError("/reason", "invalid", "the reason is too long or contains characters that are not allowed")
	}
	a := caller(ctx)
	var sum runs.RunSummary
	err := s.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		sum, err = s.cancelTx(ctx, a, cmd.RunID)
		return err
	})
	if err != nil {
		return runs.RunSummary{}, err
	}
	return sum, nil
}

// cancelTx is Cancel inside its unit of work.
func (s *Service) cancelTx(ctx context.Context, a actor.Actor, id ids.ID) (runs.RunSummary, error) {
	row, found, err := s.d.Repo.LockRun(ctx, id)
	if err != nil {
		return runs.RunSummary{}, err
	}
	res := runResource(row)
	if !found {
		res = authorize.Resource{Kind: authorize.ResourceRun, ID: id, ThreadID: id}
	}
	d, err := s.d.Authorizer.Authorize(ctx, a, authorize.ActionRunCancel, res)
	if err != nil {
		return runs.RunSummary{}, err
	}
	if d.Outcome == authorize.OutcomeNotFound || (d.Allowed() && !found) {
		return runs.RunSummary{}, invisible()
	}
	if !d.Allowed() {
		return runs.RunSummary{}, d.Err()
	}
	attrs := auditing.Attrs{attrStateBefore: auditing.Code(string(row.State))}
	switch domain.CancelDecision(row.State) {
	case domain.CancelNoChange:
		attrs[attrNoChange] = auditing.Bool(true)
	case domain.CancelInvalid:
		return runs.RunSummary{}, errs.New(errs.CodeInvalidStateTransition, "an ended run cannot be cancelled")
	case domain.CancelImmediate:
		attrs[attrImmediate], attrs[attrCancelReason] = auditing.Bool(true), auditing.Code(domain.RunReasonUserCancelled)
		if err := s.requestCancel(ctx, row, domain.RunReasonUserCancelled, a.ID, true); err != nil {
			return runs.RunSummary{}, err
		}
	case domain.CancelRequest:
		attrs[attrImmediate], attrs[attrCancelReason] = auditing.Bool(false), auditing.Code(domain.RunReasonUserCancelled)
		if err := s.requestCancel(ctx, row, domain.RunReasonUserCancelled, a.ID, false); err != nil {
			return runs.RunSummary{}, err
		}
	}
	if s.d.Audit == nil {
		return runs.RunSummary{}, errs.New(errs.CodeInternalError, "runs: cancelRun has no audit recorder")
	}
	if err := s.d.Audit.Record(ctx, auditing.Entry{
		Action: string(authorize.ActionRunCancel), Decision: auditing.DecisionAllow, ResourceKind: resourceRun, ResourceID: row.ID, Attrs: attrs,
	}); err != nil {
		return runs.RunSummary{}, err
	}
	cur, _, err := s.d.Repo.Run(ctx, row.ID)
	if err != nil {
		return runs.RunSummary{}, err
	}
	return s.summarize(ctx, cur)
}

// requestCancel is the one function behind every cancellation of a run (design note 7 g): it
// appends run.cancel_requested with reason for the run's agent and then either ends the run at
// once with cancelled and that reason (end true: a queued run a person stops, a disabled agent) or
// schedules runs.cancel_timeout at now + agent.cancel_grace (a running or waiting run, a revoked
// starter). requestedBy is the person of user_cancelled and zero otherwise. The caller holds the
// row lock. M2-16 (/stop) and M8-03 (channel.read_only.set) reuse it.
func (s *Service) requestCancel(ctx context.Context, row RunRow, reason string, requestedBy ids.ID, end bool) error {
	if err := s.appendCancelRequested(ctx, row, reason, requestedBy); err != nil {
		return err
	}
	if end {
		_, err := s.endRun(ctx, row, domain.StateCancelled, reason, domain.SetterServer, TransitionStore{})
		return err
	}
	if s.d.CancelTimeouts == nil {
		return errs.New(errs.CodeInternalError, "runs: a cancel request without the scheduler of runs.cancel_timeout")
	}
	return s.d.CancelTimeouts.Schedule(ctx, row.ID, s.now().Add(s.cfg.CancelGrace))
}

// appendCancelRequested appends run.cancel_requested (05 section 6, design note 7 f): a personal
// event to the run's agent with the thread scope of now, resource run without version, receive
// action agent.events.consume and the payload RunCancelRequestedPayload.
func (s *Service) appendCancelRequested(ctx context.Context, row RunRow, reason string, requestedBy ids.ID) error {
	info, err := s.d.Threads.Info(ctx, row.ThreadID)
	if err != nil {
		return err
	}
	body := cancelRequestedJSON{RunID: row.ID.String(), ThreadID: row.ThreadID.String(), Reason: reason}
	if reason == domain.RunReasonUserCancelled && !requestedBy.IsZero() {
		body.RequestedBy = requestedBy.String()
	}
	p, err := encode(body)
	if err != nil {
		return err
	}
	kind, actorID := eventActor(ctx)
	return s.d.Events.Append(ctx, eventlog.Event{
		Type: string(authorize.EventRunCancelRequested), SchemaVersion: 1, ActorKind: kind, ActorPrincipalID: actorID,
		SpaceID: info.SpaceID, ChannelID: info.ChannelID, ThreadID: row.ThreadID, ResourceKind: resourceRun, ResourceID: row.ID,
		AudiencePrincipalID: row.AgentID, ReceiveAction: string(authorize.ActionAgentEventsConsume), Payload: p,
	})
}

// CancelTimeout is the use case of the job runs.cancel_timeout (design note 7 h) in one unit of
// work: a run that is still queued, running or waiting_input ends cancelled with cancel_timeout
// through endRun, without an actor; an ended or unknown run is left as it is, so a repeated job is
// harmless (02 12.4 rule 1). Errors: those of the database.
func (s *Service) CancelTimeout(ctx context.Context, runID ids.ID) error {
	ctx = actor.WithActor(ctx, actor.Anonymous())
	return s.d.UoW.Do(ctx, func(ctx context.Context) error {
		row, found, err := s.d.Repo.LockRun(ctx, runID)
		if err != nil || !found || !row.State.Active() {
			return err
		}
		_, err = s.endRun(ctx, row, domain.StateCancelled, domain.RunReasonCancelTimeout, domain.SetterServer, TransitionStore{})
		return err
	})
}
