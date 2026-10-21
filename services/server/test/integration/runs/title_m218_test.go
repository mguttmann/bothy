// SPDX-License-Identifier: Apache-2.0

package runs_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"sync"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

// titleWorld is runsWorld plus the forum Support/Fragen of the brief M2-18 ("Integration"): it
// inherits Lotse as operator, Sam and Texter have write on it.
type titleWorld struct {
	*world
	fragen ids.ID
}

// newTitleWorld builds the world of the M2-18 tests over runsWorld.
func newTitleWorld(t *testing.T) *titleWorld {
	t.Helper()
	return withFragen(runsWorld(t))
}

// withFragen adds Support/Fragen with write for Sam and Texter to w.
func withFragen(w *world) *titleWorld {
	w.t.Helper()
	tw := &titleWorld{world: w, fragen: w.channel(w.support, "forum", "Fragen")}
	for _, who := range []struct{ kind, id string }{{"user", w.sam.String()}, {"agent", w.texter.String()}} {
		w.grant(`"subject_kind":"` + who.kind + `","subject_id":"` + who.id + `","target_kind":"channel","target_id":"` + tw.fragen.String() +
			`","level":"write","reason":"Questions"`)
	}
	return tw
}

// createPost sends createThread as Sam in forum with a first message (a forum post needs one);
// title nil sends no title. It returns the thread of the 201 body.
func (w *titleWorld) createPost(forum ids.ID, title *string) map[string]any {
	w.t.Helper()
	body := `{"first_message":{"client_msg_id":"` + ids.New().String() + `","content_md":"Details follow"}`
	if title != nil {
		b, _ := json.Marshal(*title)
		body += `,"title":` + string(b)
	}
	body += "}"
	m := w.want(w.do("sam", http.MethodPost, "/api/v1/channels/"+forum.String()+"/threads", body, key()...), http.StatusCreated, "")
	return m["thread"].(map[string]any)
}

// setTitle sends setThreadTitle as name with If-Match version.
func (w *titleWorld) setTitle(name, thread, title string, version int64) *httpservertest.Response {
	w.t.Helper()
	b, _ := json.Marshal(title)
	return w.do(name, http.MethodPut, "/api/v1/threads/"+thread+"/title", `{"title":`+string(b)+`}`, ifMatch(version)...)
}

// threadRow is the stored title, source and version of a thread.
func (w *titleWorld) threadRow(id string) (title, source string, version int64) {
	w.t.Helper()
	must(w.t, w.db.QueryRow(context.Background(), `SELECT title, title_source, version FROM threads WHERE id = $1`, id).Scan(&title, &source, &version))
	return title, source, version
}

// threadEvents returns the payload threads of the events typ about thread id.
func (w *titleWorld) threadEvents(typ, id string) []map[string]any {
	w.t.Helper()
	var out []map[string]any
	for _, p := range w.eventsOf(typ, id) {
		out = append(out, p["thread"].(map[string]any))
	}
	return out
}

// ptrTo returns a pointer to s.
func ptrTo(s string) *string { return &s }

// TestCreateForumPost_TitleSource (plan rows "placeholders with a number detected" and "a title
// set by a person becomes manual"): Sam's real title gives manual, no title and placeholders with
// and without a number give auto; the title is stored as sent and thread.created carries the same
// source.
func TestCreateForumPost_TitleSource(t *testing.T) {
	t.Parallel()
	w := newTitleWorld(t)
	for _, c := range []struct {
		title *string
		want  string
	}{
		{ptrTo("Invoice export fails"), "manual"},
		{nil, "auto"},
		{ptrTo("New chat 2"), "auto"},
		{ptrTo("Neuer Chat 3"), "auto"},
		{ptrTo("  ohne   titel  "), "auto"},
	} {
		th := w.createPost(w.fragen, c.title)
		id := th["id"].(string)
		sent := ""
		if c.title != nil {
			sent = *c.title
		}
		if th["title_source"] != c.want || th["title"] != sent {
			t.Fatalf("%q: %v", sent, th)
		}
		if title, source, _ := w.threadRow(id); title != sent || source != c.want {
			t.Fatalf("%q: stored %q %s", sent, title, source)
		}
		evs := w.threadEvents("thread.created", id)
		if len(evs) != 1 || evs[0]["title_source"] != c.want || evs[0]["title"] != sent {
			t.Fatalf("%q: thread.created %v", sent, evs)
		}
	}
}

