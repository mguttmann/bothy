// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// sessionSwitch is the descriptor of agent_session.reset and agent_session.bind on thread th
// (brief M2-12 design note 5): the thread with the invoke facts of a session switch, which names
// the thread's operator agent explicitly and starts no run.
func sessionSwitch(th, agent ids.ID) authorize.Resource {
	return authorize.Resource{Kind: authorize.ResourceThread, ID: th,
		Invoke: &authorize.InvokeFacts{AgentID: agent, TriggerOrigin: "human", MentionsAgent: true}}
}

// sessionRequirementCases runs the cases the brief M2-12 lists for both session switches in the
// world of 07 section 10: level trigger plus the agent.invoke guard (05 section 4.6).
func sessionRequirementCases(t *testing.T, act authorize.Action) {
	t.Helper()
	w := newWorld(t)
	allgemein := sessionSwitch(w.mainThread[w.allgemein], w.lotse)
	t.Run("owner", func(t *testing.T) {
		w.allow(t, w.user(w.owner), act, allgemein)
	})
	t.Run("trigger on Support", func(t *testing.T) {
		w.allow(t, w.user(w.alex), act, allgemein)
	})
	t.Run("only read", func(t *testing.T) {
		// Sam reads Allgemein through the group Leser (G-4).
		w.expect(t, w.user(w.sam), act, allgemein, forbidden, "level_insufficient")
	})
	t.Run("no read", func(t *testing.T) {
		w.expect(t, w.user(w.robin), act, allgemein, notFound, "no_grant")
		w.expect(t, w.user(w.robin), act, sessionSwitch(ids.New(), w.lotse), notFound, "no_grant")
	})
	t.Run("agent or app", func(t *testing.T) {
		w.expect(t, w.agent(w.lotse), act, allgemein, forbidden, "level_insufficient")
		w.expect(t, w.app(), act, sessionSwitch(w.mainThread[w.eskalation], w.lotse), forbidden, "level_insufficient")
	})
	t.Run("tool class host", func(t *testing.T) {
		// AZ-M20: a trigger grant on Werkzeuge does not reach Runner with tool class host.
		w.grant(t, userSubject(w.alex), domain.Target{Kind: domain.TargetChannel, ChannelID: w.werkzeuge}, domain.LevelTrigger, time.Time{})
		w.versions.Bump()
		w.expect(t, w.user(w.alex), act, sessionSwitch(w.mainThread[w.werkzeuge], w.runner), forbidden, "tool_class_owner_only")
		w.allow(t, w.user(w.owner), act, sessionSwitch(w.mainThread[w.werkzeuge], w.runner))
	})
	t.Run("personal agent", func(t *testing.T) {
		w.grant(t, userSubject(w.alex), domain.Target{Kind: domain.TargetChannel, ChannelID: w.notizen}, domain.LevelTrigger, time.Time{})
		w.versions.Bump()
		w.expect(t, w.user(w.alex), act, sessionSwitch(w.mainThread[w.notizen], w.assistent), forbidden, "billing_mode_personal_not_holder")
		w.allow(t, w.user(w.owner), act, sessionSwitch(w.mainThread[w.notizen], w.assistent))
	})
	t.Run("thread without operator", func(t *testing.T) {
		th := ids.New()
		w.structure.PutThread(authz.ThreadFacts{ID: th, ChannelID: w.allgemein, CreatedBy: w.alex})
		w.expect(t, w.user(w.alex), act, sessionSwitch(th, ids.ID{}), forbidden, "agent_not_reachable")
	})
	t.Run("without invoke facts", func(t *testing.T) {
		if d, err := w.az.Authorize(t.Context(), w.user(w.alex), act, w.thread(w.allgemein)); err == nil || d.Allowed() {
			t.Fatalf("got %+v, %v", d, err)
		}
	})
	t.Run("read-only channel", func(t *testing.T) {
		w.setChannel(t, w.allgemein, func(c *authz.ChannelFacts) { c.ReadOnly = true })
		w.expect(t, w.user(w.alex), act, allgemein, forbidden, "channel_read_only")
	})
}

// TestAgentSessionResetRequirement checks agent_session.reset (brief M2-12 design note 5).
func TestAgentSessionResetRequirement(t *testing.T) {
	sessionRequirementCases(t, authorize.ActionAgentSessionReset)
}

// TestAgentSessionBindRequirement checks agent_session.bind without a second target: the
// reference is current nowhere (TargetThreadID zero).
func TestAgentSessionBindRequirement(t *testing.T) {
	sessionRequirementCases(t, authorize.ActionAgentSessionBind)
}

