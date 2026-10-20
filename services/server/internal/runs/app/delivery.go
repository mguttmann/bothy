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
}

// resolver reads the trigger of one delivery of the locked run.
type resolver func(s *Service, ctx context.Context, run RunRow, in runs.DeliveryInput) (trigger, error)

// resolvers is the table of design note 5: one entry per event type, for agent.invocation one per
// runs.trigger_kind. interaction.created, command.invoked and the trigger kinds command and
// interaction come with M2-15 and M2-16; until then such an event is never sent.
var resolvers = map[string]map[string]resolver{
	domain.EventAgentInvocation: {triggerKindMessage: resolveInvocationMessage},
	domain.EventAgentFollowup:   {"": resolveFollowup},
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
		res, err = s.prepare(ctx, in, box.issuer)
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
	run, found, err := s.d.Repo.LockRun(ctx, in.ResourceID)
	if err != nil {
		return outcome{}, err
	}
	if !found || run.AgentID != in.AgentID || in.ResourceKind != resourceRun {
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
	if run.State == domain.StateQueued && domain.QueuedTooOld(run.QueuedAt, s.now(), s.cfg.QueuedMaxAge) {
		// Not an authorization decision: no check, no token, no audit (design note 3 c).
		return outcome{}, s.expire(ctx, run)
	}
	tr, err := res(s, ctx, run, in)
	if err != nil {
		return outcome{}, err
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
		if _, err := s.Transition(ctx, TransitionCmd{
			RunID: run.ID, From: run.State, Version: run.Version, To: domain.StateFailed, Reason: reason, SetBy: domain.SetterServer,
		}); err != nil {
			return err
		}
	case domain.DenyFailFollowup:
		if tr.human {
			if err := s.d.Messages.SetTriggerState(ctx, messaging.SetTriggerStateCmd{
				MessageID: tr.message, State: string(domain.TriggerStateFailed), Reason: reason,
			}); err != nil {
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
	if in.Type == domain.EventAgentInvocation && run.State == domain.StateQueued && tr.human && tr.message == run.TriggeringMessageID &&
		tr.state == triggerStateReceived {
		if err := s.d.Messages.SetTriggerState(ctx, messaging.SetTriggerStateCmd{MessageID: tr.message, State: string(domain.TriggerStateQueued)}); err != nil {
			return outcome{}, err
		}
	}
	return outcome{out: runs.DeliveryOutcome{Send: true, InvocationToken: tok.Token.Reveal()}, checked: true, allowed: true}, nil
}

// expire fails a queued run that reached run.queued_max_age with agent_unavailable (09 5.5,
// D-AG-16); Transition sets trigger_state failed with the same reason.
func (s *Service) expire(ctx context.Context, run RunRow) error {
	_, err := s.Transition(ctx, TransitionCmd{
		RunID: run.ID, From: domain.StateQueued, Version: run.Version, To: domain.StateFailed,
		Reason: domain.RunReasonAgentUnavailable, SetBy: domain.SetterServer,
	})
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
