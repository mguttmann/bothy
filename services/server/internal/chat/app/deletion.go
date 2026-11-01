// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"slices"
	"time"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/chat/domain"
	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// DeletionStore is the outgoing port of deleting and restoring (FEAT-delete-spaces-chats) over
// spaces, channels, threads and project_bindings (implemented by adapters/postgres). Writes run
// only inside a unit of work. Errors are kernel/errs errors without SQL text or values.
type DeletionStore interface {
	// SaveSpaceDeletion writes archived_at, deleted_at, deleted_by and version of sp.
	SaveSpaceDeletion(ctx context.Context, sp domain.Space) error
	// SaveChannelDeletion writes archived_at, deleted_at, deleted_by and version of c.
	SaveChannelDeletion(ctx context.Context, c domain.Channel) error
	// SaveThreadDeletion writes deleted_at, deleted_by and version of th.
	SaveThreadDeletion(ctx context.Context, th domain.Thread) error
	// DeletionScope counts what deleting the item removes: for a space its channels that are not
	// deleted on their own and not dm channels, and their threads that are not deleted; for a
	// channel itself and its threads that are not deleted; for a thread itself.
	DeletionScope(ctx context.Context, kind string, id ids.ID) (Scope, error)
	// BindingTrash reads the folder part of the binding of channel; found is false without one.
	BindingTrash(ctx context.Context, channel ids.ID) (b BindingTrash, found bool, err error)
	// LockBindingTrash is BindingTrash with the binding row locked.
	LockBindingTrash(ctx context.Context, channel ids.ID) (b BindingTrash, found bool, err error)
	// RequestTrash sets trash_state requested, trash_agent_id agent and trash_requested_at at and
	// clears trash_reason.
	RequestTrash(ctx context.Context, channel, agent ids.ID, at time.Time) error
	// DropTrashRequest clears a folder request of channel that is still requested.
	DropTrashRequest(ctx context.Context, channel ids.ID) error
	// SetTrashCheck writes the folder check of channel received at at.
	SetTrashCheck(ctx context.Context, channel ids.ID, c chat.ProjectTrashCheckReport, at time.Time) error
	// DuePurges returns at most limit deleted items with deleted_at before before and purged_at
	// NULL, oldest deletion first.
	DuePurges(ctx context.Context, before time.Time, limit int) ([]PurgeItem, error)
	// FinishPurge clears the thread titles and channel topics of the item and sets purged_at at;
	// an item purged already changes nothing.
	FinishPurge(ctx context.Context, kind string, id ids.ID, at time.Time) error
	// PendingTrash returns the requests of agent whose channel is purged.
	PendingTrash(ctx context.Context, agent ids.ID) ([]chat.ProjectTrashRequest, error)
	// LockTrashRequests locks the bindings of channels that agent was asked to move and that are
	// still requested, in ascending channel id, and returns those channels.
	LockTrashRequests(ctx context.Context, agent ids.ID, channels []ids.ID) ([]ids.ID, error)
	// SetTrashResult writes the outcome of a requested move: trash_state result and the reason
	// ("" is NULL, only with refused).
	SetTrashResult(ctx context.Context, channel ids.ID, result, reason string) error
}

// Scope are the counts of DeletionStore.DeletionScope and the threads they cover, main threads
// included, for the file and session counts.
type Scope struct {
	Channels, Threads, Messages int64
	ThreadIDs                   []ids.ID
}

// BindingTrash is the folder part of a project binding (03 project_bindings, D-DEL-10, D-DEL-11).
type BindingTrash struct {
	WorkspaceRef string
	// Check is the last folder check; nil before the first one.
	Check *chat.ProjectTrashCheck
	// State is requested, trashed, refused or ""; Reason the refusal code; AgentID the runner
	// asked to move the folder.
	State, Reason string
	AgentID       ids.ID
}

// PurgeItem is one deleted item that the purge is due for.
type PurgeItem struct {
	Kind string
	ID   ids.ID
}

// The audit details of deleting and restoring (D-DEL-9) and of channel.project.trash.
const (
	attrTrashFolder = "trash_project_folder"
	attrResult      = "result"
	attrReasonCode  = "reason_code"
)

// pointerTrashFolder is the member of ChannelDelete that asks for the folder move.
const pointerTrashFolder = "/trash_project_folder"

// Compile-time checks: the incoming ports.
var (
	_ chat.Deletions    = (*Service)(nil)
	_ chat.Purges       = (*Service)(nil)
	_ chat.ProjectTrash = (*Service)(nil)
)

