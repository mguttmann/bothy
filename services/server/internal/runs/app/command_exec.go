// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"log/slog"

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

// CommandRegister is the part of agents.Commands the execution uses: the live command usable in a
// channel (Resolve, which does not authorize).
type CommandRegister interface {
	Resolve(ctx context.Context, commandID, channelID ids.ID) (agents.Command, error)
}

// The values of command execution.
const (
	channelForum          = "forum"
	ownerKindAgent        = "agent"
	resourceCommand       = "command"
	reasonDecisionCommand = "command_decision_mismatch"
	msgCommandMismatch    = "runs: command_decision_mismatch: the command decision rejected what command.invoke allowed; failing closed"
)

// Compile-time check: Service is the command execution port.
var _ runs.CommandExec = (*Service)(nil)

// BindCommands sets the command register the execution resolves commands through; internal/wire
// calls it once the agents register exists. Errors: a nil register.
func (s *Service) BindCommands(c CommandRegister) error {
	if c == nil {
		return errors.New("runs: BindCommands needs the command register")
	}
	s.commands.Store(&commandBox{c})
	return nil
}

// commandBox holds the bound register.
type commandBox struct{ register CommandRegister }

// Invoke implements runs.CommandExec (brief M2-16 design notes 10 to 12) in one unit of work:
// resolve the command, find the target thread, lock it for a model command, gather the facts,
// authorize command.invoke and agent.invoke, check the option values, then append command.invoked
// for a control command or open the run and append agent.invocation for a model command.
func (s *Service) Invoke(ctx context.Context, cmd runs.InvokeCommand) (runs.CommandInvocation, error) {
	box := s.commands.Load()
	if box == nil {
		return runs.CommandInvocation{}, errs.New(errs.CodeInternalError, "runs: Invoke without the command register")
	}
	var out runs.CommandInvocation
	err := s.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		out, err = s.invokeCommand(ctx, box.register, cmd)
		return err
	})
	return out, err
}

// invocationTarget is the place of one invocation.
type invocationTarget struct {
	info chat.ThreadInfo
	ch   chat.TriggerChannel
}

// invokeCommand is Invoke inside its unit of work.
func (s *Service) invokeCommand(ctx context.Context, reg CommandRegister, cmd runs.InvokeCommand) (runs.CommandInvocation, error) {
	c, err := reg.Resolve(ctx, cmd.CommandID, cmd.ChannelID)
	if err != nil {
		return runs.CommandInvocation{}, err
	}
	if c.OwnerKind != ownerKindAgent {
		// App commands come with M2-25; until then no app owner can exist (fail closed).
		return runs.CommandInvocation{}, errs.New(errs.CodeInternalError, "runs: a command of an app before M2-25")
	}
	tg, err := s.commandTarget(ctx, c, cmd)
	if err != nil {
		return runs.CommandInvocation{}, err
	}
	f, err := s.commandFacts(ctx, c, tg)
	if err != nil {
		return runs.CommandInvocation{}, err
	}
	if err := s.authorizeCommand(ctx, c, tg, f.found); err != nil {
		return runs.CommandInvocation{}, err
	}
	if f.decision.Outcome == domain.OutcomeReject {
		// The decision and authorize disagree: fail closed (same rule as D-M2-05-14).
		s.d.Log.LogAttrs(ctx, slog.LevelError, msgCommandMismatch, slog.String(keyAction, string(authorize.ActionCommandInvoke)),
			slog.String(keyReason, reasonDecisionCommand))
		return runs.CommandInvocation{}, invocationDenied(string(domain.ReasonRunActive))
	}
	values, err := domain.ValidateOptionValues(optionDefs(c), optionValues(cmd.Options))
	if err != nil {
		return runs.CommandInvocation{}, err
	}
	invoker, err := s.invoker(ctx)
	if err != nil {
		return runs.CommandInvocation{}, err
	}
	inv := runs.CommandInvocation{InvocationID: ids.New(), CommandID: c.ID, ChannelID: tg.info.ChannelID, ThreadID: tg.info.ID}
	if c.Control {
		var runID ids.ID
		if f.found && f.active.AgentID == c.OwnerPrincipalID {
			runID = f.active.ID
		}
		return inv, s.emitCommandInvoked(ctx, c, tg, inv, runID, invoker, values)
	}
	return s.openCommandRun(ctx, c, tg, inv, f.mode, invoker, values)
}

