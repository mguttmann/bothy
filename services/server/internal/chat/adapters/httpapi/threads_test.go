// SPDX-License-Identifier: Apache-2.0

package httpapi_test

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// threadPath and channelThreadsPath are the paths of the thread operations.
func threadPath(id ids.ID) string         { return "/api/v1/threads/" + id.String() }
func channelThreadsPath(ch ids.ID) string { return channelPath(ch) + "/threads" }

// errRollback makes a unit of work roll back in the tests.
var errRollback = errors.New("roll back")

// unit runs fn in one unit of work of the world, the way messaging calls the port.
func (w *world) unit(fn func(ctx context.Context) error) error {
	return w.uow.Do(context.Background(), fn)
}

// newThread creates a thread through the port as token name inside its own unit of work.
func (w *world) newThread(name string, cmd chat.CreateThreadCmd) (chat.Thread, error) {
	var out chat.Thread
	err := w.unit(func(ctx context.Context) error {
		var err error
		out, err = w.svc.Create(ctx, w.actors.Get(name), cmd)
		return err
	})
	return out, err
}

// post creates a forum post as the owner with title and tags and returns it.
func (w *world) post(forum ids.ID, title string, tags ...ids.ID) chat.Thread {
	w.t.Helper()
	th, err := w.newThread("owner", chat.CreateThreadCmd{ChannelID: forum, Title: &title, TagIDs: tags})
	if err != nil {
		w.t.Fatalf("create post %q: %v", title, err)
	}
	return th
}

// mainThread returns the main thread of the text channel ch.
func (w *world) mainThread(ch ids.ID) ids.ID {
	w.t.Helper()
	var id string
	if err := w.owner.QueryRow(context.Background(), `SELECT id::text FROM threads WHERE channel_id = $1 AND kind = 'main'`, ch.String()).Scan(&id); err != nil {
		w.t.Fatal(err)
	}
	return ids.MustParse(id)
}

// operatorAgent inserts an active agent with an agents row (the foreign key of
// threads.operator_agent_id) and registers its policy with billing mode.
func (w *world) operatorAgent(name, billing string) ids.ID {
	w.t.Helper()
	id := w.agent(name)
	holder := any(nil)
	if billing == "personal" {
		holder = w.ownerID.String()
	}
	w.sql(`INSERT INTO agents (principal_id, adapter_kind, billing_mode, credential_holder_id) VALUES ($1, 'hermes', $2, $3)`, id.String(), billing, holder)
	p := authz.AgentPolicy{BillingMode: billing, ToolClass: "chat"}
	if billing == "personal" {
		p.CredentialHolderID = w.ownerID
	}
	w.policies.Put(id, p)
	return id
}

// setOperator sets the own operator of the channel ch directly (M1-54 builds setChannelOperator).
func (w *world) setOperator(ch, agent ids.ID) {
	w.t.Helper()
	w.sql(`UPDATE channels SET operator_inherit = false, operator_agent_id = $2, version = version + 1 WHERE id = $1`, ch.String(), agent.String())
}

// forumTag creates a tag in the forum as the owner and returns its id.
func (w *world) forumTag(forum ids.ID, name string, restricted bool) ids.ID {
	w.t.Helper()
	body := `{"name":"` + name + `"`
	if restricted {
		body += `,"restricted":true`
	}
	m := w.want(w.do("owner", http.MethodPost, tagsPath(forum), body+"}", key()...), http.StatusCreated, "")
	return ids.MustParse(m["id"].(string))
}

// tagIDs returns the member tag_ids of a thread body.
func tagIDs(m map[string]any) []string {
	var out []string
	for _, t := range m["tag_ids"].([]any) {
		out = append(out, t.(string))
	}
	return out
}

// threadEvents returns the events of typ for the thread id.
func (w *world) threadEvents(typ string, id ids.ID) []event {
	var out []event
	for _, e := range w.events(typ) {
		if e.ResourceID == id.String() {
			out = append(out, e)
		}
	}
	return out
}

