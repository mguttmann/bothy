// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"slices"
	"time"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// The target host of M2-39 (07 section 2.12): a host lies outside the tree instance, space,
// channel. Its facts come fresh from the HostSource of the terminal module; the rights on it are
// TerminalOn of the snapshot and the holder of the host.

// noHosts is the HostSource of a world without the terminal module: no host exists, so every host
// resource is not found and no scope lists a host (fail closed).
type noHosts struct{}

func (noHosts) Host(context.Context, ids.ID) (authz.HostFacts, bool, error) {
	return authz.HostFacts{}, false, nil
}
func (noHosts) HostIDs(context.Context) ([]ids.ID, error)                { return nil, nil }
func (noHosts) AssignedTo(context.Context, ids.ID) ([]ids.ID, error)     { return nil, nil }
func (noHosts) PersonalHosts(context.Context) ([]authz.HostFacts, error) { return nil, nil }
func (noHosts) SessionUsersOf(context.Context, ids.ID) ([]authz.SessionUserFact, error) {
	return nil, nil
}

// hostsOrNone returns h, or noHosts for nil.
func hostsOrNone(h authz.HostSource) authz.HostSource {
	if h == nil {
		return noHosts{}
	}
	return h
}

// resolveHost resolves a host resource through the host source; an unknown or zero id is not
// found (no_grant).
func (a *Authorizer) resolveHost(ctx context.Context, id ids.ID) (resolved, bool, error) {
	if id.IsZero() {
		return resolved{}, false, nil
	}
	h, ok, err := a.hosts.Host(ctx, id)
	if err != nil {
		return resolved{}, false, sourceError("host source", err)
	}
	if !ok {
		return resolved{}, false, nil
	}
	return resolved{path: domain.Path{Kind: domain.PathNone}, host: &h}, true, nil
}

// hostView reports what one snapshot says about host h with holder holder: whether the
// principal sees it (owner, holder or terminal), is its holder and holds terminal on it.
func hostView(snap domain.Snapshot, principal ids.ID, h authz.HostFacts) (visible, assignee, terminal bool) {
	assignee = !principal.IsZero() && h.AssignedUserID == principal
	terminal = domain.TerminalOn(snap, h.ID)
	return snap.Owner || assignee || terminal, assignee, terminal
}

// hostFacts computes the facts of a host resource for s (07 section 2.12 point 5): visible when
// the principal sees the host and, with "view as", the viewed person sees it too. The atoms are
// intersected the same way; "view as" is open to owners only, whose own view holds every host.
func hostFacts(s *actingPrincipal, h authz.HostFacts) *domain.HostFacts {
	visible, assignee, terminal := hostView(s.entry.snap, s.id, h)
	if s.viewAs != nil {
		vv, va, vt := hostView(s.viewAs.snap, s.viewAs.key.principal, h)
		visible, assignee, terminal = visible && vv, (assignee || s.entry.snap.Owner) && va, terminal && vt
	}
	f := hostGuardFacts(h)
	f.Visible, f.Assignee, f.Terminal = visible, assignee, terminal
	return &f
}

// hostSet is the set of hosts one snapshot reads: every host for an effective owner (all), else
// the hosts with terminal and the own hosts of the holder.
type hostSet struct {
	all bool
	ids map[ids.ID]bool
}

// hostSetOf reads the host set of the principal with snap from the host source.
func (a *Authorizer) hostSetOf(ctx context.Context, snap domain.Snapshot, principal ids.ID) (hostSet, error) {
	if snap.Owner {
		return hostSet{all: true}, nil
	}
	set := hostSet{ids: map[ids.ID]bool{}}
	for h := range snap.Hosts {
		if domain.TerminalOn(snap, h) {
			set.ids[h] = true
		}
	}
	own, err := a.hosts.AssignedTo(ctx, principal)
	if err != nil {
		return hostSet{}, sourceError("host source", err)
	}
	for _, h := range own {
		set.ids[h] = true
	}
	return set, nil
}

// buildHostScope is the scope of host.read (07 section 7.4 rule 1): AllHosts for an effective
// owner, otherwise the ascending host ids; with "view as" the intersection with the viewed
// person's set.
func (a *Authorizer) buildHostScope(ctx context.Context, s *actingPrincipal, v int64) (authorize.Scope, error) {
	own, err := a.hostSetOf(ctx, s.entry.snap, s.id)
	if err != nil {
		return authorize.Scope{}, err
	}
	if s.viewAs != nil {
		other, err := a.hostSetOf(ctx, s.viewAs.snap, s.viewAs.key.principal)
		if err != nil {
			return authorize.Scope{}, err
		}
		own = intersectHosts(own, other)
	}
	sc := authorize.Scope{PermissionVersion: v, AllHosts: own.all}
	for h := range own.ids {
		sc.HostIDs = append(sc.HostIDs, h)
	}
	sc.HostIDs = domain.SortIDs(sc.HostIDs)
	return sc, nil
}

