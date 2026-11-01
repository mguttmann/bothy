// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"slices"
	"strings"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/authz/domain/previewtext"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// errPreviewRollback ends the unit of work of a preview after its evaluation, so the unit rolls
// back: no grant, no event, no audit row, no counter and no notification survive (R-RE-10, brief
// M1-49 "Dry run"). It never leaves this package.
var errPreviewRollback = errors.New("authz: the preview rolls back")

// The codes of PreviewHint and InvariantViolation (OpenAPI).
const (
	hintReadHorizon    = "read_horizon"
	hintToolClass      = "trigger_blocked_tool_class"
	hintGroupReach     = "group_reach"
	hintExpiryMissing  = "expiry_missing"
	hintNotEffective   = "terminal_not_effective"
	codeI1Violation    = "i1_violation"
	codeI6Violation    = "i6_violation"
	codeLastOwner      = "last_owner"
	hintOrganization   = "Switch the agent to organization first"
	billingOrgSupports = domain.BillingOrganization
)

// PreviewGrant implements authz.Admin (access.preview, 07 section 9.3, brief M1-49). The shape of
// cmd is checked like CreateGrant (400); authorize decides access.preview with the requirement of
// grant.create on the target (404 without read, 403 without owner or manage; never step-up, never
// audit). A delegation limit (07 section 2.6) gives Allowed false with DenialReason and no dry run.
// Otherwise the apply step of CreateGrant runs in a unit of work declared like CreateGrant's; the
// evaluation of the rights check (RightsHook.Evaluate) and the hints are computed inside it, then
// errPreviewRollback rolls everything back. Errors of the apply step (an unknown subject, an
// archived target) are returned as CreateGrant would return them.
func (s *AdminService) PreviewGrant(ctx context.Context, caller actor.Actor, cmd authz.CreateGrantCmd) (authz.GrantPreview, error) {
	ctx, err := withCaller(ctx, caller)
	if err != nil {
		return authz.GrantPreview{}, err
	}
	p, err := s.newPreviewer(caller, cmd)
	if err != nil {
		return authz.GrantPreview{}, err
	}
	if txn.InUnit(ctx) {
		// Inside an ambient unit the rollback of the preview would fail the caller's unit.
		return authz.GrantPreview{}, internal("access.preview runs only outside a unit of work")
	}
	res, err := p.authorize(ctx)
	if err != nil {
		return authz.GrantPreview{}, err
	}
	if p.resolve != nil {
		// Every listed agent with the target action agent.billing_mode.set (M2-21 design note 9).
		if err := p.s.d.Resolver.AuthorizePreview(ctx, caller, p.resolve.AgentIDs); err != nil {
			return authz.GrantPreview{}, err
		}
	}
	denial, err := delegationLimit(ctx, p.x.a, caller, res, p.grant)
	if err != nil {
		return authz.GrantPreview{}, err
	}
	if denial != "" {
		if err := s.targetExists(ctx, p.grant.Target); err != nil {
			return authz.GrantPreview{}, err
		}
		out := authz.GrantPreview{DenialReason: string(denial), Effects: []authz.AccessEffect{},
			InvariantViolations: []authz.InvariantViolation{}, Hints: []authz.PreviewHint{}}
		return p.finish(ctx, out, previewFacts{denial: denial})
	}
	out, facts, err := p.dryRun(ctx)
	if err != nil {
		return authz.GrantPreview{}, err
	}
	return p.finish(ctx, out, facts)
}

// newPreviewer checks the shape of cmd like CreateGrant (400 before authorize) and returns the
// previewer of the proposed grant over the Authorizer and the rights hook of authz.
func (s *AdminService) newPreviewer(caller actor.Actor, cmd authz.CreateGrantCmd) (*previewer, error) {
	eng, err := s.engine()
	if err != nil {
		return nil, err
	}
	hook, err := s.rightsHook()
	if err != nil {
		return nil, err
	}
	now := s.d.Clock.Now()
	reason := strings.TrimSpace(cmd.Reason)
	subject, target, level, caps, err := grantShape(cmd, reason, now)
	if err := joinFields(err, s.checkResolve(cmd.ResolveI1)); err != nil {
		return nil, err
	}
	g := domain.Grant{Subject: subject, Target: target, Level: level, Caps: caps, ExpiresAt: cmd.ExpiresAt, Reason: reason, GrantedBy: caller.ID, GrantedAt: now}
	return &previewer{s: s, hook: hook, x: &explainer{a: eng, t: now, states: map[ids.ID]*principalState{}}, caller: caller, grant: g, targetID: cmd.TargetID,
		resolve: cmd.ResolveI1}, nil
}

// authorize decides access.preview with the requirement of grant.create on the target.
func (p *previewer) authorize(ctx context.Context) (authorize.Resource, error) {
	kind, ok := authorize.ParseResourceKind(string(p.grant.Target.Kind))
	if !ok {
		return authorize.Resource{}, internal("a grant target kind outside the catalogue")
	}
	res := authorize.Resource{Kind: kind, ID: p.targetID, PreviewOf: authorize.ActionGrantCreate}
	return res, p.s.check(ctx, p.caller, authorize.ActionAccessPreview, res)
}

