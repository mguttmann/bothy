// SPDX-License-Identifier: Apache-2.0

package messaging

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Register declares the durable events messaging appends (M1-25, docs/spec/05-actions.md
// section 6): message.created (M1-56), message.updated and message.deleted (M1-58),
// reaction.added, reaction.removed, pin.added and pin.removed (M1-59). internal/wire lists it in
// moduleRegistrations.
func Register(r *authorize.Registry) {
	r.Event("messaging", authorize.EventMessageCreated)
	r.Event("messaging", authorize.EventMessageUpdated)
	r.Event("messaging", authorize.EventMessageDeleted)
	r.Event("messaging", authorize.EventReactionAdded)
	r.Event("messaging", authorize.EventReactionRemoved)
	r.Event("messaging", authorize.EventPinAdded)
	r.Event("messaging", authorize.EventPinRemoved)
}

// MessageDeleted is published inside the deleting transaction, after the tombstone, the removal
// of revisions and mentions and the redaction of the event payloads (M1-58, 02 section 4.3). runs
// subscribes in M2 (cancel the run of a deleted trigger message, delete its components). It never
// carries content.
type MessageDeleted struct {
	// MessageID is the deleted message; ThreadID and ChannelID its place.
	MessageID, ThreadID, ChannelID ids.ID
	// ThreadSeq is the position the tombstone keeps.
	ThreadSeq int64
	// AuthorID is the author; DeletedBy the principal who deleted (author or moderator).
	AuthorID, DeletedBy ids.ID
	// MentionedPrincipalIDs are the mentions the message had before the deletion, sorted.
	MentionedPrincipalIDs []ids.ID
	// Version is the version of the tombstone.
	Version int64
}

// UpdateCommand is the request of updateMessage (schema MessageUpdate) with its precondition.
// A pointer member is nil when the request leaves the field out; it never reaches JSON.
type UpdateCommand struct {
	MessageID ids.ID
	// IfMatch is the version of the required If-Match header; 0 for a value that is not a
	// version, which never matches.
	IfMatch int64
	// Content is the new content_md; nil keeps the current one.
	Content *string
	// MentionIDs is the optional check of the resolved mention set; nil when absent.
	MentionIDs *[]ids.ID
}

// RevisionView is one earlier version of a message (schema MessageRevision).
type RevisionView struct {
	// Revision is 1 for the original content, then ascending.
	Revision int
	// ContentMD is the content before the edit.
	ContentMD string
	// EditedBy is the principal who edited.
	EditedBy ids.ID
	// CreatedAt is the time of the edit.
	CreatedAt time.Time
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
	// Kind is "text", "system" or, for the live box anchor of a run (CreateRunAnchor, M2-05),
	// "run"; only a text message of origin human can trigger an agent (09 section 3.2).
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

// Message is a message as the API shows it (schema Message) without attachments and components,
// which later tickets add. Zero values are absent members. It is the one message DTO of the port
// (the MessageView of brief M1-57): Create, CreateThread, History and Get return it. A deleted
// message is a tombstone: empty content, no mentions, no reactions, not pinned, DeletedAt set.
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
	// RunID is the run of a live box anchor (kind run); zero otherwise.
	RunID ids.ID
	// TriggerState is messages.trigger_state (schema TriggerState); "" while NULL.
	// TriggerStateReason is set for failed and cancelled; TriggerStateAt is zero while NULL.
	TriggerState, TriggerStateReason string
	TriggerStateAt                   time.Time
	// MentionIDs are the resolved mentions, sorted; never nil.
	MentionIDs []ids.ID
	// Reactions is the summary per symbol, ordered by the earliest reaction of each symbol, then
	// by symbol; never nil (M1-59).
	Reactions []ReactionCount
	// MyReactions are the symbols of the principal of the request in the order of Reactions;
	// never nil. Only REST responses carry them, never events (schema Message).
	MyReactions []string
	// Pinned reports a pin of the message.
	Pinned  bool
	Version int64
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
	// SkipMentions makes the message resolve no mentions: the scanner does not run, no mentions row is written and
	// MessagePosted.MentionedIDs is empty (incoming webhooks, R-EX-09, M2-27). The content stays unchanged. MentionIDs
	// is ignored with it, because the caller of this path never sends mention_ids.
	SkipMentions bool
	// HashContext, when not empty, replaces the context part of the request hash (M1-56 D3), so the caller decides what a
	// repetition of client_msg_id is bound to (M2-27 design note 6c). Empty keeps the thread context of createMessage and
	// the new-thread context of createThread.
	HashContext string
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

// SetTriggerStateCmd is the input of Messages.SetTriggerState (M2-05).
type SetTriggerStateCmd struct {
	// MessageID is the triggering message; it must have origin human (03 messages check).
	MessageID ids.ID
	// State is a value of schema TriggerState.
	State string
	// Reason is trigger_state_reason; required for failed and cancelled, empty otherwise.
	Reason string
}

// RunAnchorCmd is the input of Messages.CreateRunAnchor (M2-05): the live box anchor of RunID by
// the agent AgentID in ThreadID (09 section 5.4).
type RunAnchorCmd struct {
	ThreadID, RunID, AgentID ids.ID
}

// TriggerMessage are the facts of a triggering message that runs reads at a delivery (M2-08
// design note 5): place, author, origin and kind, the mentions now and the trigger state. It carries
// no content.
type TriggerMessage struct {
	ID, ThreadID, ChannelID ids.ID
	// Author is the author as PrincipalRef (the payload member triggered_by).
	Author PrincipalRef
	// Origin is human, agent, app, import or system; Kind text, system or run.
	Origin, Kind string
	// MentionedIDs are the stored mentions, sorted; empty for a tombstone.
	MentionedIDs []ids.ID
	// TriggerState and TriggerStateReason are the stored values; "" while NULL.
	TriggerState, TriggerStateReason string
}

// AnchorRef names a written live box anchor.
type AnchorRef struct {
	// MessageID is the anchor; ThreadSeq its position in the thread.
	MessageID ids.ID
	ThreadSeq int64
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
	// Update edits a message of the caller (the principal of ctx; message.update, M1-58): 428 is
	// the HTTP chain's for a missing If-Match; 404 for a missing or invisible message, 403 for a
	// caller who is not the author or a read-only channel, 410 message_deleted, 412
	// version_conflict with the current representation, 413 and 400 for the content and
	// mention_ids. The same content and mentions answer the current message without a write;
	// otherwise the previous content becomes the next revision (not for a run anchor), the
	// mentions are resolved again and message.updated is appended. An edit never publishes
	// MessagePosted and never changes trigger_state.
	//
	// Example:
	//
	//	m, err := msgs.Update(ctx, messaging.UpdateCommand{MessageID: id, IfMatch: 3, Content: &text})
	Update(ctx context.Context, c UpdateCommand) (Message, error)
	// Delete deletes a message (message.delete, M1-58): the author with write, anyone else with
	// manage, audited. It is idempotent: an already deleted message answers nil. In one unit of
	// work the tombstone keeps the row and thread_seq, revisions and mentions go, the thread
	// counts the message down, the payloads of the stored message events are redacted,
	// MessageDeleted is published and message.deleted appended. Errors: 404 for a missing or
	// invisible message, 403 without the right.
	Delete(ctx context.Context, messageID ids.ID) error
	// Revisions returns the earlier versions of a message in ascending order
	// (message.revisions.read, M1-58); a deleted message has none. Errors: 404 for a missing or
	// invisible message.
	Revisions(ctx context.Context, messageID ids.ID) ([]RevisionView, error)
	// SetTriggerState sets trigger_state, trigger_state_reason and trigger_state_at of a triggering
	// message for runs (M2-05, 09 section 6). It never authorizes: the caller (runs) decided. It
	// runs only inside the caller's unit of work. The same state and reason as stored write nothing
	// (D-M2-05-17); otherwise the version rises by one and message.updated is appended with the
	// representation of M1-56 D7 (a deleted message keeps its tombstone form). Errors: internal
	// outside a unit of work, for an unknown message, a message whose origin is not human, an
	// unknown state or a reason that does not fit the state.
	//
	// Example:
	//
	//	err := msgs.SetTriggerState(ctx, messaging.SetTriggerStateCmd{MessageID: m, State: "received"})
	SetTriggerState(ctx context.Context, cmd SetTriggerStateCmd) error
	// CreateRunAnchor writes the live box anchor of a run inside the caller's unit of work (M2-05,
	// 09 section 5.4): the next thread_seq of the thread, kind run, origin agent, the agent as
	// author, empty content, silent, run_id, a server client_msg_id, no mentions; then
	// message.created and MessagePosted with Kind run. It never authorizes: runs accepted the
	// trigger. Errors: internal outside a unit of work; not_found for an unknown thread.
	CreateRunAnchor(ctx context.Context, cmd RunAnchorCmd) (AnchorRef, error)
	// TriggerMessageOf reads the trigger facts of a message for runs (M2-08): the delivery check
	// re-evaluates the trigger with the origin and the mentions of now. It never authorizes and
	// returns no content; runs decides with agent.invoke. Errors: not_found for an unknown message.
	//
	// Example:
	//
	//	m, err := msgs.TriggerMessageOf(ctx, run.TriggeringMessageID)
	TriggerMessageOf(ctx context.Context, messageID ids.ID) (TriggerMessage, error)
	// PrincipalRefs returns the public views (schema PrincipalRef) of the known principals among
	// principalIDs, whatever their status; an unknown id is absent (M2-16: runs renders the
	// invoker of a command, invoked_by and triggered_by, and may not use identity, 02 section
	// 4.4). It never authorizes: the caller shows only principals it already decided about.
	//
	// Example:
	//
	//	refs, err := msgs.PrincipalRefs(ctx, []ids.ID{alex})
	PrincipalRefs(ctx context.Context, principalIDs []ids.ID) (map[ids.ID]PrincipalRef, error)
}
