// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"slices"

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