// TestCreateForumPost_ProtectedShortNamesManual (plan row "protected short names not treated as
// placeholders", docs/REUSE.md L23): Sam's posts "AI", "C#" and "PC 2" give manual.
func TestCreateForumPost_ProtectedShortNamesManual(t *testing.T) {
	t.Parallel()
	w := newTitleWorld(t)
	for _, name := range []string{"AI", "C#", "PC 2"} {
		if th := w.createPost(w.fragen, ptrTo(name)); th["title_source"] != "manual" {
			t.Fatalf("%q: %v", name, th)
		}
	}
}

// TestCreateThreadFromMessage_TitleSource (Ruling D-M2-18-5): the placeholder rule applies to a
// person's thread from a message too.
func TestCreateThreadFromMessage_TitleSource(t *testing.T) {
	t.Parallel()
	w := newTitleWorld(t)
	for title, want := range map[string]string{"Chat 3": "auto", "Release plan": "manual"} {
		parent := w.post("robin", w.mainThread(w.eskalation), "Question")
		b, _ := json.Marshal(title)
		m := w.want(w.do("robin", http.MethodPost, "/api/v1/channels/"+w.eskalation.String()+"/threads",
			`{"parent_message_id":"`+parent["id"].(string)+`","title":`+string(b)+`}`, key()...), http.StatusCreated, "")
		if th := m["thread"].(map[string]any); th["title_source"] != want || th["title"] != title {
			t.Fatalf("%q: %v", title, th)
		}
	}
}

// TestSetThreadTitle_PersonAlwaysManual (Ruling D-M2-18-6): on Sam's auto post Sam sets a
// placeholder text and the source becomes manual, with one thread.updated.
func TestSetThreadTitle_PersonAlwaysManual(t *testing.T) {
	t.Parallel()
	w := newTitleWorld(t)
	th := w.createPost(w.fragen, nil)
	id := th["id"].(string)
	r := w.setTitle("sam", id, "Neuer Chat 2", int64(th["version"].(float64)))
	if r.StatusCode != http.StatusOK {
		t.Fatalf("setThreadTitle %d %s", r.StatusCode, r.Body)
	}
	if title, source, _ := w.threadRow(id); title != "Neuer Chat 2" || source != "manual" {
		t.Fatalf("stored %q %s", title, source)
	}
	if evs := w.threadEvents("thread.updated", id); len(evs) != 1 || evs[0]["title_source"] != "manual" {
		t.Fatalf("thread.updated %v", evs)
	}
}

// autoThreadAfterRun posts Alex's mention of Lotse in the main thread of Support/Eskalation, which
// creates the auto thread T' (M2-05), and lets Lotse's adapter code take the delivery, start the
// run with the delivered token and finish it completed. It returns T'.
func (w *titleWorld) autoThreadAfterRun() string {
	w.t.Helper()
	msg := w.post("alex", w.mainThread(w.eskalation), "@lotse the export fails")["id"].(string)
	r, ok := w.runOf(msg)
	if !ok {
		w.t.Fatal("the mention opened no run")
	}
	if title, source, _ := w.threadRow(r.Thread); title != "" || source != "auto" {
		w.t.Fatalf("T' %q %s", title, source)
	}
	tok := w.deliver(r.ID)
	w.wantOK(w.start("lotse", r.ID, tok))
	w.wantOK(w.finish("lotse", r.ID, `"state":"completed","summary":"Found the cause"`))
	if got := w.lifecycle(r.ID); got.State != "completed" {
		w.t.Fatalf("run %s", got.State)
	}
	w.noTokenAnywhere(tok)
	return r.Thread
}

// etag reads getThread as name and returns the version of its ETag.
func (w *titleWorld) etag(name, thread string) int64 {
	w.t.Helper()
	r := w.do(name, http.MethodGet, "/api/v1/threads/"+thread, "")
	w.want(r, http.StatusOK, "")
	tag := r.Header.Get("ETag")
	v, err := strconv.ParseInt(tag[1:len(tag)-1], 10, 64)
	must(w.t, err)
	return v
}

