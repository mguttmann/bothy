// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"slices"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// The principal kinds as authz sees them (07 section 3; kernel/actor.Kind as text).
const (
	KindUser      = "user"
	KindAgent     = "agent"
	KindApp       = "app"
	KindAnonymous = "anonymous"
)

// PathKind says how deep a resolved path reaches.
type PathKind uint8

const (
	// PathInstance is the instance alone (resources outside spaces and channels).
	PathInstance PathKind = iota
	// PathSpace is a space below the instance.
	PathSpace
	// PathChannel is a channel, and optionally a thread in it.
	PathChannel
	// PathNone has no place in the tree (a host, 07 section 2.12): no level and no capability.
	PathNone
)

// Path is the resolved place of a resource (05 section 2.2), read fresh from the structure per
// call.
type Path struct {
	// Kind is the depth.
	Kind PathKind
	// SpaceID is the space; zero for a dm channel and above spaces.
	SpaceID ids.ID
	// ChannelID is the channel.
	ChannelID ids.ID
	// DM marks a dm channel; DMParticipants are its current participants.
	DM             bool
	DMParticipants []ids.ID
	// ReadOnly is channels.read_only.
	ReadOnly bool
	// ChannelOperatorID is the channel's effective operator; zero for none.
	ChannelOperatorID ids.ID
	// ThreadID is the thread; zero when the resource is not in a thread.
	ThreadID ids.ID
	// ThreadMain marks the main thread of the channel.
	ThreadMain bool
	// ThreadOperatorID is the fixed operator of a non-main thread; zero for none.
	ThreadOperatorID ids.ID
	// ThreadCreatedBy is the creator of the thread.
	ThreadCreatedBy ids.ID
}

// Participant reports whether p takes part in the dm channel of the path.
func (p Path) Participant(principal ids.ID) bool {
	return p.DM && slices.Contains(p.DMParticipants, principal)
}

// ThreadOperator returns the agent that operates the thread of the path: the fixed operator of a
// non-main thread, otherwise the channel's effective operator.
func (p Path) ThreadOperator() ids.ID {
	if !p.ThreadID.IsZero() && !p.ThreadMain && !p.ThreadOperatorID.IsZero() {
		return p.ThreadOperatorID
	}
	return p.ChannelOperatorID
}

// botCaps are the only capabilities agents and apps can hold (R-DM-20).
var botCaps = CapsOf(CapUpload)

// Effective returns the rights of the principal with snapshot s and kind on path (brief M1-28,
// "Effective rights on a path"; 05 section 2.3):
//
//  1. dm channel: a participant has write and upload, anyone else nothing, owners too (SR6);
//  2. owner: trigger and every capability;
//  3. otherwise the highest level of the instance, space and channel grants and the union of
//     their capabilities;
//  4. agents: at least write as the channel's effective operator or as the fixed operator of a
//     non-main thread (07 section 2.9);
//  5. agents and apps: at most write and upload (07 section 2.1), which also neutralises a grant
//     row whose subject kind does not match the real kind.
//
// A path of kind PathNone keeps only the owner role.
func Effective(s Snapshot, kind string, principal ids.ID, path Path) Rights {
	if path.Kind == PathNone {
		return Rights{Owner: s.Owner}
	}
	if path.DM {
		if path.Participant(principal) {
			return Rights{Level: LevelWrite, Caps: botCaps}
		}
		return Rights{}
	}
	if s.Owner {
		return Rights{Level: LevelTrigger, Caps: allCaps, Owner: true}
	}
	r := s.Instance
	if path.Kind >= PathSpace && !path.SpaceID.IsZero() {
		r = r.union(s.Spaces[path.SpaceID])
	}
	if path.Kind == PathChannel {
		r = r.union(s.Channels[path.ChannelID])
	}
	r.Owner = false
	if kind == KindAgent && path.Kind == PathChannel && path.ThreadOperator() == principal && !principal.IsZero() {
		r.Level = MaxLevel(r.Level, LevelWrite)
	}
	if kind == KindAgent && path.Kind == PathChannel && path.ChannelOperatorID == principal && !principal.IsZero() {
		r.Level = MaxLevel(r.Level, LevelWrite)
	}
	if kind != KindUser {
		r.Level = MinLevel(r.Level, LevelWrite)
		r.Caps = r.Caps.Intersect(botCaps)
	}
	return r
}
