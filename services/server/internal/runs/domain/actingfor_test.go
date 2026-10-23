// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// TestActingForVerdict covers Design note 3 b to d of M2-14: every binding that does not match
// the token is invalid with zero IDs, an active run is bound and an ended run is run_inactive,
// both naming the triggering person, the run, its thread and the jti.
func TestActingForVerdict(t *testing.T) {
	t.Parallel()
	agent, alex, run, thread, jti := ids.New(), ids.New(), ids.New(), ids.New(), ids.New()
	claims := domain.InvocationClaims{Aud: agent.String(), Sub: alex.String(), Jti: jti.String(),
		TriggerKind: string(domain.TriggerMessage), RunID: run.String(), MessageID: ids.New().String(), ThreadID: thread.String()}
	binding := domain.RunBinding{ID: run, AgentID: agent, TriggeredBy: alex, ThreadID: thread, State: string(domain.StateRunning)}
	bound := func(s domain.ActingForStatus) domain.ActingForResult {
		return domain.ActingForResult{Status: s, UserID: alex, RunID: run, ThreadID: thread, JTI: jti.String()}
	}
	invalid := domain.ActingForResult{Status: domain.ActingForInvalid}

	type tc struct {
		claims  domain.InvocationClaims
		binding domain.RunBinding
		found   bool
		want    domain.ActingForResult
	}
	with := func(f func(*domain.InvocationClaims, *domain.RunBinding)) (domain.InvocationClaims, domain.RunBinding) {
		c, b := claims, binding
		f(&c, &b)
		return c, b
	}
	cases := map[string]tc{}
	add := func(name string, found bool, want domain.ActingForResult, f func(*domain.InvocationClaims, *domain.RunBinding)) {
		c, b := with(f)
		cases[name] = tc{claims: c, binding: b, found: found, want: want}
	}
	add("no run", false, invalid, func(_ *domain.InvocationClaims, b *domain.RunBinding) { *b = domain.RunBinding{} })
	add("no run with a row", false, invalid, func(*domain.InvocationClaims, *domain.RunBinding) {})
	add("other agent", true, invalid, func(_ *domain.InvocationClaims, b *domain.RunBinding) { b.AgentID = ids.New() })
	add("sub not triggered_by", true, invalid, func(c *domain.InvocationClaims, _ *domain.RunBinding) { c.Sub = ids.New().String() })
	add("command sub not triggered_by", true, invalid, func(c *domain.InvocationClaims, _ *domain.RunBinding) {
		c.TriggerKind, c.MessageID, c.CommandID, c.Sub = string(domain.TriggerCommand), "", ids.New().String(), ids.New().String()
	})
	// An interaction or a follow-up of another person in the run names that person in sub; the
	// binding stays the run's triggering person (D-M2-15-30).
	add("interaction of another person", true, bound(domain.ActingForBound), func(c *domain.InvocationClaims, _ *domain.RunBinding) {
		c.TriggerKind, c.MessageID, c.InteractionID, c.Sub = string(domain.TriggerInteraction), "", ids.New().String(), ids.New().String()
	})
	add("follow-up of another person", true, bound(domain.ActingForBound), func(c *domain.InvocationClaims, _ *domain.RunBinding) {
		c.TriggerKind, c.Sub = string(domain.TriggerFollowup), ids.New().String()
	})
	add("interaction sub not canonical", true, invalid, func(c *domain.InvocationClaims, _ *domain.RunBinding) {
		c.TriggerKind, c.MessageID, c.InteractionID, c.Sub = string(domain.TriggerInteraction), "", ids.New().String(), "x"
	})
	add("other thread_id", true, invalid, func(c *domain.InvocationClaims, _ *domain.RunBinding) { c.ThreadID = ids.New().String() })
	add("claims without run_id", true, invalid, func(c *domain.InvocationClaims, _ *domain.RunBinding) {
		c.RunID, c.MessageID, c.CommandID, c.TriggerKind = "", "", ids.New().String(), string(domain.TriggerAutocomplete)
	})
	add("run_id of another run", true, invalid, func(c *domain.InvocationClaims, _ *domain.RunBinding) { c.RunID = ids.New().String() })
	add("run_id not canonical", true, invalid, func(c *domain.InvocationClaims, _ *domain.RunBinding) { c.RunID = "not-a-uuid" })
	add("sub not canonical", true, invalid, func(c *domain.InvocationClaims, _ *domain.RunBinding) { c.Sub = "" })
	add("unknown state", true, invalid, func(_ *domain.InvocationClaims, b *domain.RunBinding) { b.State = "paused" })
	add("empty state", true, invalid, func(_ *domain.InvocationClaims, b *domain.RunBinding) { b.State = "" })
	add("zero agent", true, invalid, func(_ *domain.InvocationClaims, b *domain.RunBinding) { b.AgentID = ids.ID{} })
	for _, s := range []domain.State{domain.StateQueued, domain.StateRunning, domain.StateWaitingInput} {
		add(string(s), true, bound(domain.ActingForBound), func(_ *domain.InvocationClaims, b *domain.RunBinding) { b.State = string(s) })
	}
	for _, s := range []domain.State{domain.StateCompleted, domain.StateFailed, domain.StateCancelled} {
		add(string(s), true, bound(domain.ActingForRunInactive), func(_ *domain.InvocationClaims, b *domain.RunBinding) { b.State = string(s) })
	}
	for name, c := range cases {
		got := domain.ActingForVerdict(c.claims, c.binding, c.found, agent)
		if got != c.want {
			t.Errorf("%s: ActingForVerdict = %+v, want %+v", name, got, c.want)
		}
	}
	if got := domain.ActingForVerdict(claims, binding, true, ids.ID{}); got != invalid {
		t.Errorf("zero agent of the request: %+v, want invalid", got)
	}
}

// TestActingForStatus_Values pins the three values to those of kernel/actor (Design note 2).
func TestActingForStatus_Values(t *testing.T) {
	t.Parallel()
	for got, want := range map[domain.ActingForStatus]string{
		domain.ActingForBound: "bound", domain.ActingForRunInactive: "run_inactive", domain.ActingForInvalid: "invalid",
	} {
		if string(got) != want {
			t.Errorf("status %q, want %q", got, want)
		}
	}
}
