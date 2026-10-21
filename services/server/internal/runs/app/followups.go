// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"log/slog"
	"slices"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/messaging"
	"github.com/OWNER/bothy/services/server/internal/runs"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// FollowupStates is the outgoing port of the follow-up states a patch reads and writes (M2-10
// design note 7; 04 section 15.4 point 3; 09 section 4.7 row "Acknowledge follow-up"). The path
// rule and the value check live in runs/domain; the rows are run_followups. Followups implements
// it with the forward-only rule and trigger_state = followup_delivered (M2-11). Both methods join
// the unit of work of ctx, in which the caller holds the row lock of the run. Errors are
// kernel/errs errors.
type FollowupStates interface {
	// States returns the states of the follow-ups of runID among messageIDs; a message that is no
	// follow-up of the run is absent from the map.
	States(ctx context.Context, runID ids.ID, messageIDs []ids.ID) (map[ids.ID]string, error)
	// SetState stores state (delivered or absorbed) for the follow-up messageID of runID.
	SetState(ctx context.Context, runID, messageID ids.ID, state string) error
}

// NoFollowups is a FollowupStates that knows no follow-up, so every follow-up op of a patch gives
// patch_target_missing. The runs use cases use it only without a follow-up repository
// (Deps.FollowupRows nil, the fakes of tests that need no follow-ups).
type NoFollowups struct{}

// Compile-time check: NoFollowups is the port.
var _ FollowupStates = NoFollowups{}

// States implements FollowupStates: no follow-ups.
func (NoFollowups) States(context.Context, ids.ID, []ids.ID) (map[ids.ID]string, error) {
	return map[ids.ID]string{}, nil
}

// SetState implements FollowupStates. It is never reached, because States knows no follow-up; a
// call is a programming error.
func (NoFollowups) SetState(context.Context, ids.ID, ids.ID, string) error {
	return errs.New(errs.CodeInternalError, "runs: no follow-up store is bound")
}

// The audit stage of a refused follow-on run (ruling D-M2-11-6).
const stageRunEnd = "run_end"

// The log line of a follow-up denial with a reason the acceptance does not map.
const msgUnknownFollowupDenial = "runs: run.followup.create denied with a reason the acceptance does not map; stored unchanged"

// Followups is the service of the follow-ups of a run (M2-11, 09 section 7, ADR 0025): the
// acceptance of a follow-up, the adapter's acknowledgement (FollowupStates), the facts and the
// refusal of a delivery of agent.followup, the end of a run with its follow-on run, the deleted
// follow-up message and the follow-ups of the run document. Only this service changes a
// follow-up state (ruling D-M2-11-1). It never opens its own unit of work: every method joins the
// unit of its caller (the acceptance of createMessage, a patch, a delivery, the end of a run, the
// deletion of a message), in which the caller holds the locks of design note 12. It is safe for
// concurrent use.
type Followups struct {
	s    *Service
	rows FollowupRepository
}

// Compile-time check: Followups is the port of the patch.
var _ FollowupStates = (*Followups)(nil)

// Followups returns the follow-up service of the use cases; nil without a follow-up repository.
func (s *Service) Followups() *Followups { return s.fu }

// mode reads the effective capability followups of agentID now (09 section 2.2, design note 15):
// the owner's supports_followups and the adapter's last persisted report; none for an unknown
// agent.
func (f *Followups) mode(ctx context.Context, agentID ids.ID) (domain.FollowupMode, error) {
	facts, err := f.s.d.Agents.RunFacts(ctx, []ids.ID{agentID})
	if err != nil {
		return "", err
	}
	a, ok := facts[agentID]
	if !ok {
		return domain.FollowupNone, nil
	}
	return domain.EffectiveFollowupMode(a.FollowupsPermitted, a.FollowupsReported), nil
}

// Accept handles a message M that the trigger decision made a follow-up of the active run run
// (design note 3), inside the unit of work of createMessage with the run row locked: a candidate
// that M does not mention is checked in filter mode first (a person without trigger gets no
// answer, R-AG-05); then run.followup.create is authorized for p in request mode, so authz audits
// a denial. A denial sets trigger_state failed with the mapped reason and writes nothing else. An
// allow inserts the row in queued, raises runs.version, sets trigger_state received and appends
// agent.followup when FollowupEventNeeded for the effective mode. The run's state, invocation_jti
// and billing mode stay. Errors: those of the collaborators.
func (f *Followups) Accept(ctx context.Context, ev messaging.MessagePosted, info chat.ThreadInfo, d domain.Decision, run RunRow, p actor.Actor) error {
	allowed, err := f.authorizeFollowup(ctx, ev, info, d, run, p)
	if err != nil || !allowed {
		return err
	}
	return f.store(ctx, ev.MessageID, run)
}