// deletions returns the deletion store. Errors: internal without one.
func (s *Service) deletions() (DeletionStore, error) {
	if s.d.Deletions == nil {
		return nil, errs.New(errs.CodeInternalError, "chat: no store for deletions")
	}
	return s.d.Deletions, nil
}

// defaultUndoWindow is the default of chat.delete_undo_window (wire/configkeys_chat.go), used by a
// Config built without the key.
const defaultUndoWindow = 30 * time.Second

// window is chat.delete_undo_window.
func (s *Service) window() time.Duration {
	if s.cfg.DeleteUndoWindow == 0 {
		return defaultUndoWindow
	}
	return s.cfg.DeleteUndoWindow
}

// DeleteSpace implements chat.Deletions (07 4.5a, D-DEL-2): a first unit loads and authorizes
// space.delete (owner, step-up); a deleted space answers its deletion without effect. Otherwise a
// second unit, declared as the rights change space.delete, locks the space, archives it unless it
// is archived and marks it deleted with the same time stamp, publishes ContentDeleted for every
// channel of the space (runs end), records the audit entry and appends space.archived with
// deleted_at; the rights hook raises permission_version and writes access.changed.
func (s *Service) DeleteSpace(ctx context.Context, a actor.Actor, id ids.ID) (chat.DeletionInfo, error) {
	ctx, err := withCaller(ctx, a)
	if err != nil {
		return chat.DeletionInfo{}, err
	}
	st, err := s.deletions()
	if err != nil {
		return chat.DeletionInfo{}, err
	}
	var change bool
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		sp, err := s.load(ctx, a, authorize.ActionSpaceDelete, id)
		if err != nil {
			return err
		}
		if change = !sp.Deleted(); !change {
			return s.record(ctx, authorize.ActionSpaceDelete, id, auditing.Attrs{attrNoChange: auditing.Bool(true)})
		}
		return nil
	})
	if err != nil {
		return chat.DeletionInfo{}, err
	}
	if change {
		ref := txn.Ref{Kind: resourceSpace, ID: id}
		err = s.d.UoW.Do(ctx, func(ctx context.Context) error { return s.deleteSpaceWrite(ctx, st, a, id) },
			txn.DeclareRightsChange(txn.RightsChangeSpec{Action: string(authorize.ActionSpaceDelete), Resource: ref, Targets: []txn.Ref{ref}}))
		if err != nil {
			return chat.DeletionInfo{}, err
		}
	}
	return s.spaceDeletion(ctx, id)
}

// deleteSpaceWrite is the second unit of DeleteSpace.
func (s *Service) deleteSpaceWrite(ctx context.Context, st DeletionStore, a actor.Actor, id ids.ID) error {
	sp, err := s.lock(ctx, id)
	if err != nil {
		return err
	}
	if sp.Deleted() {
		// A concurrent call won the race: answered as without effect (06 section 2).
		return s.record(ctx, authorize.ActionSpaceDelete, id, auditing.Attrs{attrNoChange: auditing.Bool(true)})
	}
	sp.Delete(a.ID, s.now())
	if err := st.SaveSpaceDeletion(ctx, sp); err != nil {
		return err
	}
	channels, err := s.d.Store.ChannelIDsOfSpace(ctx, id)
	if err != nil {
		return err
	}
	if err := txn.Publish(ctx, chat.ContentDeleted{Kind: chat.TargetSpace, ID: id, ChannelIDs: channels}); err != nil {
		return err
	}
	if err := s.record(ctx, authorize.ActionSpaceDelete, id, nil); err != nil {
		return err
	}
	return s.emitSpace(ctx, authorize.EventSpaceArchived, sp)
}

