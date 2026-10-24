// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/messaging"
	"github.com/OWNER/bothy/services/server/internal/runs"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// The trigger kinds of 03 runs.trigger_kind that Open accepts.
const (
	triggerKindMessage = "message"
	triggerKindCommand = "command"
)

// The step of a replaced adapter reason (ruling D-M2-05-8, 09 5.2).
const (
	stepKindStatus        = "status"
	stepNameAdapterReason = "adapter_reason"
	stepStatusFailed      = "failed"
)

// OpenCmd is the input of Open: a new run in queued.
type OpenCmd struct {
	// ThreadID is the target thread; AgentID the agent; TriggeredBy the person who triggered;
	// TriggeringMessageID the message whose trigger_state follows the run (zero for none).
	ThreadID, AgentID, TriggeredBy, TriggeringMessageID ids.ID
	// TriggeringCommandID is the command of a command run (M2-16); zero otherwise.
	TriggeringCommandID ids.ID
	// TriggeringInteractionID is the interaction of an interaction run (M2-15); zero otherwise.
	TriggeringInteractionID ids.ID
	// TriggerKind is message, command or interaction: message needs no command and no
	// interaction, command needs TriggeringCommandID and interaction TriggeringInteractionID, each
	// without a message.
	TriggerKind string
	// BillingModeExpected is the agent's billing mode now (ruling D-M2-05-5).
	BillingModeExpected string
	// InvocationHeldUntil is the release time of a held agent.invocation, stored in
	// runs.invocation_held_until (R-DM-44, M3-07); zero: not held (NULL).
	InvocationHeldUntil time.Time
}

// openTrigger reports a trigger Open accepts: kind message without a command or an interaction,
// kind command with its command, or kind interaction with its interaction (M2-15), each of the two
// without a message and without the object of the other (03 runs, the check on the triggering_*
// columns).
func openTrigger(cmd OpenCmd) bool {
	switch cmd.TriggerKind {
	case triggerKindMessage:
		return cmd.TriggeringCommandID.IsZero() && cmd.TriggeringInteractionID.IsZero()
	case triggerKindCommand:
		return !cmd.TriggeringCommandID.IsZero() && cmd.TriggeringMessageID.IsZero() && cmd.TriggeringInteractionID.IsZero()
	case triggerKindInteraction:
		return !cmd.TriggeringInteractionID.IsZero() && cmd.TriggeringMessageID.IsZero() && cmd.TriggeringCommandID.IsZero()
	}
	return false
}

// ActiveRunExistsError is the error of Open when the thread already has an active run (R-DM-14):
// nothing was written. Active is that run.
type ActiveRunExistsError struct {
	Active runs.RunSummary
}

// Error implements error.
func (e *ActiveRunExistsError) Error() string { return "runs: the thread has an active run" }

// Unwrap gives the conflict error the HTTP chain renders, should it ever reach a response.
func (e *ActiveRunExistsError) Unwrap() error {
	return errs.New(errs.CodeConflict, "the thread has an active run")
}

// ConcurrentChangeError is the error of Transition when the run is no longer in the expected state
// and version: another unit changed it first (RK-18). Current is the run now. Its default mapping
// is 409 invalid_state_transition; M2-09 maps a lost start race to already_started or to the
// replay of the same token.
type ConcurrentChangeError struct {
	Current runs.RunSummary
}

// Error implements error.
func (e *ConcurrentChangeError) Error() string { return "runs: the run changed concurrently" }

// Unwrap gives the default mapping 409 invalid_state_transition.
func (e *ConcurrentChangeError) Unwrap() error {
	return errs.New(errs.CodeInvalidStateTransition, "the run is no longer in the expected state")
}

// TransitionCmd is the input of Transition: the compare-and-swap of 03 runs.
type TransitionCmd struct {
	RunID ids.ID
	// From and Version are the state and version the caller read.
	From    domain.State
	Version int64
	// To is the new state; Reason the reason of failed and cancelled (09 5.2).
	To     domain.State
	Reason string
	// SetBy is the setter of the reason: the server, or the adapter (failed with the reason rules
	// of 09 5.2, cancelled with a reason of a cancel request, M2-09 design note 6 c).
	SetBy domain.Setter
	// Store are the columns the same swap stores (M2-09); the zero value stores none.
	Store TransitionStore
}

