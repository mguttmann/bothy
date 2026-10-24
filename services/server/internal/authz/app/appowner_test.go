// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// The tests of the app as owner of commands and cards (brief M2-25 design notes 3 and 4) in the
// world of 07 section 11.1: the app Tracker with write on Support/Eskalation (G-6).

// appScopes are the granted scopes of Tracker in these tests unless a test narrows them.
var appScopes = []string{domain.ScopeMessagesWrite, domain.ScopeCommandsWrite, domain.ScopeComponentsWrite, domain.ScopeEventsSubscribe}

// setTracker stores the facts of Tracker: its installation state and granted scopes.
func (w *world) setTracker(active bool, scopes ...string) {
	w.principals.Put(authz.PrincipalFacts{ID: w.tracker, Kind: actor.KindApp, Status: authz.PrincipalActive, AppInstallationActive: active,
		AppGrantedScopes: scopes})
	w.versions.Bump()
}

// appCommandInvoke is command.invoke of an instance-wide command of Tracker with required level
// trigger in the main thread of channel; the resource names the thread's creator (none here) like runs does.
func (w *world) appCommandInvoke(channel ids.ID) authorize.Resource {
	r := w.thread(channel)
	r.Command = &authorize.CommandFacts{OwnerKind: domain.KindApp, OwnerID: w.tracker, RequiredLevel: "trigger"}
	return r
}

// appCommand is the command resource of an instance-wide command of Tracker (getCommand).
func (w *world) appCommand() authorize.Resource {
	return authorize.Resource{Kind: authorize.ResourceCommand, ID: ids.New(), OwnerID: w.tracker,
		Command: &authorize.CommandFacts{OwnerKind: domain.KindApp, ScopeKind: "instance", RequiredLevel: "trigger"}}
}

// appComponent is interaction.create on a component of a card of Tracker in the main thread of
// channel at stage.
func (w *world) appComponent(channel ids.ID, stage string, mod func(*authorize.InteractionFacts)) authorize.Resource {
	f := &authorize.InteractionFacts{Stage: stage, OwnerKind: domain.KindApp, ComponentState: domain.ComponentActive, RestrictedTo: domain.RestrictedAny}
	if mod != nil {
		mod(f)
	}
	return authorize.Resource{Kind: authorize.ResourceComponent, ID: ids.New(), ThreadID: w.mainThread[channel], OwnerID: w.tracker, Interaction: f}
}

// TestCommandInvoke_AppBranchSkipsInvokeGuard: Alex holds trigger on Research through a grant
// that simulates an I1 defect (OQ-01), while Assistent stays personal with the owner as holder. A
// command of Tracker there is allowed (no agent.invoke, no I2), the trigger of Assistent is
// refused with billing_mode_personal_not_holder.
func TestCommandInvoke_AppBranchSkipsInvokeGuard(t *testing.T) {
	w := newWorld(t)
	user := func(id ids.ID) domain.Subject { return domain.Subject{Kind: domain.SubjectUser, PrincipalID: id} }
	w.grant(t, user(w.alex), domain.Target{Kind: domain.TargetSpace, SpaceID: w.resea}, domain.LevelTrigger, time.Time{})
	w.grant(t, domain.Subject{Kind: domain.SubjectApp, PrincipalID: w.tracker}, domain.Target{Kind: domain.TargetChannel, ChannelID: w.notizen},
		domain.LevelWrite, time.Time{})
	w.setTracker(true, appScopes...)
	w.allow(t, w.user(w.alex), authorize.ActionCommandInvoke, w.appCommandInvoke(w.notizen))
	w.allow(t, w.user(w.alex), authorize.ActionInteractionCreate, w.appComponent(w.notizen, authorize.InteractionStageAccept, nil))
	w.expect(t, w.user(w.alex), authorize.ActionAgentInvoke, w.invoke(w.notizen, w.assistent, true), forbidden, "billing_mode_personal_not_holder")
}

