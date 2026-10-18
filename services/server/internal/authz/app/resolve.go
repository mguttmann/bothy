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

// resolved is the place of a resource, read fresh from the structure (brief M1-28 "Resolution").
type resolved struct {
	path    domain.Path
	channel authz.ChannelFacts
	// grant is the stored grant of a grant resource; grantTarget its target kind.
	grant       *domain.Grant
	grantTarget domain.TargetKind
}

// pick returns the one id of a resource and its hint: both set and different is a contradiction.
func pick(id, hint ids.ID, what string) (ids.ID, error) {
	switch {
	case id.IsZero():
		return hint, nil
	case hint.IsZero() || hint == id:
		return id, nil
	}
	return ids.ID{}, internal("contradictory " + what + " hints")
}

// resolve returns the path of r; found is false when its thread, channel, space or grant does not
// exist (not_found no_grant). The structure decides the path; a hint that contradicts it is an
// internal error. act decides whether a revoked grant resolves (grant.revoke only).
func (a *Authorizer) resolve(ctx context.Context, act authorize.Action, r authorize.Resource) (resolved, bool, error) {
	kind := string(r.Kind)
	switch {
	case domain.IsChannelPathKind(kind):
		threadID, channelID := r.ThreadID, r.ChannelID
		var err error
		if r.Kind == authorize.ResourceThread {
			if threadID, err = pick(r.ID, r.ThreadID, "thread"); err != nil {
				return resolved{}, false, err
			}
		}
		if r.Kind == authorize.ResourceChannel {
			if channelID, err = pick(r.ID, r.ChannelID, "channel"); err != nil {
				return resolved{}, false, err
			}
		}
		return a.resolveChannel(ctx, threadID, channelID, r.SpaceID)
	case r.Kind == authorize.ResourceSpace:
		id, err := pick(r.ID, r.SpaceID, "space")
		if err != nil {
			return resolved{}, false, err
		}
		return a.resolveSpace(ctx, id)
	case r.Kind == authorize.ResourceGrant:
		return a.resolveGrant(ctx, r.ID, act == authorize.ActionGrantRevoke)
	case r.Kind == authorize.ResourceHost:
		return resolved{path: domain.Path{Kind: domain.PathNone}}, true, nil
	}
	return resolved{path: domain.Path{Kind: domain.PathInstance}}, true, nil
}

// resolveSpace resolves a space; an archived space caps every level at read (07 section 4.5).
func (a *Authorizer) resolveSpace(ctx context.Context, id ids.ID) (resolved, bool, error) {
	if id.IsZero() {
		return resolved{}, false, internal("space resource without an id")
	}
	ok, err := a.structure.Space(ctx, id)
	if err != nil || !ok {
		return resolved{}, false, wrapSource(err)
	}
	archived, err := a.structure.SpaceArchived(ctx, id)
	if err != nil {
		return resolved{}, false, sourceError("structure source", err)
	}
	return resolved{path: domain.Path{Kind: domain.PathSpace, SpaceID: id, Archived: archived}}, true, nil
}

// wrapSource wraps a structure source error; nil stays nil.
func wrapSource(err error) error {
	if err == nil {
		return nil
	}
	return sourceError("structure source", err)
}

// resolveChannel resolves a thread or channel and the channel's space.
func (a *Authorizer) resolveChannel(ctx context.Context, threadID, channelID, spaceHint ids.ID) (resolved, bool, error) {
	var th authz.ThreadFacts
	if !threadID.IsZero() {
		t, ok, err := a.structure.Thread(ctx, threadID)
		if err != nil {
			return resolved{}, false, sourceError("structure source", err)
		}
		if !ok {
			return resolved{}, false, nil
		}
		if !channelID.IsZero() && channelID != t.ChannelID {
			return resolved{}, false, internal("contradictory channel hint")
		}
		th, channelID = t, t.ChannelID
	}
	if channelID.IsZero() {
		return resolved{}, false, internal("resource without a thread or channel")
	}
	ch, ok, err := a.structure.Channel(ctx, channelID)
	if err != nil {
		return resolved{}, false, sourceError("structure source", err)
	}
	if !ok {
		return resolved{}, false, nil
	}
	if !spaceHint.IsZero() && spaceHint != ch.SpaceID {
		return resolved{}, false, internal("contradictory space hint")
	}
	path := channelPath(ch)
	if !th.ID.IsZero() {
		path.ThreadID, path.ThreadMain, path.ThreadOperatorID, path.ThreadCreatedBy = th.ID, th.Main, th.OperatorAgentID, th.CreatedBy
	}
	return resolved{path: path, channel: ch}, true, nil
}

