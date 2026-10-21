// SPDX-License-Identifier: Apache-2.0

package runs_test

import (
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/runs"
)

// TestRegister_Events: runs declares run.state_changed (M2-05), agent.invocation (M2-08),
// run.cancel_requested (M2-09) and interaction.created (M2-15).
func TestRegister_Events(t *testing.T) {
	t.Parallel()
	r := authorize.NewRegistry()
	runs.Register(r)
	ev := r.Events()
	if len(ev) != 4 || ev[0].Module != "runs" || ev[0].Type != authorize.EventAgentInvocation ||
		ev[1].Type != authorize.EventInteractionCreated || ev[2].Type != authorize.EventRunCancelRequested ||
		ev[3].Type != authorize.EventRunStateChanged {
		t.Fatalf("events %+v", ev)
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
