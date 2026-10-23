// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// followup is the descriptor of run.followup.create for a message of the thread of channel to
// the run of agent: the active run is the agent's own unless activeAgent says otherwise.
func (w *world) followup(channel, agent, activeAgent ids.ID, mention bool) authorize.Resource {
	r := w.invoke(channel, agent, mention)
	r.Invoke.ActiveRunAgentID = activeAgent
	return r
}

// TestRunFollowupCreateRequirement holds the requirement of run.followup.create (05 4.10 "write
// and agent.invoke", M2-11 design note 11): the levels write and trigger plus the guard of
// agent.invoke with the descriptor's facts, so I2, SR4, the read-only channel and the active run
// of another agent refuse; a missing descriptor is an internal error.
func TestRunFollowupCreateRequirement(t *testing.T) {
	w := newWorld(t)
	act := authorize.ActionRunFollowupCreate
	w.grant(t, userSubject(w.robin), domain.Target{Kind: domain.TargetChannel, ChannelID: w.eskalation}, domain.LevelWrite, time.Time{})
	w.grant(t, userSubject(w.sam), domain.Target{Kind: domain.TargetChannel, ChannelID: w.notizen}, domain.LevelTrigger, time.Time{})
	w.versions.Bump()
	alex := w.user(w.alex)

	w.allow(t, alex, act, w.followup(w.allgemein, w.lotse, w.lotse, true))
	w.allow(t, alex, act, w.followup(w.allgemein, w.lotse, w.lotse, false))
	w.expect(t, w.user(w.robin), act, w.followup(w.eskalation, w.lotse, w.lotse, true), forbidden, "level_insufficient")
	w.expect(t, w.user(w.sam), act, w.followup(w.allgemein, w.lotse, w.lotse, true), forbidden, "level_insufficient")
	w.expect(t, alex, act, w.followup(w.kampagnen, w.texter, w.texter, true), notFound, "no_grant")
	w.expect(t, alex, act, w.followup(w.allgemein, w.lotse, w.texter, true), forbidden, "run_active_other_agent")
	// I1 is bypassed in the fake: Sam holds trigger where the owner's personal agent operates.
	w.expect(t, w.user(w.sam), act, w.followup(w.notizen, w.assistent, w.assistent, true), forbidden, "billing_mode_personal_not_holder")
	w.allow(t, w.user(w.owner), act, w.followup(w.notizen, w.assistent, w.assistent, true))
	origin := w.followup(w.allgemein, w.lotse, w.lotse, true)
	origin.Invoke.TriggerOrigin = "agent"
	w.expect(t, alex, act, origin, forbidden, "origin_not_human")

	f, _, _ := w.principals.Principal(context.Background(), w.lotse)
	f.Status = authz.PrincipalDisabled
	w.principals.Put(f)
	w.expect(t, alex, act, w.followup(w.allgemein, w.lotse, w.lotse, true), forbidden, "agent_not_reachable")
	f.Status = authz.PrincipalActive
	w.principals.Put(f)

	w.setChannel(t, w.allgemein, func(c *authz.ChannelFacts) { c.ReadOnly = true })
	w.expect(t, alex, act, w.followup(w.allgemein, w.lotse, w.lotse, true), forbidden, "channel_read_only")
	w.setChannel(t, w.allgemein, func(c *authz.ChannelFacts) { c.ReadOnly = false })

	_, err := w.az.Authorize(context.Background(), alex, act, w.thread(w.allgemein))
	if e, ok := errs.As(err); !ok || e.Code != errs.CodeInternalError {
		t.Fatalf("missing descriptor: %v", err)
	}
}
