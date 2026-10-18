// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"cmp"
	"slices"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// The kinds of AccessEntry (OpenAPI AccessTargetKind).
const (
	AccessKindSpace   = "space"
	AccessKindChannel = "channel"
)

// AccessView is what one principal sees at one moment, the input of access.changed (04 section
// 8.2): the visible spaces and the visible channels with their effective rights. It is built from
// the same snapshot and Effective function as authorize (AccessViewOf), never from the cache.
type AccessView struct {
	// Spaces are the visible spaces: read on the space or on one of its channels.
	Spaces map[ids.ID]bool
	// SpaceLevels are the effective levels on the spaces with at least read on the space itself;
	// a space visible only through a channel has no entry (M1-52: archiving a space changes them).
	SpaceLevels map[ids.ID]Level
	// Channels are the channels with at least read, with level and capabilities (Owner unused).
	Channels map[ids.ID]Rights
}

// AccessViewOf returns the view of the principal with snapshot s and kind over channels (the
// paths of every channel that is not a dm channel) and spaces (the paths of every space, with
// Archived set for an archived one). A channel is visible with level read or more; a space with
// read on itself or a visible channel in it (07 section 7.2). dm channels are not part of the
// view: rights changes never change dm participation.
func AccessViewOf(s Snapshot, kind string, principal ids.ID, channels []Path, spaces []Path) AccessView {
	v := AccessView{Spaces: map[ids.ID]bool{}, SpaceLevels: map[ids.ID]Level{}, Channels: map[ids.ID]Rights{}}
	for _, p := range channels {
		if p.DM || p.Kind != PathChannel {
			continue
		}
		r := Effective(s, kind, principal, p)
		if r.Level < LevelRead {
			continue
		}
		v.Channels[p.ChannelID] = Rights{Level: r.Level, Caps: r.Caps}
		if !p.SpaceID.IsZero() {
			v.Spaces[p.SpaceID] = true
		}
	}
	for _, p := range spaces {
		if p.Kind != PathSpace || p.SpaceID.IsZero() {
			continue
		}
		if l := Effective(s, kind, principal, p).Level; l >= LevelRead {
			v.Spaces[p.SpaceID] = true
			v.SpaceLevels[p.SpaceID] = l
		}
	}
	return v
}

// AccessEntry is one entry of a difference (OpenAPI AccessTarget): Level and Caps are the new
// rights of a channel in Added and Changed, PreviousLevel the old level in Changed; spaces and
// removed entries carry only Kind and ID.
type AccessEntry struct {
	Kind          string
	ID            ids.ID
	Level         Level
	PreviousLevel Level
	Caps          Caps
}

// PrincipalDiff is the difference of two views of one principal; every list is sorted by
// (Kind, ID).
type PrincipalDiff struct {
	Added, Removed, Changed []AccessEntry
}

// Empty reports a difference without entries: no access.changed for this principal.
func (d PrincipalDiff) Empty() bool {
	return len(d.Added) == 0 && len(d.Removed) == 0 && len(d.Changed) == 0
}

// DiffVisible returns the difference from before to after (04 section 8.2): spaces appear and
// disappear with kind and id, and change with the new and the previous level when the level on the
// space itself differs in both views (archiving a space, M1-52); channels appear with level and
// capabilities, disappear with kind and id, and change when level or capabilities differ, with
// the new level, the previous level and the new capabilities.
//
// Example:
//
//	d := domain.DiffVisible(before, after)
//	if !d.Empty() { /* append access.changed */ }
func DiffVisible(before, after AccessView) PrincipalDiff {
	var d PrincipalDiff
	for id, r := range after.Channels {
		old, ok := before.Channels[id]
		switch {
		case !ok:
			d.Added = append(d.Added, AccessEntry{Kind: AccessKindChannel, ID: id, Level: r.Level, Caps: r.Caps})
		case old.Level != r.Level || old.Caps != r.Caps:
			d.Changed = append(d.Changed, AccessEntry{Kind: AccessKindChannel, ID: id, Level: r.Level, PreviousLevel: old.Level, Caps: r.Caps})
		}
	}
	for id := range before.Channels {
		if _, ok := after.Channels[id]; !ok {
			d.Removed = append(d.Removed, AccessEntry{Kind: AccessKindChannel, ID: id})
		}
	}
	diffSpaces(before, after, &d)
	for _, l := range []*[]AccessEntry{&d.Added, &d.Removed, &d.Changed} {
		slices.SortFunc(*l, func(a, b AccessEntry) int { return cmp.Or(cmp.Compare(a.Kind, b.Kind), compareIDs(a.ID, b.ID)) })
	}
	return d
}

// diffSpaces adds the space entries of the difference from before to after to d: added and
// removed spaces with kind and id, and changed spaces whose own level differs in both views.
func diffSpaces(before, after AccessView, d *PrincipalDiff) {
	for id := range after.Spaces {
		if !before.Spaces[id] {
			d.Added = append(d.Added, AccessEntry{Kind: AccessKindSpace, ID: id})
		}
	}
	for id := range before.Spaces {
		if !after.Spaces[id] {
			d.Removed = append(d.Removed, AccessEntry{Kind: AccessKindSpace, ID: id})
		}
	}
	for id, l := range after.SpaceLevels {
		if old, ok := before.SpaceLevels[id]; ok && old != l {
			d.Changed = append(d.Changed, AccessEntry{Kind: AccessKindSpace, ID: id, Level: l, PreviousLevel: old})
		}
	}
}
