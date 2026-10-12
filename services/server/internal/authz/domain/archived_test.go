// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"testing"

	"github.com/OWNER/bothy/services/server/internal/authz/domain"
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
