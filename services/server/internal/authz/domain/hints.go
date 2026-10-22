// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"cmp"
	"slices"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// HintKind is the kind of one entry of the list "Permission hints" (07 section 9.5, OpenAPI
// PermissionHintKind). The kinds are listed in the order of HintKinds.
type HintKind string

// The four kinds of 07 section 9.5.
const (
	// HintH1 is hint H1 of 07 section 6: a person of T(A) cannot read a channel of H(A).
	HintH1 HintKind = "h1_read_horizon"
	// HintGrantToDisabled is a grant whose subject principal is disabled or deleted.
	HintGrantToDisabled HintKind = "grant_to_disabled"
	// HintTriggerBlocked is a grant with level trigger that SR2 or SR3 keeps from enabling any
	// triggering.
	HintTriggerBlocked HintKind = "trigger_blocked"
	// HintAgentPersonal is an agent with billing_mode personal and its trigger set (I1).
	HintAgentPersonal HintKind = "agent_personal"
)

// HintKinds are the kinds in the fixed order of the list (brief M6-07 design note 2).
var HintKinds = []HintKind{HintH1, HintGrantToDisabled, HintTriggerBlocked, HintAgentPersonal}

// KindRank returns the position of k in HintKinds, or -1 for an unknown kind.
func KindRank(k HintKind) int { return slices.Index(HintKinds, k) }

// RuleSR3 is the protection rule of dev isolation (07 SR3), next to RuleSR2.
const RuleSR3 = "sr3"

// Hint is one entry of the list "Permission hints". Fields that do not apply to the kind stay
// zero: H1 names AgentID, PersonID, SpaceID and ChannelID; the two grant kinds carry Grant, and
// trigger_blocked the blocking Rules (sr2, sr3, ascending); agent_personal names AgentID,
// CredentialHolderID and the TriggerSet.
type Hint struct {
	Kind                                  HintKind
	AgentID, PersonID, SpaceID, ChannelID ids.ID
	Grant                                 Grant
	Rules                                 []string
	CredentialHolderID                    ids.ID
	TriggerSet                            []ids.ID
}

// HintKey is the stable position of a hint within its kind (brief M6-07 design note 2): the
// agent, person and channel of H1, the grant id of the two grant kinds, the agent id of
// agent_personal. Unused parts are zero.
type HintKey struct {
	A, B, C ids.ID
}

// Key returns the stable key of h within its kind.
func (h Hint) Key() HintKey {
	switch h.Kind {
	case HintH1:
		return HintKey{A: h.AgentID, B: h.PersonID, C: h.ChannelID}
	case HintGrantToDisabled, HintTriggerBlocked:
		return HintKey{A: h.Grant.ID}
	case HintAgentPersonal:
		return HintKey{A: h.AgentID}
	}
	return HintKey{}
}

// CompareKeys orders two keys of the same kind: by A, then B, then C.
func CompareKeys(a, b HintKey) int {
	return cmp.Or(compareIDs(a.A, b.A), compareIDs(a.B, b.B), compareIDs(a.C, b.C))
}

// SortHints orders hints of one kind by their key.
func SortHints(list []Hint) {
	slices.SortFunc(list, func(a, b Hint) int { return CompareKeys(a.Key(), b.Key()) })
}

// ReadHorizonGaps returns the H1 hints of agent (07 section 6): one per person P of triggerers
// and channel K of horizon (the paths of H(A)) where P cannot read K with its snapshot, ordered by
// person and channel. readers holds the snapshot of each person; a person without one has no
// rights and so lacks read everywhere. It is the one H1 function of authz: "who has access"
// (via_agent, M1-49) and the list "Permission hints" (M6-07) both call it, so the two can never
// disagree.
//
// Example:
//
//	gaps := domain.ReadHorizonGaps(texter, []domain.Path{intern}, []ids.ID{sam}, snaps)
//	// [{Kind: h1_read_horizon, AgentID: texter, PersonID: sam, ChannelID: intern.ChannelID, ...}]
func ReadHorizonGaps(agent ids.ID, horizon []Path, triggerers []ids.ID, readers map[ids.ID]Snapshot) []Hint {
	var out []Hint
	for _, p := range SortIDs(slices.Clone(triggerers)) {
		snap := readers[p]
		for _, k := range horizon {
			if Effective(snap, KindUser, p, k).Level < LevelRead {
				out = append(out, Hint{Kind: HintH1, AgentID: agent, PersonID: p, SpaceID: k.SpaceID, ChannelID: k.ChannelID})
			}
		}
	}
	SortHints(out)
	return dedupeKeys(out)
}

