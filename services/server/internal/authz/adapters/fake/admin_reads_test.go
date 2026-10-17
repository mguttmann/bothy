// SPDX-License-Identifier: Apache-2.0

package fake_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/authz/adapters/fake"
	"github.com/OWNER/bothy/services/server/internal/authz/app"
	"github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// TestStore_AdminReads walks the list reads of the group and grant API (M1-48): live groups with
// and without a member filter, member counts and lists, grants by filter, the limit, and the
// injected error of every read.
func TestStore_AdminReads(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := fake.NewStore()
	g1, g2, user := ids.New(), ids.New(), ids.New()
	must(t, s.InsertGroup(ctx, domain.Group{ID: g1, Name: "ops"}))
	must(t, s.InsertGroup(ctx, domain.Group{ID: g2, Name: "dev"}))
	must(t, s.InsertMembership(ctx, domain.Membership{ID: ids.New(), GroupID: g1, UserID: user}))
	must(t, s.InsertMembership(ctx, domain.Membership{ID: ids.New(), GroupID: g1, UserID: ids.New(), ExpiresAt: at.Add(-time.Hour)}))
	must(t, s.InsertGrant(ctx, domain.Grant{ID: ids.New(), Subject: domain.Subject{Kind: domain.SubjectUser, PrincipalID: user},
		Target: domain.Target{Kind: domain.TargetInstance}, Level: domain.LevelTrigger}))

	if got, err := s.ListLiveGroups(ctx, ids.ID{}, ids.ID{}, at, 10); err != nil || len(got) != 2 {
		t.Errorf("ListLiveGroups = %d, %v; want 2", len(got), err)
	}
	if got, _ := s.ListLiveGroups(ctx, ids.ID{}, user, at, 10); len(got) != 1 || got[0].ID != g1 {
		t.Errorf("ListLiveGroups of the member = %+v", got)
	}
	if got, _ := s.ListLiveGroups(ctx, ids.ID{}, ids.ID{}, at, 1); len(got) != 1 {
		t.Errorf("ListLiveGroups with limit 1 = %d", len(got))
	}
	if n, err := s.CountMembersInEffect(ctx, []ids.ID{g1, g2}, at); err != nil || n[g1] != 1 || n[g2] != 0 {
		t.Errorf("CountMembersInEffect = %v, %v", n, err)
	}
	if got, err := s.ListMembersInEffect(ctx, g1, ids.ID{}, at, 10); err != nil || len(got) != 1 || got[0].UserID != user {
		t.Errorf("ListMembersInEffect = %+v, %v", got, err)
	}
	if got, err := s.ListGrants(ctx, app.GrantFilter{AllTargets: true}, ids.ID{}, at, 10); err != nil || len(got) != 1 {
		t.Errorf("ListGrants = %d, %v; want 1", len(got), err)
	}

	s.Fail(true)
	if _, err := s.ListLiveGroups(ctx, ids.ID{}, ids.ID{}, at, 10); !errors.Is(err, fake.ErrInjected) {
		t.Errorf("ListLiveGroups with failure = %v", err)
	}
	if _, err := s.CountMembersInEffect(ctx, []ids.ID{g1}, at); !errors.Is(err, fake.ErrInjected) {
		t.Errorf("CountMembersInEffect with failure = %v", err)
	}
	if _, err := s.ListMembersInEffect(ctx, g1, ids.ID{}, at, 10); !errors.Is(err, fake.ErrInjected) {
		t.Errorf("ListMembersInEffect with failure = %v", err)
	}
	if _, err := s.ListGrants(ctx, app.GrantFilter{}, ids.ID{}, at, 10); !errors.Is(err, fake.ErrInjected) {
		t.Errorf("ListGrants with failure = %v", err)
	}
}

// TestPrincipals_Refs: a known principal gets its id as handle and name, an unknown one is left
// out, and the injected error comes back.
func TestPrincipals_Refs(t *testing.T) {
	t.Parallel()
	p := fake.NewPrincipals()
	id := ids.New()
	p.Put(authz.PrincipalFacts{ID: id, Kind: actor.KindUser})
	refs, err := p.Refs(context.Background(), []ids.ID{id, ids.New()})
	if err != nil || len(refs) != 1 || refs[id].Handle != id.String() {
		t.Fatalf("Refs = %+v, %v", refs, err)
	}
	p.Fail(true)
	if _, err := p.Refs(context.Background(), []ids.ID{id}); !errors.Is(err, fake.ErrInjected) {
		t.Errorf("Refs with failure = %v", err)
	}
}
