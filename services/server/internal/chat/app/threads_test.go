// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// forbidden is the refusal of the fake authorizer for a missing capability.
var forbidden = authorize.Decision{Outcome: authorize.OutcomeForbidden, Reason: "capability_missing"}

// newThread runs Threads.Create as a inside a unit of work of the fakes.
func (e *env) newThread(a actor.Actor, cmd chat.CreateThreadCmd) (chat.Thread, error) {
	var out chat.Thread
	err := e.uow.Do(context.Background(), func(ctx context.Context) error {
		var err error
		out, err = e.svc.Create(ctx, a, cmd)
		return err
	})
	return out, err
}

// appendTo runs Threads.AppendToThread as the owner inside a unit of work of the fakes.
func (e *env) appendTo(id ids.ID, reopen bool) (chat.Appended, error) {
	var out chat.Appended
	err := e.uow.Do(context.Background(), func(ctx context.Context) error {
		var err error
		out, err = e.svc.AppendToThread(ctx, e.owner, chat.AppendCmd{ThreadID: id, ReopenIfClosed: reopen})
		return err
	})
	return out, err
}

// eventCount returns the appended events of typ.
func (e *env) eventCount(typ string) int {
	e.events.mu.Lock()
	defer e.events.mu.Unlock()
	n := 0
	for _, ev := range e.events.evs {
		if ev.Type == typ {
			n++
		}
	}
	return n
}

