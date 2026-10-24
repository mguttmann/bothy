// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/importer"
	"github.com/OWNER/bothy/services/server/internal/importer/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/messaging"
)

// Target kinds of import_items.target_kind for the content items (M8-11). A pin item names the
// pinned message, because the pin has no id of its own (pins is keyed by message_id).
const (
	targetThread  = "thread"
	targetMessage = "message"
	targetFile    = "file"
)

// threadRun is the state of one thread batch: its thread and whether the batch changed it.
type threadRun struct {
	*batch
	threadID ids.ID
	// changed reports a new message, a new pin, a new thread or a mirrored change: the batch ends
	// with the floor, the version and thread.updated (M8-11 design note 5).
	changed bool
}

// contentDeps refuses a thread batch of a service without the content ports.
func (s *Service) contentDeps() error {
	if s.d.Threads == nil || s.d.Messages == nil {
		return errs.New(errs.CodeInternalError, "importer: the service has no ports for the content items")
	}
	return nil
}

// threadNotImported is 409 conflict thread_not_imported (M8-11 design note 1).
func threadNotImported(detail string) error {
	return conflict(importer.ReasonThreadNotImported, detail)
}

// posted returns the posted item of key in the source; found is false for a missing, pending,
// skipped or failed item.
func (s *Service) posted(ctx context.Context, bt *batch, key string) (ItemRow, bool, error) {
	row, found, err := s.d.Store.Item(ctx, bt.src.ID, key)
	if err != nil || !found || row.State != importer.ItemPosted || row.TargetID.IsZero() {
		return ItemRow{}, false, err
	}
	return row, true, nil
}

// applyThreadBatch writes a thread batch (M8-11 design notes 1 to 5, 22 7.2 and 8): the thread of
// the key (the main thread of a posted text channel, a posted thread, or the thread its first
// item creates or mirrors), then every message and pin in request order, then the end of the
// batch: when something changed, the floor, the version and one thread.updated through
// chat.ImportThreads.FinishBatch, then the counters and import.progress. A batch that only
// answered posted items writes nothing (22 7.2 item 1).
func (s *Service) applyThreadBatch(ctx context.Context, a actor.Actor, bt *batch, tk domain.ThreadKey, items []parsed) (importer.BatchResult, error) {
	if err := s.contentDeps(); err != nil {
		return importer.BatchResult{}, err
	}
	run := &threadRun{batch: bt}
	out := importer.BatchResult{Items: make([]importer.Item, 0, len(items))}
	first, err := s.resolveThread(ctx, run, tk, items)
	if err != nil {
		return importer.BatchResult{}, err
	}
	start := 0
	if first != nil {
		out.Items, start = append(out.Items, *first), 1
	}
	if run.threadID.IsZero() {
		if len(items) > start {
			return importer.BatchResult{}, threadNotImported("the thread is skipped")
		}
		return out, s.count(ctx, bt)
	}
	for i := start; i < len(items); i++ {
		it, err := s.contentItem(ctx, a, run, items[i])
		if err != nil {
			return importer.BatchResult{}, s.refused(ctx, bt, err, i)
		}
		out.Items = append(out.Items, it)
	}
	floor, err := s.d.Threads.FinishBatch(ctx, run.threadID, run.changed)
	if err != nil {
		return importer.BatchResult{}, err
	}
	out.UnreadFloorSeq = &floor
	bt.changed = run.changed
	return out, s.count(ctx, bt)
}

// resolveThread sets the thread of run: the main thread of a channel: key, the thread the first
// item creates or mirrors (returned as the first item of the answer), or a posted thread.
func (s *Service) resolveThread(ctx context.Context, run *threadRun, tk domain.ThreadKey, items []parsed) (*importer.Item, error) {
	switch {
	case tk.Kind == domain.KindChannel:
		return nil, s.mainThread(ctx, run, tk)
	case len(items) > 0 && items[0].obj.ItemKind == domain.KindThread:
		it, err := s.threadItem(ctx, run, items[0])
		if err != nil {
			return nil, s.refused(ctx, run.batch, err, 0)
		}
		return &it, nil
	}
	row, found, err := s.posted(ctx, run.batch, tk.Key)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, threadNotImported("the thread is not imported")
	}
	run.threadID = row.TargetID
	return nil, nil
}