// TransitionStore are the columns a transition of M2-09 stores with the swap: the start columns,
// the status line of a wait, the metrics and the summary of an end. Zero values keep the column.
type TransitionStore struct {
	// StartedJTI is the jti of the start token; it also clears renewed_from_jti.
	StartedJTI ids.ID
	// BillingModeUsed, Model, StatusLine and Summary are "" to keep.
	BillingModeUsed, Model, StatusLine, Summary string
	// BillingEvidence and Metrics are encoded JSON objects; nil keeps.
	BillingEvidence, Metrics []byte
	// Cost are the cost columns of a finish that booked costs (M2-22); nil keeps them.
	Cost *CostColumns
	// At is the time of the change; zero means now. The finish that books costs passes the one
	// clock reading that is also occurred_at of its cost entries (ruling D-M2-22-6).
	At time.Time
}

// CostColumns are runs.cost_micros, runs.currency and runs.cost_is_estimate as a finish that
// booked costs stores them: the sum of the new entries in the currency of the first entry, an
// estimate (ruling D-M2-22-7).
type CostColumns struct {
	Micros     int64
	Currency   string
	IsEstimate bool
}

// Open writes a new run in queued in the caller's unit of work (brief note 5): the run row (ON
// CONFLICT on the active run of the thread DO NOTHING), the live box anchor in the run's thread,
// run.state_changed, with a triggering message trigger_state received, and the job
// runs.queued_expiry at queued_at + run.queued_max_age (M2-08). The lock order is
// runs before messages; the caller (the acceptance) holds the thread row of the message.
//
// Errors: *ActiveRunExistsError when the thread has an active run (nothing written); internal
// outside a unit of work or for an unknown trigger kind; the errors of the collaborators.
//
// Example:
//
//	sum, err := svc.Open(ctx, app.OpenCmd{ThreadID: t, AgentID: lotse, TriggeredBy: alex, TriggeringMessageID: m,
//		TriggerKind: "message", BillingModeExpected: "organization"})
func (s *Service) Open(ctx context.Context, cmd OpenCmd) (runs.RunSummary, error) {
	if err := needUnit(ctx, "Open"); err != nil {
		return runs.RunSummary{}, err
	}
	if !openTrigger(cmd) || cmd.ThreadID.IsZero() || cmd.AgentID.IsZero() || cmd.TriggeredBy.IsZero() {
		return runs.RunSummary{}, errs.New(errs.CodeInternalError,
			"runs: Open needs a thread, an agent, a person and a trigger of kind message, command or interaction with its object")
	}
	row, inserted, err := s.d.Repo.InsertRun(ctx, NewRun{
		ID: ids.New(), ThreadID: cmd.ThreadID, AgentID: cmd.AgentID, TriggeredBy: cmd.TriggeredBy, TriggerKind: cmd.TriggerKind,
		TriggeringMessageID: cmd.TriggeringMessageID, TriggeringCommandID: cmd.TriggeringCommandID,
		TriggeringInteractionID: cmd.TriggeringInteractionID,
		BillingModeExpected:     cmd.BillingModeExpected, InvocationJTI: ids.New(), QueuedAt: s.now(),
		InvocationHeldUntil: cmd.InvocationHeldUntil,
	})
	if err != nil {
		return runs.RunSummary{}, err
	}
	if !inserted {
		return runs.RunSummary{}, s.activeExists(ctx, cmd.ThreadID)
	}
	if _, err := s.d.Messages.CreateRunAnchor(ctx, messaging.RunAnchorCmd{ThreadID: row.ThreadID, RunID: row.ID, AgentID: row.AgentID}); err != nil {
		return runs.RunSummary{}, err
	}
	info, err := s.d.Threads.Info(ctx, row.ThreadID)
	if err != nil {
		return runs.RunSummary{}, err
	}
	sum := summary(row, info.ChannelID, 0)
	change := domain.TriggerOnOpen()
	trigger := domain.TriggerStateNone
	if !row.TriggeringMessageID.IsZero() {
		trigger = change.State
	}
	if err := s.emitStateChanged(ctx, info.SpaceID, sum, row.TriggeringMessageID, trigger); err != nil {
		return runs.RunSummary{}, err
	}
	if trigger != domain.TriggerStateNone {
		if err := s.d.Messages.SetTriggerState(ctx, messaging.SetTriggerStateCmd{MessageID: row.TriggeringMessageID, State: string(change.State)}); err != nil {
			return runs.RunSummary{}, err
		}
	}
	// The run fails with agent_unavailable when nobody starts it within run.queued_max_age
	// (M2-08 design note 11); every run of this creation function gets the job.
	if err := s.scheduleExpiry(ctx, row); err != nil {
		return runs.RunSummary{}, err
	}
	return sum, nil
}

