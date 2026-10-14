// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// plain runs fn in a unit of work that is not a rights change.
func (e env) plain(fn func(ctx context.Context) error) error {
	return e.uow.Do(context.Background(), fn)
}

// TestRightsWritesNeedARightsChange (M1-29): memberships, grants and the deletion of a group
// change rights, so the store refuses them in a unit of work without RightsChange; groups are
// created and renamed without one.
func TestRightsWritesNeedARightsChange(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	alex := e.principal(t, "user", "alex")
	sam := e.principal(t, "user", "sam")
	g, err := domain.NewGroup(ids.New(), "Support", "", limits, alex, t0)
	if err != nil {
		t.Fatal(err)
	}
	if err := e.plain(func(ctx context.Context) error { return e.repo.InsertGroup(ctx, g) }); err != nil {
		t.Fatalf("InsertGroup without a rights change: %v", err)
	}
	if err := e.plain(func(ctx context.Context) error {
		_, err := e.repo.UpdateGroup(ctx, g.ID, "Support Team", "the team")
		return err
	}); err != nil {
		t.Fatalf("UpdateGroup without a rights change: %v", err)
	}
	m, _ := domain.NewMembership(ids.New(), g.ID, sam, alex, time.Time{}, t0)
	gr, _ := domain.NewGrant(ids.New(), domain.Subject{Kind: domain.SubjectUser, PrincipalID: sam}, domain.Target{Kind: domain.TargetInstance},
		domain.LevelRead, 0, time.Time{}, "reading", alex, t0)
	writes := map[string]func(ctx context.Context) error{
		"InsertMembership": func(ctx context.Context) error { return e.repo.InsertMembership(ctx, m) },
		"RemoveMembership": func(ctx context.Context) error { _, err := e.repo.RemoveMembership(ctx, m.ID, alex, t0); return err },
		"InsertGrant":      func(ctx context.Context) error { return e.repo.InsertGrant(ctx, gr) },
		"RevokeGrant":      func(ctx context.Context) error { _, _, err := e.repo.RevokeGrant(ctx, gr.ID, alex, "", t0); return err },
		"SoftDeleteGroup":  func(ctx context.Context) error { _, err := e.repo.SoftDeleteGroup(ctx, g.ID, t0); return err },
	}
	for name, w := range writes {
		wantError(t, e.plain(w), errs.KindInternal, errs.CodeInternalError, "")
		if err := e.write(w); err != nil {
			t.Errorf("%s with a rights change: %v", name, err)
		}
	}
}

// TestUpdateGroup: rename and describe a live group; a taken name is conflict on /name; a deleted
// or unknown group changes nothing.
func TestUpdateGroup(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ctx := context.Background()
	alex := e.principal(t, "user", "alex")
	support := e.group(t, "Support", alex)
	e.group(t, "Marketing", alex)
	var changed bool
	if err := e.plain(func(ctx context.Context) (err error) {
		changed, err = e.repo.UpdateGroup(ctx, support.ID, "First Line", "first line")
		return err
	}); err != nil || !changed {
		t.Fatalf("UpdateGroup: %v %v", changed, err)
	}
	got, _, _ := e.repo.GetGroup(ctx, support.ID)
	if got.Name != "First Line" || got.Description != "first line" {
		t.Fatalf("group %+v", got)
	}
	err := e.plain(func(ctx context.Context) error {
		_, err := e.repo.UpdateGroup(ctx, support.ID, "MARKETING", "")
		return err
	})
	wantError(t, err, errs.KindConflict, errs.CodeConflict, "/name")
	if err := e.write(func(ctx context.Context) error { _, err := e.repo.SoftDeleteGroup(ctx, support.ID, t0); return err }); err != nil {
		t.Fatal(err)
	}
	if err := e.plain(func(ctx context.Context) (err error) {
		changed, err = e.repo.UpdateGroup(ctx, support.ID, "Again", "")
		return err
	}); err != nil || changed {
		t.Fatalf("UpdateGroup of a deleted group: %v %v", changed, err)
	}
}

