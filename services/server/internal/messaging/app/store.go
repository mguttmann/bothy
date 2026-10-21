// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/messaging"
	"github.com/OWNER/bothy/services/server/internal/messaging/domain"
)

// ErrClientMsgIDTaken is the error of Store.InsertMessage when the author already has a message
// with the client_msg_id: a concurrent identical request committed first (04 section 5.1 point 4).
// The use case rolls back and answers like a repetition.
var ErrClientMsgIDTaken = errors.New("messaging: the author already used this client_msg_id")

// Store is the outgoing port of the message rows; adapters/postgres implements it over messages
// and mentions. Reads join the ambient transaction of ctx; writes need one. Errors are
// kernel/errs errors except ErrClientMsgIDTaken.
type Store interface {
	// Message reads the message id; found is false for an unknown id.
	Message(ctx context.Context, id ids.ID) (m domain.Message, found bool, err error)
	// MessageByClientID reads the message of author with clientMsgID (the idempotency lookup).
	MessageByClientID(ctx context.Context, author, clientMsgID ids.ID) (m domain.Message, found bool, err error)
	// InsertMessage writes m and returns it as stored. Errors: ErrClientMsgIDTaken (wrapped) for
	// a second (author, client_msg_id).
	InsertMessage(ctx context.Context, m domain.Message) (domain.Message, error)
	// SetTriggerState sets trigger_state, trigger_state_reason (NULL for "") and trigger_state_at
	// of the message id of origin human and raises its version, only when state or reason differ
	// from the stored ones (M2-05). changed is false, and nothing is written, for an unknown id,
	// another origin and the same state and reason; the caller tells them apart by reading.
	SetTriggerState(ctx context.Context, id ids.ID, state, reason string, at time.Time) (m domain.Message, changed bool, err error)
	// InsertMentions writes one mentions row per principal of message in thread, in one statement.
	InsertMentions(ctx context.Context, message, thread ids.ID, principals []ids.ID, at time.Time) error
	// Mentions returns the mentioned principals of message, sorted.
	Mentions(ctx context.Context, message ids.ID) ([]ids.ID, error)
	// MyMentions returns up to q.Limit mentions of q.PrincipalID in q.Visible without tombstones,
	// newest first, after the keyset position of q when q.HasAfter.
	MyMentions(ctx context.Context, q MentionQuery) ([]MentionRow, error)
	// CountUnreadMentions implements the statement of messaging.Mentions.CountUnread.
	CountUnreadMentions(ctx context.Context, principal ids.ID, visible []ids.ID, lastRead map[ids.ID]int64) ([]messaging.ThreadMentionCount, error)

	// PersonalTemplates returns the templates of owner without deleted ones, by lower(name), id.
	PersonalTemplates(ctx context.Context, owner ids.ID) ([]domain.Template, error)
	// ChannelTemplates returns the personal templates of caller and the templates of channel
	// when it is in visible, without deleted ones, by lower(name), id.
	ChannelTemplates(ctx context.Context, caller, channel ids.ID, visible []ids.ID) ([]domain.Template, error)
	// LockVisibleTemplate reads and locks the template id when it belongs to caller or to a
	// channel in visible; withDeleted includes a deleted one. found is false otherwise.
	LockVisibleTemplate(ctx context.Context, id, caller ids.ID, visible []ids.ID, withDeleted bool) (t domain.Template, found bool, err error)
	// LockTemplateScope takes the transaction advisory lock of one scope (brief note 9).
	LockTemplateScope(ctx context.Context, key int64) error
	// CountTemplates counts the templates of the scope of t (its owner or its channel) without
	// deleted ones.
	CountTemplates(ctx context.Context, t domain.Template) (int, error)
	// InsertTemplate writes t with version 1 and returns it as stored.
	InsertTemplate(ctx context.Context, t domain.Template) (domain.Template, error)
	// UpdateTemplate writes name and content of t and raises the version.
	UpdateTemplate(ctx context.Context, t domain.Template) (domain.Template, error)
	// DeleteTemplate sets deleted_at of the template id and raises the version.
	DeleteTemplate(ctx context.Context, id ids.ID, at time.Time) error

	// History runs the one statement of q.Mode (brief M1-57 note 4) and returns its rows.
	History(ctx context.Context, q HistoryRead) (HistoryRows, error)
	// VisibleMessage reads the message id when its channel is one of visible; found is false for
	// an unknown id and for a message in another channel alike.
	VisibleMessage(ctx context.Context, id ids.ID, visible []ids.ID) (m domain.Message, found bool, err error)

	// The statements of editing and deleting (M1-58) carry the visible channels of the caller
	// (07 section 7.4 rule 1): a message outside them reads like a missing one; they read with
	// VisibleMessage above (INT-W22/4).

	// LockCard reads the message id without a visibility predicate and locks its row until the
	// end of the unit of work (M2-15, Messages.LockCard): runs authorized the change on the card's
	// path before, or a job or the end of a run names the card.
	LockCard(ctx context.Context, id ids.ID) (m domain.Message, found bool, err error)
	// RaiseVersion raises the version of the locked message id by one and returns the row
	// (M2-15, Messages.ComponentsChanged).
	RaiseVersion(ctx context.Context, id ids.ID) (domain.Message, error)

	// LockMessage reads the message id like VisibleMessage and locks its row until the end of
	// the unit of work.
	LockMessage(ctx context.Context, id ids.ID, visible []ids.ID) (m domain.Message, found bool, err error)
	// UpdateContent sets the content of the locked message id, edited_at to at and the next
	// version, and returns the row.
	UpdateContent(ctx context.Context, id ids.ID, visible []ids.ID, content string, at time.Time) (domain.Message, error)
	// Tombstone deletes the locked message id (domain.Tombstone in SQL) and returns the row.
	Tombstone(ctx context.Context, id ids.ID, visible []ids.ID, by ids.ID, at time.Time) (domain.Message, error)
	// InsertRevision writes r with the next revision number of its message.
	InsertRevision(ctx context.Context, r domain.Revision, visible []ids.ID) error
	// Revisions returns the revisions of message in ascending order.
	Revisions(ctx context.Context, message ids.ID, visible []ids.ID) ([]domain.Revision, error)
	// DeleteRevisions removes every revision of message.
	DeleteRevisions(ctx context.Context, message ids.ID, visible []ids.ID) error
	// DeleteMentionsExcept removes the mentions of message whose principal is not in keep.
	DeleteMentionsExcept(ctx context.Context, message ids.ID, visible, keep []ids.ID) error

	// The statements of reactions and pins (M1-59) join messages with the visible channels of
	// the caller (brief design note 13); the writes run under the row lock of LockMessage.

	// ReactionStats returns the number of distinct symbols of message and the number of
	// reactions with emoji.
	ReactionStats(ctx context.Context, message ids.ID, emoji string, visible []ids.ID) (distinct, ofEmoji int, err error)
	// InsertReaction writes the reaction of principal with emoji at; inserted is false when it
	// exists.
	InsertReaction(ctx context.Context, message, principal ids.ID, emoji string, at time.Time, visible []ids.ID) (inserted bool, err error)
	// DeleteReaction removes the reaction of principal with emoji; deleted is false when there
	// was none.
	DeleteReaction(ctx context.Context, message, principal ids.ID, emoji string, visible []ids.ID) (deleted bool, err error)
	// DeleteReactions removes every reaction of message (its deletion, M1-59 design note 9).
	DeleteReactions(ctx context.Context, message ids.ID, visible []ids.ID) error
	// ReactionGroups returns the symbols of each live message of messages with count and first
	// reaction, in one statement; a message without reactions is absent.
	ReactionGroups(ctx context.Context, messages, visible []ids.ID) (map[ids.ID][]domain.ReactionGroup, error)
	// OwnReactions returns the symbols principal reacted with on each of messages, in one
	// statement.
	OwnReactions(ctx context.Context, principal ids.ID, messages, visible []ids.ID) (map[ids.ID]map[string]bool, error)
	// ListReactions returns up to q.Limit reactions of one live message in keyset order.
	ListReactions(ctx context.Context, q ReactionRead) ([]ReactionRow, error)
	// Pinned returns the live messages of messages that have a pin, in one statement.
	Pinned(ctx context.Context, messages, visible []ids.ID) (map[ids.ID]bool, error)
	// Pin reads the pin of message; found is false without one.
	Pin(ctx context.Context, message ids.ID, visible []ids.ID) (p PinRow, found bool, err error)
	// CountPins counts the pins of thread.
	CountPins(ctx context.Context, thread ids.ID, visible []ids.ID) (int, error)
	// InsertPin writes the pin p of its message in the thread of that message; inserted is
	// false when the message is pinned already.
	InsertPin(ctx context.Context, p PinRow, visible []ids.ID) (inserted bool, err error)
	// DeletePin removes the pin of message and returns it; found is false without one.
	DeletePin(ctx context.Context, message ids.ID, visible []ids.ID) (p PinRow, found bool, err error)
	// ThreadPins returns the pins of the live messages of thread, newest pin first, then by
	// message id descending.
	ThreadPins(ctx context.Context, thread ids.ID, visible []ids.ID) ([]PinRow, error)
}