// dryRun runs the apply step of CreateGrant in a unit of work with its rights change, evaluates it
// and rolls it back with errPreviewRollback.
func (p *previewer) dryRun(ctx context.Context) (authz.GrantPreview, previewFacts, error) {
	var out authz.GrantPreview
	var facts previewFacts
	g := p.grant
	err := p.s.d.UoW.Do(ctx, func(ctx context.Context) error {
		row, err := domain.NewGrant(ids.New(), g.Subject, g.Target, g.Level, g.Caps, g.ExpiresAt, g.Reason, g.GrantedBy, g.GrantedAt)
		if err != nil {
			return err
		}
		if err := p.s.createGrant(ctx, row, func(ctx context.Context) error {
			var err error
			facts.switches, err = p.switchPreview(ctx)
			return err
		}); err != nil {
			return err
		}
		ev, err := p.hook.Evaluate(ctx)
		if err != nil {
			return err
		}
		switches := facts.switches
		if out, facts, err = p.evaluate(ctx, ev); err != nil {
			return err
		}
		facts.switches = switches
		return errPreviewRollback
	}, grantRightsChange(g.Subject, g.Target, p.targetID))
	if errors.Is(err, errPreviewRollback) {
		return out, facts, nil
	}
	if err == nil {
		err = internal("the preview unit committed")
	}
	return authz.GrantPreview{}, previewFacts{}, err
}

// grantRightsChange is the rights change of grant.create (CreateGrant and its preview): the
// subject person as affected, the members of a group named by the apply step.
func grantRightsChange(subject domain.Subject, target domain.Target, targetID ids.ID) txn.Option {
	var principals []ids.ID
	if subject.Kind != domain.SubjectGroup {
		principals = append(principals, subject.PrincipalID)
	}
	return txn.DeclareRightsChange(txn.RightsChangeSpec{
		Action: string(authorize.ActionGrantCreate), Resource: txn.Ref{Kind: string(target.Kind), ID: targetID}, Principals: principals,
	})
}

// rightsHook returns AdminDeps.Hook, or Rights when it is the RightsHook.
func (s *AdminService) rightsHook() (*RightsHook, error) {
	if s.d.Hook != nil {
		return s.d.Hook, nil
	}
	if h, ok := s.d.Rights.(*RightsHook); ok {
		return h, nil
	}
	return nil, errs.New(errs.CodeInternalError, "authz: access.preview needs the rights hook of authz")
}

// delegationLimit returns the reason a delegation limit of 07 section 2.6 refuses the grant g to
// caller (CheckCreate with the caller's effective rights on the target, read like authorize), or
// "" when the limits hold.
func delegationLimit(ctx context.Context, a *Authorizer, caller actor.Actor, r authorize.Resource, g domain.Grant) (domain.Reason, error) {
	v, err := a.version(ctx)
	if err != nil {
		return "", err
	}
	t := a.clock.Now()
	sp, d, err := a.principalOf(ctx, caller, string(authorize.ActionAccessPreview), v, t)
	if err != nil {
		return "", err
	}
	if d.Outcome != domain.OutcomeAllow {
		return "", outcome(d).Err()
	}
	res, found, err := a.resolve(ctx, authorize.ActionGrantCreate, r)
	if err != nil {
		return "", err
	}
	if !found {
		return "", errNotFound()
	}
	eff, _ := sp.rights(res.path)
	return domain.CheckCreate(domain.Delegator{IsOwner: eff.Owner, Level: eff.Level, Caps: eff.Caps}, g.Shape()), nil
}

// previewer computes one preview.
type previewer struct {
	s      *AdminService
	hook   *RightsHook
	x      *explainer
	caller actor.Actor
	// grant is the proposed grant without id; targetID the id of its target (zero: instance).
	grant    domain.Grant
	targetID ids.ID
	// callerView is the caller as authorize sees it, loaded by callerReads.
	callerView *actingPrincipal
	// resolve is the guided flow of the request; nil for none (M2-21).
	resolve *authz.ResolveI1
}

// switchLines are the agents of the switch lines of the text with their names (M2-21).
func switchLines(list []previewSwitch, name func(ids.ID) string) []previewtext.Switch {
	var out []previewtext.Switch
	for _, sw := range list {
		out = append(out, previewtext.Switch{Agent: name(sw.agent), OrganizationReady: sw.organizationReady})
	}
	return out
}

// previewSwitch is one agent the previewed request switches from personal (M2-21 design note 9).
type previewSwitch struct {
	agent ids.ID
	// organizationReady reports organization among the agent's billing_modes_supported.
	organizationReady bool
}