// TestI1Reads (M1-29): the trigger grants that are not revoked, the active memberships of groups
// with their group rows, the grants on targets with the instance, and the memberships of one
// person in one group with history.
func TestI1Reads(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ctx := context.Background()
	alex := e.principal(t, "user", "alex")
	sam := e.principal(t, "user", "sam")
	robin := e.principal(t, "user", "robin")
	support := e.group(t, "Support", alex)
	other := e.group(t, "Other", alex)
	space, channel, foreign := e.space(t, alex), e.channel(t, alex), e.channel(t, alex)
	grant := func(s domain.Subject, tg domain.Target, l domain.Level) domain.Grant {
		t.Helper()
		g, err := domain.NewGrant(ids.New(), s, tg, l, 0, time.Time{}, "work", alex, t0)
		if err != nil {
			t.Fatal(err)
		}
		if err := e.write(func(ctx context.Context) error { return e.repo.InsertGrant(ctx, g) }); err != nil {
			t.Fatal(err)
		}
		return g
	}
	user := func(id ids.ID) domain.Subject { return domain.Subject{Kind: domain.SubjectUser, PrincipalID: id} }
	gInst := grant(user(sam), domain.Target{Kind: domain.TargetInstance}, domain.LevelTrigger)
	gSpace := grant(domain.Subject{Kind: domain.SubjectGroup, GroupID: support.ID}, domain.Target{Kind: domain.TargetSpace, SpaceID: space}, domain.LevelTrigger)
	gRead := grant(user(robin), domain.Target{Kind: domain.TargetChannel, ChannelID: channel}, domain.LevelRead)
	gForeign := grant(user(robin), domain.Target{Kind: domain.TargetChannel, ChannelID: foreign}, domain.LevelTrigger)
	gRevoked := grant(user(alex), domain.Target{Kind: domain.TargetChannel, ChannelID: channel}, domain.LevelTrigger)
	if err := e.write(func(ctx context.Context) error {
		_, _, err := e.repo.RevokeGrant(ctx, gRevoked.ID, alex, "", t0)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	trig, err := e.repo.ActiveTriggerGrants(ctx)
	if err != nil || !reflect.DeepEqual(trig, []domain.Grant{gInst, gSpace, gForeign}) {
		t.Fatalf("ActiveTriggerGrants = %+v, %v", trig, err)
	}
	on, err := e.repo.ActiveGrantsOnTargets(ctx, []ids.ID{space}, []ids.ID{channel})
	if err != nil || !reflect.DeepEqual(on, []domain.Grant{gInst, gSpace, gRead}) {
		t.Fatalf("ActiveGrantsOnTargets = %+v, %v", on, err)
	}
	add := func(g domain.Group, u ids.ID) domain.Membership {
		t.Helper()
		m, _ := domain.NewMembership(ids.New(), g.ID, u, alex, time.Time{}, t0)
		if err := e.write(func(ctx context.Context) error { return e.repo.InsertMembership(ctx, m) }); err != nil {
			t.Fatal(err)
		}
		return m
	}
	m1 := add(support, sam)
	m2 := add(support, robin)
	add(other, alex)
	if err := e.write(func(ctx context.Context) error {
		_, err := e.repo.RemoveMembership(ctx, m2.ID, alex, t0.Add(time.Minute))
		return err
	}); err != nil {
		t.Fatal(err)
	}
	m3 := add(support, robin)
	ms, err := e.repo.ActiveMembershipsOfGroups(ctx, []ids.ID{support.ID})
	if err != nil || !reflect.DeepEqual(ms, []domain.MembershipWithGroup{{Membership: m1, Group: support}, {Membership: m3, Group: support}}) {
		t.Fatalf("ActiveMembershipsOfGroups = %+v, %v", ms, err)
	}
	if none, err := e.repo.ActiveMembershipsOfGroups(ctx, nil); err != nil || len(none) != 0 {
		t.Fatalf("no groups: %v %v", none, err)
	}
	hist, err := e.repo.MembershipsOfUserInGroup(ctx, support.ID, robin)
	removed := m2
	removed.RemovedAt, removed.RemovedBy = t0.Add(time.Minute), alex
	if err != nil || !reflect.DeepEqual(hist, []domain.Membership{removed, m3}) {
		t.Fatalf("MembershipsOfUserInGroup = %+v, %v", hist, err)
	}
}
