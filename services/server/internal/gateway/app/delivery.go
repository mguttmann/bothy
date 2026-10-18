// SPDX-License-Identifier: Apache-2.0

package app

import "context"

// Delivery is the outgoing port to the delivery of events (the hub of M1-65). The gateway calls
// Register when a session becomes ready on a connection, before it reads the head for ready or
// resumed, so every later event is buffered and every earlier one is in the snapshot or the
// catch-up (04 section 6.3 step 4); Attach once the client is known to be at fromSeq (the cursor,
// or the head after ready and after resync_required); Detach when the connection ends. Until
// M1-65 internal/wire binds an implementation that delivers nothing.
type Delivery interface {
	// Register announces s with its connection to delivery; events from now on are buffered.
	Register(s *Session)
	// Attach starts delivery for s after fromSeq. Errors end the connection with 4000.
	Attach(ctx context.Context, s *Session, fromSeq int64) error
	// Detach ends delivery for s on its connection.
	Detach(s *Session)
}
