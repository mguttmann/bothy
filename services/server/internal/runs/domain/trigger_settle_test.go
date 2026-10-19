// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

func invokeDecision(t *testing.T, active bool) domain.Decision {
	t.Helper()
	in := threadMsg()
	in.ThreadOperator.Active = active
	d := mustDecide(t, in)
	if d.Outcome != domain.OutcomeInvoke {
		t.Fatalf("setup: %+v", d)
	}
	return d
}

// TestSettle is 09 3.4 step 7 and D-M2-04-3: allow gives received, a denial gives failed with the
// mapped reason, none and reject ignore the verdict, commands keep trigger_state NULL.
func TestSettle(t *testing.T) {
	settle := func(t *testing.T, d domain.Decision, v domain.Verdict) domain.Settlement {
		t.Helper()
		s, err := domain.Settle(d, v)
		if err != nil {
			t.Fatalf("Settle: %v", err)
		}
		return s
	}
	t.Run("allow", func(t *testing.T) {
		s := settle(t, invokeDecision(t, true), domain.Verdict{Allowed: true})
		if s != (domain.Settlement{Accepted: true, State: domain.TriggerStateReceived}) {
			t.Fatalf("got %+v", s)
		}
	})
	t.Run("allow follow-up", func(t *testing.T) {
		in := threadMsg()
		in.ActiveRun = &domain.ActiveRun{RunID: runR, AgentID: lotseID}
		s := settle(t, mustDecide(t, in), domain.Verdict{Allowed: true})
		if s != (domain.Settlement{Accepted: true, State: domain.TriggerStateReceived}) {
			t.Fatalf("got %+v", s)
		}
	})
	mapping := []struct {
		deny   string
		active bool
		want   domain.Reason
	}{
		{string(auditing.ReasonAgentNotReachable), false, domain.ReasonAgentDisabled},
		{string(auditing.ReasonAgentNotReachable), true, domain.ReasonAgentNotAllowedHere},
		{string(auditing.ReasonLevelInsufficient), true, domain.ReasonNoTriggerRight},
		{string(auditing.ReasonNoGrant), true, domain.ReasonNoTriggerRight},
		{string(auditing.ReasonBillingModePersonalNotHolder), true, domain.ReasonBillingModePersonalNotHolder},
		{string(auditing.ReasonProjectNotReady), true, domain.ReasonProjectNotReady},
		{string(auditing.ReasonRunActiveOtherAgent), true, domain.ReasonRunActiveOtherAgent},
		{"tool_class_owner_only", true, "tool_class_owner_only"},
		{"dev_environment_not_ready", true, "dev_environment_not_ready"},
		{"channel_read_only", true, "channel_read_only"},
	}
	for _, m := range mapping {
		t.Run("deny/"+m.deny, func(t *testing.T) {
			s := settle(t, invokeDecision(t, m.active), domain.Verdict{Reason: m.deny})
			if s != (domain.Settlement{State: domain.TriggerStateFailed, Reason: m.want}) {
				t.Fatalf("got %+v, want failed/%q", s, m.want)
			}
		})
	}
	t.Run("none ignores the verdict", func(t *testing.T) {
		in := mainMsg()
		in.Origin = domain.OriginAgent
		for _, v := range []domain.Verdict{{Allowed: true}, {Reason: "no_grant"}, {}} {
			if s := settle(t, mustDecide(t, in), v); s != (domain.Settlement{}) {
				t.Fatalf("got %+v", s)
			}
		}
	})
	t.Run("reject ignores the verdict", func(t *testing.T) {
		in := mainMsg()
		in.Mentioned = []domain.AgentFacts{lotse(), texter()}
		for _, v := range []domain.Verdict{{Allowed: true}, {Reason: "no_grant"}, {}} {
			s := settle(t, mustDecide(t, in), v)
			if s != (domain.Settlement{State: domain.TriggerStateFailed, Reason: domain.ReasonMultipleAgentsMentioned}) {
				t.Fatalf("got %+v", s)
			}
		}
	})
	t.Run("commands keep trigger_state NULL", func(t *testing.T) {
		cmd := domain.CommandTrigger{CommandID: commandID, ThreadID: threadT, Owner: lotse()}
		d, err := domain.DecideCommand(cmd)
		if err != nil {
			t.Fatal(err)
		}
		if s := settle(t, d, domain.Verdict{Allowed: true}); s != (domain.Settlement{Accepted: true}) {
			t.Fatalf("allow: %+v", s)
		}
		if s := settle(t, d, domain.Verdict{Reason: "no_grant"}); s != (domain.Settlement{Reason: domain.ReasonNoTriggerRight}) {
			t.Fatalf("deny: %+v", s)
		}
		cmd.ActiveRun = &domain.ActiveRun{RunID: runR, AgentID: lotseID}
		d, err = domain.DecideCommand(cmd)
		if err != nil {
			t.Fatal(err)
		}
		if s := settle(t, d, domain.Verdict{Allowed: true}); s != (domain.Settlement{Reason: domain.ReasonRunActive}) {
			t.Fatalf("run_active: %+v", s)
		}
	})
	t.Run("errors", func(t *testing.T) {
		bad := map[string]struct {
			d domain.Decision
			v domain.Verdict
		}{
			"denial without reason":    {invokeDecision(t, true), domain.Verdict{}},
			"rejection without reason": {domain.Decision{Outcome: domain.OutcomeReject}, domain.Verdict{}},
			"invoke without candidate": {domain.Decision{Outcome: domain.OutcomeInvoke, Check: domain.CheckInvoke}, domain.Verdict{Allowed: true}},
			"unknown outcome":          {domain.Decision{Outcome: 99}, domain.Verdict{Allowed: true}},
		}
		for name, b := range bad {
			t.Run(name, func(t *testing.T) {
				s, err := domain.Settle(b.d, b.v)
				wantInternal(t, err)
				if s != (domain.Settlement{}) {
					t.Fatalf("settlement with error = %+v", s)
				}
			})
		}
	})
}
