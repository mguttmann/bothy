// SPDX-License-Identifier: Apache-2.0

package chat

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Thread is a thread as the API shows it (schema Thread). Zero values are absent members.
type Thread struct {
	ID, ChannelID ids.ID
	// Kind is main or thread.
	Kind string
	// ParentMessageID is the message the thread started from; zero for none.
	ParentMessageID ids.ID
	// Title may be empty; TitleSource is auto or manual.
	Title, TitleSource string
	// State is open or closed; ClosedAt and ClosedBy are set exactly while closed.
	State    string
	ClosedAt time.Time
	ClosedBy ids.ID
	// OperatorAgentID is the fixed operator of a thread of kind thread (R-DM-12), zero for none
	// and for a main thread; BillingMode its current policy (personal or organization), "" when
	// OperatorAgentID is zero.
	OperatorAgentID ids.ID
	BillingMode     string
	// LastMessageSeq, UnreadFloorSeq and MessageCount are the counters.
	LastMessageSeq, UnreadFloorSeq int64
	MessageCount                   int
	// CreatedBy is the creator; CreatedAt, LastActivityAt and MovedAt the times.
	CreatedBy                          ids.ID
	CreatedAt, LastActivityAt, MovedAt time.Time
	// Version is the ETag.
	Version int64
	// TagIDs are the live tags of a forum post in the order of the tags; never nil.
	TagIDs []ids.ID
}

// ThreadInfo is the routing and state data of a thread (Threads.Info, AppendToThread).
type ThreadInfo struct {
	ID, ChannelID, SpaceID ids.ID
	// ChannelKind is text, forum or dm; Kind main or thread; State open or closed.
	ChannelKind, Kind, State string
	// OperatorAgentID is the fixed operator; zero for none and for a main thread.
	OperatorAgentID ids.ID
	// CreatedBy is the creator.
	CreatedBy ids.ID
	// Version is threads.version.
	Version int64
	// ChannelArchived reports an archived channel or a channel of an archived space.
	ChannelArchived bool
	// LastMessageSeq and UnreadFloorSeq are the counters of the thread (03 threads): the read
	// position of a person lies in 0..LastMessageSeq, and messages up to UnreadFloorSeq count as
	// unread for nobody (M1-61).
	LastMessageSeq, UnreadFloorSeq int64
}

// ThreadCounter are the counters of one thread for the unread counters of the snapshot (M1-63):
// a person's unread count there is LastMessageSeq - GREATEST(last_read_seq, UnreadFloorSeq).
type ThreadCounter struct {
	ThreadID, ChannelID            ids.ID
	LastMessageSeq, UnreadFloorSeq int64
}

// ParentMessage is the message of the main thread a thread starts from. The caller (messaging)
// has verified that it exists and is not deleted.
type ParentMessage struct {
	MessageID, ThreadID ids.ID
}

// CreateThreadCmd is the input of Threads.Create.
type CreateThreadCmd struct {
	ChannelID ids.ID
	// Parent is required in a text channel and forbidden in a forum.
	Parent *ParentMessage
	// Title is the given title; nil is absent.
	Title *string
	// TagIDs are the tags of a forum post.
	TagIDs []ids.ID
}

// AppendCmd is the input of Threads.AppendToThread.
type AppendCmd struct {
	ThreadID ids.ID
	// ReopenIfClosed reopens a closed thread (an implicit thread.reopen by a new message).
	ReopenIfClosed bool
}

// Appended is the result of Threads.AppendToThread.
type Appended struct {
	// Seq is the thread_seq of the new message.
	Seq int64
	// Reopened reports that the thread was closed and is open now.
	Reopened bool
	// Thread is the thread after the change.
	Thread ThreadInfo
}

// ListThreadsQuery is the query of listChannelThreads.
type ListThreadsQuery struct {
	ChannelID ids.ID
	// State is open, closed or "" for both.
	State string
	// TagID keeps the threads with this tag; zero for any.
	TagID ids.ID
	// Sort is last_activity, created or "" for the forum_default_sort of the channel.
	Sort string
	// Cursor is the opaque position of the page; "" for the first.
	Cursor string
	// Limit is the page size; 0 for api.page_size_default, capped at api.page_size_max.
	Limit int
}

// ThreadPage is one page of threads; NextCursor is "" on the last page.
type ThreadPage struct {
	Items      []Thread
	NextCursor string
}

// SetTitleCmd is the body of setThreadTitle.
type SetTitleCmd struct {
	ThreadID ids.ID
	Title    string
	// IfMatch is the required precondition.
	IfMatch Precondition
}

// SetTagsCmd is the body of setThreadTags.
type SetTagsCmd struct {
	ThreadID ids.ID
	// TagIDs is the complete set.
	TagIDs []ids.ID
	// IfMatch is the optional precondition (06 section 8 point 3).
	IfMatch Precondition
}

