// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/measure"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/messaging"
	"github.com/OWNER/bothy/services/server/internal/runs"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// TestMeasureTransition holds the own measurement of the run transitions (25 3.2, M6-13): a start
// records runs_started with the billing mode and a good sli.run_start within the threshold; a
// queued run that fails is a bad sli.run_start and runs_finished failed without a duration; a
// cancelled queued run is no sli.run_start; a completed run records runs_finished with its
// duration and sli.turn_duration; a run failed with billing_mode_mismatch also counts in
// invocations_denied.
func TestMeasureTransition(t *testing.T) {
	t.Parallel()
	ch, ag := ids.New(), ids.New()
	key := measure.Key{ChannelID: ch, AgentID: ag}
	sum := func(used, expected, reason string) runs.RunSummary {
		return runs.RunSummary{ChannelID: ch, AgentID: ag, BillingModeUsed: used, BillingModeExpected: expected, StateReason: reason}
	}
	type rec struct {
		m   measure.Metric
		dim string
		v   measure.Value
	}
	cases := []struct {
		name string
		t    Transitioned
		want []rec
	}{
		{"start", Transitioned{Run: sum("organization", "personal", ""), From: domain.StateQueued, To: domain.StateRunning, QueuedFor: 2 * time.Second},
			[]rec{{measure.RunsStarted, "organization", measure.One()}, {measure.SLIRunStart, "", measure.SLI(2*time.Second, true)}}},
		{"start with the expected mode", Transitioned{Run: sum("", "personal", ""), From: domain.StateQueued, To: domain.StateRunning},
			[]rec{{measure.RunsStarted, "personal", measure.One()}, {measure.SLIRunStart, "", measure.SLI(0, true)}}},
		{"queued run fails", Transitioned{Run: sum("", "personal", "agent_unavailable"), From: domain.StateQueued, To: domain.StateFailed, QueuedFor: time.Minute},
			[]rec{{measure.SLIRunStart, "", measure.SLI(time.Minute, false)}, {measure.RunsFinished, "failed", measure.One()}}},
		{"queued run cancelled", Transitioned{Run: sum("", "personal", "user_cancelled"), From: domain.StateQueued, To: domain.StateCancelled},
			[]rec{{measure.RunsFinished, "cancelled", measure.One()}}},
		{"completed", Transitioned{Run: sum("personal", "personal", ""), From: domain.StateRunning, To: domain.StateCompleted, Started: true, Ran: 90 * time.Second},
			[]rec{{measure.RunsFinished, "completed", measure.Timed(90 * time.Second)}, {measure.SLITurnDuration, "", measure.SLI(90*time.Second, true)}}},
		{"billing mode mismatch", Transitioned{Run: sum("personal", "organization", domain.RunReasonBillingModeMismatch), From: domain.StateRunning,
			To: domain.StateFailed, Started: true, Ran: time.Second},
			[]rec{{measure.RunsFinished, "failed", measure.Timed(time.Second)}, {measure.InvocationsDenied, "billing_mode_mismatch", measure.One()}}},
		{"waiting for input", Transitioned{Run: sum("personal", "personal", ""), From: domain.StateRunning, To: domain.StateWaitingInput}, nil},
	}
	for _, c := range cases {
		r := &measure.Recording{}
		measureTransition(r, c.t)
		got := r.Calls("")
		if len(r.Invalid()) != 0 || len(got) != len(c.want) {
			t.Errorf("%s: calls %+v, invalid %v, want %+v", c.name, got, r.Invalid(), c.want)
			continue
		}
		for i, w := range c.want {
			k := key
			k.Dimension = w.dim
			if got[i] != (measure.Call{Metric: w.m, Key: k, Value: w.v}) {
				t.Errorf("%s: call %d = %+v, want %s %q %+v", c.name, i, got[i], w.m, w.dim, w.v)
			}
		}
	}
}

