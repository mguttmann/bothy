// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

var (
	alexID  = ids.MustParse("019a0000-0000-7000-8000-00000000a1e0")
	samID   = ids.MustParse("019a0000-0000-7000-8000-0000000005a0")
	agentID = ids.MustParse("019a0000-0000-7000-8000-00000000a6e0")
	spaceS  = ids.MustParse("019a0000-0000-7000-8000-000000000501")
	chanC   = ids.MustParse("019a0000-0000-7000-8000-000000000c01")
	chanD   = ids.MustParse("019a0000-0000-7000-8000-000000000c02")
	groupG  = ids.MustParse("019a0000-0000-7000-8000-000000000901")
)

func grantOn(t *testing.T, subj domain.Subject, tg domain.Target, l domain.Level, c domain.Caps, exp time.Time) domain.Grant {
	t.Helper()
	return domain.Grant{ID: ids.New(), Subject: subj, Target: tg, Level: l, Caps: c, ExpiresAt: exp, GrantedAt: t0}
}

func TestBuildSnapshot(t *testing.T) {
	t.Parallel()
	exp := t0.Add(time.Hour)
	memExp := t0.Add(30 * time.Minute)
	own := []domain.Grant{
		grantOn(t, domain.Subject{Kind: domain.SubjectUser, PrincipalID: samID}, domain.Target{Kind: domain.TargetSpace, SpaceID: spaceS}, domain.LevelRead, 0, exp),
		grantOn(t, domain.Subject{Kind: domain.SubjectUser, PrincipalID: samID}, domain.Target{Kind: domain.TargetInstance}, domain.LevelRead, domain.CapsOf(domain.CapExport), time.Time{}),
		// A host grant never enters the tree (07 section 2.12, AZ-M60).
		{ID: ids.New(), Subject: domain.Subject{Kind: domain.SubjectUser, PrincipalID: samID}, Target: domain.Target{Kind: domain.TargetHost, HostID: ids.New()}, Caps: domain.CapsOf(domain.CapTerminal)},
		// Expired and revoked grants do not count.
		grantOn(t, domain.Subject{Kind: domain.SubjectUser, PrincipalID: samID}, domain.Target{Kind: domain.TargetChannel, ChannelID: chanD}, domain.LevelTrigger, 0, t0),
	}
	g := domain.Group{ID: groupG, Name: "Leser"}
	mems := []domain.MembershipWithGroup{{Membership: domain.Membership{ID: ids.New(), GroupID: groupG, UserID: samID, ExpiresAt: memExp}, Group: g}}
	groupGrants := []domain.Grant{grantOn(t, domain.Subject{Kind: domain.SubjectGroup, GroupID: groupG}, domain.Target{Kind: domain.TargetChannel, ChannelID: chanC}, domain.LevelWrite, domain.CapsOf(domain.CapUpload), time.Time{})}
	// An expired membership in a second group does not let that group's grants act.
	old := domain.Group{ID: ids.New(), Name: "Alt"}
	mems = append(mems, domain.MembershipWithGroup{Membership: domain.Membership{ID: ids.New(), GroupID: old.ID, UserID: samID, ExpiresAt: t0}, Group: old})
	groupGrants = append(groupGrants, grantOn(t, domain.Subject{Kind: domain.SubjectGroup, GroupID: old.ID}, domain.Target{Kind: domain.TargetChannel, ChannelID: chanD}, domain.LevelTrigger, 0, time.Time{}))
	s := domain.BuildSnapshot(domain.SnapshotInput{PrincipalID: samID, Version: 4, At: t0, Active: true, Grants: own, Memberships: mems, GroupGrants: groupGrants})
	if s.Instance.Level != domain.LevelRead || s.Instance.Caps != domain.CapsOf(domain.CapExport) {
		t.Fatalf("instance %+v", s.Instance)
	}
	if s.Spaces[spaceS].Level != domain.LevelRead || s.Channels[chanC].Level != domain.LevelWrite {
		t.Fatalf("targets %+v %+v", s.Spaces, s.Channels)
	}
	if _, ok := s.Channels[chanD]; ok {
		t.Fatal("expired grant counted")
	}
	if !s.ValidUntil.Equal(memExp) || s.PermissionVersion != 4 || len(s.Groups) != 1 {
		t.Fatalf("valid until %v version %d groups %v", s.ValidUntil, s.PermissionVersion, s.Groups)
	}
	// An inactive principal has no rights at all (SR7), an owner flag only through Owner.
	if in := domain.BuildSnapshot(domain.SnapshotInput{PrincipalID: samID, At: t0, Grants: own}); in.Instance.Level != domain.LevelNone || len(in.Spaces) != 0 {
		t.Fatalf("inactive snapshot %+v", in)
	}
}