// commandFacts are the facts of one invocation: the active run of the target thread (any agent),
// the owner's billing mode and the decision of M2-04.
type commandFacts struct {
	active   RunRow
	found    bool
	mode     string
	decision domain.Decision
}

// commandFacts locks the target thread for a model command (the order of locks of the message
// acceptance: thread, then runs, 03 section 6.3), reads the active run and the owner's facts and
// decides with DecideCommand.
func (s *Service) commandFacts(ctx context.Context, c agents.Command, tg invocationTarget) (commandFacts, error) {
	if !c.Control {
		if err := s.d.Threads.LockForMessageChange(ctx, tg.info.ID); err != nil {
			return commandFacts{}, err
		}
	}
	var f commandFacts
	var err error
	if f.active, f.found, err = s.d.Repo.ActiveRunOfThread(ctx, tg.info.ID); err != nil {
		return commandFacts{}, err
	}
	facts, err := s.d.Agents.RunFacts(ctx, []ids.ID{c.OwnerPrincipalID})
	if err != nil {
		return commandFacts{}, err
	}
	f.mode = facts[c.OwnerPrincipalID].BillingMode
	trig := domain.CommandTrigger{CommandID: c.ID, ThreadID: tg.info.ID, Owner: *agentFacts(facts, c.OwnerPrincipalID), Control: c.Control}
	if f.found {
		trig.ActiveRun = &domain.ActiveRun{RunID: f.active.ID, AgentID: f.active.AgentID}
	}
	f.decision, err = domain.DecideCommand(trig)
	return f, err
}

// openCommandRun opens the run of a model command (design note 12) and appends its
// agent.invocation. A run another path opened in the thread meanwhile is decided again with the
// active run, whose refusal (run_active) authz audits; nothing of the unit is written then.
func (s *Service) openCommandRun(ctx context.Context, c agents.Command, tg invocationTarget, inv runs.CommandInvocation, mode string,
	invoker domain.PrincipalRefPayload, values []domain.OptionValue) (runs.CommandInvocation, error) {
	sum, err := s.Open(ctx, OpenCmd{
		ThreadID: tg.info.ID, AgentID: c.OwnerPrincipalID, TriggeredBy: caller(ctx).ID, TriggerKind: triggerKindCommand,
		TriggeringCommandID: c.ID, BillingModeExpected: mode,
	})
	var exists *ActiveRunExistsError
	if errors.As(err, &exists) {
		if err := s.authorizeCommand(ctx, c, tg, true); err != nil {
			return runs.CommandInvocation{}, err
		}
		return runs.CommandInvocation{}, invocationDenied(string(domain.ReasonRunActive))
	}
	if err != nil {
		return runs.CommandInvocation{}, err
	}
	inv.RunID = sum.ID
	return inv, s.emitCommandInvocation(ctx, c, tg, inv, mode, invoker, values)
}

// commandTarget resolves the target thread (design note 10 b): the thread of thread_id in the
// channel; without it the main thread of a text channel for a control command, and for a model
// command only while auto_thread is off; otherwise 400 /thread_id required, after a visibility
// check of the channel so an invisible channel answers 404 like an unknown one.
func (s *Service) commandTarget(ctx context.Context, c agents.Command, cmd runs.InvokeCommand) (invocationTarget, error) {
	ch, err := s.d.Channels.TriggerChannel(ctx, cmd.ChannelID)
	if err != nil {
		return invocationTarget{}, err
	}
	thread := cmd.ThreadID
	if thread.IsZero() {
		if ch.Kind == channelForum || (!c.Control && ch.AutoThread) || ch.MainThreadID.IsZero() {
			d, err := s.d.Authorizer.Authorize(authorize.Filter(ctx), caller(ctx), authorize.ActionChannelRead,
				authorize.Resource{Kind: authorize.ResourceChannel, ID: cmd.ChannelID})
			if err != nil {
				return invocationTarget{}, err
			}
			if !d.Allowed() {
				return invocationTarget{}, invisible()
			}
			return invocationTarget{}, domain.OptionField("/thread_id", "required", "the command needs a thread here")
		}
		thread = ch.MainThreadID
	}
	info, err := s.d.Threads.Info(ctx, thread)
	if errs.KindOf(err) == errs.KindNotFound {
		return invocationTarget{}, invisible()
	}
	if err != nil {
		return invocationTarget{}, err
	}
	if info.ChannelID != cmd.ChannelID {
		return invocationTarget{}, invisible()
	}
	return invocationTarget{info: info, ch: ch}, nil
}

