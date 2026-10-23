// SPDX-License-Identifier: Apache-2.0

package sync

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// ReadPositions reads stored read positions (read_states.last_read_seq) of people the caller
// already selected, for the worker consumer push and the jobs of notify (brief M3-19 design notes
// 9 d and 16 a). It never calls authorize and has no caller in ctx: the consumer push selected the
// people, and every hint and badge count is checked with push.deliver on its own. It is a port of
// its own beside ReadStates, whose methods all act for the caller in ctx. Implementations read in
// the ambient transaction of ctx when there is one.
type ReadPositions interface {
	// PositionsFor returns the position of each person of userIDs in threadID. Every requested
	// person gets an entry; a person without a row reads as 0 (03 "Unread": the implicit
	// position). No people give an empty map.
	//
	// Errors: those of the read (kernel/errs, internal).
	PositionsFor(ctx context.Context, threadID ids.ID, userIDs []ids.ID) (map[ids.ID]int64, error)
	// PositionsOfUser returns the position of userID in each thread of threadIDs. Every requested
	// thread gets an entry; a thread without a row reads as 0. No threads give an empty map.
	//
	// Errors: those of the read (kernel/errs, internal).
	PositionsOfUser(ctx context.Context, userID ids.ID, threadIDs []ids.ID) (map[ids.ID]int64, error)
}
