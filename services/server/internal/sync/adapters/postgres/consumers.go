// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/sync"
	"github.com/OWNER/bothy/services/server/internal/sync/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/sync/app"
)

// Compile-time checks: Repository is the store and the reader of the worker consumers.
var (
	_ app.ConsumerCursorStore = (*Repository)(nil)
	_ app.ConsumerLogReader   = (*Repository)(nil)
)

// EnsureCursor inserts the cursor row of name at max(seq) unless it exists
// (app.ConsumerCursorStore). It needs a unit of work.
func (r *Repository) EnsureCursor(ctx context.Context, name sync.ConsumerName) error {
	tx, err := pg.Tx(ctx)
	if err != nil {
		return err
	}
	return sqlcgen.New(tx).EnsureConsumerCursor(ctx, string(name))
}

// LockCursor locks the cursor row of name with FOR UPDATE SKIP LOCKED in the ambient transaction
// (app.ConsumerCursorStore); locked is false when another transaction holds the row or it does
// not exist. It needs a unit of work, because the lock lives until its commit.
func (r *Repository) LockCursor(ctx context.Context, name sync.ConsumerName) (int64, bool, error) {
	tx, err := pg.Tx(ctx)
	if err != nil {
		return 0, false, err
	}
	rows, err := sqlcgen.New(tx).LockConsumerCursor(ctx, string(name))
	if err != nil || len(rows) == 0 {
		return 0, false, err
	}
	return rows[0], true, nil
}

// AdvanceCursor sets the cursor of name to lastSeq (app.ConsumerCursorStore). It needs a unit of
// work; anything other than one updated row is kind internal.
func (r *Repository) AdvanceCursor(ctx context.Context, name sync.ConsumerName, lastSeq int64) error {
	tx, err := pg.Tx(ctx)
	if err != nil {
		return err
	}
	n, err := sqlcgen.New(tx).AdvanceConsumerCursor(ctx, sqlcgen.AdvanceConsumerCursorParams{LastSeq: lastSeq, Name: string(name)})
	if err != nil {
		return err
	}
	if n != 1 {
		return errs.New(errs.CodeInternalError, fmt.Sprintf("sync: moving the cursor of %s updated %d rows", name, n))
	}
	return nil
}

// ListCursors returns every cursor row in name order with the head of the log, in one statement
// (app.ConsumerCursorStore). Outside a unit of work it reads on the pool.
func (r *Repository) ListCursors(ctx context.Context) ([]app.CursorRow, error) {
	rows, err := r.queries(ctx).ListConsumerCursors(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]app.CursorRow, len(rows))
	for i, row := range rows {
		out[i] = app.CursorRow{Name: sync.ConsumerName(row.Name), LastSeq: row.LastSeq, HeadSeq: row.HeadSeq, UpdatedAt: row.UpdatedAt.Time.UTC()}
	}
	return out, nil
}

// OldestEventAfter returns occurred_at of the first event after after (app.ConsumerLogReader);
// ok is false when there is none. Outside a unit of work it reads on the pool.
func (r *Repository) OldestEventAfter(ctx context.Context, after int64) (time.Time, bool, error) {
	rows, err := r.queries(ctx).OldestEventAfter(ctx, after)
	if err != nil || len(rows) == 0 {
		return time.Time{}, false, err
	}
	return rows[0].Time.UTC(), true, nil
}

// MoveAllConsumerCursors sets last_seq of every worker consumer to headSeq in the unit of work of
// ctx (app.CursorMoveStore, bothy restore finalize).
func (r *Repository) MoveAllConsumerCursors(ctx context.Context, headSeq int64) (int64, error) {
	tx, err := pg.Tx(ctx)
	if err != nil {
		return 0, err
	}
	return sqlcgen.New(tx).MoveAllConsumerCursors(ctx, headSeq)
}

// Compile-time check: Repository moves the cursors of bothy restore finalize.
var _ app.CursorMoveStore = (*Repository)(nil)
