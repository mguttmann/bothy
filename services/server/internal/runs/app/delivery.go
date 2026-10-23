// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"log/slog"
	"slices"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/reqmeta"
	"github.com/OWNER/bothy/services/server/internal/messaging"
	"github.com/OWNER/bothy/services/server/internal/runs"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// TokenIssuer issues invocation tokens (InvocationTokens.Issue, M2-06).
type TokenIssuer interface {
	Issue(ctx context.Context, g domain.InvocationGrant) (IssuedInvocationToken, error)
}

// PermissionVersions reads the current permission version (instance.Versions), which a delivery
// check pins (M2-08 design note 3 d).
type PermissionVersions interface {
	PermissionVersion(ctx context.Context) (int64, error)
}

// AuditRecorder writes the audit entry of a denial in the unit of work of the delivery
// (auditing.Recorder.Record; the unit commits, so RecordDenial does not apply).
type AuditRecorder interface {
	Record(ctx context.Context, e auditing.Entry) error
}

// DeliveryTelemetry is the outgoing port of the delivery signals (signals.md): the counter
// bothy.agent.invocation.denied and the span agent.dispatch. adapters/otelmetrics implements it;
// nil records nothing.
type DeliveryTelemetry interface {
	// Denied counts one denial before a delivery with the agent handle and the mapped reason.
	Denied(ctx context.Context, agentHandle, reason string)
	// StartDispatch opens the span agent.dispatch of one checked delivery; end closes it with the
	// decision (allow or deny).
	StartDispatch(ctx context.Context, runID ids.ID, agentHandle string) (context.Context, func(decision string))
}

// The values of bothy.decision on agent.dispatch.
const (
	decisionAllow = "allow"
	decisionDeny  = "deny"
)

// The audit attributes of a denial (M2-08 design note 8).
const (
	attrRunID       = "run_id"
	attrMessageID   = "message_id"
	attrAgentID     = "agent_id"
	attrStage       = "stage"
	attrEventType   = "event_type"
	attrTriggerKind = "trigger_kind"
	stageDelivery   = "delivery"
	stageAcceptance = "acceptance"
	resourceThread  = "thread"
)

// The log lines of the delivery; they never carry IDs, a token or a jti.
const (
	msgNoResolver = "runs: no trigger resolver for this invocation-bearing event; nothing is sent"
	keyEventType  = "bothy.event_type"
)

// The trigger state a delivery reads and sets.
const triggerStateReceived = "received"

// trigger is what a resolver reads for one delivery (M2-08 design note 5).
type trigger struct {
	// person is the triggering human; object the triggering object of the token.
	person, object ids.ID
	// message is the message whose trigger_state a denial sets; zero for none. human reports its
	// origin human (only such a message carries a trigger state, 03 messages).
	message ids.ID
	human   bool
	// origin and mentions are the facts of agent.invoke; newThread marks the auto thread.
	origin    string
	mentions  bool
	newThread bool
	// state is the stored trigger_state of message.
	state string
	// interaction is the interaction of interaction.created whose state the delivery sets
	// (dispatched on allow, failed on denial; zero for every other event, M2-15).
	interaction ids.ID
	// facts are the facts of the component for the check at stage deliver (M2-15 design note 8 c);
	// nil when the trigger is no interaction or its interaction was deleted.
	facts *authorize.InteractionFacts
	// gone reports an interaction.created whose interaction no longer exists (its card was
	// deleted): nothing is sent, without a check or an audit entry (design note 11 a).
	gone bool
}

// resolver reads the trigger of one delivery of the locked run.
type resolver func(s *Service, ctx context.Context, run RunRow, in runs.DeliveryInput) (trigger, error)

