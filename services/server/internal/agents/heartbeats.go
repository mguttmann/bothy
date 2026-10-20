// SPDX-License-Identifier: Apache-2.0

package agents

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// HeartbeatState is the last persisted heartbeat of one agent: the columns last_heartbeat_at and
// reported_details.active_runs of agents. It lags the live heartbeat of the presence tracker by up
// to agent.heartbeat_persist_interval, and it is what a worker without the tracker can see.
type HeartbeatState struct {
	// AgentID is the agent.
	AgentID ids.ID
	// LastHeartbeatAt is last_heartbeat_at; zero when the agent never sent a heartbeat.
	LastHeartbeatAt time.Time
	// ActiveRunIDs are the runs of reported_details.active_runs; nil when not reported.
	ActiveRunIDs []ids.ID
	// ActiveRunsReported is true when the stored heartbeat carried active_runs, also an empty
	// list. Without it no run can be missing from the heartbeat.
	ActiveRunsReported bool
}

// Heartbeats is the incoming port that reads persisted heartbeats; runs.stall_check of M2-09
// finds interrupted and stale runs with it (09 4.8 and 5.5).
type Heartbeats interface {
	// HeartbeatStates reads agents.last_heartbeat_at and agents.reported_details.active_runs of
	// agentIDs (stored rows only, not the in-memory tracker), ascending by id; unknown ids are
	// absent and an empty list reads nothing. It reads in the ambient transaction of ctx when
	// there is one and on the pool otherwise, and it does not authorize: the caller is a job of
	// the server. Errors: a database error, or stored reported_details outside its schema
	// (internal_error).
	//
	// Example:
	//
	//	states, err := heartbeats.HeartbeatStates(ctx, []ids.ID{agentID})
	HeartbeatStates(ctx context.Context, agentIDs []ids.ID) ([]HeartbeatState, error)
}
