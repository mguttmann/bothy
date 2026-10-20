// SPDX-License-Identifier: Apache-2.0

package agents_test

import (
	"testing"

	"github.com/OWNER/bothy/services/server/internal/agents"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
)

// TestRegister_Events: agents declares the durable event it appends (M1-51): agent.updated, also
// at creation, because 05 has no agent.created; and the ephemeral agent.presence_changed (M2-20,
// AZ-P2).
func TestRegister_Events(t *testing.T) {
	t.Parallel()
	r := authorize.NewRegistry()
	agents.Register(r)
	got := r.Events()
	if len(got) != 2 || got[0].Module != "agents" || got[0].Type != authorize.EventAgentPresenceChanged ||
		got[1].Module != "agents" || got[1].Type != authorize.EventAgentUpdated {
		t.Fatalf("events %+v", got)
	}
}