// resolvers is the table of design note 5: one entry per event type, for agent.invocation one per
// runs.trigger_kind; the trigger kind command and command.invoked come with M2-16
// (delivery_command.go; command.invoked has no run and takes its own path, prepareInvoked);
// interaction.created and the trigger kind interaction come with M2-15.
var resolvers = map[string]map[string]resolver{
	domain.EventAgentInvocation: {triggerKindMessage: resolveInvocationMessage, triggerKindCommand: resolveInvocationCommand,
		triggerKindInteraction: resolveInvocationInteraction},
	domain.EventAgentFollowup:      {"": resolveFollowup},
	domain.EventCommandInvoked:     {},
	domain.EventInteractionCreated: {"": resolveInteraction},
}

// resolveInteraction is interaction.created (M2-15 design note 11 a): the trigger is the person of
// the interaction, the triggering object the interaction, the component's facts are read now for
// stage deliver. An interaction that no longer exists (its card was deleted) is gone.
func resolveInteraction(s *Service, ctx context.Context, _ RunRow, in runs.DeliveryInput) (trigger, error) {
	ixID, _, err := domain.InteractionDeliveryIDs(in.Payload)
	if err != nil {
		return trigger{}, errs.Wrap(errs.CodeInternalError, err, "runs: the payload of interaction.created")
	}
	if s.d.Components == nil {
		return trigger{gone: true}, nil
	}
	ix, found, err := s.d.Components.Interaction(ctx, ixID)
	if err != nil {
		return trigger{}, err
	}
	if !found {
		return trigger{gone: true}, nil
	}
	f, found, err := s.deliverFacts(ctx, ix.ComponentID)
	if err != nil {
		return trigger{}, err
	}
	if !found {
		return trigger{gone: true}, nil
	}
	return trigger{person: ix.PrincipalID, object: ix.ID, origin: originHuman, interaction: ix.ID, facts: f}, nil
}

// resolveInvocationInteraction is agent.invocation with trigger_kind interaction (M2-15 design
// note 11 b): the trigger is runs.triggered_by, the triggering object runs.triggering_interaction_id
// (or, once the card was deleted and the column is NULL, the interaction_id of the stored payload),
// the facts of the interaction's component at stage deliver; without its interaction the check
// runs without them (points 1 to 5, 7 and 8 still re-check the person). Without any interaction id
// the event cannot carry a valid token and is gone.
func resolveInvocationInteraction(s *Service, ctx context.Context, run RunRow, in runs.DeliveryInput) (trigger, error) {
	object := run.TriggeringInteractionID
	if object.IsZero() {
		id, err := domain.InvocationInteractionID(in.Payload)
		if err != nil {
			return trigger{}, errs.Wrap(errs.CodeInternalError, err, "runs: the payload of agent.invocation")
		}
		object = id
	}
	if object.IsZero() {
		return trigger{gone: true}, nil
	}
	tr := trigger{person: run.TriggeredBy, object: object, origin: originHuman}
	if s.d.Components == nil || run.TriggeringInteractionID.IsZero() {
		return tr, nil
	}
	ix, found, err := s.d.Components.Interaction(ctx, run.TriggeringInteractionID)
	if err != nil || !found {
		return tr, err
	}
	f, found, err := s.deliverFacts(ctx, ix.ComponentID)
	if err != nil || !found {
		return tr, err
	}
	tr.facts = f
	return tr, nil
}

// deliverFacts reads the facts of a component for the check at stage deliver: state, capability,
// restriction and the starter of its run, read now (the rights of the moment decide, design note
// 8 c). found is false for a component that is gone.
func (s *Service) deliverFacts(ctx context.Context, componentID ids.ID) (*authorize.InteractionFacts, bool, error) {
	comp, found, err := s.d.Components.Component(ctx, componentID)
	if err != nil || !found {
		return nil, found, err
	}
	f := &authorize.InteractionFacts{
		Stage: authorize.InteractionStageDeliver, OwnerKind: ownerKindAgent, ComponentState: comp.State, ExpiresAt: comp.ExpiresAt,
		RequiredCapability: comp.RequiredCapability, RestrictedTo: comp.RestrictedTo, ComponentRunID: comp.RunID,
	}
	if !comp.RunID.IsZero() {
		r, found, err := s.d.Repo.Run(ctx, comp.RunID)
		if err != nil {
			return nil, false, err
		}
		if found {
			f.ComponentRunActive, f.RunStarterID = r.State.Active(), r.TriggeredBy
		}
	}
	return f, true, nil
}