// RestoreSpace implements chat.Deletions (D-DEL-1, D-DEL-2): space.restore (owner, step-up); a
// live space answers without effect; after the undo window 409 undo_window_passed. Otherwise, as
// the rights change space.restore, it clears the deletion and the archive the delete set, records
// the audit entry and appends space.updated without deleted_at.
func (s *Service) RestoreSpace(ctx context.Context, a actor.Actor, id ids.ID) (chat.Space, error) {
	ctx, err := withCaller(ctx, a)
	if err != nil {
		return chat.Space{}, err
	}
	st, err := s.deletions()
	if err != nil {
		return chat.Space{}, err
	}
	var (
		out    domain.Space
		change bool
	)
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		if out, err = s.load(ctx, a, authorize.ActionSpaceRestore, id); err != nil {
			return err
		}
		if change = out.Deleted(); !change {
			return s.record(ctx, authorize.ActionSpaceRestore, id, auditing.Attrs{attrNoChange: auditing.Bool(true)})
		}
		return out.Deletion.CheckRestore(s.now(), s.window())
	})
	if err != nil || !change {
		return spaceView(out), err
	}
	ref := txn.Ref{Kind: resourceSpace, ID: id}
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		out, err = s.restoreSpaceWrite(ctx, st, id)
		return err
	}, txn.DeclareRightsChange(txn.RightsChangeSpec{Action: string(authorize.ActionSpaceRestore), Resource: ref, Targets: []txn.Ref{ref}}))
	return spaceView(out), err
}

// restoreSpaceWrite is the second unit of RestoreSpace.
func (s *Service) restoreSpaceWrite(ctx context.Context, st DeletionStore, id ids.ID) (domain.Space, error) {
	sp, err := s.lock(ctx, id)
	if err != nil {
		return domain.Space{}, err
	}
	if !sp.Deleted() {
		return sp, s.record(ctx, authorize.ActionSpaceRestore, id, auditing.Attrs{attrNoChange: auditing.Bool(true)})
	}
	if err := sp.Deletion.CheckRestore(s.now(), s.window()); err != nil {
		return domain.Space{}, err
	}
	sp.Restore()
	if err := st.SaveSpaceDeletion(ctx, sp); err != nil {
		return domain.Space{}, err
	}
	if err := s.record(ctx, authorize.ActionSpaceRestore, id, nil); err != nil {
		return domain.Space{}, err
	}
	return sp, s.emitSpace(ctx, authorize.EventSpaceUpdated, sp)
}

// SpaceDeletion implements chat.Deletions: space.read, also on a deleted space.
func (s *Service) SpaceDeletion(ctx context.Context, a actor.Actor, id ids.ID) (chat.DeletionInfo, error) {
	if _, err := s.load(ctx, a, authorize.ActionSpaceRead, id); err != nil {
		return chat.DeletionInfo{}, err
	}
	return s.spaceDeletion(ctx, id)
}

// spaceDeletion counts the deletion of the space id without authorization.
func (s *Service) spaceDeletion(ctx context.Context, id ids.ID) (chat.DeletionInfo, error) {
	sp, found, err := s.d.Store.Space(ctx, id)
	if err != nil {
		return chat.DeletionInfo{}, err
	}
	if !found {
		return chat.DeletionInfo{}, invisible()
	}
	return s.deletionInfo(ctx, chat.TargetSpace, id, sp.Deletion)
}

// DeleteChannel implements chat.Deletions (07 4.5a, D-DEL-2, D-DEL-10, D-DEL-11): a first unit
// loads and authorizes channel.delete (manage, step-up) and, with trashFolder, channel.project.bind
// (owner, step-up) and the binding: a channel without one is validation_failed on
// /trash_project_folder, a last folder check that is not ok or missing 409
// project_folder_refused. A dm channel is 409 dm_channel; a deleted channel answers its deletion
// without effect. Otherwise a second unit, declared as the rights change channel.delete, locks the
// channel, archives it unless it is archived and marks it deleted with the same time stamp, with
// trashFolder requests the folder move from the effective operator of the forum, publishes
// ContentDeleted (runs end), records the audit entry with trash_project_folder and appends
// channel.archived with deleted_at.
func (s *Service) DeleteChannel(ctx context.Context, a actor.Actor, id ids.ID, trashFolder bool) (chat.DeletionInfo, error) {
	ctx, err := withCaller(ctx, a)
	if err != nil {
		return chat.DeletionInfo{}, err
	}
	st, err := s.deletions()
	if err != nil {
		return chat.DeletionInfo{}, err
	}
	attrs := func() auditing.Attrs { return auditing.Attrs{attrTrashFolder: auditing.Bool(trashFolder)} }
	var change bool
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		change, err = s.deleteChannelCheck(ctx, st, a, id, trashFolder, attrs())
		return err
	})
	if err != nil {
		return chat.DeletionInfo{}, err
	}
	if change {
		ref := txn.Ref{Kind: resourceChannel, ID: id}
		err = s.d.UoW.Do(ctx, func(ctx context.Context) error { return s.deleteChannelWrite(ctx, st, a, id, trashFolder, attrs()) },
			txn.DeclareRightsChange(txn.RightsChangeSpec{Action: string(authorize.ActionChannelDelete), Resource: ref, Targets: []txn.Ref{ref}}))
		if err != nil {
			return chat.DeletionInfo{}, err
		}
	}
	return s.channelDeletion(ctx, a, id)
}

