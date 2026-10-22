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

// The values of the command facts (03 commands).
const (
	cmdAgent    = "agent"
	cmdApp      = "app"
	cmdInstance = "instance"
	cmdSpace    = "space"
	cmdChannel  = "channel"
	cmdTrigger  = "trigger"
	cmdWrite    = "write"
)

// command is a command resource of owner with the scope of kind on target (zero for instance).
func command(owner ids.ID, ownerKind, kind string, target ids.ID) authorize.Resource {
	f := &authorize.CommandFacts{OwnerKind: ownerKind, ScopeKind: kind}
	switch kind {
	case cmdSpace:
		f.ScopeSpaceID = target
	case cmdChannel:
		f.ScopeChannelID = target
	}
	return authorize.Resource{Kind: authorize.ResourceCommand, ID: ids.New(), OwnerID: owner, Command: f}
}

// offered is command r as an item of the command list of channel.
func offered(r authorize.Resource, channel ids.ID) authorize.Resource {
	f := *r.Command
	f.OfferedInChannelID = channel
	r.Command = &f
	return r
}

// readExpect checks one decision of command.read in filter mode, the mode of the list items and
// of the receive check.
func (w *world) readExpect(t *testing.T, p authorize.Principal, r authorize.Resource, o authorize.Outcome) {
	t.Helper()
	d := w.decide(t, authorize.Filter(context.Background()), p, authorize.ActionCommandRead, r)
	if d.Outcome != o {
		t.Fatalf("command.read: got %s/%s, want %s", d.Outcome, d.Reason, o)
	}
}

// bump raises the rights version so the snapshots read the changed fakes.
func (w *world) bump() {
	v, _ := w.versions.PermissionVersion(context.Background())
	w.versions.Set(v + 1)
}

// TestCommandReadByKind: command.read is read on a channel and the visibility rule on a single
// command (brief M2-16 design note 8 d).
func TestCommandReadByKind(t *testing.T) {
	req, ok := domain.RequirementOf("command.read")
	if !ok || req.Expr.String() != "ByKind(channel:Level(read),command:Auth)" {
		t.Fatalf("command.read requirement %v", req.Expr)
	}
	if spec, _ := domain.LookupAction("command.read"); len(domain.ActionResourceKinds(spec)) != 2 {
		t.Fatalf("resource kinds %v", domain.ActionResourceKinds(spec))
	}
	w := newWorld(t)
	w.allow(t, w.user(w.alex), authorize.ActionCommandRead, w.channel(w.allgemein))
	w.expect(t, w.user(w.robin), authorize.ActionCommandRead, w.channel(w.allgemein), authorize.OutcomeNotFound, "no_grant")
	if _, ok := domain.RequirementOf("command.invoke"); !ok {
		t.Fatal("command.invoke has no requirement")
	}
	for _, a := range domain.DeferredActions() {
		if a == "command.invoke" {
			t.Fatal("command.invoke is still deferred")
		}
	}
}

