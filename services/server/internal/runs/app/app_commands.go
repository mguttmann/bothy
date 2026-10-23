// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/agents"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/runs"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// ownerKindApp is the owner kind of an app's command and the author kind of an app's card.
const ownerKindApp = "app"

// appTarget is the place of an app command or autocomplete request: the channel's space and the
// target thread (zero in a forum without thread_id).
type appTarget struct {
	spaceID, threadID ids.ID
}

// appCommandPlace checks the thread of the request against the channel before authorize (a given
// thread_id must belong to the channel, else 404) and returns the target of design note 5 b.
func (s *Service) appCommandPlace(ctx context.Context, channel, thread ids.ID) (appTarget, error) {
	ch, err := s.d.Channels.TriggerChannel(ctx, channel)
	if errs.KindOf(err) == errs.KindNotFound {
		return appTarget{}, invisible()
	}
	if err != nil {
		return appTarget{}, err
	}
	if !thread.IsZero() {
		info, err := s.d.Threads.Info(ctx, thread)
		if errs.KindOf(err) == errs.KindNotFound || err == nil && info.ChannelID != channel {
			return appTarget{}, invisible()
		}
		if err != nil {
			return appTarget{}, err
		}
	}
	return appTarget{spaceID: ch.SpaceID, threadID: domain.AppCommandTarget(thread, ch.Kind, ch.MainThreadID)}, nil
}

// invokeAppCommand is the app branch of Invoke (brief M2-25 design note 5) inside its unit of
// work, which joins the unit of the idempotency stage: the target thread, command.invoke in
// request mode with the app-owner guard (never agent.invoke, R-AZ-18), the option values, then
// command.invoked with the app as audience and the job runs.app_dispatch. No run, no token, no
// trigger state; control is passed through unchanged.
func (s *Service) invokeAppCommand(ctx context.Context, c agents.Command, cmd runs.InvokeCommand) (runs.CommandInvocation, error) {
	tg, err := s.appCommandPlace(ctx, cmd.ChannelID, cmd.ThreadID)
	if err != nil {
		return runs.CommandInvocation{}, err
	}
	d, err := s.d.Authorizer.Authorize(ctx, caller(ctx), authorize.ActionCommandInvoke, appCommandResource(c, cmd.ChannelID, tg.threadID))
	if err != nil {
		return runs.CommandInvocation{}, err
	}
	if !d.Allowed() {
		return runs.CommandInvocation{}, mapInvocationDenial(d)
	}
	values, err := domain.ValidateOptionValues(optionDefs(c), optionValues(cmd.Options))
	if err != nil {
		return runs.CommandInvocation{}, err
	}
	invoker, err := s.invoker(ctx)
	if err != nil {
		return runs.CommandInvocation{}, err
	}
	inv := runs.CommandInvocation{InvocationID: ids.New(), CommandID: c.ID, ChannelID: cmd.ChannelID, ThreadID: tg.threadID}
	p, err := domain.AppCommandInvokedPayload(domain.AppCommandInvokedInput{
		InvocationID: inv.InvocationID, CommandID: c.ID, CommandName: c.Name, Control: c.Control, ChannelID: cmd.ChannelID,
		ThreadID: tg.threadID, InvokedBy: invoker, Options: values,
	})
	if err != nil {
		return runs.CommandInvocation{}, errs.Wrap(errs.CodeInternalError, err, "runs: the payload of command.invoked")
	}
	if err := s.fits(p); err != nil {
		return runs.CommandInvocation{}, err
	}
	eventID := ids.New()
	kind, actorID := eventActor(ctx)
	if err := s.d.Events.Append(ctx, eventlog.Event{
		ID: eventID, Type: string(authorize.EventCommandInvoked), SchemaVersion: 1, ActorKind: kind, ActorPrincipalID: actorID,
		SpaceID: tg.spaceID, ChannelID: cmd.ChannelID, ThreadID: tg.threadID, ResourceKind: resourceCommand, ResourceID: c.ID,
		AudiencePrincipalID: c.OwnerPrincipalID, ReceiveAction: string(authorize.ActionAgentEventsConsume), Payload: p,
	}); err != nil {
		return runs.CommandInvocation{}, err
	}
	a := caller(ctx)
	return inv, s.scheduleAppDispatch(ctx, DispatchJob{
		Kind: AppDispatchCommand, EventID: eventID, AppID: c.OwnerPrincipalID, PersonID: a.ID,
		PersonInstallationID: a.InstallationID, ChannelID: cmd.ChannelID, ThreadID: tg.threadID, CommandID: c.ID,
	})
}
