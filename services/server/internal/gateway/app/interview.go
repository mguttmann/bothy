// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"fmt"

	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
)

// AskBuilder offers r, an agent.interview_requested without a token, to exactly one live
// connection of the builder agent in this process (docs/spec/26-builtin-agents.md 6.1), by the
// rules of AskAutocomplete: consumer default if it is live, otherwise the most recently attached
// live connection, admitted with agent.events.consume on the agent in delivery mode, queued without
// blocking. sent is false when no connection was live, admitted or had room. Errors only report
// programming faults: another type, a schema version below 1, an audience that is not the agent, a
// token, or a payload that is no JSON object, contains null or reaches
// sync.event_payload_max_bytes.
func (h *Hub) AskBuilder(ctx context.Context, r AutocompleteRequest) (bool, error) {
	ev := r.Event
	if ev.Type != string(authorize.EventAgentInterviewRequested) || ev.SchemaVersion < 1 || ev.Audience != r.OwnerID || r.InvocationToken != "" {
		return false, fmt.Errorf("gateway: AskBuilder takes only %s for its agent, without a token", authorize.EventAgentInterviewRequested)
	}
	return h.askOwner(ctx, r)
}