// TestCommandInvoke_AppBranchOrder: an unusable app command is hidden (not_found no_grant) before
// the level of the command is checked, the level before the read-only rule, and an archived
// channel gives channel_archived.
func TestCommandInvoke_AppBranchOrder(t *testing.T) {
	w := newWorld(t)
	w.setTracker(true, appScopes...)
	// Sam reads Support/Allgemein through Leser; Tracker has no grant there: hidden.
	w.expect(t, w.user(w.sam), authorize.ActionCommandInvoke, w.appCommandInvoke(w.allgemein), authorize.OutcomeNotFound, "no_grant")
	w.grant(t, domain.Subject{Kind: domain.SubjectApp, PrincipalID: w.tracker}, domain.Target{Kind: domain.TargetChannel, ChannelID: w.allgemein},
		domain.LevelWrite, time.Time{})
	w.versions.Bump()
	w.expect(t, w.user(w.sam), authorize.ActionCommandInvoke, w.appCommandInvoke(w.allgemein), forbidden, "level_insufficient")
	w.allow(t, w.user(w.alex), authorize.ActionCommandInvoke, w.appCommandInvoke(w.allgemein))
	// A suspended Tracker or one without commands:write hides the command again, also for Alex.
	w.setTracker(false, appScopes...)
	w.expect(t, w.user(w.alex), authorize.ActionCommandInvoke, w.appCommandInvoke(w.allgemein), authorize.OutcomeNotFound, "no_grant")
	w.setTracker(true, domain.ScopeMessagesWrite)
	auto := w.channel(w.allgemein)
	auto.ThreadID = w.mainThread[w.allgemein]
	auto.Command = &authorize.CommandFacts{OwnerKind: domain.KindApp, OwnerID: w.tracker, RequiredLevel: "trigger"}
	w.expect(t, w.user(w.alex), authorize.ActionCommandAutocomplete, auto, authorize.OutcomeNotFound, "no_grant")
	w.setTracker(true, appScopes...)
	w.allow(t, w.user(w.alex), authorize.ActionCommandAutocomplete, auto)
	w.setTracker(true, appScopes...)
	ch, _, _ := w.structure.Channel(context.Background(), w.allgemein)
	ch.ReadOnly = true
	w.structure.PutChannel(ch)
	w.versions.Bump()
	w.expect(t, w.user(w.sam), authorize.ActionCommandInvoke, w.appCommandInvoke(w.allgemein), forbidden, "level_insufficient")
	w.expect(t, w.user(w.alex), authorize.ActionCommandInvoke, w.appCommandInvoke(w.allgemein), forbidden, "channel_read_only")
	ch.ReadOnly, ch.Archived = false, true
	w.structure.PutChannel(ch)
	w.versions.Bump()
	w.expect(t, w.user(w.alex), authorize.ActionCommandInvoke, w.appCommandInvoke(w.allgemein), forbidden, "channel_archived")
}

// TestCommandRead_AppCommandHidden: getCommand of an instance-wide app command in request mode is
// allowed with an active installation and commands:write whatever the grants of the app are, and
// not_found no_grant when the app is suspended or lacks the scope.
func TestCommandRead_AppCommandHidden(t *testing.T) {
	w := newWorld(t)
	w.setTracker(true, appScopes...)
	w.allow(t, w.user(w.alex), authorize.ActionCommandRead, w.appCommand())
	w.allow(t, w.user(w.robin), authorize.ActionCommandRead, w.appCommand())
	w.setTracker(false, appScopes...)
	w.expect(t, w.user(w.alex), authorize.ActionCommandRead, w.appCommand(), authorize.OutcomeNotFound, "no_grant")
	w.setTracker(true, domain.ScopeMessagesWrite)
	w.expect(t, w.user(w.alex), authorize.ActionCommandRead, w.appCommand(), authorize.OutcomeNotFound, "no_grant")
	// An item of the list of Support/Allgemein needs write of Tracker there as well.
	w.setTracker(true, appScopes...)
	item := w.appCommand()
	item.Command.OfferedInChannelID = w.allgemein
	w.expect(t, w.user(w.alex), authorize.ActionCommandRead, item, authorize.OutcomeNotFound, "no_grant")
	item.Command.OfferedInChannelID = w.eskalation
	w.allow(t, w.user(w.alex), authorize.ActionCommandRead, item)
}

