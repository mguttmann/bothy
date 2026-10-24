// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"log/slog"
	"sync/atomic"

	"github.com/OWNER/bothy/services/server/internal/agents"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/runs"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// The kinds of DispatchJob.
const (
	// AppDispatchCommand is an app command (command.invoked).
	AppDispatchCommand = "command"
	// AppDispatchInteraction is an interaction on a component of an app card (interaction.created).
	AppDispatchInteraction = "interaction"
)

// JobKindAppDispatch is the kind of the job that hands an app its command or interaction (02
// section 12.3); adapters/jobs registers it under this name.
const JobKindAppDispatch = "runs.app_dispatch"

// The log line of the job (brief M2-25 design note 8): codes only, never a value.
const (
	msgAppDispatch     = "runs: app dispatch"
	outcomeNotAnswered = "not_answered"
	principalKindApp   = "app"
	// The attributes of signals.md 5.3 (the names of platform/otel/attr, which runs/app may not
	// import).
	keyJobKind       = "bothy.job.kind"
	keyEvent         = "bothy.event.type"
	keyPrincipalKind = "bothy.principal.kind"
	keyOutcome       = "bothy.outcome"
)

// DispatchJob is the input of the job runs.app_dispatch (brief M2-25 design note 8): IDs and
// codes only (02 section 12.1). It is inserted with every app command and every interaction on an
// app card, whatever the way of the app.
type DispatchJob struct {
	// Kind is AppDispatchCommand or AppDispatchInteraction.
	Kind string
	// EventID is the stored command.invoked or interaction.created; AppID its audience.
	EventID, AppID ids.ID
	// PersonID is the invoking principal, the principal of the re-check (R-AZ-29), and
	// PersonInstallationID the installation of an invoking app (design note 5 g). The kind of the
	// principal is read at the re-check, because the arguments of a job are IDs only (02 12.1).
	PersonID             ids.ID
	PersonInstallationID ids.ID
	// ChannelID and ThreadID are the place; ThreadID is zero for an app command without a target
	// thread.
	ChannelID, ThreadID ids.ID
	// CommandID is set for a command, InteractionID for an interaction.
	CommandID, InteractionID ids.ID
}

// DispatchScheduler inserts the job runs.app_dispatch in the unit of work of ctx;
// adapters/jobs implements it over jobs.Inserter.InsertTx (02 section 8.5: no request to an app
// inside a transaction).
type DispatchScheduler interface {
	ScheduleAppDispatch(ctx context.Context, j DispatchJob) error
}

// appBox holds the bound dispatcher of the module apps.
type appBox struct{ d runs.AppDispatcher }

// appDispatcherState is the atomic holder of the bound dispatcher in Service.
type appDispatcherState = atomic.Pointer[appBox]

// BindAppDispatcher sets the implementation of runs.AppDispatcher (apps/adapters/appdispatch);
// internal/wire calls it once the module apps exists, which is built after runs. Errors: a nil
// dispatcher.
func (s *Service) BindAppDispatcher(d runs.AppDispatcher) error {
	if d == nil {
		return errors.New("runs: BindAppDispatcher needs the dispatcher")
	}
	s.apps.Store(&appBox{d: d})
	return nil
}

// appDispatcher returns the bound dispatcher or an internal error.
func (s *Service) appDispatcher() (runs.AppDispatcher, error) {
	b := s.apps.Load()
	if b == nil {
		return nil, errs.New(errs.CodeInternalError, "runs: an app command or interaction without the app dispatcher")
	}
	return b.d, nil
}

// scheduleAppDispatch inserts the job of j in the ambient unit of work.
func (s *Service) scheduleAppDispatch(ctx context.Context, j DispatchJob) error {
	if s.d.AppDispatches == nil {
		return errs.New(errs.CodeInternalError, "runs: an app command or interaction without the job scheduler")
	}
	return s.d.AppDispatches.ScheduleAppDispatch(ctx, j)
}

