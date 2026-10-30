// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"log/slog"
	"sync/atomic"

	"github.com/OWNER/bothy/services/server/internal/agents"
	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/messaging"
	"github.com/OWNER/bothy/services/server/internal/runs"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// The values of MessagePosted that can trigger (09 section 3.2).
const (
	originHuman = "human"
	originAgent = "agent"
	kindText    = "text"
	kindVoice   = "voice"
	kindRun     = "run"
	kindSystem  = "system"
	channelDM   = "dm"
	agentActive = "active"
)

// The deny reasons of 07 4.1 the acceptance handles itself (the values of kernel/auditing.Reason).
const (
	denyLevelInsufficient = "level_insufficient"
)

// knownDenials are the deny reasons of agent.invoke that 09 3.5 and 05 section 5 name; any other
// reason is stored unchanged and logged (brief note 6 step 5).
var knownDenials = map[string]bool{
	"level_insufficient": true, "no_grant": true, "agent_not_reachable": true, "billing_mode_personal_not_holder": true,
	"tool_class_owner_only": true, "project_not_ready": true, "dev_environment_not_ready": true, "run_active_other_agent": true,
}

// The log lines of the acceptance; they never carry IDs or contents.
const (
	msgTriggerDecisionMismatch = "runs: trigger_decision_mismatch: the trigger decision rejected what agent.invoke allowed; failing closed"
	msgUnknownDenial           = "runs: agent.invoke denied with a reason the acceptance does not map; stored unchanged"
	keyReason                  = "bothy.reason"
	keyAction                  = "bothy.action"
	reasonTriggerMismatch      = "trigger_decision_mismatch"
)

// Acceptor is the in-transaction subscriber of messaging.MessagePosted (02 section 4.3, brief note
// 6). internal/wire creates it with the module objects of the txn registry, before the runs use
// cases exist, and the start binds it to them (Bind). Until then it writes nothing, so a test
// world built without the runs module keeps the behaviour of M1 (D-M2-05-19).
type Acceptor struct {
	svc atomic.Pointer[Service]
}

// NewAcceptor returns an unbound acceptor.
func NewAcceptor() *Acceptor { return &Acceptor{} }

// Bind sets the use cases the acceptor runs. Errors: a nil service.
func (a *Acceptor) Bind(s *Service) error {
	if s == nil {
		return errors.New("runs: Acceptor.Bind needs the runs use cases")
	}
	a.svc.Store(s)
	return nil
}

// OnMessagePosted implements the acceptance of 09 3.4 point 7 inside the transaction of the
// message (brief note 6). Business outcomes never veto; only infrastructure errors return an
// error, which rolls the message back (02 section 8.3).
func (a *Acceptor) OnMessagePosted(ctx context.Context, ev messaging.MessagePosted) error {
	s := a.svc.Load()
	if s == nil {
		return nil
	}
	return s.accept(ctx, ev)
}

// OnMessageDeleted handles a deleted message inside the deleting unit of work: it discards the
// open follow-up of the message (M2-11 design note 9, ruling D-M2-11-7), then deletes the
// components of a deleted card (M2-15, deleteComponentsOf), in the lock order runs before
// message_components; unbound it writes nothing. Only infrastructure errors return an error.
func (a *Acceptor) OnMessageDeleted(ctx context.Context, ev messaging.MessageDeleted) error {
	s := a.svc.Load()
	if s == nil {
		return nil
	}
	if s.fu != nil {
		if err := s.fu.OnMessageDeleted(ctx, ev); err != nil {
			return err
		}
	}
	return s.deleteComponentsOf(ctx, ev.MessageID)
}

