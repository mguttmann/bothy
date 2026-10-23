// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"slices"
	"time"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/chat/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// ImportThreadStore is the outgoing port of the imported threads (M8-11) over threads;
// adapters/postgres implements it. Every method needs a unit of work. Errors are kernel/errs
// errors without SQL text or values.
type ImportThreadStore interface {
	// InsertImportedThread inserts th of kind thread with its state, closed_at, created_at and
	// last_activity_at and returns the stored row. Errors: conflict on /parent_message_id for a
	// parent that started a thread already.
	InsertImportedThread(ctx context.Context, th domain.Thread) (domain.Thread, error)
	// SaveImportedThread writes title, state, closed_at, closed_by and last_activity_at of th
	// without the version.
	SaveImportedThread(ctx context.Context, th domain.Thread) error
	// ReserveImportedSeqs raises last_message_seq and message_count of the locked thread id by n
	// and last_activity_at to lastActivity when later, and returns the row after the change.
	ReserveImportedSeqs(ctx context.Context, id ids.ID, n int, lastActivity time.Time) (domain.Thread, error)
	// FinishImportBatch sets unread_floor_seq to last_message_seq, raises the version by one and
	// returns the row after the change.
	FinishImportBatch(ctx context.Context, id ids.ID) (domain.Thread, error)
}

// ImportThreadsPort implements chat.ImportThreads over the service (M8-11); Service.ImportThreads
// returns it. A type of its own, because Service implements chat.Threads, whose Create has
// another signature.
type ImportThreadsPort struct{ s *Service }

// Compile-time check: the import port of the threads.
var _ chat.ImportThreads = ImportThreadsPort{}

// ImportThreads returns the port chat.ImportThreads over s.
func (s *Service) ImportThreads() ImportThreadsPort { return ImportThreadsPort{s: s} }

// importThreads returns the store of the imported threads; internal when Deps had none.
func (s *Service) importThreads(ctx context.Context) (ImportThreadStore, error) {
	if err := inUnit(ctx, "ImportThreads"); err != nil {
		return nil, err
	}
	if s.d.ImportThreads == nil {
		return nil, errs.New(errs.CodeInternalError, "chat: the service has no store of imported threads")
	}
	return s.d.ImportThreads, nil
}

// importTitle cuts name to thread.title_max_chars at a rune boundary (22 5.1.1).
func importTitle(name string, maxChars int) string {
	if r := []rune(name); len(r) > maxChars {
		return string(r[:maxChars])
	}
	return name
}

// Create implements chat.ImportThreads (M8-11 design note 3 a, 22 5.1.1): the live text or forum
// channel, the title cut and checked like a title of thread.create, title_source by the
// placeholder rule of the auto-title (a placeholder name stays auto, any other name is manual),
// the planned operator as given (never the effective operator of the channel, R-IM-08), the times
// and the done state of the source, the parent only when it lies in the main thread of the channel
// and started no thread yet, the live tags of the forum. No event: the batch appends thread.updated
// at its end.
func (p ImportThreadsPort) Create(ctx context.Context, t chat.ImportedThread) (ids.ID, error) {
	s := p.s
	st, err := s.importThreads(ctx)
	if err != nil {
		return ids.ID{}, err
	}
	a, err := importActor(ctx)
	if err != nil {
		return ids.ID{}, err
	}
	c, found, err := s.d.Store.Channel(ctx, t.ChannelID)
	if err != nil {
		return ids.ID{}, err
	}
	if !found || archived(c) || c.Channel.Kind == domain.ChannelDM {
		return ids.ID{}, invisible()
	}
	title := importTitle(t.Name, s.cfg.ThreadTitleMaxChars)
	if err := domain.CheckThreadTitle(title, s.cfg.ThreadTitleMaxChars); err != nil {
		return ids.ID{}, asNamePointer(err)
	}
	th := domain.Thread{
		ID: ids.New(), ChannelID: c.Channel.ID, Kind: domain.ThreadKindThread, State: domain.ThreadOpen, Title: title,
		TitleSource: domain.TitleManual, OperatorAgentID: t.OperatorAgentID, CreatedBy: a.ID, CreatedAt: t.CreatedAt,
		LastActivityAt: t.LastActivityAt, Version: 1,
	}
	if s.placeholders.IsPlaceholder(title) {
		th.TitleSource = domain.TitleAuto
	}
	if !t.ClosedAt.IsZero() {
		th.State, th.ClosedAt = domain.ThreadClosed, t.ClosedAt
	}
	if th.ParentMessageID, err = s.importParent(ctx, c, t.Parent); err != nil {
		return ids.ID{}, err
	}
	if th.SearchLanguage, err = s.d.Directory.InstanceSearchLanguage(ctx); err != nil {
		return ids.ID{}, err
	}
	if th, err = st.InsertImportedThread(ctx, th); err != nil {
		return ids.ID{}, err
	}
	tags, err := s.importTags(ctx, c, t.TagIDs)
	if err != nil || len(tags) == 0 {
		return th.ID, err
	}
	return th.ID, s.d.Store.ReplaceThreadTags(ctx, th.ID, tags, a.ID, th.CreatedAt)
}

