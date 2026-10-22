// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// autocomplete is command.autocomplete in channel for a command of owner (brief M2-17 design note
// 3): the channel with the target thread as hint, the command facts and, for an agent owner, the
// invoke facts of a human request that addresses the owner explicitly and has no active run.
// thread zero means no target thread (a forum without thread_id), which sets NewThread.
func autocomplete(channel, thread, owner ids.ID, ownerKind, level string) authorize.Resource {
	r := authorize.Resource{Kind: authorize.ResourceChannel, ID: channel, ThreadID: thread,
		Command: &authorize.CommandFacts{OwnerKind: ownerKind, RequiredLevel: level}}
	if ownerKind == cmdAgent {
		r.Invoke = &authorize.InvokeFacts{AgentID: owner, TriggerOrigin: "human", MentionsAgent: true, NewThread: thread.IsZero()}
	}
	return r
}

// TestCommandAutocompleteRequirement covers the requirement of command.autocomplete (brief M2-17
// design note 3) through Authorize: the trigger right and the agent.invoke guard without a run for
// an agent owner, required_level alone for an app owner.
func TestCommandAutocompleteRequirement(t *testing.T) {
	w := newWorld(t)
	user := func(id ids.ID) domain.Subject { return domain.Subject{Kind: domain.SubjectUser, PrincipalID: id} }
	w.grant(t, user(w.robin), domain.Target{Kind: domain.TargetChannel, ChannelID: w.allgemein}, domain.LevelRead, time.Time{})
	w.grant(t, user(w.alex), domain.Target{Kind: domain.TargetChannel, ChannelID: w.werkzeuge}, domain.LevelTrigger, time.Time{})
	w.grant(t, user(w.sam), domain.Target{Kind: domain.TargetChannel, ChannelID: w.eskalation}, domain.LevelWrite, time.Time{})
	// Sam reads Allgemein through the group Leser here; the outsider of the brief reads nothing.
	outsider := ids.New()
	w.principals.Put(authz.PrincipalFacts{ID: outsider, Kind: actor.KindUser, Status: authz.PrincipalActive})
	w.addSession(outsider)
	w.bump()
	ac := authorize.ActionCommandAutocomplete
	alex, sam, robin, owner := w.user(w.alex), w.user(w.sam), w.user(w.robin), w.user(w.owner)
	allg := w.mainThread[w.allgemein]
	lotse := autocomplete(w.allgemein, allg, w.lotse, cmdAgent, cmdTrigger)

	w.expect(t, w.user(outsider), ac, lotse, authorize.OutcomeNotFound, "no_grant")
	w.expect(t, robin, ac, lotse, authorize.OutcomeForbidden, "level_insufficient")
	w.allow(t, alex, ac, lotse)
	// A guest owner needs allow_mentioned_agents (09 section 10.3).
	w.expect(t, alex, ac, autocomplete(w.allgemein, allg, w.texter, cmdAgent, cmdTrigger), authorize.OutcomeForbidden, "agent_not_reachable")
	// SR2: Runner (host) only for owners.
	tools := w.mainThread[w.werkzeuge]
	w.expect(t, alex, ac, autocomplete(w.werkzeuge, tools, w.runner, cmdAgent, cmdTrigger), authorize.OutcomeForbidden, "tool_class_owner_only")
	w.allow(t, owner, ac, autocomplete(w.werkzeuge, tools, w.runner, cmdAgent, cmdTrigger))
	// SR4: an agent never reaches trigger (05 section 2.3 point 5).
	w.expect(t, w.agent(w.lotse), ac, lotse, authorize.OutcomeForbidden, "level_insufficient")
	// No agent in a dm channel.
	w.expect(t, alex, ac, autocomplete(w.dm, w.mainThread[w.dm], w.lotse, cmdAgent, cmdTrigger), authorize.OutcomeForbidden,
		"level_insufficient")
	// A forum without thread_id is checked as for a new thread of the channel (R-AZ-26).
	w.allow(t, owner, ac, autocomplete(w.projektA, ids.ID{}, w.runner, cmdAgent, cmdTrigger))
	w.expect(t, alex, ac, autocomplete(w.projektA, ids.ID{}, w.runner, cmdAgent, cmdTrigger), authorize.OutcomeNotFound, "no_grant")
	// A project-bound forum must be ready (09 3.6; M2-03).
	w.setChannel(t, w.projektA, func(c *authz.ChannelFacts) { c.ProjectReady = false })
	w.expect(t, owner, ac, autocomplete(w.projektA, ids.ID{}, w.runner, cmdAgent, cmdTrigger), authorize.OutcomeForbidden, "project_not_ready")
	w.setChannel(t, w.projektA, func(c *authz.ChannelFacts) { c.ProjectReady = true })
	// An app owner: required_level only, without the invoke guard and I2 (R-AZ-18).
	w.allow(t, sam, ac, autocomplete(w.eskalation, w.mainThread[w.eskalation], w.tracker, cmdApp, cmdWrite))
	w.expect(t, sam, ac, autocomplete(w.eskalation, w.mainThread[w.eskalation], w.tracker, cmdApp, cmdTrigger), authorize.OutcomeForbidden,
		"level_insufficient")
	// Programming errors: no command facts, an agent owner without invoke facts.
	bare := lotse
	bare.Command = nil
	if _, err := w.az.Authorize(context.Background(), alex, ac, bare); errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("without command facts: %v", err)
	}
	noInvoke := lotse
	noInvoke.Invoke = nil
	if _, err := w.az.Authorize(context.Background(), alex, ac, noInvoke); errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("without invoke facts: %v", err)
	}
	// I2: Lotse switched to personal with the Owner as holder answers only the Owner.
	w.policies.Put(w.lotse, authz.AgentPolicy{BillingMode: domain.BillingPersonal, CredentialHolderID: w.owner, ToolClass: domain.ToolChat})
	w.bump()
	w.expect(t, alex, ac, lotse, authorize.OutcomeForbidden, "billing_mode_personal_not_holder")
	w.allow(t, owner, ac, lotse)
	w.policies.Put(w.lotse, authz.AgentPolicy{BillingMode: domain.BillingOrganization, ToolClass: domain.ToolChat})
	w.bump()
	// Read-only and archived channels refuse it, owners included.
	w.setChannel(t, w.allgemein, func(c *authz.ChannelFacts) { c.ReadOnly = true })
	for _, p := range []authorize.Principal{alex, owner} {
		w.expect(t, p, ac, lotse, authorize.OutcomeForbidden, "channel_read_only")
	}
	w.setChannel(t, w.eskalation, func(c *authz.ChannelFacts) { c.Archived = true })
	w.expect(t, alex, ac, autocomplete(w.eskalation, w.mainThread[w.eskalation], w.lotse, cmdAgent, cmdTrigger), authorize.OutcomeForbidden,
		"channel_archived")
	if slices.Contains(domain.DeferredActions(), "command.autocomplete") {
		t.Fatal("command.autocomplete is still deferred")
	}
}