// authorizeFollowup decides run.followup.create for p (design note 3 c and d): a candidate that M
// does not mention is checked in filter mode first and a person without trigger is silent; a
// denial in request mode sets trigger_state failed with the reason of domain.Settle. It reports
// whether the follow-up is allowed.
func (f *Followups) authorizeFollowup(ctx context.Context, ev messaging.MessagePosted, info chat.ThreadInfo, d domain.Decision, run RunRow,
	p actor.Actor) (bool, error) {
	s := f.s
	res := authorize.Resource{
		Kind: authorize.ResourceThread, ID: info.ID, ChannelID: info.ChannelID, SpaceID: info.SpaceID, OwnerID: info.CreatedBy,
		Invoke: &authorize.InvokeFacts{AgentID: run.AgentID, TriggerOrigin: originHuman, MentionsAgent: d.Mentioned,
			ActiveRunAgentID: run.AgentID},
	}
	if !d.Mentioned {
		v, err := s.d.Authorizer.Authorize(authorize.Filter(ctx), p, authorize.ActionRunFollowupCreate, res)
		if err != nil || (!v.Allowed() && v.Reason == denyLevelInsufficient) {
			return false, err
		}
	}
	v, err := s.d.Authorizer.Authorize(ctx, p, authorize.ActionRunFollowupCreate, res)
	if err != nil || v.Allowed() {
		return v.Allowed(), err
	}
	if !knownDenials[string(v.Reason)] {
		s.d.Log.LogAttrs(ctx, slog.LevelError, msgUnknownFollowupDenial, slog.String(keyAction, string(authorize.ActionRunFollowupCreate)))
	}
	st, err := domain.Settle(d, domain.Verdict{Reason: string(v.Reason)})
	if err != nil {
		return false, err
	}
	return false, s.fail(ctx, ev.MessageID, string(st.Reason))
}

// store writes an allowed follow-up (design note 3 e): the row in queued, one version step of the
// run, trigger_state received and agent.followup when the effective mode asks for it.
func (f *Followups) store(ctx context.Context, message ids.ID, run RunRow) error {
	s := f.s
	if err := f.rows.InsertFollowup(ctx, run.ID, message); err != nil {
		return err
	}
	if _, err := s.d.Repo.RaiseRunVersion(ctx, run.ID); err != nil {
		return err
	}
	if err := s.d.Messages.SetTriggerState(ctx, messaging.SetTriggerStateCmd{MessageID: message, State: string(domain.TriggerStateReceived)}); err != nil {
		return err
	}
	mode, err := f.mode(ctx, run.AgentID)
	if err != nil {
		return err
	}
	if domain.FollowupEventNeeded(mode, run.State) {
		return f.emit(ctx, run.ID, run.ThreadID, run.AgentID, message)
	}
	return nil
}

// emit appends agent.followup for the follow-up message of the run (design note 4): the personal
// event of the run's agent with the receive action agent.events.consume, the resource run
// without version, the scope of the run's thread now and the payload AgentFollowupPayload, which
// names the message only by id. The token is a delivery field that Prepare adds per delivery.
func (f *Followups) emit(ctx context.Context, runID, threadID, agentID, message ids.ID) error {
	s := f.s
	info, err := s.d.Threads.Info(ctx, threadID)
	if err != nil {
		return err
	}
	p, err := domain.FollowupPayload(runID, threadID, message)
	if err != nil {
		return errs.Wrap(errs.CodeInternalError, err, "runs: the payload of agent.followup")
	}
	kind, actorID := eventActor(ctx)
	return s.d.Events.Append(ctx, eventlog.Event{
		Type: string(authorize.EventAgentFollowup), SchemaVersion: 1, ActorKind: kind, ActorPrincipalID: actorID,
		SpaceID: info.SpaceID, ChannelID: info.ChannelID, ThreadID: threadID, ResourceKind: resourceRun, ResourceID: runID,
		AudiencePrincipalID: agentID, ReceiveAction: string(authorize.ActionAgentEventsConsume), Payload: p,
	})
}