// resolveInvocationMessage is agent.invocation with trigger_kind message: the run's trigger and
// its triggering message now; the run's thread is the auto thread when the message lives in
// another thread (09 3.7).
func resolveInvocationMessage(s *Service, ctx context.Context, run RunRow, _ runs.DeliveryInput) (trigger, error) {
	m, err := s.d.Messages.TriggerMessageOf(ctx, run.TriggeringMessageID)
	if err != nil {
		return trigger{}, err
	}
	return trigger{
		person: run.TriggeredBy, object: m.ID, message: m.ID, human: m.Origin == originHuman, origin: m.Origin,
		mentions: slices.Contains(m.MentionedIDs, run.AgentID), newThread: m.ThreadID != run.ThreadID, state: m.TriggerState,
	}, nil
}

// resolveFollowup is agent.followup: the author of the follow-up message of the payload.
func resolveFollowup(s *Service, ctx context.Context, run RunRow, in runs.DeliveryInput) (trigger, error) {
	id, err := domain.FollowupMessageID(in.Payload)
	if err != nil {
		return trigger{}, errs.Wrap(errs.CodeInternalError, err, "runs: the payload of agent.followup")
	}
	m, err := s.d.Messages.TriggerMessageOf(ctx, id)
	if err != nil {
		return trigger{}, err
	}
	return trigger{
		person: m.Author.ID, object: m.ID, message: m.ID, human: m.Origin == originHuman, origin: m.Origin,
		mentions: slices.Contains(m.MentionedIDs, run.AgentID), state: m.TriggerState,
	}, nil
}

// resolverFor returns the resolver of a delivery of typ for run; nil when none is registered.
func resolverFor(typ string, run RunRow) resolver {
	byKind := resolvers[typ]
	if byKind == nil {
		return nil
	}
	if typ == domain.EventAgentInvocation {
		return byKind[run.TriggerKind]
	}
	return byKind[""]
}

// tokenBox holds the bound issuer.
type tokenBox struct{ issuer TokenIssuer }

// BindTokens sets the issuer of the delivery tokens; internal/wire calls it in the signing keys
// step, after the chat step built the use cases. Errors: a nil issuer.
func (s *Service) BindTokens(t TokenIssuer) error {
	if t == nil {
		return errors.New("runs: BindTokens needs the invocation tokens")
	}
	s.tokens.Store(&tokenBox{issuer: t})
	return nil
}

// Compile-time check: Service is the delivery port.
var _ runs.Delivery = (*Service)(nil)

// outcome is the internal result of one prepared delivery: what Prepare returns and the signals
// it records after the commit.
type outcome struct {
	out     runs.DeliveryOutcome
	checked bool
	allowed bool
	reason  string
	handle  string
}

