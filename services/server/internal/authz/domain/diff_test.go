// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"reflect"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

func view(spaces []ids.ID, chans map[ids.ID]domain.Rights) domain.AccessView {
	v := domain.AccessView{Spaces: map[ids.ID]bool{}, SpaceLevels: map[ids.ID]domain.Level{}, Channels: map[ids.ID]domain.Rights{}, Hosts: map[ids.ID]bool{}}
	for _, s := range spaces {
		v.Spaces[s] = true
	}
	for id, r := range chans {
		v.Channels[id] = r
	}
	return v
}

// leveled sets the levels on the spaces themselves (pairs of space and level) in v.
func leveled(v domain.AccessView, pairs ...any) domain.AccessView {
	for i := 0; i+1 < len(pairs); i += 2 {
		v.SpaceLevels[pairs[i].(ids.ID)] = pairs[i+1].(domain.Level)
	}
	return v
}

// TestDiffVisible_Table: brief M1-29 test 3.
func TestDiffVisible_Table(t *testing.T) {
	t.Parallel()
	sp1 := ids.MustParse("019a0000-0000-7000-8000-000000000101")
	sp2 := ids.MustParse("019a0000-0000-7000-8000-000000000102")
	c1 := ids.MustParse("019a0000-0000-7000-8000-000000000201")
	c2 := ids.MustParse("019a0000-0000-7000-8000-000000000202")
	c3 := ids.MustParse("019a0000-0000-7000-8000-000000000203")
	read := domain.Rights{Level: domain.LevelRead}
	write := domain.Rights{Level: domain.LevelWrite, Caps: domain.CapsOf(domain.CapUpload)}
	trig := domain.Rights{Level: domain.LevelTrigger, Caps: domain.CapsOf(domain.CapUpload, domain.CapApproveActions)}
	ch := func(id ids.ID) domain.AccessEntry { return domain.AccessEntry{Kind: domain.AccessKindChannel, ID: id} }
	sp := func(id ids.ID) domain.AccessEntry { return domain.AccessEntry{Kind: domain.AccessKindSpace, ID: id} }
	cases := []struct {
		name          string
		before, after domain.AccessView
		want          domain.PrincipalDiff
	}{
		{"added", view(nil, nil), view([]ids.ID{sp1}, map[ids.ID]domain.Rights{c1: write}),
			domain.PrincipalDiff{Added: []domain.AccessEntry{{Kind: domain.AccessKindChannel, ID: c1, Level: domain.LevelWrite, Caps: write.Caps}, sp(sp1)}}},
		{"removed", view([]ids.ID{sp1}, map[ids.ID]domain.Rights{c1: trig, c2: read}), view(nil, nil),
			domain.PrincipalDiff{Removed: []domain.AccessEntry{ch(c1), ch(c2), sp(sp1)}}},
		{"changed by level", view([]ids.ID{sp1}, map[ids.ID]domain.Rights{c1: trig}), view([]ids.ID{sp1}, map[ids.ID]domain.Rights{c1: {Level: domain.LevelRead, Caps: trig.Caps}}),
			domain.PrincipalDiff{Changed: []domain.AccessEntry{{Kind: domain.AccessKindChannel, ID: c1, Level: domain.LevelRead, PreviousLevel: domain.LevelTrigger, Caps: trig.Caps}}}},
		{"changed by capabilities only", view(nil, map[ids.ID]domain.Rights{c1: write}), view(nil, map[ids.ID]domain.Rights{c1: {Level: domain.LevelWrite}}),
			domain.PrincipalDiff{Changed: []domain.AccessEntry{{Kind: domain.AccessKindChannel, ID: c1, Level: domain.LevelWrite, PreviousLevel: domain.LevelWrite}}}},
		{"space visibility only", view([]ids.ID{sp1}, nil), view([]ids.ID{sp2}, nil),
			domain.PrincipalDiff{Added: []domain.AccessEntry{sp(sp2)}, Removed: []domain.AccessEntry{sp(sp1)}}},
		{"sorted output", view(nil, nil), view([]ids.ID{sp2, sp1}, map[ids.ID]domain.Rights{c3: read, c1: read, c2: read}),
			domain.PrincipalDiff{Added: []domain.AccessEntry{
				{Kind: domain.AccessKindChannel, ID: c1, Level: domain.LevelRead}, {Kind: domain.AccessKindChannel, ID: c2, Level: domain.LevelRead},
				{Kind: domain.AccessKindChannel, ID: c3, Level: domain.LevelRead}, sp(sp1), sp(sp2)}}},
		{"space level changed", leveled(view([]ids.ID{sp1, sp2}, nil), sp1, domain.LevelTrigger, sp2, domain.LevelRead),
			leveled(view([]ids.ID{sp1, sp2}, nil), sp1, domain.LevelRead, sp2, domain.LevelRead),
			domain.PrincipalDiff{Changed: []domain.AccessEntry{{Kind: domain.AccessKindSpace, ID: sp1, Level: domain.LevelRead, PreviousLevel: domain.LevelTrigger}}}},
		{"empty difference", view([]ids.ID{sp1}, map[ids.ID]domain.Rights{c1: write}), view([]ids.ID{sp1}, map[ids.ID]domain.Rights{c1: write}), domain.PrincipalDiff{}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got := domain.DiffVisible(c.before, c.after)
			if !reflect.DeepEqual(got, c.want) {
				t.Fatalf("DiffVisible =\n%+v\nwant\n%+v", got, c.want)
			}
			if got.Empty() != (len(c.want.Added)+len(c.want.Removed)+len(c.want.Changed) == 0) {
				t.Fatal("Empty")
			}
		})
	}
}

