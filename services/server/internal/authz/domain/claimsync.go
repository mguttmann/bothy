// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"slices"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// ClaimDiff is the result of ReconcileClaimMemberships: the group ids whose membership the claim
// sync adds, extends or removes, each sorted by id, and the expiry of the added and extended
// memberships. Empty slices are nil.
type ClaimDiff struct {
	// Add are the matched groups without a current membership: a new row with ExpiresAt.
	Add []ids.ID
	// Extend are the matched groups with a current membership, live or expired: its expires_at
	// becomes ExpiresAt (an expired row is extended, never duplicated).
	Extend []ids.ID
	// Remove are the groups of current memberships that the claim no longer names.
	Remove []ids.ID
	// ExpiresAt is the login time plus auth.group_claim_max_age.
	ExpiresAt time.Time
}

// Empty reports a diff without anything to add, extend or remove: the sync writes nothing and
// does not raise permission_version (docs/spec/08-identity.md section 3.4 item 3).
func (d ClaimDiff) Empty() bool { return len(d.Add) == 0 && len(d.Extend) == 0 && len(d.Remove) == 0 }

// ReconcileClaimMemberships compares the group claim of an OIDC login with the person's claim
// memberships (docs/spec/08-identity.md section 3.4, brief M1-42). Only groups with source
// oidc_claim of provider that are not deleted take part: memberships in local groups, in deleted
// groups and in claim groups of other providers are never touched, and matched groups outside
// that set are ignored.
//
// Parameters: provider is the provider of the login; current are the memberships of the person
// that are not removed, each with its group (expired ones included); matched are the claim groups
// whose external_ref is one of the claim values; claim are the values of the group claim (order
// and duplicates do not matter; values without a group are ignored); now is the login time and
// maxAge is auth.group_claim_max_age. A matched group counts only when its external_ref is in
// claim. The function is pure and never fails.
//
// Example: a person in Support whose claim names only Marketing, both mapped, gets
// Add=[Marketing], Remove=[Support].
func ReconcileClaimMemberships(provider ids.ID, current []MembershipWithGroup, matched []Group, claim []string,
	now time.Time, maxAge time.Duration) ClaimDiff {
	want := wantedClaimGroups(provider, matched, claim)
	have := map[ids.ID]bool{}
	d := ClaimDiff{ExpiresAt: now.Add(maxAge)}
	for _, m := range current {
		if !claimGroupOf(m.Group, provider) || !m.Membership.RemovedAt.IsZero() || have[m.Group.ID] {
			continue
		}
		have[m.Group.ID] = true
		if want[m.Group.ID] {
			d.Extend = append(d.Extend, m.Group.ID)
		} else {
			d.Remove = append(d.Remove, m.Group.ID)
		}
	}
	for g := range want {
		if !have[g] {
			d.Add = append(d.Add, g)
		}
	}
	for _, list := range []*[]ids.ID{&d.Add, &d.Extend, &d.Remove} {
		slices.SortFunc(*list, compareIDs)
	}
	return d
}

// claimGroupOf reports whether g takes part in the sync of provider: a live claim group of it.
func claimGroupOf(g Group, provider ids.ID) bool {
	return g.Source == GroupSourceOIDCClaim && g.ProviderID == provider && g.DeletedAt.IsZero()
}

// wantedClaimGroups returns the ids of the matched groups of provider whose claim value is in
// claim.
func wantedClaimGroups(provider ids.ID, matched []Group, claim []string) map[ids.ID]bool {
	want := map[ids.ID]bool{}
	for _, g := range matched {
		if claimGroupOf(g, provider) && slices.Contains(claim, g.ExternalRef) {
			want[g.ID] = true
		}
	}
	return want
}
