// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"slices"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// EffectiveToolClass returns the higher of the target and the reported tool class of an agent
// (07 section 2.11); an unknown class counts as host (fail closed).
func EffectiveToolClass(target, reported string) string {
	switch max(toolRank(target), toolRank(reported)) {
	case 0:
		return ToolChat
	case 1:
		return ToolScoped
	}
	return ToolHost
}

// SR2Blocks reports that SR2 keeps a person from triggering an agent of the effective tool class
// class: the class is host and the person is no owner (07 SR2; a ready own dev environment, SR3,
// comes with V2).
func SR2Blocks(class string, owner bool) bool {
	return toolRank(class) >= toolRank(ToolHost) && !owner
}

// Horizon returns the read horizon H(A) of agent (07 section 6): the channels among channels
// (non-dm paths) the agent reads with its snapshot s (grants and the operator rule, Effective),
// plus the channels of served (the targets it serves, among them channels with a thread whose
// fixed operator is the agent), ascending and without duplicates.
func Horizon(s Snapshot, agent ids.ID, channels []Path, served []ServedTarget) []ids.ID {
	known := map[ids.ID]bool{}
	for _, p := range channels {
		if !p.DM && p.Kind == PathChannel {
			known[p.ChannelID] = true
		}
	}
	var out []ids.ID
	for _, p := range channels {
		if known[p.ChannelID] && !p.DM && p.Kind == PathChannel && Effective(s, KindAgent, agent, p).Level >= LevelRead {
			out = append(out, p.ChannelID)
		}
	}
	for _, t := range served {
		if !t.ChannelID.IsZero() && known[t.ChannelID] {
			out = append(out, t.ChannelID)
		}
	}
	return slices.Compact(SortIDs(out))
}

// Triggerers returns the trigger set T(A) of agent (07 section 6): the people of TriggerSet over
// w at t (owners and people with effective trigger on a target the agent serves) without those
// SR2 blocks for the agent's effective tool class class, ascending.
func Triggerers(w World, agent ids.ID, class string, t time.Time) []ids.ID {
	owners := map[ids.ID]bool{}
	for _, o := range w.Owners {
		owners[o] = true
	}
	set := TriggerSet(w, PersonalAgent{AgentID: agent}, t)
	return slices.DeleteFunc(set, func(p ids.ID) bool { return SR2Blocks(class, owners[p]) })
}