// authorizeCommand decides command.invoke and then agent.invoke in request mode (design note 10 e
// to g): the command addresses its owner explicitly like a mention, and the active-run rule is
// command.invoke's, so agent.invoke gets no active run. A denial is mapped to the answer of 06
// section 4 point 6; authz audits it.
func (s *Service) authorizeCommand(ctx context.Context, c agents.Command, tg invocationTarget, activeRun bool) error {
	p := caller(ctx)
	r := authorize.Resource{
		Kind: authorize.ResourceThread, ID: tg.info.ID, ChannelID: tg.info.ChannelID, SpaceID: tg.info.SpaceID, OwnerID: tg.info.CreatedBy,
		Command: &authorize.CommandFacts{OwnerKind: c.OwnerKind, RequiredLevel: c.RequiredLevel, Control: c.Control, ActiveRun: activeRun},
	}
	d, err := s.d.Authorizer.Authorize(ctx, p, authorize.ActionCommandInvoke, r)
	if err != nil {
		return err
	}
	if !d.Allowed() {
		return mapInvocationDenial(d)
	}
	ir := authorize.Resource{
		Kind: authorize.ResourceThread, ID: tg.info.ID, ChannelID: tg.info.ChannelID, SpaceID: tg.info.SpaceID, OwnerID: tg.info.CreatedBy,
		Invoke: &authorize.InvokeFacts{AgentID: c.OwnerPrincipalID, TriggerOrigin: originOf(p), MentionsAgent: true},
	}
	if d, err = s.d.Authorizer.Authorize(ctx, p, authorize.ActionAgentInvoke, ir); err != nil {
		return err
	}
	if !d.Allowed() {
		return mapInvocationDenial(d)
	}
	return nil
}

// originOf is the trigger origin of the invoker: human for a person, else its principal kind.
func originOf(a actor.Actor) string {
	if a.Kind == actor.KindUser {
		return originHuman
	}
	return string(a.Kind)
}

// mapInvocationDenial maps a refusal (06 section 4 point 6, design note 10 g): not_found stays
// 404, a read-only or archived channel stays 403 channel_read_only, every other forbidden is 403
// agent_invocation_denied with the decision's reason.
func mapInvocationDenial(d authorize.Decision) error {
	err := d.Err()
	if e, ok := errs.As(err); ok && e.Code == errs.CodeForbidden {
		return invocationDenied(string(d.Reason))
	}
	return err
}

// invocationDenied is 403 agent_invocation_denied with reason.
func invocationDenied(reason string) error {
	return errs.New(errs.CodeAgentInvocationDenied, "").WithReason(reason)
}

// optionDefs converts the options of c.
func optionDefs(c agents.Command) []domain.OptionDef {
	out := make([]domain.OptionDef, 0, len(c.Options))
	for _, o := range c.Options {
		d := domain.OptionDef{Name: o.Name, Type: o.Type, Required: o.Required, Autocomplete: o.Autocomplete}
		for _, ch := range o.Choices {
			d.Choices = append(d.Choices, ch.Value)
		}
		out = append(out, d)
	}
	return out
}

// optionValues converts the submitted values.
func optionValues(in []runs.OptionValue) []domain.OptionValue {
	out := make([]domain.OptionValue, 0, len(in))
	for _, v := range in {
		out = append(out, domain.OptionValue(v))
	}
	return out
}

// invoker returns the invoker as PrincipalRef (design note 14).
func (s *Service) invoker(ctx context.Context) (domain.PrincipalRefPayload, error) {
	id := caller(ctx).ID
	refs, err := s.d.Messages.PrincipalRefs(ctx, []ids.ID{id})
	if err != nil {
		return domain.PrincipalRefPayload{}, err
	}
	ref, ok := refs[id]
	if !ok {
		return domain.PrincipalRefPayload{}, errs.New(errs.CodeInternalError, "runs: the invoker is not in the directory")
	}
	return refPayload(ref), nil
}

// refPayload converts a PrincipalRef of messaging.
func refPayload(r messaging.PrincipalRef) domain.PrincipalRefPayload {
	return domain.PrincipalRefPayload{ID: r.ID.String(), Kind: r.Kind, Handle: r.Handle, DisplayName: r.DisplayName,
		AvatarFileID: optionalID(r.AvatarFileID)}
}

