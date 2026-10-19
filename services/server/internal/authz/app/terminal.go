// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// The terminal rights of M2-40 in the application layer: the session users and personal hosts of
// the snapshot, the facts of the fresh step-up, the quick reattach and the guards, the resolution
// of a terminal_session, the check of the fresh step-up for identity and I6 in VerifyInvariants.

// terminalState adds the terminal part of the rights state of a person to snap (07 SR12, Design
// notes 6 and 7 of M2-40): the valid session user assignments by host and, for an effective
// owner, the holders of the hosts with a personal LLM login. Both change only with a new rights
// version, so the snapshot cache keeps them.
func terminalState(ctx context.Context, hosts authz.HostSource, snap *domain.Snapshot) error {
	users, err := hosts.SessionUsersOf(ctx, snap.PrincipalID)
	if err != nil {
		return sourceError("host source", err)
	}
	snap.SessionUsers = make(map[ids.ID]domain.SessionUserFact, len(users))
	for _, u := range users {
		snap.SessionUsers[u.HostID] = domain.SessionUserFact{HostID: u.HostID, HostSessionUserID: u.HostSessionUserID, OSUser: u.OSUser, Reported: u.Reported}
	}
	if !snap.Owner {
		return nil
	}
	personal, err := hosts.PersonalHosts(ctx)
	if err != nil {
		return sourceError("host source", err)
	}
	snap.PersonalHosts = make(map[ids.ID]ids.ID, len(personal))
	for _, h := range personal {
		snap.PersonalHosts[h.ID] = h.CredentialHolderID
	}
	return nil
}

// resolveTerminalSession resolves a terminal_session through the host of its path hint (Design
// note 3 of M2-40): a zero HostID is an internal error, an unknown host not found.
func (a *Authorizer) resolveTerminalSession(ctx context.Context, r authorize.Resource) (resolved, bool, error) {
	if r.HostID.IsZero() {
		return resolved{}, false, internal("terminal_session resource without its host")
	}
	return a.resolveHost(ctx, r.HostID)
}

// terminalFacts fills the facts of the fresh step-up, of the quick reattach and of the guards
// SR10 to SR12 (Design notes 4 and 5 of M2-40). Errors: terminal.attach without Resource.Terminal
// and a terminal action without its host are internal errors.
func (a *Authorizer) terminalFacts(s *actingPrincipal, spec domain.ActionSpec, r authorize.Resource, in *domain.Input) error {
	if spec.StepUpFresh {
		in.Fresh = domain.FreshFacts{
			StepUpAt: s.p.StepUpTime(), StepUpConsumedAt: s.p.StepUpConsumedTime(),
			StepUpMethod: s.p.StepUpMethodName(), ClientComponent: s.p.ClientComponentName(),
		}
		in.FreshMaxAge, in.ReattachWindow = a.cfg.TerminalStepUpMaxAge, a.cfg.QuickReattachWindow
	}
	if spec.ID == domain.ActionTerminalAttach {
		if r.Terminal == nil {
			return internal("terminal.attach without the facts of its session")
		}
		in.Reattach = &domain.ReattachFacts{
			Action: spec.ID, PrincipalDeviceID: s.ref.DeviceID, PrincipalSessionID: s.ref.SessionID, ClientComponent: s.p.ClientComponentName(),
			Session: domain.TerminalSessionFacts{
				SessionState: r.Terminal.SessionState, SessionDeviceID: r.Terminal.SessionDeviceID,
				SessionAuthSessionID: r.Terminal.SessionAuthSessionID, DetachedAt: r.Terminal.DetachedAt,
			},
		}
	}
	if spec.ID != domain.ActionTerminalOpen && spec.ID != domain.ActionTerminalAttach {
		return nil
	}
	if in.Host == nil {
		return internal(spec.ID + " without its host")
	}
	g := &domain.TerminalGuardFacts{PrincipalID: s.id, Owner: in.Eff.Owner, Host: *in.Host}
	if u, ok := s.entry.snap.SessionUsers[in.Host.ID]; ok {
		g.SessionUser = &u
	}
	in.Terminal = g
	return nil
}

// FreshCheck implements authz.FreshStepUpCheck with domain.FreshStepUp and
// terminal.step_up_max_age, the rule of step 8 of authorize (Design note 4 of M2-40); internal/wire
// passes it to identity for StepUp.ConsumeFresh.
type FreshCheck struct {
	// MaxAge is terminal.step_up_max_age.
	MaxAge time.Duration `config:"terminal.step_up_max_age"`
}

// Compile-time check: the port of authz.
var _ authz.FreshStepUpCheck = FreshCheck{}

// Fresh implements authz.FreshStepUpCheck.
func (c FreshCheck) Fresh(f authz.FreshStepUpFacts, now time.Time) bool {
	return domain.FreshStepUp(domain.FreshFacts{
		StepUpAt: f.StepUpAt, StepUpConsumedAt: f.StepUpConsumedAt, StepUpMethod: f.StepUpMethod, ClientComponent: f.ClientComponent,
	}, now, c.MaxAge)
}