// channelPath is the path of channel facts without a thread.
func channelPath(ch authz.ChannelFacts) domain.Path {
	return domain.Path{
		Kind: domain.PathChannel, SpaceID: ch.SpaceID, ChannelID: ch.ID, DM: ch.Kind == authz.ChannelDM,
		DMParticipants: ch.DMParticipants, ReadOnly: ch.ReadOnly, Archived: ch.Archived, ChannelOperatorID: ch.EffectiveOperatorID,
	}
}

// resolveGrant loads a grant and resolves the path of its target. An unknown grant is not found,
// a revoked one too unless revoked is set: grant.revoke of a revoked grant is a success without
// change (06 section 2 "No effect"), so it is judged like the revocation of the stored grant
// (docs/DECISIONS.md M1-29). A host target has no path (07 section 2.12).
func (a *Authorizer) resolveGrant(ctx context.Context, id ids.ID, revoked bool) (resolved, bool, error) {
	g, ok, err := a.store.GetGrant(ctx, id)
	if err != nil {
		return resolved{}, false, sourceError("grant store", err)
	}
	if !ok || (!g.RevokedAt.IsZero() && !revoked) {
		return resolved{}, false, nil
	}
	var res resolved
	switch g.Target.Kind {
	case domain.TargetInstance:
		res = resolved{path: domain.Path{Kind: domain.PathInstance}}
	case domain.TargetSpace:
		var found bool
		if res, found, err = a.resolveSpace(ctx, g.Target.SpaceID); err != nil || !found {
			return resolved{}, found, err
		}
	case domain.TargetChannel:
		var found bool
		if res, found, err = a.resolveChannel(ctx, ids.ID{}, g.Target.ChannelID, ids.ID{}); err != nil || !found {
			return resolved{}, found, err
		}
	case domain.TargetHost:
		res = resolved{path: domain.Path{Kind: domain.PathNone}}
	default:
		return resolved{}, false, internal("grant with an unknown target kind")
	}
	res.grant, res.grantTarget = &g, g.Target.Kind
	return res, true, nil
}

// gather builds the fact bundle of domain.Decide for one call: the effective rights on the path
// with the "view as" and acting-for intersections and the facts the action and the resource kind
// need (brief M1-28: decide.go stays pure).
func (a *Authorizer) gather(ctx context.Context, s *actingPrincipal, spec domain.ActionSpec, req domain.Expr,
	r authorize.Resource, res resolved, t time.Time) (domain.Input, error) {
	eff, base := s.rights(res.path)
	in := domain.Input{
		Action: spec.ID, Spec: spec, Req: req, Now: t, StepUpMaxAge: a.cfg.StepUpMaxAge,
		Principal: domain.Principal{
			ID: s.id, Kind: s.kind, HasSession: !s.ref.SessionID.IsZero(), StepUpAt: s.p.StepUpTime(),
			DeviceID: s.ref.DeviceID, GrantedScopes: s.p.AppGrantedScopes(),
			ViewAs: s.viewAs != nil, ActingFor: s.actingFor != nil,
		},
		Resource: domain.ResourceFacts{Kind: string(r.Kind), ID: r.ID, OwnerID: r.OwnerID,
			HasAttachments: r.HasAttachments, RestrictedTags: r.RestrictedTags},
		Path: res.path, Eff: eff, BaseEff: base, GrantTarget: res.grantTarget,
	}
	in.Principal.Pending = pendingReason(s.liveness())
	baseIn := in
	baseIn.Eff = base
	if err := a.kindFacts(ctx, s, req, r, res, &in, &baseIn); err != nil {
		return domain.Input{}, err
	}
	if err := a.grantFacts(spec.ID, r, res, &in); err != nil {
		return domain.Input{}, err
	}
	if spec.ID == string(authorize.ActionTokenCreate) {
		pf, found, err := a.principal.Principal(ctx, r.ID)
		if err != nil {
			return domain.Input{}, sourceError("principal source", err)
		}
		in.Token = &domain.TokenFacts{HolderFound: found, HolderKind: string(pf.Kind)}
	}
	if spec.ID == string(authorize.ActionAgentInvoke) && r.Invoke != nil {
		var err error
		if in.Invoke, err = a.invokeFacts(ctx, s, r.Invoke, res, t); err != nil {
			return domain.Input{}, err
		}
	}
	if in.Principal.ActingFor {
		var err error
		if in.BaseVisible, _, err = domain.Visible(baseIn); err != nil {
			return domain.Input{}, err
		}
	}
	return in, nil
}

