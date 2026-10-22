// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/apps"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// CursorMoveStore is the outgoing port of RestoreCursors: one statement that sets every cursor
// row to a head (adapters/postgres). It runs in the ambient transaction of ctx.
type CursorMoveStore interface {
	// MoveAllSubscriptionCursors sets webhook_subscriptions.last_delivered_seq of every row to headSeq and returns the number of rows.
	MoveAllSubscriptionCursors(ctx context.Context, headSeq int64) (int64, error)
}

// restoreCursors implements apps.RestoreCursors over a CursorMoveStore.
type restoreCursors struct {
	store CursorMoveStore
}

// Compile-time check: restoreCursors is the port.
var _ apps.RestoreCursors = restoreCursors{}

// NewRestoreCursors returns the port of bothy restore finalize over store (M7-04).
func NewRestoreCursors(store CursorMoveStore) apps.RestoreCursors {
	return restoreCursors{store: store}
}

// MoveCursorsTo implements apps.RestoreCursors. It refuses to run outside a unit of work, because
// the cursors must move in the transaction of the new sync epoch.
func (r restoreCursors) MoveCursorsTo(ctx context.Context, headSeq int64) error {
	if !txn.InUnit(ctx) {
		return errs.New(errs.CodeInternalError, "apps: MoveCursorsTo needs a unit of work")
	}
	_, err := r.store.MoveAllSubscriptionCursors(ctx, headSeq)
	return err
}
