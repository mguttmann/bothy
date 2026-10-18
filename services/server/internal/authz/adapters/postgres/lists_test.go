// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/authz/app"
	"github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// TestListQueries (M1-48): the keyset pages of groups, members and grants, the member filter and
// member_count at the second of an expiry, the grant filters, include_revoked and the manage
// scope applied in the query.
func TestListQueries(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ctx := context.Background()
	owner := e.principal(t, "user", "owner")
	sam := e.principal(t, "user", "sam")
	robin := e.principal(t, "user", "robin")
	a, b := e.group(t, "A", owner), e.group(t, "B", owner)
	deleted := e.group(t, "C", owner)
	exp := t0.Add(time.Hour)
	ms := []domain.Membership{
		{ID: ids.New(), GroupID: a.ID, UserID: sam, AddedBy: owner, AddedAt: t0},
		{ID: ids.New(), GroupID: a.ID, UserID: robin, AddedBy: owner, AddedAt: t0, ExpiresAt: exp},
		{ID: ids.New(), GroupID: b.ID, UserID: robin, AddedBy: owner, AddedAt: t0},
	}
	space, channel, other := e.space(t, owner), ids.New(), ids.New()
	user := func(id ids.ID) domain.Subject { return domain.Subject{Kind: domain.SubjectUser, PrincipalID: id} }
	grant := func(s domain.Subject, tg domain.Target, expires time.Time) domain.Grant {
		g, err := domain.NewGrant(ids.New(), s, tg, domain.LevelRead, 0, expires, "r", owner, t0)
		if err != nil {
			t.Fatal(err)
		}
		return g
	}
	gs := []domain.Grant{
		grant(user(sam), domain.Target{Kind: domain.TargetSpace, SpaceID: space}, time.Time{}),
		grant(user(robin), domain.Target{Kind: domain.TargetChannel, ChannelID: channel}, exp),
		grant(domain.Subject{Kind: domain.SubjectGroup, GroupID: a.ID}, domain.Target{Kind: domain.TargetChannel, ChannelID: other}, time.Time{}),
		grant(user(sam), domain.Target{Kind: domain.TargetInstance}, time.Time{}),
	}
	err := e.write(func(ctx context.Context) error {
		for _, m := range ms {
			if err := e.repo.InsertMembership(ctx, m); err != nil {
				return err
			}
		}
		for _, g := range gs {
			if err := e.repo.InsertGrant(ctx, g); err != nil {
				return err
			}
		}
		_, err := e.repo.SoftDeleteGroup(ctx, deleted.ID, t0)
		return err
	})
	mustNil(t, err)

	groups, err := e.repo.ListLiveGroups(ctx, ids.ID{}, ids.ID{}, t0, 10)
	mustNil(t, err)
	if got := groupIDs(groups); !slices.Equal(got, []ids.ID{a.ID, b.ID}) {
		t.Fatalf("live groups %v", got)
	}
	if page, _ := e.repo.ListLiveGroups(ctx, a.ID, ids.ID{}, t0, 1); !slices.Equal(groupIDs(page), []ids.ID{b.ID}) {
		t.Fatalf("page after A %v", groupIDs(page))
	}
	if at, _ := e.repo.ListLiveGroups(ctx, ids.ID{}, robin, exp, 10); !slices.Equal(groupIDs(at), []ids.ID{b.ID}) {
		t.Fatalf("Robin's groups at the expiry %v", groupIDs(at))
	}
	counts, err := e.repo.CountMembersInEffect(ctx, []ids.ID{a.ID, b.ID, deleted.ID}, exp.Add(-time.Second))
	mustNil(t, err)
	if counts[a.ID] != 2 || counts[b.ID] != 1 || counts[deleted.ID] != 0 {
		t.Fatalf("counts before the expiry %v", counts)
	}
	if counts, _ = e.repo.CountMembersInEffect(ctx, []ids.ID{a.ID}, exp); counts[a.ID] != 1 {
		t.Fatalf("count at the expiry %v", counts)
	}
	if none, err := e.repo.CountMembersInEffect(ctx, nil, exp); err != nil || len(none) != 0 {
		t.Fatalf("no groups: %v %v", none, err)
	}
	members, err := e.repo.ListMembersInEffect(ctx, a.ID, ids.ID{}, exp, 10)
	mustNil(t, err)
	if len(members) != 1 || members[0].UserID != sam {
		t.Fatalf("members at the expiry %+v", members)
	}

	list := func(f app.GrantFilter, at time.Time) []ids.ID {
		t.Helper()
		out, err := e.repo.ListGrants(ctx, f, ids.ID{}, at, 10)
		mustNil(t, err)
		return grantIDList(out)
	}
	all := app.GrantFilter{AllTargets: true}
	if got := list(all, t0); !slices.Equal(got, grantIDList(gs)) {
		t.Fatalf("all grants %v", got)
	}
	if got := list(all, exp); slices.Contains(got, gs[1].ID) {
		t.Fatal("an expired grant without include_revoked")
	}
	all.IncludeRevoked = true
	if got := list(all, exp); !slices.Contains(got, gs[1].ID) {
		t.Fatal("include_revoked leaves out the expired grant")
	}
	for name, c := range map[string]struct {
		f    app.GrantFilter
		want []ids.ID
	}{
		"target":   {app.GrantFilter{AllTargets: true, TargetKind: domain.TargetSpace, TargetID: space}, []ids.ID{gs[0].ID}},
		"instance": {app.GrantFilter{AllTargets: true, TargetKind: domain.TargetInstance}, []ids.ID{gs[3].ID}},
		"subject":  {app.GrantFilter{AllTargets: true, SubjectKind: domain.SubjectUser, SubjectID: sam}, []ids.ID{gs[0].ID, gs[3].ID}},
		"group":    {app.GrantFilter{AllTargets: true, SubjectKind: domain.SubjectGroup, SubjectID: a.ID}, []ids.ID{gs[2].ID}},
		"scope":    {app.GrantFilter{ManageSpaces: []ids.ID{space}, ManageChannels: []ids.ID{channel}}, []ids.ID{gs[0].ID, gs[1].ID}},
		"narrowed": {app.GrantFilter{ManageSpaces: []ids.ID{space}, SubjectKind: domain.SubjectUser, SubjectID: sam}, []ids.ID{gs[0].ID}},
		"empty":    {app.GrantFilter{}, nil},
	} {
		if got := list(c.f, t0); !slices.Equal(got, c.want) {
			t.Errorf("%s: %v, want %v", name, got, c.want)
		}
	}
	if page, _ := e.repo.ListGrants(ctx, app.GrantFilter{AllTargets: true}, gs[1].ID, t0, 1); !slices.Equal(grantIDList(page), []ids.ID{gs[2].ID}) {
		t.Fatalf("page after the second grant %v", grantIDList(page))
	}
}

func groupIDs(gs []domain.Group) []ids.ID {
	out := make([]ids.ID, 0, len(gs))
	for _, g := range gs {
		out = append(out, g.ID)
	}
	return out
}

func grantIDList(gs []domain.Grant) []ids.ID {
	var out []ids.ID
	for _, g := range gs {
		out = append(out, g.ID)
	}
	return out
}

func mustNil(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
