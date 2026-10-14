// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"slices"
	"time"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// ExplainAccess implements authz.Admin (access.explain, 07 section 9.1, brief M1-49): authorize
// first (no read gives 404, read without owner or manage 403, no step-up, no audit), then every
// candidate with its effective rights on the target and their provenance, computed with the
// snapshot code of authorize (readGrants, BuildSnapshot, Effective), never from the cache.
func (s *AdminService) ExplainAccess(ctx context.Context, caller actor.Actor, target authz.AccessTargetRef) (authz.AccessExplanation, error) {
	ctx, err := withCaller(ctx, caller)
	if err != nil {
		return authz.AccessExplanation{}, err
	}
	eng, err := s.engine()
	if err != nil {
		return authz.AccessExplanation{}, err
	}
	kind, ok := authorize.ParseResourceKind(target.Kind)
	if !ok || (kind != authorize.ResourceSpace && kind != authorize.ResourceChannel) || target.ID.IsZero() {
		return authz.AccessExplanation{}, internal("access.explain needs a space or a channel")
	}
	if err := s.check(ctx, caller, authorize.ActionAccessExplain, authorize.Resource{Kind: kind, ID: target.ID}); err != nil {
		return authz.AccessExplanation{}, err
	}
	x := &explainer{a: eng, t: s.d.Clock.Now(), states: map[ids.ID]*principalState{}}
	entries, err := x.explain(ctx, kind, target.ID)
	if err != nil {
		return authz.AccessExplanation{}, err
	}
	return authz.AccessExplanation{TargetKind: string(kind), TargetID: target.ID, Entries: entries}, nil
}

// principalState is a principal with the snapshot input and snapshot of an explanation.
type principalState struct {
	facts authz.PrincipalFacts
	found bool
	in    domain.SnapshotInput
	snap  domain.Snapshot
}

// explainer computes one explanation at one time t.
type explainer struct {
	a        *Authorizer
	t        time.Time
	channels []domain.Path
	states   map[ids.ID]*principalState
}

// state returns the facts and the snapshot of id at x.t, built as for an active principal, so an
// inactive one shows what its grants would give (07 section 9.1); a person counts as in a session
// (the owner role takes effect), as in the views of access.changed.
func (x *explainer) state(ctx context.Context, id ids.ID) (*principalState, error) {
	if st, ok := x.states[id]; ok {
		return st, nil
	}
	facts, found, err := x.a.principal.Principal(ctx, id)
	if err != nil {
		return nil, sourceError("principal source", err)
	}
	st := &principalState{facts: facts, found: found}
	if found {
		kind := string(facts.Kind)
		st.in = domain.SnapshotInput{PrincipalID: id, At: x.t, Active: true, Owner: facts.IsOwner && kind == domain.KindUser}
		if err := readGrants(ctx, x.a.store, id, kind == domain.KindUser, &st.in, x.t); err != nil {
			return nil, err
		}
		st.snap = domain.BuildSnapshot(st.in)
	}
	x.states[id] = st
	return st, nil
}

// loadChannels reads the paths of the channels that are not dm channels.
func (x *explainer) loadChannels(ctx context.Context) error {
	chans, err := x.a.structure.Channels(ctx)
	if err != nil {
		return sourceError("structure source", err)
	}
	x.channels = x.channels[:0]
	for _, c := range chans {
		x.channels = append(x.channels, channelPath(c))
	}
	return nil
}

// explainTarget is the resolved target with its candidates.
type explainTarget struct {
	path domain.Path
	// defaultOperator is the default operator of a space target.
	defaultOperator ids.ID
	// relevant are the channels whose H1 counts: the channel itself, or the channels of a space.
	relevant []domain.Path
}

// explain returns the entries of the target, sorted (domain.SortEntries).
func (x *explainer) explain(ctx context.Context, kind authorize.ResourceKind, id ids.ID) ([]authz.AccessEntry, error) {
	if err := x.loadChannels(ctx); err != nil {
		return nil, err
	}
	tg, err := x.target(ctx, kind, id)
	if err != nil {
		return nil, err
	}
	candidates, agents, err := x.candidates(ctx, tg)
	if err != nil {
		return nil, err
	}
	entries := map[ids.ID]*domain.ExplainEntry{}
	for _, p := range candidates {
		e, err := x.entry(ctx, tg, p)
		if err != nil {
			return nil, err
		}
		if e != nil {
			entries[p] = e
		}
	}
	if err := x.viaAgents(ctx, tg, agents, entries); err != nil {
		return nil, err
	}
	return x.output(ctx, entries)
}