// TestCommandVisibility covers both modes of design note 8 b.
func TestCommandVisibility(t *testing.T) {
	t.Run("RequestMode", func(t *testing.T) {
		w := newWorld(t)
		allg := command(w.lotse, cmdAgent, cmdChannel, w.allgemein)
		kamp := command(w.lotse, cmdAgent, cmdChannel, w.kampagnen)
		w.allow(t, w.agent(w.lotse), authorize.ActionCommandRead, kamp)
		w.allow(t, w.user(w.owner), authorize.ActionCommandRead, kamp)
		w.allow(t, w.user(w.alex), authorize.ActionCommandRead, allg)
		w.expect(t, w.user(w.alex), authorize.ActionCommandRead, kamp, authorize.OutcomeNotFound, "no_grant")
		w.allow(t, w.user(w.alex), authorize.ActionCommandRead, command(w.lotse, cmdAgent, cmdSpace, w.support))
		w.expect(t, w.user(w.alex), authorize.ActionCommandRead, command(w.lotse, cmdAgent, cmdSpace, w.marketing),
			authorize.OutcomeNotFound, "no_grant")
		w.allow(t, w.user(w.robin), authorize.ActionCommandRead, command(w.lotse, cmdAgent, cmdInstance, ids.ID{}))
		// An unknown scope target is not found like an invisible one.
		w.expect(t, w.user(w.alex), authorize.ActionCommandRead, command(w.lotse, cmdAgent, cmdChannel, ids.New()),
			authorize.OutcomeNotFound, "no_grant")
		// Without the command facts a request is a programming error.
		bare := authorize.Resource{Kind: authorize.ResourceCommand, ID: ids.New(), OwnerID: w.lotse}
		if _, err := w.az.Authorize(context.Background(), w.user(w.alex), authorize.ActionCommandRead, bare); errs.KindOf(err) != errs.KindInternal {
			t.Fatalf("no facts: %v", err)
		}
	})
	t.Run("EventScope", func(t *testing.T) {
		w := newWorld(t)
		ev := authorize.ResourceFromEvent("command", ids.New(), w.support, w.allgemein, ids.ID{})
		w.readExpect(t, w.user(w.alex), ev, authorize.OutcomeAllow)
		w.readExpect(t, w.user(w.robin), ev, authorize.OutcomeNotFound)
		inst := authorize.ResourceFromEvent("command", ids.New(), ids.ID{}, ids.ID{}, ids.ID{})
		w.readExpect(t, w.user(w.robin), inst, authorize.OutcomeAllow)
		space := authorize.ResourceFromEvent("command", ids.New(), w.support, ids.ID{}, ids.ID{})
		w.readExpect(t, w.user(w.alex), space, authorize.OutcomeAllow)
		w.readExpect(t, w.user(w.robin), space, authorize.OutcomeNotFound)
	})
	t.Run("Offered", func(t *testing.T) {
		w := newWorld(t)
		alex := w.user(w.alex)
		lotse := command(w.lotse, cmdAgent, cmdInstance, ids.ID{})
		w.readExpect(t, alex, offered(lotse, w.allgemein), authorize.OutcomeAllow)
		// The scope must cover the channel.
		esk := command(w.lotse, cmdAgent, cmdChannel, w.eskalation)
		w.readExpect(t, alex, offered(esk, w.allgemein), authorize.OutcomeNotFound)
		w.readExpect(t, alex, offered(esk, w.eskalation), authorize.OutcomeAllow)
		// A dm channel never offers a command.
		w.readExpect(t, alex, offered(lotse, w.dm), authorize.OutcomeNotFound)
		// Texter as a guest: only with allow_mentioned_agents and its own write.
		texter := command(w.texter, cmdAgent, cmdInstance, ids.ID{})
		w.readExpect(t, alex, offered(texter, w.allgemein), authorize.OutcomeNotFound)
		w.setChannel(t, w.allgemein, func(c *authz.ChannelFacts) { c.AllowMentionedAgents = true })
		w.readExpect(t, alex, offered(texter, w.allgemein), authorize.OutcomeNotFound)
		agentSubject := func(id ids.ID) domain.Subject { return domain.Subject{Kind: domain.SubjectAgent, PrincipalID: id} }
		w.grant(t, agentSubject(w.texter), domain.Target{Kind: domain.TargetChannel, ChannelID: w.allgemein}, domain.LevelWrite, time.Time{})
		w.grant(t, agentSubject(w.assistent), domain.Target{Kind: domain.TargetChannel, ChannelID: w.allgemein}, domain.LevelWrite, time.Time{})
		w.bump()
		w.readExpect(t, alex, offered(texter, w.allgemein), authorize.OutcomeAllow)
		// A personal agent is never a guest.
		w.readExpect(t, alex, offered(command(w.assistent, cmdAgent, cmdInstance, ids.ID{}), w.allgemein),
			authorize.OutcomeNotFound)
		// A disabled owner offers nothing.
		w.principals.Put(authz.PrincipalFacts{ID: w.texter, Kind: actor.KindAgent, Status: authz.PrincipalDisabled})
		w.bump()
		w.readExpect(t, alex, offered(texter, w.allgemein), authorize.OutcomeNotFound)
		// Robin does not read Allgemein.
		w.readExpect(t, w.user(w.robin), offered(lotse, w.allgemein), authorize.OutcomeNotFound)
	})
	t.Run("AppOwner", func(t *testing.T) {
		w := newWorld(t)
		tracker := command(w.tracker, cmdApp, cmdInstance, ids.ID{})
		w.readExpect(t, w.user(w.alex), offered(tracker, w.eskalation), authorize.OutcomeAllow)
		w.readExpect(t, w.user(w.alex), offered(tracker, w.allgemein), authorize.OutcomeNotFound)
	})
}

