// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// ThreadMoveRows is the outgoing port the subscriber of chat.ThreadMoved uses; adapters/postgres
// implements it over messages.
type ThreadMoveRows interface {
	// MoveThreadMessages sets channel_id = channel on every message of thread, deleted ones
	// included, without a version change. It needs a unit of work.
	MoveThreadMessages(ctx context.Context, thread, channel ids.ID) error
}

// Subscribers holds the in-transaction subscribers of messaging (02 section 4.3): OnThreadMoved
// (M1-62). internal/wire builds it with the module objects of the txn registry, so it opens no
// connection.
type Subscribers struct {
	rows ThreadMoveRows
}

// NewSubscribers returns the subscribers over rows. Errors: a nil rows.
func NewSubscribers(rows ThreadMoveRows) (*Subscribers, error) {
	if rows == nil {
		return nil, errors.New("messaging: NewSubscribers needs the message rows")
	}
	return &Subscribers{rows: rows}, nil
}

// OnThreadMoved runs in the transaction of thread.move (brief M1-62 note 1f) and changes the copy
// messages.channel_id of every message of the thread to the target channel, so a message is never
// read at the old place after the commit (03 threads). It writes no version and no event: clients
// learn the new channel from thread.moved. A rollback of the move rolls the copy back with it.
func (s *Subscribers) OnThreadMoved(ctx context.Context, ev chat.ThreadMoved) error {
	return s.rows.MoveThreadMessages(ctx, ev.ThreadID, ev.ToChannelID)
}