// switchPreview runs the switch part of the guided flow inside the dry run (AddAffected and the
// switch, rolled back with the preview) and returns the agents it switched from personal, read
// before the switch, ascending.
func (p *previewer) switchPreview(ctx context.Context) ([]previewSwitch, error) {
	if p.resolve == nil {
		return nil, nil
	}
	var out []previewSwitch
	for _, id := range ascending(p.resolve.AgentIDs) {
		pol, found, err := p.x.a.policies.AgentPolicy(ctx, id)
		if err != nil {
			return nil, sourceError("agent policy source", err)
		}
		if found && pol.BillingMode == domain.BillingPersonal {
			out = append(out, previewSwitch{agent: id, organizationReady: slices.Contains(pol.BillingModesSupported, billingOrgSupports)})
		}
	}
	return out, p.s.d.Resolver.ApplyPreview(ctx, p.caller, p.resolve.AgentIDs, p.resolve.Reason)
}

// previewFacts are the facts of the text that only the dry run knows.
type previewFacts struct {
	denial     domain.Reason
	i1         []authz.InvariantViolation
	toolClass  []ids.ID
	horizon    []authz.PreviewHint
	groupReach *authz.PreviewHint
	// i1Agents are the agents with an I1 violation (never in the abilities line).
	i1Agents []ids.ID
	// i6 are the I6 violations and notEffective the hint terminal_not_effective of a grant on a
	// host (M2-40).
	i6           []authz.I6Conflict
	notEffective previewtext.NotEffective
	// switches are the agents the guided flow of the request switches from personal (M2-21).
	switches []previewSwitch
}

// evaluate turns the evaluation of the rights check into the effects, violations and hints of the
// preview, inside the unit of the dry run (the grant is written there, so every read sees it).
func (p *previewer) evaluate(ctx context.Context, ev RightsEvaluation) (authz.GrantPreview, previewFacts, error) {
	out := authz.GrantPreview{Effects: []authz.AccessEffect{}, InvariantViolations: []authz.InvariantViolation{}, Hints: []authz.PreviewHint{}}
	var facts previewFacts
	if err := p.x.loadChannels(ctx); err != nil {
		return out, facts, err
	}
	effects, err := p.effects(ctx, ev)
	if err != nil {
		return out, facts, err
	}
	out.Effects = effects
	if ev.Violations.LastOwner {
		out.InvariantViolations = append(out.InvariantViolations, authz.InvariantViolation{Code: codeLastOwner, PrincipalIDs: []ids.ID{}})
	}
	for _, c := range ev.Violations.I1 {
		pol, _, err := p.x.a.policies.AgentPolicy(ctx, c.AgentID)
		if err != nil {
			return out, facts, sourceError("agent policy source", err)
		}
		v := authz.InvariantViolation{Code: codeI1Violation, AgentID: c.AgentID, CredentialHolderID: c.CredentialHolderID,
			PrincipalIDs: append([]ids.ID{}, c.PrincipalIDs...), OrganizationReady: slices.Contains(pol.BillingModesSupported, billingOrgSupports),
			Hint: hintOrganization}
		out.InvariantViolations = append(out.InvariantViolations, v)
		facts.i1Agents = append(facts.i1Agents, c.AgentID)
	}
	facts.i1 = out.InvariantViolations
	for _, c := range ev.Violations.I6 {
		out.InvariantViolations = append(out.InvariantViolations, authz.InvariantViolation{Code: codeI6Violation, HostID: c.HostID,
			CredentialHolderID: c.CredentialHolderID, PrincipalIDs: append([]ids.ID{}, c.PrincipalIDs...), Hint: authz.HintI6})
		facts.i6 = append(facts.i6, c)
	}
	hints, err := p.hints(ctx, &facts)
	if err != nil {
		return out, facts, err
	}
	out.Hints = hints
	return out, facts, nil
}

// effects returns one AccessEffect per principal and target of the difference, principals
// ascending.
func (p *previewer) effects(ctx context.Context, ev RightsEvaluation) ([]authz.AccessEffect, error) {
	type pending struct {
		principal ids.ID
		e         domain.Effect
	}
	reached, err := p.reached(ctx)
	if err != nil {
		return nil, err
	}
	var list []pending
	var people []ids.ID
	for _, id := range ev.Principals {
		if !slices.Contains(reached, id) {
			// Another principal of the change, for example a due expiry the change folds in.
			continue
		}
		shown, err := p.shownEffects(ctx, domain.Effects(ev.Before[id], ev.After[id]))
		if err != nil {
			return nil, err
		}
		for _, e := range shown {
			list = append(list, pending{principal: id, e: e})
		}
		people = append(people, id)
	}
	refs, err := p.x.a.principal.Profiles(ctx, people)
	if err != nil {
		return nil, sourceError("principal source", err)
	}
	out := make([]authz.AccessEffect, 0, len(list))
	for _, x := range list {
		ref, ok := refs[x.principal]
		if !ok {
			return nil, internal("an access effect without a principal")
		}
		out = append(out, authz.AccessEffect{Principal: ref.PrincipalRef, TargetKind: x.e.Kind, TargetID: x.e.ID,
			BeforeLevel: levelText(x.e.Before), AfterLevel: levelText(x.e.After), CapabilitiesAdded: x.e.CapsAdded.Names()})
	}
	return out, nil
}