// TestFinishedToolCalls holds which steps of a patch count in tool_calls: a tool_call that reached
// completed or failed in this patch (also one that arrives finished), with started_at to
// finished_at; not one that was already finished, still runs, ended cancelled or is no tool_call.
func TestFinishedToolCalls(t *testing.T) {
	t.Parallel()
	t0 := time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC)
	a, b, c, d, e, f := ids.New(), ids.New(), ids.New(), ids.New(), ids.New(), ids.New()
	step := func(id ids.ID, kind, status string, ran time.Duration) domain.DocStep {
		s := domain.DocStep{ID: id, Kind: kind, Status: status, StartedAt: t0}
		if ran > 0 {
			s.FinishedAt = t0.Add(ran)
		}
		return s
	}
	before := []domain.DocStep{
		step(a, "tool_call", "running", 0), step(b, "tool_call", "completed", time.Second), step(c, "tool_call", "running", 0),
		step(d, "status", "running", 0),
	}
	after := []domain.DocStep{
		step(a, "tool_call", "completed", 3*time.Second), step(b, "tool_call", "completed", time.Second), step(c, "tool_call", "cancelled", time.Second),
		step(d, "status", "completed", time.Second), step(e, "tool_call", "failed", 500*time.Millisecond), step(f, "tool_call", "running", 0),
	}
	got := finishedToolCalls(before, after)
	want := []finishedToolCall{{status: "completed", duration: 3 * time.Second}, {status: "failed", duration: 500 * time.Millisecond}}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("finishedToolCalls = %+v, want %+v", got, want)
	}
}

// TestApprovalDimension maps the choices of 09 11.5 to the dimensions of approval_cards.
func TestApprovalDimension(t *testing.T) {
	t.Parallel()
	for choice, want := range map[string]string{"once": "approved", "session": "approved", "always": "approved", "deny": "denied", "": "", "maybe": ""} {
		got, ok := approvalDimension(choice)
		if got != want || ok != (want != "") {
			t.Errorf("approvalDimension(%q) = %q, %v", choice, got, ok)
		}
	}
	if approvalCard([]ComponentRow{{RequiredCapability: "upload"}}) || !approvalCard([]ComponentRow{{}, {RequiredCapability: "approve_actions"}}) {
		t.Error("approvalCard does not look at required_capability")
	}
}

// cardPorts answers Info and LockCard of the ports that the approval_cards points read; every
// other method of the embedded interfaces is unused here and would panic.
type cardPorts struct {
	ThreadPort
	CardMessages
	channel ids.ID
	facts   messaging.CardFacts
}

func (p cardPorts) Info(context.Context, ids.ID) (chat.ThreadInfo, error) {
	return chat.ThreadInfo{ChannelID: p.channel}, nil
}

func (p cardPorts) LockCard(context.Context, ids.ID) (messaging.CardFacts, error) {
	return p.facts, nil
}

// TestMeasureApprovalCardsOnlyOfAgents: approval_cards counts the approval cards of agents only
// (25 3.2, D-M6-13-5). An app may post a card with an approval component since M2-25, but it has
// no runs and its interactions never reach the approved or denied points, so neither requested
// nor expired is counted for it (its id is no agent_id of stats_minutes).
func TestMeasureApprovalCardsOnlyOfAgents(t *testing.T) {
	t.Parallel()
	rows := []ComponentRow{{RequiredCapability: domain.CapabilityApproveActions}}
	for kind, want := range map[string]int{string(actor.KindAgent): 2, string(actor.KindApp): 0} {
		ch, author := ids.New(), ids.New()
		rec := &measure.Recording{}
		ports := cardPorts{channel: ch, facts: messaging.CardFacts{ChannelID: ch, AuthorID: author, AuthorKind: kind}}
		s := &Service{d: Deps{Measure: rec, Threads: ports, Cards: ports}}
		u, ctl := txn.NewControlledUnit(txn.NewRegistry(), txn.Options{})
		ctx := txn.WithUnit(context.Background(), u)
		if err := s.measureRequested(ctx, messaging.CardInput{AuthorKind: kind, AuthorID: author, ThreadID: ids.New()}, rows); err != nil {
			t.Fatal(err)
		}
		if err := s.measureExpired(ctx, ids.New(), rows); err != nil {
			t.Fatal(err)
		}
		ctl.Committed()
		if got := rec.Calls(measure.ApprovalCards); len(got) != want || len(rec.Invalid()) != 0 {
			t.Errorf("%s card: approval_cards %+v, invalid %v; want %d calls", kind, got, rec.Invalid(), want)
		}
	}
}
