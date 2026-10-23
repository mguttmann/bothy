// SPDX-License-Identifier: Apache-2.0

package chat

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// ReasonNoMainThread is the field code of a thread batch whose channel has no main thread (a
// forum channel, M8-11 design note 1).
const ReasonNoMainThread = "no_main_thread"

// ImportedThread is a thread an import batch creates (22 5.1.1, M8-11 design note 3 a).
type ImportedThread struct {
	// ChannelID is the text or forum channel of the thread.
	ChannelID ids.ID
	// Name is the thread name of the source; chat cuts it to thread.title_max_chars and decides
	// title_source with the placeholder rule of the auto-title (M2-18).
	Name string
	// Parent is the starter message in the main thread; nil for none. A parent outside the main
	// thread of the channel, or one that started a thread already, is dropped (NULL), never an
	// error (22 5.1.1).
	Parent *ParentMessage
	// OperatorAgentID is the planned operator of the mapping (R-IM-08); zero for none.
	OperatorAgentID ids.ID
	// CreatedAt and LastActivityAt are the times of the source.
	CreatedAt, LastActivityAt time.Time
	// ClosedAt is the archive time of a closed thread; zero for an open one.
	ClosedAt time.Time
	// TagIDs are the live tags of the forum the thread gets (thread_tags); chat ignores a tag that
	// is not a live tag of the channel.
	TagIDs []ids.ID
}

// ImportedThreadState is the stored state of an imported thread that the mirror rules compare
// with the source (22 12.2 step 3).
type ImportedThreadState struct {
	ID, ChannelID ids.ID
	// Title and TitleSource (auto or manual) are the stored title.
	Title, TitleSource string
	// Closed reports state closed; ClosedAt is its time.
	Closed   bool
	ClosedAt time.Time
	// LastActivityAt is the stored time of the last activity.
	LastActivityAt time.Time
}

// MirrorChange is what a mirror round changes on a posted thread (22 12.2 step 3); a nil member
// changes nothing.
type MirrorChange struct {
	// Title is the new title of a thread whose title_source is auto; the source stays auto.
	Title *string
	// CloseAt closes an open thread at this time; Reopen opens a closed one (closed_at and
	// closed_by cleared as the reopen rule of 03 says).
	CloseAt *time.Time
	Reopen  bool
	// LastActivityAt raises last_activity_at to a later time.
	LastActivityAt *time.Time
}

// Empty reports a change without any member.
func (m MirrorChange) Empty() bool {
	return m.Title == nil && m.CloseAt == nil && !m.Reopen && m.LastActivityAt == nil
}

// Reserved is the answer of ImportThreads.Reserve: the first of the numbers handed out and the
// routing data of the locked thread.
type Reserved struct {
	// FirstSeq is the thread_seq of the first message; the n numbers are FirstSeq..FirstSeq+n-1.
	FirstSeq int64
	// ThreadID, ChannelID and SpaceID are where the messages go.
	ThreadID, ChannelID, SpaceID ids.ID
}

// ImportThreads is the incoming port of chat for the threads of an import (M8-11 design note 7):
// importer creates and mirrors threads and finishes a batch through it, messaging takes the
// thread numbers of imported messages. It authorizes nothing (the batch was authorized with
// import.apply) and every method runs only inside the caller's unit of work (internal error
// otherwise), so one batch is one transaction across modules. Only FinishBatch appends an event
// (thread.updated, 04 15.8); the import never appends thread.created, message.created or
// thread.reopened. Errors are kernel/errs errors.
type ImportThreads interface {
	// Create inserts a thread of kind thread with the fields of t (22 5.1.1): the title cut to
	// thread.title_max_chars, title_source auto for a placeholder name and manual otherwise,
	// state closed with closed_at when ClosedAt is set, the instance search language and the
	// importing principal of ctx as creator. Errors: not_found for an unknown, archived or dm
	// channel, validation_failed on /name for a name with control characters.
	Create(ctx context.Context, t ImportedThread) (ids.ID, error)
	// MainThread returns the main thread of a live text channel. Errors: not_found for an unknown,
	// archived or dm channel; validation_failed with the code ReasonNoMainThread for a forum.
	MainThread(ctx context.Context, channelID ids.ID) (ids.ID, error)
	// Lock locks the thread row and returns its stored state. Errors: not_found.
	Lock(ctx context.Context, threadID ids.ID) (ImportedThreadState, error)
	// ApplyMirror writes c to the thread locked by Lock without raising its version (FinishBatch
	// does that once per batch). An empty change writes nothing.
	ApplyMirror(ctx context.Context, threadID ids.ID, c MirrorChange) error
	// Reserve hands out n consecutive thread numbers under the row lock without reopening the
	// thread, raises message_count by n and last_activity_at to lastActivity when that is later.
	// Errors: not_found for an unknown thread.
	Reserve(ctx context.Context, threadID ids.ID, n int, lastActivity time.Time) (Reserved, error)
	// FinishBatch ends a batch of the thread (04 15.8, 22 6.4): when changed it sets
	// unread_floor_seq to last_message_seq, raises the version once and appends one thread.updated
	// with the thread scope; otherwise it writes nothing. It returns unread_floor_seq after the
	// call.
	FinishBatch(ctx context.Context, threadID ids.ID, changed bool) (int64, error)
}