// target resolves the path of the target, the default operator of a space and the channels of H1.
func (x *explainer) target(ctx context.Context, kind authorize.ResourceKind, id ids.ID) (explainTarget, error) {
	if kind == authorize.ResourceChannel {
		res, found, err := x.a.resolveChannel(ctx, ids.ID{}, id, ids.ID{})
		if err != nil {
			return explainTarget{}, err
		}
		if !found {
			return explainTarget{}, errNotFound()
		}
		return explainTarget{path: res.path, relevant: []domain.Path{res.path}}, nil
	}
	tg := explainTarget{path: domain.Path{Kind: domain.PathSpace, SpaceID: id}}
	details, err := x.a.structure.SpaceDetails(ctx, []ids.ID{id})
	if err != nil {
		return explainTarget{}, sourceError("structure source", err)
	}
	tg.defaultOperator = details[id].DefaultOperatorID
	for _, p := range x.channels {
		if p.SpaceID == id {
			tg.relevant = append(tg.relevant, p)
		}
	}
	return tg, nil
}

// candidates returns the principals to explain and the agents whose read horizon counts (brief
// M1-49): the owners, active or not; the subjects of grants on the target's path that are not
// revoked (valid or not) and the members of such groups; the operator of a channel or the
// default operator of a space; the participants of a dm channel. Agents are the agents among
// them and the operators of the relevant channels.
func (x *explainer) candidates(ctx context.Context, tg explainTarget) (people, agents []ids.ID, err error) {
	owners, err := x.a.principal.OwnerIDs(ctx)
	if err != nil {
		return nil, nil, sourceError("principal source", err)
	}
	out := slices.Clone(owners)
	var spaces, channels []ids.ID
	if !tg.path.SpaceID.IsZero() {
		spaces = append(spaces, tg.path.SpaceID)
	}
	if tg.path.Kind == domain.PathChannel {
		channels = append(channels, tg.path.ChannelID)
	}
	subjects, agentSubjects, err := x.grantSubjects(ctx, spaces, channels)
	if err != nil {
		return nil, nil, err
	}
	out, agents = append(out, subjects...), agentSubjects
	out = append(out, tg.path.DMParticipants...)
	for _, op := range []ids.ID{tg.path.ChannelOperatorID, tg.defaultOperator} {
		if !op.IsZero() {
			out, agents = append(out, op), append(agents, op)
		}
	}
	for _, p := range tg.relevant {
		if !p.ChannelOperatorID.IsZero() {
			agents = append(agents, p.ChannelOperatorID)
		}
	}
	return uniqueIDs(out), uniqueIDs(agents), nil
}

// grantSubjects returns the subjects of the grants not revoked on the instance, spaces and
// channels, with the members of subject groups, and the agents among the subjects.
func (x *explainer) grantSubjects(ctx context.Context, spaces, channels []ids.ID) (subjects, agents []ids.ID, err error) {
	grants, err := x.a.store.ActiveGrantsOnTargets(ctx, spaces, channels)
	if err != nil {
		return nil, nil, sourceError("grant store", err)
	}
	var groups []ids.ID
	for _, g := range grants {
		switch g.Subject.Kind {
		case domain.SubjectGroup:
			groups = append(groups, g.Subject.GroupID)
			continue
		case domain.SubjectAgent:
			agents = append(agents, g.Subject.PrincipalID)
		case domain.SubjectUser, domain.SubjectApp:
		}
		subjects = append(subjects, g.Subject.PrincipalID)
	}
	if len(groups) == 0 {
		return subjects, agents, nil
	}
	ms, err := x.a.store.ActiveMembershipsOfGroups(ctx, groups)
	if err != nil {
		return nil, nil, sourceError("grant store", err)
	}
	for _, m := range ms {
		subjects = append(subjects, m.Membership.UserID)
	}
	return subjects, agents, nil
}

// uniqueIDs returns list ascending without duplicates and zero ids.
func uniqueIDs(list []ids.ID) []ids.ID {
	out := slices.DeleteFunc(slices.Clone(list), func(id ids.ID) bool { return id.IsZero() })
	return slices.Compact(domain.SortIDs(out))
}