// activeExists returns the ActiveRunExistsError of a thread whose insert met the partial unique
// index: the active run is read in a new statement, which sees the committed winner.
func (s *Service) activeExists(ctx context.Context, threadID ids.ID) error {
	active, found, err := s.d.Repo.ActiveRunOfThread(ctx, threadID)
	if err != nil {
		return err
	}
	if !found {
		// The winner ended between the two statements; the caller may decide again.
		return errs.New(errs.CodeConflict, "runs: the active run of the thread changed concurrently")
	}
	sum, err := s.summarize(ctx, active)
	if err != nil {
		return err
	}
	return &ActiveRunExistsError{Active: sum}
}

// summarize returns the summary of one row with the channel of its thread now and its steps.
func (s *Service) summarize(ctx context.Context, r RunRow) (runs.RunSummary, error) {
	info, err := s.d.Threads.Info(ctx, r.ThreadID)
	if err != nil {
		return runs.RunSummary{}, err
	}
	counts, err := s.d.Repo.StepCounts(ctx, []ids.ID{r.ID})
	if err != nil {
		return runs.RunSummary{}, err
	}
	return summary(r, info.ChannelID, counts[r.ID]), nil
}

// Transition changes the state of a run in the caller's unit of work (brief note 5): the pair must
// be one of 03 runs, the reason one of 09 5.2 for its setter, then one compare-and-swap UPDATE on
// state and version, the status step of a replaced adapter reason, run.state_changed and the
// trigger_state of the triggering message as 09 6 maps it. The metrics are recorded after the
// commit of the outermost unit (txn.AfterCommit), so a vetoed unit records nothing.
//
// Errors: 409 invalid_state_transition for a pair 03 does not allow (nothing written);
// domain.ErrReasonRequired for an adapter failed without a reason; not_found for an unknown run;
// *ConcurrentChangeError when state or version no longer match; internal outside a unit of work or
// for a reason that does not fit the state or its setter.
//
// Example:
//
//	t, err := svc.Transition(ctx, app.TransitionCmd{RunID: id, From: domain.StateQueued, Version: 1,
//		To: domain.StateRunning, SetBy: domain.SetterAdapter})
func (s *Service) Transition(ctx context.Context, cmd TransitionCmd) (Transitioned, error) {
	if err := needUnit(ctx, "Transition"); err != nil {
		return Transitioned{}, err
	}
	if !domain.CanTransition(cmd.From, cmd.To) {
		return Transitioned{}, errs.New(errs.CodeInvalidStateTransition, fmt.Sprintf("a run cannot go from %s to %s", cmd.From, cmd.To))
	}
	reason, original, err := s.reasonOf(cmd)
	if err != nil {
		return Transitioned{}, err
	}
	at := cmd.Store.At
	if at.IsZero() {
		at = s.now()
	}
	row, changed, err := s.d.Repo.TransitionRun(ctx, TransitionWrite{
		ID: cmd.RunID, From: cmd.From, Version: cmd.Version, To: cmd.To, Reason: reason, At: at,
		StartedJTI: cmd.Store.StartedJTI, BillingModeUsed: cmd.Store.BillingModeUsed, Model: cmd.Store.Model,
		StatusLine: cmd.Store.StatusLine, Summary: cmd.Store.Summary, BillingEvidence: cmd.Store.BillingEvidence,
		Metrics: cmd.Store.Metrics, Cost: cmd.Store.Cost,
	})
	if err != nil {
		return Transitioned{}, err
	}
	if !changed {
		return Transitioned{}, s.lostSwap(ctx, cmd.RunID)
	}
	if original != "" {
		detail, err := encode(domain.OriginalReasonDetail(original, s.cfg.StepDetailMaxBytes))
		if err != nil {
			return Transitioned{}, err
		}
		if _, err := s.d.Repo.AppendStep(ctx, StepRow{
			ID: ids.New(), RunID: row.ID, Kind: stepKindStatus, Name: stepNameAdapterReason, Status: stepStatusFailed,
			Detail: detail, StartedAt: at, FinishedAt: at,
		}); err != nil {
			return Transitioned{}, err
		}
	}
	return s.afterTransition(ctx, cmd.From, row)
}