// TestInteractionCreate_AppBranch: a component of a Tracker card needs trigger, an active state
// before its expiry and the app-owner guard with components:write (else component_inactive), the
// required capability and the restriction; no reason of the agent.invoke guard ever appears.
func TestInteractionCreate_AppBranch(t *testing.T) {
	w := newWorld(t)
	w.setTracker(true, appScopes...)
	accept := authorize.InteractionStageAccept
	w.allow(t, w.user(w.alex), authorize.ActionInteractionCreate, w.appComponent(w.eskalation, accept, nil))
	w.expect(t, w.user(w.sam), authorize.ActionInteractionCreate, w.appComponent(w.eskalation, accept, nil), authorize.OutcomeNotFound, "no_grant")
	cases := []struct {
		name   string
		p      actor.Actor
		mod    func(*authorize.InteractionFacts)
		reason string
	}{
		{"used", w.user(w.alex), func(f *authorize.InteractionFacts) { f.ComponentState = domain.ComponentUsed }, "component_inactive"},
		{"expired", w.user(w.alex), func(f *authorize.InteractionFacts) { f.ExpiresAt = w.clk.Now() }, "component_inactive"},
		{"capability", w.user(w.alex), func(f *authorize.InteractionFacts) { f.RequiredCapability = "export" }, "capability_missing"},
		{"owner only", w.user(w.alex), func(f *authorize.InteractionFacts) { f.RestrictedTo = domain.RestrictedOwner }, "component_restricted"},
		{"run starter", w.user(w.alex), func(f *authorize.InteractionFacts) { f.RestrictedTo = domain.RestrictedRunStarter }, "component_restricted"},
	}
	for _, c := range cases {
		d := w.decide(t, context.Background(), c.p, authorize.ActionInteractionCreate, w.appComponent(w.eskalation, accept, c.mod))
		if d.Outcome != forbidden || string(d.Reason) != c.reason {
			t.Errorf("%s: %s/%s, want %s", c.name, d.Outcome, d.Reason, c.reason)
		}
	}
	w.allow(t, w.user(w.alex), authorize.ActionInteractionCreate, w.appComponent(w.eskalation, accept, func(f *authorize.InteractionFacts) {
		f.RequiredCapability = "approve_actions"
	}))
	for _, r := range []string{domain.RestrictedOwner, domain.RestrictedRunStarter} {
		w.allow(t, w.user(w.owner), authorize.ActionInteractionCreate, w.appComponent(w.eskalation, accept, func(f *authorize.InteractionFacts) {
			f.RestrictedTo = r
		}))
	}
	// The guard: without components:write, suspended, or without write of Tracker on the place.
	w.setTracker(true, domain.ScopeMessagesWrite, domain.ScopeCommandsWrite)
	w.expect(t, w.user(w.alex), authorize.ActionInteractionCreate, w.appComponent(w.eskalation, accept, nil), forbidden, "component_inactive")
	w.setTracker(false, appScopes...)
	w.expect(t, w.user(w.alex), authorize.ActionInteractionCreate, w.appComponent(w.eskalation, accept, nil), forbidden, "component_inactive")
	w.setTracker(true, appScopes...)
	w.expect(t, w.user(w.alex), authorize.ActionInteractionCreate, w.appComponent(w.allgemein, accept, nil), forbidden, "component_inactive")
	// An app card with the trigger of an agent or bound to a run cannot be evaluated.
	withInvoke := w.appComponent(w.eskalation, accept, nil)
	withInvoke.Invoke = &authorize.InvokeFacts{AgentID: w.tracker, TriggerOrigin: "human"}
	if _, err := w.az.Authorize(context.Background(), w.user(w.alex), authorize.ActionInteractionCreate, withInvoke); errs.KindOf(err) != errs.KindInternal {
		t.Errorf("app card with invoke facts: %v, want internal", err)
	}
}

