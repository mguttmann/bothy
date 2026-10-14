// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// TestReconcileClaimMemberships is the table of the claim reconciliation (brief M1-42 "Unit
// tests"): add, extend of a live and of an expired membership, remove, unknown values ignored,
// local groups untouched, empty claim removes all claim memberships, duplicate claim values,
// deleted groups ignored, groups of other providers untouched, output sorted.
func TestReconcileClaimMemberships(t *testing.T) {
	t.Parallel()
	provider, other := ids.New(), ids.New()
	maxAge := 30 * 24 * time.Hour
	claimGroup := func(ref string) domain.Group {
		return domain.Group{ID: ids.New(), Name: ref, Source: domain.GroupSourceOIDCClaim, ProviderID: provider, ExternalRef: ref}
	}
	support, marketing, engineering, research := claimGroup("support"), claimGroup("marketing"), claimGroup("engineering"), claimGroup("research")
	foreign := domain.Group{ID: ids.New(), Name: "Foreign", Source: domain.GroupSourceOIDCClaim, ProviderID: other, ExternalRef: "support"}
	local := domain.Group{ID: ids.New(), Name: "Local", Source: domain.GroupSourceLocal}
	deleted := claimGroup("deleted")
	deleted.DeletedAt = t0.Add(-time.Hour)
	member := func(g domain.Group, expires time.Time) domain.MembershipWithGroup {
		return domain.MembershipWithGroup{Membership: domain.Membership{ID: ids.New(), GroupID: g.ID, ExpiresAt: expires}, Group: g}
	}
	live, expired := t0.Add(time.Hour), t0.Add(-time.Hour)
	sorted := func(list ...ids.ID) []ids.ID { return domain.SortIDs(list) }
	cases := []struct {
		name    string
		current []domain.MembershipWithGroup
		matched []domain.Group
		claim   []string
		add     []ids.ID
		extend  []ids.ID
		remove  []ids.ID
	}{
		{name: "add", matched: []domain.Group{support}, claim: []string{"support"}, add: []ids.ID{support.ID}},
		{name: "extend live", current: []domain.MembershipWithGroup{member(support, live)}, matched: []domain.Group{support},
			claim: []string{"support"}, extend: []ids.ID{support.ID}},
		{name: "extend expired", current: []domain.MembershipWithGroup{member(support, expired)}, matched: []domain.Group{support},
			claim: []string{"support"}, extend: []ids.ID{support.ID}},
		{name: "remove", current: []domain.MembershipWithGroup{member(support, live)}, claim: []string{"marketing"},
			remove: []ids.ID{support.ID}},
		{name: "unknown values ignored", matched: []domain.Group{support}, claim: []string{"support", "unknown", ""}, add: []ids.ID{support.ID}},
		{name: "local groups untouched", current: []domain.MembershipWithGroup{member(local, time.Time{})}, claim: []string{}},
		{name: "empty claim removes all claim memberships",
			current: []domain.MembershipWithGroup{member(support, live), member(marketing, expired), member(local, time.Time{})},
			claim:   []string{}, remove: sorted(support.ID, marketing.ID)},
		{name: "duplicate claim values", matched: []domain.Group{support, support}, claim: []string{"support", "support"}, add: []ids.ID{support.ID}},
		{name: "deleted groups ignored", current: []domain.MembershipWithGroup{member(deleted, live)}, matched: []domain.Group{deleted},
			claim: []string{"deleted"}},
		{name: "other provider untouched", current: []domain.MembershipWithGroup{member(foreign, live)}, matched: []domain.Group{foreign},
			claim: []string{"support"}},
		{name: "matched group whose value is not in the claim", matched: []domain.Group{research}, claim: []string{"support"}},
		{name: "output sorted",
			current: []domain.MembershipWithGroup{member(research, live), member(marketing, live), member(engineering, live)},
			matched: []domain.Group{engineering, support, research}, claim: []string{"research", "support", "engineering"},
			add: []ids.ID{support.ID}, extend: sorted(research.ID, engineering.ID), remove: []ids.ID{marketing.ID}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			d := domain.ReconcileClaimMemberships(provider, c.current, c.matched, c.claim, t0, maxAge)
			if !reflect.DeepEqual(d.Add, c.add) || !reflect.DeepEqual(d.Extend, c.extend) || !reflect.DeepEqual(d.Remove, c.remove) {
				t.Fatalf("diff add %v extend %v remove %v, want %v %v %v", d.Add, d.Extend, d.Remove, c.add, c.extend, c.remove)
			}
			if !d.ExpiresAt.Equal(t0.Add(maxAge)) {
				t.Fatalf("expires_at %v", d.ExpiresAt)
			}
			if d.Empty() != (len(c.add)+len(c.extend)+len(c.remove) == 0) {
				t.Fatalf("Empty %v", d.Empty())
			}
		})
	}
}
