// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"slices"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// hintNow is the clock of the hint tests.
var hintNow = time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)

// readSnapshot is the snapshot of p with read grants on the channels chans.
func readSnapshot(p ids.ID, chans ...ids.ID) domain.Snapshot {
	in := domain.SnapshotInput{PrincipalID: p, At: hintNow, Active: true}
	for _, c := range chans {
		in.Grants = append(in.Grants, domain.Grant{ID: ids.New(), Subject: domain.Subject{Kind: domain.SubjectUser, PrincipalID: p},
			Target: domain.Target{Kind: domain.TargetChannel, ChannelID: c}, Level: domain.LevelRead, Reason: "r"})
	}
	return domain.BuildSnapshot(in)
}

// TestReadHorizonGaps: one H1 hint per person of T(A) and channel of H(A) the person cannot read,
// ordered by person and channel, a duplicate channel once, an owner never, a person without a
// snapshot everywhere.
func TestReadHorizonGaps(t *testing.T) {
	t.Parallel()
	agent, sam, robin, owner, space := ids.New(), ids.New(), ids.New(), ids.New(), ids.New()
	kampagnen, intern := ids.New(), ids.New()
	paths := []domain.Path{
		{Kind: domain.PathChannel, SpaceID: space, ChannelID: kampagnen},
		{Kind: domain.PathChannel, SpaceID: space, ChannelID: intern},
		{Kind: domain.PathChannel, SpaceID: space, ChannelID: intern},
	}
	readers := map[ids.ID]domain.Snapshot{
		sam:   readSnapshot(sam, kampagnen),
		owner: domain.BuildSnapshot(domain.SnapshotInput{PrincipalID: owner, At: hintNow, Active: true, Owner: true}),
	}
	got := domain.ReadHorizonGaps(agent, paths, []ids.ID{robin, sam, owner}, readers)
	want := []domain.HintKey{{A: agent, B: sam, C: intern}, {A: agent, B: robin, C: kampagnen}, {A: agent, B: robin, C: intern}}
	slices.SortFunc(want, domain.CompareKeys)
	if len(got) != len(want) {
		t.Fatalf("hints %+v", got)
	}
	for i, h := range got {
		if h.Kind != domain.HintH1 || h.Key() != want[i] || h.SpaceID != space {
			t.Errorf("hint %d %+v, want key %+v", i, h, want[i])
		}
	}
	readers[sam] = readSnapshot(sam, kampagnen, intern)
	if got := domain.ReadHorizonGaps(agent, paths, []ids.ID{sam}, readers); len(got) != 0 {
		t.Errorf("sam reads everything: %+v", got)
	}
}

// TestGrantToDisabled: a person, agent or app subject that is not active qualifies; an active or
// unknown subject and a group never.
func TestGrantToDisabled(t *testing.T) {
	t.Parallel()
	user := domain.Grant{Subject: domain.Subject{Kind: domain.SubjectUser, PrincipalID: ids.New()}}
	group := domain.Grant{Subject: domain.Subject{Kind: domain.SubjectGroup, GroupID: ids.New()}}
	for _, c := range []struct {
		g      domain.Grant
		st     domain.PrincipalState
		known  bool
		want   bool
		reason string
	}{
		{user, domain.PrincipalState{Kind: domain.KindUser}, true, true, "disabled person"},
		{user, domain.PrincipalState{Kind: domain.KindUser, Active: true}, true, false, "active person"},
		{user, domain.PrincipalState{}, false, false, "unknown subject"},
		{group, domain.PrincipalState{}, true, false, "group"},
	} {
		if got := domain.GrantToDisabled(c.g, c.st, c.known); got != c.want {
			t.Errorf("%s: %v", c.reason, got)
		}
	}
}

// TestTriggerBlockRules: blocked only when every reached operated channel is blocked by SR2 or
// SR3; a ready own dev environment, an owner or no reached channel never block.
func TestTriggerBlockRules(t *testing.T) {
	t.Parallel()
	host := domain.TriggerChannel{ChannelID: ids.New(), ToolClass: domain.ToolHost}
	chat := domain.TriggerChannel{ChannelID: ids.New(), ToolClass: domain.ToolChat}
	project := domain.TriggerChannel{ChannelID: ids.New(), ToolClass: domain.ToolScoped, ProjectBound: true}
	both := domain.TriggerChannel{ChannelID: ids.New(), ToolClass: "unknown", ProjectBound: true}
	ready := domain.TriggerChannel{ChannelID: ids.New(), ToolClass: domain.ToolHost, ProjectBound: true, DevReady: true}
	for _, c := range []struct {
		name    string
		reach   []domain.TriggerChannel
		owner   bool
		rules   []string
		blocked bool
	}{
		{"host only", []domain.TriggerChannel{host}, false, []string{domain.RuleSR2}, true},
		{"project only", []domain.TriggerChannel{project}, false, []string{domain.RuleSR3}, true},
		{"unknown class counts as host", []domain.TriggerChannel{both}, false, []string{domain.RuleSR2, domain.RuleSR3}, true},
		{"host and project", []domain.TriggerChannel{project, host}, false, []string{domain.RuleSR2, domain.RuleSR3}, true},
		{"one open channel", []domain.TriggerChannel{host, chat}, false, nil, false},
		{"ready environment", []domain.TriggerChannel{ready}, false, nil, false},
		{"owner", []domain.TriggerChannel{host}, true, nil, false},
		{"no operated channel", nil, false, nil, false},
	} {
		rules, blocked := domain.TriggerBlockRules(c.reach, c.owner)
		if blocked != c.blocked || !slices.Equal(rules, c.rules) {
			t.Errorf("%s: rules %v blocked %v", c.name, rules, blocked)
		}
	}
}

