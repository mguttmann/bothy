// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/chat/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/chat/domain"
)

// SaveChannelReadOnly implements app.Store: read_only and the version of c (M8-03).
func (r *Repository) SaveChannelReadOnly(ctx context.Context, c domain.Channel) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	err = q.SaveChannelReadOnly(ctx, sqlcgen.SaveChannelReadOnlyParams{ID: uuid(c.ID), ReadOnly: c.ReadOnly, Version: c.Version})
	return mapError(err, "setting the read-only flag of a channel")
}

// SaveChannelAutonomousRuns implements app.Store: allow_autonomous_runs and the version of c
// (channel.autonomous_runs.set, AR-04).
func (r *Repository) SaveChannelAutonomousRuns(ctx context.Context, c domain.Channel) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	err = q.SaveChannelAutonomousRuns(ctx, sqlcgen.SaveChannelAutonomousRunsParams{
		ID: uuid(c.ID), AllowAutonomousRuns: c.AllowAutonomousRuns, Version: c.Version,
	})
	return mapError(err, "setting the autonomous-runs switch of a channel")
}