// pendingReason is the reason of a pending person or device whose pending gate passed.
func pendingReason(live domain.Liveness) domain.Reason {
	switch {
	case live.Status == string(authz.PrincipalPending):
		return domain.ReasonPrincipalPending
	case live.HasDevice && live.DeviceState == domain.DevicePending:
		return domain.ReasonDevicePending
	}
	return ""
}

// kindFacts fills the facts of the visibility rules and atoms that depend on the resource kind,
// for the final rights (in) and the base rights before acting for (base).
func (a *Authorizer) kindFacts(ctx context.Context, s *actingPrincipal, req domain.Expr, r authorize.Resource,
	res resolved, in, base *domain.Input) error {
	var err error
	if r.Kind == authorize.ResourceSpace && in.Eff.Level < domain.LevelRead {
		if in.SpaceHasVisibleChannel, base.SpaceHasVisibleChannel, err = a.spaceHasVisibleChannel(ctx, s, res.path.SpaceID); err != nil {
			return err
		}
	}
	if r.Kind == authorize.ResourceAgent || req.Contains(domain.ReadOnOperatedChannel) {
		if in.ReadOnOperatedChannel, base.ReadOnOperatedChannel, err = a.readsOperatedChannel(ctx, s, r.ID); err != nil {
			return err
		}
	}
	if r.Kind == authorize.ResourceGroup {
		in.GroupMember = slices.Contains(s.entry.snap.Groups, r.ID) &&
			(s.viewAs == nil || slices.Contains(s.viewAs.snap.Groups, r.ID))
		base.GroupMember = in.GroupMember
	}
	if req.Contains(domain.TargetHasCap(domain.CapManage)) {
		if in.Target, err = a.target(ctx, s, r.TargetChannelID); err != nil {
			return err
		}
		base.Target = in.Target
	}
	return nil
}

// spaceHasVisibleChannel reports read on at least one channel of space (final and base rights).
func (a *Authorizer) spaceHasVisibleChannel(ctx context.Context, s *actingPrincipal, space ids.ID) (bool, bool, error) {
	chans, err := a.structure.Channels(ctx)
	if err != nil {
		return false, false, sourceError("structure source", err)
	}
	var final, base bool
	for _, c := range chans {
		if c.SpaceID != space {
			continue
		}
		f, b := s.rights(channelPath(c))
		final = final || f.Level >= domain.LevelRead
		base = base || b.Level >= domain.LevelRead
	}
	return final, base, nil
}

// readsOperatedChannel reports read on a channel the agent operates (final and base rights).
func (a *Authorizer) readsOperatedChannel(ctx context.Context, s *actingPrincipal, agent ids.ID) (bool, bool, error) {
	if agent.IsZero() {
		return false, false, nil
	}
	list, err := a.structure.ChannelsOperatedBy(ctx, agent)
	if err != nil {
		return false, false, sourceError("structure source", err)
	}
	var final, base bool
	for _, id := range list {
		c, ok, err := a.structure.Channel(ctx, id)
		if err != nil {
			return false, false, sourceError("structure source", err)
		}
		if !ok {
			continue
		}
		f, b := s.rights(channelPath(c))
		final = final || f.Level >= domain.LevelRead
		base = base || b.Level >= domain.LevelRead
	}
	return final, base, nil
}

// target resolves the destination channel of thread.move.
func (a *Authorizer) target(ctx context.Context, s *actingPrincipal, channel ids.ID) (*domain.TargetFacts, error) {
	if channel.IsZero() {
		return nil, internal("thread.move without a target channel")
	}
	res, found, err := a.resolveChannel(ctx, ids.ID{}, channel, ids.ID{})
	if err != nil {
		return nil, err
	}
	if !found {
		return &domain.TargetFacts{}, nil
	}
	f, _ := s.rights(res.path)
	visible := f.Level >= domain.LevelRead
	if res.path.DM {
		visible = res.path.Participant(s.id) && f.Level >= domain.LevelRead
	}
	return &domain.TargetFacts{Visible: visible, Eff: f}, nil
}