// accept is the acceptance of one new message. The owner's switch supports_files_out comes before
// every other branch (M3-07); then the answer check of M2-19 (design note 2 f): an agent message
// that names a run is checked and may veto the unit; then every message whose origin is not human,
// or whose kind is neither text nor voice (M3-09), returns without trigger work (AG-01, 07 SR4).
func (s *Service) accept(ctx context.Context, ev messaging.MessagePosted) error {
	if err := s.checkFilesOut(ctx, ev); err != nil {
		return err
	}
	if err := s.answer(ctx, ev); err != nil {
		return err
	}
	if ev.Origin != originHuman || (ev.Kind != kindText && ev.Kind != kindVoice) {
		return nil
	}
	info, err := s.d.Threads.Info(ctx, ev.ThreadID)
	if err != nil {
		return err
	}
	if info.ChannelKind == channelDM {
		return nil
	}
	in, active, err := s.trigger(ctx, ev, info)
	if err != nil {
		return err
	}
	// Two rounds at most: the second re-decides with the run another path opened in between.
	for range 2 {
		next, err := s.settle(ctx, ev, info, in, active)
		if err != nil || next == nil {
			return err
		}
		active = next.run
		in.ActiveRun = nil
		if active != nil {
			in.ActiveRun = &domain.ActiveRun{RunID: active.ID, AgentID: active.AgentID}
		}
	}
	return nil
}

// answer is the first branch of accept: an agent's answer that names a run in run_id is checked
// (M2-19); every other message passes.
func (s *Service) answer(ctx context.Context, ev messaging.MessagePosted) error {
	if ev.RunID == nil || ev.Kind == kindRun {
		return nil
	}
	if ev.Origin != originAgent {
		// messaging never builds this event; a run_id of another origin is a defect.
		return errs.New(errs.CodeInternalError, "runs: a run_id on a message that is not an agent's answer")
	}
	if ev.Kind == kindSystem {
		return nil
	}
	return s.checkAnswer(ctx, ev)
}

// redecide asks settle's caller for another round with run as the locked active run of the
// thread (nil: none).
type redecide struct{ run *RunRow }

// trigger gathers the facts of the decision through the ports: thread and channel from chat, the
// agents from agents, the active run of the thread from this module. The active run is read with
// its row lock (M2-11 design note 3 a), so it stays active until the commit: a follow-up is never
// written to a run that ended meanwhile; the end of the run waits for this unit. A voice message
// adds the facts of 09 section 13 (M3-09 design note 12 b): Voice, the transcription mode of its
// author from files, and VoiceInput of every agent (in the facts of the one RunFacts read); a text
// message makes no extra call.
func (s *Service) trigger(ctx context.Context, ev messaging.MessagePosted, info chat.ThreadInfo) (domain.MessageTrigger, *RunRow, error) {
	ch, err := s.d.Channels.TriggerChannel(ctx, info.ChannelID)
	if err != nil {
		return domain.MessageTrigger{}, nil, err
	}
	want := append([]ids.ID{}, ev.MentionedIDs...)
	if !info.OperatorAgentID.IsZero() {
		want = append(want, info.OperatorAgentID)
	}
	if !ch.EffectiveOperatorID.IsZero() {
		want = append(want, ch.EffectiveOperatorID)
	}
	facts, err := s.d.Agents.RunFacts(ctx, want)
	if err != nil {
		return domain.MessageTrigger{}, nil, err
	}
	in := domain.MessageTrigger{
		MessageID: ev.MessageID, ThreadID: ev.ThreadID, ChannelID: info.ChannelID, Origin: domain.Origin(ev.Origin),
		ChannelKind: domain.ChannelKind(info.ChannelKind), ThreadKind: domain.ThreadKind(info.Kind),
		Switches: domain.Switches{
			RequireMention: ch.RequireMention, ThreadRequireMention: ch.ThreadRequireMention, FreeResponse: ch.FreeResponse,
			AutoThread: ch.AutoThread, AllowMentionedAgents: ch.AllowMentionedAgents,
		},
		// The trigger right and the agent's own write level are decided by agent.invoke (filter
		// mode first for a candidate without mention, then request mode), which also writes the
		// audit of a denial; the domain therefore assumes both and authz is the truth (D-M2-05-20).
		AuthorHasTrigger: true, VoiceMode: domain.VoiceOff,
	}
	if ev.Kind == kindVoice {
		in.Voice = true
		if in.VoiceMode, err = s.voiceMode(ctx, ev); err != nil {
			return domain.MessageTrigger{}, nil, err
		}
	}
	if info.Kind == string(domain.ThreadThread) && !info.OperatorAgentID.IsZero() {
		in.ThreadOperator = agentFacts(facts, info.OperatorAgentID)
	}
	if !ch.EffectiveOperatorID.IsZero() {
		in.ChannelOperator = agentFacts(facts, ch.EffectiveOperatorID)
	}
	for _, id := range ev.MentionedIDs {
		if _, ok := facts[id]; ok {
			in.Mentioned = append(in.Mentioned, *agentFacts(facts, id))
		}
	}
	locked, err := s.activeFacts(ctx, &in, info, ch.EffectiveOperatorID)
	if err != nil {
		return domain.MessageTrigger{}, nil, err
	}
	return in, locked, nil
}

