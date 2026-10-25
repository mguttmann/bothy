// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"slices"
	"time"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/messaging"
	"github.com/OWNER/bothy/services/server/internal/messaging/domain"
)

// caller and messageResource (mentions.go) are shared with the mention list (INT-W22/4).

// visible returns the channels in which a may read messages (07 section 7.4 rule 1); every
// statement of editing and deleting carries them.
func (s *Service) visible(ctx context.Context, a actor.Actor) ([]ids.ID, error) {
	sc, err := s.d.Authorizer.Scope(ctx, a, authorize.ActionMessageRead)
	if err != nil {
		return nil, err
	}
	return sc.ChannelIDs, nil
}

// loadMessage reads the message id in the visible channels of a and authorizes act on it before
// it answers a missing one: a missing and an invisible message take the same path, an unknown
// thread hint, and give the same 404 (02 section 9.2).
func (s *Service) loadMessage(ctx context.Context, a actor.Actor, act authorize.Action, id ids.ID) (domain.Message, []ids.ID, error) {
	return s.loadAs(ctx, a, act, id, messageResource)
}

// loadAs is loadMessage with the resource of act built by resource from the message (the
// reaction of the caller for reaction.remove, M1-59); a missing message is checked as that
// resource of a message with the id as thread hint, which no thread has.
func (s *Service) loadAs(ctx context.Context, a actor.Actor, act authorize.Action, id ids.ID,
	resource func(domain.Message) authorize.Resource) (domain.Message, []ids.ID, error) {
	visible, err := s.visible(ctx, a)
	if err != nil {
		return domain.Message{}, nil, err
	}
	m, found, err := s.d.Store.VisibleMessage(ctx, id, visible)
	if err != nil {
		return domain.Message{}, nil, err
	}
	r := resource(domain.Message{ID: id, ThreadID: id})
	if found {
		r = resource(m)
	}
	if err := s.check(ctx, a, act, r); err != nil {
		return domain.Message{}, nil, err
	}
	if !found {
		return domain.Message{}, nil, invisible()
	}
	return m, visible, nil
}

// lockVisible locks the message id inside the unit of work; a message that left the visible
// channels in the meantime is 404.
func (s *Service) lockVisible(ctx context.Context, id ids.ID, visible []ids.ID) (domain.Message, error) {
	m, found, err := s.d.Store.LockMessage(ctx, id, visible)
	if err != nil {
		return domain.Message{}, err
	}
	if !found {
		return domain.Message{}, invisible()
	}
	return m, nil
}

// Update implements messaging.Messages (M1-58 design note 2). The precedence is 404 > 403 > 410 >
// 412 > 413 > 400; 428 is the HTTP chain's.
func (s *Service) Update(ctx context.Context, c messaging.UpdateCommand) (messaging.Message, error) {
	a := caller(ctx)
	m, visible, err := s.loadMessage(ctx, a, authorize.ActionMessageUpdate, c.MessageID)
	if err != nil {
		return messaging.Message{}, err
	}
	if m.Kind == domain.KindSystem {
		// A system line is the server's, even though its author column names a person.
		return messaging.Message{}, errs.New(errs.CodeForbidden, "a system line cannot be edited")
	}
	var out messaging.Message
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		out, err = s.edit(ctx, a, c, visible)
		return err
	})
	return out, err
}

// edit is the unit of work of Update: the message row lock, 410, 412, the resulting content and
// mentions, then nothing for no change, otherwise the revision, the row, the mentions and
// message.updated. It never publishes MessagePosted and never touches trigger_state (09 section
// 6: only new human messages trigger).
func (s *Service) edit(ctx context.Context, a actor.Actor, c messaging.UpdateCommand, visible []ids.ID) (messaging.Message, error) {
	row, err := s.lockVisible(ctx, c.MessageID, visible)
	if err != nil {
		return messaging.Message{}, err
	}
	if !row.DeletedAt.IsZero() {
		return messaging.Message{}, errs.New(errs.CodeMessageDeleted, "the message was deleted")
	}
	cur, err := s.stored(ctx, row, visible)
	if err != nil {
		return messaging.Message{}, err
	}
	if row.Version != c.IfMatch {
		return messaging.Message{}, errs.VersionConflict(messageBody(cur))
	}
	content, mentions, info, err := s.resulting(ctx, a, c, row)
	if err != nil {
		return messaging.Message{}, err
	}
	if domain.Unchanged(row.ContentMD, cur.MentionIDs, content, mentions) {
		return cur, nil
	}
	now := s.now()
	if domain.StoresRevision(row.Kind) {
		rev := domain.Revision{ID: ids.New(), MessageID: row.ID, ContentMD: row.ContentMD, EditedBy: a.ID, CreatedAt: now}
		if err := s.d.Store.InsertRevision(ctx, rev, visible); err != nil {
			return messaging.Message{}, err
		}
	}
	updated, err := s.d.Store.UpdateContent(ctx, row.ID, visible, content, now)
	if err != nil {
		return messaging.Message{}, err
	}
	if err := s.replaceMentions(ctx, row, visible, cur.MentionIDs, mentions, now); err != nil {
		return messaging.Message{}, err
	}
	out := view(updated, cur.Author, mentions)
	// The row lock serialises reactions, pins and component changes (runs locks the card through
	// LockCard, M2-15) with the edit, so those of cur are current; an edit never changes the
	// attachments (03 messages, M3-07), so those of cur are the message's.
	out.Reactions, out.MyReactions, out.Pinned, out.Components = cur.Reactions, cur.MyReactions, cur.Pinned, cur.Components
	out.Attachments = cur.Attachments
	return out, s.emitMessage(ctx, authorize.EventMessageUpdated, info, out, false)
}