// reached returns the principals the proposed grant reaches: its subject principal, or the
// active members of its subject group.
func (p *previewer) reached(ctx context.Context) ([]ids.ID, error) {
	if p.grant.Subject.Kind == domain.SubjectGroup {
		return p.people(ctx)
	}
	return []ids.ID{p.grant.Subject.PrincipalID}, nil
}

// shows reports whether the effect e belongs to the grant's target (the target itself, a channel
// below it, the space of a channel target) and the caller reads its target (05 rule 2): a space
// through the space or one of its channels.
func (p *previewer) shows(ctx context.Context, e domain.Effect) (bool, error) {
	tg := p.grant.Target
	if e.Kind == domain.AccessKindHost {
		// Only an owner previews a grant on a host, and an owner sees every host (M2-40).
		return tg.Kind == domain.TargetHost && e.ID == tg.HostID, nil
	}
	if e.Kind == domain.AccessKindChannel {
		path, ok := pathOf(p.x.channels, e.ID)
		under := ok && (tg.Kind == domain.TargetInstance || (tg.Kind == domain.TargetSpace && path.SpaceID == tg.SpaceID) ||
			(tg.Kind == domain.TargetChannel && path.ChannelID == tg.ChannelID))
		if !under {
			return false, nil
		}
		return p.callerReads(ctx, path)
	}
	switch tg.Kind {
	case domain.TargetSpace:
		if e.ID != tg.SpaceID {
			return false, nil
		}
	case domain.TargetChannel:
		if c, ok := pathOf(p.x.channels, tg.ChannelID); !ok || c.SpaceID != e.ID {
			return false, nil
		}
	case domain.TargetInstance, domain.TargetHost:
	}
	return p.callerReadsSpace(ctx, e.ID)
}

// shownEffects keeps the effects shows lets through.
func (p *previewer) shownEffects(ctx context.Context, list []domain.Effect) ([]domain.Effect, error) {
	var out []domain.Effect
	for _, e := range list {
		ok, err := p.shows(ctx, e)
		if err != nil {
			return nil, err
		}
		if ok {
			out = append(out, e)
		}
	}
	return out, nil
}

// callerReadsSpace reports read of the caller on the space or one of its channels.
func (p *previewer) callerReadsSpace(ctx context.Context, space ids.ID) (bool, error) {
	if ok, err := p.callerReads(ctx, domain.Path{Kind: domain.PathSpace, SpaceID: space}); ok || err != nil {
		return ok, err
	}
	for _, c := range p.x.channels {
		if c.SpaceID != space {
			continue
		}
		if ok, err := p.callerReads(ctx, c); ok || err != nil {
			return ok, err
		}
	}
	return false, nil
}

// people returns the persons the subject reaches at the time of the preview: the subject person,
// or the active members of a subject group with a membership in effect; agents and apps none.
func (p *previewer) people(ctx context.Context) ([]ids.ID, error) {
	sub := p.grant.Subject
	switch sub.Kind {
	case domain.SubjectUser:
		return []ids.ID{sub.PrincipalID}, nil
	case domain.SubjectGroup:
		ms, err := p.x.a.store.ActiveMembershipsOfGroups(ctx, []ids.ID{sub.GroupID})
		if err != nil {
			return nil, sourceError("grant store", err)
		}
		var out []ids.ID
		for _, m := range ms {
			if !m.Membership.InEffect(m.Group, p.x.t) {
				continue
			}
			st, err := p.x.state(ctx, m.Membership.UserID)
			if err != nil {
				return nil, err
			}
			if st.found && st.facts.Kind == actor.KindUser && st.facts.Status == authz.PrincipalActive {
				out = append(out, m.Membership.UserID)
			}
		}
		return uniqueIDs(out), nil
	case domain.SubjectAgent, domain.SubjectApp:
	}
	return nil, nil
}

// operators returns the active operator agents of the target, ascending: the effective operator
// of a channel; the default operator and the operators of the channels of a space; every operator
// of the instance.
func (p *previewer) operators(ctx context.Context) ([]ids.ID, error) {
	tg := p.grant.Target
	if tg.Kind == domain.TargetHost {
		return nil, nil
	}
	if p.x.channels == nil {
		if err := p.x.loadChannels(ctx); err != nil {
			return nil, err
		}
	}
	var out []ids.ID
	for _, c := range p.x.channels {
		if tg.Kind == domain.TargetInstance || (tg.Kind == domain.TargetSpace && c.SpaceID == tg.SpaceID) ||
			(tg.Kind == domain.TargetChannel && c.ChannelID == tg.ChannelID) {
			out = append(out, c.ChannelOperatorID)
		}
	}
	defaults, err := p.defaultOperators(ctx)
	if err != nil {
		return nil, err
	}
	return p.activeAgents(ctx, uniqueIDs(append(out, defaults...)))
}