// activeFacts adds the facts that depend on the runs of the thread to in: the active run, read
// with its row lock (returned, nil for none), and whether the fixed thread operator is outside
// the channel whose effective operator is channelOp (09 3.4 step 4a).
func (s *Service) activeFacts(ctx context.Context, in *domain.MessageTrigger, info chat.ThreadInfo, channelOp ids.ID) (*RunRow, error) {
	active, found, err := s.d.Repo.LockActiveRunOfThread(ctx, info.ID)
	if err != nil {
		return nil, err
	}
	var locked *RunRow
	if found {
		in.ActiveRun = &domain.ActiveRun{RunID: active.ID, AgentID: active.AgentID}
		locked = &active
	}
	if in.ThreadOperator != nil {
		if in.ThreadOperatorOutside, err = s.operatorOutside(ctx, info, channelOp, locked); err != nil {
			return nil, err
		}
	}
	return locked, nil
}

// operatorOutside reports whether the fixed operator of the thread info is no longer allowed in
// its channel (09 3.4 step 4a, addendum of 2026-10-30): it is not the channel's effective operator
// channelOp, the thread's active run (nil: none) is not its run, it has no write of its own on the
// channel (a grant: thread.create on the channel decided for the agent in filter mode, which the
// operator rule of the thread does not reach, 07 2.9), and it never completed a run in the
// thread. The check and the read run only when the first two do not decide.
func (s *Service) operatorOutside(ctx context.Context, info chat.ThreadInfo, channelOp ids.ID, active *RunRow) (bool, error) {
	op := info.OperatorAgentID
	if op.IsZero() || op == channelOp || (active != nil && active.AgentID == op) {
		return false, nil
	}
	v, err := s.d.Authorizer.Authorize(authorize.Filter(ctx), actor.Actor{ID: op, Kind: actor.KindAgent}, authorize.ActionThreadCreate,
		authorize.Resource{Kind: authorize.ResourceChannel, ID: info.ChannelID})
	if err != nil || v.Allowed() {
		return false, err
	}
	worked, err := s.d.Repo.CompletedRunOfAgent(ctx, info.ID, op)
	return !worked, err
}

// voiceMode returns the transcription mode of the author of the voice message ev (12 8.2): the
// effective mode of files, mode off without the port (Deps.Transcripts).
func (s *Service) voiceMode(ctx context.Context, ev messaging.MessagePosted) (domain.VoiceMode, error) {
	if s.d.Transcripts == nil {
		return domain.VoiceOff, nil
	}
	m, err := s.d.Transcripts.Mode(ctx, ev.AuthorID, ev.AuthorKind)
	if err != nil {
		return "", err
	}
	// Decide refuses a value outside off, agent and service as invalid input (D-M2-04-5).
	return domain.VoiceMode(m), nil
}

// agentFacts converts the agent facts of id; an agent unknown to agents is inactive and of
// organization mode, which the guard refuses as not reachable. VoiceInput is the effective
// feature voice_input (owner's supports_voice and the adapter's report, 09 2.2); Decide reads it
// only for a voice message.
func agentFacts(facts map[ids.ID]agents.RunFacts, id ids.ID) *domain.AgentFacts {
	f, ok := facts[id]
	mode := domain.BillingMode(f.BillingMode)
	if !ok || (mode != domain.BillingPersonal && mode != domain.BillingOrganization) {
		mode = domain.BillingOrganization
	}
	return &domain.AgentFacts{ID: id, Active: ok && f.Status == agentActive, BillingMode: mode, WriteOnChannel: true,
		VoiceInput: ok && domain.EffectiveVoiceInput(f.VoicePermitted, f.VoiceReported)}
}

// candidate is the agent a decision names and whether M mentions it.
type candidate struct {
	facts     domain.AgentFacts
	mentioned bool
}

