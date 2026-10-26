// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// recheck is the principal of the check without a request (R-AZ-29, M2-08 design note 4).
func recheck(id ids.ID) actor.Actor { return actor.Recheck(id) }

// TestRecheckPrincipal_OwnerWithoutSession: the owner passes the delivery check of the host agent
// Runner without a session (AZ-M19 at the decision level), a personal token of the owner does not.
func TestRecheckPrincipal_OwnerWithoutSession(t *testing.T) {
	w := newWorld(t)
	w.allow(t, recheck(w.owner), authorize.ActionAgentInvoke, w.invoke(w.werkzeuge, w.runner, true))
	w.addToken(w.owner)
	personal := actor.Actor{ID: w.owner, Kind: actor.KindUser, TokenID: w.tokens[w.owner]}
	w.expect(t, personal, authorize.ActionAgentInvoke, w.invoke(w.werkzeuge, w.runner, true), authorize.OutcomeNotFound, "no_grant")
	// The I2 rule of the personal Assistent: the owner is the holder.
	w.allow(t, recheck(w.owner), authorize.ActionAgentInvoke, w.invoke(w.notizen, w.assistent, true))
}

// TestRecheckPrincipal_DisabledPerson: liveness comes only from the principal status.
func TestRecheckPrincipal_DisabledPerson(t *testing.T) {
	w := newWorld(t)
	w.allow(t, recheck(w.alex), authorize.ActionAgentInvoke, w.invoke(w.allgemein, w.lotse, true))
	w.principals.Put(authz.PrincipalFacts{ID: w.alex, Kind: actor.KindUser, Status: authz.PrincipalDisabled})
	w.versions.Bump()
	w.expect(t, recheck(w.alex), authorize.ActionAgentInvoke, w.invoke(w.allgemein, w.lotse, true), authorize.OutcomeUnauthenticated, "principal_inactive")
	w.principals.Put(authz.PrincipalFacts{ID: w.alex, Kind: actor.KindUser, Status: authz.PrincipalPending})
	w.versions.Bump()
	w.expect(t, recheck(w.alex), authorize.ActionAgentInvoke, w.invoke(w.allgemein, w.lotse, true), forbidden, "principal_pending")
	// The owner who lost the owner role is judged by the grants alone.
	w.principals.Put(authz.PrincipalFacts{ID: w.owner, Kind: actor.KindUser, Status: authz.PrincipalActive})
	w.versions.Bump()
	w.expect(t, recheck(w.owner), authorize.ActionAgentInvoke, w.invoke(w.werkzeuge, w.runner, true), authorize.OutcomeNotFound, "no_grant")
}

// TestRecheckPrincipal_OnlyAgentInvoke: the marker with any action other than agent.invoke,
// command.invoke, interaction.create (M2-25), push.deliver and channel.read (M7-06) is an internal
// error.
func TestRecheckPrincipal_OnlyAgentInvoke(t *testing.T) {
	w := newWorld(t)
	for _, act := range []authorize.Action{authorize.ActionMessageRead, authorize.ActionUserCreate, authorize.ActionRunRead} {
		_, err := w.az.Authorize(context.Background(), recheck(w.owner), act, w.thread(w.allgemein))
		if errs.KindOf(err) != errs.KindInternal {
			t.Fatalf("%s with the re-check principal: err %v, want internal", act, err)
		}
	}
}

// TestRecheckPrincipal_ChannelRead (M7-06): the names rule of the privacy export asks channel.read
// for the person without a request: a channel of a current grant is readable, one without a grant
// is not, and one whose grant expired is not readable any more.
func TestRecheckPrincipal_ChannelRead(t *testing.T) {
	w := newWorld(t)
	w.allow(t, recheck(w.alex), authorize.ActionChannelRead, authorize.Resource{Kind: authorize.ResourceChannel, ID: w.allgemein})
	w.expect(t, recheck(w.alex), authorize.ActionChannelRead, authorize.Resource{Kind: authorize.ResourceChannel, ID: w.kampagnen},
		authorize.OutcomeNotFound, "no_grant")
	w.allow(t, recheck(w.robin), authorize.ActionChannelRead, authorize.Resource{Kind: authorize.ResourceChannel, ID: w.projektA})
	w.clk.Advance(w.g3Expiry.Sub(w.clk.Now()) + time.Second)
	w.versions.Bump()
	w.expect(t, recheck(w.robin), authorize.ActionChannelRead, authorize.Resource{Kind: authorize.ResourceChannel, ID: w.projektA},
		authorize.OutcomeNotFound, "no_grant")
}