// TestThreadsCreate_ForumPostTitleSource (09 section 12, brief D1): a person's real title gives
// manual; empty, absent, "New chat" and "Neuer Chat 3" give auto; an app gives auto; every create
// appends thread.created with the thread as payload.
func TestThreadsCreate_ForumPostTitleSource(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	forum := w.channel(w.create("Support"), "forum", "bugs")
	for _, c := range []struct {
		title *string
		want  string
	}{
		{ptrTo("Invoice export fails"), "manual"},
		{ptrTo(""), "auto"},
		{nil, "auto"},
		{ptrTo("New chat"), "auto"},
		{ptrTo("  neuer   CHAT 3 "), "auto"},
	} {
		th, err := w.newThread("owner", chat.CreateThreadCmd{ChannelID: forum, Title: c.title})
		if err != nil {
			t.Fatal(err)
		}
		if th.TitleSource != c.want || th.Kind != "thread" || th.State != "open" || th.Version != 1 || th.CreatedBy != w.ownerID ||
			!th.OperatorAgentID.IsZero() || th.TagIDs == nil {
			t.Errorf("title %v: %+v", c.title, th)
		}
		if c.title != nil && th.Title != *c.title {
			t.Errorf("title stored as %q, want %q", th.Title, *c.title)
		}
		evs := w.threadEvents("thread.created", th.ID)
		if len(evs) != 1 || evs[0].ReceiveAction != "thread.read" || evs[0].Version != 1 || evs[0].ResourceKind != "thread" {
			t.Fatalf("thread.created %+v", evs)
		}
		if p := evs[0].Payload["thread"].(map[string]any); p["title_source"] != c.want || p["id"] != th.ID.String() {
			t.Errorf("payload %v", p)
		}
	}
	app := ids.New()
	w.sql(`INSERT INTO principals (id, kind, status, handle, display_name) VALUES ($1, 'app', 'active', 'reporter', 'Reporter')`, app.String())
	w.actors.Set("app", actor.Actor{ID: app, Kind: actor.KindApp})
	th, err := w.newThread("app", chat.CreateThreadCmd{ChannelID: forum, Title: ptrTo("Nightly report")})
	if err != nil || th.TitleSource != "auto" {
		t.Fatalf("app thread %+v, %v", th, err)
	}
	long := strings.Repeat("x", titleMax+1)
	_, err = w.newThread("owner", chat.CreateThreadCmd{ChannelID: forum, Title: &long})
	wantField(t, err, "/title", "too_long")
	_, err = w.newThread("owner", chat.CreateThreadCmd{ChannelID: forum, Title: ptrTo("a\u202eb")})
	wantField(t, err, "/title", "bidi_control")
	if _, err = w.newThread("owner", chat.CreateThreadCmd{ChannelID: ids.New()}); errCode(err) != errs.CodeNotFound {
		t.Fatalf("unknown channel: %v", err)
	}
}

// TestThreadsCreate_FromMainThreadMessage (brief D1): a parent message of the main thread is
// accepted; a parent of another channel's main thread is 400; a second thread of the same parent
// is 409; tags in a text channel and a parent in a forum are 400.
func TestThreadsCreate_FromMainThreadMessage(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	space := w.create("Support")
	text, other, forum := w.channel(space, "text", "tickets"), w.channel(space, "text", "sales"), w.channel(space, "forum", "bugs")
	main, otherMain := w.mainThread(text), w.mainThread(other)
	msg := ids.New()
	th, err := w.newThread("owner", chat.CreateThreadCmd{ChannelID: text, Parent: &chat.ParentMessage{MessageID: msg, ThreadID: main}})
	if err != nil || th.ParentMessageID != msg || th.Kind != "thread" || th.ChannelID != text {
		t.Fatalf("thread from message: %+v, %v", th, err)
	}
	_, err = w.newThread("owner", chat.CreateThreadCmd{ChannelID: text, Parent: &chat.ParentMessage{MessageID: ids.New(), ThreadID: otherMain}})
	wantField(t, err, "/parent_message_id", "not_in_main_thread")
	_, err = w.newThread("owner", chat.CreateThreadCmd{ChannelID: text, Parent: &chat.ParentMessage{MessageID: msg, ThreadID: main}})
	if errCode(err) != errs.CodeConflict {
		t.Fatalf("second thread of the parent: %v", err)
	}
	_, err = w.newThread("owner", chat.CreateThreadCmd{ChannelID: text})
	wantField(t, err, "/parent_message_id", "required_in_text_channel")
	_, err = w.newThread("owner", chat.CreateThreadCmd{ChannelID: text, Parent: &chat.ParentMessage{MessageID: ids.New(), ThreadID: main}, TagIDs: []ids.ID{ids.New()}})
	wantField(t, err, "/tag_ids", "tags_only_in_forum")
	_, err = w.newThread("owner", chat.CreateThreadCmd{ChannelID: forum, Parent: &chat.ParentMessage{MessageID: ids.New(), ThreadID: main}})
	wantField(t, err, "/parent_message_id", "not_allowed_in_forum")
	if n := w.count(`SELECT count(*)::int FROM threads WHERE kind = 'thread'`); n != 1 {
		t.Fatalf("%d threads, want 1", n)
	}
}

