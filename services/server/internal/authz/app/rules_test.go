// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/authz/adapters/fake"
	"github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// setChannel changes the facts of a channel in the fake structure.
func (w *world) setChannel(t testing.TB, id ids.ID, mod func(*authz.ChannelFacts)) {
	t.Helper()
	c, ok, _ := w.structure.Channel(context.Background(), id)
	if !ok {
		t.Fatal("no channel")
	}
	mod(&c)
	w.structure.PutChannel(c)
}

func TestSR5_OwnerOnlyInSession(t *testing.T) {
	w := newWorld(t)
	w.addToken(w.owner)
	personal := actor.Actor{ID: w.owner, Kind: actor.KindUser, TokenID: w.tokens[w.owner]}
	w.expect(t, personal, authorize.ActionUserCreate, authorize.Resource{Kind: authorize.ResourceInstance}, forbidden, "owner_only")
	w.expect(t, personal, authorize.ActionIdentityLink, authorize.Resource{Kind: authorize.ResourceUser, ID: w.owner, OwnerID: w.owner},
		forbidden, "step_up_not_possible")
	// The same owner in a session has the owner rights.
	w.allow(t, w.user(w.owner), authorize.ActionUserCreate, authorize.Resource{Kind: authorize.ResourceInstance})
	// With the token the owner reads nothing, because the owner holds no grants (SR5).
	w.expect(t, personal, authorize.ActionMessageRead, w.msg(w.notizen, w.assistent), notFound, "no_grant")
}

func TestSR6_OwnerNeverSeesDM(t *testing.T) {
	w := newWorld(t)
	w.expect(t, w.user(w.owner), authorize.ActionMessageRead, w.msg(w.dm, w.alex), notFound, "no_grant")
	w.expect(t, w.user(w.owner), authorize.ActionChannelRead, w.channel(w.dm), notFound, "no_grant")
	w.allow(t, w.user(w.alex), authorize.ActionMessageRead, w.msg(w.dm, w.sam))
	w.allow(t, w.user(w.sam), authorize.ActionMessageCreate, w.thread(w.dm))
	w.expect(t, w.user(w.robin), authorize.ActionMessageRead, w.msg(w.dm, w.sam), notFound, "no_grant")
}

func TestOperatorRule_MainAndFixedThread(t *testing.T) {
	w := newWorld(t)
	// A thread in Kampagnen with the fixed operator Lotse.
	th := ids.New()
	w.structure.PutThread(authz.ThreadFacts{ID: th, ChannelID: w.kampagnen, OperatorAgentID: w.lotse, CreatedBy: w.sam})
	thread := authorize.Resource{Kind: authorize.ResourceThread, ID: th}
	w.allow(t, w.agent(w.lotse), authorize.ActionMessageCreate, thread)
	w.allow(t, w.agent(w.texter), authorize.ActionMessageCreate, w.thread(w.kampagnen))
	// The channel's operator changes to Runner: the rule follows, without a new rights version.
	w.setChannel(t, w.kampagnen, func(c *authz.ChannelFacts) { c.EffectiveOperatorID = w.runner })
	w.allow(t, w.agent(w.runner), authorize.ActionMessageCreate, w.thread(w.kampagnen))
	w.expect(t, w.agent(w.texter), authorize.ActionMessageCreate, w.thread(w.kampagnen), notFound, "no_grant")
	w.allow(t, w.agent(w.lotse), authorize.ActionMessageCreate, thread)
	w.expect(t, w.agent(w.lotse), authorize.ActionMessageCreate, w.thread(w.kampagnen), notFound, "no_grant")
	// The channel's operator writes in every thread of the channel, fixed-operator threads included.
	w.allow(t, w.agent(w.runner), authorize.ActionMessageCreate, thread)
}

func TestBotLimits_MismatchingSubjectKindIsCapped(t *testing.T) {
	w := newWorld(t)
	// A grant row that names Lotse as a person with trigger and manage on Marketing.
	w.grant(t, userSubject(w.lotse), domain.Target{Kind: domain.TargetSpace, SpaceID: w.marketing}, domain.LevelTrigger, time.Time{}, domain.CapManage)
	w.versions.Bump()
	w.allow(t, w.agent(w.lotse), authorize.ActionMessageCreate, w.thread(w.kampagnen))
	w.expect(t, w.agent(w.lotse), authorize.ActionChannelUpdate, w.channel(w.kampagnen), forbidden, "capability_missing")
	w.expect(t, w.agent(w.lotse), authorize.ActionExportChannel, w.channel(w.kampagnen), forbidden, "capability_missing")
}

