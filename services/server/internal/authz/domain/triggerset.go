// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"slices"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// PrincipalState is what TriggerSet needs to know of a principal: its kind and whether its
// status is active.
type PrincipalState struct {
	// Kind is KindUser, KindAgent or KindApp.
	Kind string
	// Active reports principals.status = 'active'.
	Active bool
}

// PersonalAgent is an active agent with billing_mode = 'personal' and its credential holder.
type PersonalAgent struct {
	AgentID            ids.ID
	CredentialHolderID ids.ID
}

// ServedTarget is a target an agent serves (10 section 5.2): a channel with its space, or a
// space alone (ChannelID zero) whose default operator is the agent. The application layer reads
// the list from StructureSource.ServedTargets; authz never computes it.
type ServedTarget struct {
	ChannelID ids.ID
	SpaceID   ids.ID
}

// World holds the facts of the I1 check (03 section 6.4, 10 section 5.3) at one time, read inside
// the transaction of the rights change.
type World struct {
	// Principals are the states of the principals the grants and memberships name; a principal
	// missing here counts as not active.
	Principals map[ids.ID]PrincipalState
	// Owners are the active owners (owners.revoked_at IS NULL, principal active).
	Owners []ids.ID
	// Grants are grants that are not revoked; TriggerSet looks only at level trigger.
	Grants []Grant
	// Memberships are the memberships that are not removed of the groups that hold grants, each
	// with the row of its group.
	Memberships []MembershipWithGroup
	// Served are the served targets per agent.
	Served map[ids.ID][]ServedTarget
}

// TriggerSet returns the principals with effective trigger on any target agent serves at t
// (07 SR1, 03 section 6.4, 10 section 5.3), ascending and without duplicates: the active owners;
// active people with a grant of level trigger in effect at t on the instance, on a served space
// (the space of a served channel, or a space the agent serves as default operator) or on a served
// channel; and active members, with a membership in effect at t in a group that is not deleted,
// of groups holding such a grant. Validity follows 07 section 2.5: at expires_at = t a grant or
// membership is already invalid. Agents and apps never appear, also not through a grant row
// whose subject kind does not match the real kind (R-DM-08).
//
// Example:
//
//	set := domain.TriggerSet(w, domain.PersonalAgent{AgentID: a, CredentialHolderID: owner}, now)
//	ok := slices.Equal(set, []ids.ID{owner}) // I1 holds for a
func TriggerSet(w World, agent PersonalAgent, t time.Time) []ids.ID {
	spaces, channels := servedSets(w.Served[agent.AgentID])
	set := map[ids.ID]bool{}
	for _, o := range w.Owners {
		set[o] = true
	}
	people, groups := triggerSubjects(w.Grants, spaces, channels, t)
	for _, p := range people {
		if w.person(p) {
			set[p] = true
		}
	}
	for _, m := range w.Memberships {
		if groups[m.Membership.GroupID] && m.Group.ID == m.Membership.GroupID &&
			m.Membership.InEffect(m.Group, t) && w.person(m.Membership.UserID) {
			set[m.Membership.UserID] = true
		}
	}
	out := make([]ids.ID, 0, len(set))
	for id := range set {
		out = append(out, id)
	}
	return slices.Clip(SortIDs(out))
}

// person reports an active principal of kind user.
func (w World) person(id ids.ID) bool {
	st, ok := w.Principals[id]
	return ok && st.Active && st.Kind == KindUser
}

// servedSets returns the served spaces and channels of a target list.
func servedSets(served []ServedTarget) (spaces, channels map[ids.ID]bool) {
	spaces, channels = map[ids.ID]bool{}, map[ids.ID]bool{}
	for _, s := range served {
		if !s.SpaceID.IsZero() {
			spaces[s.SpaceID] = true
		}
		if !s.ChannelID.IsZero() {
			channels[s.ChannelID] = true
		}
	}
	return spaces, channels
}

// triggerSubjects returns the person subjects and the groups of the grants with level trigger in
// effect at t on a served target. Bots never hold trigger (R-DM-08).
func triggerSubjects(grants []Grant, spaces, channels map[ids.ID]bool, t time.Time) ([]ids.ID, map[ids.ID]bool) {
	var people []ids.ID
	groups := map[ids.ID]bool{}
	for _, g := range grants {
		if g.Level < LevelTrigger || !g.InEffect(t) || !onServed(g.Target, spaces, channels) {
			continue
		}
		switch g.Subject.Kind {
		case SubjectUser:
			people = append(people, g.Subject.PrincipalID)
		case SubjectGroup:
			groups[g.Subject.GroupID] = true
		case SubjectAgent, SubjectApp:
		}
	}
	return people, groups
}

// onServed reports whether a grant target lies on the served targets: the instance, a served
// space or a served channel.
func onServed(tg Target, spaces, channels map[ids.ID]bool) bool {
	switch tg.Kind {
	case TargetInstance:
		return true
	case TargetSpace:
		return spaces[tg.SpaceID]
	case TargetChannel:
		return channels[tg.ChannelID]
	case TargetHost:
		return false
	}
	return false
}
