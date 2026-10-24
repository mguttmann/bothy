// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"slices"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// starterID is the person who started the run of a card in these tests (Robin).
var starterID = ids.MustParse("019a0000-0000-7000-8000-0000000000b1")

// cardFacts are the facts of an active single-use component of agentID without a run, without a
// capability and open to any person.
func cardFacts(stage string) *domain.InteractionFacts {
	return &domain.InteractionFacts{Stage: stage, OwnerKind: domain.KindAgent, ComponentState: domain.ComponentActive,
		ExpiresAt: t0.Add(time.Hour), RestrictedTo: domain.RestrictedAny}
}

// interactionInput is action on a card of agentID in the main thread of channel chanC, which
// agentID operates, decided for Alex with the rights eff, the trigger facts of a reachable chat
// agent of billing mode organization and the interaction facts f. interaction.create takes the
// component, agent.invoke the thread.
func interactionInput(t *testing.T, action string, eff domain.Rights, f *domain.InteractionFacts) domain.Input {
	t.Helper()
	kind := "component"
	if action == "agent.invoke" {
		kind = "thread"
	}
	in := input(t, action, kind, domain.Principal{ID: alexID, Kind: domain.KindUser, HasSession: true, StepUpAt: t0}, eff)
	in.Path = domain.Path{Kind: domain.PathChannel, SpaceID: spaceS, ChannelID: chanC, ChannelOperatorID: agentID,
		ThreadID: ids.New(), ThreadMain: true}
	in.Resource.OwnerID = agentID
	in.Invoke = &domain.InvokeFacts{AgentID: agentID, TriggerOrigin: domain.OriginHuman, AgentFound: true, AgentKind: domain.KindAgent,
		AgentStatus: "active", PolicyFound: true, BillingMode: domain.BillingOrganization, ToolClass: domain.ToolChat,
		AgentLevel: domain.LevelWrite}
	in.Interaction = f
	return in
}

// trigger are the rights of a person with trigger and no capability on the path.
var trigger = domain.Rights{Level: domain.LevelTrigger}

// denied is a forbidden decision with reason r.
func denied(r domain.Reason) domain.Decision {
	return domain.Decision{Outcome: domain.OutcomeForbidden, Reason: r}
}

// TestInteractionCreate_RequirementNotDeferred (M2-15 design note 8 b): interaction.create needs
// trigger on the path and is no longer deferred; the golden list no longer names it (AZ-P3 is
// TestDeferredActionsGolden).
func TestInteractionCreate_RequirementNotDeferred(t *testing.T) {
	t.Parallel()
	r, ok := domain.RequirementOf("interaction.create")
	if !ok {
		t.Fatal("interaction.create: no entry")
	}
	if _, deferred := r.Expr.DeferredTo(); deferred {
		t.Fatal("interaction.create: still deferred")
	}
	if got, want := r.Expr.String(), domain.AtLeast(domain.LevelTrigger).String(); got != want {
		t.Errorf("interaction.create: %s, want %s", got, want)
	}
	if slices.Contains(domain.DeferredActions(), "interaction.create") {
		t.Error("interaction.create is still in the deferred list")
	}
	// Read alone is refused by the requirement before any guard.
	got, err := domain.Decide(interactionInput(t, "interaction.create", domain.Rights{Level: domain.LevelWrite}, cardFacts(domain.InteractionStageAccept)))
	if err != nil || got != denied(domain.ReasonLevelInsufficient) {
		t.Errorf("write only: %+v, %v", got, err)
	}
}