// defaultOperators returns the default operators of the target's space, or of every space for
// the instance.
func (p *previewer) defaultOperators(ctx context.Context) ([]ids.ID, error) {
	a, tg := p.x.a, p.grant.Target
	var spaces []ids.ID
	switch tg.Kind {
	case domain.TargetSpace:
		spaces = []ids.ID{tg.SpaceID}
	case domain.TargetInstance:
		var err error
		if spaces, err = a.structure.Spaces(ctx); err != nil {
			return nil, sourceError("structure source", err)
		}
	case domain.TargetChannel, domain.TargetHost:
		return nil, nil
	}
	details, err := a.structure.SpaceDetails(ctx, spaces)
	if err != nil {
		return nil, sourceError("structure source", err)
	}
	out := make([]ids.ID, 0, len(details))
	for _, d := range details {
		out = append(out, d.DefaultOperatorID)
	}
	return out, nil
}

// activeAgents returns the agents of list whose status is active.
func (p *previewer) activeAgents(ctx context.Context, list []ids.ID) ([]ids.ID, error) {
	var out []ids.ID
	for _, id := range list {
		st, err := p.x.state(ctx, id)
		if err != nil {
			return nil, err
		}
		if st.found && st.facts.Kind == actor.KindAgent && st.facts.Status == authz.PrincipalActive {
			out = append(out, id)
		}
	}
	return out, nil
}

// triggers reports a subject that gains trigger: a person or group with level trigger.
func (p *previewer) triggers() bool {
	k := p.grant.Subject.Kind
	return p.grant.Level == domain.LevelTrigger && (k == domain.SubjectUser || k == domain.SubjectGroup)
}

// ownerSubject reports a subject person that is an owner (SR2 does not block it).
func (p *previewer) ownerSubject(ctx context.Context) (bool, error) {
	if p.grant.Subject.Kind != domain.SubjectUser {
		return false, nil
	}
	st, err := p.x.state(ctx, p.grant.Subject.PrincipalID)
	return err == nil && st.found && st.facts.IsOwner && st.facts.Status == authz.PrincipalActive, err
}

// hints returns the hints of 07 section 9.3 in the order tool class, read horizon, group reach,
// missing expiry, and records their facts for the text.
func (p *previewer) hints(ctx context.Context, facts *previewFacts) ([]authz.PreviewHint, error) {
	out := []authz.PreviewHint{}
	people, err := p.people(ctx)
	if err != nil {
		return nil, err
	}
	if p.triggers() {
		trig, err := p.triggerHints(ctx, people, facts)
		if err != nil {
			return nil, err
		}
		out = append(out, trig...)
	}
	if ne, err := p.notEffectiveHint(ctx, people, facts); err != nil {
		return nil, err
	} else if ne != nil {
		out = append(out, *ne)
	}
	if p.grant.Subject.Kind == domain.SubjectGroup {
		h := p.subjectHint(hintGroupReach)
		h.Count, h.HasCount = int64(len(people)), true
		out = append(out, h)
		facts.groupReach = &h
	}
	if p.grant.Subject.Kind == domain.SubjectUser && p.grant.ExpiresAt.IsZero() {
		out = append(out, p.subjectHint(hintExpiryMissing))
	}
	return out, nil
}

// notEffectiveHint is the hint terminal_not_effective of a grant on a host (07 section 2.12 point
// 4, Design note 13 of M2-40): for a person subject when SR10 or SR12 would deny the person on the
// host after the change, with the first failing guard as the cause of the text; for a group
// subject with the number of its active members in effect whom they would deny, absent for none.
// No hint when an I6 violation names the host: the grant is refused anyway.
func (p *previewer) notEffectiveHint(ctx context.Context, people []ids.ID, facts *previewFacts) (*authz.PreviewHint, error) {
	tg := p.grant.Target
	if tg.Kind != domain.TargetHost || slices.ContainsFunc(facts.i6, func(c authz.I6Conflict) bool { return c.HostID == tg.HostID }) {
		return nil, nil
	}
	a := p.x.a
	h, found, err := a.hosts.Host(ctx, tg.HostID)
	if err != nil || !found {
		return nil, wrapHostSource(err)
	}
	var count int64
	var cause domain.Reason
	for _, person := range people {
		r, err := p.denied(ctx, person, h)
		if err != nil {
			return nil, err
		}
		if r != "" {
			count++
			if cause == "" {
				cause = r
			}
		}
	}
	if count == 0 {
		return nil, nil
	}
	hint := p.subjectHint(hintNotEffective)
	facts.notEffective = previewtext.NotEffective{Set: true, Cause: notEffectiveCauses[cause], Count: count}
	if p.grant.Subject.Kind == domain.SubjectGroup {
		hint.Count, hint.HasCount = count, true
	}
	return &hint, nil
}

