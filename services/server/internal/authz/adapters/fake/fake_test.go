// SPDX-License-Identifier: Apache-2.0

package fake_test

import (
	"bytes"
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/authz/adapters/fake"
	"github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

var at = time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)

func TestPrincipals(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := fake.NewPrincipals()
	owner, agent := ids.New(), ids.New()
	s.Put(authz.PrincipalFacts{ID: owner, Kind: actor.KindUser, Status: authz.PrincipalActive, IsOwner: true})
	s.Put(authz.PrincipalFacts{ID: agent, Kind: actor.KindAgent, Status: authz.PrincipalActive})
	if f, ok, err := s.Principal(ctx, owner); err != nil || !ok || !f.IsOwner {
		t.Fatalf("%+v %v %v", f, ok, err)
	}
	if got, _ := s.ActiveOwnerIDs(ctx); !slices.Equal(got, []ids.ID{owner}) {
		t.Fatalf("owners %v", got)
	}
	s.Fail(true)
	if _, _, err := s.Principal(ctx, owner); !errors.Is(err, fake.ErrInjected) {
		t.Fatal(err)
	}
	if _, err := s.ActiveOwnerIDs(ctx); !errors.Is(err, fake.ErrInjected) {
		t.Fatal(err)
	}
}

func TestStructure(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := fake.NewStructure()
	space, ch, dm, th, agent, alex := ids.New(), ids.New(), ids.New(), ids.New(), ids.New(), ids.New()
	s.PutSpace(space)
	s.PutChannel(authz.ChannelFacts{ID: ch, SpaceID: space, Kind: authz.ChannelText, EffectiveOperatorID: agent})
	s.PutChannel(authz.ChannelFacts{ID: dm, Kind: authz.ChannelDM, DMParticipants: []ids.ID{alex}})
	s.PutThread(authz.ThreadFacts{ID: th, ChannelID: ch, Main: true})
	if ok, _ := s.Space(ctx, space); !ok {
		t.Fatal("space")
	}
	if got, _ := s.Spaces(ctx); !slices.Equal(got, []ids.ID{space}) {
		t.Fatal("spaces")
	}
	if c, ok, _ := s.Channel(ctx, dm); !ok || c.Kind != authz.ChannelDM {
		t.Fatal("channel")
	}
	if tf, ok, _ := s.Thread(ctx, th); !ok || tf.ChannelID != ch {
		t.Fatal("thread")
	}
	if got, _ := s.Channels(ctx); len(got) != 1 || got[0].ID != ch {
		t.Fatalf("channels %v", got)
	}
	if got, _ := s.ChannelsOperatedBy(ctx, agent); !slices.Equal(got, []ids.ID{ch}) {
		t.Fatal("operated")
	}
	if got, _ := s.DMChannelsOf(ctx, alex); !slices.Equal(got, []ids.ID{dm}) {
		t.Fatal("dms")
	}
	s.Fail(true)
	for _, err := range []error{
		func() error { _, err := s.Space(ctx, space); return err }(),
		func() error { _, err := s.Spaces(ctx); return err }(),
		func() error { _, _, err := s.Channel(ctx, ch); return err }(),
		func() error { _, _, err := s.Thread(ctx, th); return err }(),
		func() error { _, err := s.Channels(ctx); return err }(),
		func() error { _, err := s.ChannelsOperatedBy(ctx, agent); return err }(),
		func() error { _, err := s.DMChannelsOf(ctx, alex); return err }(),
	} {
		if !errors.Is(err, fake.ErrInjected) {
			t.Fatalf("got %v", err)
		}
	}
}

