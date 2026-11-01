// SPDX-License-Identifier: Apache-2.0

package chat

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// The reasons of 409 conflict of deleting and restoring (docs/design/delete.md, 06 section 5);
// the same values as in chat/domain, held equal by a test.
const (
	ReasonUndoWindowPassed     = "undo_window_passed"
	ReasonMainThread           = "main_thread"
	ReasonDMChannel            = "dm_channel"
	ReasonProjectFolderRefused = "project_folder_refused"
	ReasonSpaceDeleted         = "space_deleted"
)

// The target kinds of a deletion (schema DeletionInfo.target_kind) are TargetSpace,
// TargetChannel and TargetThread of notification.go.

// The folder move states of a binding (03 project_bindings.trash_state, 09 18.15).
const (
	TrashRequested = "requested"
	TrashTrashed   = "trashed"
	TrashRefused   = "refused"
)

// DeletionInfo is what deleting a space, channel or thread removes, or the state of its deletion
// (schema DeletionInfo, D-DEL-6).
type DeletionInfo struct {
	// TargetKind is space, channel or thread; TargetID the item.
	TargetKind string
	TargetID   ids.ID
	// State is live, pending or final.
	State string
	// DeletedAt and UndoUntil are zero while live.
	DeletedAt, UndoUntil time.Time
	// Contents are counted when read.
	Contents DeletionContents
	// ProjectFolder is set for a forum with a project binding and an owner as caller; nil
	// otherwise.
	ProjectFolder *ProjectFolderDeletion
}

// DeletionContents are the counts of schema DeletionContents.
type DeletionContents struct {
	Channels, Threads, Messages, Files, Sessions int64
}

// ProjectFolderDeletion is the project folder of a forum (schema ProjectFolderDeletion).
type ProjectFolderDeletion struct {
	WorkspaceRef string
	// TrashCheck is the last folder check of the serving runner; nil before the first one.
	TrashCheck *ProjectTrashCheck
	// TrashState is requested, trashed or refused; "" when no move was requested.
	TrashState string
	// TrashReason is the refusal code with TrashState refused.
	TrashReason string
}

// ProjectTrashCheck is a stored folder check (schema ProjectTrashCheck).
type ProjectTrashCheck struct {
	OK bool
	// Reason is the code with OK false; "" otherwise.
	Reason string
	// CheckedAt is when the server received the check.
	CheckedAt time.Time
}

// Deletions deletes and restores spaces, channels and threads (05 space.delete, space.restore,
// channel.delete, channel.restore, thread.delete, thread.restore; 01 6.18, D-DEL-1 to D-DEL-11)
// and answers DeletionInfo. Each call authorizes its action first; a missing and an invisible
// item give the same 404 not_found. Errors are kernel/errs errors.
type Deletions interface {
	// DeleteSpace deletes a space (owner, step-up): archives it unless it is archived, marks it
	// deleted in the same rights change, publishes ContentDeleted and space.archived with
	// deleted_at. A deleted space answers its deletion without effect.
	DeleteSpace(ctx context.Context, a actor.Actor, id ids.ID) (DeletionInfo, error)
	// RestoreSpace undoes a pending deletion. Errors: conflict undo_window_passed.
	RestoreSpace(ctx context.Context, a actor.Actor, id ids.ID) (Space, error)
	// SpaceDeletion answers DeletionInfo of a space (space.read).
	SpaceDeletion(ctx context.Context, a actor.Actor, id ids.ID) (DeletionInfo, error)
	// DeleteChannel deletes a channel (manage, step-up); trashFolder also requests the move of
	// the project folder (channel.project.bind and an ok folder check). Errors: conflict
	// dm_channel, project_folder_refused; validation_failed not_project_bound on
	// /trash_project_folder for a channel without a binding.
	DeleteChannel(ctx context.Context, a actor.Actor, id ids.ID, trashFolder bool) (DeletionInfo, error)
	// RestoreChannel undoes a pending deletion and drops a folder request. Errors: conflict
	// undo_window_passed, space_deleted.
	RestoreChannel(ctx context.Context, a actor.Actor, id ids.ID) (Channel, error)
	// ChannelDeletion answers DeletionInfo of a channel (channel.read).
	ChannelDeletion(ctx context.Context, a actor.Actor, id ids.ID) (DeletionInfo, error)
	// DeleteThread deletes a chat (write and creator or manage), publishes ContentDeleted and
	// thread.updated with deleted_at. Errors: conflict main_thread.
	DeleteThread(ctx context.Context, a actor.Actor, id ids.ID) (DeletionInfo, error)
	// RestoreThread undoes a pending deletion. Errors: conflict undo_window_passed.
	RestoreThread(ctx context.Context, a actor.Actor, id ids.ID) (Thread, error)
	// ThreadDeletion answers DeletionInfo of a thread (thread.read, also for a deleted thread).
	ThreadDeletion(ctx context.Context, a actor.Actor, id ids.ID) (DeletionInfo, error)
}

