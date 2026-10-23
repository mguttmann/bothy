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
	// RunID is the run this message belongs to: for kind run the anchored run (messages.run_id);
	// for an agent message of any other kind the run the request named in run_id, after the body
	// rules (M2-19 design note 3), which runs checks before the commit (design note 4); nil
	// otherwise. It is never stored for an agent message.
	RunID *ids.ID
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
	// HasAttachments reports that the message was created with at least one attachment (M3-07);
	// runs holds the agent invocation while one of them is not final (04 section 15.3 point 1).
	HasAttachments bool
	// CreatedAt is the creation time.
	CreatedAt time.Time
}

// MessageRef names one message by ID and position in its thread.
type MessageRef struct {
	// ID is the message.
	ID ids.ID
	// ThreadSeq is its position in the thread.
	ThreadSeq int64
}

// Attachment is one attachment of a message (schema Attachment, M3-07): its position from 1 and
// its file. The port of messaging depends on no other module (02 section 7.2), so the file is the
// messaging mirror of files.File in its attached form, without scope_channel_id.
type Attachment struct {
	Position int
	File     AttachedFile
}

// AttachedFile is the schema File of an attached file: zero values are absent members. SHA256 is
// set only once UploadState is complete; scope_channel_id is never shown once a file is attached
// (docs/decisions.d/M3-07.md D-M3-07).
type AttachedFile struct {
	ID        ids.ID
	SHA256    string
	SizeBytes int64
	// MIMEType is the detected type; OriginalName the cleaned name (never logged).
	MIMEType, OriginalName string
	UploadedBy             ids.ID
	// Purpose is attachment; UploadState pending, complete or failed; ScanStatus pending, clean,
	// infected, error or skipped.
	Purpose, UploadState, ScanStatus string
	// Width, Height and DurationMS are zero when unknown.
	Width, Height int
	DurationMS    int64
	// TranscriptStatus is none, pending, done or failed; TranscriptLanguage and TranscriptSource
	// are empty when absent.
	TranscriptStatus, TranscriptLanguage, TranscriptSource string
	// Thumbnails are the stored variants in the order small, medium, large, poster; never nil.
	Thumbnails []AttachedThumbnail
	CreatedAt  time.Time
}