func TestEffectiveRules(t *testing.T) {
	t.Parallel()
	s := domain.Snapshot{
		Instance: domain.Rights{},
		Spaces:   map[ids.ID]domain.Rights{spaceS: {Level: domain.LevelRead}},
		Channels: map[ids.ID]domain.Rights{chanC: {Level: domain.LevelTrigger, Caps: domain.CapsOf(domain.CapManage, domain.CapUpload)}},
	}
	path := domain.Path{Kind: domain.PathChannel, SpaceID: spaceS, ChannelID: chanC}
	if r := domain.Effective(s, domain.KindUser, samID, path); r.Level != domain.LevelTrigger || !r.Caps.Has(domain.CapManage) {
		t.Fatalf("user %+v", r)
	}
	// Agents and apps are capped at write and upload (07 section 2.1).
	if r := domain.Effective(s, domain.KindAgent, agentID, path); r.Level != domain.LevelWrite || r.Caps != domain.CapsOf(domain.CapUpload) {
		t.Fatalf("agent %+v", r)
	}
	// Operator rule on the channel and on a fixed-operator thread (07 section 2.9).
	other := domain.Path{Kind: domain.PathChannel, SpaceID: ids.New(), ChannelID: ids.New(), ChannelOperatorID: agentID}
	if r := domain.Effective(domain.Snapshot{}, domain.KindAgent, agentID, other); r.Level != domain.LevelWrite {
		t.Fatalf("channel operator %+v", r)
	}
	thread := domain.Path{Kind: domain.PathChannel, SpaceID: ids.New(), ChannelID: ids.New(), ChannelOperatorID: ids.New(),
		ThreadID: ids.New(), ThreadOperatorID: agentID}
	if r := domain.Effective(domain.Snapshot{}, domain.KindAgent, agentID, thread); r.Level != domain.LevelWrite {
		t.Fatalf("thread operator %+v", r)
	}
	thread.ThreadMain = true
	if r := domain.Effective(domain.Snapshot{}, domain.KindAgent, agentID, thread); r.Level != domain.LevelNone {
		t.Fatalf("main thread follows the channel's operator, got %+v", r)
	}
	// Owners have everything except DMs (SR6); participants have write and upload.
	owner := domain.Snapshot{Owner: true}
	if r := domain.Effective(owner, domain.KindUser, alexID, path); r.Level != domain.LevelTrigger || !r.Owner {
		t.Fatalf("owner %+v", r)
	}
	dm := domain.Path{Kind: domain.PathChannel, ChannelID: chanD, DM: true, DMParticipants: []ids.ID{alexID, samID}}
	if r := domain.Effective(owner, domain.KindUser, ids.New(), dm); r.Level != domain.LevelNone || r.Owner {
		t.Fatalf("owner in a foreign dm %+v", r)
	}
	if r := domain.Effective(domain.Snapshot{}, domain.KindUser, samID, dm); r.Level != domain.LevelWrite || r.Caps != domain.CapsOf(domain.CapUpload) {
		t.Fatalf("participant %+v", r)
	}
	// A host has no path: only the owner flag survives.
	if r := domain.Effective(owner, domain.KindUser, alexID, domain.Path{Kind: domain.PathNone}); r.Level != domain.LevelNone || !r.Owner {
		t.Fatalf("host path %+v", r)
	}
}