// Threads is the incoming port of the threads of chat (M1-55): a fixed ID, the gapless thread
// sequence, title with its source, done and reopen, forum posts with tags. The caller a is the
// principal of the request. Errors are kernel/errs errors; a missing and an invisible thread give
// the same 404 not_found. The thread actions are not audited (05 section 4.5, column Audit).
type Threads interface {
	// Create inserts a thread of kind thread in a text or forum channel, captures the effective
	// operator of the channel (R-DM-12), sets tags and appends thread.created. It does not
	// authorize thread.create (messaging did, 02 section 4.6); restricted tags it checks with
	// thread.tags.set, because only chat knows the tags. It must run inside the caller's unit of
	// work (internal error otherwise). Errors: not_found for an unknown channel, validation_failed
	// on /parent_message_id, /title or /tag_ids, conflict for a second thread of the same parent,
	// forbidden for restricted tags without manage.
	//
	// Example:
	//
	//	th, err := threads.Create(ctx, a, chat.CreateThreadCmd{ChannelID: forum, Title: &title})
	Create(ctx context.Context, a actor.Actor, cmd CreateThreadCmd) (Thread, error)
	// AppendToThread hands out the next thread_seq under the row lock (03 section 6.2) and, when
	// the thread is closed and ReopenIfClosed is set, reopens it (authorizing thread.reopen for a)
	// and appends thread.reopened. The caller must already have authorized message.create on the
	// thread in the same unit of work; outside one it fails with internal. A rolled back unit
	// returns the number with it. Errors: not_found for an unknown thread, the refusal of
	// thread.reopen.
	AppendToThread(ctx context.Context, a actor.Actor, cmd AppendCmd) (Appended, error)
	// LockForMessageChange locks the thread row (SELECT ... FOR UPDATE) for a change of one of
	// its messages, before the message row (M1-58, lock order of the messaging README). It does
	// not authorize; the caller has authorized the message action. It must run inside the
	// caller's unit of work (internal otherwise). Errors: not_found for an unknown thread.
	LockForMessageChange(ctx context.Context, threadID ids.ID) error
	// DecrementMessageCount lowers message_count of the thread by one and raises its version, for
	// a deleted message (M1-58, 03 threads: the count excludes deleted messages). The caller holds
	// the lock of LockForMessageChange in its unit of work (internal otherwise). Errors: internal
	// for an unknown thread or a count that would go below zero.
	DecrementMessageCount(ctx context.Context, threadID ids.ID) error
	// Info returns routing and state data of a thread without authorizing; callers authorize
	// before they reveal anything derived from it. Errors: not_found for an unknown thread.
	Info(ctx context.Context, threadID ids.ID) (ThreadInfo, error)
	// InfoForShare returns Info and holds FOR SHARE on the thread row until the end of the unit of
	// work of ctx (M1-61): a concurrent move (M1-62 updates the row) either committed before, so
	// the channel copied into read_states or drafts is the new one, or waits for the caller's
	// commit. It does not authorize. Errors: not_found for an unknown thread, internal outside a
	// unit of work.
	//
	// Example:
	//
	//	info, err := threads.InfoForShare(ctx, threadID) // inside uow.Do
	InfoForShare(ctx context.Context, threadID ids.ID) (ThreadInfo, error)
	// Counters returns the counters of the threads in channelIDs whose last_message_seq is above
	// unread_floor_seq, ascending by thread (M1-63, the unread counters of the snapshot of sync);
	// other threads have nothing unread for anybody. It does not authorize: the caller passes
	// only channels the principal may read (Authorizer.Scope) and checks thread.read per thread
	// it reveals. Errors: the database error.
	//
	// Example:
	//
	//	counters, err := threads.Counters(ctx, scope.ChannelIDs)
	Counters(ctx context.Context, channelIDs []ids.ID) ([]ThreadCounter, error)

	// List returns a page of the threads of kind thread of a channel (thread.read on the channel,
	// then per item in filter mode, 07 section 7.4), newest first by last_activity or created,
	// without totals. Errors: not_found for an invisible channel, cursor_invalid.
	List(ctx context.Context, a actor.Actor, q ListThreadsQuery) (ThreadPage, error)
	// Get returns the thread (thread.read).
	Get(ctx context.Context, a actor.Actor, threadID ids.ID) (Thread, error)
	// View returns the thread like Get but without authorizing, for a module that has just
	// authorized and performed a write on it and answers with the result (messaging after
	// createThread: an app with messages:write and no messages:read gets the thread it created,
	// 05 section 2.6). Errors: not_found for an unknown thread.
	View(ctx context.Context, threadID ids.ID) (Thread, error)
	// SetTitle sets the title (thread.title.update): a person sets the source manual, the operator
	// agent keeps it auto and gets title_manual for a manual title (09 section 12). Errors:
	// precondition_required, version_conflict with current, validation_failed on /title,
	// title_manual.
	SetTitle(ctx context.Context, a actor.Actor, cmd SetTitleCmd) (Thread, error)
	// Close marks the thread done (thread.close); a closed thread answers without change. Errors:
	// invalid_state_transition for a main thread.
	Close(ctx context.Context, a actor.Actor, threadID ids.ID) (Thread, error)
	// Reopen opens a closed thread (thread.reopen); an open thread answers without change.
	Reopen(ctx context.Context, a actor.Actor, threadID ids.ID) (Thread, error)
	// SetTags replaces the tags of a forum post (thread.tags.set; restricted tags need manage).
	// Errors: validation_failed on /tag_ids, version_conflict for a stale If-Match, forbidden.
	SetTags(ctx context.Context, a actor.Actor, cmd SetTagsCmd) (Thread, error)
}
