// SPDX-License-Identifier: Apache-2.0

package sync

import "context"

// RestoreCursors is the port of bothy restore finalize to the cursors of this module
// (event_consumer_cursors.last_seq; docs/spec/03-datenmodell.md 6.7 item 1, ADR 0059 item 1). The module instance calls it
// through internal/wire inside its own unit of work, so the cursors move in the transaction that
// sets the new sync epoch, or not at all.
type RestoreCursors interface {
	// MoveCursorsTo sets every cursor of the module to headSeq, the head of the restored event
	// log, in the unit of work of ctx: no receiver catches up events of the old timeline. Errors:
	// kind internal outside a unit of work, the database error otherwise.
	//
	// Example (inside instance's unit of work):
	//
	//	err := cursors.MoveCursorsTo(ctx, head)
	MoveCursorsTo(ctx context.Context, headSeq int64) error
}