// notEffectiveCauses maps the guard reasons to the causes of the text.
var notEffectiveCauses = map[domain.Reason]string{
	domain.ReasonTerminalServerHostOwnerOnly: previewtext.CauseServerHost,
	domain.ReasonTerminalNotHostAssignee:     previewtext.CauseNotAssignee,
	domain.ReasonTerminalNoSessionUser:       previewtext.CauseNoSessionUser,
	// The owner account of a host is no usable session user for a person who is no owner (ADR 0073).
	domain.ReasonTerminalOwnerAccountOwnerOnly: previewtext.CauseNoSessionUser,
}

// denied returns the guard of SR10 or SR12 that would deny person on the host h (an active owner
// counts as in a session, like the views of a rights change), or "".
func (p *previewer) denied(ctx context.Context, person ids.ID, h authz.HostFacts) (domain.Reason, error) {
	st, err := p.x.state(ctx, person)
	if err != nil {
		return "", err
	}
	users, err := p.x.a.hosts.SessionUsersOf(ctx, person)
	if err != nil {
		return "", wrapHostSource(err)
	}
	f := domain.TerminalGuardFacts{PrincipalID: person, Owner: st.found && st.facts.IsOwner && st.facts.Status == authz.PrincipalActive,
		Host: hostGuardFacts(h)}
	for _, u := range users {
		if u.HostID == h.ID {
			f.SessionUser = &domain.SessionUserFact{HostID: u.HostID, HostSessionUserID: u.HostSessionUserID, OSUser: u.OSUser, Reported: u.Reported,
				OwnerAccount: u.OwnerAccount}
		}
	}
	return domain.TerminalNotEffective(f), nil
}

// subjectHint is a hint of code naming the subject person or group.
func (p *previewer) subjectHint(code string) authz.PreviewHint {
	h := authz.PreviewHint{Code: code}
	if p.grant.Subject.Kind == domain.SubjectGroup {
		h.GroupID = p.grant.Subject.GroupID
	} else {
		h.PrincipalID = p.grant.Subject.PrincipalID
	}
	return h
}

// triggerHints are the hints of a subject that gains trigger: the tool class of each operator SR2
// blocks for it, then the read horizon.
func (p *previewer) triggerHints(ctx context.Context, people []ids.ID, facts *previewFacts) ([]authz.PreviewHint, error) {
	owner, err := p.ownerSubject(ctx)
	if err != nil {
		return nil, err
	}
	ops, err := p.operators(ctx)
	if err != nil {
		return nil, err
	}
	var out []authz.PreviewHint
	for _, op := range ops {
		class, _, err := p.x.toolClass(ctx, op)
		if err != nil {
			return nil, err
		}
		if domain.SR2Blocks(class, owner) {
			h := p.subjectHint(hintToolClass)
			h.AgentID = op
			out = append(out, h)
			facts.toolClass = append(facts.toolClass, op)
		}
	}
	for _, op := range ops {
		horizon, err := p.readHorizon(ctx, op, people)
		if err != nil {
			return nil, err
		}
		out = append(out, horizon...)
		facts.horizon = append(facts.horizon, horizon...)
	}
	return out, nil
}

// readHorizon returns the hints H1 of operator op (07 section 6): for each channel K in H(op), the
// people of the subject who may trigger op (SR2) but cannot read K after the change; a person
// subject gets one hint per channel, a group subject one per channel with the count of such
// members.
func (p *previewer) readHorizon(ctx context.Context, op ids.ID, people []ids.ID) ([]authz.PreviewHint, error) {
	st, err := p.x.state(ctx, op)
	if err != nil {
		return nil, err
	}
	class, _, err := p.x.toolClass(ctx, op)
	if err != nil {
		return nil, err
	}
	served, err := servedOf(ctx, p.x.a.structure, op)
	if err != nil {
		return nil, err
	}
	var out []authz.PreviewHint
	for _, k := range domain.Horizon(st.snap, op, p.x.channels, served) {
		path, ok := pathOf(p.x.channels, k)
		if !ok {
			continue
		}
		h, err := p.horizonHint(ctx, op, class, path, people)
		if err != nil {
			return nil, err
		}
		if h != nil {
			out = append(out, *h)
		}
	}
	return out, nil
}

// horizonHint is the H1 hint of op and the channel path, or nil when no person of the subject
// lacks read there or the caller cannot read the channel (it is never named to it, 05 rule 2).
func (p *previewer) horizonHint(ctx context.Context, op ids.ID, class string, path domain.Path, people []ids.ID) (*authz.PreviewHint, error) {
	visible, err := p.callerReads(ctx, path)
	if err != nil || !visible {
		return nil, err
	}
	gaps, err := p.unreadable(ctx, path, class, people)
	if err != nil || len(gaps) == 0 {
		return nil, err
	}
	h := p.subjectHint(hintReadHorizon)
	h.AgentID, h.SpaceID, h.ChannelID = op, path.SpaceID, path.ChannelID
	if p.grant.Subject.Kind == domain.SubjectGroup {
		h.Count, h.HasCount = int64(len(gaps)), true
	}
	return &h, nil
}