// TestAG19_OperatorAgentTitleAfterRun (AG-19, T-AI-04, RK-01): after a real run the operator agent
// replaces the auto title twice and the source stays auto; after the creator's title it gets
// 409 title_manual and nothing changes; with the old If-Match it gets 412 version_conflict.
func TestAG19_OperatorAgentTitleAfterRun(t *testing.T) {
	t.Parallel()
	w := newTitleWorld(t)
	th := w.autoThreadAfterRun()
	for _, title := range []string{"Export fails", "Export fails since Monday"} {
		v := w.etag("lotse", th)
		r := w.setTitle("lotse", th, title, v)
		if r.StatusCode != http.StatusOK {
			t.Fatalf("%q: %d %s", title, r.StatusCode, r.Body)
		}
		if got, source, version := w.threadRow(th); got != title || source != "auto" || version != v+1 {
			t.Fatalf("%q: stored %q %s %d", title, got, source, version)
		}
	}
	if evs := w.threadEvents("thread.updated", th); len(evs) != 2 || evs[1]["title_source"] != "auto" {
		t.Fatalf("thread.updated %v", evs)
	}
	old := w.etag("alex", th)
	if r := w.setTitle("alex", th, "Invoice export", old); r.StatusCode != http.StatusOK {
		t.Fatalf("creator title %d %s", r.StatusCode, r.Body)
	}
	cur := w.etag("lotse", th)
	before := w.maxSeq()
	w.want(w.do("lotse", http.MethodPut, "/api/v1/threads/"+th+"/title", `{"title":"Better"}`, ifMatch(cur)...), http.StatusConflict, "title_manual")
	w.want(w.do("lotse", http.MethodPut, "/api/v1/threads/"+th+"/title", `{"title":"Better"}`, ifMatch(old)...), http.StatusPreconditionFailed, "version_conflict")
	if title, source, version := w.threadRow(th); title != "Invoice export" || source != "manual" || version != cur {
		t.Fatalf("stored %q %s %d", title, source, version)
	}
	if n := len(w.eventsAfter(before)); n != 0 {
		t.Fatalf("%d events after the refusals", n)
	}
}

// TestGuestAgentSetsNoTitle (09 section 12 point 4, T-AI-04): after Lotse's run the owner mentions
// Texter in T', which opens a guest run; Texter's title is 403 forbidden there and 404 on a thread
// it cannot read, the same answer as an unknown thread; nothing changes.
func TestGuestAgentSetsNoTitle(t *testing.T) {
	t.Parallel()
	w := newTitleWorld(t)
	th := w.autoThreadAfterRun()
	// agent.read lets a person mention only agents that operate a channel the person reads, so the
	// mention of Texter (operator of Marketing) comes from the owner, as in the acceptance tests
	// of M2-05 (D-M2-05-22).
	w.post("owner", ids.MustParse(th), "@texter can you help")
	if rs := w.runsWhere(`thread_id = $1 AND agent_id = $2`, th, w.texter.String()); len(rs) != 1 {
		t.Fatalf("guest runs %+v", rs)
	}
	list := w.wantOK(w.do("alex", http.MethodGet, "/api/v1/threads/"+th+"/runs", ""))
	if items := list["items"].([]any); len(items) != 2 {
		t.Fatalf("listThreadRuns %v", list)
	}
	hidden := w.newThread(w.allgemein).String()
	hv := w.etag("owner", hidden)
	v := w.etag("alex", th)
	before := w.maxSeq()
	deniesBefore := len(w.denies("thread.title.update", "not_self"))
	w.want(w.do("texter", http.MethodPut, "/api/v1/threads/"+th+"/title", `{"title":"Mine"}`, ifMatch(v)...), http.StatusForbidden, "forbidden")
	if got := len(w.denies("thread.title.update", "not_self")); got != deniesBefore+1 {
		t.Fatalf("not_self denials %d, want %d", got, deniesBefore+1)
	}
	a := w.do("texter", http.MethodPut, "/api/v1/threads/"+hidden+"/title", `{"title":"Mine"}`, ifMatch(hv)...)
	b := w.do("texter", http.MethodPut, "/api/v1/threads/"+ids.New().String()+"/title", `{"title":"Mine"}`, ifMatch(hv)...)
	pa, pb := w.want(a, http.StatusNotFound, "not_found"), w.want(b, http.StatusNotFound, "not_found")
	// The instance path names the thread id and the correlation id the request.
	for _, p := range []map[string]any{pa, pb} {
		delete(p, "instance")
		delete(p, "correlation_id")
	}
	if ja, jb := string(mustJSON(t, pa)), string(mustJSON(t, pb)); ja != jb || a.Header.Get("Content-Type") != b.Header.Get("Content-Type") {
		t.Fatalf("404 bodies differ: %s / %s", ja, jb)
	}
	if title, source, version := w.threadRow(th); title != "" || source != "auto" || version != v {
		t.Fatalf("T' changed: %q %s %d", title, source, version)
	}
	if n := len(w.eventsAfter(before)); n != 0 {
		t.Fatalf("%d events after the refusals", n)
	}
}