// deleteChannelCheck is the first unit of DeleteChannel; change reports a channel to delete.
func (s *Service) deleteChannelCheck(ctx context.Context, st DeletionStore, a actor.Actor, id ids.ID, trashFolder bool,
	attrs auditing.Attrs) (bool, error) {
	c, err := s.loadChannel(ctx, a, authorize.ActionChannelDelete, id)
	if err != nil {
		return false, err
	}
	if c.Channel.Deleted() {
		return false, s.noChangeChannel(ctx, authorize.ActionChannelDelete, id, attrs)
	}
	if c.Channel.Kind == domain.ChannelDM {
		return false, c.Channel.Delete(a.ID, s.now())
	}
	if trashFolder {
		if err := s.checkTrash(ctx, st, a, c, false); err != nil {
			return false, err
		}
	}
	return true, nil
}

// checkTrash checks a folder move of the channel c: a binding (validation_failed otherwise),
// channel.project.bind for a (once, before the lock) and a last folder check that is ok (409
// project_folder_refused otherwise); locked reads the binding under its row lock.
func (s *Service) checkTrash(ctx context.Context, st DeletionStore, a actor.Actor, c ChannelRow, locked bool) error {
	if !c.ProjectBound {
		return errs.New(errs.CodeValidationFailed, "the channel has no project binding").WithFields(errs.FieldError{
			Pointer: pointerTrashFolder, Code: domain.FieldNotAllowed, Detail: "the channel has no project binding",
		})
	}
	read := st.BindingTrash
	if locked {
		read = st.LockBindingTrash
	} else if err := s.check(ctx, a, authorize.ActionChannelProjectBind, channelResource(c.Channel.ID, c.Channel.SpaceID)); err != nil {
		return err
	}
	b, found, err := read(ctx, c.Channel.ID)
	if err != nil {
		return err
	}
	if !found {
		return errs.New(errs.CodeValidationFailed, "the channel has no project binding").WithFields(errs.FieldError{
			Pointer: pointerTrashFolder, Code: domain.FieldNotAllowed, Detail: "the channel has no project binding",
		})
	}
	if b.Check == nil || !b.Check.OK || c.Channel.Operator(c.SpaceDefaultOperatorID).EffectiveAgentID.IsZero() {
		return errs.New(errs.CodeConflict, "the serving runner refuses or never checked the project folder").
			WithReason(chat.ReasonProjectFolderRefused)
	}
	return nil
}

// deleteChannelWrite is the second unit of DeleteChannel.
func (s *Service) deleteChannelWrite(ctx context.Context, st DeletionStore, a actor.Actor, id ids.ID, trashFolder bool, attrs auditing.Attrs) error {
	c, err := s.lockChannel(ctx, id)
	if err != nil {
		return err
	}
	if c.Channel.Deleted() {
		return s.noChangeChannel(ctx, authorize.ActionChannelDelete, id, attrs)
	}
	at := s.now()
	if err := c.Channel.Delete(a.ID, at); err != nil {
		return err
	}
	if trashFolder {
		if err := s.checkTrash(ctx, st, a, c, true); err != nil {
			return err
		}
		if err := st.RequestTrash(ctx, id, c.Channel.Operator(c.SpaceDefaultOperatorID).EffectiveAgentID, at); err != nil {
			return err
		}
	}
	if err := st.SaveChannelDeletion(ctx, c.Channel); err != nil {
		return err
	}
	if err := txn.Publish(ctx, chat.ContentDeleted{Kind: chat.TargetChannel, ID: id, ChannelIDs: []ids.ID{id}}); err != nil {
		return err
	}
	if err := s.recordChannel(ctx, authorize.ActionChannelDelete, id, attrs); err != nil {
		return err
	}
	return s.emitChannel(ctx, authorize.EventChannelArchived, c)
}

