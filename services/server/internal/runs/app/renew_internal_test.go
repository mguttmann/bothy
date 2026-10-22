// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// TestRenewalInput_CommandRun: a token of a command run (M2-16) is renewed as its agent.invocation:
// the rebuilt delivery carries the command of the token, the resolver of command runs takes it as
// the triggering object, and the invoker is the run's trigger. A command token without its
// command is an internal error.
func TestRenewalInput_CommandRun(t *testing.T) {
	t.Parallel()
	row := RunRow{ID: ids.New(), ThreadID: ids.New(), AgentID: ids.New(), TriggerKind: string(domain.TriggerCommand), TriggeredBy: ids.New()}
	cmd := ids.New()
	in, err := renewalInput(row, domain.InvocationClaims{TriggerKind: string(domain.TriggerCommand), CommandID: cmd.String()})
	if err != nil || in.Type != domain.EventAgentInvocation {
		t.Fatalf("renewalInput: %+v %v", in, err)
	}
	res := resolverFor(in.Type, row)
	if res == nil {
		t.Fatal("no resolver for the renewal of a command run")
	}
	tr, err := res(nil, context.Background(), row, in)
	if err != nil || tr.object != cmd || tr.person != row.TriggeredBy || !tr.mentions {
		t.Fatalf("trigger %+v %v", tr, err)
	}
	// The class: every run trigger kind the delivery check resolves for agent.invocation is
	// renewable, so a trigger kind added there without a renewal fails here.
	for kind := range resolvers[domain.EventAgentInvocation] {
		c := domain.InvocationClaims{TriggerKind: kind, CommandID: cmd.String()}
		r := row
		r.TriggerKind = kind
		if got, err := renewalInput(r, c); err != nil || got.Type != domain.EventAgentInvocation || resolverFor(got.Type, r) == nil {
			t.Errorf("trigger kind %s: %+v %v", kind, got, err)
		}
	}
	if _, err := renewalInput(row, domain.InvocationClaims{TriggerKind: string(domain.TriggerCommand)}); err == nil {
		t.Fatal("a command token without its command was renewed")
	}
}
