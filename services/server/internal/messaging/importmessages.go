// SPDX-License-Identifier: Apache-2.0

package messaging

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// ImportedMessage is one message of an import batch (22 6.2, M8-11 design note 4).
type ImportedMessage struct {
	// ImportItemID is the import_items row of the message (messages.import_item_id).
	ImportItemID ids.ID
	// AuthorID is the mapped author: a person, an agent or a placeholder.
	AuthorID ids.ID
	// ContentMD is the converted content of the tool, stored byte for byte.
	ContentMD string
	// CreatedAt is the time of the source; EditedAt its edit time, zero for none (no revision).
	CreatedAt, EditedAt time.Time
	// ReplyToID is the reply target in the same thread; zero for none.
	ReplyToID ids.ID
	// AttachmentIDs are files uploaded by the author for the channel of the thread, in order.
	AttachmentIDs []ids.ID
}

// ImportMessages is the incoming port of messaging for the content of an import (M8-11 design
// note 7). It authorizes nothing (the batch was authorized with import.apply) and runs only inside
// the caller's unit of work (internal error otherwise). An imported message has origin import,
// kind text, silent, no mentions rows and no trigger_state, a client_msg_id of the server, the
// instance search language and no revision; Append publishes no MessagePosted and appends no
// message.created, so no run, invocation, notification or push follows (05 section 5 rule 1, 04
// 15.8), and Pin appends no pin.added. Errors are kernel/errs errors whose field pointers name the
// members of the import data (/content_md, /reply_to_origin_key, /attachment_file_ids).
type ImportMessages interface {
	// Append inserts msgs in the given order at the next numbers of the thread
	// (chat.ImportThreads.Reserve) and returns their ids in the same order. Errors:
	// validation_failed on /content_md (too_long, empty, invalid), /reply_to_origin_key
	// (not_in_thread) or /attachment_file_ids (limit_reached); conflict file_not_attachable from
	// files with the pointer of the entry in detail.
	Append(ctx context.Context, threadID ids.ID, msgs []ImportedMessage) ([]ids.ID, error)
	// ThreadOf returns the thread of a message, deleted ones included; found is false for an
	// unknown id.
	ThreadOf(ctx context.Context, messageID ids.ID) (threadID ids.ID, found bool, err error)
	// Pin pins the live message of threadID with pinnedBy as pinned_by, without pin.added; a pinned
	// message is no change. Errors: validation_failed on /message_origin_key for a message of
	// another thread or a deleted one (not_in_thread), limit_reached above pin.max_per_thread.
	Pin(ctx context.Context, threadID, messageID, pinnedBy ids.ID) error
}
