// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"testing"

	"github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// TestEffective_ArchivedCapsAtRead: in an archived channel every effective level is at most read,
// owners included; capabilities stay (07 section 4.5 point 1).
func TestEffective_ArchivedCapsAtRead(t *testing.T) {
	t.Parallel()
	path := domain.Path{Kind: domain.PathChannel, SpaceID: spaceS, ChannelID: chanC, Archived: true, ChannelOperatorID: agentID}
	owner := domain.Snapshot{Owner: true}
	if r := domain.Effective(owner, domain.KindUser, alexID, path); r.Level != domain.LevelRead || !r.Owner {
		t.Fatalf("owner in an archived channel: %+v", r)
	}
	if r := domain.Effective(domain.Snapshot{}, domain.KindAgent, agentID, path); r.Level != domain.LevelRead {
		t.Fatalf("operator in an archived channel: %+v", r)
	}
	none := domain.Effective(domain.Snapshot{}, domain.KindUser, samID, path)
	if none.Level != domain.LevelNone {
		t.Fatalf("archiving opened a channel: %+v", none)
	}
}

// TestEffective_ArchivedSpaceCapsAtRead: in an archived space every effective level on the space is
// at most read, owners included; capabilities stay, and archiving opens nothing (07 section 4.5,
// M1-52).
func TestEffective_ArchivedSpaceCapsAtRead(t *testing.T) {
	t.Parallel()
	path := domain.Path{Kind: domain.PathSpace, SpaceID: spaceS, Archived: true}
	if r := domain.Effective(domain.Snapshot{Owner: true}, domain.KindUser, alexID, path); r.Level != domain.LevelRead || !r.Owner {
		t.Fatalf("owner in an archived space: %+v", r)
	}
	writer := domain.Snapshot{Spaces: map[ids.ID]domain.Rights{spaceS: {Level: domain.LevelTrigger, Caps: domain.CapsOf(domain.CapUpload)}}}
	if r := domain.Effective(writer, domain.KindUser, samID, path); r.Level != domain.LevelRead || r.Caps != domain.CapsOf(domain.CapUpload) {
		t.Fatalf("trigger in an archived space: %+v", r)
	}
	if r := domain.Effective(domain.Snapshot{}, domain.KindUser, samID, path); r.Level != domain.LevelNone {
		t.Fatalf("archiving opened a space: %+v", r)
	}
	path.Archived = false
	if r := domain.Effective(writer, domain.KindUser, samID, path); r.Level != domain.LevelTrigger {
		t.Fatalf("a space that is not archived: %+v", r)
	}
}

// TestCheckGroupText: the rules of NewGroup for a rename (group.update).
func TestCheckGroupText(t *testing.T) {
	t.Parallel()
	limits := domain.GroupLimits{NameMaxChars: 5, DescriptionMaxChars: 5}
	if err := domain.CheckGroupText("Team", "", limits); err != nil {
		t.Fatal(err)
	}
	for _, c := range [][2]string{{" ", ""}, {"Team\u202e", ""}, {"Teams!", ""}, {"Team", "toolong"}} {
		if err := domain.CheckGroupText(c[0], c[1], limits); err == nil {
			t.Errorf("CheckGroupText(%q, %q) passed", c[0], c[1])
		}
	}
}