// TestAccessViewOf: the visible channels with level and capabilities, and the spaces visible by a
// grant on them or through a visible channel; a dm path is never part of the view.
func TestAccessViewOf(t *testing.T) {
	t.Parallel()
	sp1, sp2 := ids.New(), ids.New()
	c1, c2 := ids.New(), ids.New()
	snap := domain.BuildSnapshot(domain.SnapshotInput{PrincipalID: alexID, At: t0, Active: true, Grants: []domain.Grant{
		grantOn(t, domain.Subject{Kind: domain.SubjectUser, PrincipalID: alexID}, domain.Target{Kind: domain.TargetChannel, ChannelID: c1}, domain.LevelWrite, domain.CapsOf(domain.CapUpload), t0.Add(1)),
	}})
	paths := []domain.Path{
		{Kind: domain.PathChannel, SpaceID: sp1, ChannelID: c1},
		{Kind: domain.PathChannel, SpaceID: sp2, ChannelID: c2},
	}
	v := domain.AccessViewOf(snap, domain.KindUser, alexID, paths, []domain.Path{{Kind: domain.PathSpace, SpaceID: sp1}, {Kind: domain.PathSpace, SpaceID: sp2}}, nil)
	if !reflect.DeepEqual(v, view([]ids.ID{sp1}, map[ids.ID]domain.Rights{c1: {Level: domain.LevelWrite, Caps: domain.CapsOf(domain.CapUpload)}})) {
		t.Fatalf("view %+v", v)
	}
}

// TestAccessViewOf_SpaceLevels: a grant on a space gives its level in SpaceLevels, capped at read
// in an archived space (07 section 4.5 point 3, M1-52); a space seen only through a channel has
// no level there.
func TestAccessViewOf_SpaceLevels(t *testing.T) {
	t.Parallel()
	sp1, sp2, c1 := ids.New(), ids.New(), ids.New()
	snap := domain.BuildSnapshot(domain.SnapshotInput{PrincipalID: alexID, At: t0, Active: true, Grants: []domain.Grant{
		grantOn(t, domain.Subject{Kind: domain.SubjectUser, PrincipalID: alexID}, domain.Target{Kind: domain.TargetSpace, SpaceID: sp2}, domain.LevelTrigger, 0, t0.Add(1)),
		grantOn(t, domain.Subject{Kind: domain.SubjectUser, PrincipalID: alexID}, domain.Target{Kind: domain.TargetChannel, ChannelID: c1}, domain.LevelRead, 0, t0.Add(1)),
	}})
	paths := []domain.Path{{Kind: domain.PathChannel, SpaceID: sp1, ChannelID: c1}}
	spaces := []domain.Path{{Kind: domain.PathSpace, SpaceID: sp1}, {Kind: domain.PathSpace, SpaceID: sp2}}
	v := domain.AccessViewOf(snap, domain.KindUser, alexID, paths, spaces, nil)
	if !reflect.DeepEqual(v.SpaceLevels, map[ids.ID]domain.Level{sp2: domain.LevelTrigger}) || !v.Spaces[sp1] || !v.Spaces[sp2] {
		t.Fatalf("view %+v", v)
	}
	spaces[1].Archived = true
	v = domain.AccessViewOf(snap, domain.KindUser, alexID, paths, spaces, nil)
	if !reflect.DeepEqual(v.SpaceLevels, map[ids.ID]domain.Level{sp2: domain.LevelRead}) {
		t.Fatalf("archived view %+v", v)
	}
}