// callerReads reports whether the caller reads path, with the rights of authorize.
func (p *previewer) callerReads(ctx context.Context, path domain.Path) (bool, error) {
	if p.callerView == nil {
		a := p.x.a
		v, err := a.version(ctx)
		if err != nil {
			return false, err
		}
		sp, d, err := a.principalOf(ctx, p.caller, string(authorize.ActionAccessPreview), v, a.clock.Now())
		if err != nil {
			return false, err
		}
		if d.Outcome != domain.OutcomeAllow {
			return false, outcome(d).Err()
		}
		p.callerView = sp
	}
	r, _ := p.callerView.rights(path)
	return r.Level >= domain.LevelRead, nil
}

// unreadable returns the people who may trigger an agent of class (SR2) but cannot read path.
func (p *previewer) unreadable(ctx context.Context, path domain.Path, class string, people []ids.ID) ([]ids.ID, error) {
	var out []ids.ID
	for _, person := range people {
		ps, err := p.x.state(ctx, person)
		if err != nil {
			return nil, err
		}
		if !domain.SR2Blocks(class, ps.facts.IsOwner) && domain.Effective(ps.snap, domain.KindUser, person, path).Level < domain.LevelRead {
			out = append(out, person)
		}
	}
	return out, nil
}

// pathOf returns the path of channel among paths.
func pathOf(paths []domain.Path, channel ids.ID) (domain.Path, bool) {
	for _, p := range paths {
		if p.ChannelID == channel {
			return p, true
		}
	}
	return domain.Path{}, false
}

// finish adds Allowed and the text (after the rollback, read outside the unit; names do not
// depend on the grant).
func (p *previewer) finish(ctx context.Context, out authz.GrantPreview, facts previewFacts) (authz.GrantPreview, error) {
	out.Allowed = facts.denial == "" && len(out.InvariantViolations) == 0
	// The dry run's states saw the grant; the text reads fresh.
	p.x.states = map[ids.ID]*principalState{}
	in, err := p.textInput(ctx, facts)
	if err != nil {
		return authz.GrantPreview{}, err
	}
	out.Text, out.TextLocale = previewtext.Render(in)
	return out, nil
}

// textInput gathers the names and facts of the text.
func (p *previewer) textInput(ctx context.Context, facts previewFacts) (previewtext.Input, error) {
	in := previewtext.Input{Level: p.grant.Level, ExpiresAt: p.grant.ExpiresAt, Caps: p.grant.Caps, Denial: facts.denial,
		Subject: previewtext.Subject{Kind: p.grant.Subject.Kind}, Target: previewtext.Target{Kind: p.grant.Target.Kind}}
	ops, err := p.operators(ctx)
	if err != nil {
		return in, err
	}
	name, profiles, err := p.names(ctx, ops, facts)
	if err != nil {
		return in, err
	}
	in.Locale, in.TimeZone = profiles[p.caller.ID].Locale, profiles[p.caller.ID].TimeZone
	if in.Subject.Name, err = p.subjectName(ctx, name); err != nil {
		return in, err
	}
	if in.Target, err = p.targetNames(ctx); err != nil {
		return in, err
	}
	if in.TriggerAgents, err = p.triggerAgents(ctx, ops, facts, name); err != nil {
		return in, err
	}
	for _, v := range facts.i1 {
		if v.Code == codeI1Violation {
			in.I1 = append(in.I1, previewtext.I1{Agent: name(v.AgentID), Holder: name(v.CredentialHolderID)})
		}
	}
	for _, op := range facts.toolClass {
		in.ToolClass = append(in.ToolClass, name(op))
	}
	in.Switches = switchLines(facts.switches, name)
	if err := p.horizonNames(ctx, facts.horizon, name, &in); err != nil {
		return in, err
	}
	if facts.groupReach != nil {
		in.GroupReach = previewtext.Count{Set: true, N: facts.groupReach.Count}
	}
	in.ExpiryMissing = facts.denial == "" && p.grant.Subject.Kind == domain.SubjectUser && p.grant.ExpiresAt.IsZero()
	if p.grant.Target.Kind == domain.TargetHost {
		if err := p.hostNames(ctx, facts, name, &in); err != nil {
			return in, err
		}
	}
	return in, nil
}

// hostNames adds the name of the target host, the I6 violations with the display names of their
// holders and terminal_not_effective to in (M2-40).
func (p *previewer) hostNames(ctx context.Context, facts previewFacts, name func(ids.ID) string, in *previewtext.Input) error {
	hosts := map[ids.ID]string{}
	for _, id := range append([]ids.ID{p.grant.Target.HostID}, i6Hosts(facts.i6)...) {
		h, found, err := p.x.a.hosts.Host(ctx, id)
		if err != nil {
			return wrapHostSource(err)
		}
		hosts[id] = id.String()
		if found {
			hosts[id] = h.Name
		}
	}
	in.Target.Host = hosts[p.grant.Target.HostID]
	for _, c := range facts.i6 {
		in.HostI6 = append(in.HostI6, previewtext.I6{Host: hosts[c.HostID], Holder: name(c.CredentialHolderID)})
	}
	in.NotEffective = facts.notEffective
	return nil
}

