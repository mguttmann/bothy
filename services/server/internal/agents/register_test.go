// SPDX-License-Identifier: Apache-2.0

package agents_test

import (
	"testing"

	"github.com/OWNER/bothy/services/server/internal/agents"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
)

// TestRegister_Events: agents declares the durable event it appends (M1-51): agent.updated, also
// at creation, because 05 has no agent.created; the ephemeral agent.presence_changed (M2-20,
// AZ-P2), command.changed of the command register (M2-16) and agent.definition_changed of the
// definitions of built-in agents (A1-02), agent.hosting_changed to a hosting runner (A1-03) and
// the ephemeral agent.interview_requested of the interview (A1-07).
func TestRegister_Events(t *testing.T) {
	t.Parallel()
	r := authorize.NewRegistry()
	agents.Register(r)
	got := r.Events()
	want := []authorize.EventType{
		authorize.EventAgentDefinitionChanged, authorize.EventAgentHostingChanged, authorize.EventAgentInterviewRequested, authorize.EventAgentPresenceChanged,
		authorize.EventAgentUpdated, authorize.EventCommandChanged,
	}
	if len(got) != len(want) {
		t.Fatalf("events %+v", got)
	}
	for i, w := range want {
		if got[i].Module != "agents" || got[i].Type != w {
			t.Fatalf("events %+v, want %v", got, want)
		}
	}
}
