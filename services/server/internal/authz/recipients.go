// SPDX-License-Identifier: Apache-2.0

package authz

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Recipients answers who may be a recipient of a hint in a channel (13 section 12 point 2,
// docs/DECISIONS.md D-M3-19-1). It does not call authorize and applies no guard: the answer is a
// superset that the caller decides exactly per candidate with authorize(r, push.deliver, thread)
// in filter mode, so a candidate the guards exclude never gets a hint.
type Recipients interface {
	// Readers returns the active humans that a source of 07 makes able to read the channel:
	// owners (not for DM channels, SR6), grants on the instance, the space or the channel to the
	// human or to a valid group of the human, and the participants of a DM channel; ascending
	// and without duplicates.
	//
	// Errors: not_found for an unknown channel; those of the reads.
	Readers(ctx context.Context, channelID ids.ID) ([]ids.ID, error)
}
