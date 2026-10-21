// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/sync"
	"github.com/OWNER/bothy/services/server/internal/sync/adapters/postgres/sqlcgen"
)

// Compile-time check: PersonalRepository is the read port of the positions (M3-19).
var _ sync.ReadPositions = (*PersonalRepository)(nil)

// PositionsFor implements sync.ReadPositions with one read of read_states: every person of
// userIDs gets an entry, 0 without a row. It never authorizes (the consumer push selected the
// people); inside a unit of work it reads the ambient transaction, else the pool.
func (r *PersonalRepository) PositionsFor(ctx context.Context, threadID ids.ID, userIDs []ids.ID) (map[ids.ID]int64, error) {
	out := make(map[ids.ID]int64, len(userIDs))
	if len(userIDs) == 0 {
		return out, nil
	}
	rows, err := r.read(ctx).ReadPositionsOfThread(ctx, sqlcgen.ReadPositionsOfThreadParams{ThreadID: uuid(threadID), UserIds: uuids(userIDs)})
	if err != nil {
		return nil, failed(err, "reading read positions")
	}
	for _, id := range userIDs {
		out[id] = 0
	}
	for _, row := range rows {
		out[idOf(row.UserID)] = row.LastReadSeq
	}
	return out, nil
}

// PositionsOfUser implements sync.ReadPositions with one read of read_states: every thread of
// threadIDs gets an entry, 0 without a row. It never authorizes (notify selected the person and
// confirms every thread with push.deliver); inside a unit of work it reads the ambient
// transaction, else the pool.
func (r *PersonalRepository) PositionsOfUser(ctx context.Context, userID ids.ID, threadIDs []ids.ID) (map[ids.ID]int64, error) {
	out := make(map[ids.ID]int64, len(threadIDs))
	if len(threadIDs) == 0 {
		return out, nil
	}
	rows, err := r.read(ctx).ReadPositionsOfUser(ctx, sqlcgen.ReadPositionsOfUserParams{UserID: uuid(userID), ThreadIds: uuids(threadIDs)})
	if err != nil {
		return nil, failed(err, "reading read positions")
	}
	for _, id := range threadIDs {
		out[id] = 0
	}
	for _, row := range rows {
		out[idOf(row.ThreadID)] = row.LastReadSeq
	}
	return out, nil
}
