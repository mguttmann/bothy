// SPDX-License-Identifier: Apache-2.0

package agents

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// StatusAgent is one active agent with its presence, as the status page shows it (M7-09).
type StatusAgent struct {
	ID          ids.ID
	Handle      string
	DisplayName string
	// State is StateOnline, StateDegraded, StateOffline or StateUnknown.
	State string
}

// StatusAgents is the incoming port of the agent presence for the status page (health_source
// agent.presence of docs/spec/16-operations.md 12, M7-09). It reads the stored presence
// (agents.reported_runtime_state), so every process gives the same answer, also the worker that
// tracks no heartbeat. It never authorizes: the status module shows an agent component only to
// readers with agent.read. Implementations are safe for concurrent use.
type StatusAgents interface {
	// ActiveAgents returns every agent whose principal is active, ascending by id. A stored
	// online or degraded state whose last heartbeat is older than agent.presence_timeout plus
	// agent.heartbeat_persist_interval counts as offline: the node that tracked it stopped
	// without storing the end. Errors: the database error.
	//
	// Example:
	//
	//	list, err := src.ActiveAgents(ctx)
	ActiveAgents(ctx context.Context) ([]StatusAgent, error)
}
