// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"slices"

	"github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
)

// eventTarget returns the resource the receive action act decides for the event descriptor r
// (authorize.ResourceFromEvent, M1-63): r itself when act takes its kind, otherwise the first
// object act takes among the target of a grant (grant.read takes space, channel, host or the
// principal of an instance grant, not the grant) and the thread, channel and space of the event's
// scope (pin.added names a message, thread.read takes the thread; command.changed names a command,
// command.read takes the channel). found is false when no object fits, which the caller answers
// as not_found: an event nobody can be shown is filtered, never an internal error that would stop
// every catch-up and delivery at its seq; so is an event whose receive action has no requirement
// yet. A resource without Event is returned unchanged.
func (a *Authorizer) eventTarget(ctx context.Context, act authorize.Action, r authorize.Resource) (authorize.Resource, bool, error) {
	if r.Event == nil {
		return r, true, nil
	}
	if deferredAction(act) {
		// The receive action has no requirement before a later etappe (file.read until M3):
		// nobody may receive the event yet, so it is filtered without the error line of Reject.
		return authorize.Resource{}, false, nil
	}
	spec, _, err := a.requirement(act)
	if err != nil {
		return authorize.Resource{}, false, err
	}
	if slices.Contains(spec.Resources, string(r.Kind)) {
		return r, true, nil
	}
	candidates, err := a.eventCandidates(ctx, r)
	if err != nil {
		return authorize.Resource{}, false, err
	}
	for _, c := range candidates {
		if slices.Contains(spec.Resources, string(c.Kind)) {
			return c, true, nil
		}
	}
	return authorize.Resource{}, false, nil
}

// eventCandidates returns the objects an event descriptor r may be decided on, in order: the
// target of its grant (for kind grant), then the thread, channel and space of its scope, and last
// the agent of an audience (EventScope.AudienceID).
func (a *Authorizer) eventCandidates(ctx context.Context, r authorize.Resource) ([]authorize.Resource, error) {
	var out []authorize.Resource
	if r.Kind == authorize.ResourceGrant && !r.ID.IsZero() {
		g, found, err := a.store.GetGrant(ctx, r.ID)
		if err != nil {
			return nil, sourceError("grant store", err)
		}
		if found {
			out = append(out, grantTarget(g)...)
		}
	}
	sc := r.Event
	for _, c := range []authorize.Resource{
		{Kind: authorize.ResourceThread, ID: sc.ThreadID}, {Kind: authorize.ResourceChannel, ID: sc.ChannelID},
		{Kind: authorize.ResourceSpace, ID: sc.SpaceID},
	} {
		if !c.ID.IsZero() {
			out = append(out, c)
		}
	}
	if !sc.AudienceID.IsZero() {
		// A personal event with a receive action on the agent itself (agent.events.consume of
		// agent.invocation): decided on the audience, which the delivery rule matched with the
		// receiver (M2-08).
		out = append(out, authorize.Resource{Kind: authorize.ResourceAgent, ID: sc.AudienceID})
	}
	return out, nil
}

// deferredAction reports a catalogue action whose requirement is deferred to a later etappe.
func deferredAction(act authorize.Action) bool {
	spec, ok := domain.LookupAction(string(act))
	if !ok {
		return false
	}
	req, ok := domain.RequirementOf(spec.ID)
	if !ok {
		return false
	}
	_, deferred := req.Expr.DeferredTo()
	return deferred
}

// grantTarget returns the object a grant is on as a resource of grant.read: its space, channel or
// host, or for a grant on the instance its subject principal (a group subject has none).
func grantTarget(g domain.Grant) []authorize.Resource {
	switch g.Target.Kind {
	case domain.TargetSpace:
		return []authorize.Resource{{Kind: authorize.ResourceSpace, ID: g.Target.SpaceID}}
	case domain.TargetChannel:
		return []authorize.Resource{{Kind: authorize.ResourceChannel, ID: g.Target.ChannelID}}
	case domain.TargetHost:
		return []authorize.Resource{{Kind: authorize.ResourceHost, ID: g.Target.HostID}}
	case domain.TargetInstance:
		if !g.Subject.PrincipalID.IsZero() {
			return []authorize.Resource{{Kind: authorize.ResourcePrincipal, ID: g.Subject.PrincipalID}}
		}
	}
	return nil
}