// commandsApp is the app Tracker with the scope commands:write.
func (w *world) commandsApp() actor.Actor {
	a := w.app()
	a.GrantedScopes = []string{domain.ScopeCommandsWrite}
	a.Scopes = []string{domain.ScopeCommandsWrite}
	return a
}

// TestCommandRegisterTargetGuard covers the target rule of design note 8 c.
func TestCommandRegisterTargetGuard(t *testing.T) {
	w := newWorld(t)
	w.structure.SetDefaultOperator(w.engineering, w.runner)
	agentSubject := func(id ids.ID) domain.Subject { return domain.Subject{Kind: domain.SubjectAgent, PrincipalID: id} }
	w.grant(t, agentSubject(w.texter), domain.Target{Kind: domain.TargetChannel, ChannelID: w.allgemein}, domain.LevelRead, time.Time{})
	w.grant(t, agentSubject(w.texter), domain.Target{Kind: domain.TargetChannel, ChannelID: w.eskalation}, domain.LevelWrite, time.Time{})
	w.grant(t, agentSubject(w.assistent), domain.Target{Kind: domain.TargetSpace, SpaceID: w.support}, domain.LevelRead, time.Time{})
	w.bump()
	lotse, texter, runner := w.agent(w.lotse), w.agent(w.texter), w.agent(w.runner)
	reg := authorize.ActionCommandRegister
	w.allow(t, lotse, reg, command(w.lotse, cmdAgent, cmdInstance, ids.ID{}))
	w.allow(t, lotse, reg, command(w.lotse, cmdAgent, cmdChannel, w.allgemein))
	w.expect(t, lotse, reg, command(w.lotse, cmdAgent, cmdChannel, w.kampagnen), authorize.OutcomeNotFound, "no_grant")
	w.expect(t, lotse, reg, command(w.lotse, cmdAgent, cmdChannel, ids.New()), authorize.OutcomeNotFound, "no_grant")
	w.allow(t, lotse, reg, command(w.lotse, cmdAgent, cmdSpace, w.support))
	w.expect(t, lotse, reg, command(w.lotse, cmdAgent, cmdSpace, w.marketing), authorize.OutcomeNotFound, "no_grant")
	// The default operator of a space without a grant on it.
	w.allow(t, runner, reg, command(w.runner, cmdAgent, cmdSpace, w.engineering))
	w.allow(t, texter, reg, command(w.texter, cmdAgent, cmdChannel, w.eskalation))
	w.expect(t, texter, reg, command(w.texter, cmdAgent, cmdChannel, w.allgemein), authorize.OutcomeForbidden, "level_insufficient")
	w.expect(t, w.agent(w.assistent), reg, command(w.assistent, cmdAgent, cmdSpace, w.support), authorize.OutcomeForbidden,
		"level_insufficient")
	w.expect(t, lotse, reg, command(w.lotse, cmdAgent, cmdChannel, w.dm), authorize.OutcomeNotFound, "no_grant")
	for _, p := range []ids.ID{w.lotse, w.texter, w.runner, w.assistent} {
		w.allow(t, w.agent(p), reg, command(p, cmdAgent, cmdInstance, ids.ID{}))
	}
	// People never register; another agent is not the owner.
	w.expect(t, w.user(w.alex), reg, command(w.alex, cmdAgent, cmdInstance, ids.ID{}), authorize.OutcomeForbidden, "subject_kind_not_allowed")
	w.expect(t, texter, reg, command(w.lotse, cmdAgent, cmdInstance, ids.ID{}), authorize.OutcomeForbidden, "not_self")
	// The app with commands:write registers on the instance.
	w.allow(t, w.commandsApp(), reg, command(w.tracker, cmdApp, cmdInstance, ids.ID{}))
}