// respond is command.autocomplete.respond of a request for a command of owner in the main thread
// of channel (brief M2-17 design note 3).
func (w *world) respond(channel, owner ids.ID) authorize.Resource {
	return authorize.Resource{Kind: authorize.ResourceCommand, ID: ids.New(), ChannelID: channel, ThreadID: w.mainThread[channel], OwnerID: owner}
}

// TestCommandAutocompleteRespondRequirement: only the owner answers; another agent that reads the
// place gets not_self, one that does not not_found, a person subject_kind_not_allowed.
func TestCommandAutocompleteRespondRequirement(t *testing.T) {
	w := newWorld(t)
	resp := authorize.ActionCommandAutocompleteRespond
	r := w.respond(w.allgemein, w.lotse)
	w.allow(t, w.agent(w.lotse), resp, r)
	w.expect(t, w.agent(w.texter), resp, r, authorize.OutcomeNotFound, "no_grant")
	w.grant(t, domain.Subject{Kind: domain.SubjectAgent, PrincipalID: w.texter}, domain.Target{Kind: domain.TargetChannel, ChannelID: w.allgemein},
		domain.LevelRead, time.Time{})
	w.bump()
	w.expect(t, w.agent(w.texter), resp, r, authorize.OutcomeForbidden, "not_self")
	w.expect(t, w.user(w.alex), resp, r, authorize.OutcomeForbidden, "subject_kind_not_allowed")
	// The app owner needs commands:write (05 section 2.6).
	tr := w.respond(w.eskalation, w.tracker)
	w.expect(t, w.app(), resp, tr, authorize.OutcomeForbidden, "scope_missing")
	w.allow(t, w.commandsApp(), resp, tr)
}
