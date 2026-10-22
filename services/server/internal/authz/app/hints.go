// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"slices"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// The list "Permission hints" (07 section 9.5, brief M6-07). Every hint is computed per request
// with the code "who has access" uses (explainer: readGrants, BuildSnapshot, Effective), so the
// hints follow the same permission state as authorize and never a second rule set. A page is
// computed lazily in the fixed order of kinds (domain.HintKinds) and, within a kind, by its key:
// H1 agent by agent, so a page of H1 computes only the agents it needs.

// cursorHints is the list name of the cursor of the hints.
const cursorHints = "authz.hints"

// hintCursor is the position of a page of hints: the kind and the key of its last hint.
type hintCursor struct {
	Kind string `json:"k"`
	A    string `json:"a"`
	B    string `json:"b,omitempty"`
	C    string `json:"c,omitempty"`
}

// hintPos is a decoded hintCursor; a zero rank with started false is the start of the list.
type hintPos struct {
	started bool
	rank    int
	key     domain.HintKey
}

// ListPermissionHints implements authz.Admin (access.hints.read, M6-07).
func (s *AdminService) ListPermissionHints(ctx context.Context, caller actor.Actor, q authz.HintsQuery) (authz.HintPage, error) {
	ctx, err := withCaller(ctx, caller)
	if err != nil {
		return authz.HintPage{}, err
	}
	eng, err := s.engine()
	if err != nil {
		return authz.HintPage{}, err
	}
	if s.d.Agents == nil {
		return authz.HintPage{}, internal("the permission hints need the agent directory")
	}
	kinds := domain.HintKinds
	if q.Kind != "" {
		if domain.KindRank(domain.HintKind(q.Kind)) < 0 {
			return authz.HintPage{}, fieldErr("/kind", "an unknown kind of permission hint")
		}
		kinds = []domain.HintKind{domain.HintKind(q.Kind)}
	}
	if err := s.checkHints(ctx, caller, q.SpaceID); err != nil {
		return authz.HintPage{}, err
	}
	bind := binding(caller.ID.String(), q.SpaceID.String(), q.Kind)
	pos, err := s.hintPosition(q.PageQuery, bind)
	if err != nil {
		return authz.HintPage{}, err
	}
	h, err := s.newHinter(ctx, eng, q.SpaceID)
	if err != nil {
		return authz.HintPage{}, err
	}
	limit := s.pageLimit(q.PageQuery)
	var items []domain.Hint
	for _, k := range kinds {
		rank := domain.KindRank(k)
		if pos.started && rank < pos.rank {
			continue
		}
		var after *domain.HintKey
		if pos.started && rank == pos.rank {
			after = &pos.key
		}
		more, err := h.list(ctx, k, after, limit+1-len(items))
		if err != nil {
			return authz.HintPage{}, err
		}
		if items = append(items, more...); len(items) > limit {
			break
		}
	}
	full := len(items) > limit
	items = items[:min(len(items), limit)]
	out := authz.HintPage{Items: make([]authz.PermissionHint, 0, len(items))}
	for _, x := range items {
		out.Items = append(out.Items, hintOut(x))
	}
	if out.Labels, err = h.labels(ctx, hintRefs(items)); err != nil {
		return authz.HintPage{}, err
	}
	if full {
		out.NextCursor, err = s.hintNext(bind, items[len(items)-1])
	}
	return out, err
}

// checkHints authorizes access.hints.read on the instance (zero space) or on the space: no read
// on the space is 404, read without manage 403, the instance without the owner role 403. A
// space that does not exist is 404 for owners too.
func (s *AdminService) checkHints(ctx context.Context, caller actor.Actor, space ids.ID) error {
	res := authorize.Resource{Kind: authorize.ResourceInstance}
	if !space.IsZero() {
		res = authorize.Resource{Kind: authorize.ResourceSpace, ID: space}
	}
	if err := s.check(ctx, caller, authorize.ActionAccessHintsRead, res); err != nil {
		return err
	}
	if space.IsZero() {
		return nil
	}
	ok, err := s.d.Structure.Space(ctx, space)
	if err != nil {
		return sourceError("structure source", err)
	}
	if !ok {
		return errNotFound()
	}
	return nil
}

