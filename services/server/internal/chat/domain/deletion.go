// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// The reasons of 409 conflict of deleting and restoring (docs/design/delete.md, 06 section 5).
const (
	// ReasonUndoWindowPassed refuses a restore at or after deleted_at plus
	// chat.delete_undo_window, or of a purged item (D-DEL-1).
	ReasonUndoWindowPassed = "undo_window_passed"
	// ReasonMainThread refuses deleting a main thread alone; it goes with its channel (D-DEL-3).
	ReasonMainThread = "main_thread"
	// ReasonDMChannel refuses deleting a dm channel; a person leaves a DM instead.
	ReasonDMChannel = "dm_channel"
	// ReasonProjectFolderRefused refuses moving a project folder to the trash while the last
	// folder check of the serving runner is not ok or was never reported (D-DEL-10).
	ReasonProjectFolderRefused = "project_folder_refused"
	// ReasonSpaceDeleted refuses restoring a channel while its space is deleted.
	ReasonSpaceDeleted = "space_deleted"
)

// DeletionState is the state of a deletion in DeletionInfo (D-DEL-6).
type DeletionState string

// The states of a deletion.
const (
	// DeletionLive is an item that is not deleted.
	DeletionLive DeletionState = "live"
	// DeletionPending is a deletion that can still be undone.
	DeletionPending DeletionState = "pending"
	// DeletionFinal is a deletion past its undo window, purged or about to be.
	DeletionFinal DeletionState = "final"
)

// Deletion is the deletion state of a space, channel or thread (03 R-DM-57): At and By are set
// together while the item is deleted; PurgedAt once messaging.purge_deleted removed its content.
type Deletion struct {
	At       time.Time
	By       ids.ID
	PurgedAt time.Time
}

// Deleted reports a deleted item, pending or final.
func (d Deletion) Deleted() bool { return !d.At.IsZero() }

// Purged reports an item whose content the purge removed.
func (d Deletion) Purged() bool { return !d.PurgedAt.IsZero() }

// UndoUntil is the end of the undo window, At plus window; zero for a live item.
func (d Deletion) UndoUntil(window time.Duration) time.Time {
	if !d.Deleted() {
		return time.Time{}
	}
	return d.At.Add(window)
}

// State is live, pending strictly before UndoUntil and final from then on or once purged.
func (d Deletion) State(now time.Time, window time.Duration) DeletionState {
	switch {
	case !d.Deleted():
		return DeletionLive
	case d.Purged() || !now.Before(d.UndoUntil(window)):
		return DeletionFinal
	}
	return DeletionPending
}

// CheckRestore allows a restore of a deleted item only while it is pending. Errors: 409 conflict
// with reason undo_window_passed.
func (d Deletion) CheckRestore(now time.Time, window time.Duration) error {
	if d.State(now, window) == DeletionFinal {
		return errs.New(errs.CodeConflict, "the undo window of the deletion has passed").WithReason(ReasonUndoWindowPassed)
	}
	return nil
}

// mark sets the deletion of by at at.
func (d *Deletion) mark(by ids.ID, at time.Time) { d.At, d.By = at, by }

// archiveFor returns the archive time a delete leaves: an earlier archive stays, otherwise at.
func archiveFor(archived, at time.Time) time.Time {
	if archived.IsZero() {
		return at
	}
	return archived
}

// unarchiveFor returns the archive time a restore leaves: the archive the delete set (equal time
// stamps, D-DEL-2) goes, an earlier one stays.
func unarchiveFor(archived, deleted time.Time) time.Time {
	if archived.Equal(deleted) {
		return time.Time{}
	}
	return archived
}

// Deleted reports a deleted space, pending or final.
func (s Space) Deleted() bool { return s.Deletion.Deleted() }

// Delete marks the space deleted by by at at, archives it with the same time stamp unless it is
// archived already, and bumps the version (D-DEL-2).
func (s *Space) Delete(by ids.ID, at time.Time) {
	s.ArchivedAt = archiveFor(s.ArchivedAt, at)
	s.Deletion.mark(by, at)
	s.Version++
}

// Restore clears the deletion and the archive the delete set, and bumps the version.
func (s *Space) Restore() {
	s.ArchivedAt = unarchiveFor(s.ArchivedAt, s.Deletion.At)
	s.Deletion = Deletion{}
	s.Version++
}

// Deleted reports a deleted channel, pending or final; its space may be deleted on its own.
func (c Channel) Deleted() bool { return c.Deletion.Deleted() }

// Delete marks the channel deleted by by at at, archives it with the same time stamp unless it is
// archived already, and bumps the version (D-DEL-2). Errors: 409 conflict dm_channel for a dm
// channel, which is left as it is.
func (c *Channel) Delete(by ids.ID, at time.Time) error {
	if c.Kind == ChannelDM {
		return errs.New(errs.CodeConflict, "a direct message is left, not deleted").WithReason(ReasonDMChannel)
	}
	c.ArchivedAt = archiveFor(c.ArchivedAt, at)
	c.Deletion.mark(by, at)
	c.Version++
	return nil
}

// Restore clears the deletion and the archive the delete set, and bumps the version.
func (c *Channel) Restore() {
	c.ArchivedAt = unarchiveFor(c.ArchivedAt, c.Deletion.At)
	c.Deletion = Deletion{}
	c.Version++
}

// Deleted reports a deleted thread, pending or final (D-DEL-3).
func (t Thread) Deleted() bool { return t.Deletion.Deleted() }

// Delete marks the thread deleted by by at at and bumps the version. Errors: 409 conflict
// main_thread for a main thread, which goes only with its channel.
func (t *Thread) Delete(by ids.ID, at time.Time) error {
	if t.Main() {
		return errs.New(errs.CodeConflict, "a main thread goes only with its channel").WithReason(ReasonMainThread)
	}
	t.Deletion.mark(by, at)
	t.Version++
	return nil
}

// Restore clears the deletion and bumps the version.
func (t *Thread) Restore() {
	t.Deletion = Deletion{}
	t.Version++
}
