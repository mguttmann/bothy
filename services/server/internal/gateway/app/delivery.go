// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/gateway/domain"
)

// Delivery is the outgoing port to the delivery of events, implemented by the Hub (M1-65). The
// gateway calls Register when a session becomes ready on a connection, before it reads the head
// for ready or resumed, so every later event is buffered and every earlier one is in the snapshot
// or the catch-up (04 section 6.3 step 4); Attach once the client is known to be at from (its
// cursor, or the head after ready and after resync_required) with head, the head read for ready or
// resumed; Detach when the connection ends; Focus after a successful focus op.
type Delivery interface {
	// Register announces the connection c with its session s; events from now on are buffered.
	Register(ctx context.Context, c *Connection, s *Session)
	// Attach starts delivery on c after from up to head (catch-up) and live after head. Errors
	// end the connection with 4000.
	Attach(ctx context.Context, c *Connection, from, head int64) error
	// Detach ends delivery on c.
	Detach(c *Connection)
	// Focus replaces the focus of c and of its session with f, whose entries the focus handler
	// has checked (M1-66): focus-bound ephemeral events reach c only for these threads.
	Focus(c *Connection, f domain.Focus)
}