// States implements FollowupStates: the stored states of the follow-ups of runID among
// messageIDs.
func (f *Followups) States(ctx context.Context, runID ids.ID, messageIDs []ids.ID) (map[ids.ID]string, error) {
	rows, err := f.rows.FollowupStatesOf(ctx, runID, messageIDs)
	if err != nil {
		return nil, err
	}
	out := make(map[ids.ID]string, len(rows))
	for _, r := range rows {
		out[r.MessageID] = string(r.State)
	}
	return out, nil
}

// SetState implements FollowupStates (design note 5): the row is locked and moved forward only
// (domain.AdvanceFollowup); a row that first leaves queued sets trigger_state followup_delivered
// on its message, which writes message.updated. The same state is no change. Errors:
// domain.ErrFollowupTransition (409 invalid_state_transition) for every other move; internal for
// a message that is no follow-up of the run (States reported it, so a call for it is a
// programming error).
func (f *Followups) SetState(ctx context.Context, runID, messageID ids.ID, state string) error {
	row, found, err := f.rows.LockFollowup(ctx, runID, messageID)
	if err != nil {
		return err
	}
	if !found {
		return errs.New(errs.CodeInternalError, "runs: a follow-up state for a message that is no follow-up of the run")
	}
	changed, err := domain.AdvanceFollowup(row.State, domain.FollowupState(state))
	if err != nil || !changed {
		return err
	}
	if err := f.rows.SetFollowupStates(ctx, runID, []ids.ID{messageID}, domain.FollowupState(state)); err != nil {
		return err
	}
	if row.State != domain.FollowupQueued {
		return nil
	}
	return f.s.d.Messages.SetTriggerState(ctx, messaging.SetTriggerStateCmd{MessageID: messageID, State: string(domain.TriggerStateFollowupDelivered)})
}

// ForDelivery reports whether the follow-up messageID of the locked run runID may be delivered
// (design note 6, ruling D-M2-11-4): its row exists for this run in queued or delivered and its
// message is not deleted. Otherwise the delivery sends nothing, without check, token or audit.
func (f *Followups) ForDelivery(ctx context.Context, runID, messageID ids.ID) (bool, error) {
	rows, err := f.rows.FollowupStatesOf(ctx, runID, []ids.ID{messageID})
	if err != nil || len(rows) != 1 || !rows[0].State.Open() {
		return false, err
	}
	m, err := f.s.d.Messages.TriggerMessageOf(ctx, messageID)
	if err != nil {
		return false, err
	}
	return !m.Deleted, nil
}

// RefuseDelivery settles a follow-up whose delivery agent.invoke refused (design note 6): the row
// becomes discarded and runs.version rises by one; the delivery already set trigger_state failed
// with the reason. The caller holds the row lock of the run.
func (f *Followups) RefuseDelivery(ctx context.Context, runID, messageID ids.ID) error {
	if err := f.rows.SetFollowupStates(ctx, runID, []ids.ID{messageID}, domain.FollowupDiscarded); err != nil {
		return err
	}
	_, err := f.s.d.Repo.RaiseRunVersion(ctx, runID)
	return err
}

// Document returns the follow-ups of runID for its run document (design note 10), in ascending
// message id; never nil.
func (f *Followups) Document(ctx context.Context, runID ids.ID) ([]runs.Followup, error) {
	if f == nil {
		return []runs.Followup{}, nil
	}
	rows, err := f.rows.RunFollowups(ctx, runID)
	if err != nil {
		return nil, err
	}
	out := make([]runs.Followup, 0, len(rows))
	for _, r := range rows {
		out = append(out, runs.Followup{MessageID: r.MessageID, State: string(r.State)})
	}
	return out, nil
}