// TestR_DM_12_OperatorFixedAfterChannelChange: a thread created while Lotse operates the channel
// keeps Lotse after the channel operator changes; getThread shows Lotse with Lotse's billing mode;
// the main thread never carries an operator (R-DM-21).
func TestR_DM_12_OperatorFixedAfterChannelChange(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	space := w.create("Support")
	text := w.channel(space, "text", "tickets")
	lotse, kompass := w.operatorAgent("lotse", "personal"), w.operatorAgent("kompass", "organization")
	w.setOperator(text, lotse)
	main := w.mainThread(text)
	th, err := w.newThread("owner", chat.CreateThreadCmd{ChannelID: text, Parent: &chat.ParentMessage{MessageID: ids.New(), ThreadID: main}})
	if err != nil || th.OperatorAgentID != lotse || th.BillingMode != "personal" {
		t.Fatalf("thread %+v, %v", th, err)
	}
	w.setOperator(text, kompass)
	m := w.want(w.do("owner", http.MethodGet, threadPath(th.ID), ""), http.StatusOK, "")
	if m["operator_agent_id"] != lotse.String() || m["billing_mode"] != "personal" {
		t.Fatalf("thread after the operator change %v", m)
	}
	m = w.want(w.do("owner", http.MethodGet, threadPath(main), ""), http.StatusOK, "")
	if _, ok := m["operator_agent_id"]; ok {
		t.Fatalf("main thread with an operator: %v", m)
	}
	if _, ok := m["billing_mode"]; ok {
		t.Fatalf("main thread with a billing mode: %v", m)
	}
	if w.count(`SELECT count(*)::int FROM threads WHERE id = $1 AND operator_agent_id = $2`, th.ID.String(), lotse.String()) != 1 {
		t.Fatal("operator_agent_id changed")
	}
}

// TestAppendToThread_GaplessUnderConcurrency (03 section 6.2): many goroutines append to one
// thread, every third rolls back after it got its number; the committed thread_seq values are exactly 1..n and the
// counters match.
func TestAppendToThread_GaplessUnderConcurrency(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	forum := w.channel(w.create("Support"), "forum", "bugs")
	th := w.post(forum, "Load")
	const workers = 40
	var (
		mu        sync.Mutex
		committed []int64
		wg        sync.WaitGroup
	)
	owner := w.actors.Get("owner")
	for i := range workers {
		wg.Add(1)
		go func(rollback bool) {
			defer wg.Done()
			var seq int64
			err := w.unit(func(ctx context.Context) error {
				got, err := w.svc.AppendToThread(ctx, owner, chat.AppendCmd{ThreadID: th.ID})
				if err != nil {
					return err
				}
				seq = got.Seq
				if rollback {
					return errRollback
				}
				return nil
			})
			switch {
			case rollback && errors.Is(err, errRollback):
			case err != nil:
				t.Errorf("append %d: %v", i, err)
			default:
				mu.Lock()
				committed = append(committed, seq)
				mu.Unlock()
			}
		}(i%3 == 0)
	}
	wg.Wait()
	slices.Sort(committed)
	for i, s := range committed {
		if s != int64(i+1) {
			t.Fatalf("committed sequence %v is not 1..%d", committed, len(committed))
		}
	}
	n := len(committed)
	if w.count(`SELECT last_message_seq::int FROM threads WHERE id = $1`, th.ID.String()) != n ||
		w.count(`SELECT message_count FROM threads WHERE id = $1`, th.ID.String()) != n ||
		w.count(`SELECT version::int FROM threads WHERE id = $1`, th.ID.String()) != n+1 {
		t.Fatalf("counters differ from %d committed appends", n)
	}
}