func TestReadOnlyChannel(t *testing.T) {
	w := newWorld(t)
	w.setChannel(t, w.allgemein, func(c *authz.ChannelFacts) { c.ReadOnly = true })
	w.expect(t, w.user(w.owner), authorize.ActionMessageCreate, w.thread(w.allgemein), forbidden, "channel_read_only")
	w.expect(t, w.user(w.alex), authorize.ActionAgentInvoke, w.invoke(w.allgemein, w.lotse, true), forbidden, "channel_read_only")
	// A member with manage may still delete a foreign message (moderation, 05 rule 1.8).
	w.grant(t, userSubject(w.robin), domain.Target{Kind: domain.TargetSpace, SpaceID: w.support}, domain.LevelWrite, time.Time{}, domain.CapManage)
	w.versions.Bump()
	w.allow(t, w.user(w.robin), authorize.ActionMessageDelete, w.msg(w.allgemein, w.sam))
	w.allow(t, w.user(w.owner), authorize.ActionChannelUpdate, w.channel(w.allgemein))
	// A reader gets the level reason, because the requirement fails first.
	w.expect(t, w.user(w.sam), authorize.ActionMessageCreate, w.thread(w.allgemein), forbidden, "level_insufficient")
	w.allow(t, w.user(w.sam), authorize.ActionMessageRead, w.msg(w.allgemein, w.alex))
}

func TestPendingDevice(t *testing.T) {
	w := newWorld(t)
	w.creds.PutDevice(w.devices[w.alex], fake.Device{PrincipalID: w.alex, State: authz.DevicePending})
	w.versions.Bump()
	alex := w.user(w.alex)
	w.expect(t, alex, authorize.ActionMessageRead, w.msg(w.allgemein, w.alex), forbidden, "device_pending")
	w.allow(t, alex, authorize.ActionSelfRead, authorize.Resource{Kind: authorize.ResourcePrincipal, ID: w.alex, OwnerID: w.alex})
	w.allow(t, alex, authorize.ActionDeviceList, authorize.Resource{Kind: authorize.ResourceDevice, ID: w.devices[w.alex], OwnerID: w.alex})
	w.allow(t, alex, authorize.ActionAuthLogout, authorize.Resource{Kind: authorize.ResourceAuthSession, ID: w.sessions[w.alex], OwnerID: w.alex})
	// Another person's device is invisible; another own device stays pending.
	w.expect(t, alex, authorize.ActionDeviceList, authorize.Resource{Kind: authorize.ResourceDevice, ID: w.devices[w.sam], OwnerID: w.sam},
		notFound, "not_self")
	w.expect(t, alex, authorize.ActionDeviceList, authorize.Resource{Kind: authorize.ResourceDevice, ID: ids.New(), OwnerID: w.alex},
		forbidden, "device_pending")
}

func TestPendingPerson(t *testing.T) {
	w := newWorld(t)
	f, _, _ := w.principals.Principal(context.Background(), w.robin)
	f.Status = authz.PrincipalPending
	w.principals.Put(f)
	w.versions.Bump()
	w.expect(t, w.user(w.robin), authorize.ActionMessageRead, w.msg(w.projektA, w.robin), forbidden, "principal_pending")
	w.allow(t, w.user(w.robin), authorize.ActionSelfRead, authorize.Resource{Kind: authorize.ResourcePrincipal, ID: w.robin, OwnerID: w.robin})
}

func TestStepUp(t *testing.T) {
	w := newWorld(t)
	r := authorize.Resource{Kind: authorize.ResourceSpace, ID: w.support, Grant: &authorize.GrantFacts{
		SubjectKind: "user", Level: "read", TargetKind: "space", TargetSpaceID: w.support}}
	owner := w.user(w.owner)
	owner.StepUpAt = w.clk.Now().Add(-5 * time.Minute)
	w.allow(t, owner, authorize.ActionGrantCreate, r)
	owner.StepUpAt = w.clk.Now().Add(-5*time.Minute - time.Nanosecond)
	w.expect(t, owner, authorize.ActionGrantCreate, r, stepUpNeeded, "step_up_required")
	owner.StepUpAt = time.Time{}
	w.expect(t, owner, authorize.ActionGrantCreate, r, stepUpNeeded, "step_up_required")
	w.expect(t, w.agent(w.lotse), authorize.ActionTokenCreate, authorize.Resource{Kind: authorize.ResourcePrincipal, ID: w.lotse, OwnerID: w.lotse},
		forbidden, "step_up_not_possible")
	// Without the right, no step-up is asked for (07 section 4.2: requirement before step-up).
	stale := w.user(w.alex)
	stale.StepUpAt = time.Time{}
	w.expect(t, stale, authorize.ActionUserCreate, authorize.Resource{Kind: authorize.ResourceInstance}, forbidden, "owner_only")
}

func TestViewAs_IntersectsAndIsReadOnly(t *testing.T) {
	w := newWorld(t)
	asAlex := w.user(w.owner)
	asAlex.ViewAs = &actor.ViewAs{UserID: w.alex, StartedAt: w.clk.Now()}
	w.expect(t, asAlex, authorize.ActionMessageRead, w.msg(w.kampagnen, w.sam), notFound, "no_grant")
	w.allow(t, asAlex, authorize.ActionMessageRead, w.msg(w.allgemein, w.sam))
	w.expect(t, asAlex, authorize.ActionMessageCreate, w.thread(w.allgemein), forbidden, "view_as_read_only")
	w.expect(t, asAlex, authorize.ActionMessageRead, w.msg(w.dm, w.sam), notFound, "no_grant")
	w.expect(t, asAlex, authorize.ActionUserCreate, authorize.Resource{Kind: authorize.ResourceInstance}, forbidden, "owner_only")
}

