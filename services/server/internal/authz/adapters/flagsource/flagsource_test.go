// SPDX-License-Identifier: Apache-2.0

package flagsource_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/authz/adapters/flagsource"
	"github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// fakeStore holds memberships and groups.
type fakeStore struct {
	memberships []domain.MembershipWithGroup
	groups      map[ids.ID]domain.Group
	err         error
}

func (f fakeStore) ActiveMembershipsOfUser(_ context.Context, userID ids.ID) ([]domain.MembershipWithGroup, error) {
	var out []domain.MembershipWithGroup
	for _, m := range f.memberships {
		if m.Membership.UserID == userID {
			out = append(out, m)
		}
	}
	return out, f.err
}

func (f fakeStore) GetGroup(_ context.Context, id ids.ID) (domain.Group, bool, error) {
	g, ok := f.groups[id]
	return g, ok, f.err
}

func TestValidGroupIDs_UsesMembershipValidity(t *testing.T) {
	t.Parallel()
	sam := ids.New()
	t0 := time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)
	research := domain.Group{ID: ids.New()}
	support := domain.Group{ID: ids.New()}
	deleted := domain.Group{ID: ids.New(), DeletedAt: t0.Add(-time.Hour)}
	store := fakeStore{memberships: []domain.MembershipWithGroup{
		{Membership: domain.Membership{GroupID: research.ID, UserID: sam, ExpiresAt: t0}, Group: research},
		{Membership: domain.Membership{GroupID: support.ID, UserID: sam, ExpiresAt: t0.Add(time.Second)}, Group: support},
		{Membership: domain.Membership{GroupID: deleted.ID, UserID: sam}, Group: deleted},
	}}
	src, err := flagsource.New(store)
	if err != nil {
		t.Fatal(err)
	}
	got, err := src.ValidGroupIDs(context.Background(), sam, t0)
	if err != nil || !slices.Equal(got, []ids.ID{support.ID}) {
		t.Fatalf("valid groups %v %v: the membership expiring at t is invalid, a deleted group too", got, err)
	}
	store.err = errors.New("down")
	src, _ = flagsource.New(store)
	if _, err := src.ValidGroupIDs(context.Background(), sam, t0); err == nil {
		t.Error("error lost")
	}
	if _, err := flagsource.New(nil); err == nil {
		t.Error("nil store accepted")
	}
}

func TestExistingGroupIDs_NotDeleted(t *testing.T) {
	t.Parallel()
	live, gone, unknown := ids.New(), ids.New(), ids.New()
	src, _ := flagsource.New(fakeStore{groups: map[ids.ID]domain.Group{
		live: {ID: live}, gone: {ID: gone, DeletedAt: time.Now()},
	}})
	got, err := src.ExistingGroupIDs(context.Background(), []ids.ID{unknown, live, gone, live})
	if err != nil || !slices.Equal(got, []ids.ID{live}) {
		t.Fatalf("existing %v %v", got, err)
	}
}