// hintPosition decodes the cursor of q for binding.
func (s *AdminService) hintPosition(q authz.PageQuery, binding []byte) (hintPos, error) {
	if q.Cursor == "" {
		return hintPos{}, nil
	}
	var c hintCursor
	if err := s.d.Cursors.Decode(cursorHints, binding, q.Cursor, &c); err != nil {
		return hintPos{}, errs.New(errs.CodeCursorInvalid, "")
	}
	rank := domain.KindRank(domain.HintKind(c.Kind))
	if rank < 0 {
		return hintPos{}, errs.New(errs.CodeCursorInvalid, "")
	}
	var key domain.HintKey
	for _, p := range []struct {
		dst *ids.ID
		v   string
	}{{&key.A, c.A}, {&key.B, c.B}, {&key.C, c.C}} {
		if p.v == "" {
			continue
		}
		id, err := ids.Parse(p.v)
		if err != nil {
			return hintPos{}, errs.New(errs.CodeCursorInvalid, "")
		}
		*p.dst = id
	}
	return hintPos{started: true, rank: rank, key: key}, nil
}

// hintNext is the cursor after the hint last.
func (s *AdminService) hintNext(binding []byte, last domain.Hint) (string, error) {
	k := last.Key()
	c := hintCursor{Kind: string(last.Kind), A: k.A.String()}
	if !k.B.IsZero() {
		c.B = k.B.String()
	}
	if !k.C.IsZero() {
		c.C = k.C.String()
	}
	out, err := s.d.Cursors.Encode(cursorHints, binding, c)
	if err != nil {
		return "", errs.Wrap(errs.CodeInternalError, err, "authz: hint cursor")
	}
	return out, nil
}

// hinter computes the hints of one request at one time over the instance or one space.
type hinter struct {
	s *AdminService
	x *explainer
	// space is the space of the list; zero for the instance.
	space ids.ID
	// facts are the channels that are not dm channels, by id; relevant their paths in scope.
	facts    map[ids.ID]authz.ChannelFacts
	relevant []domain.Path
	world    domain.World
	owners   map[ids.ID]bool
	// grants are the grants in effect on targets in scope, ascending by id.
	grants []domain.Grant
	// served caches the served targets per agent.
	served map[ids.ID][]domain.ServedTarget
}

// newHinter reads the structure, the active owners, the trigger world and the grants in scope.
func (s *AdminService) newHinter(ctx context.Context, eng *Authorizer, space ids.ID) (*hinter, error) {
	x := &explainer{a: eng, t: s.d.Clock.Now(), states: map[ids.ID]*principalState{}}
	h := &hinter{s: s, x: x, space: space, facts: map[ids.ID]authz.ChannelFacts{}, owners: map[ids.ID]bool{},
		served: map[ids.ID][]domain.ServedTarget{}}
	chans, err := eng.structure.Channels(ctx)
	if err != nil {
		return nil, sourceError("structure source", err)
	}
	var chanIDs []ids.ID
	for _, c := range chans {
		p := channelPath(c)
		x.channels = append(x.channels, p)
		if p.DM {
			continue
		}
		h.facts[c.ID] = c
		if space.IsZero() || c.SpaceID == space {
			h.relevant = append(h.relevant, p)
			chanIDs = append(chanIDs, c.ID)
		}
	}
	owners, err := eng.principal.ActiveOwnerIDs(ctx)
	if err != nil {
		return nil, sourceError("principal source", err)
	}
	for _, o := range owners {
		h.owners[o] = true
	}
	if h.world, err = triggerWorld(ctx, eng.store, eng.principal, owners); err != nil {
		return nil, err
	}
	if err := h.loadGrants(ctx, chanIDs); err != nil {
		return nil, err
	}
	return h, nil
}

// loadGrants reads the grants in effect at x.t on the targets in scope: for a space the space
// and its channels; for the instance also the instance and the hosts.
func (h *hinter) loadGrants(ctx context.Context, chanIDs []ids.ID) error {
	spaces := []ids.ID{h.space}
	if h.space.IsZero() {
		var err error
		if spaces, err = h.x.a.structure.Spaces(ctx); err != nil {
			return sourceError("structure source", err)
		}
	}
	grants, err := h.x.a.store.ActiveGrantsOnTargets(ctx, spaces, chanIDs)
	if err != nil {
		return sourceError("grant store", err)
	}
	if h.space.IsZero() && h.s.d.Hosts != nil {
		hosts, err := h.s.d.Hosts.HostIDs(ctx)
		if err != nil {
			return sourceError("host source", err)
		}
		more, err := h.x.a.store.ActiveGrantsOnHosts(ctx, hosts)
		if err != nil {
			return sourceError("grant store", err)
		}
		grants = append(grants, more...)
	}
	for _, g := range grants {
		if g.InEffect(h.x.t) && (h.space.IsZero() || g.Target.Kind != domain.TargetInstance) {
			h.grants = append(h.grants, g)
		}
	}
	slices.SortFunc(h.grants, func(a, b domain.Grant) int { return slices.Compare(a.ID[:], b.ID[:]) })
	return nil
}