// TestSetThreadTitle_CreatorAgentSetsManual (Ruling D-M2-18-7): Texter creates a post through
// chat.Threads.Create as messaging would: auto; Texter's title change gives manual, and Lotse,
// the thread's operator, then gets 409 title_manual.
func TestSetThreadTitle_CreatorAgentSetsManual(t *testing.T) {
	t.Parallel()
	w := newTitleWorld(t)
	a := w.actor("texter")
	var th chat.Thread
	must(t, w.uow.Do(actor.WithActor(context.Background(), a), func(ctx context.Context) error {
		var err error
		th, err = w.chat.Create(ctx, a, chat.CreateThreadCmd{ChannelID: w.fragen, Title: ptrTo("Release notes")})
		return err
	}))
	if th.TitleSource != "auto" || th.OperatorAgentID != w.lotse {
		t.Fatalf("post %+v", th)
	}
	if r := w.setTitle("texter", th.ID.String(), "Release notes 2", th.Version); r.StatusCode != http.StatusOK {
		t.Fatalf("creator title %d %s", r.StatusCode, r.Body)
	}
	if _, source, _ := w.threadRow(th.ID.String()); source != "manual" {
		t.Fatalf("source %s", source)
	}
	w.want(w.do("lotse", http.MethodPut, "/api/v1/threads/"+th.ID.String()+"/title", `{"title":"Summary"}`, ifMatch(th.Version+1)...),
		http.StatusConflict, "title_manual")
}

// TestSetThreadTitle_ConcurrentPersonAndOperatorAgent (T-AI-04): Sam and Lotse change the title of
// an auto post with the same If-Match at the same time, many rounds: one 200 and one 412 each;
// the source is manual exactly when Sam won and never turns back to auto.
func TestSetThreadTitle_ConcurrentPersonAndOperatorAgent(t *testing.T) {
	t.Parallel()
	w := newTitleWorld(t)
	const rounds = 20
	for i := range rounds {
		th := w.createPost(w.fragen, nil)
		id, v := th["id"].(string), int64(th["version"].(float64))
		codes := map[string]int{}
		var mu sync.Mutex
		var wg sync.WaitGroup
		for _, name := range []string{"sam", "lotse"} {
			wg.Add(1)
			go func() {
				defer wg.Done()
				r := w.do(name, http.MethodPut, "/api/v1/threads/"+id+"/title", `{"title":"Title of `+name+`"}`, ifMatch(v)...)
				mu.Lock()
				codes[name] = r.StatusCode
				mu.Unlock()
			}()
		}
		wg.Wait()
		ok, conflict := 0, 0
		for _, c := range codes {
			switch c {
			case http.StatusOK:
				ok++
			case http.StatusPreconditionFailed:
				conflict++
			}
		}
		if ok != 1 || conflict != 1 {
			t.Fatalf("round %d: %v", i, codes)
		}
		_, source, _ := w.threadRow(id)
		if (source == "manual") != (codes["sam"] == http.StatusOK) {
			t.Fatalf("round %d: source %s with %v", i, source, codes)
		}
		// The loser retries with the current version: Lotse never turns a manual title auto.
		cur := w.etag("sam", id)
		r := w.do("lotse", http.MethodPut, "/api/v1/threads/"+id+"/title", `{"title":"Late"}`, ifMatch(cur)...)
		if source == "manual" && r.StatusCode != http.StatusConflict {
			t.Fatalf("round %d: Lotse on manual %d", i, r.StatusCode)
		}
		if _, after, _ := w.threadRow(id); source == "manual" && after != "manual" {
			t.Fatalf("round %d: manual turned %s", i, after)
		}
	}
}

// TestPlaceholderTitles_ConfiguredList (brief M2-18 note 3c): a server whose
// thread.placeholder_titles is Draft in the configuration source of the harness (never the
// process environment) treats "Draft 2" as a placeholder and "Neuer Chat" as a real title.
func TestPlaceholderTitles_ConfiguredList(t *testing.T) {
	t.Parallel()
	schema, err := wire.ConfigSchema()
	must(t, err)
	v, err := config.Load(schema, config.Sources{Env: []string{"BOTHY_SERVER__PUBLIC_URL=https://chat.example.com", "BOTHY_THREAD__PLACEHOLDER_TITLES=Draft"}})
	must(t, err)
	loaded, err := wire.ChatConfig(v)
	must(t, err)
	cfg := chatConfig
	cfg.PlaceholderTitles = loaded.PlaceholderTitles
	w := withFragen(runsWorldWith(t, cfg))
	if th := w.createPost(w.fragen, ptrTo("Draft 2")); th["title_source"] != "auto" {
		t.Fatalf("Draft 2: %v", th)
	}
	if th := w.createPost(w.fragen, ptrTo("Neuer Chat")); th["title_source"] != "manual" {
		t.Fatalf("Neuer Chat: %v", th)
	}
}