// AttachedThumbnail is one stored variant of an attached file (schema Thumbnail).
type AttachedThumbnail struct {
	Variant, MIMEType string
	Width, Height     int
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

// Message is a message as the API shows it (schema Message). Zero values are absent members. It
// is the one message DTO of the port (the MessageView of brief M1-57): Create, CreateThread, History and Get return it. A deleted
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
	Pinned bool
	// Components are the components of a card in position order, read through ComponentReader
	// (M2-15); never nil. A tombstone has none.
	Components []Component
	// HasAttachments is messages.has_attachments (M3-07): true exactly when the message was
	// created with at least one attachment and is not deleted. The representation reads the
	// attachments only for such messages.
	HasAttachments bool
	// Attachments are the attachments in position order with the file state at the time of the
	// read (M3-07); never nil, empty for a message without attachments and for a tombstone.
	Attachments []Attachment
	Version     int64
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
	// AttachmentIDs are the files of attachment_file_ids in request order (M3-07): at most
	// message.max_attachments, each attachable by the author (docs/spec/12-files.md section 10).
	AttachmentIDs []ids.ID
	// Components are the members of components in request order; nil when absent (M2-15).
	Components []ComponentInput
	// RunID is the run an agent's message belongs to (M2-19): refused for every other principal,
	// checked by runs and never stored on the message; nil when absent. With kind card the
	// components bind to this run (M2-15, message_components.run_id) and the card's request hash
	// carries it (bothy-msg-v2); a text answer's hash does not.
	RunID *ids.ID
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

// SystemAgentSessionResumeFailed is the system_code of the fail-open line of an agent session (03
// messages, R-DM-42, 09 9.5 item 4), the Code runs passes to AppendSystemLine when the runtime no
// longer knew the bound session.
const SystemAgentSessionResumeFailed = "agent_session.resume_failed"

// SystemLine is the input of Messages.AppendSystemLine (M2-12 design note 12): a system line with
// Code in ThreadID whose author is AuthorID. Code is a system_code of 03 messages, for example
// agent_session.resume_failed with the run's agent as author (R-DM-42).
type SystemLine struct {
	ThreadID, AuthorID ids.ID
	Code               string
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
	// ThreadSeq is the position of the message in its thread (04 section 4); the end of a run
	// orders its open follow-ups by it (M2-11).
	ThreadSeq int64
	// Deleted reports a tombstone (M1-58): its content may never become the trigger of a run.
	Deleted bool
}

// AnchorRef names a live box anchor of a run (09 section 5.4: the only message with run_id, kind
// run, silent).
type AnchorRef struct {
	// MessageID is the anchor; ThreadSeq its position in the thread.
	MessageID ids.ID
	ThreadSeq int64
	// RunID is the run of the anchor; the reads of the consumer push (M3-19) set it,
	// CreateRunAnchor leaves it zero because its caller knows the run.
	RunID ids.ID
}

// PreviewMessage is a message as the preview of a push hint needs it (Messages.ForPreview, M3-22).
type PreviewMessage struct {
	// ID is the message; ThreadID and ChannelID its place; AuthorID its author.
	ID, ThreadID, ChannelID, AuthorID ids.ID
	// Kind is messages.kind (text, voice, card, run, system).
	Kind string
	// ContentMD is the content; ContentFormat its dialect version. Empty for a tombstone.
	ContentMD     string
	ContentFormat int
	// HasAttachments is messages.has_attachments.
	HasAttachments bool
	// Deleted reports a tombstone (deleted_at set).
	Deleted bool
	// MentionIDs are the principals of the message's mentions rows, sorted; nil for none.
	MentionIDs []ids.ID
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
	// ForPreview returns the facts of one message that the preview of a push hint shows (13
	// section 6.4, M3-22 design note 9): thread, channel, author, kind, content, format, the
	// attachment flag, the deletion and the mentioned principals. found is false for an unknown
	// message; a tombstone is returned with Deleted and without content and mentions. It never
	// authorizes: the preview fetch checks message.read and push.deliver itself, and the job
	// notify.gateway_send checks push.deliver. It reads in the ambient transaction of ctx when there
	// is one. Errors: those of the read.
	//
	// Example:
	//
	//	m, found, err := msgs.ForPreview(ctx, messageID)
	ForPreview(ctx context.Context, messageID ids.ID) (PreviewMessage, bool, error)
	// LockCard locks the row of a card message in the ambient unit of work for runs (M2-15 design
	// note 2 b) and returns its facts; it never authorizes, runs decided. Errors: internal outside
	// a unit of work; not_found for an unknown message (a tombstone is returned with Deleted).
	LockCard(ctx context.Context, messageID ids.ID) (CardFacts, error)
	// ComponentsChanged raises messages.version of the card by one and appends message.updated
	// with the representation of the message, its current components included, in the ambient
	// unit of work (M2-15 design note 2 b; 05 section 6 "component state"). The caller holds the
	// lock of LockCard. It returns the new version. Errors: internal outside a unit of work or for
	// a deleted or unknown message.
	ComponentsChanged(ctx context.Context, messageID ids.ID) (int64, error)
	// CardFits checks that the card, with its components as stored now in the ambient unit of
	// work, keeps the size reserve of createMessage for every later message.updated (D-M2-15-26);
	// runs calls it when updateComponent writes a new definition. Errors: 400 validation_failed
	// too_long on /definition; internal for a missing or deleted card.
	CardFits(ctx context.Context, messageID ids.ID) error
	// PrincipalOf returns the public reference of a principal the way TriggerMessageOf returns
	// the author of a message (M2-15 design note 11 c: triggered_by of a run an interaction
	// opened). It never authorizes. Errors: internal for an unknown principal.
	PrincipalOf(ctx context.Context, principalID ids.ID) (PrincipalRef, error)

	// The reads of the consumer push of notify (M3-19 design notes 5 and 9 d). None authorizes and
	// none applies a visibility predicate: the consumer has no principal and reads like the log
	// tail, and notify checks every hint with push.deliver before it decides and again before it
	// sends (07 section 7.4 rule 1). They return positions only, never content; found is false
	// when no message matches, never an error. Errors: the database error.

	// LastAnchorBefore returns the newest run anchor (kind run, run_id set; 09 section 5.4) in
	// threadID with thread_seq below beforeThreadSeq, with its message, run and position. The
	// consumer uses it to tell whether an agent message was written during a run.
	//
	// Example:
	//
	//	anchor, found, err := msgs.LastAnchorBefore(ctx, ev.ThreadID, ev.ThreadSeq)
	LastAnchorBefore(ctx context.Context, threadID ids.ID, beforeThreadSeq int64) (AnchorRef, bool, error)
	// AnchorOf returns the anchor message of runID with its run and position.
	//
	// Example:
	//
	//	anchor, found, err := msgs.AnchorOf(ctx, runID)
	AnchorOf(ctx context.Context, runID ids.ID) (AnchorRef, bool, error)
	// LastAgentMessageBefore returns the newest message of the author agentID in threadID with
	// afterThreadSeq < thread_seq < beforeThreadSeq, origin agent, kind text, voice or card (the
	// kinds the messages check of 03 allows for an answer) and not deleted: the last answer of a
	// run between its anchor and a bound.
	//
	// Example:
	//
	//	last, found, err := msgs.LastAgentMessageBefore(ctx, thread, agent, anchor.ThreadSeq, math.MaxInt64)
	LastAgentMessageBefore(ctx context.Context, threadID, agentID ids.ID, afterThreadSeq, beforeThreadSeq int64) (MessageRef, bool, error)
	// Newest returns the newest message of threadID that is not deleted and whose kind is neither
	// run nor system: the message a bundled hint points at.
	//
	// Example:
	//
	//	newest, found, err := msgs.Newest(ctx, thread)
	Newest(ctx context.Context, threadID ids.ID) (MessageRef, bool, error)
	// AppendSystemLine writes a system line inside the caller's unit of work (M2-12 design note
	// 12, M1-56 D6): the next thread_seq of the thread without a reopen (a system line never
	// reopens a closed thread, R-DM-42), kind system, origin system, l.AuthorID as author, empty
	// content, silent, a server client_msg_id, no mentions and no trigger_state; then
	// message.created and MessagePosted with Kind system, which never triggers a run (SR4). It never
	// authorizes: the caller decided (runs, for the fail open of an agent session). It returns the
	// written line. Errors: internal outside a unit of work and for a code that is not a
	// system_code of 03; not_found for an unknown thread.
	//
	// Example:
	//
	//	m, err := msgs.AppendSystemLine(ctx, messaging.SystemLine{ThreadID: th, AuthorID: agent, Code: "agent_session.resume_failed"})
	AppendSystemLine(ctx context.Context, l SystemLine) (Message, error)
}