// grantFacts fills the grant guard facts: the new grant of grant.create, whose target must equal
// the resource, and the stored grant of grant.revoke.
func (a *Authorizer) grantFacts(action string, r authorize.Resource, res resolved, in *domain.Input) error {
	switch action {
	case string(authorize.ActionGrantCreate):
		if r.Grant == nil {
			return internal("grant.create without the grant")
		}
		shape, target, err := shapeOf(r.Grant)
		if err != nil {
			return err
		}
		want := domain.Target{Kind: domain.TargetKind(r.Kind)}
		switch want.Kind {
		case domain.TargetSpace:
			want.SpaceID = res.path.SpaceID
		case domain.TargetChannel:
			want.ChannelID = res.path.ChannelID
		case domain.TargetHost:
			want.HostID = r.ID
		case domain.TargetInstance:
		}
		if target != want {
			return internal("grant.create: the grant's target is not the resource")
		}
		in.Grant = &domain.GrantFacts{Shape: shape}
	case string(authorize.ActionGrantRevoke):
		if res.grant == nil {
			return internal("grant.revoke without a stored grant")
		}
		in.Grant = &domain.GrantFacts{Shape: res.grant.Shape()}
	}
	return nil
}

// shapeOf parses the grant facts of a descriptor.
func shapeOf(g *authorize.GrantFacts) (domain.GrantShape, domain.Target, error) {
	lvl := domain.LevelNone
	if g.Level != "" {
		l, err := domain.ParseLevel(g.Level)
		if err != nil {
			return domain.GrantShape{}, domain.Target{}, internal("grant facts with an invalid level")
		}
		lvl = l
	}
	caps, err := domain.ParseCaps(g.Caps)
	if err != nil {
		return domain.GrantShape{}, domain.Target{}, internal("grant facts with an invalid capability")
	}
	tg := domain.Target{Kind: domain.TargetKind(g.TargetKind), SpaceID: g.TargetSpaceID, ChannelID: g.TargetChannelID, HostID: g.TargetHostID}
	return domain.GrantShape{SubjectKind: domain.SubjectKind(g.SubjectKind), TargetKind: tg.Kind, Level: lvl, Caps: caps}, tg, nil
}

// invokeFacts gathers the facts of the agent.invoke guard.
func (a *Authorizer) invokeFacts(ctx context.Context, s *actingPrincipal, f *authorize.InvokeFacts, res resolved, t time.Time) (*domain.InvokeFacts, error) {
	ch := res.channel
	out := &domain.InvokeFacts{
		AgentID: f.AgentID, TriggerOrigin: f.TriggerOrigin, MentionsAgent: f.MentionsAgent, ActiveRunAgentID: f.ActiveRunAgentID,
		RequireMention: ch.RequireMention, ThreadRequireMention: ch.ThreadRequireMention, FreeResponse: ch.FreeResponse,
		AllowMentionedAgents: ch.AllowMentionedAgents, ProjectBound: ch.ProjectBound, ProjectReady: ch.ProjectReady,
	}
	if f.AgentID.IsZero() || res.path.Kind != domain.PathChannel {
		return out, nil
	}
	pf, found, err := a.principal.Principal(ctx, f.AgentID)
	if err != nil {
		return nil, sourceError("principal source", err)
	}
	out.AgentFound, out.AgentKind, out.AgentStatus = found, string(pf.Kind), string(pf.Status)
	pol, ok, err := a.policies.AgentPolicy(ctx, f.AgentID)
	if err != nil {
		return nil, sourceError("agent policy source", err)
	}
	out.PolicyFound, out.BillingMode, out.CredentialHolderID = ok, pol.BillingMode, pol.CredentialHolderID
	out.ToolClass, out.ReportedToolClass = pol.ToolClass, pol.ReportedToolClass
	if found && string(pf.Kind) == domain.KindAgent {
		v := s.entry.snap.PermissionVersion
		ae, err := a.load(ctx, authz.CredentialRef{PrincipalID: f.AgentID}, v, t)
		if err != nil {
			return nil, err
		}
		out.AgentLevel = domain.Effective(ae.snap, domain.KindAgent, f.AgentID, res.path).Level
	}
	if out.DevEnvironmentReady, err = a.policies.DevEnvironmentReady(ctx, s.id, res.path.ChannelID); err != nil {
		return nil, sourceError("agent policy source", err)
	}
	return out, nil
}