// candidateOf returns the agent a decision names: the candidate of an invocation, follow-up or
// rejection by the active run, or the one mentioned agent of agent_not_allowed_here. ok is false
// for no trigger and for multiple_agents_mentioned.
func candidateOf(in domain.MessageTrigger, d domain.Decision) (candidate, bool, error) {
	if d.Agent != nil {
		return candidate{facts: *d.Agent, mentioned: d.Mentioned}, true, nil
	}
	if d.Outcome != domain.OutcomeReject || d.Reason == domain.ReasonMultipleAgentsMentioned {
		return candidate{}, false, nil
	}
	if d.Reason == domain.ReasonAgentNotAllowedHere && len(in.Mentioned) > 0 {
		return candidate{facts: in.Mentioned[0], mentioned: true}, true, nil
	}
	if d.Reason == domain.ReasonOperatorNotInChannel && in.ThreadOperator != nil {
		// The fixed operator of the thread (09 3.4 step 4a), mentioned or by the mention rule.
		return candidate{facts: *in.ThreadOperator, mentioned: mentions(in, in.ThreadOperator.ID)}, true, nil
	}
	// run_active_other_agent and voice_unsupported: the candidate as if the thread had no active
	// run and the message were text (the voice rule rejects before the target, 09 3.4).
	free := in
	free.ActiveRun, free.Voice = nil, false
	fd, err := domain.Decide(free)
	if err != nil || fd.Agent == nil {
		return candidate{}, false, err
	}
	return candidate{facts: *fd.Agent, mentioned: fd.Mentioned}, true, nil
}

// mentions reports whether the message of in mentions the agent id.
func mentions(in domain.MessageTrigger, id ids.ID) bool {
	for _, m := range in.Mentioned {
		if m.ID == id {
			return true
		}
	}
	return false
}

// settle runs the decision and step 7 for in (brief note 6 steps 3 to 7) with active, the locked
// active run of the thread (nil for none). It returns another round when another path opened a
// run in the target meanwhile, and nil otherwise.
func (s *Service) settle(ctx context.Context, ev messaging.MessagePosted, info chat.ThreadInfo, in domain.MessageTrigger,
	active *RunRow) (*redecide, error) {
	d, err := domain.Decide(in)
	if err != nil {
		return nil, err
	}
	p := authorOf(ctx, ev)
	switch {
	case d.Outcome == domain.OutcomeNone:
		return nil, nil
	case d.Outcome == domain.OutcomeFollowUp:
		return nil, s.followup(ctx, ev, info, d, active, p)
	case d.Outcome == domain.OutcomeReject && d.Reason == domain.ReasonMultipleAgentsMentioned:
		// No single agent to check: no authorize and no audit (D-M2-05-12).
		return nil, s.fail(ctx, ev.MessageID, string(d.Reason))
	}
	cand, ok, err := candidateOf(in, d)
	if err != nil || !ok {
		return nil, err
	}
	res := s.invokeResource(info, in, cand)
	if silent, err := s.withoutTriggerRight(ctx, p, res, cand); err != nil || silent {
		return nil, err
	}
	if d.Outcome == domain.OutcomeReject && (d.Reason == domain.ReasonVoiceUnsupported || d.Reason == domain.ReasonOperatorNotInChannel) {
		// A rejection of the decision function (09 3.5, M3-09 design note 12 c, and 3.4 step 4a):
		// trigger_state failed without agent.invoke in request mode, so no audit entry; a person
		// without trigger who did not mention the agent stays unanswered above (R-AG-05).
		return nil, s.fail(ctx, ev.MessageID, string(d.Reason))
	}
	v, err := s.d.Authorizer.Authorize(ctx, p, authorize.ActionAgentInvoke, res)
	if err != nil {
		return nil, err
	}
	if !v.Allowed() {
		return nil, s.denied(ctx, ev.MessageID, cand, string(v.Reason))
	}
	if d.Outcome == domain.OutcomeReject {
		// The decision and authorize disagree: fail closed (D-M2-05-14).
		s.d.Log.LogAttrs(ctx, slog.LevelError, msgTriggerDecisionMismatch,
			slog.String(keyAction, string(authorize.ActionAgentInvoke)), slog.String(keyReason, reasonTriggerMismatch))
		return nil, s.fail(ctx, ev.MessageID, string(d.Reason))
	}
	return s.invoke(ctx, ev, info, d, p)
}