// refused records the denial of a target outside the source and puts the field pointers of item i
// under /items/<i>/data.
func (s *Service) refused(ctx context.Context, bt *batch, err error, i int) error {
	if e, ok := errs.As(err); ok && e.Reason == importer.ReasonTargetOutsideSource {
		s.deny(ctx, bt.src.ID, importer.ReasonTargetOutsideSource)
	}
	return itemError(err, i)
}

// mainThread resolves the main thread of a channel: key (22 8): the channel item must be posted;
// a forum is 400 validation_failed at /thread_origin_key.
func (s *Service) mainThread(ctx context.Context, run *threadRun, tk domain.ThreadKey) error {
	ch, found, err := s.posted(ctx, run.batch, tk.Key)
	if err != nil {
		return err
	}
	if !found {
		return threadNotImported("the channel is not imported")
	}
	run.threadID, err = s.d.Threads.MainThread(ctx, ch.TargetID)
	return err
}

// threadItem writes the thread item of a thread: batch (M8-11 design note 3): a new key creates
// the thread; a posted key is answered and its thread mirrored (22 12.2 step 3).
func (s *Service) threadItem(ctx context.Context, run *threadRun, x parsed) (importer.Item, error) {
	row, created, err := s.d.Store.InsertPendingItem(ctx, NewItem{
		ID: ids.New(), SourceID: run.src.ID, OriginKey: x.obj.OriginKey, ItemKind: x.obj.ItemKind, ContentSHA256: x.sha, At: s.now(),
	})
	if err != nil {
		return importer.Item{}, err
	}
	if !created {
		it, err := s.replay(ctx, run.batch, x)
		if err != nil || it.State != importer.ItemPosted {
			return it, err
		}
		run.threadID = it.TargetID
		return it, s.mirror(ctx, run, x.thread)
	}
	target, err := s.newThread(ctx, run.batch, x.thread)
	if err != nil {
		return importer.Item{}, err
	}
	state, kind := importer.ItemPosted, targetThread
	if target.IsZero() {
		state, kind = importer.ItemSkipped, ""
	}
	if row, err = s.d.Store.FinishItem(ctx, row.ID, state, kind, target, s.now()); err != nil {
		return importer.Item{}, err
	}
	run.counts.Add(x.obj.ItemKind, state)
	run.threadID, run.changed = target, !target.IsZero()
	return itemDTO(row, false), nil
}

// mirror applies the mirror rules to the posted thread of run (M8-11 design note 3 b).
func (s *Service) mirror(ctx context.Context, run *threadRun, d domain.ThreadData) error {
	st, err := s.d.Threads.Lock(ctx, run.threadID)
	if err != nil {
		return err
	}
	m := domain.MirrorChange(domain.StoredThread{Title: st.Title, AutoTitle: st.TitleSource == "auto", Closed: st.Closed,
		LastActivityAt: st.LastActivityAt}, d, s.cfg.ThreadTitleMaxChars)
	if m.Empty() {
		return nil
	}
	run.changed = true
	return s.d.Threads.ApplyMirror(ctx, run.threadID, chat.MirrorChange{Title: m.Title, CloseAt: m.CloseAt, Reopen: m.Reopen,
		LastActivityAt: m.LastActivityAt})
}