// i6Hosts returns the hosts of the conflicts.
func i6Hosts(list []authz.I6Conflict) []ids.ID {
	out := make([]ids.ID, 0, len(list))
	for _, c := range list {
		out = append(out, c.HostID)
	}
	return out
}

// names reads the profiles of the caller, the subject person and every agent and holder the text
// names, and returns the display name lookup (the id for an unknown principal).
func (p *previewer) names(ctx context.Context, ops []ids.ID, facts previewFacts) (func(ids.ID) string, map[ids.ID]authz.PrincipalProfile, error) {
	names := append([]ids.ID{p.caller.ID}, ops...)
	names = append(names, facts.toolClass...)
	for _, v := range facts.i1 {
		names = append(names, v.AgentID, v.CredentialHolderID)
	}
	for _, c := range facts.i6 {
		names = append(names, c.CredentialHolderID)
	}
	for _, h := range facts.horizon {
		names = append(names, h.AgentID)
	}
	for _, sw := range facts.switches {
		names = append(names, sw.agent)
	}
	if p.grant.Subject.Kind != domain.SubjectGroup {
		names = append(names, p.grant.Subject.PrincipalID)
	}
	profiles, err := p.x.a.principal.Profiles(ctx, uniqueIDs(names))
	if err != nil {
		return nil, nil, sourceError("principal source", err)
	}
	return func(id ids.ID) string {
		if pr, ok := profiles[id]; ok {
			return pr.DisplayName
		}
		return id.String()
	}, profiles, nil
}

// triggerAgents are the names of the operators the subject could trigger: without agents with an
// I1 violation and without those SR2 blocks.
func (p *previewer) triggerAgents(ctx context.Context, ops []ids.ID, facts previewFacts, name func(ids.ID) string) ([]string, error) {
	owner, err := p.ownerSubject(ctx)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, op := range ops {
		class, _, err := p.x.toolClass(ctx, op)
		if err != nil {
			return nil, err
		}
		if !domain.SR2Blocks(class, owner) && !slices.Contains(facts.i1Agents, op) {
			out = append(out, name(op))
		}
	}
	return out, nil
}

// subjectName is the display name of the subject, the name of a subject group.
func (p *previewer) subjectName(ctx context.Context, name func(ids.ID) string) (string, error) {
	if p.grant.Subject.Kind != domain.SubjectGroup {
		return name(p.grant.Subject.PrincipalID), nil
	}
	g, found, err := p.x.a.store.GetGroup(ctx, p.grant.Subject.GroupID)
	if err != nil {
		return "", sourceError("grant store", err)
	}
	if !found {
		return p.grant.Subject.GroupID.String(), nil
	}
	return g.Name, nil
}

// targetNames are the names of the target's space and channel.
func (p *previewer) targetNames(ctx context.Context) (previewtext.Target, error) {
	tg := p.grant.Target
	out := previewtext.Target{Kind: tg.Kind}
	space := tg.SpaceID
	if tg.Kind == domain.TargetChannel {
		cd, err := p.x.a.structure.ChannelDetails(ctx, []ids.ID{tg.ChannelID})
		if err != nil {
			return out, sourceError("structure source", err)
		}
		out.Channel, space = cd[tg.ChannelID].Name, cd[tg.ChannelID].SpaceID
	}
	if !space.IsZero() {
		sd, err := p.x.a.structure.SpaceDetails(ctx, []ids.ID{space})
		if err != nil {
			return out, sourceError("structure source", err)
		}
		out.Space = sd[space].Name
	}
	return out, nil
}

// horizonNames adds the names of the H1 hints to in.
func (p *previewer) horizonNames(ctx context.Context, hints []authz.PreviewHint, name func(ids.ID) string, in *previewtext.Input) error {
	if len(hints) == 0 {
		return nil
	}
	var spaces, chans []ids.ID
	for _, h := range hints {
		spaces, chans = append(spaces, h.SpaceID), append(chans, h.ChannelID)
	}
	sd, err := p.x.a.structure.SpaceDetails(ctx, uniqueIDs(spaces))
	if err != nil {
		return sourceError("structure source", err)
	}
	cd, err := p.x.a.structure.ChannelDetails(ctx, uniqueIDs(chans))
	if err != nil {
		return sourceError("structure source", err)
	}
	for _, h := range hints {
		in.ReadHorizon = append(in.ReadHorizon, previewtext.Horizon{Agent: name(h.AgentID), Space: sd[h.SpaceID].Name,
			Channel: cd[h.ChannelID].Name, Count: h.Count})
	}
	return nil
}