// entry returns the entry of p on the target, or nil for an unknown principal.
func (x *explainer) entry(ctx context.Context, tg explainTarget, p ids.ID) (*domain.ExplainEntry, error) {
	st, err := x.state(ctx, p)
	if err != nil || !st.found {
		return nil, err
	}
	kind := string(st.facts.Kind)
	e := &domain.ExplainEntry{
		PrincipalID: p, Inactive: st.facts.Status != authz.PrincipalActive,
		Rights:  domain.Effective(st.snap, kind, p, tg.path),
		Sources: domain.Provenance(st.in, kind, tg.path),
	}
	if kind == domain.KindAgent && p == tg.defaultOperator && tg.path.Kind == domain.PathSpace {
		// The default operator of a space operates its inheriting channels: write (R-AZ-13).
		e.Rights.Level = domain.MaxLevel(e.Rights.Level, domain.LevelWrite)
		e.Sources = append(e.Sources, domain.Source{Kind: domain.SourceOperator, AgentID: p})
	}
	if kind == domain.KindUser && e.Rights.Level == domain.LevelTrigger && !e.Rights.Owner {
		op := tg.path.ChannelOperatorID
		if tg.path.Kind == domain.PathSpace {
			op = tg.defaultOperator
		}
		blocked, err := x.hostAgent(ctx, op)
		if err != nil {
			return nil, err
		}
		if blocked {
			e.Sources = append(e.Sources, domain.Source{Kind: domain.SourceGuardBlocked, AgentID: op, Rule: domain.RuleSR2})
		}
	}
	return e, nil
}

// hostAgent reports an agent whose effective tool class is host (SR2); false for none.
func (x *explainer) hostAgent(ctx context.Context, agent ids.ID) (bool, error) {
	if agent.IsZero() {
		return false, nil
	}
	class, ok, err := x.toolClass(ctx, agent)
	return ok && domain.SR2Blocks(class, false), err
}

// toolClass returns the effective tool class of agent; ok is false without a policy.
func (x *explainer) toolClass(ctx context.Context, agent ids.ID) (string, bool, error) {
	pol, ok, err := x.a.policies.AgentPolicy(ctx, agent)
	if err != nil {
		return "", false, sourceError("agent policy source", err)
	}
	return domain.EffectiveToolClass(pol.ToolClass, pol.ReportedToolClass), ok, nil
}

// viaAgents adds the sources via_agent (hint H1, 07 section 6): a person P in T(A) of an active
// agent A whose horizon H(A) holds a relevant channel that P cannot read. A person without an
// entry gets one with its own rights on the target (often none).
func (x *explainer) viaAgents(ctx context.Context, tg explainTarget, agents []ids.ID, entries map[ids.ID]*domain.ExplainEntry) error {
	if len(agents) == 0 || len(tg.relevant) == 0 {
		return nil
	}
	owners, err := x.a.principal.ActiveOwnerIDs(ctx)
	if err != nil {
		return sourceError("principal source", err)
	}
	world, err := triggerWorld(ctx, x.a.store, x.a.principal, owners)
	if err != nil {
		return err
	}
	for _, agent := range agents {
		readers, err := x.horizonGaps(ctx, world, agent, tg.relevant)
		if err != nil {
			return err
		}
		if err := x.addVia(ctx, tg, agent, readers, entries); err != nil {
			return err
		}
	}
	return nil
}

// addVia appends the source via_agent of agent to the entries of readers, creating missing ones.
func (x *explainer) addVia(ctx context.Context, tg explainTarget, agent ids.ID, readers []ids.ID, entries map[ids.ID]*domain.ExplainEntry) error {
	for _, p := range readers {
		e, ok := entries[p]
		if !ok {
			var err error
			if e, err = x.entry(ctx, tg, p); err != nil || e == nil {
				return err
			}
			entries[p] = e
		}
		e.Sources = append(e.Sources, domain.Source{Kind: domain.SourceViaAgent, AgentID: agent})
	}
	return nil
}

// horizonGaps returns the people of T(agent) who cannot read at least one channel of relevant
// that is in H(agent), ascending; none for an agent that is unknown, not active or without a
// policy.
func (x *explainer) horizonGaps(ctx context.Context, world domain.World, agent ids.ID, relevant []domain.Path) ([]ids.ID, error) {
	ks, class, served, err := x.agentReach(ctx, agent, relevant)
	if err != nil || len(ks) == 0 {
		return nil, err
	}
	world.Served = map[ids.ID][]domain.ServedTarget{agent: served}
	var out []ids.ID
	for _, p := range domain.Triggerers(world, agent, class, x.t) {
		ps, err := x.state(ctx, p)
		if err != nil {
			return nil, err
		}
		if slices.ContainsFunc(ks, func(k domain.Path) bool {
			return domain.Effective(ps.snap, domain.KindUser, p, k).Level < domain.LevelRead
		}) {
			out = append(out, p)
		}
	}
	return out, nil
}