// RestoreChannel implements chat.Deletions (D-DEL-1, D-DEL-2, D-DEL-11): channel.restore (manage,
// step-up); a live channel answers without effect; after the undo window 409
// undo_window_passed; in a deleted space 409 space_deleted. Otherwise, as the rights change
// channel.restore, it clears the deletion and the archive the delete set, drops a folder request
// that is still requested, records the audit entry and appends channel.updated without
// deleted_at.
func (s *Service) RestoreChannel(ctx context.Context, a actor.Actor, id ids.ID) (chat.Channel, error) {
	ctx, err := withCaller(ctx, a)
	if err != nil {
		return chat.Channel{}, err
	}
	st, err := s.deletions()
	if err != nil {
		return chat.Channel{}, err
	}
	var (
		out    ChannelRow
		change bool
	)
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		if out, err = s.loadChannel(ctx, a, authorize.ActionChannelRestore, id); err != nil {
			return err
		}
		if change = out.Channel.Deleted(); !change {
			return s.noChangeChannel(ctx, authorize.ActionChannelRestore, id, nil)
		}
		return s.checkChannelRestore(ctx, out)
	})
	if err != nil || !change {
		return channelDTO(out), err
	}
	ref := txn.Ref{Kind: resourceChannel, ID: id}
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		out, err = s.restoreChannelWrite(ctx, st, id)
		return err
	}, txn.DeclareRightsChange(txn.RightsChangeSpec{Action: string(authorize.ActionChannelRestore), Resource: ref, Targets: []txn.Ref{ref}}))
	return channelDTO(out), err
}

// checkChannelRestore refuses the restore of c after its undo window and while its space is
// deleted.
func (s *Service) checkChannelRestore(ctx context.Context, c ChannelRow) error {
	if err := c.Channel.Deletion.CheckRestore(s.now(), s.window()); err != nil {
		return err
	}
	if c.Channel.SpaceID.IsZero() {
		return nil
	}
	sp, found, err := s.d.Store.Space(ctx, c.Channel.SpaceID)
	if err != nil {
		return err
	}
	if found && sp.Deleted() {
		return errs.New(errs.CodeConflict, "the space of the channel is deleted").WithReason(chat.ReasonSpaceDeleted)
	}
	return nil
}

// restoreChannelWrite is the second unit of RestoreChannel.
func (s *Service) restoreChannelWrite(ctx context.Context, st DeletionStore, id ids.ID) (ChannelRow, error) {
	c, err := s.lockChannel(ctx, id)
	if err != nil {
		return ChannelRow{}, err
	}
	if !c.Channel.Deleted() {
		return c, s.noChangeChannel(ctx, authorize.ActionChannelRestore, id, nil)
	}
	if err := s.checkChannelRestore(ctx, c); err != nil {
		return ChannelRow{}, err
	}
	b, bound, err := st.LockBindingTrash(ctx, id)
	if err != nil {
		return ChannelRow{}, err
	}
	trash := bound && b.State == chat.TrashRequested
	if trash {
		if err := st.DropTrashRequest(ctx, id); err != nil {
			return ChannelRow{}, err
		}
	}
	c.Channel.Restore()
	if err := st.SaveChannelDeletion(ctx, c.Channel); err != nil {
		return ChannelRow{}, err
	}
	if err := s.recordChannel(ctx, authorize.ActionChannelRestore, id, auditing.Attrs{attrTrashFolder: auditing.Bool(trash)}); err != nil {
		return ChannelRow{}, err
	}
	return c, s.emitChannel(ctx, authorize.EventChannelUpdated, c)
}

// ChannelDeletion implements chat.Deletions: channel.read, also on a deleted channel.
func (s *Service) ChannelDeletion(ctx context.Context, a actor.Actor, id ids.ID) (chat.DeletionInfo, error) {
	if _, err := s.loadChannel(ctx, a, authorize.ActionChannelRead, id); err != nil {
		return chat.DeletionInfo{}, err
	}
	return s.channelDeletion(ctx, a, id)
}

// channelDeletion counts the deletion of the channel id for a without authorization: the folder
// part only for a forum with a binding and an owner as a (D-DEL-6).
func (s *Service) channelDeletion(ctx context.Context, a actor.Actor, id ids.ID) (chat.DeletionInfo, error) {
	c, found, err := s.d.Store.Channel(ctx, id)
	if err != nil {
		return chat.DeletionInfo{}, err
	}
	if !found {
		return chat.DeletionInfo{}, invisible()
	}
	out, err := s.deletionInfo(ctx, chat.TargetChannel, id, c.Channel.Deletion)
	if err != nil || !c.ProjectBound {
		return out, err
	}
	owner, err := s.isOwner(ctx, a)
	if err != nil || !owner {
		return out, err
	}
	st, err := s.deletions()
	if err != nil {
		return chat.DeletionInfo{}, err
	}
	b, bound, err := st.BindingTrash(ctx, id)
	if err != nil || !bound {
		return out, err
	}
	out.ProjectFolder = &chat.ProjectFolderDeletion{WorkspaceRef: b.WorkspaceRef, TrashCheck: b.Check, TrashState: b.State, TrashReason: b.Reason}
	return out, nil
}

