// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/runs"
)

// Compile-time check: Service is the port of maintenance mode.
var _ runs.RunningRuns = (*Service)(nil)

// Running implements runs.RunningRuns over one indexed read (M7-08).
func (s *Service) Running(ctx context.Context, agentID ids.ID) ([]runs.RunningRun, error) {
	return s.d.Repo.RunningOfAgent(ctx, agentID)
}