// Prepare implements runs.Delivery (M2-08 design note 3) in one unit of work: the run row is
// locked; the delivery matrix decides whether the event is checked; a queued run past
// run.queued_max_age fails with agent_unavailable; agent.invoke is decided in filter mode for the
// re-check principal of the trigger at the pinned permission version; a denial applies its
// consequence, writes one audit entry and sends nothing; an allow issues a fresh token, stores the
// recorded jti and the configured billing mode and moves trigger_state from received to queued on
// the first delivery of agent.invocation. The counter and the span are recorded after the commit.
//
// Errors: internal for a type that bears no invocation, a run that is missing or belongs to another
// agent, a missing collaborator; the errors of the database, the authorizer and the signer.
func (s *Service) Prepare(ctx context.Context, in runs.DeliveryInput) (runs.DeliveryOutcome, error) {
	if !domain.InvocationBearing(in.Type) {
		return runs.DeliveryOutcome{}, errs.New(errs.CodeInternalError, "runs: Prepare for an event that bears no invocation")
	}
	if _, ok := resolvers[in.Type]; !ok {
		s.noResolver(ctx, in.Type)
		return runs.DeliveryOutcome{}, nil
	}
	box := s.tokens.Load()
	if box == nil || s.d.Versions == nil || s.d.Audit == nil {
		return runs.DeliveryOutcome{}, errs.New(errs.CodeInternalError, "runs: the delivery has no tokens, versions or audit")
	}
	if !in.CorrelationID.IsZero() {
		ctx = reqmeta.WithCorrelationID(ctx, in.CorrelationID.String())
	}
	// The events of the unit (run.state_changed, message.updated) have no actor: the delivery is
	// the server's own step, not a request of the receiving agent.
	ctx = actor.WithActor(ctx, actor.Anonymous())
	var res outcome
	err := s.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		if in.Type == domain.EventCommandInvoked {
			res, err = s.prepareInvoked(ctx, in, box.issuer)
		} else {
			res, err = s.prepare(ctx, in, box.issuer)
		}
		return err
	})
	if err != nil {
		return runs.DeliveryOutcome{}, err
	}
	if res.checked && !res.allowed {
		s.telemetry().Denied(context.WithoutCancel(ctx), res.handle, res.reason)
	}
	return res.out, nil
}

// deliveryRun returns the run of a delivery: the resource of agent.invocation and agent.followup,
// the run_id of the payload of interaction.created, whose resource is the interaction (M2-15
// design note 11 a). Errors: internal for a resource or payload that names no run.
func deliveryRun(in runs.DeliveryInput) (ids.ID, error) {
	if in.Type == domain.EventInteractionCreated {
		if in.ResourceKind != resourceInteraction {
			return ids.ID{}, errs.New(errs.CodeInternalError, "runs: interaction.created without its interaction")
		}
		_, run, err := domain.InteractionDeliveryIDs(in.Payload)
		if err != nil {
			return ids.ID{}, errs.Wrap(errs.CodeInternalError, err, "runs: the payload of interaction.created")
		}
		return run, nil
	}
	if in.ResourceKind != resourceRun {
		return ids.ID{}, errs.New(errs.CodeInternalError, "runs: the resource of a delivery is no run")
	}
	return in.ResourceID, nil
}

// noResolver logs an invocation-bearing event without a registered resolver; a retry cannot help,
// so it is not sent (fail closed, design note 3 a).
func (s *Service) noResolver(ctx context.Context, typ string) {
	s.d.Log.LogAttrs(ctx, slog.LevelError, msgNoResolver, slog.String(keyEventType, typ))
}

// telemetry returns the delivery telemetry or the no-op.
func (s *Service) telemetry() DeliveryTelemetry {
	if s.d.Telemetry == nil {
		return noTelemetry{}
	}
	return s.d.Telemetry
}