// newThread creates the thread of a new thread item (22 5.1.1, M8-11 design note 3 a): the channel
// of the posted channel item (a skipped channel skips the thread), the planned operator of the
// mapping (never the effective operator, R-IM-08), the starter message when it is posted, the
// posted forum tags of the same channel.
func (s *Service) newThread(ctx context.Context, bt *batch, d domain.ThreadData) (ids.ID, error) {
	ch, found, err := s.d.Store.Item(ctx, bt.src.ID, d.ChannelOriginKey)
	switch {
	case err != nil:
		return ids.ID{}, err
	case found && ch.State == importer.ItemSkipped:
		return ids.ID{}, nil
	case !found || ch.State != importer.ItemPosted || ch.TargetID.IsZero():
		return ids.ID{}, outside("the channel of the thread is not imported")
	}
	channelKey, _ := domain.ParseOriginKey(domain.KindChannel, d.ChannelOriginKey)
	t := chat.ImportedThread{ChannelID: ch.TargetID, Name: d.Name, CreatedAt: d.CreatedAt, LastActivityAt: d.LastActivityAt}
	if d.Closed {
		t.ClosedAt = d.ClosedAt
	}
	if t.OperatorAgentID, err = s.plannedOperator(ctx, bt, channelKey.IDs[0]); err != nil {
		return ids.ID{}, err
	}
	if t.Parent, err = s.starter(ctx, bt, d.StarterMessageOriginKey); err != nil {
		return ids.ID{}, err
	}
	if t.TagIDs, err = s.tags(ctx, bt, channelKey.IDs[0], d.TagOriginKeys); err != nil {
		return ids.ID{}, err
	}
	return s.d.Threads.Create(ctx, t)
}

// plannedOperator resolves planned_operator of the channel in the stored mapping; none is zero. An
// agent that no longer exists is outside the source.
func (s *Service) plannedOperator(ctx context.Context, bt *batch, channel string) (ids.ID, error) {
	e := bt.m.Channel(channel)
	if e == nil || e.PlannedOperator == "" {
		return ids.ID{}, nil
	}
	id, err := s.d.People.Resolve(ctx, identity.ImportRef{Kind: actor.KindAgent, Handle: e.PlannedOperator})
	if ee, ok := errs.As(err); ok && ee.Code == errs.CodeNotFound {
		return ids.ID{}, outside("the planned operator does not exist")
	}
	return id, err
}

// starter returns the starter message of a thread when its item is posted, with its thread; chat
// keeps it only when that is the main thread of the channel.
func (s *Service) starter(ctx context.Context, bt *batch, key string) (*chat.ParentMessage, error) {
	if key == "" {
		return nil, nil
	}
	row, found, err := s.posted(ctx, bt, key)
	if err != nil || !found {
		return nil, err
	}
	thread, found, err := s.d.Messages.ThreadOf(ctx, row.TargetID)
	if err != nil || !found {
		return nil, err
	}
	return &chat.ParentMessage{MessageID: row.TargetID, ThreadID: thread}, nil
}

// tags returns the targets of the posted forum_tag items among keys; a key of another channel is
// 400 at its pointer.
func (s *Service) tags(ctx context.Context, bt *batch, channel string, keys []string) ([]ids.ID, error) {
	var p domain.Problems
	out := make([]ids.ID, 0, len(keys))
	for i, k := range keys {
		key, _ := domain.ParseOriginKey(domain.KindForumTag, k)
		if key.IDs[0] != channel {
			p.Add(domain.Pointer("/tag_origin_keys", i), domain.FieldInvalid, "the tag belongs to another channel")
			continue
		}
		row, found, err := s.posted(ctx, bt, k)
		if err != nil {
			return nil, err
		}
		if found {
			out = append(out, row.TargetID)
		}
	}
	return out, p.Err()
}

