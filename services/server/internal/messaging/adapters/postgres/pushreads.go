// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/messaging"
	"github.com/OWNER/bothy/services/server/internal/messaging/adapters/postgres/sqlcgen"
)

// LastAnchorBefore implements app.PushReadStore with the statement LastAnchorBefore.
func (r *Repository) LastAnchorBefore(ctx context.Context, thread ids.ID, before int64) (messaging.AnchorRef, bool, error) {
	row, err := r.read(ctx).LastAnchorBefore(ctx, sqlcgen.LastAnchorBeforeParams{ThreadID: uuid(thread), BeforeThreadSeq: before})
	return optionalRow(messaging.AnchorRef{MessageID: idOf(row.ID), RunID: idOf(row.RunID), ThreadSeq: row.ThreadSeq}, err, "reading the anchor before a position")
}

// AnchorOf implements app.PushReadStore with the statement AnchorOfRun.
func (r *Repository) AnchorOf(ctx context.Context, run ids.ID) (messaging.AnchorRef, bool, error) {
	row, err := r.read(ctx).AnchorOfRun(ctx, uuid(run))
	return optionalRow(messaging.AnchorRef{MessageID: idOf(row.ID), RunID: idOf(row.RunID), ThreadSeq: row.ThreadSeq}, err, "reading the anchor of a run")
}

// LastAgentMessageBefore implements app.PushReadStore with the statement LastAgentMessageBefore.
func (r *Repository) LastAgentMessageBefore(ctx context.Context, thread, agent ids.ID, after, before int64) (messaging.MessageRef, bool, error) {
	row, err := r.read(ctx).LastAgentMessageBefore(ctx, sqlcgen.LastAgentMessageBeforeParams{
		ThreadID: uuid(thread), AuthorPrincipalID: uuid(agent), AfterThreadSeq: after, BeforeThreadSeq: before,
	})
	return optionalRow(messaging.MessageRef{ID: idOf(row.ID), ThreadSeq: row.ThreadSeq}, err, "reading the last agent message")
}

// Newest implements app.PushReadStore with the statement NewestMessage.
func (r *Repository) Newest(ctx context.Context, thread ids.ID) (messaging.MessageRef, bool, error) {
	row, err := r.read(ctx).NewestMessage(ctx, uuid(thread))
	return optionalRow(messaging.MessageRef{ID: idOf(row.ID), ThreadSeq: row.ThreadSeq}, err, "reading the newest message")
}

// optionalRow converts the result v of a single-row read: no row is the zero value with found false,
// never an error.
func optionalRow[T any](v T, err error, what string) (T, bool, error) {
	var zero T
	if errors.Is(err, pgx.ErrNoRows) {
		return zero, false, nil
	}
	if err != nil {
		return zero, false, mapError(err, what)
	}
	return v, true, nil
}
