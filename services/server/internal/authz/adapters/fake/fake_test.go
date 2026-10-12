// SPDX-License-Identifier: Apache-2.0

package fake_test

import (
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