// reasonOf validates the reason of cmd for its setter and returns the stored reason and, for a
// replaced adapter reason, the original code.
func (s *Service) reasonOf(cmd TransitionCmd) (reason, original string, err error) {
	switch {
	case cmd.SetBy == domain.SetterServer:
		return cmd.Reason, "", domain.ValidateServerReason(cmd.To, cmd.Reason)
	case cmd.SetBy == domain.SetterAdapter && cmd.To == domain.StateFailed:
		return domain.NormalizeAdapterReason(cmd.Reason)
	case cmd.SetBy == domain.SetterAdapter && cmd.Reason == "" && !cmd.To.Ended():
		// running and waiting_input carry no reason (09 5.2).
		return "", "", nil
	case cmd.SetBy == domain.SetterAdapter && cmd.To == domain.StateCompleted && cmd.Reason == "":
		return "", "", nil
	case cmd.SetBy == domain.SetterAdapter && cmd.To == domain.StateCancelled && slices.Contains(domain.CancelReasons(), cmd.Reason):
		// The adapter confirms a cancel request with its reason (M2-09 design note 6 c).
		return cmd.Reason, "", nil
	}
	return "", "", errs.New(errs.CodeInternalError, "runs: no reason rule for this setter and state")
}

// lostSwap tells a missing run from a concurrent change after a compare-and-swap without effect.
func (s *Service) lostSwap(ctx context.Context, id ids.ID) error {
	cur, found, err := s.d.Repo.Run(ctx, id)
	if err != nil {
		return err
	}
	if !found {
		return invisible()
	}
	sum, err := s.summarize(ctx, cur)
	if err != nil {
		return err
	}
	return &ConcurrentChangeError{Current: sum}
}

// afterTransition appends run.state_changed and the trigger state of the changed row, registers the
// metrics for after the commit and returns the result.
func (s *Service) afterTransition(ctx context.Context, from domain.State, row RunRow) (Transitioned, error) {
	info, err := s.d.Threads.Info(ctx, row.ThreadID)
	if err != nil {
		return Transitioned{}, err
	}
	counts, err := s.d.Repo.StepCounts(ctx, []ids.ID{row.ID})
	if err != nil {
		return Transitioned{}, err
	}
	sum := summary(row, info.ChannelID, counts[row.ID])
	change, ok := domain.TriggerOnTransition(from, row.State, row.StateReason)
	trigger := domain.TriggerStateNone
	if ok && !row.TriggeringMessageID.IsZero() {
		trigger = change.State
	}
	if err := s.emitStateChanged(ctx, info.SpaceID, sum, row.TriggeringMessageID, trigger); err != nil {
		return Transitioned{}, err
	}
	if trigger != domain.TriggerStateNone {
		if err := s.d.Messages.SetTriggerState(ctx, messaging.SetTriggerStateCmd{
			MessageID: row.TriggeringMessageID, State: string(change.State), Reason: change.Reason,
		}); err != nil {
			return Transitioned{}, err
		}
	}
	t := Transitioned{Run: sum, From: from, To: row.State, Started: !row.StartedAt.IsZero()}
	if from == domain.StateQueued {
		t.QueuedFor = durationBetween(row.QueuedAt, s.changedAt(row))
	}
	if row.State.Ended() && t.Started {
		t.Ran = durationBetween(row.StartedAt, row.FinishedAt)
	}
	if t.AgentHandle, err = s.handleOf(ctx, row.AgentID); err != nil {
		return Transitioned{}, err
	}
	metrics, rec := s.d.Metrics, s.d.Measure
	txn.AfterCommit(ctx, func() {
		metrics.RunTransitioned(context.WithoutCancel(ctx), t)
		measureTransition(rec, t)
	})
	return t, nil
}

// changedAt is the time the row left queued: started_at for a start, finished_at for an end.
func (s *Service) changedAt(row RunRow) time.Time {
	if row.State == domain.StateRunning && !row.StartedAt.IsZero() {
		return row.StartedAt
	}
	if !row.FinishedAt.IsZero() {
		return row.FinishedAt
	}
	return s.now()
}

// durationBetween returns b - a, never negative.
func durationBetween(a, b time.Time) time.Duration {
	if a.IsZero() || b.IsZero() || b.Before(a) {
		return 0
	}
	return b.Sub(a)
}

// handleOf returns the handle of the agent for bothy.agent.handle; "" when it is unknown.
func (s *Service) handleOf(ctx context.Context, agentID ids.ID) (string, error) {
	facts, err := s.d.Agents.RunFacts(ctx, []ids.ID{agentID})
	if err != nil {
		return "", err
	}
	return facts[agentID].Handle, nil
}
