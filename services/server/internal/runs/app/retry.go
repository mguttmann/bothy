// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"slices"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/runs"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// Retry implements runs.Runs (retryRun, 09 5.7, addendum of 2026-10-30) in one unit of work: the
// run must be visible (run.read in filter mode, else 404), the newest run of its thread, of
// trigger kind message and failed or cancelled (409 invalid_state_transition otherwise); a
// second retry of the same run finds the first retry as the newest run of the thread for the same
// message and answers it without a write. Then the rules of triggering: a fixed thread operator
// outside its channel is refused with operator_not_in_channel (09 3.4 step 4a), agent.invoke is
// decided in request mode for the caller (authz audits a denial), and the run opens like an
// acceptance (invokeOrHold), with the caller as triggered_by.
func (s *Service) Retry(ctx context.Context, cmd runs.RetryRun) (runs.RunSummary, error) {
	a := caller(ctx)
	var sum runs.RunSummary
	err := s.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		sum, err = s.retryTx(ctx, a, cmd.RunID)
		return err
	})
	if err != nil {
		return runs.RunSummary{}, err
	}
	return sum, nil
}

// retryTx is Retry inside its unit of work. The lock order is that of the acceptance and of model
// commands: the thread, then the runs (03 section 6.3).
func (s *Service) retryTx(ctx context.Context, a actor.Actor, id ids.ID) (runs.RunSummary, error) {
	row, err := s.lockRetryRun(ctx, a, id)
	if err != nil {
		return runs.RunSummary{}, err
	}
	replay, err := s.retryReplay(ctx, row)
	if err != nil {
		return runs.RunSummary{}, err
	}
	m, err := s.retryCheck(ctx, a, row)
	if err != nil {
		return runs.RunSummary{}, err
	}
	if replay != nil {
		// The earlier retry of the same message: answer it, write nothing (idempotent). The check
		// above ran anyway, so the answer has the same rights as the retry itself.
		return s.summarize(ctx, *replay)
	}
	facts, err := s.d.Agents.RunFacts(ctx, []ids.ID{row.AgentID})
	if err != nil {
		return runs.RunSummary{}, err
	}
	mode := facts[row.AgentID].BillingMode
	if mode == "" {
		mode = row.BillingModeExpected
	}
	return s.invokeOrHold(ctx, OpenCmd{
		ThreadID: row.ThreadID, AgentID: row.AgentID, TriggeredBy: a.ID, TriggeringMessageID: m,
		TriggerKind: triggerKindMessage, BillingModeExpected: mode,
	}, true)
}

// lockRetryRun reads the run id for a retry: unknown or not readable for a (run.read in filter
// mode) is 404; then the thread is locked for a message change and the run row with it.
func (s *Service) lockRetryRun(ctx context.Context, a actor.Actor, id ids.ID) (RunRow, error) {
	row, found, err := s.d.Repo.Run(ctx, id)
	if err != nil || !found {
		return RunRow{}, cmpErr(err, invisible())
	}
	v, err := s.d.Authorizer.Authorize(authorize.Filter(ctx), a, authorize.ActionRunRead, runResource(row))
	if err != nil || !v.Allowed() {
		return RunRow{}, cmpErr(err, invisible())
	}
	if err := s.d.Threads.LockForMessageChange(ctx, row.ThreadID); err != nil {
		return RunRow{}, err
	}
	row, found, err = s.d.Repo.LockRun(ctx, id)
	if err != nil || !found {
		return RunRow{}, cmpErr(err, invisible())
	}
	return row, nil
}

// cmpErr returns err when it is set and otherwise fallback.
func cmpErr(err, fallback error) error {
	if err != nil {
		return err
	}
	return fallback
}

// retryReplay applies the state rules of 09 5.7 to the locked row: a message run, the newest run
// of its thread unless the newest is the earlier retry of the same message and agent (returned
// as the replay), and failed or cancelled; otherwise 409 invalid_state_transition.
func (s *Service) retryReplay(ctx context.Context, row RunRow) (*RunRow, error) {
	if row.TriggerKind != triggerKindMessage || row.TriggeringMessageID.IsZero() {
		return nil, notRetryable("only a run triggered by a message can be retried")
	}
	newest, err := s.d.Repo.ThreadRuns(ctx, ThreadRunsRead{ThreadID: row.ThreadID, Limit: 1})
	if err != nil {
		return nil, err
	}
	if len(newest) == 1 && newest[0].ID != row.ID {
		if newest[0].TriggeringMessageID != row.TriggeringMessageID || newest[0].AgentID != row.AgentID {
			return nil, notRetryable("only the newest run of a thread can be retried")
		}
		return &newest[0], nil
	}
	if row.State != domain.StateFailed && row.State != domain.StateCancelled {
		return nil, notRetryable("only a failed or cancelled run can be retried")
	}
	return nil, nil
}

// retryCheck applies the rules of triggering to a retry of row and returns its triggering
// message: the message must still exist, a fixed thread operator outside its channel is refused
// with operator_not_in_channel (09 3.4 step 4a), and agent.invoke is decided in request mode for
// the caller (authz audits a denial).
func (s *Service) retryCheck(ctx context.Context, a actor.Actor, row RunRow) (ids.ID, error) {
	m, err := s.d.Messages.TriggerMessageOf(ctx, row.TriggeringMessageID)
	if err != nil {
		return ids.ID{}, err
	}
	if m.Deleted {
		return ids.ID{}, notRetryable("the triggering message was deleted")
	}
	info, err := s.d.Threads.Info(ctx, row.ThreadID)
	if err != nil {
		return ids.ID{}, err
	}
	ch, err := s.d.Channels.TriggerChannel(ctx, info.ChannelID)
	if err != nil {
		return ids.ID{}, err
	}
	if info.Kind == string(domain.ThreadThread) && info.OperatorAgentID == row.AgentID {
		outside, err := s.operatorOutside(ctx, info, ch.EffectiveOperatorID, nil)
		if err != nil {
			return ids.ID{}, err
		}
		if outside {
			return ids.ID{}, invocationDenied(string(domain.ReasonOperatorNotInChannel))
		}
	}
	res := authorize.Resource{
		Kind: authorize.ResourceThread, ID: row.ThreadID, ChannelID: info.ChannelID, SpaceID: info.SpaceID, OwnerID: info.CreatedBy,
		Invoke: &authorize.InvokeFacts{
			AgentID: row.AgentID, TriggerOrigin: originOf(a), MentionsAgent: slices.Contains(m.MentionedIDs, row.AgentID),
			// A run in the auto thread of a main-thread message keeps the main-thread mention rule (M2-08).
			NewThread: m.ThreadID != row.ThreadID,
		},
	}
	d, err := s.d.Authorizer.Authorize(ctx, a, authorize.ActionAgentInvoke, res)
	if err != nil {
		return ids.ID{}, err
	}
	if !d.Allowed() {
		return ids.ID{}, mapInvocationDenial(d)
	}
	return m.ID, nil
}

// notRetryable is 409 invalid_state_transition with detail.
func notRetryable(detail string) error {
	return errs.New(errs.CodeInvalidStateTransition, detail)
}