// ContentDeleted is the in-transaction event of space.delete, channel.delete and thread.delete
// (D-DEL-7): runs cancels the active runs in the threads of ChannelIDs and in ThreadIDs in the
// same transaction. ChannelIDs are the channel, or every channel of the space; ThreadIDs the
// thread.
type ContentDeleted struct {
	// Kind is space, channel or thread; ID the deleted item.
	Kind string
	ID   ids.ID
	// ChannelIDs and ThreadIDs are the content whose runs end.
	ChannelIDs, ThreadIDs []ids.ID
}

// FileCounter is a source port of chat (02 R-AR-04): the number of files attached to the live
// messages of threads, for DeletionContents.files. files implements it; internal/wire binds it.
type FileCounter interface {
	FilesOfThreads(ctx context.Context, threadIDs []ids.ID) (int64, error)
}

// SessionCounter is a source port of chat: the number of agent sessions of threads, for
// DeletionContents.sessions. agents implements it; internal/wire binds it.
type SessionCounter interface {
	SessionsOfThreads(ctx context.Context, threadIDs []ids.ID) (int64, error)
}

// PurgeTarget is a deleted item whose undo window passed and whose content is not purged yet
// (job messaging.purge_deleted, D-DEL-1): the messages of ChannelIDs and of ThreadIDs go.
type PurgeTarget struct {
	// Kind is space, channel or thread; ID the item.
	Kind string
	ID   ids.ID
	// ChannelIDs are the item for a channel and every channel of the item for a space.
	ChannelIDs []ids.ID
	// ThreadIDs are the item for a thread.
	ThreadIDs []ids.ID
}

// Purges is the port of the job messaging.purge_deleted onto chat. It never authorizes.
type Purges interface {
	// DuePurges returns at most limit items deleted before now minus chat.delete_undo_window and
	// not purged yet, oldest deletion first.
	DuePurges(ctx context.Context, limit int) ([]PurgeTarget, error)
	// FinishPurge clears the thread titles and channel topics of t and sets purged_at, in its own
	// unit of work; an item purged already is left as it is. The caller removed every message
	// of t before.
	FinishPurge(ctx context.Context, t PurgeTarget) error
}

// ProjectTrashCheckReport is the folder check a runner reports per bound forum (schema
// ProjectTrashCheckReport, 09 18.15).
type ProjectTrashCheckReport struct {
	OK bool
	// Reason is the code with OK false.
	Reason string
}

// ProjectTrashRequest is a folder the serving runner moves to the trash (schema
// ProjectTrashRequest).
type ProjectTrashRequest struct {
	ChannelID    ids.ID
	WorkspaceRef string
}

// ProjectTrashResult is the outcome of a folder move a runner reports (schema ProjectTrashResult).
type ProjectTrashResult struct {
	ChannelID ids.ID
	// Result is trashed or refused; Reason the refusal code.
	Result, Reason string
}

// ProjectTrash is the port of the agents heartbeat onto the folder moves of deleted forums
// (D-DEL-11). It never authorizes the heartbeat itself; the caller did.
type ProjectTrash interface {
	// ProjectTrashRequests returns the folder moves the runner agentID still owes: bindings with
	// trash_state requested and trash_agent_id agentID whose channel is purged.
	ProjectTrashRequests(ctx context.Context, agentID ids.ID) ([]ProjectTrashRequest, error)
	// ApplyProjectTrashResults stores each result of agentID once, only for a request of that
	// agent in state requested, and audits it as channel.project.trash with the agent as actor
	// (from ctx). Unknown or foreign channels are ignored.
	ApplyProjectTrashResults(ctx context.Context, agentID ids.ID, results []ProjectTrashResult) error
}
