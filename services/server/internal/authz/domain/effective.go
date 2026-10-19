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
	// KindHost is the host service of a terminal host, authenticated with its Ed25519 key (07
	// section 3, D-T25); it has no row in principals and an empty snapshot (M2-43).
	KindHost = "host"
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
	// Archived reports an archived space, an archived channel or a channel of an archived space
	// (07 section 4.5).
	Archived bool
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
// non-main thread, zero when that thread was created without one (R-DM-12, 09 section 3.1: a
// later operator of the channel does not take it over), otherwise the channel's effective
// operator (main thread and channel, R-DM-21).
func (p Path) ThreadOperator() ids.ID {
	if !p.ThreadID.IsZero() && !p.ThreadMain {
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
//     row whose subject kind does not match the real kind;
//  6. an archived space or channel: at most read for everyone, owners included (07 section 4.5
//     point 1).
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
		return archivedCap(path, Rights{Level: LevelTrigger, Caps: allCaps, Owner: true})
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
	return archivedCap(path, r)
}

// archivedCap caps the level at read on an archived space or channel path (07 section 4.5 point
// 1); spaces (M1-52) and channels (M1-53) share it.
func archivedCap(path Path, r Rights) Rights {
	if path.Archived && (path.Kind == PathChannel || path.Kind == PathSpace) {
		r.Level = MinLevel(r.Level, LevelRead)
	}
	return r
}

// TerminalOn reports the effective right terminal of the principal with snapshot s on the host
// hostID (07 section 2.12 point 3): an effective owner holds it on every host, anyone else through
// a grant terminal in effect on the host, directly or through a group in effect. It does not
// depend on the state or the holder of the host. The exception of I6 for owners on a personal host
// is decided by the guard SR11 of M2-40 (07 section 4.2 step 9), which comes after the requirement.
func TerminalOn(s Snapshot, hostID ids.ID) bool {
	return s.Owner || s.Hosts[hostID].Has(CapTerminal)
}
