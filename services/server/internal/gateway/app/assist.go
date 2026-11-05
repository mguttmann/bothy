// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"fmt"

	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
)

// AskAssistant offers r, a search.assist_requested without a token, to exactly one live connection
// of the assistant agent in this process (docs/spec/14-search.md 4.10), by the rules of
// AskAutocomplete: consumer default if it is live, otherwise the most recently attached live
// connection, admitted with agent.events.consume on the agent in delivery mode, queued without
// blocking. sent is false when no connection was live, admitted or had room. Errors only report
// programming faults: another type, a schema version below 1, an audience that is not the agent, a
// token, or a payload that is no JSON object, contains null or reaches
// sync.event_payload_max_bytes.
func (h *Hub) AskAssistant(ctx context.Context, r AutocompleteRequest) (bool, error) {
	ev := r.Event
	if ev.Type != string(authorize.EventSearchAssistRequested) || ev.SchemaVersion < 1 || ev.Audience != r.OwnerID || r.InvocationToken != "" {
		return false, fmt.Errorf("gateway: AskAssistant takes only %s for its agent, without a token", authorize.EventSearchAssistRequested)
	}
	return h.askOwner(ctx, r)
}