// TestAgentSessionBindTarget checks the second target of agent_session.bind (07 section 4.2
// "Actions with two targets", 09 9.4 point 1): the thread in which the reference is current must
// be visible, and the same requirement holds on its path with the same invoke facts. A denial on
// either path is audited once.
func TestAgentSessionBindTarget(t *testing.T) {
	w := newWorld(t)
	// A thread in Eskalation whose fixed operator is Lotse, like the thread of the binding.
	other := ids.New()
	w.structure.PutThread(authz.ThreadFacts{ID: other, ChannelID: w.eskalation, CreatedBy: w.alex, OperatorAgentID: w.lotse})
	bind := func(th, target, agent ids.ID) authorize.Resource {
		r := sessionSwitch(th, agent)
		r.TargetThreadID = target
		return r
	}
	allgemein := w.mainThread[w.allgemein]
	t.Run("invisible target", func(t *testing.T) {
		w.expect(t, w.user(w.alex), authorize.ActionAgentSessionBind, bind(allgemein, w.mainThread[w.kampagnen], w.lotse), notFound, "no_grant")
		w.expect(t, w.user(w.alex), authorize.ActionAgentSessionBind, bind(allgemein, ids.New(), w.lotse), notFound, "no_grant")
		w.expect(t, w.user(w.owner), authorize.ActionAgentSessionBind, bind(allgemein, w.mainThread[w.dm], w.lotse), notFound, "no_grant")
	})
	t.Run("only read on the target", func(t *testing.T) {
		// Sam triggers in Marketing (G-2) and reads Allgemein only (G-4).
		w.expect(t, w.user(w.sam), authorize.ActionAgentSessionBind, bind(w.mainThread[w.kampagnen], allgemein, w.texter), forbidden, "level_insufficient")
	})
	t.Run("read-only target", func(t *testing.T) {
		w.setChannel(t, w.eskalation, func(c *authz.ChannelFacts) { c.ReadOnly = true })
		w.expect(t, w.user(w.alex), authorize.ActionAgentSessionBind, bind(allgemein, other, w.lotse), forbidden, "channel_read_only")
		w.setChannel(t, w.eskalation, func(c *authz.ChannelFacts) { c.ReadOnly = false })
	})
	t.Run("guard fails on the target", func(t *testing.T) {
		w.setChannel(t, w.eskalation, func(c *authz.ChannelFacts) { c.ProjectBound, c.ProjectReady = true, false })
		before, _ := w.rec.counts()
		w.expect(t, w.user(w.alex), authorize.ActionAgentSessionBind, bind(allgemein, other, w.lotse), forbidden, "project_not_ready")
		after, _ := w.rec.counts()
		if after != before+1 {
			t.Fatalf("denials audited: %d, want 1", after-before)
		}
		w.rec.mu.Lock()
		last := w.rec.denials[len(w.rec.denials)-1]
		w.rec.mu.Unlock()
		if last.Action != string(authorize.ActionAgentSessionBind) || string(last.Reason) != "project_not_ready" {
			t.Fatalf("denial %+v", last)
		}
		w.setChannel(t, w.eskalation, func(c *authz.ChannelFacts) { c.ProjectBound, c.ProjectReady = false, false })
	})
	t.Run("guard fails on the source", func(t *testing.T) {
		th := ids.New()
		w.structure.PutThread(authz.ThreadFacts{ID: th, ChannelID: w.allgemein, CreatedBy: w.alex})
		w.expect(t, w.user(w.alex), authorize.ActionAgentSessionBind, bind(th, other, ids.ID{}), forbidden, "agent_not_reachable")
	})
	t.Run("both paths fine", func(t *testing.T) {
		w.allow(t, w.user(w.alex), authorize.ActionAgentSessionBind, bind(allgemein, other, w.lotse))
		w.allow(t, w.user(w.owner), authorize.ActionAgentSessionBind, bind(allgemein, other, w.lotse))
	})
}

// TestAgentSessionReportRequirement checks agent_session.report (05 section 4.6 agent-self): the
// descriptor names the session of the run, or none yet, with the thread as path and the run's
// agent as owner.
func TestAgentSessionReportRequirement(t *testing.T) {
	w := newWorld(t)
	// Texter may write in Allgemein, so it sees the thread and is refused by the requirement.
	w.grant(t, domain.Subject{Kind: domain.SubjectAgent, PrincipalID: w.texter}, domain.Target{Kind: domain.TargetChannel, ChannelID: w.allgemein}, domain.LevelWrite, time.Time{})
	w.versions.Bump()
	report := func(session ids.ID) authorize.Resource {
		return authorize.Resource{Kind: authorize.ResourceAgentSession, ID: session, ThreadID: w.mainThread[w.allgemein], OwnerID: w.lotse}
	}
	for name, session := range map[string]ids.ID{"no session yet": {}, "session of the run": ids.New()} {
		t.Run(name, func(t *testing.T) {
			w.allow(t, w.agent(w.lotse), authorize.ActionAgentSessionReport, report(session))
			w.expect(t, w.agent(w.texter), authorize.ActionAgentSessionReport, report(session), forbidden, "not_self")
			w.expect(t, w.user(w.alex), authorize.ActionAgentSessionReport, report(session), forbidden, "subject_kind_not_allowed")
		})
	}
}