// authorOf is the principal of the author of ev: the actor of the request when it is the author,
// else the author by id and kind.
func authorOf(ctx context.Context, ev messaging.MessagePosted) actor.Actor {
	if a, ok := actor.FromContext(ctx); ok && a.ID == ev.AuthorID {
		return a
	}
	return actor.Actor{ID: ev.AuthorID, Kind: actor.Kind(ev.AuthorKind)}
}

// followup hands a follow-up decision to the follow-up service (M2-11 design note 3); without it
// nothing is written (D-M2-05-12). The decision's run must be the locked active run.
func (s *Service) followup(ctx context.Context, ev messaging.MessagePosted, info chat.ThreadInfo, d domain.Decision, active *RunRow, p actor.Actor) error {
	if s.fu == nil {
		return nil
	}
	if active == nil || active.ID != d.RunID {
		return errs.New(errs.CodeInternalError, "runs: a follow-up decision without the locked active run")
	}
	return s.fu.Accept(ctx, ev, info, d, *active, p)
}

// withoutTriggerRight checks a candidate that M does not mention in filter mode first: a person
// without trigger gets no answer and no audit entry (R-AG-05, D-M2-05-13). It reports true when
// the acceptance writes nothing.
func (s *Service) withoutTriggerRight(ctx context.Context, p actor.Actor, res authorize.Resource, c candidate) (bool, error) {
	if c.mentioned {
		return false, nil
	}
	v, err := s.d.Authorizer.Authorize(authorize.Filter(ctx), p, authorize.ActionAgentInvoke, res)
	if err != nil {
		return false, err
	}
	return !v.Allowed() && v.Reason == denyLevelInsufficient, nil
}

// invokeResource is the descriptor of agent.invoke: always the thread of M, also when the target
// is a new auto thread (D-M2-05-2, D-M2-04-4), with the active run of that thread.
func (s *Service) invokeResource(info chat.ThreadInfo, in domain.MessageTrigger, c candidate) authorize.Resource {
	facts := &authorize.InvokeFacts{AgentID: c.facts.ID, TriggerOrigin: originHuman, MentionsAgent: c.mentioned}
	if in.ActiveRun != nil && (info.Kind != string(domain.ThreadMain) || !in.Switches.AutoThread) {
		facts.ActiveRunAgentID = in.ActiveRun.AgentID
	}
	return authorize.Resource{
		Kind: authorize.ResourceThread, ID: info.ID, ChannelID: info.ChannelID, SpaceID: info.SpaceID, OwnerID: info.CreatedBy,
		Invoke: facts,
	}
}

// denied stores the trigger state of a denial of agent.invoke with the reason a person sees.
func (s *Service) denied(ctx context.Context, message ids.ID, c candidate, reason string) error {
	if !knownDenials[reason] {
		s.d.Log.LogAttrs(ctx, slog.LevelError, msgUnknownDenial, slog.String(keyAction, string(authorize.ActionAgentInvoke)))
	}
	st, err := domain.Settle(domain.Decision{
		Outcome: domain.OutcomeInvoke, TriggerKind: domain.TriggerKindMessage, Agent: &c.facts, Check: domain.CheckInvoke,
	}, domain.Verdict{Reason: reason})
	if err != nil {
		return err
	}
	return s.fail(ctx, message, string(st.Reason))
}

// fail sets trigger_state failed with reason on the message.
func (s *Service) fail(ctx context.Context, message ids.ID, reason string) error {
	return s.d.Messages.SetTriggerState(ctx, messaging.SetTriggerStateCmd{MessageID: message, State: string(domain.TriggerStateFailed), Reason: reason})
}

