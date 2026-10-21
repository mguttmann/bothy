// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/runs/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/runs/app"
)

// Compile-time check: Repository is the outgoing port of the run-context reads as well (M2-13).
var _ app.ContextRepository = (*Repository)(nil)

// EarlierSessionRun implements app.ContextRepository with the one statement EarlierSessionRun;
// outside a unit of work it reads on the pool.
func (r *Repository) EarlierSessionRun(ctx context.Context, row app.RunRow, sessionID ids.ID) (bool, ids.ID, error) {
	got, err := r.read(ctx).EarlierSessionRun(ctx, sqlcgen.EarlierSessionRunParams{ThreadID: uuid(row.ThreadID), AgentID: uuid(row.AgentID),
		SessionID: uuid(sessionID), RunID: uuid(row.ID), QueuedAt: pgtype.Timestamptz{Time: row.QueuedAt, Valid: true}})
	if err != nil {
		return false, ids.ID{}, mapError(err, "reading the earlier runs of a session")
	}
	return got.Used, idOf(got.TriggerMessageID), nil
}

// DeliveredFollowups implements app.ContextRepository; an empty list reads nothing.
func (r *Repository) DeliveredFollowups(ctx context.Context, threadID, sessionID ids.ID, messageIDs []ids.ID) ([]ids.ID, error) {
	if len(messageIDs) == 0 {
		return []ids.ID{}, nil
	}
	rows, err := r.read(ctx).DeliveredFollowupsOfSession(ctx, sqlcgen.DeliveredFollowupsOfSessionParams{ThreadID: uuid(threadID),
		SessionID: uuid(sessionID), MessageIds: uuids(messageIDs)})
	if err != nil {
		return nil, mapError(err, "reading the delivered follow-ups of a session")
	}
	out := make([]ids.ID, 0, len(rows))
	for _, id := range rows {
		out = append(out, idOf(id))
	}
	return out, nil
}
