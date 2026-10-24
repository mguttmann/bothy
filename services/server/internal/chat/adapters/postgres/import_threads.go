// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/OWNER/bothy/services/server/internal/chat/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/chat/app"
	"github.com/OWNER/bothy/services/server/internal/chat/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Compile-time check: the store of the imported threads (M8-11).
var _ app.ImportThreadStore = (*Repository)(nil)

// InsertImportedThread implements app.ImportThreadStore: a second thread of the same parent
// message is 409 conflict on /parent_message_id, like InsertThread.
func (r *Repository) InsertImportedThread(ctx context.Context, th domain.Thread) (domain.Thread, error) {
	if err := searchLanguageSet(th); err != nil {
		return domain.Thread{}, err
	}
	q, err := r.write(ctx)
	if err != nil {
		return domain.Thread{}, err
	}
	row, err := q.InsertImportedThread(ctx, sqlcgen.InsertImportedThreadParams{
		ID: uuid(th.ID), ChannelID: uuid(th.ChannelID), ParentMessageID: optionalUUID(th.ParentMessageID), Title: th.Title,
		TitleSource: string(th.TitleSource), State: string(th.State), ClosedAt: optionalTime(th.ClosedAt),
		OperatorAgentID: optionalUUID(th.OperatorAgentID), CreatedBy: uuid(th.CreatedBy),
		CreatedAt: pgtype.Timestamptz{Time: th.CreatedAt, Valid: true}, LastActivityAt: pgtype.Timestamptz{Time: th.LastActivityAt, Valid: true},
		SearchLanguage: th.SearchLanguage,
	})
	if err := parentTaken(err); err != nil {
		return domain.Thread{}, err
	}
	if err != nil {
		return domain.Thread{}, mapError(err, "inserting an imported thread")
	}
	return toThread(threadRow(row)), nil
}

// SaveImportedThread implements app.ImportThreadStore.
func (r *Repository) SaveImportedThread(ctx context.Context, th domain.Thread) error {
	q, err := r.write(ctx)
	if err != nil {
		return err
	}
	err = q.SaveImportedThread(ctx, sqlcgen.SaveImportedThreadParams{
		ID: uuid(th.ID), Title: th.Title, State: string(th.State), ClosedAt: optionalTime(th.ClosedAt), ClosedBy: optionalUUID(th.ClosedBy),
		LastActivityAt: pgtype.Timestamptz{Time: th.LastActivityAt, Valid: true},
	})
	return mapError(err, "updating an imported thread")
}

// ReserveImportedSeqs implements app.ImportThreadStore.
func (r *Repository) ReserveImportedSeqs(ctx context.Context, id ids.ID, n int, lastActivity time.Time) (domain.Thread, error) {
	q, err := r.write(ctx)
	if err != nil {
		return domain.Thread{}, err
	}
	row, err := q.ReserveImportedSeqs(ctx, sqlcgen.ReserveImportedSeqsParams{
		ID: uuid(id), N: int64(n), LastActivityAt: pgtype.Timestamptz{Time: lastActivity, Valid: true},
	})
	if err != nil {
		return domain.Thread{}, mapError(err, "reserving thread numbers of imported messages")
	}
	return toThread(threadRow(row)), nil
}

// FinishImportBatch implements app.ImportThreadStore.
func (r *Repository) FinishImportBatch(ctx context.Context, id ids.ID) (domain.Thread, error) {
	q, err := r.write(ctx)
	if err != nil {
		return domain.Thread{}, err
	}
	row, err := q.FinishImportBatch(ctx, uuid(id))
	if err != nil {
		return domain.Thread{}, mapError(err, "finishing an import batch")
	}
	return toThread(threadRow(row)), nil
}