func TestActingFor_Intersects(t *testing.T) {
	w := newWorld(t)
	w.grant(t, domain.Subject{Kind: domain.SubjectAgent, PrincipalID: w.lotse}, domain.Target{Kind: domain.TargetSpace, SpaceID: w.engineering}, domain.LevelRead, time.Time{})
	w.versions.Bump()
	lotse := w.agent(w.lotse)
	w.allow(t, lotse, authorize.ActionMessageRead, w.msg(w.projektA, w.robin))
	lotse.ActingFor = actor.ActingFor{UserID: w.alex, RunID: ids.New(), ThreadID: w.mainThread[w.allgemein], JTI: "j"}
	w.expect(t, lotse, authorize.ActionMessageRead, w.msg(w.projektA, w.robin), notFound, "acting_for_insufficient")
	w.allow(t, lotse, authorize.ActionMessageRead, w.msg(w.allgemein, w.robin))
	// Sam reads Allgemein only; Lotse acting for Sam may read but not write there.
	lotse.ActingFor.UserID = w.sam
	w.allow(t, lotse, authorize.ActionMessageRead, w.msg(w.allgemein, w.robin))
	w.expect(t, lotse, authorize.ActionMessageCreate, w.thread(w.allgemein), forbidden, "acting_for_insufficient")
	// Neither may see Marketing: there the agent itself lacks read.
	w.expect(t, lotse, authorize.ActionMessageRead, w.msg(w.kampagnen, w.robin), notFound, "no_grant")
}

func TestAgentInvoke_Origin(t *testing.T) {
	w := newWorld(t)
	r := w.invoke(w.allgemein, w.lotse, true)
	r.Invoke.TriggerOrigin = "agent"
	w.expect(t, w.user(w.alex), authorize.ActionAgentInvoke, r, forbidden, "origin_not_human")
	// An agent never triggers (SR4); Lotse lacks trigger, so the level decides first.
	w.expect(t, w.agent(w.lotse), authorize.ActionAgentInvoke, w.invoke(w.allgemein, w.texter, true), forbidden, "level_insufficient")
}

func TestAgentInvoke_DM(t *testing.T) {
	w := newWorld(t)
	// Participants have write, never trigger: a dm never reaches an agent.
	w.expect(t, w.user(w.alex), authorize.ActionAgentInvoke, w.invoke(w.dm, w.lotse, true), forbidden, "level_insufficient")
}

func TestAgentInvoke_InactiveAgent(t *testing.T) {
	w := newWorld(t)
	f, _, _ := w.principals.Principal(context.Background(), w.lotse)
	f.Status = authz.PrincipalDisabled
	w.principals.Put(f)
	w.expect(t, w.user(w.alex), authorize.ActionAgentInvoke, w.invoke(w.allgemein, w.lotse, true), forbidden, "agent_not_reachable")
	w.expect(t, w.user(w.alex), authorize.ActionAgentInvoke, w.invoke(w.allgemein, ids.New(), true), forbidden, "agent_not_reachable")
}

func TestAgentInvoke_MentionRule(t *testing.T) {
	w := newWorld(t)
	alex := w.user(w.alex)
	w.allow(t, alex, authorize.ActionAgentInvoke, w.invoke(w.allgemein, w.lotse, false))
	w.setChannel(t, w.allgemein, func(c *authz.ChannelFacts) { c.RequireMention = true })
	w.expect(t, alex, authorize.ActionAgentInvoke, w.invoke(w.allgemein, w.lotse, false), forbidden, "agent_not_reachable")
	w.allow(t, alex, authorize.ActionAgentInvoke, w.invoke(w.allgemein, w.lotse, true))
	w.setChannel(t, w.allgemein, func(c *authz.ChannelFacts) { c.FreeResponse = true })
	w.allow(t, alex, authorize.ActionAgentInvoke, w.invoke(w.allgemein, w.lotse, false))
	// A non-main thread follows thread_require_mention. It was created while Lotse operated the
	// channel, so Lotse is its fixed operator (R-DM-12).
	th := ids.New()
	w.structure.PutThread(authz.ThreadFacts{ID: th, ChannelID: w.eskalation, CreatedBy: w.alex, OperatorAgentID: w.lotse})
	w.setChannel(t, w.eskalation, func(c *authz.ChannelFacts) { c.ThreadRequireMention = true })
	r := authorize.Resource{Kind: authorize.ResourceThread, ID: th, Invoke: &authorize.InvokeFacts{AgentID: w.lotse, TriggerOrigin: "human"}}
	w.expect(t, alex, authorize.ActionAgentInvoke, r, forbidden, "agent_not_reachable")
	r.Invoke.MentionsAgent = true
	w.allow(t, alex, authorize.ActionAgentInvoke, r)
}