// resulting returns the content of row after the edit c, its mentions resolved again for the
// author a with the rules of sending (413 and 400 on the content, 400 mention_mismatch for a given
// mention_ids that differs) and the routing data of its thread.
func (s *Service) resulting(ctx context.Context, a actor.Actor, c messaging.UpdateCommand, row domain.Message) (string, []ids.ID, chat.ThreadInfo, error) {
	content := row.ContentMD
	if c.Content != nil {
		content = *c.Content
		if err := domain.CheckContent(content, s.cfg.MaxContentBytes); err != nil {
			return "", nil, chat.ThreadInfo{}, err
		}
	}
	handles := domain.ScanMentions(content)
	if err := domain.CheckMentionCount(handles, s.cfg.MaxMentions); err != nil {
		return "", nil, chat.ThreadInfo{}, err
	}
	info, err := s.d.Threads.Info(ctx, row.ThreadID)
	if err != nil {
		return "", nil, chat.ThreadInfo{}, err
	}
	mentions, err := s.resolveMentions(ctx, a, handles, threadResource(info))
	if err != nil {
		return "", nil, chat.ThreadInfo{}, err
	}
	if c.MentionIDs != nil && !sameSet(*c.MentionIDs, mentions) {
		return "", nil, chat.ThreadInfo{}, domain.MentionMismatch()
	}
	return content, mentions, info, nil
}

// replaceMentions removes the mentions of row that are not in next and inserts the new ones
// with the thread id copy.
func (s *Service) replaceMentions(ctx context.Context, row domain.Message, visible, prev, next []ids.ID, at time.Time) error {
	if err := s.d.Store.DeleteMentionsExcept(ctx, row.ID, visible, next); err != nil {
		return err
	}
	var added []ids.ID
	for _, id := range next {
		if !slices.Contains(prev, id) {
			added = append(added, id)
		}
	}
	return s.d.Store.InsertMentions(ctx, row.ID, row.ThreadID, added, at)
}

// Delete implements messaging.Messages (M1-58 design note 3).
func (s *Service) Delete(ctx context.Context, messageID ids.ID) error {
	a := caller(ctx)
	m, visible, err := s.loadMessage(ctx, a, authorize.ActionMessageDelete, messageID)
	if err != nil || !m.DeletedAt.IsZero() {
		return err
	}
	return s.d.UoW.Do(ctx, func(ctx context.Context) error { return s.remove(ctx, a, m, visible) })
}

// remove is the unit of work of Delete in the lock order of the README: the thread row through
// chat, then the message row, then (M3-07) the files rows of its attachments in ascending id and
// the rows of message_attachments through files; a message deleted in the meantime ends the unit
// without a write.
func (s *Service) remove(ctx context.Context, a actor.Actor, m domain.Message, visible []ids.ID) error {
	if err := s.d.Threads.LockForMessageChange(ctx, m.ThreadID); err != nil {
		return err
	}
	row, err := s.lockVisible(ctx, m.ID, visible)
	if err != nil || !row.DeletedAt.IsZero() {
		return err
	}
	mentions, err := s.d.Store.Mentions(ctx, row.ID)
	if err != nil {
		return err
	}
	tomb, err := s.d.Store.Tombstone(ctx, row.ID, visible, a.ID, s.now())
	if err != nil {
		return err
	}
	if err := s.removeDependents(ctx, row, visible); err != nil {
		return err
	}
	if err := s.d.Threads.DecrementMessageCount(ctx, row.ThreadID); err != nil {
		return err
	}
	if err := s.redact(ctx, tomb); err != nil {
		return err
	}
	if err := txn.Publish(ctx, messaging.MessageDeleted{
		MessageID: tomb.ID, ThreadID: tomb.ThreadID, ChannelID: tomb.ChannelID, ThreadSeq: tomb.ThreadSeq, AuthorID: tomb.AuthorID,
		DeletedBy: a.ID, MentionedPrincipalIDs: mentions, Version: tomb.Version,
	}); err != nil {
		return err
	}
	info, err := s.d.Threads.Info(ctx, tomb.ThreadID)
	if err != nil {
		return err
	}
	if err := s.emitDeleted(ctx, info, tomb); err != nil {
		return err
	}
	if a.ID == tomb.AuthorID || a.ID.IsZero() {
		// The author's own deletion is not audited (05 message.delete); the job retention.content
		// has no actor and runs no action of the catalogue (M7-05).
		return nil
	}
	return s.d.Audit.Record(ctx, auditing.Entry{
		Action: string(authorize.ActionMessageDelete), Decision: auditing.DecisionAllow, ResourceKind: resourceMessage, ResourceID: tomb.ID,
		Attrs: auditing.Attrs{
			"thread_id": auditing.ID(tomb.ThreadID), "channel_id": auditing.ID(tomb.ChannelID), "author_principal_id": auditing.ID(tomb.AuthorID),
		},
	})
}