// prepare is Prepare inside its unit of work.
func (s *Service) prepare(ctx context.Context, in runs.DeliveryInput, issuer TokenIssuer) (outcome, error) {
	runID, err := deliveryRun(in)
	if err != nil {
		return outcome{}, err
	}
	run, found, err := s.d.Repo.LockRun(ctx, runID)
	if err != nil {
		return outcome{}, err
	}
	if !found || run.AgentID != in.AgentID {
		return outcome{}, errs.New(errs.CodeInternalError, "runs: the run of a delivery is missing or belongs to another agent")
	}
	rule, err := domain.DeliveryMatrix(in.Type, run.State)
	if err != nil {
		return outcome{}, errs.Wrap(errs.CodeInternalError, err, "runs: delivery matrix")
	}
	if rule.Skip {
		return outcome{}, nil
	}
	res := resolverFor(in.Type, run)
	if res == nil {
		s.noResolver(ctx, in.Type)
		return outcome{}, nil
	}
	if in.Type == domain.EventAgentFollowup {
		// A follow-up that is no longer open or whose message is deleted is not delivered: no
		// check, no token, no audit, like a terminal run (M2-11 design note 6).
		if ok, err := s.followupDeliverable(ctx, run.ID, in.Payload); err != nil || !ok {
			return outcome{}, err
		}
	}
	if run.State == domain.StateQueued && domain.QueuedTooOld(run.QueuedAt, s.now(), s.cfg.QueuedMaxAge) {
		// Not an authorization decision: no check, no token, no audit (design note 3 c).
		return outcome{}, s.expire(ctx, run)
	}
	tr, err := res(s, ctx, run, in)
	if err != nil {
		return outcome{}, err
	}
	if tr.gone {
		// The interaction of the event no longer exists (design note 11 a): nothing to check.
		return outcome{}, nil
	}
	facts, err := s.d.Agents.RunFacts(ctx, []ids.ID{run.AgentID})
	if err != nil {
		return outcome{}, err
	}
	agent := facts[run.AgentID]
	sctx, end := s.telemetry().StartDispatch(ctx, run.ID, agent.Handle)
	o, err := s.check(sctx, in, run, rule, tr, agent.Status == agentActive, issuer, agent.BillingMode, agent.ToolClass)
	switch {
	case err != nil:
		end(decisionDeny)
	case o.allowed:
		end(decisionAllow)
	default:
		end(decisionDeny)
	}
	o.handle = agent.Handle
	return o, err
}

// check decides agent.invoke for the trigger tr of run and applies the result.
func (s *Service) check(ctx context.Context, in runs.DeliveryInput, run RunRow, rule domain.DeliveryRule, tr trigger, agentActive bool,
	issuer TokenIssuer, billingMode, toolClass string) (outcome, error) {
	info, err := s.d.Threads.Info(ctx, run.ThreadID)
	if err != nil {
		return outcome{}, err
	}
	v, err := s.d.Versions.PermissionVersion(ctx)
	if err != nil {
		return outcome{}, err
	}
	p := actor.Recheck(tr.person)
	r := authorize.Resource{
		Kind: authorize.ResourceThread, ID: run.ThreadID, ChannelID: info.ChannelID, SpaceID: info.SpaceID, OwnerID: info.CreatedBy,
		Invoke: &authorize.InvokeFacts{AgentID: run.AgentID, TriggerOrigin: tr.origin, MentionsAgent: tr.mentions,
			ActiveRunAgentID: run.AgentID, NewThread: tr.newThread},
		Interaction: tr.facts,
	}
	d, err := s.d.Authorizer.Authorize(authorize.WithPermissionVersion(authorize.Filter(ctx), v), p, authorize.ActionAgentInvoke, r)
	if err != nil {
		return outcome{}, err
	}
	if !d.Allowed() {
		reason := domain.DeliveryReason(string(d.Reason), agentActive)
		if err := s.deny(ctx, in, run, rule, tr, reason, string(d.Reason)); err != nil {
			return outcome{}, err
		}
		return outcome{checked: true, reason: reason}, nil
	}
	return s.allow(ctx, in, run, rule, tr, info.ChannelID, info.SpaceID, v, issuer, billingMode, toolClass)
}