// TestRecheckPrincipal_RejectsSessionOrToken: the marker with any credential, view_as, acting_for
// or another kind is an internal error.
func TestRecheckPrincipal_RejectsSessionOrToken(t *testing.T) {
	w := newWorld(t)
	for name, mod := range map[string]func(*actor.Actor){
		"session":    func(a *actor.Actor) { a.SessionID = w.sessions[w.owner] },
		"device":     func(a *actor.Actor) { a.DeviceID = w.devices[w.owner] },
		"token":      func(a *actor.Actor) { a.TokenID = ids.New() },
		"view_as":    func(a *actor.Actor) { a.ViewAs = &actor.ViewAs{UserID: w.alex, StartedAt: w.clk.Now()} },
		"acting_for": func(a *actor.Actor) { a.ActingFor = actor.ActingFor{UserID: w.alex, RunID: ids.New()} },
		"agent":      func(a *actor.Actor) { a.Kind = actor.KindAgent },
	} {
		p := recheck(w.owner)
		mod(&p)
		_, err := w.az.Authorize(context.Background(), p, authorize.ActionAgentInvoke, w.invoke(w.werkzeuge, w.runner, true))
		if errs.KindOf(err) != errs.KindInternal {
			t.Fatalf("%s: err %v, want internal", name, err)
		}
	}
}

// TestRecheckPrincipal_CacheKeySeparate: the owner snapshot of the re-check never reaches the
// principal of a request of the same person, in either order.
func TestRecheckPrincipal_CacheKeySeparate(t *testing.T) {
	w := newWorld(t)
	w.addToken(w.owner)
	personal := actor.Actor{ID: w.owner, Kind: actor.KindUser, TokenID: w.tokens[w.owner]}
	bare := actor.Actor{ID: w.owner, Kind: actor.KindUser}
	r := w.invoke(w.werkzeuge, w.runner, true)
	w.allow(t, recheck(w.owner), authorize.ActionAgentInvoke, r)
	w.expect(t, personal, authorize.ActionAgentInvoke, r, authorize.OutcomeNotFound, "no_grant")
	w.expect(t, bare, authorize.ActionAgentInvoke, r, authorize.OutcomeNotFound, "no_grant")
	w.allow(t, recheck(w.owner), authorize.ActionAgentInvoke, r)
	w.clk.Advance(time.Second)
	w2 := newWorld(t)
	w2.addToken(w2.owner)
	p2 := actor.Actor{ID: w2.owner, Kind: actor.KindUser, TokenID: w2.tokens[w2.owner]}
	w2.expect(t, p2, authorize.ActionAgentInvoke, w2.invoke(w2.werkzeuge, w2.runner, true), authorize.OutcomeNotFound, "no_grant")
	w2.allow(t, recheck(w2.owner), authorize.ActionAgentInvoke, w2.invoke(w2.werkzeuge, w2.runner, true))
}

// TestAgentInvoke_NewThreadUsesMainRule: a new auto thread is decided with the main-thread mention
// rule (05 section 5 point 3 "for new threads"), not with thread_require_mention.
func TestAgentInvoke_NewThreadUsesMainRule(t *testing.T) {
	w := newWorld(t)
	alex := recheck(w.alex)
	th := ids.New()
	w.structure.PutThread(authz.ThreadFacts{ID: th, ChannelID: w.allgemein, CreatedBy: w.alex, OperatorAgentID: w.lotse})
	w.setChannel(t, w.allgemein, func(c *authz.ChannelFacts) { c.ThreadRequireMention, c.RequireMention = true, false })
	r := authorize.Resource{Kind: authorize.ResourceThread, ID: th, Invoke: &authorize.InvokeFacts{AgentID: w.lotse, TriggerOrigin: "human"}}
	w.expect(t, alex, authorize.ActionAgentInvoke, r, forbidden, "agent_not_reachable")
	r.Invoke.NewThread = true
	w.allow(t, alex, authorize.ActionAgentInvoke, r)
	w.setChannel(t, w.allgemein, func(c *authz.ChannelFacts) { c.ThreadRequireMention, c.RequireMention = false, true })
	w.expect(t, alex, authorize.ActionAgentInvoke, r, forbidden, "agent_not_reachable")
}

// TestEventTarget_AudienceAgent (D-M2-08-14): agent.events.consume of a run event with an
// audience is allowed for the audience agent and refused for any other agent; without the
// audience the event has no object the action takes.
func TestEventTarget_AudienceAgent(t *testing.T) {
	w := newWorld(t)
	ctx := authorize.Filter(context.Background())
	r := authorize.ResourceFromEvent("run", ids.New(), w.support, w.allgemein, w.mainThread[w.allgemein], ids.ID{})
	without := r
	r.Event = &authorize.EventScope{SpaceID: w.support, ChannelID: w.allgemein, ThreadID: w.mainThread[w.allgemein], AudienceID: w.lotse}
	if d := w.decide(t, ctx, w.agent(w.lotse), authorize.ActionAgentEventsConsume, r); !d.Allowed() {
		t.Fatalf("audience: %+v", d)
	}
	if d := w.decide(t, ctx, w.agent(w.texter), authorize.ActionAgentEventsConsume, r); d.Allowed() {
		t.Fatalf("another agent: %+v", d)
	}
	if d := w.decide(t, ctx, w.agent(w.lotse), authorize.ActionAgentEventsConsume, without); d.Allowed() {
		t.Fatalf("without the audience: %+v", d)
	}
}