// DispatchToApp works the job runs.app_dispatch once (brief M2-25 design note 8): the way of the
// app; for the interaction address the re-check with the rights of this moment in filter mode
// (R-SY-02, R-AZ-29), the interaction received -> dispatched, one request outside every
// transaction and the interaction state of its outcome. It returns nil after the request, whatever
// the app answered (09 section 1 point 5: no automatic repetition); an error only for a fault before
// the request (Post without runs.ErrAfterRequest), and ctx.Err() on shutdown, so a repeated job
// sends the same webhook-id.
func (s *Service) DispatchToApp(ctx context.Context, j DispatchJob) error {
	d, err := s.appDispatcher()
	if err != nil {
		return err
	}
	way, err := d.Way(ctx, j.AppID)
	if err != nil {
		return err
	}
	switch way {
	case runs.AppWayGateway:
		// The event reaches the app through its connection, catch-up or webhook subscription; an
		// interaction stays received until respondToInteraction.
		return nil
	case runs.AppWayInactive:
		s.logApp(ctx, j, runs.AppOutcome{Reason: runs.AppReasonInactive})
		return s.failInteraction(ctx, j)
	case runs.AppWayInteractionsURL:
	default:
		return errs.New(errs.CodeInternalError, "runs: an unknown way to an app")
	}
	allowed, err := s.recheckApp(ctx, j)
	if err != nil {
		return err
	}
	if !allowed {
		s.logApp(ctx, j, runs.AppOutcome{Reason: runs.AppReasonDenied})
		return s.failInteraction(ctx, j)
	}
	if j.Kind == AppDispatchInteraction {
		proceed, err := s.markDispatched(ctx, j.InteractionID)
		if err != nil || !proceed {
			return err
		}
	}
	out, err := d.Post(ctx, runs.AppPost{AppID: j.AppID, EventID: j.EventID, ReplyThreadID: j.ThreadID})
	if cerr := ctx.Err(); cerr != nil {
		return cerr
	}
	switch {
	case errors.Is(err, runs.ErrAfterRequest):
		// A fault of the server after the app got the request: no repetition (09 section 1 point 5).
		s.d.Log.LogAttrs(ctx, slog.LevelError, msgAppDispatch, slog.String(keyJobKind, JobKindAppDispatch),
			slog.String(keyPrincipalKind, principalKindApp), slog.String(keyReason, "internal"))
		out = runs.AppOutcome{Reason: runs.AppReasonTransport}
	case err != nil:
		// A fault before the request (the database, the keys): the job system runs the job again,
		// which proceeds from dispatched (11 section 9).
		return err
	}
	if j.Kind == AppDispatchInteraction {
		return s.settleInteraction(ctx, j.InteractionID, out)
	}
	return nil
}

// logApp writes the log line of a dispatch that ended without a request (inactive, denied); the
// dispatcher logs every request it sends.
func (s *Service) logApp(ctx context.Context, j DispatchJob, o runs.AppOutcome) {
	s.d.Log.LogAttrs(ctx, slog.LevelWarn, msgAppDispatch, slog.String(keyJobKind, JobKindAppDispatch),
		slog.String(keyEvent, eventTypeOf(j)), slog.String(keyPrincipalKind, principalKindApp),
		slog.String(keyOutcome, outcomeNotAnswered), slog.String(keyReason, o.Reason))
}

// eventTypeOf is the event type of a job.
func eventTypeOf(j DispatchJob) string {
	if j.Kind == AppDispatchInteraction {
		return string(authorize.EventInteractionCreated)
	}
	return string(authorize.EventCommandInvoked)
}

// recheckApp decides the re-check of the job (design note 8 step 2) in filter mode without audit
// (AZ-P13): command.invoke with the descriptor of the invocation, or interaction.create at stage
// deliver, for the person as the principal of the check without a request (R-AZ-29). A command or
// component that is gone counts as denied.
func (s *Service) recheckApp(ctx context.Context, j DispatchJob) (bool, error) {
	fctx := authorize.Filter(ctx)
	person, err := s.recheckPrincipal(ctx, j)
	if err != nil {
		return false, err
	}
	var (
		act authorize.Action
		r   authorize.Resource
		ok  bool
	)
	switch j.Kind {
	case AppDispatchCommand:
		act = authorize.ActionCommandInvoke
		r, ok, err = s.appCommandRecheck(ctx, j)
	case AppDispatchInteraction:
		act = authorize.ActionInteractionCreate
		r, ok, err = s.appInteractionRecheck(ctx, j)
	default:
		return false, errs.New(errs.CodeInternalError, "runs: an app dispatch of an unknown kind")
	}
	if err != nil || !ok {
		return false, err
	}
	d, err := s.d.Authorizer.Authorize(fctx, person, act, r)
	if err != nil {
		return false, err
	}
	return d.Allowed(), nil
}

// appCommandRecheck builds the descriptor of command.invoke of the job; ok is false for a command
// that is no longer usable in the channel.
func (s *Service) appCommandRecheck(ctx context.Context, j DispatchJob) (authorize.Resource, bool, error) {
	box := s.commands.Load()
	if box == nil {
		return authorize.Resource{}, false, errs.New(errs.CodeInternalError, "runs: the app dispatch without the command register")
	}
	c, err := box.register.Resolve(ctx, j.CommandID, j.ChannelID)
	if errs.KindOf(err) == errs.KindNotFound {
		return authorize.Resource{}, false, nil
	}
	if err != nil {
		return authorize.Resource{}, false, err
	}
	return appCommandResource(c, j.ChannelID, j.ThreadID), true, nil
}

// appCommandResource is the descriptor of command.invoke of an app command (design note 5 c): the
// channel with the target thread as path hint and the facts of the command with its owner.
func appCommandResource(c agents.Command, channel, thread ids.ID) authorize.Resource {
	return authorize.Resource{
		Kind: authorize.ResourceChannel, ID: channel, ThreadID: thread,
		Command: &authorize.CommandFacts{OwnerKind: c.OwnerKind, OwnerID: c.OwnerPrincipalID, RequiredLevel: c.RequiredLevel, Control: c.Control},
	}
}

