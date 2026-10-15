// SPDX-License-Identifier: Apache-2.0

package messaging

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Register declares the durable events messaging appends (M1-25, docs/spec/05-aktionen.md
// section 6): message.created (M1-56). internal/wire lists it in moduleRegistrations.
func Register(r *authorize.Registry) {
	r.Event("messaging", authorize.EventMessageCreated)
}

// MessagePosted is published inside the writing transaction for every new message, including
// system lines (02 sections 4.3 and 8.3). Subscribers (runs in M2, sync in M1-61, notify in M3)
// may write and may veto. It never carries content; subscribers that need it load the message
// through the port.
type MessagePosted struct {
	// MessageID is the new message; ThreadID, ChannelID and SpaceID its place.
	MessageID, ThreadID, ChannelID, SpaceID ids.ID
	// ThreadKind is "main" or "thread".
	ThreadKind string
	// ThreadSeq is the position of the message in its thread.
	ThreadSeq int64
	// AuthorID is the author; AuthorKind "user", "agent" or "app".
	AuthorID   ids.ID
	AuthorKind string
	// Origin is "human", "agent", "app" or "system" ("import" in M8).
	Origin string
	// Kind is "text" or "system" in M1.
	Kind string
	// ClientMsgID is the idempotency key of the message.
	ClientMsgID ids.ID
	// AuthorDeviceID is the device of the author's session; nil for agents, apps and system lines.
	AuthorDeviceID *ids.ID
	// ReplyToID is the quoted message; nil for none.
	ReplyToID *ids.ID
	// MentionedIDs are the resolved mentions, sorted, without duplicates.
	MentionedIDs []ids.ID
	// Silent suppresses notifications.
	Silent bool
	// ClearDraft asks to delete the author's draft of the thread (04 section 14, M1-61).
	ClearDraft bool
	// CreatedThread marks the first message of a thread created in the same transaction.
	CreatedThread bool
	// ReopenedThread marks the message that reopened the thread.
	ReopenedThread bool
	// CreatedAt is the creation time.
	CreatedAt time.Time
}

// PrincipalRef is the minimal public view of a principal (schema PrincipalRef).
type PrincipalRef struct {
	ID ids.ID
	// Kind is user, agent or app.
	Kind string
	// Handle and DisplayName are the public names.
	Handle, DisplayName string
	// AvatarFileID is zero for none.
	AvatarFileID ids.ID
}

// Message is a message as the API shows it (schema Message) without attachments, reactions,
// components and pins, which later tickets add. Zero values are absent members. It is the one
// message DTO of the port (the MessageView of brief M1-57): Create, CreateThread, History and Get
// return it. A deleted message is a tombstone: empty content, no mentions, DeletedAt set.
type Message struct {
	ID, ThreadID, ChannelID ids.ID
	ThreadSeq               int64
	Author                  PrincipalRef
	ClientMsgID             ids.ID
	// Origin and Kind are the column values; SystemCode is set exactly for kind system.
	Origin, Kind, SystemCode string
	// ContentMD is the content, full except in History; ContentFormat its dialect version.
	ContentMD     string
	ContentFormat int
	// ContentTruncated reports that History cut ContentMD at message.list_content_max_bytes; Get
	// reloads the full content (06 section 6 item 5).
	ContentTruncated bool
	// ReplyToID is the quoted message; zero for none.
	ReplyToID ids.ID
	Silent    bool
	// MentionIDs are the resolved mentions, sorted; never nil.
	MentionIDs []ids.ID
	Version    int64
	// EditedAt and DeletedAt are zero while not edited or deleted.
	EditedAt, DeletedAt, CreatedAt time.Time
}

// CreateMessageCmd is the request of createMessage (schema MessageCreate). Pointer and slice
// members are nil when absent.
type CreateMessageCmd struct {
	ThreadID    ids.ID
	ClientMsgID ids.ID
	// Kind is nil for text.
	Kind      *string
	ContentMD *string
	// ReplyToID is the quoted message; nil for none.
	ReplyToID *ids.ID
	// AttachmentIDs, Components and RunID are refused in M1 (brief D2) when not empty.
	AttachmentIDs []ids.ID
	Components    int
	RunID         *ids.ID
	// MentionIDs is the optional check of the resolved set; nil when absent.
	MentionIDs []ids.ID
	Silent     bool
	ClearDraft bool
}

// Created is the result of Messages.Create: the message and whether it is the original of an
// earlier identical request (200 with Idempotent-Replay).
type Created struct {
	Message Message
	Replay  bool
}

// CreateThreadCmd is the request of createThread (schema ThreadCreate).
type CreateThreadCmd struct {
	ChannelID ids.ID
	// Title is nil when absent.
	Title *string
	// ParentMessageID is nil for a forum post.
	ParentMessageID *ids.ID
	TagIDs          []ids.ID
	// FirstMessage is nil when absent; its ThreadID is ignored.
	FirstMessage *CreateMessageCmd
}

// ThreadCreated is the result of Messages.CreateThread: the new thread, its first message (nil
// without one) and whether it is the original of an earlier request with the same
// first_message.client_msg_id. The port of a module depends on no other module (02 section 7.2),
// so the caller reads the thread itself through chat.Threads.View (schema ThreadCreated).
type ThreadCreated struct {
	ThreadID     ids.ID
	FirstMessage *Message
	Replay       bool
}

// HistoryMode is the window of a History page (06 section 6 item 5); the zero value is the newest
// page.
type HistoryMode int

// The windows of listThreadMessages.
const (
	// HistoryNewest is the page without an anchor: the newest messages of the thread.
	HistoryNewest HistoryMode = iota
	// HistoryBefore (before_seq) is the highest messages with thread_seq below Anchor.
	HistoryBefore
	// HistoryAfter (after_seq) is the lowest messages with thread_seq above Anchor.
	HistoryAfter
	// HistoryAround (around_seq) is at most (limit - 1) / 2 messages below Anchor and the rest from
	// Anchor on; a short side gives its share to the other.
	HistoryAround
)

// HistoryQuery is the query of Messages.History (listThreadMessages).
type HistoryQuery struct {
	ThreadID ids.ID
	Mode     HistoryMode
	// Anchor is the thread_seq of the mode: at least 0 for HistoryAfter, at least 1 for
	// HistoryBefore and HistoryAround, below 2^53; ignored for HistoryNewest.
	Anchor int64
	// Limit is the page size; 0 means api.page_size_default, larger values are capped at
	// api.page_size_max.
	Limit int
}

// HistoryPage is one page of a thread's history: the items ascending by thread_seq without a gap,
// tombstones at the positions of deleted messages, and whether messages exist before the first and
// after the last item. Items and flags come from one snapshot.
type HistoryPage struct {
	Items         []Message
	HasMoreBefore bool
	HasMoreAfter  bool
}

// Messages is the incoming port of sending (M1-56) and reading (M1-57). The caller a is the principal of the request;
// errors are kernel/errs errors, and an unknown and an invisible thread or channel give the same
// 404 not_found.
type Messages interface {
	// Create sends a message into a thread (message.create, 04 section 5.1): idempotent by the
	// author and client_msg_id, with the gapless thread_seq, the origin of the principal kind,
	// resolved mentions and, in a closed thread, the reopen with its system line. It publishes
	// MessagePosted and appends message.created in the same transaction.
	//
	// Example:
	//
	//	c, err := msgs.Create(ctx, a, messaging.CreateMessageCmd{ThreadID: th, ClientMsgID: ids.New(), ContentMD: &text})
	Create(ctx context.Context, a actor.Actor, cmd CreateMessageCmd) (Created, error)
	// CreateThread creates a forum post or a thread from a message of the main thread
	// (thread.create on the channel), with an optional first message in the same transaction.
	CreateThread(ctx context.Context, a actor.Actor, cmd CreateThreadCmd) (ThreadCreated, error)
	// History returns one page of the history of a thread (listThreadMessages, thread.read; 06
	// section 6 item 5) for the principal of ctx (kernel/actor; anonymous when none). The page is a
	// keyset page over thread_seq, so a new message never changes it; every item passed
	// message.read in filter mode, and content above message.list_content_max_bytes is cut with
	// ContentTruncated. Errors: validation_failed for an anchor out of range; not_found for a
	// missing and an invisible thread alike; the refusal of thread.read. Callers: the REST handler,
	// runs (run context, M2) and search (M3).
	//
	// Example:
	//
	//	page, err := msgs.History(ctx, messaging.HistoryQuery{ThreadID: th, Mode: messaging.HistoryBefore, Anchor: 21, Limit: 10})
	History(ctx context.Context, q HistoryQuery) (HistoryPage, error)
	// Get returns one message with its full content (getMessage, message.read) for the principal of
	// ctx; a tombstone is returned too. Errors: not_found for a missing and an invisible message
	// alike; the refusal of message.read.
	Get(ctx context.Context, messageID ids.ID) (Message, error)
}
