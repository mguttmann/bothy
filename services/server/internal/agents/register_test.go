// SPDX-License-Identifier: Apache-2.0

package agents_test

import (
	"testing"

	"github.com/OWNER/bothy/services/server/internal/agents"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
)

// TestRegister_Events: agents declares the durable event it appends (M1-51): agent.updated, also
// at creation, because 05 has no agent.created.
func TestRegister_Events(t *testing.T) {
	t.Parallel()
	r := authorize.NewRegistry()
	agents.Register(r)
	got := r.Events()
	if len(got) != 1 || got[0].Module != "agents" || got[0].Type != authorize.EventAgentUpdated {
		t.Fatalf("events %+v", got)
	}
}
