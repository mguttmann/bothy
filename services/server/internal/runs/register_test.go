// SPDX-License-Identifier: Apache-2.0

package runs_test

import (
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/runs"
)

// TestRegister_Events: runs declares run.state_changed (M2-05), agent.invocation (M2-08),
// command.invoked (M2-16), run.cancel_requested (M2-09), the ephemeral
// command.autocomplete_requested and the op command.autocomplete_result (M2-17).
func TestRegister_Events(t *testing.T) {
	t.Parallel()
	r := authorize.NewRegistry()
	runs.Register(r)
	ev := r.Events()
	if len(ev) != 5 || ev[0].Module != "runs" || ev[0].Type != authorize.EventAgentInvocation ||
		ev[1].Module != "runs" || ev[1].Type != authorize.EventCommandAutocompleteRequested ||
		ev[2].Module != "runs" || ev[2].Type != authorize.EventCommandInvoked ||
		ev[3].Type != authorize.EventRunCancelRequested || ev[4].Type != authorize.EventRunStateChanged {
		t.Fatalf("events %+v", ev)
	}
	// M2-17 and M2-10: runs serves the gateway ops command.autocomplete_result and run.update.
	ops := r.Ops()
	if len(ops) != 2 || ops[0].Module != "runs" || ops[0].Op != authorize.OpCommandAutocompleteResult || ops[1].Module != "runs" || ops[1].Op != authorize.OpRunUpdate {
		t.Fatalf("ops %+v", ops)
	}
}

// TestInvocationBearing: the four types of 04 15.3 and nothing else.
func TestInvocationBearing(t *testing.T) {
	t.Parallel()
	for _, typ := range []string{"agent.invocation", "agent.followup", "interaction.created", "command.invoked"} {
		if !runs.InvocationBearing(typ) {
			t.Errorf("%s", typ)
		}
	}
	for _, typ := range []string{"run.cancel_requested", "agent.config_changed", "run.state_changed", ""} {
		if runs.InvocationBearing(typ) {
			t.Errorf("%s", typ)
		}
	}
}
