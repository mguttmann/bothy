// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// ThreadKindMain is threads.kind of the main thread of a channel.
const ThreadKindMain = "main"

// Thread is one row of threads (03 section 4.5) as far as M1-53 writes it: the main thread of a
// text channel (R-DM-11), created with the channel. The thread use cases come with M1-55.
type Thread struct {
	ID, ChannelID ids.ID
	// Kind is main or thread; Title, TitleSource and State the columns of the same name.
	Kind, Title, TitleSource, State string
	// LastMessageSeq, UnreadFloorSeq and MessageCount are the counters, 0 for a new thread.
	LastMessageSeq, UnreadFloorSeq int64
	MessageCount                   int
	// CreatedBy is the creator; CreatedAt and LastActivityAt the times.
	CreatedBy                 ids.ID
	CreatedAt, LastActivityAt time.Time
	// Version is 1 for a new thread.
	Version int64
}

// NewMainThread returns the main thread of channel with the defaults of 03: title ”, title
// source auto, state open, no operator, no parent message (R-DM-11, R-DM-21).
func NewMainThread(id, channel, by ids.ID, at time.Time) Thread {
	return Thread{
		ID: id, ChannelID: channel, Kind: ThreadKindMain, TitleSource: "auto", State: "open", CreatedBy: by,
		CreatedAt: at, LastActivityAt: at, Version: 1,
	}
}
