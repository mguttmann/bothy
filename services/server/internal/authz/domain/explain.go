// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"cmp"
	"slices"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// SourceKind is the kind of one source of access (OpenAPI AccessSource.kind, 07 section 9.1).
type SourceKind string

// The source kinds of 07 section 9.1.
const (
	SourceOwner         SourceKind = "owner"
	SourceDirect        SourceKind = "direct"
	SourceGroup         SourceKind = "group"
	SourceInherited     SourceKind = "inherited"
	SourceDMParticipant SourceKind = "dm_participant"
	SourceOperator      SourceKind = "operator"
	SourceViaAgent      SourceKind = "via_agent"
	SourceGuardBlocked  SourceKind = "guard_blocked"
)

// RuleSR2 is the protection rule of a guard_blocked source for the tool class (07 SR2).
const RuleSR2 = "sr2"

// Source is one link of the provenance of a principal's access (07 section 7.2 "provenance",
// section 9.1). Fields that do not apply to the kind stay zero.
type Source struct {
	Kind SourceKind
	// GrantID is the grant of direct, group and inherited; GroupID the group of a group grant.
	GrantID, GroupID ids.ID
	// FromTargetKind and FromTargetID name the ancestor of inherited (no id for the instance).
	FromTargetKind TargetKind
	FromTargetID   ids.ID
	// AgentID is the agent of operator, via_agent and guard_blocked; Rule the rule of
	// guard_blocked.
	AgentID ids.ID
	Rule    string
	// ExpiresAt is the expiry of the grant; MembershipExpiresAt that of the membership of a group
	// grant; Reason the reason of a direct grant.
	ExpiresAt, MembershipExpiresAt time.Time
	Reason                         string
}

// Provenance returns the sources of the rights the principal of in (kind kind) has on path at
// in.At, the provenance mode of the snapshot builder (07 section 7.2; never cached). It reads the
// same grants and memberships as BuildSnapshot, so the sources explain exactly the rights
// Effective computes: owner for an effective owner; dm_participant in a dm channel; direct or
// group for a grant on the target itself; inherited for a grant on an ancestor of path (the
// instance, or the space of a channel), with the group data of a group grant; operator for the
// channel's effective operator. Grants not in effect at in.At and grants on hosts are no source.
// The sources are ordered: owner, dm_participant, the grant sources by grant id, operator.
//
// Example:
//
//	src := domain.Provenance(in, domain.KindUser, path)
//	// [{Kind: inherited, GrantID: g1, FromTargetKind: space, FromTargetID: support}]
func Provenance(in SnapshotInput, kind string, path Path) []Source {
	if path.DM {
		if path.Participant(in.PrincipalID) {
			return []Source{{Kind: SourceDMParticipant}}
		}
		return nil
	}
	var out []Source
	if in.Owner && kind == KindUser {
		out = append(out, Source{Kind: SourceOwner})
	}
	grants := append(ownGrantSources(in, path), groupGrantSources(in, path)...)
	slices.SortStableFunc(grants, func(a, b Source) int { return compareIDs(a.GrantID, b.GrantID) })
	out = append(out, grants...)
	if kind == KindAgent && path.Kind == PathChannel && !in.PrincipalID.IsZero() && path.ChannelOperatorID == in.PrincipalID {
		out = append(out, Source{Kind: SourceOperator, AgentID: in.PrincipalID})
	}
	return out
}

// ownGrantSources are the sources of the grants with the principal of in as subject.
func ownGrantSources(in SnapshotInput, path Path) []Source {
	var out []Source
	for _, g := range in.Grants {
		if g.Subject.PrincipalID != in.PrincipalID || g.Subject.Kind == SubjectGroup || !g.InEffect(in.At) {
			continue
		}
		if s, ok := grantSource(g, path); ok {
			out = append(out, s)
		}
	}
	return out
}

// groupGrantSources are the sources of the grants of the groups with a membership of the
// principal of in in effect: group on the target itself, inherited with the group data above it.
func groupGrantSources(in SnapshotInput, path Path) []Source {
	memberships := map[ids.ID]time.Time{}
	for _, m := range in.Memberships {
		if m.Membership.UserID == in.PrincipalID && m.Membership.GroupID == m.Group.ID && m.Membership.InEffect(m.Group, in.At) {
			memberships[m.Group.ID] = m.Membership.ExpiresAt
		}
	}
	var out []Source
	for _, g := range in.GroupGrants {
		exp, member := memberships[g.Subject.GroupID]
		if g.Subject.Kind != SubjectGroup || !member || !g.InEffect(in.At) {
			continue
		}
		s, ok := grantSource(g, path)
		if !ok {
			continue
		}
		s.GroupID, s.MembershipExpiresAt = g.Subject.GroupID, exp
		if s.Kind == SourceDirect {
			s.Kind = SourceGroup
		}
		out = append(out, s)
	}
	return out
}

// grantSource is the source of grant g on path: direct on the target itself, inherited from an
// ancestor, none elsewhere.
func grantSource(g Grant, path Path) (Source, bool) {
	s := Source{GrantID: g.ID, ExpiresAt: g.ExpiresAt}
	switch {
	case g.Target.Kind == TargetInstance && path.Kind >= PathSpace && path.Kind <= PathChannel:
		s.Kind, s.FromTargetKind = SourceInherited, TargetInstance
	case g.Target.Kind == TargetSpace && path.Kind == PathSpace && g.Target.SpaceID == path.SpaceID:
		s.Kind, s.Reason = SourceDirect, g.Reason
	case g.Target.Kind == TargetSpace && path.Kind == PathChannel && !path.SpaceID.IsZero() && g.Target.SpaceID == path.SpaceID:
		s.Kind, s.FromTargetKind, s.FromTargetID = SourceInherited, TargetSpace, path.SpaceID
	case g.Target.Kind == TargetChannel && path.Kind == PathChannel && g.Target.ChannelID == path.ChannelID:
		s.Kind, s.Reason = SourceDirect, g.Reason
	default:
		return Source{}, false
	}
	return s, true
}

// HasSource reports whether list holds a source of kind k.
func HasSource(list []Source, k SourceKind) bool {
	return slices.ContainsFunc(list, func(s Source) bool { return s.Kind == k })
}

// ExplainEntry is one principal of an explanation (OpenAPI AccessEntry) before names are added.
type ExplainEntry struct {
	PrincipalID ids.ID
	// Name is the display name the order uses.
	Name string
	// Rights are the effective level and capabilities on the target.
	Rights Rights
	// Inactive marks a principal whose status is not active (SR7).
	Inactive bool
	Sources  []Source
}

// SortEntries orders an explanation: owners first, then by display name, then by id (brief M1-49).
func SortEntries(list []ExplainEntry) {
	slices.SortFunc(list, func(a, b ExplainEntry) int {
		ao, bo := HasSource(a.Sources, SourceOwner), HasSource(b.Sources, SourceOwner)
		if ao != bo {
			if ao {
				return -1
			}
			return 1
		}
		return cmp.Or(cmp.Compare(a.Name, b.Name), compareIDs(a.PrincipalID, b.PrincipalID))
	})
}