// TestAgentEventsConsume_ReceiveRuleForApps: in filter mode the audience app with an active
// installation and events:subscribe receives its personal events, without the scope it gets
// scope_missing, another principal's event not_found not_self; in request mode (the consumer
// operations of M2-07) the requirement stays AgentSelf, so an app is refused as before; agents are
// unchanged.
func TestAgentEventsConsume_ReceiveRuleForApps(t *testing.T) {
	w := newWorld(t)
	w.setTracker(true, appScopes...)
	// The descriptor of the hub and the catch-up (the audience) and the one of the consumer
	// operations of M2-07 (agentResource: the principal itself as owner).
	self := authorize.Resource{Kind: authorize.ResourceAgent, ID: w.tracker}
	selfOwned := authorize.Resource{Kind: authorize.ResourceAgent, ID: w.tracker, OwnerID: w.tracker}
	other := authorize.Resource{Kind: authorize.ResourceAgent, ID: w.lotse}
	filter := authorize.Filter(context.Background())
	check := func(name string, ctx context.Context, p authorize.Principal, r authorize.Resource, o authorize.Outcome, reason string) {
		t.Helper()
		d := w.decide(t, ctx, p, authorize.ActionAgentEventsConsume, r)
		if d.Outcome != o || string(d.Reason) != reason {
			t.Errorf("%s: %s/%s, want %s/%s", name, d.Outcome, d.Reason, o, reason)
		}
	}
	check("own event", filter, w.app(), self, authorize.OutcomeAllow, "")
	check("own event, delivery", authorize.Delivery(context.Background()), w.app(), self, authorize.OutcomeAllow, "")
	check("other's event", filter, w.app(), other, authorize.OutcomeNotFound, "not_self")
	check("request mode", context.Background(), w.app(), selfOwned, authorize.OutcomeNotFound, "no_grant")
	check("agent own", filter, w.agent(w.lotse), other, authorize.OutcomeAllow, "")
	w.setTracker(true, domain.ScopeMessagesWrite)
	check("without events:subscribe", filter, w.app(), self, forbidden, "scope_missing")
	w.setTracker(false, appScopes...)
	check("suspended", filter, w.app(), self, authorize.OutcomeUnauthenticated, "app_not_installed")
}

// TestRecheckPrincipal_AppActions (R-AZ-29): the owner's re-check principal, without a grant on
// Support/Eskalation, allows command.invoke of a Tracker command there and interaction.create at
// stage deliver on a Tracker card restricted to owners; a disabled person is principal_inactive.
func TestRecheckPrincipal_AppActions(t *testing.T) {
	w := newWorld(t)
	w.setTracker(true, appScopes...)
	deliver := w.appComponent(w.eskalation, authorize.InteractionStageDeliver, func(f *authorize.InteractionFacts) {
		f.RestrictedTo = domain.RestrictedOwner
		f.ComponentState = domain.ComponentUsed
	})
	w.allow(t, recheck(w.owner), authorize.ActionCommandInvoke, w.appCommandInvoke(w.eskalation))
	w.allow(t, recheck(w.owner), authorize.ActionInteractionCreate, deliver)
	w.allow(t, recheck(w.alex), authorize.ActionCommandInvoke, w.appCommandInvoke(w.eskalation))
	w.expect(t, recheck(w.alex), authorize.ActionInteractionCreate, deliver, forbidden, "component_restricted")
	w.principals.Put(authz.PrincipalFacts{ID: w.alex, Kind: actor.KindUser, Status: authz.PrincipalDisabled})
	w.versions.Bump()
	w.expect(t, recheck(w.alex), authorize.ActionCommandInvoke, w.appCommandInvoke(w.eskalation), authorize.OutcomeUnauthenticated,
		"principal_inactive")
	if _, err := w.az.Authorize(context.Background(), recheck(w.owner), authorize.ActionCommandRead, w.appCommand()); errs.KindOf(err) != errs.KindInternal {
		t.Errorf("command.read with the re-check principal: %v, want internal", err)
	}
}