// isOwner reports whether a is an active owner, from the owner list of identity, the source of
// getMe is_owner. channel.project.bind would demand a fresh step-up for a read (D-DEL-SRV-3).
func (s *Service) isOwner(ctx context.Context, a actor.Actor) (bool, error) {
	if a.Kind != actor.KindUser || a.ID.IsZero() {
		return false, nil
	}
	owners, err := s.d.Directory.ActiveOwners(ctx)
	if err != nil {
		return false, err
	}
	return slices.ContainsFunc(owners, func(o identity.OwnerView) bool { return o.User.ID == a.ID }), nil
}

// DeleteThread implements chat.Deletions (07 4.5a, D-DEL-3): one unit with thread.delete (write
// and creator or manage, no step-up), the row lock, then a main thread is 409 main_thread and a
// deleted thread answers its deletion without effect; otherwise the deletion, ContentDeleted
// (runs of the thread end), the audit entry and thread.updated with deleted_at.
func (s *Service) DeleteThread(ctx context.Context, a actor.Actor, id ids.ID) (chat.DeletionInfo, error) {
	ctx, err := withCaller(ctx, a)
	if err != nil {
		return chat.DeletionInfo{}, err
	}
	st, err := s.deletions()
	if err != nil {
		return chat.DeletionInfo{}, err
	}
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error { return s.deleteThreadUnit(ctx, st, a, id) })
	if err != nil {
		return chat.DeletionInfo{}, err
	}
	return s.threadDeletion(ctx, id)
}

// deleteThreadUnit is the unit of work of DeleteThread.
func (s *Service) deleteThreadUnit(ctx context.Context, st DeletionStore, a actor.Actor, id ids.ID) error {
	if _, err := s.loadThread(ctx, a, authorize.ActionThreadDelete, id); err != nil {
		return err
	}
	th, err := s.lockThread(ctx, id)
	if err != nil {
		return err
	}
	if th.Deleted() {
		return s.recordThread(ctx, authorize.ActionThreadDelete, id, auditing.Attrs{attrNoChange: auditing.Bool(true)})
	}
	if err := th.Delete(a.ID, s.now()); err != nil {
		return err
	}
	if err := st.SaveThreadDeletion(ctx, th); err != nil {
		return err
	}
	if err := txn.Publish(ctx, chat.ContentDeleted{Kind: chat.TargetThread, ID: id, ThreadIDs: []ids.ID{id}}); err != nil {
		return err
	}
	if err := s.recordThread(ctx, authorize.ActionThreadDelete, id, nil); err != nil {
		return err
	}
	return s.emitThreadChange(ctx, th)
}

// RestoreThread implements chat.Deletions (D-DEL-1, D-DEL-3): one unit with thread.restore, the row
// lock, then a live thread answers without effect and after the undo window 409
// undo_window_passed; otherwise the restore, the audit entry and thread.updated without
// deleted_at.
func (s *Service) RestoreThread(ctx context.Context, a actor.Actor, id ids.ID) (chat.Thread, error) {
	ctx, err := withCaller(ctx, a)
	if err != nil {
		return chat.Thread{}, err
	}
	st, err := s.deletions()
	if err != nil {
		return chat.Thread{}, err
	}
	var out chat.Thread
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		out, err = s.restoreThreadUnit(ctx, st, a, id)
		return err
	})
	return out, err
}