func TestPoliciesAndCredentials(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	p := fake.NewPolicies()
	agent, user, ch := ids.New(), ids.New(), ids.New()
	p.Put(agent, authz.AgentPolicy{BillingMode: "organization"})
	p.SetDevReady(user, ch, true)
	if pol, ok, _ := p.AgentPolicy(ctx, agent); !ok || pol.BillingMode != "organization" {
		t.Fatal("policy")
	}
	if ok, _ := p.DevEnvironmentReady(ctx, user, ch); !ok {
		t.Fatal("ready")
	}
	p.Fail(true)
	if _, _, err := p.AgentPolicy(ctx, agent); err == nil {
		t.Fatal("no error")
	}
	if _, err := p.DevEnvironmentReady(ctx, user, ch); err == nil {
		t.Fatal("no error")
	}

	c := fake.NewCredentials()
	sess, dev, tok := ids.New(), ids.New(), ids.New()
	c.PutDevice(dev, fake.Device{PrincipalID: user, State: authz.DeviceActive})
	c.PutSession(sess, fake.Session{PrincipalID: user, DeviceID: dev, ExpiresAt: at})
	c.PutToken(tok, fake.Token{PrincipalID: user, ExpiresAt: at})
	f, _ := c.Credentials(ctx, authz.CredentialRef{PrincipalID: user, SessionID: sess, DeviceID: dev, TokenID: tok})
	if !f.SessionActive || f.DeviceState != authz.DeviceActive || !f.TokenActive {
		t.Fatalf("%+v", f)
	}
	// Someone else's credentials are unknown.
	f, _ = c.Credentials(ctx, authz.CredentialRef{PrincipalID: agent, SessionID: sess, DeviceID: dev, TokenID: tok})
	if f.SessionActive || f.DeviceState != authz.DeviceUnknown || f.TokenActive {
		t.Fatalf("%+v", f)
	}
	c.Fail(true)
	if _, err := c.Credentials(ctx, authz.CredentialRef{}); err == nil {
		t.Fatal("no error")
	}
}

func TestStoreAndVersions(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := fake.NewStore()
	g, m, gr, user := ids.New(), ids.New(), ids.New(), ids.New()
	if err := s.InsertGroup(ctx, domain.Group{ID: g, Name: "Leser"}); err != nil {
		t.Fatal(err)
	}
	_ = s.InsertMembership(ctx, domain.Membership{ID: m, GroupID: g, UserID: user})
	_ = s.InsertGrant(ctx, domain.Grant{ID: gr, Subject: domain.Subject{Kind: domain.SubjectGroup, GroupID: g}, Target: domain.Target{Kind: domain.TargetInstance}, Level: domain.LevelRead})
	if got, _ := s.ActiveMembershipsOfUser(ctx, user); len(got) != 1 || got[0].Group.ID != g {
		t.Fatal("memberships")
	}
	if got, _ := s.ActiveGrantsOfGroups(ctx, []ids.ID{g}); len(got) != 1 {
		t.Fatal("group grants")
	}
	if got, _ := s.ActiveGrantsOfGroups(ctx, nil); got != nil {
		t.Fatal("empty list")
	}
	if got, _ := s.ActiveGrantsOfPrincipal(ctx, user); len(got) != 0 || s.Reads() != 1 {
		t.Fatal("principal grants")
	}
	if gg, ok, _ := s.GetGroup(ctx, g); !ok || gg.Name != "Leser" {
		t.Fatal("group")
	}
	if _, ok, _ := s.GetGrant(ctx, gr); !ok {
		t.Fatal("grant")
	}
	if ok, _ := s.RemoveMembership(ctx, m, user, at); !ok {
		t.Fatal("remove")
	}
	if ok, _ := s.RemoveMembership(ctx, m, user, at); ok {
		t.Fatal("remove twice")
	}
	if _, ok, _ := s.RevokeGrant(ctx, gr, user, "r", at); !ok {
		t.Fatal("revoke")
	}
	if _, ok, _ := s.RevokeGrant(ctx, gr, user, "r", at); ok {
		t.Fatal("revoke twice")
	}
	if ok, _ := s.SoftDeleteGroup(ctx, g, at); !ok {
		t.Fatal("delete")
	}
	if ok, _ := s.SoftDeleteGroup(ctx, g, at); ok {
		t.Fatal("delete twice")
	}
	s.Fail(true)
	for _, err := range []error{
		s.InsertGroup(ctx, domain.Group{}), s.InsertMembership(ctx, domain.Membership{}), s.InsertGrant(ctx, domain.Grant{}),
		func() error { _, _, err := s.GetGroup(ctx, g); return err }(),
		func() error { _, err := s.SoftDeleteGroup(ctx, g, at); return err }(),
		func() error { _, err := s.RemoveMembership(ctx, m, user, at); return err }(),
		func() error { _, err := s.ActiveMembershipsOfUser(ctx, user); return err }(),
		func() error { _, _, err := s.GetGrant(ctx, gr); return err }(),
		func() error { _, _, err := s.RevokeGrant(ctx, gr, user, "", at); return err }(),
		func() error { _, err := s.ActiveGrantsOfPrincipal(ctx, user); return err }(),
		func() error { _, err := s.ActiveGrantsOfGroups(ctx, []ids.ID{g}); return err }(),
	} {
		if !errors.Is(err, fake.ErrInjected) {
			t.Fatalf("got %v", err)
		}
	}

	var v fake.Versions
	v.Set(3)
	if v.Bump() != 4 {
		t.Fatal("bump")
	}
	if n, _ := v.PermissionVersion(ctx); n != 4 || v.Reads() != 1 {
		t.Fatal("read")
	}
	v.Fail(true)
	if _, err := v.PermissionVersion(ctx); err == nil {
		t.Fatal("no error")
	}
}

