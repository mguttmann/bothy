// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"os"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// TestScope_AgentConfigRead: Scope of agent.config.read (M2-02 design note 9a). An agent gets
// exactly its operated channels that are not archived and their spaces, nothing fully visible or
// instance-wide; a person, an owner and an app get forbidden subject_kind_not_allowed; the check
// log holds one entry; an agent acting for a person gets the intersection with that person's
// reads.
func TestScope_AgentConfigRead(t *testing.T) {
	w := newWorld(t)
	ctx := authorize.WithCheckLog(context.Background())
	sc, err := w.az.Scope(ctx, w.agent(w.lotse), authorize.ActionAgentConfigRead)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(sc.ChannelIDs, domain.SortIDs([]ids.ID{w.allgemein, w.eskalation})) || !slices.Equal(sc.SpaceIDs, []ids.ID{w.support}) ||
		len(sc.FullyVisibleSpaceIDs) != 0 || sc.InstanceWide {
		t.Fatalf("Lotse scope %+v", sc)
	}
	if got := authorize.Checks(ctx); !slices.Equal(got, []authorize.Check{{Action: "agent.config.read", Kind: authorize.CheckScope, Allowed: true}}) {
		t.Fatalf("checks %+v", got)
	}
	w.setChannel(t, w.eskalation, func(c *authz.ChannelFacts) { c.Archived = true })
	sc, err = w.az.Scope(context.Background(), w.agent(w.lotse), authorize.ActionAgentConfigRead)
	if err != nil || !slices.Equal(sc.ChannelIDs, []ids.ID{w.allgemein}) {
		t.Fatalf("archived operated channel listed: %+v %v", sc, err)
	}
	if d := w.decide(t, context.Background(), w.agent(w.lotse), authorize.ActionAgentConfigRead, w.channel(w.eskalation)); !d.Allowed() {
		t.Fatalf("the operator reads its archived channel: %+v", d)
	}
	for name, p := range map[string]authorize.Principal{"person": w.user(w.alex), "owner": w.user(w.owner), "app": w.app()} {
		_, err := w.az.Scope(context.Background(), p, authorize.ActionAgentConfigRead)
		e, ok := errs.As(err)
		if !ok || e.Code != errs.CodeForbidden || e.Reason != string(domain.ReasonSubjectKindNotAllowed) {
			t.Errorf("%s: %v, want forbidden subject_kind_not_allowed", name, err)
		}
	}
	w.setChannel(t, w.eskalation, func(c *authz.ChannelFacts) { c.Archived = false })
	lotse := w.agent(w.lotse)
	lotse.ActingFor = actor.ActingFor{UserID: w.sam}
	sc, err = w.az.Scope(context.Background(), lotse, authorize.ActionAgentConfigRead)
	if err != nil || !slices.Equal(sc.ChannelIDs, []ids.ID{w.allgemein}) {
		t.Fatalf("Lotse acting for Sam: %+v %v, want only Support/Allgemein", sc, err)
	}
}

// TestAZ_P8_ScopeAgentConfigRead: the property test of AZ-P8 for the list of agent.config.read
// (M2-02): in random worlds and after random mutations the Scope set of every live agent equals
// the channels on which Authorize(agent.config.read) allows (the reference worlds have no
// archived channels). A person gets forbidden.
func TestAZ_P8_ScopeAgentConfigRead(t *testing.T) {
	seed := uint64(time.Now().UnixNano())
	if s := os.Getenv("BOTHY_TEST_SEED"); s != "" { //nolint:forbidigo // test input (seed), not server configuration
		n, err := strconv.ParseUint(s, 10, 64)
		if err != nil {
			t.Fatal(err)
		}
		seed = n
	}
	for w := range 10 {
		ws := seed + uint64(w)
		m := newRefWorld(t, ws)
		var trail []string
		for step := range 30 {
			trail = append(trail, m.mutate())
			for _, p := range m.principals {
				if p.disabled {
					continue
				}
				a := m.actorOf(p)
				sc, err := m.az.Scope(context.Background(), a, authorize.ActionAgentConfigRead)
				if p.kind != domain.KindAgent {
					if errs.KindOf(err) != errs.KindForbidden {
						t.Fatalf("seed %d, world %d, step %d, %s: scope %v, want forbidden", ws, w, step, p.kind, err)
					}
					continue
				}
				var want []ids.ID
				for _, c := range m.channels {
					d, err := m.az.Authorize(context.Background(), a, authorize.ActionAgentConfigRead, authorize.Resource{Kind: authorize.ResourceChannel, ID: c.id})
					if err != nil {
						t.Fatal(err)
					}
					if d.Allowed() {
						want = append(want, c.id)
					}
				}
				want = domain.SortIDs(want)
				if err != nil || !slices.Equal(sc.ChannelIDs, want) {
					t.Fatalf("seed %d (BOTHY_TEST_SEED=%d), world %d, step %d: scope %v %v, want %v\ntrail %v", seed, ws, w, step, sc.ChannelIDs, err, want, trail)
				}
			}
		}
	}
}
