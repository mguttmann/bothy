// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// The outgoing ports of the gateway across several api nodes (M7-17, docs/spec/02-architecture.md
// sections 11.6 and 16; docs/decisions.d/M7-17.md). adapters/postgres implements both.

// SignalSender sends one encoded gateway signal (domain.EncodeSignal) on the channel ephemeral, in
// its own statement outside any transaction, to every node that listens, the sender included
// (each node ignores its own). It is safe for concurrent use. Errors: those of the statement; the
// caller logs them and drops the signal, because ephemeral events are at most once (04 9 point 4).
type SignalSender interface {
	Send(ctx context.Context, payload []byte) error
}

// PresenceRow is one counted connection of this node for the table gateway_presence (03 4.16,
// R-DM-51): a live connection of a human with a device (D-M3-23-2), its gateway session, the
// focused threads after the checks of the gateway and the idle flag of the last focus op.
type PresenceRow struct {
	SessionID, PrincipalID, DeviceID ids.ID
	// Focus is never nil; empty for a connection without focus.
	Focus []ids.ID
	Idle  bool
}

// PresenceStore is the outgoing port of the presence writer over gateway_presence (M7-17 Design
// note 2): only the gateway, the owner of the table, writes it (03 principle 12). Implementations
// are safe for concurrent use.
type PresenceStore interface {
	// Flush makes the rows of node equal rows, every one with updated_at = at (an upsert, so rows
	// lost with the UNLOGGED table come back), deletes the rows of node whose session is not in
	// rows, and deletes the rows of any node with updated_at before staleBefore (a node that
	// stopped refreshing, cluster.presence_ttl). Errors: those of the database; nothing is
	// half-applied.
	Flush(ctx context.Context, node ids.ID, rows []PresenceRow, at, staleBefore time.Time) error
	// DeleteNode deletes every row of node (a clean shutdown). Errors: those of the database.
	DeleteNode(ctx context.Context, node ids.ID) error
}