// restoreThreadUnit is the unit of work of RestoreThread.
func (s *Service) restoreThreadUnit(ctx context.Context, st DeletionStore, a actor.Actor, id ids.ID) (chat.Thread, error) {
	if _, err := s.loadThread(ctx, a, authorize.ActionThreadRestore, id); err != nil {
		return chat.Thread{}, err
	}
	th, err := s.lockThread(ctx, id)
	if err != nil {
		return chat.Thread{}, err
	}
	if !th.Deleted() {
		out, err := s.threadView(ctx, th)
		if err != nil {
			return chat.Thread{}, err
		}
		return out, s.recordThread(ctx, authorize.ActionThreadRestore, id, auditing.Attrs{attrNoChange: auditing.Bool(true)})
	}
	if err := th.Deletion.CheckRestore(s.now(), s.window()); err != nil {
		return chat.Thread{}, err
	}
	th.Restore()
	if err := st.SaveThreadDeletion(ctx, th); err != nil {
		return chat.Thread{}, err
	}
	if err := s.recordThread(ctx, authorize.ActionThreadRestore, id, nil); err != nil {
		return chat.Thread{}, err
	}
	out, err := s.threadView(ctx, th)
	if err != nil {
		return chat.Thread{}, err
	}
	return out, s.emitThreadChange(ctx, th)
}

// ThreadDeletion implements chat.Deletions: thread.read on the thread, which authorize resolves
// also for a deleted thread.
func (s *Service) ThreadDeletion(ctx context.Context, a actor.Actor, id ids.ID) (chat.DeletionInfo, error) {
	if _, err := s.loadThread(ctx, a, authorize.ActionThreadRead, id); err != nil {
		return chat.DeletionInfo{}, err
	}
	return s.threadDeletion(ctx, id)
}

// threadDeletion counts the deletion of the thread id without authorization.
func (s *Service) threadDeletion(ctx context.Context, id ids.ID) (chat.DeletionInfo, error) {
	th, found, err := s.d.Store.Thread(ctx, id)
	if err != nil {
		return chat.DeletionInfo{}, err
	}
	if !found {
		return chat.DeletionInfo{}, invisible()
	}
	return s.deletionInfo(ctx, chat.TargetThread, id, th.Deletion)
}

// recordThread writes the allow entry of act on the thread id inside the unit of work.
func (s *Service) recordThread(ctx context.Context, act authorize.Action, id ids.ID, attrs auditing.Attrs) error {
	return s.d.Audit.Record(ctx, auditing.Entry{
		Action: string(act), Decision: auditing.DecisionAllow, ResourceKind: resourceThread, ResourceID: id, Attrs: attrs,
	})
}

// emitThreadChange appends thread.updated for the locked thread th in the space of its channel.
func (s *Service) emitThreadChange(ctx context.Context, th domain.Thread) error {
	c, err := s.threadChannel(ctx, th)
	if err != nil {
		return err
	}
	v, err := s.threadView(ctx, th)
	if err != nil {
		return err
	}
	return s.emitThread(ctx, authorize.EventThreadUpdated, c.Channel.SpaceID, v)
}

// deletionInfo counts the content of the item and states its deletion (D-DEL-6): channels,
// threads and messages from chat, files and sessions from the source ports of files and agents
// (zero when one is not bound).
func (s *Service) deletionInfo(ctx context.Context, kind string, id ids.ID, d domain.Deletion) (chat.DeletionInfo, error) {
	st, err := s.deletions()
	if err != nil {
		return chat.DeletionInfo{}, err
	}
	sc, err := st.DeletionScope(ctx, kind, id)
	if err != nil {
		return chat.DeletionInfo{}, err
	}
	out := chat.DeletionInfo{
		TargetKind: kind, TargetID: id, State: string(d.State(s.now(), s.window())), DeletedAt: d.At, UndoUntil: d.UndoUntil(s.window()),
		Contents: chat.DeletionContents{Channels: sc.Channels, Threads: sc.Threads, Messages: sc.Messages},
	}
	if len(sc.ThreadIDs) == 0 {
		return out, nil
	}
	if s.d.Files != nil {
		if out.Contents.Files, err = s.d.Files.FilesOfThreads(ctx, sc.ThreadIDs); err != nil {
			return chat.DeletionInfo{}, err
		}
	}
	if s.d.Sessions != nil {
		if out.Contents.Sessions, err = s.d.Sessions.SessionsOfThreads(ctx, sc.ThreadIDs); err != nil {
			return chat.DeletionInfo{}, err
		}
	}
	return out, nil
}