// TestThreadUseCases runs the thread use cases over the fakes: create in a forum and a text
// channel, append with and without reopen, info, get, close, reopen, title and tags with their
// errors and events (the same rules over PostgreSQL in adapters/httpapi).
func TestThreadUseCases(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ctx := context.Background()
	sp := e.create(t, "Support", ids.ID{})
	forum, text := e.channel(t, sp.ID, "forum", "forum"), e.channel(t, sp.ID, "text", "text")
	bug, err := e.svc.CreateForumTag(ctx, e.owner, forum.ID, chat.CreateForumTagCmd{Name: "bug"})
	if err != nil {
		t.Fatal(err)
	}
	staff, err := e.svc.CreateForumTag(ctx, e.owner, forum.ID, chat.CreateForumTagCmd{Name: "staff", Restricted: boolPtr(true)})
	if err != nil {
		t.Fatal(err)
	}
	post, err := e.newThread(e.owner, chat.CreateThreadCmd{ChannelID: forum.ID, Title: ptr("Crash"), TagIDs: []ids.ID{bug.ID, staff.ID}})
	if err != nil || post.TitleSource != "manual" || !slices.Equal(post.TagIDs, []ids.ID{bug.ID, staff.ID}) || !post.OperatorAgentID.IsZero() {
		t.Fatalf("post %+v, %v", post, err)
	}
	if _, err := e.svc.Create(ctx, e.owner, chat.CreateThreadCmd{ChannelID: forum.ID}); err == nil {
		t.Fatal("create outside a unit")
	}
	_, err = e.newThread(e.owner, chat.CreateThreadCmd{ChannelID: ids.New()})
	wantCode(t, err, errs.CodeNotFound)
	_, err = e.newThread(e.owner, chat.CreateThreadCmd{ChannelID: forum.ID, TagIDs: []ids.ID{ids.New()}})
	wantCode(t, err, errs.CodeValidationFailed)
	_, err = e.newThread(e.owner, chat.CreateThreadCmd{ChannelID: forum.ID, Title: ptr("a\u0007")})
	wantCode(t, err, errs.CodeValidationFailed)
	_, err = e.newThread(e.owner, chat.CreateThreadCmd{ChannelID: forum.ID, Parent: &chat.ParentMessage{MessageID: ids.New()}})
	wantCode(t, err, errs.CodeValidationFailed)
	e.deny(authorize.ActionThreadTagsSet, forbidden)
	_, err = e.newThread(e.owner, chat.CreateThreadCmd{ChannelID: forum.ID, TagIDs: []ids.ID{staff.ID}})
	wantCode(t, err, errs.CodeForbidden)
	e.az.rules[authorize.ActionThreadTagsSet] = nil

	msg := ids.New()
	th, err := e.newThread(e.owner, chat.CreateThreadCmd{ChannelID: text.ID, Parent: &chat.ParentMessage{MessageID: msg, ThreadID: text.MainThreadID}})
	if err != nil || th.ParentMessageID != msg || th.TitleSource != "auto" {
		t.Fatalf("thread %+v, %v", th, err)
	}
	// The routing data carry the parent message (M2-13 design note 2).
	if info, err := e.svc.Info(context.Background(), th.ID); err != nil || info.ParentMessageID != msg {
		t.Fatalf("info %+v, %v", info, err)
	}
	_, err = e.newThread(e.owner, chat.CreateThreadCmd{ChannelID: text.ID, Parent: &chat.ParentMessage{MessageID: msg, ThreadID: text.MainThreadID}})
	wantCode(t, err, errs.CodeConflict)
	for _, cmd := range []chat.CreateThreadCmd{
		{ChannelID: text.ID},
		{ChannelID: text.ID, Parent: &chat.ParentMessage{MessageID: ids.New(), ThreadID: ids.New()}},
		{ChannelID: text.ID, Parent: &chat.ParentMessage{MessageID: ids.New(), ThreadID: text.MainThreadID}, TagIDs: []ids.ID{bug.ID}},
	} {
		_, err = e.newThread(e.owner, cmd)
		wantCode(t, err, errs.CodeValidationFailed)
	}

	// Append, close and the implicit reopen.
	if got, err := e.appendTo(post.ID, false); err != nil || got.Seq != 1 || got.Reopened || got.Thread.SpaceID != sp.ID {
		t.Fatalf("append %+v, %v", got, err)
	}
	if _, err := e.svc.AppendToThread(ctx, e.owner, chat.AppendCmd{ThreadID: post.ID}); err == nil {
		t.Fatal("append outside a unit")
	}
	_, err = e.appendTo(ids.New(), false)
	wantCode(t, err, errs.CodeNotFound)
	closed, err := e.svc.Close(ctx, e.owner, post.ID)
	if err != nil || closed.State != "closed" || e.eventCount("thread.closed") != 1 {
		t.Fatalf("close %+v, %v", closed, err)
	}
	if again, err := e.svc.Close(ctx, e.owner, post.ID); err != nil || again.Version != closed.Version || e.eventCount("thread.closed") != 1 {
		t.Fatalf("close again %+v, %v", again, err)
	}
	e.deny(authorize.ActionThreadReopen, forbidden)
	_, err = e.appendTo(post.ID, true)
	wantCode(t, err, errs.CodeForbidden)
	_, err = e.svc.Reopen(ctx, e.owner, post.ID)
	wantCode(t, err, errs.CodeForbidden)
	e.az.rules[authorize.ActionThreadReopen] = nil
	if got, err := e.appendTo(post.ID, true); err != nil || !got.Reopened || got.Thread.State != "open" || e.eventCount("thread.reopened") != 1 {
		t.Fatalf("reopen by append %+v, %v", got, err)
	}
	if got, err := e.svc.Reopen(ctx, e.owner, post.ID); err != nil || got.State != "open" || e.eventCount("thread.reopened") != 1 {
		t.Fatalf("reopen open %+v, %v", got, err)
	}
	_, err = e.svc.Close(ctx, e.owner, text.MainThreadID)
	wantCode(t, err, errs.CodeInvalidStateTransition)

	// Info and Get.
	info, err := e.svc.Info(ctx, post.ID)
	if err != nil || info.ChannelKind != "forum" || info.Kind != "thread" || info.CreatedBy != e.owner.ID {
		t.Fatalf("info %+v, %v", info, err)
	}
	_, err = e.svc.Info(ctx, ids.New())
	wantCode(t, err, errs.CodeNotFound)
	// InfoForShare (M1-61): the same routing data with the counters of the thread.
	stored, _, _ := e.store.Thread(ctx, post.ID)
	shared, err := e.svc.InfoForShare(ctx, post.ID)
	if err != nil || shared.ChannelID != info.ChannelID || shared.ChannelKind != "forum" || shared.LastMessageSeq != stored.LastMessageSeq ||
		shared.LastMessageSeq == 0 || shared.UnreadFloorSeq != stored.UnreadFloorSeq {
		t.Fatalf("info for share %+v, %v", shared, err)
	}
	_, err = e.svc.InfoForShare(ctx, ids.New())
	wantCode(t, err, errs.CodeNotFound)
	if got, err := e.svc.Get(ctx, e.owner, post.ID); err != nil || got.ID != post.ID {
		t.Fatalf("get %+v, %v", got, err)
	}
	_, err = e.svc.Get(ctx, e.owner, ids.New())
	wantCode(t, err, errs.CodeNotFound)
	if got, err := e.svc.View(ctx, post.ID); err != nil || got.ID != post.ID {
		t.Fatalf("view %+v, %v", got, err)
	}
	_, err = e.svc.View(ctx, ids.New())
	wantCode(t, err, errs.CodeNotFound)

	// Title.
	cur, _ := e.svc.Get(ctx, e.owner, post.ID)
	_, err = e.svc.SetTitle(ctx, e.owner, chat.SetTitleCmd{ThreadID: post.ID, Title: "x"})
	wantCode(t, err, errs.CodePreconditionRequired)
	_, err = e.svc.SetTitle(ctx, e.owner, chat.SetTitleCmd{ThreadID: post.ID, Title: "x", IfMatch: chat.Precondition{Present: true, Valid: true, Version: 99}})
	wantCode(t, err, errs.CodeVersionConflict)
	match := chat.Precondition{Present: true, Valid: true, Version: cur.Version}
	// The agent is the operator of both threads: only the operator agent keeps the source and
	// gets title_manual (M2-18; another agent sets manual, see TestSetTitle_ByOther).
	agent := actor.Actor{ID: ids.New(), Kind: actor.KindAgent}
	e.setThreadOperator(post.ID, agent.ID)
	e.setThreadOperator(th.ID, agent.ID)
	_, err = e.svc.SetTitle(ctx, agent, chat.SetTitleCmd{ThreadID: post.ID, Title: "x", IfMatch: match})
	wantCode(t, err, errs.CodeTitleManual)
	if got, err := e.svc.SetTitle(ctx, e.owner, chat.SetTitleCmd{ThreadID: post.ID, Title: "Crash 2", IfMatch: match}); err != nil ||
		got.Title != "Crash 2" || got.Version != cur.Version+1 || e.eventCount("thread.updated") != 1 {
		t.Fatalf("title %+v, %v", got, err)
	}
	_, err = e.svc.SetTitle(ctx, e.owner, chat.SetTitleCmd{ThreadID: text.MainThreadID, Title: "x", IfMatch: chat.Precondition{Present: true, Valid: true, Version: 1}})
	wantCode(t, err, errs.CodeValidationFailed)
	auto, _ := e.svc.Get(ctx, e.owner, th.ID)
	if got, err := e.svc.SetTitle(ctx, agent, chat.SetTitleCmd{ThreadID: th.ID, Title: "Summary",
		IfMatch: chat.Precondition{Present: true, Valid: true, Version: auto.Version}}); err != nil || got.TitleSource != "auto" {
		t.Fatalf("agent title %+v, %v", got, err)
	}

	// Tags.
	cur, _ = e.svc.Get(ctx, e.owner, post.ID)
	if got, err := e.svc.SetTags(ctx, e.owner, chat.SetTagsCmd{ThreadID: post.ID, TagIDs: []ids.ID{bug.ID, staff.ID}}); err != nil || got.Version != cur.Version {
		t.Fatalf("same tags %+v, %v", got, err)
	}
	e.deny(authorize.ActionThreadTagsSet, forbidden)
	_, err = e.svc.SetTags(ctx, e.owner, chat.SetTagsCmd{ThreadID: post.ID, TagIDs: []ids.ID{bug.ID}})
	wantCode(t, err, errs.CodeForbidden)
	e.az.rules[authorize.ActionThreadTagsSet] = func(_ authorize.Principal, r authorize.Resource) authorize.Decision {
		if r.RestrictedTags {
			return forbidden
		}
		return allow
	}
	_, err = e.svc.SetTags(ctx, e.owner, chat.SetTagsCmd{ThreadID: post.ID, TagIDs: []ids.ID{bug.ID}})
	wantCode(t, err, errs.CodeForbidden)
	e.az.rules[authorize.ActionThreadTagsSet] = nil
	if got, err := e.svc.SetTags(ctx, e.owner, chat.SetTagsCmd{ThreadID: post.ID, TagIDs: []ids.ID{bug.ID}}); err != nil ||
		!slices.Equal(got.TagIDs, []ids.ID{bug.ID}) || got.Version != cur.Version+1 {
		t.Fatalf("tags %+v, %v", got, err)
	}
	_, err = e.svc.SetTags(ctx, e.owner, chat.SetTagsCmd{ThreadID: post.ID, TagIDs: nil, IfMatch: chat.Precondition{Present: true, Valid: true, Version: 1}})
	wantCode(t, err, errs.CodeVersionConflict)
	_, err = e.svc.SetTags(ctx, e.owner, chat.SetTagsCmd{ThreadID: th.ID})
	wantCode(t, err, errs.CodeValidationFailed)
	_, err = e.svc.SetTags(ctx, e.owner, chat.SetTagsCmd{ThreadID: post.ID, TagIDs: []ids.ID{bug.ID, bug.ID}})
	wantCode(t, err, errs.CodeValidationFailed)
	// Deleting a tag drops its assignments.
	if err := e.svc.DeleteForumTag(ctx, e.owner, forum.ID, bug.ID); err != nil {
		t.Fatal(err)
	}
	if got, _ := e.svc.Get(ctx, e.owner, post.ID); len(got.TagIDs) != 0 {
		t.Fatalf("tags after the delete %v", got.TagIDs)
	}
}