// invoke is step 7 on allow: the auto thread, then Open in the target with the invocation or its
// hold (invokeOrHold, M3-07). When another path opened a
// run in the target meanwhile, it returns that run, locked, for another round of the decision.
func (s *Service) invoke(ctx context.Context, ev messaging.MessagePosted, info chat.ThreadInfo, d domain.Decision, p actor.Actor) (*redecide, error) {
	target := info.ID
	if d.NewThread != nil {
		th, err := s.d.Threads.Create(ctx, p, chat.CreateThreadCmd{
			ChannelID: info.ChannelID, Parent: &chat.ParentMessage{MessageID: ev.MessageID, ThreadID: info.ID},
		})
		if err != nil {
			return nil, err
		}
		target = th.ID
	}
	facts, err := s.d.Agents.RunFacts(ctx, []ids.ID{d.Agent.ID})
	if err != nil {
		return nil, err
	}
	mode := facts[d.Agent.ID].BillingMode
	if mode == "" {
		mode = string(d.Agent.BillingMode)
	}
	_, err = s.invokeOrHold(ctx, OpenCmd{
		ThreadID: target, AgentID: d.Agent.ID, TriggeredBy: ev.AuthorID, TriggeringMessageID: ev.MessageID,
		TriggerKind: triggerKindMessage, BillingModeExpected: mode,
	}, ev.HasAttachments)
	var exists *ActiveRunExistsError
	if errors.As(err, &exists) && d.NewThread == nil {
		// The winner is locked like the run of trigger; one that ended meanwhile leaves the thread
		// without an active run for the next round.
		row, found, err := s.d.Repo.LockRun(ctx, exists.Active.ID)
		if err != nil {
			return nil, err
		}
		if !found || !row.State.Active() {
			return &redecide{}, nil
		}
		return &redecide{run: &row}, nil
	}
	return nil, err
}

// emitInvocation appends agent.invocation as the last write of the acceptance (M2-08 design note
// 14): audience the agent, receive action agent.events.consume, resource the run without version,
// the scope of the run's thread and the payload AgentInvocationPayload without a token. The token
// is a delivery field that Prepare adds per delivery (R-SY-07, R-SY-17). released marks the
// release of a held invocation (ReleaseInvocation, M3-07): its envelope has no actor and no
// correlation_id even when ctx carries them, because the writer is a job or a finalizing write
// without principal (04 section 2.2 row actor); the trigger stays in triggered_by. The payload
// carries the session of the run (M2-12 design note 9), decided after Open, so the run row is
// locked before the session rows (03 6.3).
func (s *Service) emitInvocation(ctx context.Context, sum runs.RunSummary, message ids.ID, mode string, released bool) error {
	info, err := s.d.Threads.Info(ctx, sum.ThreadID)
	if err != nil {
		return err
	}
	m, err := s.d.Messages.TriggerMessageOf(ctx, message)
	if err != nil {
		return err
	}
	ch, err := s.d.Channels.TriggerChannel(ctx, info.ChannelID)
	if err != nil {
		return err
	}
	session, err := s.invocationSession(ctx, sum, operatorOf(info, ch))
	if err != nil {
		return err
	}
	p, err := domain.InvocationPayload(domain.InvocationPayloadInput{
		RunID: sum.ID, ThreadID: sum.ThreadID, ChannelID: info.ChannelID, SpaceID: info.SpaceID, MessageID: message,
		TriggeredBy: domain.PrincipalRefPayload{
			ID: m.Author.ID.String(), Kind: m.Author.Kind, Handle: m.Author.Handle, DisplayName: m.Author.DisplayName,
			AvatarFileID: optionalID(m.Author.AvatarFileID),
		},
		BillingMode: mode, SettingsVersion: ch.SettingsVersion,
	}, session)
	if err != nil {
		return errs.Wrap(errs.CodeInternalError, err, "runs: the payload of agent.invocation")
	}
	kind, actorID := eventActor(ctx)
	if released {
		kind, actorID = "", ids.ID{}
		ctx = s.uncorrelated(ctx)
	}
	return s.d.Events.Append(ctx, eventlog.Event{
		Type: string(authorize.EventAgentInvocation), SchemaVersion: 1, ActorKind: kind, ActorPrincipalID: actorID,
		SpaceID: info.SpaceID, ChannelID: info.ChannelID, ThreadID: sum.ThreadID, ResourceKind: resourceRun, ResourceID: sum.ID,
		AudiencePrincipalID: sum.AgentID, ReceiveAction: string(authorize.ActionAgentEventsConsume), Payload: p,
	})
}