// agentReach returns the channels of relevant in H(agent), the agent's effective tool class and
// its served targets; no channels for an agent that is unknown, not active or without a policy.
func (x *explainer) agentReach(ctx context.Context, agent ids.ID, relevant []domain.Path) ([]domain.Path, string, []domain.ServedTarget, error) {
	st, err := x.state(ctx, agent)
	if err != nil || !st.found || string(st.facts.Kind) != domain.KindAgent || st.facts.Status != authz.PrincipalActive {
		return nil, "", nil, err
	}
	class, ok, err := x.toolClass(ctx, agent)
	if err != nil || !ok {
		return nil, "", nil, err
	}
	served, err := servedOf(ctx, x.a.structure, agent)
	if err != nil {
		return nil, "", nil, err
	}
	horizon := domain.Horizon(st.snap, agent, x.channels, served)
	var ks []domain.Path
	for _, p := range relevant {
		if slices.Contains(horizon, p.ChannelID) {
			ks = append(ks, p)
		}
	}
	return ks, class, served, nil
}

// servedOf returns the targets agent serves.
func servedOf(ctx context.Context, structure authz.StructureSource, agent ids.ID) ([]domain.ServedTarget, error) {
	list, err := structure.ServedTargets(ctx, agent)
	if err != nil {
		return nil, sourceError("structure source", err)
	}
	out := make([]domain.ServedTarget, 0, len(list))
	for _, s := range list {
		out = append(out, domain.ServedTarget{ChannelID: s.ChannelID, SpaceID: s.SpaceID})
	}
	return out, nil
}

// output drops the entries without level and without via_agent, adds the names and sorts.
func (x *explainer) output(ctx context.Context, entries map[ids.ID]*domain.ExplainEntry) ([]authz.AccessEntry, error) {
	var keep []domain.ExplainEntry
	var list []ids.ID
	for id, e := range entries {
		if e.Rights.Level == domain.LevelNone && !domain.HasSource(e.Sources, domain.SourceViaAgent) {
			continue
		}
		keep = append(keep, *e)
		list = append(list, id)
	}
	profiles, err := x.a.principal.Profiles(ctx, list)
	if err != nil {
		return nil, sourceError("principal source", err)
	}
	for i := range keep {
		prof, ok := profiles[keep[i].PrincipalID]
		if !ok {
			return nil, internal("an access entry without a principal")
		}
		keep[i].Name = prof.DisplayName
	}
	domain.SortEntries(keep)
	out := make([]authz.AccessEntry, 0, len(keep))
	for _, e := range keep {
		out = append(out, authz.AccessEntry{
			Principal: profiles[e.PrincipalID].PrincipalRef, Level: levelText(e.Rights.Level), Inactive: e.Inactive,
			Capabilities: e.Rights.Caps.Names(), Sources: sourcesOut(e.Sources),
		})
	}
	return out, nil
}

// levelText is the OpenAPI LevelOrNone of l.
func levelText(l domain.Level) string {
	if l == domain.LevelNone {
		return authz.LevelNone
	}
	return l.String()
}

// sourcesOut converts the sources for the port.
func sourcesOut(list []domain.Source) []authz.AccessSource {
	out := make([]authz.AccessSource, 0, len(list))
	for _, s := range list {
		out = append(out, authz.AccessSource{
			Kind: string(s.Kind), GrantID: s.GrantID, GroupID: s.GroupID, FromTargetKind: string(s.FromTargetKind),
			FromTargetID: s.FromTargetID, AgentID: s.AgentID, Rule: s.Rule, ExpiresAt: s.ExpiresAt,
			MembershipExpiresAt: s.MembershipExpiresAt, Reason: s.Reason,
		})
	}
	return out
}

// engine returns the Authorizer the access use cases compute with (M1-49): AdminDeps.Engine, or
// the Authorizer itself when it is one. Without it the use cases fail with kind internal.
func (s *AdminService) engine() (*Authorizer, error) {
	if s.d.Engine != nil {
		return s.d.Engine, nil
	}
	if a, ok := s.d.Authorizer.(*Authorizer); ok {
		return a, nil
	}
	return nil, errs.New(errs.CodeInternalError, "authz: the access use cases need the Authorizer of authz")
}
