// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// card is the descriptor of interaction.create on an active component of a card of agent in the
// main thread of channel (brief M2-15 design note 8 b): no run, no capability, open to anyone.
func (w *world) card(channel, agent ids.ID) authorize.Resource {
	return authorize.Resource{Kind: authorize.ResourceComponent, ID: ids.New(), ThreadID: w.mainThread[channel], OwnerID: agent,
		Invoke: &authorize.InvokeFacts{AgentID: agent, TriggerOrigin: "human"},
		Interaction: &authorize.InteractionFacts{Stage: authorize.InteractionStageAccept, OwnerKind: "agent", ComponentState: "active",
			ExpiresAt: w.clk.Now().Add(time.Hour), RestrictedTo: "any"}}
}

// approvalCard is card bound to an active run that starter started, with required_capability
// approve_actions and restricted_to restricted (09 section 11.5).
func (w *world) approvalCard(channel, agent, starter ids.ID, restricted string) authorize.Resource {
	r := w.card(channel, agent)
	r.Interaction.RequiredCapability, r.Interaction.RestrictedTo = "approve_actions", restricted
	r.Interaction.ComponentRunID, r.Interaction.ComponentRunActive, r.Interaction.RunStarterID = ids.New(), true, starter
	r.Invoke.ActiveRunAgentID = agent
	return r
}

// TestAZ_M42_Decision (AZ-M42): Alex, who holds approve_actions on Support, on a card of Lotse with
// restricted_to run_starter whose run the Owner started gets forbidden component_restricted; the
// Owner passes.
func TestAZ_M42_Decision(t *testing.T) {
	w := newWorld(t)
	r := w.approvalCard(w.allgemein, w.lotse, w.owner, "run_starter")
	w.expect(t, w.user(w.alex), authorize.ActionInteractionCreate, r, forbidden, "component_restricted")
	w.allow(t, w.user(w.owner), authorize.ActionInteractionCreate, r)
}

// TestAZ_M43_Decision (AZ-M43): Sam on a card of Texter in Marketing with required_capability
// approve_actions, without approve_actions, gets forbidden capability_missing, also as the starter
// of the run.
func TestAZ_M43_Decision(t *testing.T) {
	w := newWorld(t)
	w.expect(t, w.user(w.sam), authorize.ActionInteractionCreate, w.approvalCard(w.kampagnen, w.texter, w.sam, "run_starter"),
		forbidden, "capability_missing")
}

// TestInteractionCreate_World: interaction.create in the world of 07 section 10. A component in a
// thread resolves on the channel path: Robin without a right on Support gets not_found, Sam who
// only reads Allgemein forbidden level_insufficient, Alex with trigger is allowed; the guard of
// agent.invoke applies (I2 on the personal Assistent) and an archived channel refuses first.
func TestInteractionCreate_World(t *testing.T) {
	w := newWorld(t)
	w.expect(t, w.user(w.robin), authorize.ActionInteractionCreate, w.card(w.allgemein, w.lotse), notFound, "no_grant")
	w.expect(t, w.user(w.sam), authorize.ActionInteractionCreate, w.card(w.allgemein, w.lotse), forbidden, "level_insufficient")
	w.allow(t, w.user(w.alex), authorize.ActionInteractionCreate, w.card(w.allgemein, w.lotse))
	// An agent never operates a component: Texter does not see Support, and Lotse has no trigger,
	// so the level decides before the guard (as for agent.invoke, SR4).
	w.expect(t, w.agent(w.texter), authorize.ActionInteractionCreate, w.card(w.allgemein, w.lotse), notFound, "no_grant")
	w.expect(t, w.agent(w.lotse), authorize.ActionInteractionCreate, w.card(w.allgemein, w.lotse), forbidden, "level_insufficient")
	// I2: Alex with trigger on Research (an I1 defect) on a card of the personal Assistent.
	w.grant(t, userSubject(w.alex), domain.Target{Kind: domain.TargetChannel, ChannelID: w.notizen}, domain.LevelTrigger, time.Time{})
	w.versions.Bump()
	w.expect(t, w.user(w.alex), authorize.ActionInteractionCreate, w.card(w.notizen, w.assistent), forbidden, "billing_mode_personal_not_holder")
	w.allow(t, w.user(w.owner), authorize.ActionInteractionCreate, w.card(w.notizen, w.assistent))
	// A used component, and a card while the agent's run is active, without a run of its own.
	used := w.card(w.allgemein, w.lotse)
	used.Interaction.ComponentState = "used"
	w.expect(t, w.user(w.alex), authorize.ActionInteractionCreate, used, forbidden, "component_inactive")
	busy := w.card(w.allgemein, w.lotse)
	busy.Invoke.ActiveRunAgentID = w.lotse
	w.expect(t, w.user(w.alex), authorize.ActionInteractionCreate, busy, forbidden, "run_active")
	w.setChannel(t, w.allgemein, func(c *authz.ChannelFacts) { c.Archived = true })
	w.expect(t, w.user(w.alex), authorize.ActionInteractionCreate, w.card(w.allgemein, w.lotse), forbidden, "channel_archived")
}