// intersectHosts returns the hosts both sets read.
func intersectHosts(x, y hostSet) hostSet {
	switch {
	case x.all:
		return y
	case y.all:
		return x
	}
	out := hostSet{ids: map[ids.ID]bool{}}
	for h := range x.ids {
		if y.ids[h] {
			out.ids[h] = true
		}
	}
	return out
}

// isHostList reports the list action of hosts, which Scope serves besides the channel lists.
func isHostList(spec domain.ActionSpec) bool {
	return spec.ID == string(authorize.ActionHostRead) && slices.Equal(spec.Resources, []string{string(authorize.ResourceHost)})
}

// authorizeHost decides host.connect and every call of a host principal (M2-43 Design note 4, 07
// section 3): a host principal is never looked up in principals and has an empty snapshot. In this
// order: a principal of another kind (or none) on host.connect is refused like at the human gate
// (unauthenticated without a principal, otherwise the liveness of the principal, then forbidden
// subject_kind_not_allowed); a host principal is not authenticated for any action but
// host.connect (unauthenticated); it needs Resource.HostAuth (missing or zero is an internal
// error); a proof that failed is unauthenticated host_auth_failed; its own host must exist (else
// host_auth_failed) and be confirming or active (else unauthenticated principal_inactive); the
// resource must be its own row (else not_found, the visibility of a host principal).
func (a *Authorizer) authorizeHost(ctx context.Context, p authorize.Principal, spec domain.ActionSpec, req domain.Expr,
	r authorize.Resource, t time.Time) (authorize.Decision, error) {
	if p == nil || p.PrincipalKind() == domain.KindAnonymous {
		return outcome(domain.Decision{Outcome: domain.OutcomeUnauthenticated, Reason: domain.ReasonUnauthenticated}), nil
	}
	if p.PrincipalKind() != domain.KindHost {
		if _, d, err := a.acting(ctx, p, spec.ID, t); err != nil || d.Outcome != domain.OutcomeAllow {
			return outcome(d), err
		}
		return outcome(domain.Decision{Outcome: domain.OutcomeForbidden, Reason: domain.ReasonSubjectKindNotAllowed}), nil
	}
	if !req.IsHostSelf() {
		// A host principal is authenticated for host.connect only (fail closed, 07 section 3).
		return outcome(domain.Decision{Outcome: domain.OutcomeUnauthenticated, Reason: domain.ReasonUnauthenticated}), nil
	}
	return a.hostSelf(ctx, p.PrincipalID(), r)
}

// hostSelf is the decision of host.connect for the host principal id on r: the verdict of the
// proof, the state of its own host and the visibility of only its own row.
func (a *Authorizer) hostSelf(ctx context.Context, id ids.ID, r authorize.Resource) (authorize.Decision, error) {
	if r.HostAuth == nil || r.HostAuth.Verdict == authorize.HostAuthInvalidZero {
		return authorize.Decision{}, internal("a host principal needs the verdict of its proof")
	}
	failed := domain.Decision{Outcome: domain.OutcomeUnauthenticated, Reason: domain.ReasonHostAuthFailed}
	if r.HostAuth.Verdict != authorize.HostAuthValid {
		return outcome(failed), nil
	}
	own, found, err := a.hosts.Host(ctx, id)
	if err != nil {
		return authorize.Decision{}, sourceError("host source", err)
	}
	if !found {
		return outcome(failed), nil
	}
	if own.State != hostConfirming && own.State != hostActive {
		return outcome(domain.Decision{Outcome: domain.OutcomeUnauthenticated, Reason: domain.ReasonPrincipalInactive}), nil
	}
	if r.Kind != authorize.ResourceHost || r.ID != id {
		return outcome(domain.Decision{Outcome: domain.OutcomeNotFound, Reason: domain.ReasonNoGrant}), nil
	}
	return authorize.Decision{Outcome: authorize.OutcomeAllow}, nil
}

// The host states in which its host service may log in (24 section 3.1): keys stored and not
// disabled or retired.
const (
	hostConfirming = "confirming"
	hostActive     = "active"
)