func TestAgentInvoke_Guest(t *testing.T) {
	w := newWorld(t)
	alex := w.user(w.alex)
	w.setChannel(t, w.allgemein, func(c *authz.ChannelFacts) { c.AllowMentionedAgents = true })
	// Texter has no write on Allgemein: refused.
	w.expect(t, alex, authorize.ActionAgentInvoke, w.invoke(w.allgemein, w.texter, true), forbidden, "agent_not_reachable")
	w.grant(t, domain.Subject{Kind: domain.SubjectAgent, PrincipalID: w.texter}, domain.Target{Kind: domain.TargetChannel, ChannelID: w.allgemein}, domain.LevelWrite, time.Time{})
	w.grant(t, domain.Subject{Kind: domain.SubjectAgent, PrincipalID: w.assistent}, domain.Target{Kind: domain.TargetChannel, ChannelID: w.allgemein}, domain.LevelWrite, time.Time{})
	w.versions.Bump()
	w.allow(t, alex, authorize.ActionAgentInvoke, w.invoke(w.allgemein, w.texter, true))
	// Not mentioned, or a personal agent: refused (R-AZ-19).
	w.expect(t, alex, authorize.ActionAgentInvoke, w.invoke(w.allgemein, w.texter, false), forbidden, "agent_not_reachable")
	w.expect(t, alex, authorize.ActionAgentInvoke, w.invoke(w.allgemein, w.assistent, true), forbidden, "agent_not_reachable")
	// Guests need the channel's permission.
	w.setChannel(t, w.allgemein, func(c *authz.ChannelFacts) { c.AllowMentionedAgents = false })
	w.expect(t, alex, authorize.ActionAgentInvoke, w.invoke(w.allgemein, w.texter, true), forbidden, "agent_not_reachable")
}

func TestAgentInvoke_PersonalHolder(t *testing.T) {
	w := newWorld(t)
	w.grant(t, userSubject(w.alex), domain.Target{Kind: domain.TargetChannel, ChannelID: w.notizen}, domain.LevelTrigger, time.Time{})
	w.versions.Bump()
	w.expect(t, w.user(w.alex), authorize.ActionAgentInvoke, w.invoke(w.notizen, w.assistent, true), forbidden, "billing_mode_personal_not_holder")
	w.allow(t, w.user(w.owner), authorize.ActionAgentInvoke, w.invoke(w.notizen, w.assistent, true))
}

func TestAgentInvoke_ToolClassHost(t *testing.T) {
	w := newWorld(t)
	w.grant(t, userSubject(w.alex), domain.Target{Kind: domain.TargetChannel, ChannelID: w.werkzeuge}, domain.LevelTrigger, time.Time{})
	w.versions.Bump()
	w.expect(t, w.user(w.alex), authorize.ActionAgentInvoke, w.invoke(w.werkzeuge, w.runner, true), forbidden, "tool_class_owner_only")
	w.allow(t, w.user(w.owner), authorize.ActionAgentInvoke, w.invoke(w.werkzeuge, w.runner, true))
	// The reported class counts too: the higher of both.
	w.policies.Put(w.lotse, authz.AgentPolicy{BillingMode: domain.BillingOrganization, ToolClass: domain.ToolChat, ReportedToolClass: domain.ToolHost})
	w.expect(t, w.user(w.alex), authorize.ActionAgentInvoke, w.invoke(w.allgemein, w.lotse, true), forbidden, "tool_class_owner_only")
	// A ready own dev environment lifts SR2.
	w.policies.SetDevReady(w.alex, w.allgemein, true)
	w.allow(t, w.user(w.alex), authorize.ActionAgentInvoke, w.invoke(w.allgemein, w.lotse, true))
}

func TestAgentInvoke_ProjectNotReady(t *testing.T) {
	w := newWorld(t)
	w.setChannel(t, w.projektA, func(c *authz.ChannelFacts) { c.ProjectReady = false })
	w.expect(t, w.user(w.owner), authorize.ActionAgentInvoke, w.invoke(w.projektA, w.runner, true), forbidden, "project_not_ready")
}

// TestAgentInvoke_ProjectReadinessFreshInStructureScope: inside one structure scope (the hub
// checks every receiver of an event in one, M1-72) agent.invoke still reads the project facts
// fresh, because a readiness change writes no event that would end the scope (M2-03 design note
// 8); the channel facts of the path come from the memo.
func TestAgentInvoke_ProjectReadinessFreshInStructureScope(t *testing.T) {
	w := newWorld(t)
	ctx := authorize.WithStructureScope(context.Background())
	if d := w.decide(t, ctx, w.user(w.owner), authorize.ActionAgentInvoke, w.invoke(w.projektA, w.runner, true)); !d.Allowed() {
		t.Fatalf("ready binding: %s/%s", d.Outcome, d.Reason)
	}
	w.setChannel(t, w.projektA, func(c *authz.ChannelFacts) { c.ProjectReady = false })
	if d := w.decide(t, ctx, w.user(w.owner), authorize.ActionAgentInvoke, w.invoke(w.projektA, w.runner, true)); d.Reason != "project_not_ready" {
		t.Fatalf("readiness reset inside the scope: %s/%s", d.Outcome, d.Reason)
	}
	w.setChannel(t, w.projektA, func(c *authz.ChannelFacts) { c.ProjectBound, c.ProjectReady = false, false })
	if d := w.decide(t, ctx, w.user(w.owner), authorize.ActionAgentInvoke, w.invoke(w.projektA, w.runner, true)); !d.Allowed() {
		t.Fatalf("binding removed inside the scope: %s/%s", d.Outcome, d.Reason)
	}
}