// TestThreadOperatorAndBilling: a thread captures the effective operator of its channel and shows
// its billing mode from the policy source; an unknown policy leaves the mode absent.
func TestThreadOperatorAndBilling(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	sp := e.create(t, "Support", ids.ID{})
	forum := e.channel(t, sp.ID, "forum", "forum")
	op := ids.New()
	e.store.mu.Lock()
	s := e.store.spaces[sp.ID]
	s.DefaultOperatorAgentID = op
	e.store.spaces[sp.ID] = s
	e.store.mu.Unlock()
	th, err := e.newThread(e.owner, chat.CreateThreadCmd{ChannelID: forum.ID})
	if err != nil || th.OperatorAgentID != op || th.BillingMode != "" {
		t.Fatalf("thread %+v, %v", th, err)
	}
	e.store.mu.Lock()
	e.store.policies = map[ids.ID]authz.AgentPolicy{op: {BillingMode: "organization"}}
	e.store.mu.Unlock()
	if got, err := e.svc.Get(context.Background(), e.owner, th.ID); err != nil || got.BillingMode != "organization" {
		t.Fatalf("billing %+v, %v", got, err)
	}
}

// TestListThreads: pages newest first, the cursor bound to filters, the default sort of the
// forum, the tag and state filters, the per-item filter, a refused channel and store errors.
func TestListThreads(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ctx := context.Background()
	sp := e.create(t, "Support", ids.ID{})
	forum := e.channel(t, sp.ID, "forum", "forum")
	var made []ids.ID
	for range 5 {
		th, err := e.newThread(e.owner, chat.CreateThreadCmd{ChannelID: forum.ID})
		if err != nil {
			t.Fatal(err)
		}
		made = append(made, th.ID)
	}
	page, err := e.svc.List(ctx, e.owner, chat.ListThreadsQuery{ChannelID: forum.ID, Limit: 2})
	if err != nil || len(page.Items) != 2 || page.NextCursor == "" {
		t.Fatalf("first page %+v, %v", page, err)
	}
	var seen []ids.ID
	for _, it := range page.Items {
		seen = append(seen, it.ID)
	}
	for page.NextCursor != "" {
		if page, err = e.svc.List(ctx, e.owner, chat.ListThreadsQuery{ChannelID: forum.ID, Limit: 2, Cursor: page.NextCursor}); err != nil {
			t.Fatal(err)
		}
		for _, it := range page.Items {
			seen = append(seen, it.ID)
		}
	}
	slices.SortFunc(seen, func(a, b ids.ID) int { return strings.Compare(a.String(), b.String()) })
	slices.SortFunc(made, func(a, b ids.ID) int { return strings.Compare(a.String(), b.String()) })
	if !slices.Equal(seen, made) {
		t.Fatalf("pages %v, want %v", seen, made)
	}
	first, _ := e.svc.List(ctx, e.owner, chat.ListThreadsQuery{ChannelID: forum.ID, Limit: 1})
	_, err = e.svc.List(ctx, e.owner, chat.ListThreadsQuery{ChannelID: forum.ID, Limit: 1, Sort: "created", Cursor: first.NextCursor})
	wantCode(t, err, errs.CodeCursorInvalid)
	_, err = e.svc.List(ctx, e.owner, chat.ListThreadsQuery{ChannelID: forum.ID, Cursor: "garbage"})
	wantCode(t, err, errs.CodeCursorInvalid)
	if p, err := e.svc.List(ctx, e.owner, chat.ListThreadsQuery{ChannelID: forum.ID, State: "closed"}); err != nil || len(p.Items) != 0 {
		t.Fatalf("closed %+v, %v", p, err)
	}
	if p, err := e.svc.List(ctx, e.owner, chat.ListThreadsQuery{ChannelID: forum.ID, TagID: ids.New(), Sort: "created"}); err != nil || len(p.Items) != 0 {
		t.Fatalf("tag %+v, %v", p, err)
	}
	// The default page size and the per-item filter in filter mode.
	hidden := made[0]
	e.az.rules[authorize.ActionThreadRead] = func(_ authorize.Principal, r authorize.Resource) authorize.Decision {
		if r.ID == hidden {
			return authorize.Decision{Outcome: authorize.OutcomeNotFound}
		}
		return allow
	}
	if p, err := e.svc.List(ctx, e.owner, chat.ListThreadsQuery{ChannelID: forum.ID}); err != nil || len(p.Items) != 4 ||
		slices.ContainsFunc(p.Items, func(th chat.Thread) bool { return th.ID == hidden }) {
		t.Fatalf("filtered %+v, %v", p, err)
	}
	e.deny(authorize.ActionThreadRead, authorize.Decision{Outcome: authorize.OutcomeNotFound})
	_, err = e.svc.List(ctx, e.owner, chat.ListThreadsQuery{ChannelID: forum.ID})
	wantCode(t, err, errs.CodeNotFound)
	e.az.rules[authorize.ActionThreadRead] = nil
	_, err = e.svc.List(ctx, e.owner, chat.ListThreadsQuery{ChannelID: ids.New()})
	wantCode(t, err, errs.CodeNotFound)
	e.az.err = errors.New("authz down")
	if _, err := e.svc.List(ctx, e.owner, chat.ListThreadsQuery{ChannelID: forum.ID}); err == nil {
		t.Fatal("authorize error swallowed")
	}
	e.az.err = nil
	e.store.fail = errs.New(errs.CodeInternalError, "down")
	for name, call := range map[string]func() error{
		"list": func() error {
			_, err := e.svc.List(ctx, e.owner, chat.ListThreadsQuery{ChannelID: forum.ID})
			return err
		},
		"get":    func() error { _, err := e.svc.Get(ctx, e.owner, made[1]); return err },
		"info":   func() error { _, err := e.svc.Info(ctx, made[1]); return err },
		"close":  func() error { _, err := e.svc.Close(ctx, e.owner, made[1]); return err },
		"append": func() error { _, err := e.appendTo(made[1], false); return err },
		"create": func() error { _, err := e.newThread(e.owner, chat.CreateThreadCmd{ChannelID: forum.ID}); return err },
	} {
		if err := call(); err == nil {
			t.Errorf("%s: store error swallowed", name)
		}
	}
}

