// SPDX-License-Identifier: Apache-2.0

package fake_test

import (
	"context"
	"errors"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/authz/adapters/fake"
	"github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// TestStore_GroupsMembershipsAndGrants walks the write and read methods the rights changes of
// M1-29 use, including the cases that report nothing to change.
func TestStore_GroupsMembershipsAndGrants(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := fake.NewStore()
	g, other, user, admin := ids.New(), ids.New(), ids.New(), ids.New()
	must(t, s.InsertGroup(ctx, domain.Group{ID: g, Name: "ops"}))
	if ok, err := s.UpdateGroup(ctx, g, "ops2", "d"); err != nil || !ok {
		t.Fatalf("UpdateGroup = %v, %v", ok, err)
	}
	if got, ok, _ := s.GetGroup(ctx, g); !ok || got.Name != "ops2" || got.Description != "d" {
		t.Fatalf("GetGroup = %+v, %v", got, ok)
	}
	if ok, _ := s.UpdateGroup(ctx, other, "x", ""); ok {
		t.Error("UpdateGroup of a missing group reported a change")
	}

	m1, m2 := ids.New(), ids.New()
	must(t, s.InsertMembership(ctx, domain.Membership{ID: m1, GroupID: g, UserID: user}))
	must(t, s.InsertMembership(ctx, domain.Membership{ID: m2, GroupID: g, UserID: admin}))
	if got, _ := s.ActiveMembershipsOfGroups(ctx, []ids.ID{g}); len(got) != 2 {
		t.Fatalf("ActiveMembershipsOfGroups = %d, want 2", len(got))
	}
	if ok, err := s.RemoveMembership(ctx, m1, admin, at); err != nil || !ok {
		t.Fatalf("RemoveMembership = %v, %v", ok, err)
	}
	if ok, _ := s.RemoveMembership(ctx, m1, admin, at); ok {
		t.Error("removing a removed membership reported a change")
	}
	if got, _ := s.ActiveMembershipsOfUser(ctx, user); len(got) != 0 {
		t.Errorf("ActiveMembershipsOfUser after removal = %d, want 0", len(got))
	}
	if got, _ := s.MembershipsOfUserInGroup(ctx, g, user); len(got) != 1 || got[0].RemovedAt.IsZero() {
		t.Errorf("MembershipsOfUserInGroup = %+v, want the removed row", got)
	}

	space, channel := ids.New(), ids.New()
	trigger := domain.Grant{ID: ids.New(), Subject: domain.Subject{Kind: domain.SubjectUser, PrincipalID: user},
		Target: domain.Target{Kind: domain.TargetChannel, ChannelID: channel}, Level: domain.LevelTrigger}
	onSpace := domain.Grant{ID: ids.New(), Subject: domain.Subject{Kind: domain.SubjectGroup, GroupID: g},
		Target: domain.Target{Kind: domain.TargetSpace, SpaceID: space}, Level: domain.LevelRead}
	onInstance := domain.Grant{ID: ids.New(), Subject: domain.Subject{Kind: domain.SubjectUser, PrincipalID: admin},
		Target: domain.Target{Kind: domain.TargetInstance}, Level: domain.LevelRead}
	onHost := domain.Grant{ID: ids.New(), Subject: domain.Subject{Kind: domain.SubjectUser, PrincipalID: admin},
		Target: domain.Target{Kind: domain.TargetHost, HostID: ids.New()}, Level: domain.LevelNone}
	for _, gr := range []domain.Grant{trigger, onSpace, onInstance, onHost} {
		must(t, s.InsertGrant(ctx, gr))
	}
	if got, _ := s.ActiveTriggerGrants(ctx); len(got) != 1 || got[0].ID != trigger.ID {
		t.Errorf("ActiveTriggerGrants = %+v", got)
	}
	if got, _ := s.ActiveGrantsOnTargets(ctx, []ids.ID{space}, []ids.ID{channel}); len(got) != 3 {
		t.Errorf("ActiveGrantsOnTargets = %d grants, want instance, space and channel", len(got))
	}
	if got, _ := s.ActiveGrantsOfGroups(ctx, []ids.ID{g}); len(got) != 1 {
		t.Errorf("ActiveGrantsOfGroups = %d, want 1", len(got))
	}
	if got, _ := s.ActiveGrantsOfGroups(ctx, nil); got != nil {
		t.Errorf("ActiveGrantsOfGroups(nil) = %+v, want nil", got)
	}
	revoked, ok, err := s.RevokeGrant(ctx, trigger.ID, admin, "test", at)
	if err != nil || !ok || revoked.RevokedBy != admin || revoked.RevokeReason != "test" {
		t.Fatalf("RevokeGrant = %+v, %v, %v", revoked, ok, err)
	}
	if _, ok, _ := s.RevokeGrant(ctx, trigger.ID, admin, "again", at); ok {
		t.Error("revoking a revoked grant reported a change")
	}
	if got, ok, _ := s.GetGrant(ctx, trigger.ID); !ok || got.RevokedAt.IsZero() {
		t.Errorf("GetGrant after revoke = %+v, %v", got, ok)
	}
	if ok, err := s.SoftDeleteGroup(ctx, g, at); err != nil || !ok {
		t.Fatalf("SoftDeleteGroup = %v, %v", ok, err)
	}
	if ok, _ := s.SoftDeleteGroup(ctx, g, at); ok {
		t.Error("deleting a deleted group reported a change")
	}
	if ok, _ := s.UpdateGroup(ctx, g, "x", ""); ok {
		t.Error("UpdateGroup of a deleted group reported a change")
	}
}

// TestStore_InjectedErrorReachesEveryRead holds the failer on the methods of M1-29.
func TestStore_InjectedErrorReachesEveryRead(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := fake.NewStore()
	s.Fail(true)
	id := ids.New()
	_, _, e1 := s.GetGroup(ctx, id)
	_, e2 := s.UpdateGroup(ctx, id, "a", "")
	_, e3 := s.SoftDeleteGroup(ctx, id, at)
	_, e4 := s.RemoveMembership(ctx, id, id, at)
	_, e5 := s.ActiveMembershipsOfUser(ctx, id)
	_, e6 := s.ActiveMembershipsOfGroups(ctx, []ids.ID{id})
	_, e7 := s.MembershipsOfUserInGroup(ctx, id, id)
	_, e8 := s.ActiveTriggerGrants(ctx)
	_, e9 := s.ActiveGrantsOnTargets(ctx, nil, nil)
	_, _, e10 := s.GetGrant(ctx, id)
	_, _, e11 := s.RevokeGrant(ctx, id, id, "", at)
	_, e12 := s.ActiveGrantsOfGroups(ctx, []ids.ID{id})
	for i, err := range []error{e1, e2, e3, e4, e5, e6, e7, e8, e9, e10, e11, e12} {
		if !errors.Is(err, fake.ErrInjected) {
			t.Errorf("call %d: err = %v, want ErrInjected", i+1, err)
		}
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