// invokeCommand is command.invoke in the main thread of channel for a command of owner.
func (w *world) invokeCommand(channel, owner ids.ID, ownerKind, level string, control, active bool) authorize.Resource {
	r := w.thread(channel)
	r.ChannelID = channel
	r.OwnerID = owner
	r.Command = &authorize.CommandFacts{OwnerKind: ownerKind, RequiredLevel: level, Control: control, ActiveRun: active}
	return r
}

// TestCommandInvokeRequirement covers the requirement and guard of command.invoke (design note 8 e).
func TestCommandInvokeRequirement(t *testing.T) {
	w := newWorld(t)
	userSubject := func(id ids.ID) domain.Subject { return domain.Subject{Kind: domain.SubjectUser, PrincipalID: id} }
	w.grant(t, userSubject(w.sam), domain.Target{Kind: domain.TargetChannel, ChannelID: w.eskalation}, domain.LevelWrite, time.Time{})
	w.bump()
	inv := authorize.ActionCommandInvoke
	alex, sam := w.user(w.alex), w.user(w.sam)
	w.allow(t, alex, inv, w.invokeCommand(w.allgemein, w.lotse, cmdAgent, cmdTrigger, false, false))
	w.allow(t, sam, inv, w.invokeCommand(w.eskalation, w.lotse, cmdAgent, cmdWrite, false, false))
	w.expect(t, sam, inv, w.invokeCommand(w.eskalation, w.lotse, cmdAgent, cmdTrigger, false, false), authorize.OutcomeForbidden,
		"level_insufficient")
	// run_active only for a model command.
	w.expect(t, alex, inv, w.invokeCommand(w.allgemein, w.lotse, cmdAgent, cmdTrigger, false, true), authorize.OutcomeForbidden, "run_active")
	w.allow(t, alex, inv, w.invokeCommand(w.allgemein, w.lotse, cmdAgent, cmdTrigger, true, true))
	// An app owner is an internal error until M2-25.
	if _, err := w.az.Authorize(context.Background(), alex, inv, w.invokeCommand(w.allgemein, w.tracker, cmdApp, cmdTrigger, true, false)); errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("app owner: %v", err)
	}
	if _, err := w.az.Authorize(context.Background(), alex, inv, w.thread(w.allgemein)); errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("without facts: %v", err)
	}
	bad := w.invokeCommand(w.allgemein, w.lotse, cmdAgent, "read", true, false)
	if _, err := w.az.Authorize(context.Background(), alex, inv, bad); errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("unknown level: %v", err)
	}
	// Read-only and archived channels refuse every command, owners included.
	w.setChannel(t, w.allgemein, func(c *authz.ChannelFacts) { c.ReadOnly = true })
	for _, p := range []authorize.Principal{alex, w.user(w.owner)} {
		w.expect(t, p, inv, w.invokeCommand(w.allgemein, w.lotse, cmdAgent, cmdTrigger, true, false), authorize.OutcomeForbidden,
			"channel_read_only")
	}
	w.setChannel(t, w.eskalation, func(c *authz.ChannelFacts) { c.Archived = true })
	w.expect(t, alex, inv, w.invokeCommand(w.eskalation, w.lotse, cmdAgent, cmdWrite, true, false), authorize.OutcomeForbidden,
		"channel_archived")
}
