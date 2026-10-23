// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"

	"github.com/OWNER/bothy/services/server/internal/chat"
)

// Subscribers holds the in-transaction subscribers of files (02 section 4.3): OnThreadMoved
// (M3-07). internal/wire builds it with the module objects of the txn registry over a repository
// whose pool may be nil, because the subscriber writes only in the transaction of the move; so it
// opens no connection.
type Subscribers struct {
	rows ThreadAttachments
}

// NewSubscribers returns the subscribers over rows. Errors: a nil rows.
//
// Example:
//
//	subs, err := app.NewSubscribers(postgres.NewRepository(pool))
//	err = txn.Subscribe(reg, subs.OnThreadMoved)
func NewSubscribers(rows ThreadAttachments) (*Subscribers, error) {
	if rows == nil {
		return nil, errors.New("files: NewSubscribers needs the attachment rows")
	}
	return &Subscribers{rows: rows}, nil
}

// OnThreadMoved runs in the transaction of thread.move (brief M3-07 design note 8, R-DM-43) and
// sets the copy channel_id of every attachment of the thread to the target channel, so an attached
// file is read through the channel of its message from the commit on (05 2.2). It writes no
// version and no event, and files.scope_channel_id stays: it counts only before attaching. It is
// registered after the subscribers of messaging and sync, so the move locks thread, messages, the
// rows of sync and then message_attachments (03 6.3). A rollback of the move rolls the copies back
// with it.
func (s *Subscribers) OnThreadMoved(ctx context.Context, ev chat.ThreadMoved) error {
	return s.rows.MoveThreadAttachments(ctx, ev.ThreadID, ev.ToChannelID)
}