// asNamePointer moves the field errors of the title check from /title to /name, the member of the
// thread item.
func asNamePointer(err error) error {
	e, ok := errs.As(err)
	if !ok || e.Code != errs.CodeValidationFailed {
		return err
	}
	fields := make([]errs.FieldError, 0, len(e.Fields))
	for _, f := range e.Fields {
		f.Pointer = "/name"
		fields = append(fields, f)
	}
	return errs.New(errs.CodeValidationFailed, e.Detail).WithFields(fields...)
}

// importParent returns the parent of an imported thread (22 5.1.1): the message only when it lies
// in the main thread of the text channel c and no thread started from it yet; else zero.
func (s *Service) importParent(ctx context.Context, c ChannelRow, p *chat.ParentMessage) (ids.ID, error) {
	if p == nil || c.Channel.Kind != domain.ChannelText || p.ThreadID.IsZero() || p.ThreadID != c.Channel.MainThreadID {
		return ids.ID{}, nil
	}
	_, taken, err := s.d.Store.ThreadByParentMessage(ctx, p.MessageID)
	if err != nil || taken {
		return ids.ID{}, err
	}
	return p.MessageID, nil
}

// importTags keeps the live tags of the forum c among want, in the order given, without
// duplicates and at most thread.max_tags of them (22 5.1.1); a text channel has none.
func (s *Service) importTags(ctx context.Context, c ChannelRow, want []ids.ID) ([]ids.ID, error) {
	if len(want) == 0 || c.Channel.Kind != domain.ChannelForum {
		return nil, nil
	}
	live, err := s.d.Store.ForumTags(ctx, c.Channel.ID)
	if err != nil {
		return nil, err
	}
	out := make([]ids.ID, 0, len(want))
	for _, id := range want {
		if len(out) == s.cfg.ThreadMaxTags {
			// Further tags are dropped, so the set stays editable within thread.max_tags.
			break
		}
		if slices.Contains(out, id) || !slices.ContainsFunc(live, func(t domain.ForumTag) bool { return t.ID == id }) {
			continue
		}
		out = append(out, id)
	}
	return out, nil
}

// MainThread implements chat.ImportThreads.
func (p ImportThreadsPort) MainThread(ctx context.Context, channelID ids.ID) (ids.ID, error) {
	s := p.s
	if err := inUnit(ctx, "ImportThreads.MainThread"); err != nil {
		return ids.ID{}, err
	}
	c, found, err := s.d.Store.Channel(ctx, channelID)
	if err != nil {
		return ids.ID{}, err
	}
	if !found || archived(c) || c.Channel.Kind == domain.ChannelDM {
		return ids.ID{}, invisible()
	}
	if c.Channel.Kind != domain.ChannelText || c.Channel.MainThreadID.IsZero() {
		return ids.ID{}, codeError("/thread_origin_key", chat.ReasonNoMainThread, "a forum channel has no main thread")
	}
	return c.Channel.MainThreadID, nil
}

// Lock implements chat.ImportThreads.
func (p ImportThreadsPort) Lock(ctx context.Context, threadID ids.ID) (chat.ImportedThreadState, error) {
	s := p.s
	if err := inUnit(ctx, "ImportThreads.Lock"); err != nil {
		return chat.ImportedThreadState{}, err
	}
	th, found, err := s.d.Store.LockThread(ctx, threadID)
	if err != nil {
		return chat.ImportedThreadState{}, err
	}
	if !found {
		return chat.ImportedThreadState{}, invisible()
	}
	return chat.ImportedThreadState{
		ID: th.ID, ChannelID: th.ChannelID, Title: th.Title, TitleSource: string(th.TitleSource), Closed: th.State == domain.ThreadClosed,
		ClosedAt: th.ClosedAt, LastActivityAt: th.LastActivityAt,
	}, nil
}

