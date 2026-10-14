// SPDX-License-Identifier: Apache-2.0

package domain

import "github.com/OWNER/bothy/services/server/internal/kernel/ids"

// Effect is one effect of a previewed rights change on one principal and target (OpenAPI
// AccessEffect, 07 section 9.3): the level before and after (LevelNone for no access) and the
// capabilities added.
type Effect struct {
	// Kind is AccessKindSpace or AccessKindChannel; ID the target.
	Kind          string
	ID            ids.ID
	Before, After Level
	CapsAdded     Caps
}

// Effects returns the effects of the change from before to after of one principal, one per entry
// of DiffVisible (the difference of access.changed, 04 section 8.2) in its order: added, removed,
// changed. A channel carries its levels and the capabilities it gains; a space only appears or
// disappears, as read or none (docs/DECISIONS.md M1-49).
//
// Example:
//
//	for _, e := range domain.Effects(before, after) { /* AccessEffect */ }
func Effects(before, after AccessView) []Effect {
	d := DiffVisible(before, after)
	out := make([]Effect, 0, len(d.Added)+len(d.Removed)+len(d.Changed))
	conv := func(e AccessEntry) Effect {
		if e.Kind == AccessKindSpace {
			return Effect{Kind: e.Kind, ID: e.ID, Before: spaceLevel(before, e.ID), After: spaceLevel(after, e.ID)}
		}
		old, now := before.Channels[e.ID], after.Channels[e.ID]
		return Effect{Kind: e.Kind, ID: e.ID, Before: old.Level, After: now.Level, CapsAdded: now.Caps &^ old.Caps}
	}
	for _, l := range [][]AccessEntry{d.Added, d.Removed, d.Changed} {
		for _, e := range l {
			out = append(out, conv(e))
		}
	}
	return out
}

// spaceLevel is read for a visible space of v, none otherwise.
func spaceLevel(v AccessView, id ids.ID) Level {
	if v.Spaces[id] {
		return LevelRead
	}
	return LevelNone
}
