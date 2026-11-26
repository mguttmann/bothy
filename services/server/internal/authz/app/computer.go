// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// authorizeComputer decides the agent actions of computer use (computer.desktops.list,
// computer.open, computer.attach; CU-04, 07 2.15 and SR13, 27 5.1 step 5). The principal is
// loaded like every principal (liveness, pending rule and the status of the acting-for binding,
// so an invalid binding or an ended run is invocation_token_invalid before any resource is read);
// then a principal that is no agent is forbidden subject_kind_not_allowed, a descriptor without
// Computer is an internal error, and domain.DecideComputer decides with the owner status of the
// triggering person from the owner list (R-AZ-29, the snapshot of the check without a request that
// the binding loaded) and the visibility of the host for that person. SR9's intersection does not
// apply: the agent has no right on a host of its own, and SR13 is the check of the person (07
// 2.15 item 2).
func (a *Authorizer) authorizeComputer(ctx context.Context, p authorize.Principal, spec domain.ActionSpec, r authorize.Resource,
	t time.Time) (authorize.Decision, error) {
	if p == nil || p.PrincipalKind() == domain.KindAnonymous {
		return outcome(domain.Decision{Outcome: domain.OutcomeUnauthenticated, Reason: domain.ReasonUnauthenticated}), nil
	}
	s, d, err := a.acting(ctx, p, spec.ID, t)
	if err != nil || d.Outcome != domain.OutcomeAllow {
		return outcome(d), err
	}
	if s.kind != domain.KindAgent {
		return outcome(domain.Decision{Outcome: domain.OutcomeForbidden, Reason: domain.ReasonSubjectKindNotAllowed}), nil
	}
	if r.Computer == nil {
		return authorize.Decision{}, internal("a computer action without its facts")
	}
	f, err := a.computerFacts(ctx, s, spec.ID, r)
	if err != nil {
		return authorize.Decision{}, err
	}
	dd, err := domain.DecideComputer(f)
	if err != nil {
		return authorize.Decision{}, err
	}
	return outcome(dd), nil
}

// computerFacts converts the descriptor and adds what authz knows itself: the bound run and
// person of the binding, the owner status of that person now and the visibility of the host for
// that person.
func (a *Authorizer) computerFacts(ctx context.Context, s *actingPrincipal, action string, r authorize.Resource) (domain.ComputerFacts, error) {
	c := r.Computer
	f := domain.ComputerFacts{
		Action: action, AgentID: s.id, RunFound: c.RunFound, RunID: c.RunID, RunAgentID: c.RunAgentID, TriggeredBy: c.TriggeredBy,
		RunActive: c.RunActive, ToolClass: c.ToolClass, TriggerKind: c.TriggerKind, TriggerOrigin: c.TriggerOrigin,
		TriggerAuthorID: c.TriggerAuthorID, TriggeringCommand: c.TriggeringCommand, HostState: c.HostState,
		HostConnected: c.HostConnected, HostListed: c.HostListed,
	}
	for _, fd := range c.Feeders {
		f.Feeders = append(f.Feeders, domain.ComputerFeeder{PrincipalID: fd.PrincipalID, Source: fd.Source})
	}
	if s.binding != authorize.ActingForBound || s.actingFor == nil {
		return f, nil
	}
	f.BindingRunID, f.BindingUserID = s.af.RunID, s.actingForUser
	person := *s.actingFor
	live := actingForLive(person)
	f.PersonOwner = live && person.snap.Owner
	hostID := r.ID
	if r.Kind == authorize.ResourceComputerSession {
		hostID = r.HostID
	}
	if r.Kind != authorize.ResourceHost && r.Kind != authorize.ResourceComputerSession {
		return f, nil
	}
	visible, err := a.hostVisibleTo(ctx, person, live, hostID)
	if err != nil {
		return domain.ComputerFacts{}, err
	}
	f.HostVisible = visible
	return f, nil
}

// hostVisibleTo reports whether the person of e may read the host id (owner, assignee or
// terminal, 07 2.12 point 5); a person who is not live, an unknown host and a zero id are not
// visible.
func (a *Authorizer) hostVisibleTo(ctx context.Context, e entry, live bool, id ids.ID) (bool, error) {
	if !live || id.IsZero() {
		return false, nil
	}
	h, found, err := a.hosts.Host(ctx, id)
	if err != nil {
		return false, sourceError(hostSourceName, err)
	}
	if !found {
		return false, nil
	}
	visible, _, _ := hostView(e.snap, e.key.principal, h)
	return visible, nil
}
