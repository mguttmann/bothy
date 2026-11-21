// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/runs/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/runs/app"
)

// RecordFeeders implements app.Repository: one row of run_feeders per principal, NULL for a zero
// principal; a row already stored changes nothing.
func (r *Repository) RecordFeeders(ctx context.Context, runID ids.ID, source string, principals []ids.ID) error {
	if len(principals) == 0 {
		return nil
	}
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	list := make([]pgtype.UUID, len(principals))
	for i, p := range principals {
		list[i] = optionalUUID(p)
	}
	if err := q.InsertRunFeeders(ctx, sqlcgen.InsertRunFeedersParams{RunID: uuid(runID), Source: source, PrincipalIds: list}); err != nil {
		return mapError(err, "recording the feeders of a run")
	}
	return nil
}

// Feeders implements app.Repository.
func (r *Repository) Feeders(ctx context.Context, runID ids.ID) ([]app.Feeder, error) {
	rows, err := r.read(ctx).RunFeeders(ctx, uuid(runID))
	if err != nil {
		return nil, mapError(err, "reading the feeders of a run")
	}
	out := make([]app.Feeder, len(rows))
	for i, row := range rows {
		out[i] = app.Feeder{Source: row.Source}
		if row.PrincipalID.Valid {
			out[i].PrincipalID = idOf(row.PrincipalID)
		}
	}
	return out, nil
}