// TestStoreClaimQueries: the in-memory claim queries match the PostgreSQL ones (M1-42).
func TestStoreClaimQueries(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := fake.NewStore()
	prov, other, user := ids.New(), ids.New(), ids.New()
	support := domain.Group{ID: ids.New(), Name: "Support", Source: domain.GroupSourceOIDCClaim, ProviderID: prov, ExternalRef: "support"}
	foreign := domain.Group{ID: ids.New(), Name: "Foreign", Source: domain.GroupSourceOIDCClaim, ProviderID: other, ExternalRef: "support"}
	local := domain.Group{ID: ids.New(), Name: "Local", Source: domain.GroupSourceLocal}
	for _, g := range []domain.Group{support, foreign, local} {
		if err := s.InsertGroup(ctx, g); err != nil {
			t.Fatal(err)
		}
	}
	if got, err := s.ClaimGroupsByRefs(ctx, prov, []string{"support"}); err != nil || len(got) != 1 || got[0].ID != support.ID {
		t.Fatalf("ClaimGroupsByRefs %+v %v", got, err)
	}
	m := domain.Membership{ID: ids.New(), GroupID: support.ID, UserID: user, ExpiresAt: at, ExpiryRemindedAt: at, ExpiryReportedAt: at}
	for _, x := range []domain.Membership{m, {ID: ids.New(), GroupID: foreign.ID, UserID: user}, {ID: ids.New(), GroupID: local.ID, UserID: user}} {
		if err := s.InsertMembership(ctx, x); err != nil {
			t.Fatal(err)
		}
	}
	got, err := s.LockClaimMembershipsOfUser(ctx, user, prov)
	if err != nil || len(got) != 1 || got[0].Membership.ID != m.ID {
		t.Fatalf("LockClaimMembershipsOfUser %+v %v", got, err)
	}
	if ok, err := s.ExtendMembership(ctx, m.ID, at.Add(time.Hour), at); err != nil || !ok {
		t.Fatalf("ExtendMembership %v %v", ok, err)
	}
	got, _ = s.LockClaimMembershipsOfUser(ctx, user, prov)
	if e := got[0].Membership; !e.ExpiresAt.Equal(at.Add(time.Hour)) || !e.ExpiryRemindedAt.IsZero() || !e.ExpiryReportedAt.IsZero() {
		t.Fatalf("after extension %+v", e)
	}
	if ok, _ := s.ExtendMembership(ctx, ids.New(), at, at); ok {
		t.Fatal("an unknown membership was extended")
	}
}