// ReactionRead is the input of Store.ListReactions.
type ReactionRead struct {
	MessageID ids.ID
	Visible   []ids.ID
	// Emoji filters to one symbol; "" reads every symbol.
	Emoji string
	// HasAfter starts after the keyset position After.
	HasAfter bool
	After    domain.ReactionKey
	Limit    int
}

// ReactionRow is one row of reactions.
type ReactionRow struct {
	PrincipalID ids.ID
	Emoji       string
	CreatedAt   time.Time
}

// PinRow is one row of pins; ChannelID and AuthorID are those of the pinned message (ThreadPins
// only), for its check with message.read.
type PinRow struct {
	MessageID, ThreadID, PinnedBy ids.ID
	PinnedAt                      time.Time
	ChannelID, AuthorID           ids.ID
}

// MentionQuery is one batch of the mention list (Store.MyMentions).
type MentionQuery struct {
	PrincipalID ids.ID
	Visible     []ids.ID
	// HasAfter starts after the keyset position (AfterAt, AfterID).
	HasAfter bool
	AfterAt  time.Time
	AfterID  ids.ID
	Limit    int
}

// MentionRow is one mention of the list: the message and the time of the mention.
type MentionRow struct {
	Message     domain.Message
	MentionedAt time.Time
}

// HistoryRead is the input of Store.History.
type HistoryRead struct {
	ThreadID ids.ID
	// Visible are the visible channels of message.read (Authorizer.Scope); every read carries
	// channel_id = ANY(Visible).
	Visible []ids.ID
	Mode    domain.HistoryMode
	Anchor  int64
	// RowLimit is the most rows per side: the page size plus one.
	RowLimit int
}

// HistoryRows are the rows of one history statement, all from one snapshot.
type HistoryRows struct {
	// Below are the rows below the anchor (the newest page and before_seq: all rows), descending
	// by thread_seq; Above the rows from the anchor on (after_seq: above it), ascending.
	Below, Above []domain.Message
	// Opposite is the EXISTS of the side the mode does not read (before_seq: thread_seq >= anchor;
	// after_seq: thread_seq <= anchor); false for the other modes.
	Opposite bool
}
