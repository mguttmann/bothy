// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"slices"

	"github.com/OWNER/bothy/services/server/internal/agents"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// HeartbeatReader implements agents.Heartbeats over the stored presence rows. It is separate from
// the Tracker on purpose: the worker role, which runs runs.stall_check, starts no tracker and must
// see only what was persisted, the same state in every process. It is safe for concurrent use.
type HeartbeatReader struct {
	store PresenceStore
}

// Compile-time check: the incoming port.
var _ agents.Heartbeats = (*HeartbeatReader)(nil)

// NewHeartbeatReader returns the reader over store (the agents postgres repository). Errors: a nil
// store.
//
// Example:
//
//	r, err := app.NewHeartbeatReader(postgres.NewRepository(pool))
func NewHeartbeatReader(store PresenceStore) (*HeartbeatReader, error) {
	if store == nil {
		return nil, errors.New("agents: NewHeartbeatReader needs a presence store")
	}
	return &HeartbeatReader{store: store}, nil
}

// HeartbeatStates implements agents.Heartbeats: one GetPresenceRows read, which runs in the
// ambient transaction of ctx when there is one. The run ids are copied, so the caller owns the
// result.
func (r *HeartbeatReader) HeartbeatStates(ctx context.Context, agentIDs []ids.ID) ([]agents.HeartbeatState, error) {
	if len(agentIDs) == 0 {
		return nil, nil
	}
	rows, err := r.store.GetPresenceRows(ctx, agentIDs)
	if err != nil {
		return nil, err
	}
	out := make([]agents.HeartbeatState, 0, len(rows))
	for _, row := range rows {
		runs := row.Stored.Details.ActiveRuns
		out = append(out, agents.HeartbeatState{AgentID: row.ID, LastHeartbeatAt: row.LastHeartbeatAt,
			ActiveRunIDs: slices.Clone(runs), ActiveRunsReported: runs != nil})
	}
	return out, nil
}