// ApplyMirror implements chat.ImportThreads (22 12.2 step 3): the title only while the stored
// source is auto (it stays auto), close or reopen (closed_at and closed_by cleared), and a later
// last_activity_at. The caller locked the thread with Lock in the same unit.
func (p ImportThreadsPort) ApplyMirror(ctx context.Context, threadID ids.ID, c chat.MirrorChange) error {
	s := p.s
	st, err := s.importThreads(ctx)
	if err != nil || c.Empty() {
		return err
	}
	th, found, err := s.d.Store.LockThread(ctx, threadID)
	if err != nil {
		return err
	}
	if !found {
		return invisible()
	}
	if c.Title != nil && th.TitleSource == domain.TitleAuto {
		th.Title = importTitle(*c.Title, s.cfg.ThreadTitleMaxChars)
		if err := domain.CheckThreadTitle(th.Title, s.cfg.ThreadTitleMaxChars); err != nil {
			return asNamePointer(err)
		}
	}
	switch {
	case c.CloseAt != nil && th.State == domain.ThreadOpen:
		th.State, th.ClosedAt, th.ClosedBy = domain.ThreadClosed, *c.CloseAt, ids.ID{}
	case c.Reopen && th.State == domain.ThreadClosed:
		th.State, th.ClosedAt, th.ClosedBy = domain.ThreadOpen, time.Time{}, ids.ID{}
	}
	if c.LastActivityAt != nil && c.LastActivityAt.After(th.LastActivityAt) {
		th.LastActivityAt = *c.LastActivityAt
	}
	return st.SaveImportedThread(ctx, th)
}

// Reserve implements chat.ImportThreads: the row lock, then n numbers (03 section 6.2) without a
// reopen, so an imported message never opens a closed thread.
func (p ImportThreadsPort) Reserve(ctx context.Context, threadID ids.ID, n int, lastActivity time.Time) (chat.Reserved, error) {
	s := p.s
	st, err := s.importThreads(ctx)
	if err != nil {
		return chat.Reserved{}, err
	}
	if n < 1 {
		return chat.Reserved{}, errs.New(errs.CodeInternalError, "chat: Reserve of no thread number")
	}
	th, found, err := s.d.Store.LockThread(ctx, threadID)
	if err != nil {
		return chat.Reserved{}, err
	}
	if !found {
		return chat.Reserved{}, invisible()
	}
	c, found, err := s.d.Store.Channel(ctx, th.ChannelID)
	if err != nil {
		return chat.Reserved{}, err
	}
	if !found {
		return chat.Reserved{}, errs.New(errs.CodeInternalError, "chat: the channel of a thread is missing")
	}
	if th, err = st.ReserveImportedSeqs(ctx, threadID, n, lastActivity); err != nil {
		return chat.Reserved{}, err
	}
	return chat.Reserved{FirstSeq: th.LastMessageSeq - int64(n) + 1, ThreadID: th.ID, ChannelID: th.ChannelID, SpaceID: c.Channel.SpaceID}, nil
}

// FinishBatch implements chat.ImportThreads (04 15.8, 22 6.4 items 2 and 3).
func (p ImportThreadsPort) FinishBatch(ctx context.Context, threadID ids.ID, changed bool) (int64, error) {
	s := p.s
	st, err := s.importThreads(ctx)
	if err != nil {
		return 0, err
	}
	if !changed {
		th, found, err := s.d.Store.Thread(ctx, threadID)
		if err != nil {
			return 0, err
		}
		if !found {
			return 0, invisible()
		}
		return th.UnreadFloorSeq, nil
	}
	th, err := st.FinishImportBatch(ctx, threadID)
	if err != nil {
		return 0, err
	}
	c, found, err := s.d.Store.Channel(ctx, th.ChannelID)
	if err != nil {
		return 0, err
	}
	if !found {
		return 0, errs.New(errs.CodeInternalError, "chat: the channel of a thread is missing")
	}
	v, err := s.threadView(ctx, th)
	if err != nil {
		return 0, err
	}
	return th.UnreadFloorSeq, s.emitThread(ctx, authorize.EventThreadUpdated, c.Channel.SpaceID, v)
}