// fits refuses a payload that is not smaller than sync.event_payload_max_bytes (the rule of the
// appender) with /options too_large (design note 10 i).
func (s *Service) fits(p []byte) error {
	if len(p) >= s.cfg.EventPayloadMaxBytes {
		return domain.OptionField("/options", domain.FieldTooLarge, "the option values are too large for the event")
	}
	return nil
}

// emitCommandInvoked appends command.invoked of a control command (design note 11): scope of the
// thread, resource command without version, audience the owner, receive action
// agent.events.consume, the invoker as actor; nothing else is written.
func (s *Service) emitCommandInvoked(ctx context.Context, c agents.Command, tg invocationTarget, inv runs.CommandInvocation, runID ids.ID,
	invoker domain.PrincipalRefPayload, values []domain.OptionValue) error {
	p, err := domain.CommandInvokedPayload(domain.CommandInvokedInput{
		InvocationID: inv.InvocationID, CommandID: c.ID, CommandName: c.Name, ChannelID: tg.info.ChannelID, ThreadID: tg.info.ID,
		RunID: runID, InvokedBy: invoker, Options: values,
	})
	if err != nil {
		return errs.Wrap(errs.CodeInternalError, err, "runs: the payload of command.invoked")
	}
	if err := s.fits(p); err != nil {
		return err
	}
	kind, actorID := eventActor(ctx)
	return s.d.Events.Append(ctx, eventlog.Event{
		Type: string(authorize.EventCommandInvoked), SchemaVersion: 1, ActorKind: kind, ActorPrincipalID: actorID,
		SpaceID: tg.info.SpaceID, ChannelID: tg.info.ChannelID, ThreadID: tg.info.ID, ResourceKind: resourceCommand, ResourceID: c.ID,
		AudiencePrincipalID: c.OwnerPrincipalID, ReceiveAction: string(authorize.ActionAgentEventsConsume), Payload: p,
	})
}

// emitCommandInvocation appends agent.invocation of a command run (design note 12 c) with the
// command, its parsed options and the invocation, without message_id and without a token. The
// payload carries the session of the run like a message run (M2-12 design note 9), decided after
// the run row is written.
func (s *Service) emitCommandInvocation(ctx context.Context, c agents.Command, tg invocationTarget, inv runs.CommandInvocation, mode string,
	invoker domain.PrincipalRefPayload, values []domain.OptionValue) error {
	invokerID, err := ids.Parse(invoker.ID)
	if err != nil {
		return errs.Wrap(errs.CodeInternalError, err, "runs: the invoker of a command run")
	}
	sum := runs.RunSummary{ID: inv.RunID, ThreadID: tg.info.ID, AgentID: c.OwnerPrincipalID, TriggeredBy: invokerID}
	session, err := s.invocationSession(ctx, sum, operatorOf(tg.info, tg.ch))
	if err != nil {
		return err
	}
	p, err := domain.InvocationPayload(domain.InvocationPayloadInput{
		RunID: inv.RunID, ThreadID: tg.info.ID, ChannelID: tg.info.ChannelID, SpaceID: tg.info.SpaceID, TriggeredBy: invoker,
		BillingMode: mode, SettingsVersion: tg.ch.SettingsVersion, TriggerKind: triggerKindCommand, CommandInvocationID: inv.InvocationID,
		CommandID: c.ID, CommandName: c.Name, Options: values,
	}, session)
	if err != nil {
		return errs.Wrap(errs.CodeInternalError, err, "runs: the payload of agent.invocation")
	}
	if err := s.fits(p); err != nil {
		return err
	}
	kind, actorID := eventActor(ctx)
	return s.d.Events.Append(ctx, eventlog.Event{
		Type: string(authorize.EventAgentInvocation), SchemaVersion: 1, ActorKind: kind, ActorPrincipalID: actorID,
		SpaceID: tg.info.SpaceID, ChannelID: tg.info.ChannelID, ThreadID: tg.info.ID, ResourceKind: resourceRun, ResourceID: inv.RunID,
		AudiencePrincipalID: c.OwnerPrincipalID, ReceiveAction: string(authorize.ActionAgentEventsConsume), Payload: p,
	})
}
