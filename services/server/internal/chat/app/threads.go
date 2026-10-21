// SPDX-License-Identifier: Apache-2.0

package app

import (
	"cmp"
	"context"
	"encoding/json"
	"slices"
	"time"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/chat/domain"
	"github.com/OWNER/bothy/services/server/internal/chat/domain/title"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// Compile-time check: the incoming thread port.
var (
	_ chat.Threads         = (*Service)(nil)
	_ chat.ThreadsByParent = (*Service)(nil)
)

// threadResource describes the thread th for authorize: its channel as path hint and its creator
// as owner (the "Ersteller" of thread.title.update).
func threadResource(th domain.Thread) authorize.Resource {
	return authorize.Resource{Kind: authorize.ResourceThread, ID: th.ID, ChannelID: th.ChannelID, OwnerID: th.CreatedBy}
}

// now is the clock time in the precision of the columns.
func (s *Service) now() time.Time { return s.d.Clock.Now().UTC().Truncate(time.Microsecond) }

// inUnit refuses a call outside a unit of work: Create and AppendToThread run in the caller's.
func inUnit(ctx context.Context, what string) error {
	if _, ok := txn.UnitFrom(ctx); !ok {
		return errs.New(errs.CodeInternalError, "chat: "+what+" must run inside the caller's unit of work")
	}
	return nil
}

// loadThread reads the thread id and authorizes act on it before it answers a missing thread, so
// an unknown and an invisible id give the same 404 (02 section 9.2).
func (s *Service) loadThread(ctx context.Context, a actor.Actor, act authorize.Action, id ids.ID) (domain.Thread, error) {
	th, found, err := s.d.Store.Thread(ctx, id)
	if err != nil {
		return domain.Thread{}, err
	}
	r := authorize.Resource{Kind: authorize.ResourceThread, ID: id}
	if found {
		r = threadResource(th)
	}
	if err := s.check(ctx, a, act, r); err != nil {
		return domain.Thread{}, err
	}
	if !found {
		return domain.Thread{}, invisible()
	}
	return th, nil
}

// lockThread locks the thread id inside the unit of work.
func (s *Service) lockThread(ctx context.Context, id ids.ID) (domain.Thread, error) {
	th, found, err := s.d.Store.LockThread(ctx, id)
	if err != nil {
		return domain.Thread{}, err
	}
	if !found {
		return domain.Thread{}, invisible()
	}
	return th, nil
}

// threadChannel returns the channel of th.
func (s *Service) threadChannel(ctx context.Context, th domain.Thread) (ChannelRow, error) {
	c, found, err := s.d.Store.Channel(ctx, th.ChannelID)
	if err != nil {
		return ChannelRow{}, err
	}
	if !found {
		return ChannelRow{}, errs.New(errs.CodeInternalError, "chat: a thread without its channel")
	}
	return c, nil
}

// views converts threads into the port DTO (brief D7): the live tags in their order and, for a
// fixed operator, its current billing mode from the agent policy source (one read per agent).
func (s *Service) views(ctx context.Context, list []domain.Thread) ([]chat.Thread, error) {
	idList := make([]ids.ID, 0, len(list))
	for _, th := range list {
		idList = append(idList, th.ID)
	}
	tags, err := s.d.Store.ThreadTags(ctx, idList)
	if err != nil {
		return nil, err
	}
	modes := map[ids.ID]string{}
	out := make([]chat.Thread, 0, len(list))
	for _, th := range list {
		v := threadDTO(th, tags[th.ID])
		if v.BillingMode, err = s.billingMode(ctx, th.OperatorAgentID, modes); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

// billingMode returns the current billing mode of the operator agent, "" for none or an unknown
// policy; modes caches the answers of one call.
func (s *Service) billingMode(ctx context.Context, agent ids.ID, modes map[ids.ID]string) (string, error) {
	if agent.IsZero() {
		return "", nil
	}
	if mode, ok := modes[agent]; ok {
		return mode, nil
	}
	p, found, err := s.d.Policies.AgentPolicy(ctx, agent)
	if err != nil {
		return "", err
	}
	mode := ""
	if found {
		mode = p.BillingMode
	}
	modes[agent] = mode
	return mode, nil
}

// threadView converts one thread like views.
func (s *Service) threadView(ctx context.Context, th domain.Thread) (chat.Thread, error) {
	vs, err := s.views(ctx, []domain.Thread{th})
	if err != nil {
		return chat.Thread{}, err
	}
	return vs[0], nil
}

// threadDTO converts a thread with its tags into the port DTO; the tags are never nil.
func threadDTO(th domain.Thread, tags []ids.ID) chat.Thread {
	if tags == nil {
		tags = []ids.ID{}
	}
	return chat.Thread{
		ID: th.ID, ChannelID: th.ChannelID, Kind: string(th.Kind), ParentMessageID: th.ParentMessageID, Title: th.Title,
		TitleSource: string(th.TitleSource), State: string(th.State), ClosedAt: th.ClosedAt, ClosedBy: th.ClosedBy,
		OperatorAgentID: th.OperatorAgentID, LastMessageSeq: th.LastMessageSeq, UnreadFloorSeq: th.UnreadFloorSeq,
		MessageCount: th.MessageCount, CreatedBy: th.CreatedBy, CreatedAt: th.CreatedAt, LastActivityAt: th.LastActivityAt,
		MovedAt: th.MovedAt, Version: th.Version, TagIDs: tags,
	}
}

// info converts a thread and its channel into the routing data.
func info(th domain.Thread, c ChannelRow) chat.ThreadInfo {
	return chat.ThreadInfo{
		ID: th.ID, ChannelID: th.ChannelID, SpaceID: c.Channel.SpaceID, ChannelKind: string(c.Channel.Kind), Kind: string(th.Kind),
		State: string(th.State), OperatorAgentID: th.OperatorAgentID, CreatedBy: th.CreatedBy, Version: th.Version,
		ChannelArchived: archived(c), LastMessageSeq: th.LastMessageSeq, UnreadFloorSeq: th.UnreadFloorSeq,
	}
}

// Info implements chat.Threads: no authorization; callers authorize first.
func (s *Service) Info(ctx context.Context, threadID ids.ID) (chat.ThreadInfo, error) {
	th, found, err := s.d.Store.Thread(ctx, threadID)
	if err != nil {
		return chat.ThreadInfo{}, err
	}
	if !found {
		return chat.ThreadInfo{}, invisible()
	}
	c, err := s.threadChannel(ctx, th)
	if err != nil {
		return chat.ThreadInfo{}, err
	}
	return info(th, c), nil
}

// ThreadByParentMessage implements chat.ThreadsByParent: the routing data of the thread that
// started from messageID, without authorization; callers authorize first.
func (s *Service) ThreadByParentMessage(ctx context.Context, messageID ids.ID) (chat.ThreadInfo, bool, error) {
	th, found, err := s.d.Store.ThreadByParentMessage(ctx, messageID)
	if err != nil || !found {
		return chat.ThreadInfo{}, false, err
	}
	c, err := s.threadChannel(ctx, th)
	if err != nil {
		return chat.ThreadInfo{}, false, err
	}
	return info(th, c), true, nil
}

// InfoForShare implements chat.Threads: Info under FOR SHARE on the thread row (M1-61), without
// authorization; callers authorize first.
func (s *Service) InfoForShare(ctx context.Context, threadID ids.ID) (chat.ThreadInfo, error) {
	th, found, err := s.d.Store.ShareThread(ctx, threadID)
	if err != nil {
		return chat.ThreadInfo{}, err
	}
	if !found {
		return chat.ThreadInfo{}, invisible()
	}
	c, err := s.threadChannel(ctx, th)
	if err != nil {
		return chat.ThreadInfo{}, err
	}
	return info(th, c), nil
}

// Locate implements chat.Threads: the routing data of the known threads among threadIDs in one
// store read, without authorization (M2-05); no thread returns none without a query.
func (s *Service) Locate(ctx context.Context, threadIDs []ids.ID) ([]chat.ThreadInfo, error) {
	if len(threadIDs) == 0 {
		return []chat.ThreadInfo{}, nil
	}
	return s.d.Store.LocateThreads(ctx, threadIDs)
}

// Counters implements chat.Threads: the counters of the threads of channelIDs with something
// unread for somebody, without authorization (M1-63); no channel returns none without a query.
func (s *Service) Counters(ctx context.Context, channelIDs []ids.ID) ([]chat.ThreadCounter, error) {
	if len(channelIDs) == 0 {
		return []chat.ThreadCounter{}, nil
	}
	return s.d.Store.ThreadCounters(ctx, channelIDs)
}

// Get implements chat.Threads: thread.read, then 404 for an unknown id.
func (s *Service) Get(ctx context.Context, a actor.Actor, threadID ids.ID) (chat.Thread, error) {
	th, err := s.loadThread(ctx, a, authorize.ActionThreadRead, threadID)
	if err != nil {
		return chat.Thread{}, err
	}
	return s.threadView(ctx, th)
}

// View implements chat.Threads: the thread without authorization; the caller authorized the
// write it answers for.
func (s *Service) View(ctx context.Context, threadID ids.ID) (chat.Thread, error) {
	th, found, err := s.d.Store.Thread(ctx, threadID)
	if err != nil {
		return chat.Thread{}, err
	}
	if !found {
		return chat.Thread{}, invisible()
	}
	return s.threadView(ctx, th)
}

// threadUnit runs fn for the thread id in one unit of work: act on the thread (an invisible
// thread is 404), the row lock, then fn, which changes the locked thread and reports whether it
// did. A change is saved and appends the event typ (brief D9); no change appends nothing (06
// section 2 "Ohne Wirkung").
func (s *Service) threadUnit(ctx context.Context, a actor.Actor, act authorize.Action, id ids.ID, typ authorize.EventType,
	fn func(ctx context.Context, th *domain.Thread, c ChannelRow) (bool, error)) (chat.Thread, error) {
	ctx, err := withCaller(ctx, a)
	if err != nil {
		return chat.Thread{}, err
	}
	var out chat.Thread
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		out, err = s.threadChange(ctx, a, act, id, typ, fn)
		return err
	})
	return out, err
}

// threadChange is the body of threadUnit inside its unit of work.
func (s *Service) threadChange(ctx context.Context, a actor.Actor, act authorize.Action, id ids.ID, typ authorize.EventType,
	fn func(ctx context.Context, th *domain.Thread, c ChannelRow) (bool, error)) (chat.Thread, error) {
	if _, err := s.loadThread(ctx, a, act, id); err != nil {
		return chat.Thread{}, err
	}
	th, err := s.lockThread(ctx, id)
	if err != nil {
		return chat.Thread{}, err
	}
	c, err := s.threadChannel(ctx, th)
	if err != nil {
		return chat.Thread{}, err
	}
	changed, err := fn(ctx, &th, c)
	if err != nil {
		return chat.Thread{}, err
	}
	if changed {
		if err := s.d.Store.SaveThread(ctx, th); err != nil {
			return chat.Thread{}, err
		}
	}
	out, err := s.threadView(ctx, th)
	if err != nil || !changed {
		return out, err
	}
	return out, s.emitThread(ctx, typ, c.Channel.SpaceID, out)
}

// Close implements chat.Threads (brief D3): thread.close; a main thread is 409
// invalid_state_transition, a closed thread answers without change.
func (s *Service) Close(ctx context.Context, a actor.Actor, threadID ids.ID) (chat.Thread, error) {
	return s.threadUnit(ctx, a, authorize.ActionThreadClose, threadID, authorize.EventThreadClosed,
		func(_ context.Context, th *domain.Thread, _ ChannelRow) (bool, error) {
			return th.Close(a.ID, s.now())
		})
}

// Reopen implements chat.Threads (brief D3): thread.reopen; an open thread answers without change.
func (s *Service) Reopen(ctx context.Context, a actor.Actor, threadID ids.ID) (chat.Thread, error) {
	return s.threadUnit(ctx, a, authorize.ActionThreadReopen, threadID, authorize.EventThreadReopened,
		func(_ context.Context, th *domain.Thread, _ ChannelRow) (bool, error) {
			return th.Reopen(), nil
		})
}

// SetTitle implements chat.Threads (brief D4): thread.title.update decides who may call; If-Match
// is required; a main thread has no title. Then package title decides the source on the locked
// row (M2-18): the thread's operator agent keeps auto and gets 409 title_manual on a manual title
// (09 section 12, AG-19); every other principal sets manual (05 thread.title.update).
func (s *Service) SetTitle(ctx context.Context, a actor.Actor, cmd chat.SetTitleCmd) (chat.Thread, error) {
	return s.threadUnit(ctx, a, authorize.ActionThreadTitleUpdate, cmd.ThreadID, authorize.EventThreadUpdated,
		func(ctx context.Context, th *domain.Thread, _ ChannelRow) (bool, error) {
			if err := s.threadPrecondition(ctx, cmd.IfMatch, *th, true); err != nil {
				return false, err
			}
			if th.Main() {
				return false, codeError("/title", domain.FieldMainThreadHasNoTitle, "a main thread has no title")
			}
			if err := domain.CheckThreadTitle(cmd.Title, s.cfg.ThreadTitleMaxChars); err != nil {
				return false, err
			}
			source, err := title.SourceOnUpdate(titleBy(a, *th), title.Source(th.TitleSource))
			if err != nil {
				return false, err
			}
			return th.SetTitle(cmd.Title, domain.TitleSource(source)), nil
		})
}

// titleBy classifies the principal a for the title rules of package title (brief M2-18 note 6a): a
// person is ByPerson; the agent that is the operator of th is ByOperatorAgent; every other agent
// or app is ByOther. kernel/actor is a port the domain may not import, so the class is decided here.
func titleBy(a actor.Actor, th domain.Thread) title.By {
	switch {
	case a.Kind == actor.KindUser:
		return title.ByPerson
	case a.Kind == actor.KindAgent && !th.OperatorAgentID.IsZero() && a.ID == th.OperatorAgentID:
		return title.ByOperatorAgent
	}
	return title.ByOther
}

// threadPrecondition checks If-Match against the locked thread (06 section 8): without the header
// it is 428 when required and passes when optional (point 3), a stale or malformed one is 412
// version_conflict with the current thread.
func (s *Service) threadPrecondition(ctx context.Context, p chat.Precondition, th domain.Thread, required bool) error {
	switch {
	case !p.Present && required:
		return errs.PreconditionRequired()
	case !p.Present || (p.Valid && p.Version == th.Version):
		return nil
	}
	v, err := s.threadView(ctx, th)
	if err != nil {
		return err
	}
	return errs.VersionConflict(threadBody(v))
}

// SetTags implements chat.Threads (brief D5): thread.tags.set, optional If-Match, only a forum
// post, the set rules, then the difference to the current set; when an added or removed tag is
// restricted, authorize decides again with the restricted flag (manage). The same set answers
// without change; otherwise the rows are replaced and the version rises once.
func (s *Service) SetTags(ctx context.Context, a actor.Actor, cmd chat.SetTagsCmd) (chat.Thread, error) {
	return s.threadUnit(ctx, a, authorize.ActionThreadTagsSet, cmd.ThreadID, authorize.EventThreadUpdated,
		func(ctx context.Context, th *domain.Thread, c ChannelRow) (bool, error) {
			return s.setTags(ctx, a, cmd, th, c)
		})
}

// setTags is the change of SetTags on the locked thread th of channel c.
func (s *Service) setTags(ctx context.Context, a actor.Actor, cmd chat.SetTagsCmd, th *domain.Thread, c ChannelRow) (bool, error) {
	if err := s.threadPrecondition(ctx, cmd.IfMatch, *th, false); err != nil {
		return false, err
	}
	if th.Main() || c.Channel.Kind != domain.ChannelForum {
		return false, codeError("/tag_ids", domain.FieldNotAForumPost, "the thread is not a forum post")
	}
	restricted, err := s.checkTags(ctx, th.ChannelID, cmd.TagIDs)
	if err != nil {
		return false, err
	}
	current, err := s.d.Store.ThreadTags(ctx, []ids.ID{th.ID})
	if err != nil {
		return false, err
	}
	diff := domain.DiffTags(current[th.ID], cmd.TagIDs, func(id ids.ID) bool { return restricted[id] })
	if diff.Restricted {
		r := threadResource(*th)
		r.RestrictedTags = true
		if err := s.check(ctx, a, authorize.ActionThreadTagsSet, r); err != nil {
			return false, err
		}
	}
	if !diff.Changed() {
		return false, nil
	}
	if err := s.d.Store.ReplaceThreadTags(ctx, th.ID, cmd.TagIDs, a.ID, s.now()); err != nil {
		return false, err
	}
	th.Version++
	return true, nil
}

// checkTags checks a tag set for a thread of the forum channel (brief D5): no duplicate, at most
// thread.max_tags, every tag a live tag of the forum. It returns the restricted flag of each tag.
func (s *Service) checkTags(ctx context.Context, channel ids.ID, set []ids.ID) (map[ids.ID]bool, error) {
	if err := domain.CheckTagSet(set, s.cfg.ThreadMaxTags); err != nil {
		return nil, err
	}
	tags, err := s.d.Store.ForumTags(ctx, channel)
	if err != nil {
		return nil, err
	}
	restricted := make(map[ids.ID]bool, len(tags))
	for _, t := range tags {
		restricted[t.ID] = t.Restricted
	}
	for _, id := range set {
		if _, ok := restricted[id]; !ok {
			return nil, codeError("/tag_ids", domain.FieldUnknownTag, "a tag is not a tag of the forum")
		}
	}
	return restricted, nil
}

// Create implements chat.Threads (brief D1) inside the caller's unit of work.
func (s *Service) Create(ctx context.Context, a actor.Actor, cmd chat.CreateThreadCmd) (chat.Thread, error) {
	if err := inUnit(ctx, "Threads.Create"); err != nil {
		return chat.Thread{}, err
	}
	ctx, err := withCaller(ctx, a)
	if err != nil {
		return chat.Thread{}, err
	}
	c, found, err := s.d.Store.Channel(ctx, cmd.ChannelID)
	if err != nil {
		return chat.Thread{}, err
	}
	if !found {
		return chat.Thread{}, invisible()
	}
	th, err := s.newThread(a, c, cmd)
	if err != nil {
		return chat.Thread{}, err
	}
	if len(cmd.TagIDs) > 0 {
		if err := s.createTags(ctx, a, c, cmd.TagIDs); err != nil {
			return chat.Thread{}, err
		}
	}
	if th.SearchLanguage, err = s.searchLanguage(ctx, a); err != nil {
		return chat.Thread{}, err
	}
	if th, err = s.d.Store.InsertThread(ctx, th); err != nil {
		return chat.Thread{}, err
	}
	if len(cmd.TagIDs) > 0 {
		if err := s.d.Store.ReplaceThreadTags(ctx, th.ID, cmd.TagIDs, a.ID, th.CreatedAt); err != nil {
			return chat.Thread{}, err
		}
	}
	v, err := s.threadView(ctx, th)
	if err != nil {
		return chat.Thread{}, err
	}
	return v, s.emitThread(ctx, authorize.EventThreadCreated, c.Channel.SpaceID, v)
}

// searchLanguage returns the text search configuration of a thread that a creates
// (docs/spec/14-suche.md 3.1, docs/DECISIONS.md D-M3-13-1 and D-M3-13-2): the language of the
// person through identity, or the instance language for an agent or an app, without reading the
// principal. It reads inside the caller's unit of work; the value is set once at creation.
func (s *Service) searchLanguage(ctx context.Context, a actor.Actor) (string, error) {
	if a.Kind != actor.KindUser {
		return s.d.Directory.InstanceSearchLanguage(ctx)
	}
	return s.d.Directory.SearchLanguage(ctx, a.ID)
}

// newThread checks parent, tags and title of cmd for the channel c and returns the new thread
// with the effective operator of the channel now (R-DM-12).
func (s *Service) newThread(a actor.Actor, c ChannelRow, cmd chat.CreateThreadCmd) (domain.Thread, error) {
	th := domain.Thread{
		ID: ids.New(), ChannelID: c.Channel.ID, Kind: domain.ThreadKindThread, State: domain.ThreadOpen,
		OperatorAgentID: c.Channel.Operator(c.SpaceDefaultOperatorID).EffectiveAgentID, CreatedBy: a.ID, Version: 1,
	}
	if c.Channel.Kind == domain.ChannelForum {
		if cmd.Parent != nil {
			return domain.Thread{}, codeError("/parent_message_id", domain.FieldNotAllowedInForum, "a forum post has no parent message")
		}
	} else {
		switch {
		case cmd.Parent == nil:
			return domain.Thread{}, codeError("/parent_message_id", domain.FieldRequiredInTextChannel, "a thread of a text channel needs a parent message")
		case cmd.Parent.ThreadID.IsZero() || cmd.Parent.ThreadID != c.Channel.MainThreadID:
			return domain.Thread{}, codeError("/parent_message_id", domain.FieldNotInMainThread, "the parent message is not in the main thread of the channel")
		case len(cmd.TagIDs) > 0:
			return domain.Thread{}, codeError("/tag_ids", domain.FieldTagsOnlyInForum, "tags apply to forum posts only")
		}
		th.ParentMessageID = cmd.Parent.MessageID
	}
	th.Title = deref(cmd.Title)
	if err := domain.CheckThreadTitle(th.Title, s.cfg.ThreadTitleMaxChars); err != nil {
		return domain.Thread{}, err
	}
	th.TitleSource = domain.TitleSource(s.placeholders.SourceAtCreation(titleBy(a, th), th.Title))
	th.CreatedAt = s.now()
	th.LastActivityAt = th.CreatedAt
	return th, nil
}

// createTags checks the tags of a new forum post like SetTags; a restricted tag needs manage,
// decided by thread.tags.set with the restricted flag on the channel path of the new thread (05
// thread.create).
func (s *Service) createTags(ctx context.Context, a actor.Actor, c ChannelRow, set []ids.ID) error {
	restricted, err := s.checkTags(ctx, c.Channel.ID, set)
	if err != nil {
		return err
	}
	if !slices.ContainsFunc(set, func(id ids.ID) bool { return restricted[id] }) {
		return nil
	}
	return s.check(ctx, a, authorize.ActionThreadTagsSet, authorize.Resource{
		Kind: authorize.ResourceThread, ChannelID: c.Channel.ID, SpaceID: c.Channel.SpaceID, OwnerID: a.ID, RestrictedTags: true,
	})
}

// AppendToThread implements chat.Threads (brief D2) inside the caller's unit of work: the row
// lock, on an implicit reopen thread.reopen for a, then the single UPDATE of 03 section 6.2 and,
// on a reopen, thread.reopened.
func (s *Service) AppendToThread(ctx context.Context, a actor.Actor, cmd chat.AppendCmd) (chat.Appended, error) {
	if err := inUnit(ctx, "Threads.AppendToThread"); err != nil {
		return chat.Appended{}, err
	}
	ctx, err := withCaller(ctx, a)
	if err != nil {
		return chat.Appended{}, err
	}
	th, err := s.lockThread(ctx, cmd.ThreadID)
	if err != nil {
		return chat.Appended{}, err
	}
	reopen := cmd.ReopenIfClosed && th.State == domain.ThreadClosed
	if reopen {
		if err := s.check(ctx, a, authorize.ActionThreadReopen, threadResource(th)); err != nil {
			return chat.Appended{}, err
		}
	}
	if th, err = s.d.Store.AppendToThread(ctx, th.ID, reopen, s.now()); err != nil {
		return chat.Appended{}, err
	}
	c, err := s.threadChannel(ctx, th)
	if err != nil {
		return chat.Appended{}, err
	}
	out := chat.Appended{Seq: th.LastMessageSeq, Reopened: reopen, Thread: info(th, c)}
	if !reopen {
		return out, nil
	}
	v, err := s.threadView(ctx, th)
	if err != nil {
		return chat.Appended{}, err
	}
	return out, s.emitThread(ctx, authorize.EventThreadReopened, c.Channel.SpaceID, v)
}

// LockForMessageChange implements chat.Threads (M1-58) inside the caller's unit of work: the row
// lock of the thread, taken before the message row (lock order of the messaging README), so a
// deletion and a thread move (M1-62) of the same thread never deadlock. It does not authorize.
func (s *Service) LockForMessageChange(ctx context.Context, threadID ids.ID) error {
	if err := inUnit(ctx, "Threads.LockForMessageChange"); err != nil {
		return err
	}
	_, err := s.lockThread(ctx, threadID)
	return err
}

// DecrementMessageCount implements chat.Threads (M1-58) inside the caller's unit of work, which
// holds the thread lock: message_count - 1 and version + 1 (03 threads). Errors: internal for an
// unknown thread or a count of zero.
func (s *Service) DecrementMessageCount(ctx context.Context, threadID ids.ID) error {
	if err := inUnit(ctx, "Threads.DecrementMessageCount"); err != nil {
		return err
	}
	return s.d.Store.DecrementMessageCount(ctx, threadID)
}

// threadsList is the list name of the cursors of listChannelThreads.
const threadsList = "threads"

// Sort orders of listChannelThreads (schema ForumSort).
const (
	sortLastActivity = "last_activity"
	sortCreated      = "created"
)

// threadsCursor is the keyset position of a page: the sort time and the id of its last item.
type threadsCursor struct {
	At time.Time `json:"at"`
	ID ids.ID    `json:"id"`
}

// List implements chat.Threads (brief D6, 07 section 7.4 rule 3): thread.read on the channel as
// container (an invisible channel is 404), then the threads of kind thread in keyset order, each
// filtered with authorize in filter mode, until one more than the page size is visible or the
// channel ends. The cursor is bound to caller, channel, filters and sort; there are no totals. A
// tag of another channel or an unknown one gives an empty page (docs/DECISIONS.md M1-55/1).
func (s *Service) List(ctx context.Context, a actor.Actor, q chat.ListThreadsQuery) (chat.ThreadPage, error) {
	c, found, err := s.d.Store.Channel(ctx, q.ChannelID)
	if err != nil {
		return chat.ThreadPage{}, err
	}
	if err := s.check(ctx, a, authorize.ActionThreadRead, authorize.Resource{Kind: authorize.ResourceThread, ChannelID: q.ChannelID}); err != nil {
		return chat.ThreadPage{}, err
	}
	if !found {
		return chat.ThreadPage{}, invisible()
	}
	tq, binding, err := s.threadQuery(a, q, c.Channel.ForumDefaultSort)
	if err != nil {
		return chat.ThreadPage{}, err
	}
	limit := q.Limit
	if limit < 1 {
		limit = s.cfg.PageSizeDefault
	}
	limit = min(limit, s.cfg.PageSizeMax)
	visible, err := s.visibleThreads(authorize.Filter(ctx), a, tq, limit+1)
	if err != nil {
		return chat.ThreadPage{}, err
	}
	more := len(visible) > limit
	visible = visible[:min(len(visible), limit)]
	items, err := s.views(ctx, visible)
	if err != nil {
		return chat.ThreadPage{}, err
	}
	out := chat.ThreadPage{Items: items}
	if !more {
		return out, nil
	}
	last := visible[len(visible)-1]
	if out.NextCursor, err = s.d.Cursors.Encode(threadsList, binding, threadsCursor{At: sortTime(last, tq.ByCreated), ID: last.ID}); err != nil {
		return chat.ThreadPage{}, errs.Wrap(errs.CodeInternalError, err, "chat: thread list cursor")
	}
	return out, nil
}

// threadQuery returns the store query and the cursor binding of q: the sort (q.Sort, else the
// forum default sort, else last_activity) and the position of the cursor. Errors: cursor_invalid.
func (s *Service) threadQuery(a actor.Actor, q chat.ListThreadsQuery, forumSort string) (ThreadQuery, []byte, error) {
	sort := cmp.Or(q.Sort, forumSort, sortLastActivity)
	tq := ThreadQuery{ChannelID: q.ChannelID, State: q.State, TagID: q.TagID, ByCreated: sort == sortCreated}
	binding := []byte(a.ID.String() + "\x00" + q.ChannelID.String() + "\x00" + q.State + "\x00" + optionalID(q.TagID) + "\x00" + sort)
	if q.Cursor == "" {
		return tq, binding, nil
	}
	var k threadsCursor
	if err := s.d.Cursors.Decode(threadsList, binding, q.Cursor, &k); err != nil {
		return ThreadQuery{}, nil, err
	}
	if k.ID.IsZero() || k.At.IsZero() {
		return ThreadQuery{}, nil, errs.New(errs.CodeCursorInvalid, "")
	}
	tq.HasAfter, tq.AfterAt, tq.AfterID = true, k.At, k.ID
	return tq, binding, nil
}

// sortTime is the time of th in the sort of the list.
func sortTime(th domain.Thread, byCreated bool) time.Time {
	if byCreated {
		return th.CreatedAt
	}
	return th.LastActivityAt
}

// visibleThreads reads the threads of tq in batches of want and keeps those a may read, until
// want are visible or the channel ends.
func (s *Service) visibleThreads(ctx context.Context, a actor.Actor, tq ThreadQuery, want int) ([]domain.Thread, error) {
	var out []domain.Thread
	tq.Limit = want
	for len(out) < want {
		batch, err := s.d.Store.ListThreads(ctx, tq)
		if err != nil {
			return nil, err
		}
		if out, err = s.readableThreads(ctx, a, batch, out, want); err != nil {
			return nil, err
		}
		if len(batch) < want {
			break
		}
		last := batch[len(batch)-1]
		tq.HasAfter, tq.AfterAt, tq.AfterID = true, sortTime(last, tq.ByCreated), last.ID
	}
	return out, nil
}

// readableThreads appends to out the threads of batch that a may read, up to want in total.
func (s *Service) readableThreads(ctx context.Context, a actor.Actor, batch, out []domain.Thread, want int) ([]domain.Thread, error) {
	for _, th := range batch {
		if len(out) == want {
			break
		}
		d, err := s.d.Authorizer.Authorize(ctx, a, authorize.ActionThreadRead, threadResource(th))
		if err != nil {
			return nil, err
		}
		if d.Allowed() {
			out = append(out, th)
		}
	}
	return out, nil
}

// threadJSON is the schema Thread: the payload member thread of the thread events and the member
// current of 412 version_conflict. Optional members are left out, never null (ADR 0053).
type threadJSON struct {
	ID              string   `json:"id"`
	ChannelID       string   `json:"channel_id"`
	Kind            string   `json:"kind"`
	ParentMessageID string   `json:"parent_message_id,omitempty"`
	Title           string   `json:"title"`
	TitleSource     string   `json:"title_source"`
	State           string   `json:"state"`
	ClosedAt        string   `json:"closed_at,omitempty"`
	ClosedBy        string   `json:"closed_by,omitempty"`
	OperatorAgentID string   `json:"operator_agent_id,omitempty"`
	BillingMode     string   `json:"billing_mode,omitempty"`
	LastMessageSeq  int64    `json:"last_message_seq"`
	UnreadFloorSeq  int64    `json:"unread_floor_seq,omitempty"`
	LastActivityAt  string   `json:"last_activity_at"`
	MessageCount    int      `json:"message_count"`
	CreatedBy       string   `json:"created_by"`
	CreatedAt       string   `json:"created_at"`
	MovedAt         string   `json:"moved_at,omitempty"`
	Version         int64    `json:"version"`
	TagIDs          []string `json:"tag_ids"`
}

// threadBody converts a thread into the JSON form of the schema Thread.
func threadBody(v chat.Thread) threadJSON {
	tags := make([]string, 0, len(v.TagIDs))
	for _, t := range v.TagIDs {
		tags = append(tags, t.String())
	}
	return threadJSON{
		ID: v.ID.String(), ChannelID: v.ChannelID.String(), Kind: v.Kind, ParentMessageID: optionalID(v.ParentMessageID),
		Title: v.Title, TitleSource: v.TitleSource, State: v.State, ClosedAt: stamp(v.ClosedAt), ClosedBy: optionalID(v.ClosedBy),
		OperatorAgentID: optionalID(v.OperatorAgentID), BillingMode: v.BillingMode, LastMessageSeq: v.LastMessageSeq,
		UnreadFloorSeq: v.UnreadFloorSeq, LastActivityAt: stamp(v.LastActivityAt), MessageCount: v.MessageCount,
		CreatedBy: v.CreatedBy.String(), CreatedAt: stamp(v.CreatedAt), MovedAt: stamp(v.MovedAt), Version: v.Version, TagIDs: tags,
	}
}

// emitThread appends the thread event typ for v in space (05 section 6, brief D9): resource the
// thread with its version, scope space, channel and thread, payload ThreadPayload, receive action
// thread.read.
func (s *Service) emitThread(ctx context.Context, typ authorize.EventType, space ids.ID, v chat.Thread) error {
	payload, err := json.Marshal(struct {
		Thread threadJSON `json:"thread"`
	}{threadBody(v)})
	if err != nil {
		return errs.Wrap(errs.CodeInternalError, err, "chat: thread event payload")
	}
	kind, actorID := eventActor(ctx)
	return s.d.Events.Append(ctx, eventlog.Event{
		Type: string(typ), SchemaVersion: 1, ActorKind: kind, ActorPrincipalID: actorID, SpaceID: space,
		ChannelID: v.ChannelID, ThreadID: v.ID, ResourceKind: resourceThread, ResourceID: v.ID, ResourceVersion: v.Version,
		ReceiveAction: receiveThreadRead, Payload: payload,
	})
}