// setThreadOperator sets the fixed operator of the stored thread id (R-DM-12) for the title tests.
func (e *env) setThreadOperator(id, agent ids.ID) {
	e.store.mu.Lock()
	defer e.store.mu.Unlock()
	th := e.store.threads[id]
	th.OperatorAgentID = agent
	e.store.threads[id] = th
}

// TestSetTitle_ByOther (brief M2-18 Ruling 6d): an agent that is not the thread's operator and
// passed authorize (the creator) sets manual on an auto title, and the operator agent then gets
// title_manual.
func TestSetTitle_ByOther(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ctx := context.Background()
	sp := e.create(t, "Support", ids.ID{})
	forum := e.channel(t, sp.ID, "forum", "forum")
	creator, operator := actor.Actor{ID: ids.New(), Kind: actor.KindAgent}, actor.Actor{ID: ids.New(), Kind: actor.KindAgent}
	post, err := e.newThread(creator, chat.CreateThreadCmd{ChannelID: forum.ID, Title: ptr("Release plan")})
	if err != nil || post.TitleSource != "auto" {
		t.Fatalf("post %+v, %v", post, err)
	}
	e.setThreadOperator(post.ID, operator.ID)
	got, err := e.svc.SetTitle(ctx, creator, chat.SetTitleCmd{ThreadID: post.ID, Title: "Release plan 2",
		IfMatch: chat.Precondition{Present: true, Valid: true, Version: post.Version}})
	if err != nil || got.TitleSource != "manual" || got.Version != post.Version+1 {
		t.Fatalf("creator title %+v, %v", got, err)
	}
	_, err = e.svc.SetTitle(ctx, operator, chat.SetTitleCmd{ThreadID: post.ID, Title: "Summary",
		IfMatch: chat.Precondition{Present: true, Valid: true, Version: got.Version}})
	wantCode(t, err, errs.CodeTitleManual)
}