// hostGuardFacts returns the stored facts of h for the guards (domain.HostFacts without the
// atoms).
func hostGuardFacts(h authz.HostFacts) domain.HostFacts {
	return domain.HostFacts{
		ID: h.ID, AssignedUserID: h.AssignedUserID, LLMCredentials: h.LLMCredentials, CredentialHolderID: h.CredentialHolderID,
		DeclaredServerHost: h.DeclaredServerHost, ReportedIsServerHost: h.ReportedIsServerHost,
	}
}

// verifyI6 checks I6 at t for every host with a personal LLM login, inside the ambient
// transaction (03 section 6.8, 24 section 7.3 point 1, Design note 9 of M2-40): the grants on the
// host that are valid at t (not revoked, no expiry at or before t, an active subject, a live
// group) and the active members in effect of their groups.
func (h *RightsHook) verifyI6(ctx context.Context, t time.Time) ([]authz.I6Conflict, error) {
	hosts, err := h.d.Hosts.PersonalHosts(ctx)
	if err != nil || len(hosts) == 0 {
		return nil, wrapHostSource(err)
	}
	hostIDs := make([]ids.ID, 0, len(hosts))
	for _, x := range hosts {
		hostIDs = append(hostIDs, x.ID)
	}
	grants, err := h.d.Store.ActiveGrantsOnHosts(ctx, hostIDs)
	if err != nil {
		return nil, sourceError("grant store", err)
	}
	var out []authz.I6Conflict
	for _, host := range hosts {
		valid, members, err := h.validHostGrants(ctx, host.ID, grants, t)
		if err != nil {
			return nil, err
		}
		if c, bad := domain.CheckI6(hostGuardFacts(host), valid, members); bad {
			out = append(out, authz.I6Conflict{HostID: c.HostID, CredentialHolderID: c.CredentialHolderID, PrincipalIDs: c.PrincipalIDs})
		}
	}
	return out, nil
}

// validHostGrants returns the grants of all on hostID that are valid at t and the active members
// in effect of their groups.
func (h *RightsHook) validHostGrants(ctx context.Context, hostID ids.ID, all []domain.Grant, t time.Time) ([]domain.HostGrant, map[ids.ID][]ids.ID, error) {
	var valid []domain.HostGrant
	members := map[ids.ID][]ids.ID{}
	for _, g := range all {
		if g.Target.Kind != domain.TargetHost || g.Target.HostID != hostID || !g.InEffect(t) {
			continue
		}
		hg, people, ok, err := h.validHostGrant(ctx, g, t)
		if err != nil {
			return nil, nil, err
		}
		if !ok {
			continue
		}
		valid = append(valid, hg)
		if g.Subject.Kind == domain.SubjectGroup {
			members[g.Subject.GroupID] = people
		}
	}
	return valid, members, nil
}

// validHostGrant decides whether the grant g in effect at t is valid by its subject (07 section
// 2.5: an active principal, a live group) and returns it as a domain.HostGrant with the active
// members in effect of a group subject.
func (h *RightsHook) validHostGrant(ctx context.Context, g domain.Grant, t time.Time) (domain.HostGrant, []ids.ID, bool, error) {
	if g.Subject.Kind == domain.SubjectGroup {
		people, live, err := h.groupMembers(ctx, g.Subject.GroupID, t)
		if err != nil || !live {
			return domain.HostGrant{}, nil, false, err
		}
		return domain.HostGrant{SubjectKind: string(domain.SubjectGroup), SubjectID: g.Subject.GroupID}, people, true, nil
	}
	active, err := h.active(ctx, g.Subject.PrincipalID)
	if err != nil || !active {
		return domain.HostGrant{}, nil, false, err
	}
	return domain.HostGrant{SubjectKind: string(g.Subject.Kind), SubjectID: g.Subject.PrincipalID}, nil, true, nil
}

// groupMembers returns the active members in effect at t of group and whether the group is live.
func (h *RightsHook) groupMembers(ctx context.Context, group ids.ID, t time.Time) ([]ids.ID, bool, error) {
	g, found, err := h.d.Store.GetGroup(ctx, group)
	if err != nil {
		return nil, false, sourceError("grant store", err)
	}
	if !found || !g.DeletedAt.IsZero() {
		return nil, false, nil
	}
	ms, err := h.d.Store.ActiveMembershipsOfGroups(ctx, []ids.ID{group})
	if err != nil {
		return nil, false, sourceError("grant store", err)
	}
	var out []ids.ID
	for _, m := range ms {
		if !m.Membership.InEffect(m.Group, t) {
			continue
		}
		active, err := h.active(ctx, m.Membership.UserID)
		if err != nil {
			return nil, false, err
		}
		if active {
			out = append(out, m.Membership.UserID)
		}
	}
	return out, true, nil
}

// active reports a principal with status active.
func (h *RightsHook) active(ctx context.Context, id ids.ID) (bool, error) {
	f, found, err := h.d.Principals.Principal(ctx, id)
	if err != nil {
		return false, sourceError("principal source", err)
	}
	return found && f.Status == authz.PrincipalActive, nil
}

// wrapHostSource wraps a host source error; nil stays nil.
func wrapHostSource(err error) error {
	if err == nil {
		return nil
	}
	return sourceError("host source", err)
}