// contentItem writes one message or pin item of the thread of run: an existing key is answered
// with its stored item, a new key gets its target and posted in the same unit.
func (s *Service) contentItem(ctx context.Context, a actor.Actor, run *threadRun, x parsed) (importer.Item, error) {
	row, created, err := s.d.Store.InsertPendingItem(ctx, NewItem{
		ID: ids.New(), SourceID: run.src.ID, OriginKey: x.obj.OriginKey, ItemKind: x.obj.ItemKind, ContentSHA256: x.sha, At: s.now(),
	})
	if err != nil {
		return importer.Item{}, err
	}
	if !created {
		return s.replay(ctx, run.batch, x)
	}
	var target ids.ID
	if x.obj.ItemKind == domain.KindMessage {
		target, err = s.message(ctx, run, row.ID, x.msg)
	} else {
		target, err = s.pin(ctx, a, run, x.pin)
	}
	if err != nil {
		return importer.Item{}, err
	}
	if row, err = s.d.Store.FinishItem(ctx, row.ID, importer.ItemPosted, targetMessage, target, s.now()); err != nil {
		return importer.Item{}, err
	}
	run.counts.Add(x.obj.ItemKind, importer.ItemPosted)
	run.changed = true
	return itemDTO(row, false), nil
}

// message appends a message item (M8-11 design note 4): the author of the posted user item (409
// author_not_imported otherwise), the reply target of a posted message item, the files of posted
// file items of this source, then messaging.ImportMessages.Append in the unit of the batch.
func (s *Service) message(ctx context.Context, run *threadRun, itemID ids.ID, d domain.MessageData) (ids.ID, error) {
	author, found, err := s.posted(ctx, run.batch, d.AuthorOriginKey)
	if err != nil {
		return ids.ID{}, err
	}
	if !found {
		return ids.ID{}, conflict(importer.ReasonAuthorNotImported, "the author is not imported")
	}
	m := messaging.ImportedMessage{ImportItemID: itemID, AuthorID: author.TargetID, ContentMD: d.ContentMD, CreatedAt: d.CreatedAt,
		EditedAt: d.EditedAt, AttachmentIDs: d.AttachmentFileIDs}
	if d.ReplyToOriginKey != "" {
		reply, found, err := s.posted(ctx, run.batch, d.ReplyToOriginKey)
		if err != nil {
			return ids.ID{}, err
		}
		if !found {
			return ids.ID{}, fieldError("/reply_to_origin_key", "not_in_thread", "the reply target is not imported in this thread")
		}
		m.ReplyToID = reply.TargetID
	}
	if err := s.importedFiles(ctx, run.batch, d.AttachmentFileIDs); err != nil {
		return ids.ID{}, err
	}
	out, err := s.d.Messages.Append(ctx, run.threadID, []messaging.ImportedMessage{m})
	if err != nil {
		return ids.ID{}, err
	}
	return out[0], nil
}

// importedFiles checks that every file is the target of a posted file item of the source (M8-11
// design note 4 d); files checks the channel, the author and the state when it attaches.
func (s *Service) importedFiles(ctx context.Context, bt *batch, list []ids.ID) error {
	var p domain.Problems
	for i, id := range list {
		row, found, err := s.d.Store.ItemOfTarget(ctx, bt.src.ID, targetFile, id)
		if err != nil {
			return err
		}
		if !found || row.State != importer.ItemPosted {
			p.Add(domain.Pointer("/attachment_file_ids", i), domain.FieldNotImported, "the file is not a posted file item of this source")
		}
	}
	return p.Err()
}

// pin pins the message of a posted message item with the importing owner as pinned_by (22 6.6).
func (s *Service) pin(ctx context.Context, a actor.Actor, run *threadRun, d domain.PinData) (ids.ID, error) {
	row, found, err := s.posted(ctx, run.batch, d.MessageOriginKey)
	if err != nil {
		return ids.ID{}, err
	}
	if !found {
		return ids.ID{}, fieldError("/message_origin_key", domain.FieldNotImported, "the pinned message is not imported")
	}
	return row.TargetID, s.d.Messages.Pin(ctx, run.threadID, row.TargetID, a.ID)
}

// fieldError is a 400 validation_failed with one entry at pointer.
func fieldError(pointer, code, detail string) error {
	return errs.New(errs.CodeValidationFailed, detail).WithFields(errs.FieldError{Pointer: pointer, Code: code, Detail: detail})
}