// TestInteractionCreate_ReasonOrder (M2-15 design note 8 b): with every rule broken the reasons
// come in the order origin (point 1), reachability (points 2 and 3), I2 (point 4), SR2 (point
// 5), the component (point 6: inactive, capability, restriction), the project (point 7) and the
// run (point 8: run_active_other_agent, then run_active). Each step repairs the rule of the
// reason before and expects the next one.
func TestInteractionCreate_ReasonOrder(t *testing.T) {
	t.Parallel()
	f := cardFacts(domain.InteractionStageAccept)
	f.ComponentState = domain.ComponentUsed
	f.RequiredCapability = "approve_actions"
	f.RestrictedTo = domain.RestrictedRunStarter
	in := interactionInput(t, "interaction.create", trigger, f)
	in.Invoke.TriggerOrigin = "agent"
	in.Invoke.AgentStatus = "disabled"
	in.Invoke.BillingMode, in.Invoke.CredentialHolderID = domain.BillingPersonal, samID
	in.Invoke.ToolClass = domain.ToolHost
	in.Invoke.ProjectBound, in.Invoke.ProjectReady = true, false
	in.Invoke.ActiveRunAgentID = ids.New()
	steps := []struct {
		want domain.Reason
		fix  func(*domain.Input)
	}{
		{domain.ReasonOriginNotHuman, func(in *domain.Input) { in.Invoke.TriggerOrigin = domain.OriginHuman }},
		{domain.ReasonAgentNotReachable, func(in *domain.Input) { in.Invoke.AgentStatus = "active" }},
		{domain.ReasonBillingModePersonalNotHolder, func(in *domain.Input) { in.Invoke.BillingMode = domain.BillingOrganization }},
		{domain.ReasonToolClassOwnerOnly, func(in *domain.Input) { in.Invoke.ToolClass = domain.ToolChat }},
		{domain.ReasonComponentInactive, func(in *domain.Input) { in.Interaction.ComponentState = domain.ComponentActive }},
		{domain.ReasonCapabilityMissing, func(in *domain.Input) { in.Eff.Caps = domain.CapsOf(domain.CapApproveActions) }},
		{domain.ReasonComponentRestricted, func(in *domain.Input) { in.Interaction.RestrictedTo = domain.RestrictedAny }},
		{domain.ReasonProjectNotReady, func(in *domain.Input) { in.Invoke.ProjectReady = true; in.Invoke.DevEnvironmentReady = true }},
		{domain.ReasonRunActiveOtherAgent, func(in *domain.Input) { in.Invoke.ActiveRunAgentID = agentID }},
		{domain.ReasonRunActive, func(in *domain.Input) {
			in.Interaction.ComponentRunID, in.Interaction.ComponentRunActive, in.Interaction.RunStarterID = ids.New(), true, starterID
		}},
	}
	for _, s := range steps {
		got, err := domain.Decide(in)
		if err != nil || got != denied(s.want) {
			t.Fatalf("want %s: got %+v, %v", s.want, got, err)
		}
		s.fix(&in)
	}
	if got, err := domain.Decide(in); err != nil || got != domain.Allow {
		t.Fatalf("all rules met: %+v, %v", got, err)
	}
}

// TestInteractionCreate_ComponentInactive: at stage accept a component that is not active, has
// expired (at or before the request time) or belongs to a run that is no longer active gives
// component_inactive; an expiry after the request time and the zero expiry (never) pass.
func TestInteractionCreate_ComponentInactive(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		mod  func(*domain.InteractionFacts)
		want domain.Decision
	}{
		"active":         {func(*domain.InteractionFacts) {}, domain.Allow},
		"no expiry":      {func(f *domain.InteractionFacts) { f.ExpiresAt = time.Time{} }, domain.Allow},
		"used":           {func(f *domain.InteractionFacts) { f.ComponentState = domain.ComponentUsed }, denied(domain.ReasonComponentInactive)},
		"expired state":  {func(f *domain.InteractionFacts) { f.ComponentState = domain.ComponentExpired }, denied(domain.ReasonComponentInactive)},
		"disabled":       {func(f *domain.InteractionFacts) { f.ComponentState = domain.ComponentDisabled }, denied(domain.ReasonComponentInactive)},
		"expires now":    {func(f *domain.InteractionFacts) { f.ExpiresAt = t0 }, denied(domain.ReasonComponentInactive)},
		"expired before": {func(f *domain.InteractionFacts) { f.ExpiresAt = t0.Add(-time.Second) }, denied(domain.ReasonComponentInactive)},
		"run ended": {func(f *domain.InteractionFacts) { f.ComponentRunID, f.RunStarterID = ids.New(), starterID },
			denied(domain.ReasonComponentInactive)},
		"run active": {func(f *domain.InteractionFacts) {
			f.ComponentRunID, f.ComponentRunActive, f.RunStarterID = ids.New(), true, starterID
		}, domain.Allow},
	}
	for name, c := range cases {
		f := cardFacts(domain.InteractionStageAccept)
		c.mod(f)
		got, err := domain.Decide(interactionInput(t, "interaction.create", trigger, f))
		if err != nil || got != c.want {
			t.Errorf("%s: %+v, %v; want %+v", name, got, err, c.want)
		}
	}
}