// dedupeKeys drops repeated hints with the same key (a channel listed twice in horizon) from a
// sorted list.
func dedupeKeys(list []Hint) []Hint {
	return slices.CompactFunc(list, func(a, b Hint) bool { return CompareKeys(a.Key(), b.Key()) == 0 })
}

// GrantToDisabled reports a grant whose subject is a principal (person, agent or app) that is
// disabled or deleted (07 SR7: its grants take no effect). Group subjects never qualify: a
// deleted group's grants are revoked with it (M1-48).
func GrantToDisabled(g Grant, subject PrincipalState, known bool) bool {
	return g.Subject.Kind != SubjectGroup && known && !subject.Active
}

// TriggerChannel is one channel a trigger grant reaches, with what SR2 and SR3 look at: the
// effective tool class of its operator (EffectiveToolClass), whether it has a project binding
// and whether the subject has a ready dev environment of its own there.
type TriggerChannel struct {
	ChannelID    ids.ID
	ToolClass    string
	ProjectBound bool
	DevReady     bool
}

// TriggerBlockRules decides the hint trigger_blocked for a grant of level trigger (07 section
// 9.5): reach are the channels with an operator that the grant reaches; owner is the effective
// owner role of a person subject (owners pass SR2 and SR3). The grant is blocked when it reaches
// at least one such channel and on every one of them SR2 (tool class host without a ready own dev
// environment) or SR3 (project binding without a ready own dev environment) keeps the subject
// from triggering, the rules of the guard of agent.invoke (devRules). rules are the blocking
// rules, ascending.
func TriggerBlockRules(reach []TriggerChannel, owner bool) (rules []string, blocked bool) {
	if owner || len(reach) == 0 {
		return nil, false
	}
	for _, c := range reach {
		var r []string
		if toolRank(c.ToolClass) >= toolRank(ToolHost) && !c.DevReady {
			r = append(r, RuleSR2)
		}
		if c.ProjectBound && !c.DevReady {
			r = append(r, RuleSR3)
		}
		if len(r) == 0 {
			return nil, false
		}
		rules = append(rules, r...)
	}
	slices.Sort(rules)
	return slices.Compact(rules), true
}

// Reaches reports whether a grant target covers the channel path p (07 section 2.2): the
// instance every channel that is not a dm, a space its channels, a channel itself.
func Reaches(tg Target, p Path) bool {
	if p.DM || p.Kind != PathChannel {
		return false
	}
	switch tg.Kind {
	case TargetInstance:
		return true
	case TargetSpace:
		return tg.SpaceID == p.SpaceID
	case TargetChannel:
		return tg.ChannelID == p.ChannelID
	case TargetHost:
		return false
	}
	return false
}

// ExpiringKind is the kind of one entry of "Expiring soon" (07 section 9.4).
type ExpiringKind string

// The kinds of "Expiring soon", in the order of the tie break.
const (
	ExpiringGrant      ExpiringKind = "grant"
	ExpiringMembership ExpiringKind = "membership"
	ExpiringToken      ExpiringKind = "token"
)

// ExpiringPos is the stable position of an entry of "Expiring soon": its expiry, its kind and
// its id. The list is ordered by expiry, then kind (grant, membership, token), then id.
type ExpiringPos struct {
	ExpiresAt time.Time
	Kind      ExpiringKind
	ID        ids.ID
}

// expiringRank orders the kinds for the tie break.
func expiringRank(k ExpiringKind) int {
	return slices.Index([]ExpiringKind{ExpiringGrant, ExpiringMembership, ExpiringToken}, k)
}

// ComparePos orders two positions of "Expiring soon".
func ComparePos(a, b ExpiringPos) int {
	return cmp.Or(a.ExpiresAt.Compare(b.ExpiresAt), cmp.Compare(expiringRank(a.Kind), expiringRank(b.Kind)), compareIDs(a.ID, b.ID))
}

// ExpiresWithin reports an expiry in (t, until]: still in effect at t (07 section 2.5) and ending
// within the reminder lead.
func ExpiresWithin(exp, t, until time.Time) bool {
	return !exp.IsZero() && exp.After(t) && !exp.After(until)
}
