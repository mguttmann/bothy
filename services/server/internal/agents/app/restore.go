// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/agents"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// CursorMoveStore is the outgoing port of RestoreCursors: one statement that sets every cursor
// row to a head (adapters/postgres). It runs in the ambient transaction of ctx.
type CursorMoveStore interface {
	// MoveAllConsumers sets agent_consumer_cursors.acked_seq of every row to headSeq and returns the number of rows.
	MoveAllConsumers(ctx context.Context, headSeq int64) (int64, error)
}

// restoreCursors implements agents.RestoreCursors over a CursorMoveStore.
type restoreCursors struct {
	store CursorMoveStore
}

// Compile-time check: restoreCursors is the port.
var _ agents.RestoreCursors = restoreCursors{}

// NewRestoreCursors returns the port of bothy restore finalize over store (M7-04).
func NewRestoreCursors(store CursorMoveStore) agents.RestoreCursors {
	return restoreCursors{store: store}
}

// MoveCursorsTo implements agents.RestoreCursors. It refuses to run outside a unit of work, because
// the cursors must move in the transaction of the new sync epoch.
func (r restoreCursors) MoveCursorsTo(ctx context.Context, headSeq int64) error {
	if !txn.InUnit(ctx) {
		return errs.New(errs.CodeInternalError, "agents: MoveCursorsTo needs a unit of work")
	}
	_, err := r.store.MoveAllConsumers(ctx, headSeq)
	return err
}