// TestInteractionCreate_OwnerPassesRestrictions (M2-15 design note 8 b, 07 section 2.4): an
// effective owner holds every capability and passes restricted_to run_starter and owner; a run
// starter passes run_starter but not owner; anyone else passes neither.
func TestInteractionCreate_OwnerPassesRestrictions(t *testing.T) {
	t.Parallel()
	owner := domain.Rights{Level: domain.LevelTrigger, Caps: domain.CapsOf(domain.CapApproveActions, domain.CapUpload, domain.CapExport,
		domain.CapManage, domain.CapTerminal), Owner: true}
	approver := domain.Rights{Level: domain.LevelTrigger, Caps: domain.CapsOf(domain.CapApproveActions)}
	for _, restricted := range []string{domain.RestrictedAny, domain.RestrictedRunStarter, domain.RestrictedOwner} {
		f := cardFacts(domain.InteractionStageAccept)
		f.RequiredCapability, f.RestrictedTo = "approve_actions", restricted
		f.ComponentRunID, f.ComponentRunActive, f.RunStarterID = ids.New(), true, starterID
		if got, err := domain.Decide(interactionInput(t, "interaction.create", owner, f)); err != nil || got != domain.Allow {
			t.Errorf("owner, %s: %+v, %v", restricted, got, err)
		}
		want := domain.Allow
		if restricted != domain.RestrictedAny {
			want = denied(domain.ReasonComponentRestricted)
		}
		if got, err := domain.Decide(interactionInput(t, "interaction.create", approver, f)); err != nil || got != want {
			t.Errorf("approver, %s: %+v, %v; want %+v", restricted, got, err, want)
		}
		f.RunStarterID = alexID
		if restricted == domain.RestrictedOwner {
			want = denied(domain.ReasonComponentRestricted)
		} else {
			want = domain.Allow
		}
		if got, err := domain.Decide(interactionInput(t, "interaction.create", approver, f)); err != nil || got != want {
			t.Errorf("run starter, %s: %+v, %v; want %+v", restricted, got, err, want)
		}
	}
	// restricted_to run_starter on a card without a run has no starter: only owners pass.
	f := cardFacts(domain.InteractionStageAccept)
	f.RestrictedTo = domain.RestrictedRunStarter
	if got, err := domain.Decide(interactionInput(t, "interaction.create", trigger, f)); err != nil || got != denied(domain.ReasonComponentRestricted) {
		t.Errorf("run-less run_starter: %+v, %v", got, err)
	}
}