// OnMessageDeleted discards the open follow-up of a deleted message (design note 9, ruling
// D-M2-11-7) in the deleting unit of work: the run row is locked, then the follow-up row; an open
// row becomes discarded and runs.version rises by one. The tombstone keeps its trigger_state. A
// message that is no open follow-up is nothing. A row that moved to a follow-on run between the
// read and the lock is read again.
func (f *Followups) OnMessageDeleted(ctx context.Context, ev messaging.MessageDeleted) error {
	for range 3 {
		row, found, err := f.rows.FollowupOfMessage(ctx, ev.MessageID)
		if err != nil || !found || !row.State.Open() {
			return err
		}
		if _, _, err := f.s.d.Repo.LockRun(ctx, row.RunID); err != nil {
			return err
		}
		locked, found, err := f.rows.LockFollowup(ctx, row.RunID, ev.MessageID)
		if err != nil {
			return err
		}
		if !found {
			continue
		}
		if !locked.State.Open() {
			return nil
		}
		return f.RefuseDelivery(ctx, row.RunID, ev.MessageID)
	}
	return errs.New(errs.CodeConflict, "runs: the follow-up of a deleted message keeps moving")
}

// OnRunEnded settles the open follow-ups of the run that ended as t (design note 7), in the unit
// of work of the end with the run row locked: the open rows are locked in ascending message_id
// and split by domain.PlanRunEnd. A deleted message is discarded without a trigger_state change;
// after a stop by a person the rest is discarded with trigger_state cancelled / user_cancelled;
// after every other end the rest becomes deferred and the first allowed of them, in thread order,
// triggers the follow-on run (followOn). No further version step: the changes belong to the
// stored change of the final transition.
func (f *Followups) OnRunEnded(ctx context.Context, t Transitioned) error {
	open, err := f.rows.LockOpenFollowups(ctx, t.Run.ID)
	if err != nil || len(open) == 0 {
		return err
	}
	facts := make(map[ids.ID]messaging.TriggerMessage, len(open))
	in := make([]domain.OpenFollowup, 0, len(open))
	for _, o := range open {
		m, err := f.s.d.Messages.TriggerMessageOf(ctx, o.MessageID)
		if err != nil {
			return err
		}
		facts[o.MessageID] = m
		in = append(in, domain.OpenFollowup{MessageID: o.MessageID, ThreadSeq: m.ThreadSeq, State: o.State, Deleted: m.Deleted})
	}
	plan, err := domain.PlanRunEnd(domain.RunEnd{State: t.To, Reason: t.Run.StateReason}, in)
	if err != nil {
		return errs.Wrap(errs.CodeInternalError, err, "runs: the follow-ups of an ended run")
	}
	if err := f.rows.SetFollowupStates(ctx, t.Run.ID, append(slices.Clone(plan.Deleted), plan.Stopped...), domain.FollowupDiscarded); err != nil {
		return err
	}
	for _, m := range plan.Stopped {
		if err := f.s.d.Messages.SetTriggerState(ctx, messaging.SetTriggerStateCmd{
			MessageID: m, State: string(domain.TriggerStateCancelled), Reason: domain.RunReasonUserCancelled,
		}); err != nil {
			return err
		}
	}
	if len(plan.Candidates) == 0 {
		return nil
	}
	if err := f.rows.SetFollowupStates(ctx, t.Run.ID, plan.Candidates, domain.FollowupDeferred); err != nil {
		return err
	}
	return f.followOn(ctx, t.Run, plan.Candidates, facts)
}

// followOn creates the follow-on run of the deferred candidates of the ended run ended (design
// note 8): each candidate in order is checked as a trigger with agent.invoke for its author (the
// re-check principal, filter mode at the pinned permission version, no active run); a refused one
// is discarded with trigger_state failed and an audit entry of stage run_end; the first allowed
// one triggers a new run (Open, agent.invocation) and stays deferred on the ended run; the
// candidates after it move to the new run in queued with trigger_state received and, when
// FollowupEventNeeded for the mode now, get agent.followup after the new run's agent.invocation.
func (f *Followups) followOn(ctx context.Context, ended runs.RunSummary, cands []ids.ID, facts map[ids.ID]messaging.TriggerMessage) error {
	s := f.s
	info, err := s.d.Threads.Info(ctx, ended.ThreadID)
	if err != nil {
		return err
	}
	version, err := s.d.Versions.PermissionVersion(ctx)
	if err != nil {
		return err
	}
	agents, err := s.d.Agents.RunFacts(ctx, []ids.ID{ended.AgentID})
	if err != nil {
		return err
	}
	agent := agents[ended.AgentID]
	for i, m := range cands {
		msg := facts[m]
		allowed, err := f.checkTrigger(ctx, ended, info, msg, version, agent.Status == agentActive)
		if err != nil {
			return err
		}
		if !allowed {
			continue
		}
		// A job or a delivery ends runs without a principal; the anchor of the follow-on run is then
		// written by the run's agent, the same principal as after finishRun (chat refuses a caller
		// other than the principal of the unit).
		if a, ok := actor.FromContext(ctx); !ok || a.ID.IsZero() {
			ctx = actor.WithActor(ctx, actor.Actor{ID: ended.AgentID, Kind: actor.KindAgent})
		}
		sum, err := s.Open(ctx, OpenCmd{
			ThreadID: ended.ThreadID, AgentID: ended.AgentID, TriggeredBy: msg.Author.ID, TriggeringMessageID: m,
			TriggerKind: triggerKindMessage, BillingModeExpected: agent.BillingMode,
		})
		if err != nil {
			// The ended run is no longer active and the acceptances of the thread wait at the run
			// row lock, so an active run here is a programming error (design note 8 f).
			return errs.Wrap(errs.CodeInternalError, err, "runs: the follow-on run")
		}
		if err := s.emitInvocation(ctx, sum, m, agent.BillingMode); err != nil {
			return err
		}
		return f.moveRest(ctx, ended.ID, sum, cands[i+1:])
	}
	return nil
}