// removeDependents deletes what goes with a deleted message in its unit of work: revisions and
// mentions (M1-58), reactions and the pin (M1-59 design note 9), and, when the locked row had the
// flag, the attachments with the deleted_at of their files through files (brief M3-07 design note
// 7, 03 messages, 12 section 12). The tombstone UPDATE already reset has_attachments.
func (s *Service) removeDependents(ctx context.Context, row domain.Message, visible []ids.ID) error {
	if err := s.d.Store.DeleteRevisions(ctx, row.ID, visible); err != nil {
		return err
	}
	if err := s.d.Store.DeleteMentionsExcept(ctx, row.ID, visible, nil); err != nil {
		return err
	}
	if err := s.d.Store.DeleteReactions(ctx, row.ID, visible); err != nil {
		return err
	}
	if _, _, err := s.d.Store.DeletePin(ctx, row.ID, visible); err != nil {
		return err
	}
	if !row.HasAttachments {
		return nil
	}
	if s.d.Files == nil {
		return errs.New(errs.CodeInternalError, "messaging: a message with attachments in a service without files")
	}
	return s.d.Files.DeleteForMessage(ctx, row.ID)
}

// redact replaces the payloads of the stored events of the deleted message tomb with their
// redacted form (03 section 6.5, design note 6): the tombstone in its event form.
func (s *Service) redact(ctx context.Context, tomb domain.Message) error {
	author, err := s.author(ctx, tomb.AuthorID)
	if err != nil {
		return err
	}
	body, err := encode(messageBody(view(tomb, author, nil)))
	if err != nil {
		return err
	}
	_, err = s.d.Redactor.Redact(ctx, resourceMessage, tomb.ID, domain.RedactPayload(body))
	return err
}

// deletedJSON is the payload MessageDeletedPayload.
type deletedJSON struct {
	MessageID string `json:"message_id"`
	ThreadID  string `json:"thread_id"`
	ThreadSeq int64  `json:"thread_seq"`
	DeletedAt string `json:"deleted_at"`
	// DeletedBy is absent for a deletion by the job retention.content, which has no actor
	// (M7-05; the member is optional in MessageDeletedPayload).
	DeletedBy string `json:"deleted_by,omitempty"`
}

// emitDeleted appends message.deleted for the tombstone tomb in the thread of info, after the
// redaction, so it is never redacted: resource the message with its new version, scope space,
// channel and thread, receive action message.read.
func (s *Service) emitDeleted(ctx context.Context, info chat.ThreadInfo, tomb domain.Message) error {
	p, err := encode(deletedJSON{
		MessageID: tomb.ID.String(), ThreadID: tomb.ThreadID.String(), ThreadSeq: tomb.ThreadSeq, DeletedAt: stamp(tomb.DeletedAt),
		DeletedBy: deletedBy(tomb.DeletedBy),
	})
	if err != nil {
		return err
	}
	kind, actorID := eventActor(ctx)
	return s.d.Events.Append(ctx, eventlog.Event{
		Type: string(authorize.EventMessageDeleted), SchemaVersion: 1, ActorKind: kind, ActorPrincipalID: actorID,
		SpaceID: info.SpaceID, ChannelID: tomb.ChannelID, ThreadID: tomb.ThreadID, ResourceKind: resourceMessage, ResourceID: tomb.ID,
		ResourceVersion: tomb.Version, ReceiveAction: receiveMessageRead, Payload: p,
	})
}

// Revisions implements messaging.Messages (M1-58 design note 5): ascending, unpaginated, empty for
// a deleted message.
func (s *Service) Revisions(ctx context.Context, messageID ids.ID) ([]messaging.RevisionView, error) {
	a := caller(ctx)
	m, visible, err := s.loadMessage(ctx, a, authorize.ActionMessageRevisionsRead, messageID)
	if err != nil {
		return nil, err
	}
	out := []messaging.RevisionView{}
	if !m.DeletedAt.IsZero() {
		return out, nil
	}
	rows, err := s.d.Store.Revisions(ctx, m.ID, visible)
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		out = append(out, messaging.RevisionView{Revision: r.Number, ContentMD: r.ContentMD, EditedBy: r.EditedBy, CreatedAt: r.CreatedAt})
	}
	return out, nil
}

// deletedBy is the member deleted_by of message.deleted: empty for a deletion without an actor.
func deletedBy(id ids.ID) string {
	if id.IsZero() {
		return ""
	}
	return id.String()
}