// deny applies the consequence of a denial (design note 3 e to g) and writes its audit entry.
func (s *Service) deny(ctx context.Context, in runs.DeliveryInput, run RunRow, rule domain.DeliveryRule, tr trigger, reason, authzReason string) error {
	switch rule.Denial {
	case domain.DenyFailRun:
		// Through the end-of-run function, so a card bound to the queued run is disabled too
		// (M2-15 design note 15 b).
		if _, err := s.endRun(ctx, run, domain.StateFailed, reason, domain.SetterServer, TransitionStore{}); err != nil {
			return err
		}
	case domain.DenyFailFollowup:
		if !tr.interaction.IsZero() {
			// interaction.created: the interaction fails, the run stays (ruling D-M2-15-15).
			if err := s.setInteraction(ctx, tr.interaction, interactionFailed); err != nil {
				return err
			}
		}
		if tr.human {
			if err := s.d.Messages.SetTriggerState(ctx, messaging.SetTriggerStateCmd{
				MessageID: tr.message, State: string(domain.TriggerStateFailed), Reason: reason,
			}); err != nil {
				return err
			}
		}
		// A refused follow-up is never processed: its row becomes discarded (M2-11 design note 6).
		if in.Type == domain.EventAgentFollowup && s.fu != nil {
			if err := s.fu.RefuseDelivery(ctx, run.ID, tr.message); err != nil {
				return err
			}
		}
	case domain.DenyKeepRun:
	}
	return s.auditDenial(ctx, tr.person, authzReason, run.ThreadID, auditing.Attrs{
		attrRunID: auditing.ID(run.ID), attrAgentID: auditing.ID(run.AgentID), attrStage: auditing.Code(stageDelivery),
		attrEventType: auditing.Code(in.Type), attrTriggerKind: auditing.Code(triggerKindOf(in.Type, run)),
	})
}

// auditDenial writes the one audit entry of a denial of agent.invoke in the unit of work, with the
// re-check principal of person as the actor (no device, no session) and the exact authorize
// reason (design note 8). It never carries a token or a jti.
func (s *Service) auditDenial(ctx context.Context, person ids.ID, reason string, thread ids.ID, attrs auditing.Attrs) error {
	return s.d.Audit.Record(actor.WithActor(ctx, actor.Recheck(person)), auditing.Entry{
		Action: string(authorize.ActionAgentInvoke), Decision: auditing.DecisionDeny, Reason: auditing.Reason(reason),
		ResourceKind: resourceThread, ResourceID: thread, Attrs: attrs,
	})
}

// triggerKindOf is the claim trigger_kind of a delivery of typ for run.
func triggerKindOf(typ string, run RunRow) string {
	switch typ {
	case domain.EventAgentInvocation:
		return run.TriggerKind
	case domain.EventAgentFollowup:
		return string(domain.TriggerFollowup)
	case domain.EventInteractionCreated:
		return string(domain.TriggerInteraction)
	}
	return string(domain.TriggerCommand)
}

// allow issues the token of an allowed delivery and stores what changes (design note 3 h).
func (s *Service) allow(ctx context.Context, in runs.DeliveryInput, run RunRow, rule domain.DeliveryRule, tr trigger, channel, space ids.ID,
	version int64, issuer TokenIssuer, billingMode, toolClass string) (outcome, error) {
	g, err := domain.DeliveryGrant(domain.DeliveryFacts{
		EventType: in.Type, RunTriggerKind: run.TriggerKind, RunID: run.ID, AgentID: run.AgentID, TriggeredBy: tr.person,
		ObjectID: tr.object, ThreadID: run.ThreadID, ChannelID: channel, SpaceID: space, BillingMode: billingMode,
		ToolClass: toolClass, PermissionVersion: version,
	})
	if err != nil {
		return outcome{}, errs.Wrap(errs.CodeInternalError, err, "runs: the grant of a delivery")
	}
	tok, err := issuer.Issue(ctx, g)
	if err != nil {
		return outcome{}, err
	}
	w := DeliveryWrite{ID: run.ID, BillingModeExpected: billingMode}
	if rule.Record {
		w.JTI = tok.JTI
	}
	if rule.Record || run.BillingModeExpected != billingMode {
		if _, err := s.d.Repo.RecordDelivery(ctx, w); err != nil {
			return outcome{}, err
		}
	}
	if !tr.interaction.IsZero() {
		// The server sets dispatched when it hands the event to a transport (ruling D-M2-15-15).
		if err := s.setInteraction(ctx, tr.interaction, interactionDispatched); err != nil {
			return outcome{}, err
		}
	}
	if in.Type == domain.EventAgentInvocation && run.State == domain.StateQueued && tr.human && tr.message == run.TriggeringMessageID &&
		tr.state == triggerStateReceived {
		if err := s.d.Messages.SetTriggerState(ctx, messaging.SetTriggerStateCmd{MessageID: tr.message, State: string(domain.TriggerStateQueued)}); err != nil {
			return outcome{}, err
		}
	}
	return outcome{out: runs.DeliveryOutcome{Send: true, InvocationToken: tok.Token.Reveal()}, checked: true, allowed: true}, nil
}