func TestAgentInvoke_DevEnvironmentNotReady(t *testing.T) {
	w := newWorld(t)
	w.policies.Put(w.runner, authz.AgentPolicy{BillingMode: domain.BillingOrganization, ToolClass: domain.ToolScoped})
	w.expect(t, w.user(w.robin), authorize.ActionAgentInvoke, w.invoke(w.projektA, w.runner, true), forbidden, "dev_environment_not_ready")
	w.allow(t, w.user(w.owner), authorize.ActionAgentInvoke, w.invoke(w.projektA, w.runner, true))
	w.policies.SetDevReady(w.robin, w.projektA, true)
	w.allow(t, w.user(w.robin), authorize.ActionAgentInvoke, w.invoke(w.projektA, w.runner, true))
}

// TestAZM15 is AZ-M15 with the host tool class of Runner: SR2 decides before SR3.
func TestAZM15(t *testing.T) {
	w := newWorld(t)
	w.expect(t, w.user(w.robin), authorize.ActionAgentInvoke, w.invoke(w.projektA, w.runner, true), forbidden, "tool_class_owner_only")
}

func TestAgentInvoke_ActiveRunOfOtherAgent(t *testing.T) {
	w := newWorld(t)
	r := w.invoke(w.allgemein, w.lotse, true)
	r.Invoke.ActiveRunAgentID = w.texter
	w.expect(t, w.user(w.alex), authorize.ActionAgentInvoke, r, forbidden, "run_active_other_agent")
	r.Invoke.ActiveRunAgentID = w.lotse
	w.allow(t, w.user(w.alex), authorize.ActionAgentInvoke, r)
}

func TestAgentInvoke_WithoutDescriptorFailsClosed(t *testing.T) {
	w := newWorld(t)
	if d, err := w.az.Authorize(context.Background(), w.user(w.alex), authorize.ActionAgentInvoke, w.thread(w.allgemein)); err == nil || d.Allowed() {
		t.Fatalf("got %+v, %v", d, err)
	}
}

func TestGrantCreateGuard(t *testing.T) {
	w := newWorld(t)
	inst := authorize.Resource{Kind: authorize.ResourceInstance, Grant: &authorize.GrantFacts{SubjectKind: "user", Level: "read", TargetKind: "instance"}}
	w.allow(t, w.user(w.owner), authorize.ActionGrantCreate, inst)
	w.grant(t, userSubject(w.alex), domain.Target{Kind: domain.TargetInstance}, domain.LevelRead, time.Time{}, domain.CapManage)
	w.grant(t, userSubject(w.robin), domain.Target{Kind: domain.TargetSpace, SpaceID: w.support}, domain.LevelWrite, time.Time{}, domain.CapManage)
	w.versions.Bump()
	w.expect(t, w.user(w.alex), authorize.ActionGrantCreate, inst, forbidden, "owner_only")
	// AZ-M37: write and manage cannot hand out trigger.
	ch := authorize.Resource{Kind: authorize.ResourceChannel, ID: w.allgemein, Grant: &authorize.GrantFacts{
		SubjectKind: "user", Level: "trigger", TargetKind: "channel", TargetChannelID: w.allgemein}}
	w.expect(t, w.user(w.robin), authorize.ActionGrantCreate, ch, forbidden, "delegation_exceeds_own_level")
	ch.Grant.Level = "read"
	w.allow(t, w.user(w.robin), authorize.ActionGrantCreate, ch)
	// Without manage the requirement fails first.
	w.expect(t, w.user(w.sam), authorize.ActionGrantCreate, ch, forbidden, "owner_only")
	// The grant's target must be the resource.
	ch.Grant.TargetChannelID = w.eskalation
	if d, err := w.az.Authorize(context.Background(), w.user(w.robin), authorize.ActionGrantCreate, ch); err == nil || d.Allowed() {
		t.Fatalf("mismatching target: %+v, %v", d, err)
	}
}