// list returns at most need hints of kind k with a key after *after (nil: from the start), in key
// order.
func (h *hinter) list(ctx context.Context, k domain.HintKind, after *domain.HintKey, need int) ([]domain.Hint, error) {
	if need <= 0 {
		return nil, nil
	}
	var (
		out []domain.Hint
		err error
	)
	switch k {
	case domain.HintH1:
		return h.horizon(ctx, after, need)
	case domain.HintGrantToDisabled:
		out, err = h.disabled(ctx)
	case domain.HintTriggerBlocked:
		out, err = h.blocked(ctx)
	case domain.HintAgentPersonal:
		out, err = h.personal(ctx)
	}
	if err != nil {
		return nil, err
	}
	return page(out, after, need), nil
}

// page returns at most need hints of sorted list with a key after *after.
func page(list []domain.Hint, after *domain.HintKey, need int) []domain.Hint {
	domain.SortHints(list)
	var out []domain.Hint
	for _, x := range list {
		if after != nil && domain.CompareKeys(x.Key(), *after) <= 0 {
			continue
		}
		if out = append(out, x); len(out) == need {
			break
		}
	}
	return out
}

// horizon returns the H1 hints agent by agent (ascending), each over the channels in scope, and
// stops when need hints are found. Every agent counts, also one that serves the space only
// through a thread or reads a channel of the space from another space (D-M6-07-3).
func (h *hinter) horizon(ctx context.Context, after *domain.HintKey, need int) ([]domain.Hint, error) {
	if len(h.relevant) == 0 {
		return nil, nil
	}
	agents, err := h.s.d.Agents.AgentIDs(ctx)
	if err != nil {
		return nil, sourceError("agent directory", err)
	}
	var out []domain.Hint
	for _, agent := range domain.SortIDs(slices.Clone(agents)) {
		if after != nil && slices.Compare(agent[:], after.A[:]) < 0 {
			continue
		}
		gaps, err := h.x.readHorizon(ctx, h.world, agent, h.relevant)
		if err != nil {
			return nil, err
		}
		if out = append(out, page(gaps, after, need-len(out))...); len(out) >= need {
			return out, nil
		}
	}
	return out, nil
}

// subjectFacts returns the facts of a grant's subject principal; found is false for none.
func (h *hinter) subjectFacts(ctx context.Context, id ids.ID) (authz.PrincipalFacts, bool, error) {
	st, err := h.x.state(ctx, id)
	if err != nil {
		return authz.PrincipalFacts{}, false, err
	}
	return st.facts, st.found, nil
}

// disabled returns the hints grant_to_disabled: grants in scope whose subject principal is
// disabled or deleted (a pending person's grants take effect, 07 SR7).
func (h *hinter) disabled(ctx context.Context) ([]domain.Hint, error) {
	var out []domain.Hint
	for _, g := range h.grants {
		if g.Subject.Kind == domain.SubjectGroup {
			continue
		}
		f, found, err := h.subjectFacts(ctx, g.Subject.PrincipalID)
		if err != nil {
			return nil, err
		}
		st := domain.PrincipalState{Kind: string(f.Kind), Active: f.Status == authz.PrincipalActive || f.Status == authz.PrincipalPending}
		if domain.GrantToDisabled(g, st, found) {
			out = append(out, domain.Hint{Kind: domain.HintGrantToDisabled, Grant: g})
		}
	}
	return out, nil
}

