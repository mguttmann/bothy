// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/measure"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/messaging"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// The dimension codes of the own measurement the runs module records
// (docs/spec/25-visualization.md 3.2).
const (
	cardRequested     = "requested"
	cardApproved      = "approved"
	cardDenied        = "denied"
	cardExpired       = "expired"
	deniedI2          = "i2_denied"
	deniedModeMissing = "billing_mode_mismatch"
)

// measureTransition records the own measurement of one committed transition t next to
// RunTransitioned, with the same notion of the event (25 3.2, docs/DECISIONS.md D-M6-13-4):
// leaving queued is sli.run_start (cancelled runs are not counted, slos.md 2; good when it started
// within the threshold) and, into running, runs_started with the billing mode as
// bothy.run.start.latency takes it; an end state is runs_finished with the run duration of a
// started run, a completed run sli.turn_duration, and a run failed with billing_mode_mismatch
// invocations_denied (I3).
func measureTransition(rec measure.Recorder, t Transitioned) {
	key := measure.Key{ChannelID: t.Run.ChannelID, AgentID: t.Run.AgentID}
	if t.From == domain.StateQueued {
		mode := t.Run.BillingModeUsed
		if mode == "" {
			mode = t.Run.BillingModeExpected
		}
		if t.To == domain.StateRunning && mode != "" {
			rec.Record(measure.RunsStarted, withDimension(key, mode), measure.One())
		}
		if t.To != domain.StateCancelled {
			rec.Record(measure.SLIRunStart, key, measure.SLI(t.QueuedFor, t.To == domain.StateRunning))
		}
	}
	if !t.To.Ended() {
		return
	}
	v := measure.One()
	if t.Started {
		v = measure.Timed(t.Ran)
	}
	rec.Record(measure.RunsFinished, withDimension(key, string(t.To)), v)
	if t.To == domain.StateCompleted {
		rec.Record(measure.SLITurnDuration, key, measure.SLI(t.Ran, true))
	}
	if t.To == domain.StateFailed && t.Run.StateReason == domain.RunReasonBillingModeMismatch {
		rec.Record(measure.InvocationsDenied, withDimension(key, deniedModeMissing), measure.One())
	}
}

// withDimension returns k with the dimension d.
func withDimension(k measure.Key, d string) measure.Key {
	k.Dimension = d
	return k
}

// measureDenied records one I2 denial before a delivery in invocations_denied, next to the
// counter bothy.agent.invocation.denied (25 3.2).
func (s *Service) measureDenied(channel, agent ids.ID) {
	s.d.Measure.Record(measure.InvocationsDenied, measure.Key{ChannelID: channel, AgentID: agent, Dimension: deniedI2}, measure.One())
}

// measureCard registers dimension of approval_cards for the card of agent in channel for after
// the commit of the unit of work of ctx (25 3.2, docs/DECISIONS.md D-M6-13-5).
func (s *Service) measureCard(ctx context.Context, channel, agent ids.ID, dimension string) {
	rec := s.d.Measure
	txn.AfterCommit(ctx, func() {
		rec.Record(measure.ApprovalCards, measure.Key{ChannelID: channel, AgentID: agent, Dimension: dimension}, measure.One())
	})
}

// approvalCard reports whether any of rows is an approval card component (required_capability
// approve_actions, 09 11.5).
func approvalCard(rows []ComponentRow) bool {
	for _, r := range rows {
		if r.RequiredCapability == domain.CapabilityApproveActions {
			return true
		}
	}
	return false
}

// approvalDimension maps the choice of an interaction on an approval card to its dimension of
// approval_cards: once, session and always approve, deny denies; any other choice, or none, is no
// decision and records nothing.
func approvalDimension(choice string) (string, bool) {
	switch choice {
	case "once", "session", "always":
		return cardApproved, true
	case "deny":
		return cardDenied, true
	}
	return "", false
}

// finishedToolCall is a tool_call step a committed patch finished, with its run's channel and
// agent.
type finishedToolCall struct {
	status   string
	duration time.Duration
}

// measureToolCalls records tool_calls for every tool_call step of a committed patch that reached
// completed or failed (25 3.2), with started_at to finished_at as the duration.
func (s *Service) measureToolCalls(channel, agent ids.ID, calls []finishedToolCall) {
	for _, c := range calls {
		s.d.Measure.Record(measure.ToolCalls, measure.Key{ChannelID: channel, AgentID: agent, Dimension: c.status}, measure.Timed(c.duration))
	}
}

// finishedToolCalls returns the tool_call steps of after that are completed or failed and were not
// so in before (a step new in this patch counts when it arrives finished). Steps are matched by
// id; a step without a started_at counts with duration 0.
func finishedToolCalls(before, after []domain.DocStep) []finishedToolCall {
	ended := func(status string) bool { return status == "completed" || status == "failed" }
	was := make(map[ids.ID]string, len(before))
	for _, st := range before {
		was[st.ID] = st.Status
	}
	var out []finishedToolCall
	for _, st := range after {
		if st.Kind != "tool_call" || !ended(st.Status) || ended(was[st.ID]) {
			continue
		}
		var d time.Duration
		if !st.StartedAt.IsZero() && !st.FinishedAt.IsZero() {
			d = st.FinishedAt.Sub(st.StartedAt)
		}
		out = append(out, finishedToolCall{status: st.Status, duration: d})
	}
	return out
}

// measureRequested registers approval_cards requested for a new card of an agent that carries an
// approval component (25 3.2), once per card message, for after the commit of the unit of work of
// the message. The channel comes from the thread of the card. The card of an app counts nothing:
// an app has no runs, its id is no agent_id, and its interactions never reach measureDecision.
func (s *Service) measureRequested(ctx context.Context, in messaging.CardInput, rows []ComponentRow) error {
	if in.AuthorKind != string(actor.KindAgent) || !approvalCard(rows) {
		return nil
	}
	info, err := s.d.Threads.Info(ctx, in.ThreadID)
	if err != nil {
		return err
	}
	s.measureCard(ctx, info.ChannelID, in.AuthorID, cardRequested)
	return nil
}

// measureDecision registers approval_cards approved or denied for an interaction on an approval
// component whose button value is a choice of 09 11.5 (once, session, always or deny), the choice
// the audit entry of the approval records (domain.InteractionAuditDetails).
func (s *Service) measureDecision(ctx context.Context, l lockedCard) {
	if l.cur.RequiredCapability != domain.CapabilityApproveActions {
		return
	}
	d := domain.InteractionAuditDetails(domain.AuditFacts{
		ComponentType: l.cur.Type, RequiredCapability: l.cur.RequiredCapability, RestrictedTo: l.cur.RestrictedTo, Definition: l.cur.Definition,
	})
	if dim, ok := approvalDimension(d.Choice); ok {
		s.measureCard(ctx, l.facts.ChannelID, l.facts.AuthorID, dim)
	}
}

// measureExpired registers approval_cards expired once for a card of an agent when the job
// runs.component_expiry expired at least one of its approval components unused (25 3.2).
func (s *Service) measureExpired(ctx context.Context, messageID ids.ID, due []ComponentRow) error {
	if !approvalCard(due) {
		return nil
	}
	facts, err := s.d.Cards.LockCard(ctx, messageID)
	if err != nil || facts.AuthorKind != string(actor.KindAgent) {
		return err
	}
	s.measureCard(ctx, facts.ChannelID, facts.AuthorID, cardExpired)
	return nil
}