// DuePurges implements chat.Purges: the items deleted before now minus chat.delete_undo_window
// and not purged, with the channels whose messages go (every channel of a space).
func (s *Service) DuePurges(ctx context.Context, limit int) ([]chat.PurgeTarget, error) {
	st, err := s.deletions()
	if err != nil {
		return nil, err
	}
	items, err := st.DuePurges(ctx, s.now().Add(-s.window()), limit)
	if err != nil {
		return nil, err
	}
	out := make([]chat.PurgeTarget, 0, len(items))
	for _, it := range items {
		t := chat.PurgeTarget{Kind: it.Kind, ID: it.ID}
		switch it.Kind {
		case chat.TargetSpace:
			if t.ChannelIDs, err = s.d.Store.ChannelIDsOfSpace(ctx, it.ID); err != nil {
				return nil, err
			}
		case chat.TargetChannel:
			t.ChannelIDs = []ids.ID{it.ID}
		default:
			t.ThreadIDs = []ids.ID{it.ID}
		}
		out = append(out, t)
	}
	return out, nil
}

// FinishPurge implements chat.Purges in its own unit of work without an actor: the thread titles
// (title_source stays) and channel topics of the item go and purged_at is set; no event and no
// audit entry (D-DEL-9).
func (s *Service) FinishPurge(ctx context.Context, t chat.PurgeTarget) error {
	st, err := s.deletions()
	if err != nil {
		return err
	}
	return s.d.UoW.Do(ctx, func(ctx context.Context) error { return st.FinishPurge(ctx, t.Kind, t.ID, s.now()) })
}

// ProjectTrashRequests implements chat.ProjectTrash (D-DEL-11).
func (s *Service) ProjectTrashRequests(ctx context.Context, agentID ids.ID) ([]chat.ProjectTrashRequest, error) {
	st, err := s.deletions()
	if err != nil {
		return nil, err
	}
	return st.PendingTrash(ctx, agentID)
}

// ApplyProjectTrashResults implements chat.ProjectTrash (D-DEL-11) in one unit of work: the
// requests of agentID among the reported channels are locked in ascending id; each still
// requested one takes its result once, after channel.project.trash allowed the agent on it, with
// an audit entry (result, reason). Every other result is ignored.
func (s *Service) ApplyProjectTrashResults(ctx context.Context, agentID ids.ID, results []chat.ProjectTrashResult) error {
	if len(results) == 0 || agentID.IsZero() {
		return nil
	}
	st, err := s.deletions()
	if err != nil {
		return err
	}
	byChannel, channels := trashResults(results)
	a, _ := actor.FromContext(ctx)
	return s.d.UoW.Do(ctx, func(ctx context.Context) error {
		open, err := st.LockTrashRequests(ctx, agentID, channels)
		if err != nil {
			return err
		}
		for _, id := range open {
			if err := s.applyTrashResult(ctx, st, a, agentID, byChannel[id]); err != nil {
				return err
			}
		}
		return nil
	})
}

// trashResults keeps the first result per channel with a known outcome and returns them by
// channel and the channels in ascending id.
func trashResults(results []chat.ProjectTrashResult) (map[ids.ID]chat.ProjectTrashResult, []ids.ID) {
	byChannel := make(map[ids.ID]chat.ProjectTrashResult, len(results))
	channels := make([]ids.ID, 0, len(results))
	for _, r := range results {
		if _, dup := byChannel[r.ChannelID]; dup || (r.Result != chat.TrashTrashed && r.Result != chat.TrashRefused) {
			continue
		}
		byChannel[r.ChannelID] = r
		channels = append(channels, r.ChannelID)
	}
	slices.SortFunc(channels, compareIDs)
	return byChannel, channels
}

// applyTrashResult stores r for the locked request of agentID when channel.project.trash allows
// a, with its audit entry; a refused decision leaves the request as it is.
func (s *Service) applyTrashResult(ctx context.Context, st DeletionStore, a actor.Actor, agentID ids.ID, r chat.ProjectTrashResult) error {
	d, err := s.d.Authorizer.Authorize(ctx, a, authorize.ActionChannelProjectTrash,
		authorize.Resource{Kind: authorize.ResourceChannel, ID: r.ChannelID, OwnerID: agentID})
	if err != nil || !d.Allowed() {
		return err
	}
	reason := ""
	if r.Result == chat.TrashRefused {
		reason = r.Reason
	}
	if err := st.SetTrashResult(ctx, r.ChannelID, r.Result, reason); err != nil {
		return err
	}
	attrs := auditing.Attrs{attrResult: auditing.Code(r.Result)}
	if reason != "" {
		attrs[attrReasonCode] = auditing.Code(reason)
	}
	return s.recordChannel(ctx, authorize.ActionChannelProjectTrash, r.ChannelID, attrs)
}