// blocked returns the hints trigger_blocked: grants of level trigger in scope to an active
// person or a group where SR2 or SR3 blocks triggering on every operated channel the grant
// reaches (domain.TriggerBlockRules). A disabled subject is listed as grant_to_disabled instead.
func (h *hinter) blocked(ctx context.Context) ([]domain.Hint, error) {
	var out []domain.Hint
	for _, g := range h.grants {
		if g.Level != domain.LevelTrigger || (g.Subject.Kind != domain.SubjectUser && g.Subject.Kind != domain.SubjectGroup) {
			continue
		}
		people, owner, err := h.triggerPeople(ctx, g)
		if err != nil {
			return nil, err
		}
		if people == nil {
			continue
		}
		reach, err := h.reach(ctx, g, people)
		if err != nil {
			return nil, err
		}
		if rules, ok := domain.TriggerBlockRules(reach, owner); ok {
			out = append(out, domain.Hint{Kind: domain.HintTriggerBlocked, Grant: g, Rules: rules})
		}
	}
	return out, nil
}

// triggerPeople returns the people whose dev environments count for g: the person subject, or
// the members in effect of a group subject (an empty, non-nil list for a group without members);
// nil for a person subject that is unknown or not active. owner reports an effective owner as
// the person subject.
func (h *hinter) triggerPeople(ctx context.Context, g domain.Grant) ([]ids.ID, bool, error) {
	if g.Subject.Kind == domain.SubjectUser {
		f, found, err := h.subjectFacts(ctx, g.Subject.PrincipalID)
		if err != nil || !found || f.Kind != actor.KindUser || f.Status != authz.PrincipalActive {
			return nil, false, err
		}
		return []ids.ID{g.Subject.PrincipalID}, h.owners[g.Subject.PrincipalID], nil
	}
	people := []ids.ID{}
	for _, m := range h.world.Memberships {
		if m.Membership.GroupID == g.Subject.GroupID && m.Group.ID == m.Membership.GroupID && m.Membership.InEffect(m.Group, h.x.t) {
			people = append(people, m.Membership.UserID)
		}
	}
	return people, false, nil
}

// reach returns the channels with an operator that g reaches, archived channels left out (no
// one triggers there), with the facts of SR2 and SR3; DevReady is true when one of people has a
// ready own dev environment for the channel.
func (h *hinter) reach(ctx context.Context, g domain.Grant, people []ids.ID) ([]domain.TriggerChannel, error) {
	var out []domain.TriggerChannel
	for _, p := range h.x.channels {
		f, ok := h.facts[p.ChannelID]
		if !ok || f.Archived || f.EffectiveOperatorID.IsZero() || !domain.Reaches(g.Target, p) {
			continue
		}
		class, found, err := h.x.toolClass(ctx, f.EffectiveOperatorID)
		if err != nil {
			return nil, err
		}
		if !found {
			continue
		}
		c := domain.TriggerChannel{ChannelID: f.ID, ToolClass: class, ProjectBound: f.ProjectBound}
		for _, person := range people {
			ready, err := h.x.a.policies.DevEnvironmentReady(ctx, person, f.ID)
			if err != nil {
				return nil, sourceError("agent policy source", err)
			}
			if ready {
				c.DevReady = true
				break
			}
		}
		out = append(out, c)
	}
	return out, nil
}

// servedOf returns the served targets of agent, read once per request.
func (h *hinter) servedOf(ctx context.Context, agent ids.ID) ([]domain.ServedTarget, error) {
	if s, ok := h.served[agent]; ok {
		return s, nil
	}
	s, err := servedOf(ctx, h.x.a.structure, agent)
	if err != nil {
		return nil, err
	}
	h.served[agent] = s
	return s, nil
}

// personal returns the hints agent_personal: the active agents in mode personal with their
// trigger set (I1, 07 SR1); for a space only agents that serve a target in it.
func (h *hinter) personal(ctx context.Context) ([]domain.Hint, error) {
	agents, err := h.x.a.policies.PersonalAgents(ctx)
	if err != nil {
		return nil, sourceError("agent policy source", err)
	}
	var out []domain.Hint
	for _, a := range agents {
		served, err := h.servedOf(ctx, a.AgentID)
		if err != nil {
			return nil, err
		}
		if !h.space.IsZero() && !slices.ContainsFunc(served, func(t domain.ServedTarget) bool { return t.SpaceID == h.space }) {
			continue
		}
		w := h.world
		w.Served = map[ids.ID][]domain.ServedTarget{a.AgentID: served}
		set := domain.TriggerSet(w, domain.PersonalAgent{AgentID: a.AgentID, CredentialHolderID: a.CredentialHolderID}, h.x.t)
		out = append(out, domain.Hint{Kind: domain.HintAgentPersonal, AgentID: a.AgentID, CredentialHolderID: a.CredentialHolderID,
			TriggerSet: set})
	}
	return out, nil
}