// setInteraction moves the interaction id from received to state at a delivery (dispatched on
// allow, failed on denial) under its row lock; an interaction the agent already answered, or one
// already in state, keeps its state.
func (s *Service) setInteraction(ctx context.Context, id ids.ID, state string) error {
	ix, found, err := s.d.Components.LockInteraction(ctx, id)
	if err != nil || !found {
		return err
	}
	if ix.State != interactionReceived && (ix.State != interactionDispatched || state != interactionFailed) {
		return nil
	}
	_, err = s.d.Components.SetInteractionState(ctx, id, state, nil, s.now())
	return err
}

// expire fails a queued run that reached run.queued_max_age with agent_unavailable (09 5.5,
// D-AG-16); Transition sets trigger_state failed with the same reason.
// It goes through the end-of-run function, so a card bound to the queued run is disabled too
// (M2-15 design note 15 b).
func (s *Service) expire(ctx context.Context, run RunRow) error {
	_, err := s.endRun(ctx, run, domain.StateFailed, domain.RunReasonAgentUnavailable, domain.SetterServer, TransitionStore{})
	return err
}

// ExpireQueued is the use case of the job runs.queued_expiry (design note 11) in one unit of work:
// the run is locked; still queued and due, it fails with agent_unavailable; queued but not yet due
// (run.queued_max_age was raised), the job is scheduled again for the new due time; otherwise
// nothing happens. Errors: those of the database and of the scheduler; an unknown run is nothing.
func (s *Service) ExpireQueued(ctx context.Context, runID ids.ID) error {
	return s.d.UoW.Do(ctx, func(ctx context.Context) error {
		run, found, err := s.d.Repo.LockRun(ctx, runID)
		if err != nil || !found || run.State != domain.StateQueued {
			return err
		}
		if domain.QueuedTooOld(run.QueuedAt, s.now(), s.cfg.QueuedMaxAge) {
			return s.expire(ctx, run)
		}
		return s.scheduleExpiry(ctx, run)
	})
}

// scheduleExpiry inserts runs.queued_expiry for run at queued_at + run.queued_max_age in the unit
// of work of ctx; without a scheduler nothing is inserted and the next Prepare fails the run when
// it is too old (design note 3 c).
func (s *Service) scheduleExpiry(ctx context.Context, run RunRow) error {
	if s.d.Expiry == nil {
		return nil
	}
	return s.d.Expiry.ScheduleQueuedExpiry(ctx, run.ID, run.QueuedAt.Add(s.cfg.QueuedMaxAge))
}

// noTelemetry records nothing.
type noTelemetry struct{}

func (noTelemetry) Denied(context.Context, string, string) {}

func (noTelemetry) StartDispatch(ctx context.Context, _ ids.ID, _ string) (context.Context, func(string)) {
	return ctx, func(string) {}
}

// followupDeliverable reports whether the follow-up named by the payload of agent.followup may be
// delivered for the locked run; without the follow-up service nothing is deliverable.
func (s *Service) followupDeliverable(ctx context.Context, runID ids.ID, payload []byte) (bool, error) {
	if s.fu == nil {
		return false, nil
	}
	id, err := domain.FollowupMessageID(payload)
	if err != nil {
		return false, errs.Wrap(errs.CodeInternalError, err, "runs: the payload of agent.followup")
	}
	return s.fu.ForDelivery(ctx, runID, id)
}