// TestAppendToThread_ReopensClosedThread (brief D2): a closed thread with ReopenIfClosed reopens,
// appends one thread.reopened and reports it; without the flag the state stays closed.
func TestAppendToThread_ReopensClosedThread(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	forum := w.channel(w.create("Support"), "forum", "bugs")
	th := w.post(forum, "Done soon")
	w.want(w.do("owner", http.MethodPost, threadPath(th.ID)+"/close", "", key()...), http.StatusOK, "")
	owner := w.actors.Get("owner")
	var got chat.Appended
	if err := w.unit(func(ctx context.Context) error {
		var err error
		got, err = w.svc.AppendToThread(ctx, owner, chat.AppendCmd{ThreadID: th.ID})
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if got.Reopened || got.Thread.State != "closed" || got.Seq != 1 {
		t.Fatalf("append without the flag %+v", got)
	}
	if err := w.unit(func(ctx context.Context) error {
		var err error
		got, err = w.svc.AppendToThread(ctx, owner, chat.AppendCmd{ThreadID: th.ID, ReopenIfClosed: true})
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if !got.Reopened || got.Thread.State != "open" || got.Seq != 2 || got.Thread.ChannelID != forum || got.Thread.ChannelKind != "forum" {
		t.Fatalf("append with the flag %+v", got)
	}
	evs := w.threadEvents("thread.reopened", th.ID)
	if len(evs) != 1 || evs[0].Payload["thread"].(map[string]any)["state"] != "open" {
		t.Fatalf("thread.reopened %+v", evs)
	}
	if w.count(`SELECT count(*)::int FROM threads WHERE id = $1 AND closed_at IS NULL AND closed_by IS NULL`, th.ID.String()) != 1 {
		t.Fatal("closed_at or closed_by kept")
	}
}

// TestAppendToThread_RequiresUnitOfWork: Create and AppendToThread outside a unit of work are
// internal errors and write nothing.
func TestAppendToThread_RequiresUnitOfWork(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	forum := w.channel(w.create("Support"), "forum", "bugs")
	th := w.post(forum, "Alone")
	owner := w.actors.Get("owner")
	if _, err := w.svc.AppendToThread(context.Background(), owner, chat.AppendCmd{ThreadID: th.ID}); errCode(err) != errs.CodeInternalError {
		t.Fatalf("append outside a unit: %v", err)
	}
	if _, err := w.svc.Create(context.Background(), owner, chat.CreateThreadCmd{ChannelID: forum}); errCode(err) != errs.CodeInternalError {
		t.Fatalf("create outside a unit: %v", err)
	}
	if w.count(`SELECT last_message_seq::int FROM threads WHERE id = $1`, th.ID.String()) != 0 || w.count(`SELECT count(*)::int FROM threads`) != 1 {
		t.Fatal("a call outside a unit wrote")
	}
	info, err := w.svc.Info(context.Background(), th.ID)
	if err != nil || info.SpaceID.IsZero() || info.Kind != "thread" || info.Version != 1 || info.CreatedBy != w.ownerID {
		t.Fatalf("info %+v, %v", info, err)
	}
	if _, err := w.svc.Info(context.Background(), ids.New()); errCode(err) != errs.CodeNotFound {
		t.Fatalf("info of an unknown thread: %v", err)
	}
}

// TestCloseReopen_EventsAndNoOps (brief D3): close and reopen bump the version once and append one
// event each; closing a closed and reopening an open thread answer 200 without an event.
func TestCloseReopen_EventsAndNoOps(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	forum := w.channel(w.create("Support"), "forum", "bugs")
	th := w.post(forum, "Close me")
	m := w.want(w.do("owner", http.MethodPost, threadPath(th.ID)+"/close", "", key()...), http.StatusOK, "")
	if m["state"] != "closed" || m["version"] != float64(2) || m["closed_by"] != w.ownerID.String() || m["closed_at"] == nil {
		t.Fatalf("closed %v", m)
	}
	m = w.want(w.do("owner", http.MethodPost, threadPath(th.ID)+"/close", "", key()...), http.StatusOK, "")
	if m["version"] != float64(2) {
		t.Fatalf("second close %v", m)
	}
	m = w.want(w.do("owner", http.MethodPost, threadPath(th.ID)+"/reopen", "", key()...), http.StatusOK, "")
	if m["state"] != "open" || m["version"] != float64(3) || m["closed_by"] != nil {
		t.Fatalf("reopened %v", m)
	}
	w.want(w.do("owner", http.MethodPost, threadPath(th.ID)+"/reopen", "", key()...), http.StatusOK, "")
	if c, r := w.threadEvents("thread.closed", th.ID), w.threadEvents("thread.reopened", th.ID); len(c) != 1 || len(r) != 1 || c[0].Version != 2 || r[0].Version != 3 {
		t.Fatalf("events closed %+v reopened %+v", c, r)
	}
	// The same Idempotency-Key answers the stored response.
	k := key()
	first := w.want(w.do("owner", http.MethodPost, threadPath(th.ID)+"/close", "", k...), http.StatusOK, "")
	again := w.want(w.do("owner", http.MethodPost, threadPath(th.ID)+"/close", "", k...), http.StatusOK, "")
	if !reflect.DeepEqual(first, again) || len(w.threadEvents("thread.closed", th.ID)) != 2 {
		t.Fatalf("idempotent close %v %v", first, again)
	}
}

// TestCloseMainThread_409: the main thread is never done.
func TestCloseMainThread_409(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	text := w.channel(w.create("Support"), "text", "tickets")
	main := w.mainThread(text)
	w.want(w.do("owner", http.MethodPost, threadPath(main)+"/close", "", key()...), http.StatusConflict, "invalid_state_transition")
	m := w.want(w.do("owner", http.MethodPost, threadPath(main)+"/reopen", "", key()...), http.StatusOK, "")
	if m["kind"] != "main" || m["version"] != float64(1) {
		t.Fatalf("main thread %v", m)
	}
}

// TestSetThreadTitle_IfMatchAndSource (brief D4): 428 without If-Match, 412 with current on a
// stale one, a person sets manual and the ETag; an unchanged title appends no event; the main
// thread has no title; the title rules of create apply.
func TestSetThreadTitle_IfMatchAndSource(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	space := w.create("Support")
	forum, text := w.channel(space, "forum", "bugs"), w.channel(space, "text", "tickets")
	th := w.post(forum, "New chat")
	if th.TitleSource != "auto" {
		t.Fatalf("source %s", th.TitleSource)
	}
	path := threadPath(th.ID) + "/title"
	w.want(w.do("owner", http.MethodPut, path, `{"title":"Export"}`), http.StatusPreconditionRequired, "precondition_required")
	p := w.want(w.do("owner", http.MethodPut, path, `{"title":"Export"}`, ifMatch(7)...), http.StatusPreconditionFailed, "version_conflict")
	if cur, _ := p["current"].(map[string]any); cur["id"] != th.ID.String() || cur["version"] != float64(1) {
		t.Fatalf("current %v", p)
	}
	r := w.do("owner", http.MethodPut, path, `{"title":"Export"}`, ifMatch(1)...)
	m := w.want(r, http.StatusOK, "")
	if m["title"] != "Export" || m["title_source"] != "manual" || m["version"] != float64(2) || r.Header.Get("ETag") != `"2"` {
		t.Fatalf("title %v etag %q", m, r.Header.Get("ETag"))
	}
	w.want(w.do("owner", http.MethodPut, path, `{"title":"Export"}`, ifMatch(2)...), http.StatusOK, "")
	if evs := w.threadEvents("thread.updated", th.ID); len(evs) != 1 || evs[0].Version != 2 {
		t.Fatalf("thread.updated %+v", evs)
	}
	p = w.want(w.do("owner", http.MethodPut, path, `{"title":"a`+"\\u0007"+`b"}`, ifMatch(2)...), http.StatusBadRequest, "validation_failed")
	if pointer(p) != "/title" {
		t.Fatalf("problem %v", p)
	}
	w.want(w.do("owner", http.MethodPut, path, `{"title":"`+strings.Repeat("ü", titleMax+1)+`"}`, ifMatch(2)...), http.StatusBadRequest, "validation_failed")
	main := w.mainThread(text)
	p = w.want(w.do("owner", http.MethodPut, threadPath(main)+"/title", `{"title":"Main"}`, ifMatch(1)...), http.StatusBadRequest, "validation_failed")
	if pointer(p) != "/title" || p["errors"].([]any)[0].(map[string]any)["code"] != "main_thread_has_no_title" {
		t.Fatalf("main thread title %v", p)
	}
	r = w.do("owner", http.MethodGet, threadPath(th.ID), "")
	if w.want(r, http.StatusOK, "")["title"] != "Export" || r.Header.Get("ETag") != `"2"` {
		t.Fatalf("getThread %s %q", r.Body, r.Header.Get("ETag"))
	}
}

// TestAG_19_OperatorAgentTitleOnManual409 (09 section 12, AG-19): the operator agent sets an
// auto title and the source stays auto; on a manual title it gets 409 title_manual; an agent with
// write that is not the operator gets 403; the creator with write may set the title.
func TestAG_19_OperatorAgentTitleOnManual409(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	space := w.create("Support")
	forum := w.channel(space, "forum", "bugs")
	lotse := w.operatorAgent("lotse", "organization")
	w.setOperator(forum, lotse)
	other := w.operatorAgent("kompass", "organization")
	w.sql(`INSERT INTO grants (id, subject_kind, subject_principal_id, target_kind, target_channel_id, level, capabilities, reason, granted_by)
		VALUES ($1, 'agent', $2, 'channel', $3, 'write', '{}', 'fixture', $4)`, ids.New().String(), other.String(), forum.String(), w.ownerID.String())
	th := w.post(forum, "")
	path := threadPath(th.ID) + "/title"
	m := w.want(w.do("lotse", http.MethodPut, path, `{"title":"Export fails"}`, ifMatch(1)...), http.StatusOK, "")
	if m["title_source"] != "auto" || m["title"] != "Export fails" {
		t.Fatalf("operator title %v", m)
	}
	w.want(w.do("kompass", http.MethodPut, path, `{"title":"Mine"}`, ifMatch(2)...), http.StatusForbidden, "forbidden")
	w.want(w.do("owner", http.MethodPut, path, `{"title":"Invoice export"}`, ifMatch(2)...), http.StatusOK, "")
	w.want(w.do("lotse", http.MethodPut, path, `{"title":"Better"}`, ifMatch(3)...), http.StatusConflict, "title_manual")
	if w.count(`SELECT count(*)::int FROM threads WHERE id = $1 AND title = 'Invoice export' AND title_source = 'manual'`, th.ID.String()) != 1 {
		t.Fatal("the manual title was replaced")
	}
	alex := w.person("alex", false)
	w.channelGrant(alex, forum, "write", "{}")
	own, err := w.newThread("alex", chat.CreateThreadCmd{ChannelID: forum, Title: ptrTo("Mine")})
	if err != nil {
		t.Fatal(err)
	}
	w.want(w.do("alex", http.MethodPut, threadPath(own.ID)+"/title", `{"title":"Mine too"}`, ifMatch(1)...), http.StatusOK, "")
	w.want(w.do("alex", http.MethodPut, path, `{"title":"Not mine"}`, ifMatch(3)...), http.StatusForbidden, "forbidden")
	// A thread created without an operator has none; a later channel operator is not its operator
	// (R-DM-12), so Lotse may not title it.
	plain := w.channel(space, "forum", "ideas")
	orphan := w.post(plain, "")
	w.setOperator(plain, lotse)
	w.want(w.do("lotse", http.MethodPut, threadPath(orphan.ID)+"/title", `{"title":"Taken"}`, ifMatch(1)...), http.StatusForbidden, "forbidden")
}

// TestSetThreadTags_Rules (brief D5): a restricted tag needs manage; a tag of another forum,
// duplicates and too many tags are 400; a thread of a text channel is 400; the same set appends no
// event; If-Match is optional and checked when sent.
func TestSetThreadTags_Rules(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	space := w.create("Support")
	forum, otherForum, text := w.channel(space, "forum", "bugs"), w.channel(space, "forum", "ideas"), w.channel(space, "text", "tickets")
	bug, ui, staff := w.forumTag(forum, "bug", false), w.forumTag(forum, "ui", false), w.forumTag(forum, "staff", true)
	foreign := w.forumTag(otherForum, "idea", false)
	alex := w.person("alex", false)
	w.channelGrant(alex, forum, "write", "{upload}")
	th := w.post(forum, "Crash", bug)
	path := threadPath(th.ID) + "/tags"
	body := func(tags ...ids.ID) string {
		var s []string
		for _, t := range tags {
			s = append(s, `"`+t.String()+`"`)
		}
		return `{"tag_ids":[` + strings.Join(s, ",") + `]}`
	}
	w.want(w.do("alex", http.MethodPut, path, body(bug, staff)), http.StatusForbidden, "forbidden")
	r := w.do("owner", http.MethodPut, path, body(staff, bug))
	m := w.want(r, http.StatusOK, "")
	if got := tagIDs(m); !slices.Equal(got, []string{bug.String(), staff.String()}) || m["version"] != float64(2) || r.Header.Get("ETag") != `"2"` {
		t.Fatalf("tags %v (order of the tags) %q", m, r.Header.Get("ETag"))
	}
	// Keeping the restricted tag while changing a plain one needs no manage.
	w.want(w.do("alex", http.MethodPut, path, body(staff, ui)), http.StatusOK, "")
	w.want(w.do("alex", http.MethodPut, path, body(ui)), http.StatusForbidden, "forbidden")
	for _, c := range []struct {
		body, code string
	}{
		{body(ui, foreign), "unknown_tag"},
		{body(ui, ids.New()), "unknown_tag"},
		{body(ui, ui), "duplicate"},
		{body(ui, bug, staff), "limit_reached"},
	} {
		p := w.want(w.do("owner", http.MethodPut, path, c.body), http.StatusBadRequest, "validation_failed")
		if pointer(p) != "/tag_ids" || p["errors"].([]any)[0].(map[string]any)["code"] != c.code {
			t.Errorf("%s: %v", c.code, p)
		}
	}
	textThread, err := w.newThread("owner", chat.CreateThreadCmd{ChannelID: text, Parent: &chat.ParentMessage{MessageID: ids.New(), ThreadID: w.mainThread(text)}})
	if err != nil {
		t.Fatal(err)
	}
	p := w.want(w.do("owner", http.MethodPut, threadPath(textThread.ID)+"/tags", body()), http.StatusBadRequest, "validation_failed")
	if p["errors"].([]any)[0].(map[string]any)["code"] != "not_a_forum_post" {
		t.Fatalf("text channel thread %v", p)
	}
	before := len(w.threadEvents("thread.updated", th.ID))
	w.want(w.do("owner", http.MethodPut, path, body(ui, staff), ifMatch(3)...), http.StatusOK, "")
	if len(w.threadEvents("thread.updated", th.ID)) != before {
		t.Fatal("the same set appended an event")
	}
	w.want(w.do("owner", http.MethodPut, path, body(ui), ifMatch(1)...), http.StatusPreconditionFailed, "version_conflict")
	// Creating a post with a restricted tag follows thread.tags.set.
	if _, err := w.newThread("alex", chat.CreateThreadCmd{ChannelID: forum, TagIDs: []ids.ID{staff}}); errCode(err) != errs.CodeForbidden {
		t.Fatalf("create with a restricted tag without manage: %v", err)
	}
	if _, err := w.newThread("alex", chat.CreateThreadCmd{ChannelID: forum, TagIDs: []ids.ID{bug, ui, staff}}); errCode(err) != errs.CodeValidationFailed {
		t.Fatalf("create with too many tags: %v", err)
	}
	if th2, err := w.newThread("owner", chat.CreateThreadCmd{ChannelID: forum, TagIDs: []ids.ID{staff}}); err != nil || !slices.Equal(th2.TagIDs, []ids.ID{staff}) {
		t.Fatalf("owner with a restricted tag: %+v, %v", th2, err)
	}
}

// TestDeleteForumTag_RemovesThreadTags (brief D5): deleting a forum tag drops its thread_tags rows
// in the same transaction without a thread event.
func TestDeleteForumTag_RemovesThreadTags(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	forum := w.channel(w.create("Support"), "forum", "bugs")
	bug, ui := w.forumTag(forum, "bug", false), w.forumTag(forum, "ui", false)
	th := w.post(forum, "Crash", bug, ui)
	updated := len(w.events("thread.updated"))
	w.want(w.do("owner", http.MethodDelete, tagPath(forum, bug), "", key()...), http.StatusNoContent, "")
	if w.count(`SELECT count(*)::int FROM thread_tags WHERE tag_id = $1`, bug.String()) != 0 ||
		w.count(`SELECT count(*)::int FROM thread_tags WHERE tag_id = $1`, ui.String()) != 1 {
		t.Fatal("thread_tags rows of the deleted tag remain")
	}
	m := w.want(w.do("owner", http.MethodGet, threadPath(th.ID), ""), http.StatusOK, "")
	if !slices.Equal(tagIDs(m), []string{ui.String()}) || len(w.events("thread.updated")) != updated {
		t.Fatalf("thread after the delete %v", m)
	}
}

// listIDs returns the item ids and next_cursor of listChannelThreads for token name and query.
func (w *world) listIDs(name string, ch ids.ID, query url.Values) ([]string, string) {
	w.t.Helper()
	path := channelThreadsPath(ch)
	if len(query) > 0 {
		path += "?" + query.Encode()
	}
	m := w.want(w.do(name, http.MethodGet, path, ""), http.StatusOK, "")
	var out []string
	for _, it := range m["items"].([]any) {
		out = append(out, it.(map[string]any)["id"].(string))
	}
	next, _ := m["next_cursor"].(string)
	if _, ok := m["total"]; ok {
		w.t.Fatal("a page with a total")
	}
	return out, next
}

// TestListChannelThreads_FiltersSortAndPages (brief D6): the done section through state=closed,
// the tag filter, both sorts, pages stable while threads are created between requests, a cursor of
// another filter is 400 cursor_invalid, a limit above api.page_size_max is capped, the main thread
// is not listed, an unknown tag gives an empty page, an invisible channel is 404.
func TestListChannelThreads_FiltersSortAndPages(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	space := w.create("Support")
	forum, text := w.channel(space, "forum", "bugs"), w.channel(space, "text", "tickets")
	bug := w.forumTag(forum, "bug", false)
	var posts []chat.Thread
	for i := range 6 {
		w.clk.Advance(time.Minute)
		var tags []ids.ID
		if i%2 == 0 {
			tags = []ids.ID{bug}
		}
		posts = append(posts, w.post(forum, "Post "+string(rune('A'+i)), tags...))
	}
	// Activity in post A makes it the most recent by last_activity, not by created.
	w.clk.Advance(time.Minute)
	if err := w.unit(func(ctx context.Context) error {
		_, err := w.svc.AppendToThread(ctx, w.actors.Get("owner"), chat.AppendCmd{ThreadID: posts[0].ID})
		return err
	}); err != nil {
		t.Fatal(err)
	}
	w.want(w.do("owner", http.MethodPost, threadPath(posts[1].ID)+"/close", "", key()...), http.StatusOK, "")
	id := func(i int) string { return posts[i].ID.String() }
	all := url.Values{"limit": {"10"}}
	got, next := w.listIDs("owner", forum, all)
	if want := []string{id(0), id(5), id(4), id(3)}; !slices.Equal(got, want) || next == "" {
		t.Fatalf("by activity capped at 4: %v next %q", got, next)
	}
	got, next = w.listIDs("owner", forum, url.Values{"cursor": {next}, "limit": {"10"}})
	if want := []string{id(2), id(1)}; !slices.Equal(got, want) || next != "" {
		t.Fatalf("second page %v next %q", got, next)
	}
	got, _ = w.listIDs("owner", forum, url.Values{"sort": {"created"}, "limit": {"2"}})
	if want := []string{id(5), id(4)}; !slices.Equal(got, want) {
		t.Fatalf("by created %v", got)
	}
	got, _ = w.listIDs("owner", forum, url.Values{"state": {"closed"}})
	if !slices.Equal(got, []string{id(1)}) {
		t.Fatalf("done section %v", got)
	}
	got, _ = w.listIDs("owner", forum, url.Values{"tag_id": {bug.String()}, "state": {"open"}})
	if want := []string{id(0), id(4), id(2)}; !slices.Equal(got, want) {
		t.Fatalf("tag filter %v", got)
	}
	got, _ = w.listIDs("owner", forum, url.Values{"tag_id": {ids.New().String()}})
	if len(got) != 0 {
		t.Fatalf("unknown tag %v", got)
	}
	// Default page size api.page_size_default; a new thread between the pages moves nothing.
	got, next = w.listIDs("owner", forum, nil)
	if want := []string{id(0), id(5), id(4)}; !slices.Equal(got, want) {
		t.Fatalf("default page %v", got)
	}
	w.clk.Advance(time.Minute)
	w.post(forum, "Late")
	got, next2 := w.listIDs("owner", forum, url.Values{"cursor": {next}})
	if want := []string{id(3), id(2), id(1)}; !slices.Equal(got, want) || next2 != "" {
		t.Fatalf("page after a new thread %v next %q", got, next2)
	}
	w.want(w.do("owner", http.MethodGet, channelThreadsPath(forum)+"?state=open&cursor="+url.QueryEscape(next), ""), http.StatusBadRequest, "cursor_invalid")
	w.want(w.do("owner", http.MethodGet, channelThreadsPath(forum)+"?sort=created&cursor="+url.QueryEscape(next), ""), http.StatusBadRequest, "cursor_invalid")
	// A text channel lists its threads, never the main thread.
	th, err := w.newThread("owner", chat.CreateThreadCmd{ChannelID: text, Parent: &chat.ParentMessage{MessageID: ids.New(), ThreadID: w.mainThread(text)}})
	if err != nil {
		t.Fatal(err)
	}
	got, _ = w.listIDs("owner", text, nil)
	if !slices.Equal(got, []string{th.ID.String()}) {
		t.Fatalf("text channel %v", got)
	}
	// Another caller cannot use the cursor; an invisible channel is 404 like an unknown one.
	alex := w.person("alex", false)
	w.channelGrant(alex, forum, "read", "{}")
	w.want(w.do("alex", http.MethodGet, channelThreadsPath(forum)+"?cursor="+url.QueryEscape(next), ""), http.StatusBadRequest, "cursor_invalid")
	w.want(w.do("alex", http.MethodGet, channelThreadsPath(text), ""), http.StatusNotFound, "not_found")
	w.want(w.do("alex", http.MethodGet, channelThreadsPath(ids.New()), ""), http.StatusNotFound, "not_found")
	if got, _ := w.listIDs("alex", forum, url.Values{"limit": {"1"}}); len(got) != 1 {
		t.Fatalf("reader page %v", got)
	}
}

// TestGetThread_ForeignAndUnknownIdentical404 (AZ-P4 style): a thread the caller cannot see and
// an unknown thread give the same problem, apart from correlation_id and instance.
func TestGetThread_ForeignAndUnknownIdentical404(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	forum := w.channel(w.create("Support"), "forum", "bugs")
	th := w.post(forum, "Secret")
	w.person("alex", false)
	strip := func(p map[string]any) map[string]any {
		delete(p, "correlation_id")
		delete(p, "instance")
		return p
	}
	for _, op := range []struct{ method, suffix, body string }{
		{http.MethodGet, "", ""},
		{http.MethodPost, "/close", ""},
		{http.MethodPut, "/tags", `{"tag_ids":[]}`},
	} {
		foreign := strip(w.want(w.do("alex", op.method, threadPath(th.ID)+op.suffix, op.body, key()...), http.StatusNotFound, "not_found"))
		unknown := strip(w.want(w.do("alex", op.method, threadPath(ids.New())+op.suffix, op.body, key()...), http.StatusNotFound, "not_found"))
		if !reflect.DeepEqual(foreign, unknown) {
			t.Fatalf("%s%s: foreign %v unknown %v", op.method, op.suffix, foreign, unknown)
		}
	}
}

// TestThreadWrites_ArchivedChannel403: close, title and tags in an archived channel give 403
// channel_read_only with reason channel_archived; reading stays allowed.
func TestThreadWrites_ArchivedChannel403(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	forum := w.channel(w.create("Support"), "forum", "bugs")
	th := w.post(forum, "Old")
	w.want(w.do("owner", http.MethodPost, channelPath(forum)+"/archive", `{"reason":"Moved"}`, key()...), http.StatusOK, "")
	for _, c := range []struct {
		method, suffix, body string
		headers              []string
	}{
		{http.MethodPost, "/close", "", key()},
		{http.MethodPost, "/reopen", "", key()},
		{http.MethodPut, "/title", `{"title":"New"}`, ifMatch(1)},
		{http.MethodPut, "/tags", `{"tag_ids":[]}`, nil},
	} {
		p := w.want(w.do("owner", c.method, threadPath(th.ID)+c.suffix, c.body, c.headers...), http.StatusForbidden, "channel_read_only")
		if p["reason"] != "channel_archived" {
			t.Errorf("%s: %v", c.suffix, p)
		}
	}
	w.want(w.do("owner", http.MethodGet, threadPath(th.ID), ""), http.StatusOK, "")
	if got, _ := w.listIDs("owner", forum, nil); !slices.Equal(got, []string{th.ID.String()}) {
		t.Fatalf("list of an archived forum %v", got)
	}
}

// ptrTo returns a pointer to s.
func ptrTo(s string) *string { return &s }

// errCode returns the problem code of err, "" for none.
func errCode(err error) errs.Code {
	if e, ok := errs.As(err); ok {
		return e.Code
	}
	return ""
}

// wantField checks that err is validation_failed with the field error code on pointer.
func wantField(t *testing.T, err error, pointer, code string) {
	t.Helper()
	e, ok := errs.As(err)
	if !ok || e.Code != errs.CodeValidationFailed || len(e.Fields) != 1 || e.Fields[0].Pointer != pointer || e.Fields[0].Code != code {
		t.Fatalf("error %v, want validation_failed %s on %s", err, code, pointer)
	}
}