func TestGrantRevokeGuard(t *testing.T) {
	w := newWorld(t)
	strong := w.grant(t, userSubject(w.sam), domain.Target{Kind: domain.TargetChannel, ChannelID: w.allgemein}, domain.LevelTrigger, time.Time{})
	weak := w.grant(t, userSubject(w.sam), domain.Target{Kind: domain.TargetChannel, ChannelID: w.eskalation}, domain.LevelRead, time.Time{})
	w.grant(t, userSubject(w.robin), domain.Target{Kind: domain.TargetSpace, SpaceID: w.support}, domain.LevelWrite, time.Time{}, domain.CapManage)
	w.versions.Bump()
	g := func(id ids.ID) authorize.Resource { return authorize.Resource{Kind: authorize.ResourceGrant, ID: id} }
	w.expect(t, w.user(w.robin), authorize.ActionGrantRevoke, g(strong), forbidden, "delegation_exceeds_own_level")
	w.allow(t, w.user(w.robin), authorize.ActionGrantRevoke, g(weak))
	w.allow(t, w.user(w.owner), authorize.ActionGrantRevoke, g(strong))
	// Sam reads Marketing but holds no manage there: the grant is invisible (brief M1-48).
	w.expect(t, w.user(w.sam), authorize.ActionGrantRevoke, g(w.g2), notFound, "no_grant")
	w.expect(t, w.user(w.alex), authorize.ActionGrantRevoke, g(w.g2), notFound, "no_grant")
	w.expect(t, w.user(w.owner), authorize.ActionGrantRevoke, g(ids.New()), notFound, "no_grant")
}

// TestSpaceReadThroughAChannel: on a space, Level(read) means "space visible" (07 section 4.3):
// read on one of its channels is enough (found by AZ-P8).
func TestSpaceReadThroughAChannel(t *testing.T) {
	w := newWorld(t)
	w.allow(t, w.user(w.robin), authorize.ActionSpaceRead, authorize.Resource{Kind: authorize.ResourceSpace, ID: w.engineering})
	w.allow(t, w.agent(w.texter), authorize.ActionSpaceRead, authorize.Resource{Kind: authorize.ResourceSpace, ID: w.marketing})
	w.expect(t, w.user(w.robin), authorize.ActionSpaceRead, authorize.Resource{Kind: authorize.ResourceSpace, ID: w.support}, notFound, "no_grant")
}

// TestViewAs_OwnDMOfTheViewerStaysInvisible: "view as" never opens a dm the viewed person does not
// take part in, also when the viewer does (SR6; red team M1-28 minor 2).
func TestViewAs_OwnDMOfTheViewerStaysInvisible(t *testing.T) {
	w := newWorld(t)
	own := ids.New()
	w.structure.PutChannel(authz.ChannelFacts{ID: own, Kind: authz.ChannelDM, DMParticipants: []ids.ID{w.owner, w.alex}})
	w.addMainThread(own)
	asSam := w.user(w.owner)
	asSam.ViewAs = &actor.ViewAs{UserID: w.sam, StartedAt: w.clk.Now()}
	w.expect(t, asSam, authorize.ActionMessageRead, w.msg(own, w.alex), notFound, "no_grant")
	w.expect(t, asSam, authorize.ActionDmLeave, w.channel(own), notFound, "no_grant")
	asAlex := w.user(w.owner)
	asAlex.ViewAs = &actor.ViewAs{UserID: w.alex, StartedAt: w.clk.Now()}
	w.allow(t, asAlex, authorize.ActionMessageRead, w.msg(own, w.alex))
}

// TestSelf_IDOfThePrincipalResource: a principal or user resource that names the principal in ID
// only is the principal's own (red team M1-28 minor 3).
func TestSelf_IDOfThePrincipalResource(t *testing.T) {
	w := newWorld(t)
	w.allow(t, w.user(w.alex), authorize.ActionSelfRead, authorize.Resource{Kind: authorize.ResourcePrincipal, ID: w.alex})
	w.allow(t, w.agent(w.lotse), authorize.ActionSelfRead, authorize.Resource{Kind: authorize.ResourcePrincipal, ID: w.lotse})
	w.expect(t, w.agent(w.lotse), authorize.ActionSelfRead, authorize.Resource{Kind: authorize.ResourcePrincipal, ID: w.texter}, notFound, "no_grant")
}

// TestTokenCreateGuard_NotSelf: token.create gives a personal token only to the person itself
// (brief M1-44, guard of 07 section 4.2 step 9): an owner in a session with a fresh step-up may
// create tokens for agents and for itself, never for another person (not_self); a person gets
// not_self for another person and owner_only for an agent; an unknown principal passes the guard,
// so the use case answers 404.
func TestTokenCreateGuard_NotSelf(t *testing.T) {
	w := newWorld(t)
	principal := func(id ids.ID) authorize.Resource {
		return authorize.Resource{Kind: authorize.ResourcePrincipal, ID: id, OwnerID: id}
	}
	owner := w.user(w.owner)
	owner.StepUpAt = w.clk.Now()
	w.allow(t, owner, authorize.ActionTokenCreate, principal(w.lotse))
	w.allow(t, owner, authorize.ActionTokenCreate, principal(w.owner))
	w.allow(t, owner, authorize.ActionTokenCreate, principal(ids.New()))
	w.expect(t, owner, authorize.ActionTokenCreate, principal(w.alex), forbidden, "not_self")
	sam := w.user(w.sam)
	sam.StepUpAt = w.clk.Now()
	w.allow(t, sam, authorize.ActionTokenCreate, principal(w.sam))
	w.expect(t, sam, authorize.ActionTokenCreate, principal(w.alex), forbidden, "not_self")
	w.expect(t, sam, authorize.ActionTokenCreate, principal(w.lotse), forbidden, "owner_only")
}