func TestRightsIntersect(t *testing.T) {
	t.Parallel()
	a := domain.Rights{Level: domain.LevelTrigger, Caps: domain.CapsOf(domain.CapManage, domain.CapUpload), Owner: true}
	b := domain.Rights{Level: domain.LevelRead, Caps: domain.CapsOf(domain.CapUpload)}
	got := a.Intersect(b)
	if got.Level != domain.LevelRead || got.Caps != domain.CapsOf(domain.CapUpload) || got.Owner {
		t.Fatalf("intersect %+v", got)
	}
}

func TestCheckLiveness(t *testing.T) {
	t.Parallel()
	ok := domain.Liveness{Status: "active", HasSession: true, SessionActive: true, SessionExpiresAt: t0.Add(time.Hour), HasDevice: true, DeviceState: "active"}
	for _, c := range []struct {
		name string
		mod  func(*domain.Liveness)
		want domain.Reason
	}{
		{"ok", func(*domain.Liveness) {}, ""},
		{"disabled", func(l *domain.Liveness) { l.Status = "disabled" }, domain.ReasonPrincipalInactive},
		{"deleted", func(l *domain.Liveness) { l.Status = "deleted" }, domain.ReasonPrincipalInactive},
		{"unknown principal", func(l *domain.Liveness) { l.Status = "" }, domain.ReasonUnauthenticated},
		{"session revoked", func(l *domain.Liveness) { l.SessionActive = false }, domain.ReasonSessionRevoked},
		{"session expired", func(l *domain.Liveness) { l.SessionExpiresAt = t0 }, domain.ReasonSessionRevoked},
		{"device revoked", func(l *domain.Liveness) { l.DeviceState = "revoked" }, domain.ReasonDeviceInactive},
		{"device unknown", func(l *domain.Liveness) { l.DeviceState = "unknown" }, domain.ReasonDeviceInactive},
		{"device key revoked", func(l *domain.Liveness) { l.DeviceKeyRevoked = true }, domain.ReasonDeviceInactive},
		{"key revoked on a pending device", func(l *domain.Liveness) { l.DeviceState, l.DeviceKeyRevoked = "pending", true }, domain.ReasonDeviceInactive},
		{"token revoked", func(l *domain.Liveness) { l.HasToken, l.TokenActive = true, false }, domain.ReasonTokenRevoked},
		{"token expired", func(l *domain.Liveness) { l.HasToken, l.TokenActive, l.TokenExpiresAt = true, true, t0 }, domain.ReasonTokenRevoked},
		{"app without installation", func(l *domain.Liveness) { l.IsApp = true }, domain.ReasonAppNotInstalled},
	} {
		l := ok
		c.mod(&l)
		d := domain.CheckLiveness(l, t0)
		if d.Reason != c.want || (c.want == "") != (d.Outcome == domain.OutcomeAllow) {
			t.Errorf("%s: %+v", c.name, d)
		}
		if c.want != "" && d.Outcome != domain.OutcomeUnauthenticated {
			t.Errorf("%s: outcome %s", c.name, d.Outcome)
		}
	}
}

func TestCheckPending(t *testing.T) {
	t.Parallel()
	pending := domain.Liveness{Status: "active", HasDevice: true, DeviceState: "pending"}
	if d := domain.CheckPending(pending, "message.read"); d.Reason != domain.ReasonDevicePending || d.Outcome != domain.OutcomeForbidden {
		t.Fatalf("message.read %+v", d)
	}
	for _, a := range []string{"self.read", "device.list", "auth.token.refresh", "auth.logout"} {
		if d := domain.CheckPending(pending, a); d.Outcome != domain.OutcomeAllow {
			t.Errorf("%s: %+v", a, d)
		}
	}
	prov := domain.Liveness{Status: "pending"}
	if d := domain.CheckPending(prov, "message.read"); d.Reason != domain.ReasonPrincipalPending {
		t.Fatalf("pending person %+v", d)
	}
}
