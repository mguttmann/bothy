// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"testing"

	"github.com/OWNER/bothy/services/server/internal/chat/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// TestOperatorSet_Validation checks the rules of D1 of M1-54 over the modes, a missing and a
// superfluous agent_id and inherit on a space.
func TestOperatorSet_Validation(t *testing.T) {
	agent := ids.New()
	cases := []struct {
		name    string
		a       domain.OperatorAssignment
		space   bool
		pointer string
		code    string
	}{
		{"channel inherit", domain.OperatorAssignment{Mode: domain.OperatorInherit}, false, "", ""},
		{"channel none", domain.OperatorAssignment{Mode: domain.OperatorNone}, false, "", ""},
		{"channel agent", domain.OperatorAssignment{Mode: domain.OperatorAgent, AgentID: agent}, false, "", ""},
		{"space none", domain.OperatorAssignment{Mode: domain.OperatorNone}, true, "", ""},
		{"space agent", domain.OperatorAssignment{Mode: domain.OperatorAgent, AgentID: agent}, true, "", ""},
		{"space inherit", domain.OperatorAssignment{Mode: domain.OperatorInherit}, true, "/mode", domain.FieldInheritOnlyForChannels},
		{"agent without id", domain.OperatorAssignment{Mode: domain.OperatorAgent}, false, "/agent_id", domain.FieldRequired},
		{"space agent without id", domain.OperatorAssignment{Mode: domain.OperatorAgent}, true, "/agent_id", domain.FieldRequired},
		{"none with id", domain.OperatorAssignment{Mode: domain.OperatorNone, AgentID: agent}, false, "/agent_id", domain.FieldNotAllowedForMode},
		{"inherit with id", domain.OperatorAssignment{Mode: domain.OperatorInherit, AgentID: agent}, false, "/agent_id", domain.FieldNotAllowedForMode},
		{"space none with id", domain.OperatorAssignment{Mode: domain.OperatorNone, AgentID: agent}, true, "/agent_id", domain.FieldNotAllowedForMode},
		{"unknown mode", domain.OperatorAssignment{Mode: "owner"}, false, "/mode", domain.FieldInvalid},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var err error
			if c.space {
				err = c.a.CheckForSpace()
			} else {
				err = c.a.CheckForChannel()
			}
			if c.code == "" {
				if err != nil {
					t.Fatalf("err %v", err)
				}
				return
			}
			e, ok := errs.As(err)
			if !ok || e.Code != errs.CodeValidationFailed || len(e.Fields) != 1 || e.Fields[0].Pointer != c.pointer || e.Fields[0].Code != c.code {
				t.Fatalf("err %#v, want %s %s", err, c.pointer, c.code)
			}
		})
	}
}

// TestEffectiveOperator checks the effective operator of a channel (03 channels): inherit with and
// without a space default, none, agent.
func TestEffectiveOperator(t *testing.T) {
	def, own := ids.New(), ids.New()
	cases := []struct {
		name         string
		inherit      bool
		own, def     ids.ID
		want         ids.ID
		wantMode     domain.OperatorMode
		wantAgentID  ids.ID
		wantAssigned domain.OperatorAssignment
	}{
		{"inherit with default", true, ids.ID{}, def, def, domain.OperatorInherit, ids.ID{}, domain.OperatorAssignment{Mode: domain.OperatorInherit}},
		{"inherit without default", true, ids.ID{}, ids.ID{}, ids.ID{}, domain.OperatorInherit, ids.ID{}, domain.OperatorAssignment{Mode: domain.OperatorInherit}},
		{"none", false, ids.ID{}, def, ids.ID{}, domain.OperatorNone, ids.ID{}, domain.OperatorAssignment{Mode: domain.OperatorNone}},
		{"agent", false, own, def, own, domain.OperatorAgent, own, domain.OperatorAssignment{Mode: domain.OperatorAgent, AgentID: own}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := domain.EffectiveOperator(c.inherit, c.own, c.def); got != c.want {
				t.Fatalf("EffectiveOperator %v, want %v", got, c.want)
			}
			ch := domain.Channel{OperatorInherit: c.inherit, OperatorAgentID: c.own}
			op := ch.Operator(c.def)
			if op.Mode != c.wantMode || op.AgentID != c.wantAgentID || op.EffectiveAgentID != c.want {
				t.Fatalf("Operator %+v", op)
			}
			if got := ch.Assignment(); got != c.wantAssigned {
				t.Fatalf("Assignment %+v, want %+v", got, c.wantAssigned)
			}
		})
	}
}

// TestSetOperator_Domain checks that a channel and a space change only for a different assignment
// and bump their version once.
func TestSetOperator_Domain(t *testing.T) {
	agent := ids.New()
	ch := domain.Channel{OperatorInherit: true, Version: 3}
	if ch.SetOperator(domain.OperatorAssignment{Mode: domain.OperatorInherit}) || ch.Version != 3 {
		t.Fatalf("same assignment changed the channel: %+v", ch)
	}
	if !ch.SetOperator(domain.OperatorAssignment{Mode: domain.OperatorAgent, AgentID: agent}) || ch.Version != 4 ||
		ch.OperatorInherit || ch.OperatorAgentID != agent {
		t.Fatalf("agent: %+v", ch)
	}
	if !ch.SetOperator(domain.OperatorAssignment{Mode: domain.OperatorNone}) || ch.Version != 5 || ch.OperatorInherit || !ch.OperatorAgentID.IsZero() {
		t.Fatalf("none: %+v", ch)
	}
	sp := domain.Space{Version: 2}
	if sp.SetOperator(domain.OperatorAssignment{Mode: domain.OperatorNone}) || sp.Version != 2 {
		t.Fatalf("same default changed the space: %+v", sp)
	}
	if !sp.SetOperator(domain.OperatorAssignment{Mode: domain.OperatorAgent, AgentID: agent}) || sp.Version != 3 || sp.DefaultOperatorAgentID != agent {
		t.Fatalf("agent: %+v", sp)
	}
	if got := sp.Assignment(); got != (domain.OperatorAssignment{Mode: domain.OperatorAgent, AgentID: agent}) {
		t.Fatalf("Assignment %+v", got)
	}
}