// appInteractionRecheck builds the descriptor of interaction.create at stage deliver (M2-15 design
// note 8 c: the state is not checked); ok is false when the component or its card is gone.
func (s *Service) appInteractionRecheck(ctx context.Context, j DispatchJob) (authorize.Resource, bool, error) {
	ix, found, err := s.d.Components.Interaction(ctx, j.InteractionID)
	if err != nil || !found {
		return authorize.Resource{}, false, err
	}
	comp, found, err := s.d.Components.Component(ctx, ix.ComponentID)
	if err != nil || !found {
		return authorize.Resource{}, false, err
	}
	card, err := s.d.Messages.TriggerMessageOf(ctx, comp.MessageID)
	if errs.KindOf(err) == errs.KindNotFound {
		return authorize.Resource{}, false, nil
	}
	if err != nil {
		return authorize.Resource{}, false, err
	}
	return appComponentResource(comp, card.ThreadID, card.Author.ID, authorize.InteractionStageDeliver), true, nil
}

// appComponentResource is the descriptor of interaction.create on a component of an app card
// (design note 7 a): no trigger facts, because no agent is triggered.
func appComponentResource(c ComponentRow, thread, author ids.ID, stage string) authorize.Resource {
	return authorize.Resource{
		Kind: authorize.ResourceComponent, ID: c.ID, ThreadID: thread, OwnerID: author,
		Interaction: &authorize.InteractionFacts{
			Stage: stage, OwnerKind: principalKindApp, ComponentState: c.State, ExpiresAt: c.ExpiresAt,
			RequiredCapability: c.RequiredCapability, RestrictedTo: c.RestrictedTo,
		},
	}
}

// failInteraction moves the interaction of an interaction job from received or dispatched to
// failed (compare-and-swap); a command job stores nothing.
func (s *Service) failInteraction(ctx context.Context, j DispatchJob) error {
	if j.Kind != AppDispatchInteraction {
		return nil
	}
	return s.d.UoW.Do(ctx, func(ctx context.Context) error {
		cur, found, err := s.d.Components.LockInteraction(ctx, j.InteractionID)
		if err != nil || !found {
			return err
		}
		if cur.State != domain.InteractionReceived && cur.State != domain.InteractionDispatched {
			return nil
		}
		_, err = s.d.Components.SetInteractionState(ctx, cur.ID, domain.InteractionFailed, nil, s.now())
		return err
	})
}

// markDispatched is step 3 of design note 8: received -> dispatched; a dispatched interaction (a
// rerun) continues; any final state ends the job without a request (proceed false).
func (s *Service) markDispatched(ctx context.Context, id ids.ID) (bool, error) {
	proceed := false
	err := s.d.UoW.Do(ctx, func(ctx context.Context) error {
		cur, found, err := s.d.Components.LockInteraction(ctx, id)
		if err != nil || !found {
			return err
		}
		switch cur.State {
		case domain.InteractionReceived:
			if _, err := s.d.Components.SetInteractionState(ctx, cur.ID, domain.InteractionDispatched, nil, s.now()); err != nil {
				return err
			}
			proceed = true
		case domain.InteractionDispatched:
			proceed = true
		}
		return nil
	})
	return proceed, err
}

// settleInteraction is step 5 of design note 8: the state of the outcome as a compare-and-swap
// from dispatched in a short unit of its own; a state respondToInteraction set meanwhile stays.
func (s *Service) settleInteraction(ctx context.Context, id ids.ID, o runs.AppOutcome) error {
	return s.d.UoW.Do(ctx, func(ctx context.Context) error {
		cur, found, err := s.d.Components.LockInteraction(ctx, id)
		if err != nil || !found {
			return err
		}
		to, change := domain.InteractionStateAfterOutcome(cur.State, domain.AppOutcomeFacts{Answered: o.Answered, Deferred: o.Deferred})
		if !change {
			return nil
		}
		_, err = s.d.Components.SetInteractionState(ctx, cur.ID, to, nil, s.now())
		return err
	})
}

// recheckPrincipal is the principal of the re-check of j: a person as the principal of the check
// without a request (R-AZ-29); an agent or app that ran an app command (design note 5 g) as itself
// without a credential, so its status, installation and grants of this moment decide. The kind
// comes from the directory; an unknown principal is checked as a person, which authz refuses.
func (s *Service) recheckPrincipal(ctx context.Context, j DispatchJob) (actor.Actor, error) {
	refs, err := s.d.Messages.PrincipalRefs(ctx, []ids.ID{j.PersonID})
	if err != nil {
		return actor.Actor{}, err
	}
	switch actor.Kind(refs[j.PersonID].Kind) {
	case actor.KindAgent:
		return actor.Actor{ID: j.PersonID, Kind: actor.KindAgent}, nil
	case actor.KindApp:
		return actor.Actor{ID: j.PersonID, Kind: actor.KindApp, InstallationID: j.PersonInstallationID}, nil
	case actor.KindUser, actor.KindHost, actor.KindAnonymous:
	}
	return actor.Recheck(j.PersonID), nil
}