// TestStructureDetails: the fake reports the details of PutSpaceDetail and the names of
// PutChannelDetail over the facts of PutChannel, and leaves unknown ids out (M1-49).
func TestStructureDetails(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := fake.NewStructure()
	space, plain, op, ch, other := ids.New(), ids.New(), ids.New(), ids.New(), ids.New()
	s.PutSpaceDetail(authz.SpaceDetail{ID: space, Name: "Marketing", SortKey: "a1", DefaultOperatorID: op, Version: 2, CreatedAt: at})
	s.PutSpace(plain)
	s.PutChannel(authz.ChannelFacts{ID: ch, SpaceID: space, Kind: authz.ChannelText, EffectiveOperatorID: op, ReadOnly: true})
	s.PutChannelDetail(authz.ChannelDetail{ID: ch, Name: "Kampagnen", Topic: "t", OperatorMode: authz.OperatorInherit})
	sd, err := s.SpaceDetails(ctx, []ids.ID{space, plain, other})
	if err != nil || len(sd) != 2 || sd[space].Name != "Marketing" || sd[space].DefaultOperatorID != op || sd[plain].Name != plain.String() {
		t.Fatalf("SpaceDetails = %+v, %v", sd, err)
	}
	if served, _ := s.ServedTargets(ctx, op); !slices.Contains(served, authz.ChannelRef{SpaceID: space}) {
		t.Fatalf("the default operator of PutSpaceDetail serves the space: %v", served)
	}
	cd, err := s.ChannelDetails(ctx, []ids.ID{ch, other})
	c := cd[ch]
	if err != nil || len(cd) != 1 || c.Name != "Kampagnen" || c.SpaceID != space || !c.ReadOnly || c.EffectiveOperatorID != op ||
		c.OperatorMode != authz.OperatorInherit || c.Kind != authz.ChannelText {
		t.Fatalf("ChannelDetails = %+v, %v", cd, err)
	}
	s.Fail(true)
	if _, err := s.SpaceDetails(ctx, []ids.ID{space}); !errors.Is(err, fake.ErrInjected) {
		t.Fatal(err)
	}
	if _, err := s.ChannelDetails(ctx, []ids.ID{ch}); !errors.Is(err, fake.ErrInjected) {
		t.Fatal(err)
	}
}

// TestHosts (M2-39): the fake host source serves its hosts, ascending, the hosts of a holder,
// and fails on demand.
func TestHosts(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	h := fake.NewHosts()
	alex := ids.New()
	a, b := ids.New(), ids.New()
	h.Put(authz.HostFacts{ID: a, State: "active", AssignedUserID: alex})
	h.Put(authz.HostFacts{ID: b, State: "retired"})
	if f, ok, err := h.Host(ctx, a); err != nil || !ok || f.AssignedUserID != alex {
		t.Fatalf("Host = %+v, %v, %v", f, ok, err)
	}
	if _, ok, _ := h.Host(ctx, ids.New()); ok {
		t.Fatal("an unknown host is found")
	}
	if all, err := h.HostIDs(ctx); err != nil || len(all) != 2 || bytes.Compare(all[0][:], all[1][:]) > 0 {
		t.Fatalf("HostIDs = %v, %v", all, err)
	}
	if mine, err := h.AssignedTo(ctx, alex); err != nil || len(mine) != 1 || mine[0] != a {
		t.Fatalf("AssignedTo = %v, %v", mine, err)
	}
	if none, _ := h.AssignedTo(ctx, ids.ID{}); len(none) != 0 {
		t.Fatal("the zero holder holds hosts")
	}
	h.Fail(true)
	if _, _, err := h.Host(ctx, a); err == nil {
		t.Fatal("Fail(true) does not fail Host")
	}
	if _, err := h.HostIDs(ctx); err == nil {
		t.Fatal("Fail(true) does not fail HostIDs")
	}
	if _, err := h.AssignedTo(ctx, alex); err == nil {
		t.Fatal("Fail(true) does not fail AssignedTo")
	}
	s := fake.NewStore()
	if err := s.InsertGrant(ctx, domain.Grant{ID: ids.New(), Subject: domain.Subject{Kind: domain.SubjectUser, PrincipalID: alex},
		Target: domain.Target{Kind: domain.TargetHost, HostID: a}, Caps: domain.CapsOf(domain.CapTerminal)}); err != nil {
		t.Fatal(err)
	}
	if got, err := s.ActiveGrantsOnHosts(ctx, []ids.ID{a}); err != nil || len(got) != 1 {
		t.Fatalf("ActiveGrantsOnHosts = %v, %v", got, err)
	}
	if got, _ := s.ActiveGrantsOnHosts(ctx, []ids.ID{b}); len(got) != 0 {
		t.Fatal("a grant on another host")
	}
}
