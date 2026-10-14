// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// TestProvenanceKinds: direct, group, inherited and operator sources of one path, expired grants
// and grants elsewhere left out, dm participation alone in a dm.
func TestProvenanceKinds(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)
	p, space, ch, other, group := ids.New(), ids.New(), ids.New(), ids.New(), ids.New()
	g := func(tg domain.Target, sub domain.Subject, exp time.Time) domain.Grant {
		return domain.Grant{ID: ids.New(), Subject: sub, Target: tg, Level: domain.LevelRead, ExpiresAt: exp, Reason: "r"}
	}
	user := domain.Subject{Kind: domain.SubjectUser, PrincipalID: p}
	direct := g(domain.Target{Kind: domain.TargetChannel, ChannelID: ch}, user, time.Time{})
	inst := g(domain.Target{Kind: domain.TargetInstance}, user, time.Time{})
	expired := g(domain.Target{Kind: domain.TargetSpace, SpaceID: space}, user, now)
	elsewhere := g(domain.Target{Kind: domain.TargetChannel, ChannelID: other}, user, time.Time{})
	grp := g(domain.Target{Kind: domain.TargetSpace, SpaceID: space}, domain.Subject{Kind: domain.SubjectGroup, GroupID: group}, time.Time{})
	mexp := now.Add(time.Hour)
	in := domain.SnapshotInput{PrincipalID: p, At: now, Active: true,
		Grants:      []domain.Grant{direct, inst, expired, elsewhere},
		Memberships: []domain.MembershipWithGroup{{Membership: domain.Membership{ID: ids.New(), GroupID: group, UserID: p, ExpiresAt: mexp}, Group: domain.Group{ID: group}}},
		GroupGrants: []domain.Grant{grp}}
	path := domain.Path{Kind: domain.PathChannel, SpaceID: space, ChannelID: ch}
	src := domain.Provenance(in, domain.KindUser, path)
	if len(src) != 3 {
		t.Fatalf("sources %+v", src)
	}
	byGrant := map[ids.ID]domain.Source{}
	for _, s := range src {
		byGrant[s.GrantID] = s
	}
	if s := byGrant[direct.ID]; s.Kind != domain.SourceDirect || s.Reason != "r" {
		t.Errorf("direct %+v", s)
	}
	if s := byGrant[inst.ID]; s.Kind != domain.SourceInherited || s.FromTargetKind != domain.TargetInstance || !s.FromTargetID.IsZero() {
		t.Errorf("instance %+v", s)
	}
	if s := byGrant[grp.ID]; s.Kind != domain.SourceInherited || s.GroupID != group || !s.MembershipExpiresAt.Equal(mexp) || s.FromTargetID != space {
		t.Errorf("inherited group grant %+v", s)
	}
	spaceSrc := domain.Provenance(in, domain.KindUser, domain.Path{Kind: domain.PathSpace, SpaceID: space})
	if len(spaceSrc) != 2 || !domain.HasSource(spaceSrc, domain.SourceGroup) || !domain.HasSource(spaceSrc, domain.SourceInherited) {
		t.Errorf("space sources %+v", spaceSrc)
	}
	agent := domain.Provenance(domain.SnapshotInput{PrincipalID: p, At: now, Active: true}, domain.KindAgent,
		domain.Path{Kind: domain.PathChannel, SpaceID: space, ChannelID: ch, ChannelOperatorID: p})
	if len(agent) != 1 || agent[0].Kind != domain.SourceOperator || agent[0].AgentID != p {
		t.Errorf("operator %+v", agent)
	}
	dm := domain.Provenance(in, domain.KindUser, domain.Path{Kind: domain.PathChannel, ChannelID: ids.New(), DM: true, DMParticipants: []ids.ID{p}})
	if len(dm) != 1 || dm[0].Kind != domain.SourceDMParticipant {
		t.Errorf("dm %+v", dm)
	}
	in.Owner = true
	if own := domain.Provenance(in, domain.KindUser, path); own[0].Kind != domain.SourceOwner {
		t.Errorf("owner first %+v", own)
	}
}

