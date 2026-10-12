// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"slices"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Rights are effective rights on one target or path: the level, the capabilities and whether the
// owner role takes effect there.
type Rights struct {
	// Level is the effective level.
	Level Level
	// Caps are the effective capabilities.
	Caps Caps
	// Owner reports the owner role on this path (never in a dm channel, SR6).
	Owner bool
}

// Intersect returns the rights both r and o grant: the lower level, the common capabilities, and
// owner only if both are owner (07 section 4.2 step 4, SR9 and "view as").
func (r Rights) Intersect(o Rights) Rights {
	return Rights{Level: MinLevel(r.Level, o.Level), Caps: r.Caps.Intersect(o.Caps), Owner: r.Owner && o.Owner}
}

// union returns the highest level and the union of capabilities (07 section 2.2 and 2.4).
func (r Rights) union(o Rights) Rights {
	return Rights{Level: MaxLevel(r.Level, o.Level), Caps: r.Caps.Union(o.Caps), Owner: r.Owner || o.Owner}
}

// Snapshot is the rights snapshot of one principal at one rights version (07 section 7.2, steps
// 1, 2 and 4): the direct grant maxima per target, the owner role and the time the snapshot stops
// being valid. Everything that depends on the structure (propagation along the path, dm
// participation, the operator rule, the limits of agents and apps) is applied by Effective when a
// path is evaluated, so a channel created after the snapshot is judged correctly (docs/DECISIONS.md
// M1-28 D5).
type Snapshot struct {
	// PrincipalID is the principal.
	PrincipalID ids.ID
	// PermissionVersion is the rights version the snapshot was built at.
	PermissionVersion int64
	// ComputedAt is the time of the build.
	ComputedAt time.Time
	// ValidUntil is the earliest expiry of a grant, membership, session or token used; zero means
	// no expiry. At ValidUntil the snapshot is already invalid.
	ValidUntil time.Time
	// Owner is the effective owner role: owners row, kind user, a session, status active (SR5).
	Owner bool
	// Instance are the rights of grants on the instance.
	Instance Rights
	// Spaces are the rights of grants on spaces.
	Spaces map[ids.ID]Rights
	// Channels are the rights of grants on channels.
	Channels map[ids.ID]Rights
	// Groups are the groups with a membership in effect, ascending.
	Groups []ids.ID
}

// SnapshotInput is what BuildSnapshot needs; the application layer reads it from the store and
// the sources.
type SnapshotInput struct {
	// PrincipalID is the principal.
	PrincipalID ids.ID
	// Version is the rights version.
	Version int64
	// At is the time t of the call (07 section 2.5).
	At time.Time
	// Active reports principal status active; grants of other principals do not act (SR7).
	Active bool
	// Owner is the owner role as decided by the caller (IsOwner, kind user, a session, active).
	Owner bool
	// Grants are the active grants with the principal as subject.
	Grants []Grant
	// Memberships are the active memberships of a person with their groups.
	Memberships []MembershipWithGroup
	// GroupGrants are the active grants of the groups of Memberships.
	GroupGrants []Grant
	// Expiries are further instants that end the validity: session and token expiry.
	Expiries []time.Time
	// Assume names grants and memberships treated as not expired (the view before an expiry
	// batch, M1-30); the zero value assumes nothing. Only authz/app sets it, never authorize.
	Assume AssumeValid
}

// BuildSnapshot builds the snapshot of in: grants in effect at in.At (07 section 2.5), for people
// also those of the groups with a membership in effect, the maximum level and the union of
// capabilities per target. Grants on a host never enter the tree (07 section 2.12). ValidUntil is
// the earliest expiry after in.At of every grant and membership read and of in.Expiries.
func BuildSnapshot(in SnapshotInput) Snapshot {
	s := Snapshot{
		PrincipalID: in.PrincipalID, PermissionVersion: in.Version, ComputedAt: in.At,
		Spaces: map[ids.ID]Rights{}, Channels: map[ids.ID]Rights{},
	}
	expiries := slices.Clone(in.Expiries)
	if !in.Active {
		s.ValidUntil = NextExpiry(in.At, expiries...)
		return s
	}
	s.Owner = in.Owner
	add := func(g Grant) {
		g = in.Assume.Grant(g)
		expiries = append(expiries, g.ExpiresAt)
		if !g.InEffect(in.At) {
			return
		}
		r := Rights{Level: g.Level, Caps: g.Caps}
		switch g.Target.Kind {
		case TargetInstance:
			s.Instance = s.Instance.union(r)
		case TargetSpace:
			s.Spaces[g.Target.SpaceID] = s.Spaces[g.Target.SpaceID].union(r)
		case TargetChannel:
			s.Channels[g.Target.ChannelID] = s.Channels[g.Target.ChannelID].union(r)
		case TargetHost:
			// Outside the tree; M2-39 adds a map for hosts.
		}
	}
	for _, g := range in.Grants {
		if g.Subject.PrincipalID == in.PrincipalID {
			add(g)
		}
	}
	groups, memExpiries := groupsInEffect(in)
	expiries = append(expiries, memExpiries...)
	for _, g := range in.GroupGrants {
		if g.Subject.Kind == SubjectGroup && slices.Contains(groups, g.Subject.GroupID) {
			add(g)
		}
	}
	s.Groups = groups
	s.ValidUntil = NextExpiry(in.At, expiries...)
	return s
}

// groupsInEffect returns the groups of the memberships of in that are in effect at in.At,
// ascending, and the expiries of all memberships read.
func groupsInEffect(in SnapshotInput) ([]ids.ID, []time.Time) {
	var groups []ids.ID
	var expiries []time.Time
	for _, mg := range in.Memberships {
		m := MembershipWithGroup{Membership: in.Assume.Membership(mg.Membership), Group: mg.Group}
		expiries = append(expiries, m.Membership.ExpiresAt)
		ok := m.Membership.UserID == in.PrincipalID && m.Membership.GroupID == m.Group.ID && m.Membership.InEffect(m.Group, in.At)
		if ok && !slices.Contains(groups, m.Group.ID) {
			groups = append(groups, m.Group.ID)
		}
	}
	return SortIDs(groups), expiries
}

// ValidAt reports whether s may be used at t: before ValidUntil (strict, 07 section 2.5).
func (s Snapshot) ValidAt(t time.Time) bool {
	return s.ValidUntil.IsZero() || t.Before(s.ValidUntil)
}

// compareIDs orders ids by their bytes, which is the order of UUID text.
func compareIDs(a, b ids.ID) int {
	return slices.Compare(a[:], b[:])
}

// SortIDs sorts list ascending in place and returns it.
func SortIDs(list []ids.ID) []ids.ID {
	slices.SortFunc(list, compareIDs)
	return list
}
