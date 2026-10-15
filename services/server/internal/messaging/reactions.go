// SPDX-License-Identifier: Apache-2.0

package messaging

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// ReactionCount is one entry of the reaction summary of a message (schema ReactionSummary): the
// symbol and how many principals reacted with it.
type ReactionCount struct {
	Emoji string
	Count int
}

// ReactionListQuery is the request of listReactions (M1-59 design note 8).
type ReactionListQuery struct {
	MessageID ids.ID
	// Emoji filters the list to one symbol; nil lists every symbol. A given value passes the
	// symbol rule (400 with pointer #query/emoji otherwise).
	Emoji *string
	// Cursor is the opaque cursor of the previous page; "" for the first page.
	Cursor string
	// Limit is the page size; 0 means api.page_size_default, larger values are capped at
	// api.page_size_max.
	Limit int
}

// ReactionView is one reaction of the list (schema Reaction).
type ReactionView struct {
	Emoji     string
	Principal PrincipalRef
	CreatedAt time.Time
}

// ReactionPage is one page of listReactions (schema ReactionPage); NextCursor is "" on the last
// page.
type ReactionPage struct {
	Items      []ReactionView
	NextCursor string
}

// Reactions is the incoming port of reactions (M1-59). The caller is the principal in ctx
// (kernel/actor); errors are kernel/errs errors, and a missing and an invisible message give the
// same 404. A reaction is free expression and never a status: no operation changes
// trigger_state or the message version.
type Reactions interface {
	// Add adds the caller's reaction emoji to the message (reaction.add, write; 403
	// channel_read_only in a read-only channel). It is idempotent: a repetition changes nothing
	// and appends nothing; a new reaction appends reaction.added with the count after the
	// insert. Errors: 400 validation_failed invalid_emoji (pointer #path/emoji) before any
	// lookup, 404, 403, 410 message_deleted, 400 limit_reached for one symbol more than
	// reaction.max_distinct_per_message.
	//
	// Example:
	//
	//	err := reactions.Add(ctx, messageID, "\U0001F44D")
	Add(ctx context.Context, messageID ids.ID, emoji string) error
	// Remove removes the caller's own reaction emoji (reaction.remove); nothing to remove and a
	// deleted message answer nil without an event, a removal appends reaction.removed with the
	// count after the delete. Errors: 400 invalid_emoji, 404, 403.
	Remove(ctx context.Context, messageID ids.ID, emoji string) error
	// List returns one page of the reactions of a message (message.read), ascending by creation
	// time, then principal, then symbol; a deleted message has an empty page. Errors: 400
	// invalid_emoji (pointer #query/emoji), 400 cursor_invalid for a cursor of another list,
	// message or filter, 404.
	List(ctx context.Context, q ReactionListQuery) (ReactionPage, error)
}

// PinView is one pinned message (schema Pin).
type PinView struct {
	MessageID, ThreadID, PinnedBy ids.ID
	PinnedAt                      time.Time
}

// Pins is the incoming port of pinning (M1-59): moderation with manage. The caller is the
// principal in ctx; errors are kernel/errs errors.
type Pins interface {
	// Pin pins the message in its thread (pin.create, manage; allowed in a read-only channel).
	// An already pinned message answers nil without an event; a new pin appends pin.added.
	// Errors: 404 for a missing or invisible message, 403 without manage, 410 message_deleted, 400
	// limit_reached (empty pointer) when the thread holds pin.max_per_thread pins.
	//
	// Example:
	//
	//	err := pins.Pin(ctx, messageID)
	Pin(ctx context.Context, messageID ids.ID) error
	// Unpin removes the pin of the message (pin.delete); nothing to remove and a deleted message
	// answer nil without an event, a removal appends pin.removed with the former pin. Errors: 404,
	// 403.
	Unpin(ctx context.Context, messageID ids.ID) error
	// List returns the pins of a thread (thread.read), newest pin first, then by message id
	// descending; pins of deleted messages are absent, and each item passed message.read in filter
	// mode. Unpaginated, bounded by pin.max_per_thread. Errors: 404 for a missing or invisible
	// thread.
	List(ctx context.Context, threadID ids.ID) ([]PinView, error)
}