// TestInteractionCreate_MentionRuleMetWorld: with require_mention and thread_require_mention set on
// Allgemein, a card of its operator Lotse is reachable without a mention, while agent.invoke by a
// message without a mention is not (gather fills the trigger facts for interaction.create).
func TestInteractionCreate_MentionRuleMetWorld(t *testing.T) {
	w := newWorld(t)
	w.setChannel(t, w.allgemein, func(c *authz.ChannelFacts) { c.RequireMention, c.ThreadRequireMention = true, true })
	w.allow(t, w.user(w.alex), authorize.ActionInteractionCreate, w.card(w.allgemein, w.lotse))
	w.expect(t, w.user(w.alex), authorize.ActionAgentInvoke, w.invoke(w.allgemein, w.lotse, false), forbidden, "agent_not_reachable")
}

// TestInteractionCreate_FactsRequired: interaction.create without its trigger or interaction
// facts is an internal error, never an allow.
func TestInteractionCreate_FactsRequired(t *testing.T) {
	w := newWorld(t)
	for name, mod := range map[string]func(*authorize.Resource){
		"no invoke":      func(r *authorize.Resource) { r.Invoke = nil },
		"no interaction": func(r *authorize.Resource) { r.Interaction = nil },
		"app card":       func(r *authorize.Resource) { r.Interaction.OwnerKind = "app" },
	} {
		r := w.card(w.allgemein, w.lotse)
		mod(&r)
		if d, err := w.az.Authorize(context.Background(), w.user(w.alex), authorize.ActionInteractionCreate, r); err == nil || d.Allowed() {
			t.Errorf("%s: got %+v, %v", name, d, err)
		}
	}
}

// TestInteractionCreate_StageDeliverWorld: agent.invoke on the thread of a card with the
// interaction facts of stage deliver (design note 8 c) re-checks the capability and the
// restriction with the rights of the moment and skips the state.
func TestInteractionCreate_StageDeliverWorld(t *testing.T) {
	w := newWorld(t)
	deliver := func() authorize.Resource {
		c := w.approvalCard(w.allgemein, w.lotse, w.alex, "run_starter")
		c.Interaction.Stage, c.Interaction.ComponentState = authorize.InteractionStageDeliver, "used"
		r := w.thread(w.allgemein)
		r.Invoke, r.Interaction = c.Invoke, c.Interaction
		return r
	}
	w.allow(t, w.user(w.alex), authorize.ActionAgentInvoke, deliver())
	// Alex loses approve_actions: G-1 is replaced by trigger with upload alone.
	w.revoke(t, w.g1)
	w.grant(t, userSubject(w.alex), domain.Target{Kind: domain.TargetSpace, SpaceID: w.support}, domain.LevelTrigger, time.Time{}, domain.CapUpload)
	w.versions.Bump()
	w.expect(t, w.user(w.alex), authorize.ActionAgentInvoke, deliver(), forbidden, "capability_missing")
}

// TestComponentAndRespond_SubjectRules (design note 8 e): component.update and
// interaction.respond keep Any(AgentSelf, AppSelf) with OwnerID the card's author on the
// component and interaction paths. The author is allowed; a person gets forbidden
// subject_kind_not_allowed; another agent without read on the path not_found, and with read
// forbidden not_self.
func TestComponentAndRespond_SubjectRules(t *testing.T) {
	w := newWorld(t)
	component := authorize.Resource{Kind: authorize.ResourceComponent, ID: ids.New(), ThreadID: w.mainThread[w.allgemein], OwnerID: w.lotse}
	interaction := authorize.Resource{Kind: authorize.ResourceInteraction, ID: ids.New(), ThreadID: w.mainThread[w.allgemein], OwnerID: w.lotse}
	for _, c := range []struct {
		a authorize.Action
		r authorize.Resource
	}{{authorize.ActionComponentUpdate, component}, {authorize.ActionInteractionRespond, interaction}} {
		w.allow(t, w.agent(w.lotse), c.a, c.r)
		w.expect(t, w.user(w.alex), c.a, c.r, forbidden, "subject_kind_not_allowed")
		w.expect(t, w.user(w.owner), c.a, c.r, forbidden, "subject_kind_not_allowed")
		w.expect(t, w.user(w.robin), c.a, c.r, notFound, "no_grant")
		w.expect(t, w.agent(w.texter), c.a, c.r, notFound, "no_grant")
	}
	w.grant(t, domain.Subject{Kind: domain.SubjectAgent, PrincipalID: w.texter}, domain.Target{Kind: domain.TargetChannel, ChannelID: w.allgemein},
		domain.LevelRead, time.Time{})
	w.versions.Bump()
	w.expect(t, w.agent(w.texter), authorize.ActionComponentUpdate, component, forbidden, "not_self")
	w.expect(t, w.agent(w.texter), authorize.ActionInteractionRespond, interaction, forbidden, "not_self")
}