// TestInteractionCreate_MentionRuleMet (M2-15 design note 8 b, 05 section 5 point 3): a card is the
// agent's own address to the person, so the mention rule counts as met for the operator of the
// thread whatever the mention switches say; agent.invoke without interaction facts still needs the
// mention. A guest agent still needs allow_mentioned_agents, its own write and billing mode
// organization.
func TestInteractionCreate_MentionRuleMet(t *testing.T) {
	t.Parallel()
	switches := func(in *domain.Input) {
		in.Invoke.RequireMention, in.Invoke.ThreadRequireMention = true, true
	}
	in := interactionInput(t, "interaction.create", trigger, cardFacts(domain.InteractionStageAccept))
	switches(&in)
	if got, err := domain.Decide(in); err != nil || got != domain.Allow {
		t.Errorf("operator card in the main thread: %+v, %v", got, err)
	}
	// A non-main thread with thread_require_mention and the agent as its fixed operator.
	in.Path.ThreadMain, in.Path.ThreadOperatorID = false, agentID
	if got, err := domain.Decide(in); err != nil || got != domain.Allow {
		t.Errorf("operator card in a thread: %+v, %v", got, err)
	}
	plain := interactionInput(t, "agent.invoke", trigger, nil)
	switches(&plain)
	plain.Path.ThreadMain, plain.Path.ThreadOperatorID = false, agentID
	if got, err := domain.Decide(plain); err != nil || got != denied(domain.ReasonAgentNotReachable) {
		t.Errorf("agent.invoke without a mention: %+v, %v", got, err)
	}
	// A guest: the channel is operated by another agent.
	guest := interactionInput(t, "interaction.create", trigger, cardFacts(domain.InteractionStageAccept))
	guest.Path.ChannelOperatorID = ids.New()
	if got, err := domain.Decide(guest); err != nil || got != denied(domain.ReasonAgentNotReachable) {
		t.Errorf("guest without allow_mentioned_agents: %+v, %v", got, err)
	}
	guest.Invoke.AllowMentionedAgents = true
	if got, err := domain.Decide(guest); err != nil || got != domain.Allow {
		t.Errorf("allowed guest: %+v, %v", got, err)
	}
	guest.Invoke.AgentLevel = domain.LevelRead
	if got, err := domain.Decide(guest); err != nil || got != denied(domain.ReasonAgentNotReachable) {
		t.Errorf("guest without write: %+v, %v", got, err)
	}
	guest.Invoke.AgentLevel = domain.LevelWrite
	guest.Invoke.BillingMode, guest.Invoke.CredentialHolderID = domain.BillingPersonal, alexID
	if got, err := domain.Decide(guest); err != nil || got != denied(domain.ReasonAgentNotReachable) {
		t.Errorf("personal guest: %+v, %v", got, err)
	}
}

// TestInteractionCreate_StageDeliver (M2-15 design note 8 c): agent.invoke with interaction facts
// of stage deliver skips the state check and the rule run_active, and evaluates the capability
// and the restriction with the rights of the moment; agent.invoke without them is unchanged.
func TestInteractionCreate_StageDeliver(t *testing.T) {
	t.Parallel()
	f := cardFacts(domain.InteractionStageDeliver)
	f.ComponentState, f.ExpiresAt = domain.ComponentUsed, t0.Add(-time.Hour)
	f.ComponentRunID, f.ComponentRunActive, f.RunStarterID = ids.New(), false, alexID
	in := interactionInput(t, "agent.invoke", trigger, f)
	in.Invoke.ActiveRunAgentID = agentID
	if got, err := domain.Decide(in); err != nil || got != domain.Allow {
		t.Errorf("used component, ended run: %+v, %v", got, err)
	}
	// run_active belongs to stage accept only: a run-less card while the agent's run is active.
	f.ComponentRunID, f.RunStarterID = ids.ID{}, ids.ID{}
	if got, err := domain.Decide(in); err != nil || got != domain.Allow {
		t.Errorf("run-less component, active run: %+v, %v", got, err)
	}
	f.RequiredCapability = "approve_actions"
	if got, err := domain.Decide(in); err != nil || got != denied(domain.ReasonCapabilityMissing) {
		t.Errorf("capability lost: %+v, %v", got, err)
	}
	in.Eff.Caps = domain.CapsOf(domain.CapApproveActions)
	f.RestrictedTo = domain.RestrictedOwner
	if got, err := domain.Decide(in); err != nil || got != denied(domain.ReasonComponentRestricted) {
		t.Errorf("owner restriction: %+v, %v", got, err)
	}
	f.RestrictedTo = domain.RestrictedAny
	in.Invoke.ActiveRunAgentID = ids.New()
	if got, err := domain.Decide(in); err != nil || got != denied(domain.ReasonRunActiveOtherAgent) {
		t.Errorf("run of another agent: %+v, %v", got, err)
	}
	// The same component at stage accept is inactive.
	acc := interactionInput(t, "interaction.create", trigger, cardFacts(domain.InteractionStageAccept))
	acc.Interaction.ComponentState = domain.ComponentUsed
	if got, err := domain.Decide(acc); err != nil || got != denied(domain.ReasonComponentInactive) {
		t.Errorf("used component at accept: %+v, %v", got, err)
	}
}