// TestEffects: a channel gained, a channel changed with new capabilities, a space gained.
func TestEffects(t *testing.T) {
	t.Parallel()
	a, b, s := ids.New(), ids.New(), ids.New()
	before := domain.AccessView{Spaces: map[ids.ID]bool{}, Channels: map[ids.ID]domain.Rights{b: {Level: domain.LevelRead, Caps: domain.CapsOf(domain.CapExport)}}}
	after := domain.AccessView{Spaces: map[ids.ID]bool{s: true}, Channels: map[ids.ID]domain.Rights{
		a: {Level: domain.LevelTrigger}, b: {Level: domain.LevelWrite, Caps: domain.CapsOf(domain.CapExport, domain.CapUpload)}}}
	got := map[ids.ID]domain.Effect{}
	for _, e := range domain.Effects(before, after) {
		got[e.ID] = e
	}
	if e := got[a]; e.Before != domain.LevelNone || e.After != domain.LevelTrigger || e.CapsAdded != 0 {
		t.Errorf("added %+v", e)
	}
	if e := got[b]; e.Before != domain.LevelRead || e.After != domain.LevelWrite || e.CapsAdded != domain.CapsOf(domain.CapUpload) {
		t.Errorf("changed %+v", e)
	}
	if e := got[s]; e.Kind != domain.AccessKindSpace || e.Before != domain.LevelNone || e.After != domain.LevelRead {
		t.Errorf("space %+v", e)
	}
}

// TestHorizonAndTriggerers: H(A) holds the channels the agent reads by grant or operator rule and
// the served channels; T(A) loses the people SR2 blocks for a host agent, owners stay.
func TestHorizonAndTriggerers(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)
	agent, owner, sam, space, k1, k2, k3 := ids.New(), ids.New(), ids.New(), ids.New(), ids.New(), ids.New(), ids.New()
	snap := domain.BuildSnapshot(domain.SnapshotInput{PrincipalID: agent, At: now, Active: true, Grants: []domain.Grant{
		{ID: ids.New(), Subject: domain.Subject{Kind: domain.SubjectAgent, PrincipalID: agent}, Target: domain.Target{Kind: domain.TargetChannel, ChannelID: k2}, Level: domain.LevelRead}}})
	paths := []domain.Path{
		{Kind: domain.PathChannel, SpaceID: space, ChannelID: k1, ChannelOperatorID: agent},
		{Kind: domain.PathChannel, SpaceID: space, ChannelID: k2},
		{Kind: domain.PathChannel, SpaceID: space, ChannelID: k3},
	}
	h := domain.Horizon(snap, agent, paths, []domain.ServedTarget{{SpaceID: space, ChannelID: k3}})
	if want := domain.SortIDs([]ids.ID{k1, k2, k3}); len(h) != 3 || h[0] != want[0] || h[2] != want[2] {
		t.Fatalf("H(A) = %v", h)
	}
	w := domain.World{Principals: map[ids.ID]domain.PrincipalState{sam: {Kind: domain.KindUser, Active: true}}, Owners: []ids.ID{owner},
		Grants: []domain.Grant{{ID: ids.New(), Subject: domain.Subject{Kind: domain.SubjectUser, PrincipalID: sam}, Target: domain.Target{Kind: domain.TargetChannel, ChannelID: k1}, Level: domain.LevelTrigger}},
		Served: map[ids.ID][]domain.ServedTarget{agent: {{SpaceID: space, ChannelID: k1}}}}
	if got := domain.Triggerers(w, agent, domain.ToolScoped, now); len(got) != 2 {
		t.Fatalf("T(A) scoped = %v", got)
	}
	if got := domain.Triggerers(w, agent, domain.ToolHost, now); len(got) != 1 || got[0] != owner {
		t.Fatalf("T(A) host = %v", got)
	}
	if domain.EffectiveToolClass(domain.ToolChat, domain.ToolHost) != domain.ToolHost || domain.EffectiveToolClass("", "") != domain.ToolChat ||
		domain.EffectiveToolClass("odd", domain.ToolChat) != domain.ToolHost {
		t.Fatal("EffectiveToolClass")
	}
}