// labelRefs are the ids a page names, by kind of label.
type labelRefs struct {
	principals, groups, spaces, channels []ids.ID
}

// addGrant adds the subject and the target of g.
func (r *labelRefs) addGrant(g domain.Grant) {
	if g.Subject.Kind == domain.SubjectGroup {
		r.groups = append(r.groups, g.Subject.GroupID)
	} else {
		r.principals = append(r.principals, g.Subject.PrincipalID)
	}
	r.spaces = append(r.spaces, g.Target.SpaceID)
	r.channels = append(r.channels, g.Target.ChannelID)
}

// hintRefs collects the ids of items.
func hintRefs(items []domain.Hint) labelRefs {
	var r labelRefs
	for _, x := range items {
		r.principals = append(r.principals, x.AgentID, x.PersonID, x.CredentialHolderID)
		r.principals = append(r.principals, x.TriggerSet...)
		r.spaces = append(r.spaces, x.SpaceID)
		r.channels = append(r.channels, x.ChannelID)
		if !x.Grant.ID.IsZero() {
			r.addGrant(x.Grant)
		}
	}
	return r
}

// labels returns the names of r (authz.AccessLabels): the public views of the principals, the
// names of the groups, and the spaces and channels with the space of every channel. Unknown ids
// are left out.
func (h *hinter) labels(ctx context.Context, r labelRefs) (authz.AccessLabels, error) {
	return pageLabels(ctx, h.x.a, h.s.d.Store, r)
}

// pageLabels is labels over the sources of eng and store.
func pageLabels(ctx context.Context, eng *Authorizer, store Store, r labelRefs) (authz.AccessLabels, error) {
	out := authz.AccessLabels{Principals: []authz.PrincipalRef{}, Groups: []authz.AccessLabel{}, Spaces: []authz.AccessLabel{},
		Channels: []authz.ChannelLabel{}}
	profiles, err := eng.principal.Profiles(ctx, uniqueIDs(r.principals))
	if err != nil {
		return authz.AccessLabels{}, sourceError("principal source", err)
	}
	for _, id := range uniqueIDs(r.principals) {
		if p, ok := profiles[id]; ok {
			out.Principals = append(out.Principals, p.PrincipalRef)
		}
	}
	for _, id := range uniqueIDs(r.groups) {
		g, found, err := store.GetGroup(ctx, id)
		if err != nil {
			return authz.AccessLabels{}, sourceError("grant store", err)
		}
		if found {
			out.Groups = append(out.Groups, authz.AccessLabel{ID: g.ID, Name: g.Name})
		}
	}
	chans, err := eng.structure.ChannelDetails(ctx, uniqueIDs(r.channels))
	if err != nil {
		return authz.AccessLabels{}, sourceError("structure source", err)
	}
	spaces := slices.Clone(r.spaces)
	for _, id := range uniqueIDs(r.channels) {
		if c, ok := chans[id]; ok {
			out.Channels = append(out.Channels, authz.ChannelLabel{ID: c.ID, SpaceID: c.SpaceID, Name: c.Name})
			spaces = append(spaces, c.SpaceID)
		}
	}
	details, err := eng.structure.SpaceDetails(ctx, uniqueIDs(spaces))
	if err != nil {
		return authz.AccessLabels{}, sourceError("structure source", err)
	}
	for _, id := range uniqueIDs(spaces) {
		if d, ok := details[id]; ok {
			out.Spaces = append(out.Spaces, authz.AccessLabel{ID: d.ID, Name: d.Name})
		}
	}
	return out, nil
}

// hintOut converts a hint for the port.
func hintOut(x domain.Hint) authz.PermissionHint {
	out := authz.PermissionHint{Kind: string(x.Kind), AgentID: x.AgentID, PersonID: x.PersonID, SpaceID: x.SpaceID, ChannelID: x.ChannelID,
		Rules: x.Rules, CredentialHolderID: x.CredentialHolderID, TriggerSet: x.TriggerSet}
	if !x.Grant.ID.IsZero() {
		g := grantOut(x.Grant)
		out.Grant = &g
	}
	if x.Kind == domain.HintAgentPersonal && out.TriggerSet == nil {
		out.TriggerSet = []ids.ID{}
	}
	return out
}
