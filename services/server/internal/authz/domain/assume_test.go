// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// TestBuildSnapshot_AssumeValid: an expired grant and an expired membership named in Assume count
// as valid; revoked rows, removed memberships and deleted groups stay invalid (M1-30).
func TestBuildSnapshot_AssumeValid(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)
	person, space, group := ids.New(), ids.New(), ids.New()
	grant := domain.Grant{ID: ids.New(), Subject: domain.Subject{Kind: domain.SubjectUser, PrincipalID: person},
		Target: domain.Target{Kind: domain.TargetSpace, SpaceID: space}, Level: domain.LevelRead, ExpiresAt: at}
	channel := ids.New()
	groupGrant := domain.Grant{ID: ids.New(), Subject: domain.Subject{Kind: domain.SubjectGroup, GroupID: group},
		Target: domain.Target{Kind: domain.TargetChannel, ChannelID: channel}, Level: domain.LevelWrite}
	m := domain.Membership{ID: ids.New(), GroupID: group, UserID: person, ExpiresAt: at.Add(-time.Hour)}
	in := domain.SnapshotInput{PrincipalID: person, At: at, Active: true, Grants: []domain.Grant{grant},
		Memberships: []domain.MembershipWithGroup{{Membership: m, Group: domain.Group{ID: group}}}, GroupGrants: []domain.Grant{groupGrant}}
	if s := domain.BuildSnapshot(in); len(s.Spaces) != 0 || len(s.Groups) != 0 {
		t.Fatalf("expired rows counted without Assume: %+v", s)
	}
	in.Assume = domain.AssumeValid{GrantIDs: []ids.ID{grant.ID}, MembershipIDs: []ids.ID{m.ID}}
	s := domain.BuildSnapshot(in)
	if s.Spaces[space].Level != domain.LevelRead || len(s.Groups) != 1 || s.Channels[channel].Level != domain.LevelWrite {
		t.Fatalf("assumed rows not counted: %+v", s)
	}
	revoked := grant
	revoked.RevokedAt = at.Add(-time.Minute)
	in.Grants = []domain.Grant{revoked}
	in.Memberships[0].Group.DeletedAt = at.Add(-time.Minute)
	if s := domain.BuildSnapshot(in); len(s.Spaces) != 0 || len(s.Groups) != 0 {
		t.Fatalf("Assume revived a revoked grant or a deleted group: %+v", s)
	}
	// Unreported: every row without ExpiryReportedAt counts as valid, a reported one does not.
	in.Grants = []domain.Grant{grant}
	in.Memberships[0].Group.DeletedAt = time.Time{}
	in.Assume = domain.AssumeValid{Unreported: true}
	if s := domain.BuildSnapshot(in); len(s.Spaces) != 1 || len(s.Groups) != 1 {
		t.Fatalf("unreported rows not counted: %+v", s)
	}
	reported := grant
	reported.ExpiryReportedAt = at
	in.Grants = []domain.Grant{reported}
	if s := domain.BuildSnapshot(in); len(s.Spaces) != 0 || len(s.Groups) != 1 {
		t.Fatalf("a reported grant counted: %+v", s)
	}
	if !(domain.AssumeValid{}).Empty() || in.Assume.Empty() {
		t.Fatal("Empty")
	}
}