// TestConditionalStepUp: passkey.register needs step-up only once the person has an active
// passkey (07 section 4.2 step 8, hat_anmeldedaten; M1-40); a human with a session stays required.
func TestConditionalStepUp(t *testing.T) {
	w := newWorld(t)
	self := authorize.Resource{Kind: authorize.ResourceUser, ID: w.alex, OwnerID: w.alex}
	alex := w.user(w.alex)
	alex.StepUpAt = time.Time{}
	// First passkey: no step-up.
	w.allow(t, alex, authorize.ActionPasskeyRegister, self)
	// A passkey exists: step-up, and a fresh one passes.
	w.principals.SetPasskey(w.alex, true)
	w.expect(t, alex, authorize.ActionPasskeyRegister, self, stepUpNeeded, "step_up_required")
	alex.StepUpAt = w.clk.Now()
	w.allow(t, alex, authorize.ActionPasskeyRegister, self)
	// Without a session (a personal token) step-up is impossible, with or without a passkey.
	tok := actor.Actor{ID: w.alex, Kind: actor.KindUser, TokenID: ids.New()}
	w.creds.PutToken(tok.TokenID, fakeToken(w.alex))
	w.expect(t, tok, authorize.ActionPasskeyRegister, self, forbidden, "step_up_not_possible")
	w.principals.SetPasskey(w.alex, false)
	w.expect(t, tok, authorize.ActionPasskeyRegister, self, forbidden, "step_up_not_possible")
	// passkey.revoke has step-up "ja": always required.
	stale := w.user(w.alex)
	stale.StepUpAt = time.Time{}
	w.expect(t, stale, authorize.ActionPasskeyRevoke, authorize.Resource{Kind: authorize.ResourcePasskey, ID: ids.New(), OwnerID: w.alex},
		stepUpNeeded, "step_up_required")
	// A failing source fails the call, never an allow.
	w.principals.Fail(true)
	if _, err := w.az.Authorize(context.Background(), alex, authorize.ActionPasskeyRegister, self); err == nil {
		t.Fatal("source error ignored")
	}
}

// TestDelegationBeforeStepUp (07 section 4.2 step 6, brief M1-48 scope 3): the delegation reasons
// of grant.create and grant.revoke are part of the requirement, so a caller without a fresh
// step-up learns them before step_up_required.
func TestDelegationBeforeStepUp(t *testing.T) {
	w := newWorld(t)
	w.grant(t, userSubject(w.robin), domain.Target{Kind: domain.TargetSpace, SpaceID: w.support}, domain.LevelWrite, time.Time{}, domain.CapManage)
	robin := w.user(w.robin)
	robin.StepUpAt = time.Time{}
	create := func(level string, caps ...string) authorize.Resource {
		return authorize.Resource{Kind: authorize.ResourceSpace, ID: w.support, Grant: &authorize.GrantFacts{
			SubjectKind: "user", Level: level, Caps: caps, TargetKind: "space", TargetSpaceID: w.support}}
	}
	w.expect(t, robin, authorize.ActionGrantCreate, create("trigger"), forbidden, "delegation_exceeds_own_level")
	w.expect(t, robin, authorize.ActionGrantCreate, create("read", "export"), forbidden, "delegation_exceeds_own_capabilities")
	agent := create("read")
	agent.Grant.SubjectKind = "agent"
	w.expect(t, robin, authorize.ActionGrantCreate, agent, forbidden, "subject_kind_not_allowed")
	// Within the limits the step-up is what is missing.
	w.expect(t, robin, authorize.ActionGrantCreate, create("write"), stepUpNeeded, "step_up_required")
	// Revoking the owner's stronger grant: the delegation reason first, too (AZ-M39).
	strong := w.grant(t, userSubject(w.sam), domain.Target{Kind: domain.TargetSpace, SpaceID: w.support}, domain.LevelTrigger, time.Time{})
	w.expect(t, robin, authorize.ActionGrantRevoke, authorize.Resource{Kind: authorize.ResourceGrant, ID: strong}, forbidden, "delegation_exceeds_own_level")
}

// TestGrantVisibleOnlyWithManage (brief M1-48 "Visibility"): a grant is visible to owners and to
// holders of manage on its target (a channel inherits manage from its space); reading the target
// is not enough, and instance grants are owner only.
func TestGrantVisibleOnlyWithManage(t *testing.T) {
	w := newWorld(t)
	onSpace := w.grant(t, userSubject(w.sam), domain.Target{Kind: domain.TargetSpace, SpaceID: w.support}, domain.LevelRead, time.Time{})
	onChannel := w.grant(t, userSubject(w.sam), domain.Target{Kind: domain.TargetChannel, ChannelID: w.allgemein}, domain.LevelRead, time.Time{})
	onInstance := w.grant(t, userSubject(w.sam), domain.Target{Kind: domain.TargetInstance}, domain.LevelRead, time.Time{})
	revoke := func(id ids.ID) authorize.Resource { return authorize.Resource{Kind: authorize.ResourceGrant, ID: id} }
	// Alex reads Support (G-1, trigger) but has no manage: the grants on it are invisible.
	w.expect(t, w.user(w.alex), authorize.ActionGrantRevoke, revoke(onSpace), notFound, "no_grant")
	w.expect(t, w.user(w.alex), authorize.ActionGrantRevoke, revoke(onChannel), notFound, "no_grant")
	// Robin with manage on Support sees both, the channel through its space, but not the instance grant.
	w.grant(t, userSubject(w.robin), domain.Target{Kind: domain.TargetSpace, SpaceID: w.support}, domain.LevelTrigger, time.Time{}, domain.CapManage)
	w.allow(t, w.user(w.robin), authorize.ActionGrantRevoke, revoke(onSpace))
	w.allow(t, w.user(w.robin), authorize.ActionGrantRevoke, revoke(onChannel))
	w.expect(t, w.user(w.robin), authorize.ActionGrantRevoke, revoke(onInstance), notFound, "no_grant")
	w.allow(t, w.user(w.owner), authorize.ActionGrantRevoke, revoke(onInstance))
}

