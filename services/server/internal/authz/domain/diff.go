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
	// AccessKindHost is a host with the capability terminal and no level (M2-39).
	AccessKindHost = "host"
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
	// Hosts are the hosts with the effective right terminal (TerminalEffective, M2-39 and M2-40).
	Hosts map[ids.ID]bool
}

// AccessViewOf returns the view of the principal with snapshot s and kind over channels (the
// paths of every channel that is not a dm channel), spaces (the paths of every space, with
// Archived set for an archived one) and hosts (every host). A channel is visible with level read
// or more; a space with read on itself or a visible channel in it (07 section 7.2); a host of a
// person with TerminalOn: every host of hosts for an effective owner, the hosts of the grants
// otherwise (M2-39). dm channels are not part of the view: rights changes never change dm
// participation. Agents and apps hold no host.
func AccessViewOf(s Snapshot, kind string, principal ids.ID, channels []Path, spaces []Path, hosts []ids.ID) AccessView {
	v := AccessView{Spaces: map[ids.ID]bool{}, SpaceLevels: map[ids.ID]Level{}, Channels: map[ids.ID]Rights{}, Hosts: hostsOf(s, kind, hosts)}
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

// hostsOf returns the hosts of a person with the effective right terminal (TerminalEffective, the
// I6 clause of 07 section 2.12 point 3 included, M2-40): every host of hosts for an effective
// owner except the personal hosts of another holder, the hosts of the grants otherwise; agents and
// apps hold none (M2-39).
func hostsOf(s Snapshot, kind string, hosts []ids.ID) map[ids.ID]bool {
	out := map[ids.ID]bool{}
	if kind != KindUser {
		return out
	}
	if s.Owner {
		for _, h := range hosts {
			if TerminalEffective(s, h) {
				out[h] = true
			}
		}
	}
	for h := range s.Hosts {
		if TerminalEffective(s, h) {
			out[h] = true
		}
	}
	return out
}

// AccessEntry is one entry of a difference (OpenAPI AccessTarget): Level and Caps are the new
// rights of a channel in Added and Changed, PreviousLevel the old level in Changed; a host in
// Added carries Caps {terminal} and no level; spaces and removed entries carry only Kind and ID.
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
// space itself differs in both views (archiving a space, M1-52); hosts only appear or disappear (a
// host has one capability and no level, M2-39); channels appear with level and capabilities,
// disappear with kind and id, and change when level or capabilities differ, with the new level,
// the previous level and the new capabilities.
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
	diffSet(&d, AccessKindHost, before.Hosts, after.Hosts, CapsOf(CapTerminal))
	for _, l := range []*[]AccessEntry{&d.Added, &d.Removed, &d.Changed} {
		slices.SortFunc(*l, func(a, b AccessEntry) int { return cmp.Or(cmp.Compare(a.Kind, b.Kind), compareIDs(a.ID, b.ID)) })
	}
	return d
}

// diffSpaces adds the space entries of the difference from before to after to d: added and
// removed spaces with kind and id, and changed spaces whose own level differs in both views.
func diffSpaces(before, after AccessView, d *PrincipalDiff) {
	diffSet(d, AccessKindSpace, before.Spaces, after.Spaces, 0)
	for id, l := range after.SpaceLevels {
		if old, ok := before.SpaceLevels[id]; ok && old != l {
			d.Changed = append(d.Changed, AccessEntry{Kind: AccessKindSpace, ID: id, Level: l, PreviousLevel: old})
		}
	}
}

// diffSet adds the targets of kind that only appear (with caps) or disappear (kind and id only)
// between the sets before and after; a host never changes, the level change of a space is added by
// diffSpaces.
func diffSet(d *PrincipalDiff, kind string, before, after map[ids.ID]bool, caps Caps) {
	for id := range after {
		if !before[id] {
			d.Added = append(d.Added, AccessEntry{Kind: kind, ID: id, Caps: caps})
		}
	}
	for id := range before {
		if !after[id] {
			d.Removed = append(d.Removed, AccessEntry{Kind: kind, ID: id})
		}
	}
}