// checkTrigger decides agent.invoke for the candidate msg of the follow-on run of ended (design
// note 8 a and b) and applies a denial: the row becomes discarded, trigger_state failed with the
// delivery's reason mapping, one audit entry with stage run_end.
func (f *Followups) checkTrigger(ctx context.Context, ended runs.RunSummary, info chat.ThreadInfo, msg messaging.TriggerMessage,
	version int64, agentActive bool) (bool, error) {
	s := f.s
	r := authorize.Resource{
		Kind: authorize.ResourceThread, ID: ended.ThreadID, ChannelID: info.ChannelID, SpaceID: info.SpaceID, OwnerID: info.CreatedBy,
		Invoke: &authorize.InvokeFacts{AgentID: ended.AgentID, TriggerOrigin: msg.Origin, MentionsAgent: slices.Contains(msg.MentionedIDs, ended.AgentID)},
	}
	d, err := s.d.Authorizer.Authorize(authorize.WithPermissionVersion(authorize.Filter(ctx), version), actor.Recheck(msg.Author.ID),
		authorize.ActionAgentInvoke, r)
	if err != nil || d.Allowed() {
		return d.Allowed(), err
	}
	if err := f.rows.SetFollowupStates(ctx, ended.ID, []ids.ID{msg.ID}, domain.FollowupDiscarded); err != nil {
		return false, err
	}
	reason := domain.DeliveryReason(string(d.Reason), agentActive)
	if err := s.d.Messages.SetTriggerState(ctx, messaging.SetTriggerStateCmd{
		MessageID: msg.ID, State: string(domain.TriggerStateFailed), Reason: reason,
	}); err != nil {
		return false, err
	}
	return false, s.auditDenial(ctx, msg.Author.ID, string(d.Reason), ended.ThreadID, auditing.Attrs{
		attrRunID: auditing.ID(ended.ID), attrMessageID: auditing.ID(msg.ID), attrAgentID: auditing.ID(ended.AgentID),
		attrStage: auditing.Code(stageRunEnd), attrTriggerKind: auditing.Code(triggerKindMessage),
	})
}

// moveRest moves the remaining candidates of the ended run to the follow-on run sum (design note
// 8 d, ruling D-M2-11-6) and appends their agent.followup in thread order when the mode asks for
// it.
func (f *Followups) moveRest(ctx context.Context, endedID ids.ID, sum runs.RunSummary, rest []ids.ID) error {
	if len(rest) == 0 {
		return nil
	}
	if err := f.rows.MoveFollowups(ctx, endedID, sum.ID, rest); err != nil {
		return err
	}
	for _, m := range rest {
		if err := f.s.d.Messages.SetTriggerState(ctx, messaging.SetTriggerStateCmd{MessageID: m, State: string(domain.TriggerStateReceived)}); err != nil {
			return err
		}
	}
	mode, err := f.mode(ctx, sum.AgentID)
	if err != nil || !domain.FollowupEventNeeded(mode, domain.StateQueued) {
		return err
	}
	for _, m := range rest {
		if err := f.emit(ctx, sum.ID, sum.ThreadID, sum.AgentID, m); err != nil {
			return err
		}
	}
	return nil
}