// TestReaches: the instance reaches every channel but no dm, a space its channels, a channel
// itself, a host nothing.
func TestReaches(t *testing.T) {
	t.Parallel()
	space, ch := ids.New(), ids.New()
	p := domain.Path{Kind: domain.PathChannel, SpaceID: space, ChannelID: ch}
	dm := domain.Path{Kind: domain.PathChannel, ChannelID: ch, DM: true}
	for _, c := range []struct {
		tg   domain.Target
		p    domain.Path
		want bool
	}{
		{domain.Target{Kind: domain.TargetInstance}, p, true},
		{domain.Target{Kind: domain.TargetInstance}, dm, false},
		{domain.Target{Kind: domain.TargetSpace, SpaceID: space}, p, true},
		{domain.Target{Kind: domain.TargetSpace, SpaceID: ids.New()}, p, false},
		{domain.Target{Kind: domain.TargetChannel, ChannelID: ch}, p, true},
		{domain.Target{Kind: domain.TargetChannel, ChannelID: ids.New()}, p, false},
		{domain.Target{Kind: domain.TargetHost, HostID: ids.New()}, p, false},
		{domain.Target{Kind: domain.TargetSpace, SpaceID: space}, domain.Path{Kind: domain.PathSpace, SpaceID: space}, false},
	} {
		if got := domain.Reaches(c.tg, c.p); got != c.want {
			t.Errorf("%+v on %+v: %v", c.tg, c.p, got)
		}
	}
}

// TestHintOrder: the kinds come in the fixed order and keys compare part by part.
func TestHintOrder(t *testing.T) {
	t.Parallel()
	for i, k := range domain.HintKinds {
		if domain.KindRank(k) != i {
			t.Errorf("%s at %d", k, domain.KindRank(k))
		}
	}
	if domain.KindRank("other") != -1 {
		t.Error("an unknown kind has a rank")
	}
	a, b := ids.New(), ids.New()
	if domain.CompareKeys(domain.HintKey{A: a, B: b}, domain.HintKey{A: a, B: a}) <= 0 || domain.CompareKeys(domain.HintKey{A: a}, domain.HintKey{A: b}) >= 0 {
		t.Error("keys compare by A, then B")
	}
	g := domain.Hint{Kind: domain.HintTriggerBlocked, Grant: domain.Grant{ID: a}}
	if g.Key() != (domain.HintKey{A: a}) {
		t.Errorf("grant key %+v", g.Key())
	}
	ag := domain.Hint{Kind: domain.HintAgentPersonal, AgentID: b}
	if ag.Key() != (domain.HintKey{A: b}) {
		t.Errorf("agent key %+v", ag.Key())
	}
	if (domain.Hint{Kind: "other"}).Key() != (domain.HintKey{}) {
		t.Error("an unknown kind has a key")
	}
}

// TestExpiringOrder: expiry first, then grant before membership before token, then id; the
// window is (t, until].
func TestExpiringOrder(t *testing.T) {
	t.Parallel()
	a, b := ids.New(), ids.New()
	early := domain.ExpiringPos{ExpiresAt: hintNow, Kind: domain.ExpiringToken, ID: b}
	late := domain.ExpiringPos{ExpiresAt: hintNow.Add(time.Second), Kind: domain.ExpiringGrant, ID: a}
	if domain.ComparePos(early, late) >= 0 {
		t.Error("expiry first")
	}
	g := domain.ExpiringPos{ExpiresAt: hintNow, Kind: domain.ExpiringGrant, ID: b}
	m := domain.ExpiringPos{ExpiresAt: hintNow, Kind: domain.ExpiringMembership, ID: a}
	if domain.ComparePos(g, m) >= 0 || domain.ComparePos(m, early) >= 0 {
		t.Error("grant, membership, token")
	}
	if domain.ComparePos(domain.ExpiringPos{ExpiresAt: hintNow, Kind: domain.ExpiringGrant, ID: a}, g) >= 0 {
		t.Error("then id")
	}
	until := hintNow.Add(time.Hour)
	for _, c := range []struct {
		exp  time.Time
		want bool
	}{{time.Time{}, false}, {hintNow, false}, {hintNow.Add(time.Nanosecond), true}, {until, true}, {until.Add(time.Nanosecond), false}} {
		if got := domain.ExpiresWithin(c.exp, hintNow, until); got != c.want {
			t.Errorf("%v: %v", c.exp, got)
		}
	}
}
