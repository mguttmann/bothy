// SPDX-License-Identifier: Apache-2.0

package runs

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// RunningRun is a run in state running with its thread.
type RunningRun struct {
	ID, ThreadID ids.ID
}

// RunningRuns is the port of maintenance mode (M7-08, ADR 0065 item 4): the exceptions of an
// agent depend on its runs in state running at request time. It never authorizes; the HTTP stage
// of maintenance mode only lets a request continue to its own authorize check.
type RunningRuns interface {
	// Running returns the runs of agentID in state running in ascending id; none is an empty
	// slice. Runs in queued or waiting_input are not running. Errors: the database error.
	//
	// Example:
	//
	//	rs, err := src.Running(ctx, agentID)
	Running(ctx context.Context, agentID ids.ID) ([]RunningRun, error)
}