// TestAdminScope (M1-48): the owner role, the manage scope through a space grant (its channels
// included) and through a channel grant alone, an app refused by the scope rule and an unknown
// action refused.
func TestAdminScope(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	sc, err := w.az.AdminScope(ctx, w.user(w.owner), authorize.ActionGrantRead)
	if err != nil || !sc.Owner || len(sc.ManageSpaces) != 0 {
		t.Fatalf("owner scope %+v, %v", sc, err)
	}
	w.grant(t, userSubject(w.robin), domain.Target{Kind: domain.TargetSpace, SpaceID: w.support}, domain.LevelRead, time.Time{}, domain.CapManage)
	w.grant(t, userSubject(w.robin), domain.Target{Kind: domain.TargetChannel, ChannelID: w.kampagnen}, domain.LevelRead, time.Time{}, domain.CapManage)
	w.versions.Bump()
	sc, err = w.az.AdminScope(ctx, w.user(w.robin), authorize.ActionGrantRead)
	if err != nil || sc.Owner || len(sc.ManageSpaces) != 1 || sc.ManageSpaces[0] != w.support ||
		!slices.Contains(sc.ManageChannels, w.allgemein) || !slices.Contains(sc.ManageChannels, w.eskalation) ||
		!slices.Contains(sc.ManageChannels, w.kampagnen) || slices.Contains(sc.ManageChannels, w.notizen) {
		t.Fatalf("Robin's scope %+v, %v", sc, err)
	}
	if sc, err = w.az.AdminScope(ctx, w.user(w.alex), authorize.ActionGroupRead); err != nil || sc.Owner || len(sc.ManageSpaces) != 0 {
		t.Fatalf("Alex's scope %+v, %v", sc, err)
	}
	if _, err := w.az.AdminScope(ctx, w.app(), authorize.ActionGrantRead); err == nil {
		t.Fatal("an app got the scope of grant.read")
	}
	if _, err := w.az.AdminScope(ctx, w.user(w.owner), authorize.ActionMessageRead); err == nil {
		t.Fatal("AdminScope served message.read")
	}
}

// TestLocalCredentialStepUpOnlyWhenEstablished: step-up "conditional" of local_credential.set (05
// 2.4, 07 section 4.2 step 8, M1-39): without an established password path a session needs no
// step-up; with one it does; a personal token is never enough; the source is asked for the
// resource's person; a source error never allows.
func TestLocalCredentialStepUpOnlyWhenEstablished(t *testing.T) {
	w := newWorld(t)
	self := authorize.Resource{Kind: authorize.ResourceUser, ID: w.alex, OwnerID: w.alex}
	stale := w.user(w.alex)
	stale.StepUpAt = time.Time{}
	w.allow(t, stale, authorize.ActionLocalCredentialSet, self)
	w.addToken(w.alex)
	personal := actor.Actor{ID: w.alex, Kind: actor.KindUser, TokenID: w.tokens[w.alex]}
	w.expect(t, personal, authorize.ActionLocalCredentialSet, self, forbidden, "step_up_not_possible")
	// Another person's established path does not count for Alex.
	w.principals.SetLocalCredential(w.sam, true)
	w.allow(t, stale, authorize.ActionLocalCredentialSet, self)
	w.principals.SetLocalCredential(w.alex, true)
	w.expect(t, stale, authorize.ActionLocalCredentialSet, self, stepUpNeeded, "step_up_required")
	w.allow(t, w.user(w.alex), authorize.ActionLocalCredentialSet, self)
	w.expect(t, personal, authorize.ActionLocalCredentialSet, self, forbidden, "step_up_not_possible")
	// passkey.register keeps its own condition (M1-40, INT-W18/2): a password path is no passkey.
	w.allow(t, stale, authorize.ActionPasskeyRegister, self)
	w.principals.SetLocalCredential(w.alex, false)
	// A failing source is an error, never an allow.
	w.principals.Fail(true)
	if _, err := w.az.Authorize(context.Background(), stale, authorize.ActionLocalCredentialSet, self); err == nil {
		t.Fatal("a source error allowed")
	}
}