// TestInteractionCreate_MissingFactsFailClosed: interaction.create without its trigger or
// interaction facts, a card of an app without the facts of the app or with agent.invoke, a stage that does not belong to the action,
// and an unknown capability, restriction or state are internal errors, never an allow.
func TestInteractionCreate_MissingFactsFailClosed(t *testing.T) {
	t.Parallel()
	cases := map[string]domain.Input{}
	noInvoke := interactionInput(t, "interaction.create", trigger, cardFacts(domain.InteractionStageAccept))
	noInvoke.Invoke = nil
	cases["no invoke"] = noInvoke
	cases["no interaction"] = interactionInput(t, "interaction.create", trigger, nil)
	mod := func(action string, m func(*domain.InteractionFacts)) domain.Input {
		stage := domain.InteractionStageAccept
		if action == "agent.invoke" {
			stage = domain.InteractionStageDeliver
		}
		f := cardFacts(stage)
		m(f)
		return interactionInput(t, action, trigger, f)
	}
	cases["app card"] = mod("interaction.create", func(f *domain.InteractionFacts) { f.OwnerKind = domain.KindApp })
	cases["app card deliver"] = mod("agent.invoke", func(f *domain.InteractionFacts) { f.OwnerKind = domain.KindApp })
	cases["deliver at create"] = mod("interaction.create", func(f *domain.InteractionFacts) { f.Stage = domain.InteractionStageDeliver })
	cases["accept at invoke"] = mod("agent.invoke", func(f *domain.InteractionFacts) { f.Stage = domain.InteractionStageAccept })
	cases["no stage"] = mod("interaction.create", func(f *domain.InteractionFacts) { f.Stage = "" })
	cases["unknown capability"] = mod("interaction.create", func(f *domain.InteractionFacts) { f.RequiredCapability = "admin" })
	cases["unknown restriction"] = mod("interaction.create", func(f *domain.InteractionFacts) { f.RestrictedTo = "everyone" })
	cases["no restriction"] = mod("interaction.create", func(f *domain.InteractionFacts) { f.RestrictedTo = "" })
	cases["unknown state"] = mod("interaction.create", func(f *domain.InteractionFacts) { f.ComponentState = "open" })
	for name, in := range cases {
		got, err := domain.Decide(in)
		if errs.KindOf(err) != errs.KindInternal {
			t.Errorf("%s: %+v, %v; want an internal error", name, got, err)
		}
	}
}

// TestAgentInvoke_WithoutInteractionUnchanged: agent.invoke without interaction facts keeps the
// order of M1-28 (SR2 before the project rules, run_active_other_agent last) and never gives
// run_active.
func TestAgentInvoke_WithoutInteractionUnchanged(t *testing.T) {
	t.Parallel()
	in := interactionInput(t, "agent.invoke", trigger, nil)
	in.Invoke.ToolClass = domain.ToolHost
	in.Invoke.ProjectBound = true
	in.Invoke.ActiveRunAgentID = ids.New()
	for _, s := range []struct {
		want domain.Reason
		fix  func(*domain.Input)
	}{
		{domain.ReasonToolClassOwnerOnly, func(in *domain.Input) { in.Invoke.ToolClass = domain.ToolChat }},
		{domain.ReasonProjectNotReady, func(in *domain.Input) { in.Invoke.ProjectReady = true }},
		{domain.ReasonDevEnvironmentNotReady, func(in *domain.Input) { in.Invoke.DevEnvironmentReady = true }},
		{domain.ReasonRunActiveOtherAgent, func(in *domain.Input) { in.Invoke.ActiveRunAgentID = agentID }},
	} {
		got, err := domain.Decide(in)
		if err != nil || got != denied(s.want) {
			t.Fatalf("want %s: got %+v, %v", s.want, got, err)
		}
		s.fix(&in)
	}
	if got, err := domain.Decide(in); err != nil || got != domain.Allow {
		t.Fatalf("own active run: %+v, %v", got, err)
	}
}
