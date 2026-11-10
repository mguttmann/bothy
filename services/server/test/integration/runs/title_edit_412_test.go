// SPDX-License-Identifier: Apache-2.0

package runs_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// activeChat is the sequence of FIX-TITLE-EDIT-412, read from an instance in operation: Alex opens a
// forum post with the placeholder title of a new chat (title_source auto), and Lotse, the operator
// of the forum, runs for Alex's first message and answers in the thread. It returns the thread and
// the version a client holds that applied every thread object it was sent: the 201 of createThread
// and the thread events of the log.
func (w *titleWorld) activeChat() (thread string, held int64) {
	w.t.Helper()
	m := w.want(w.do("alex", http.MethodPost, "/api/v1/channels/"+w.fragen.String()+"/threads",
		`{"title":"Neuer Chat","first_message":{"client_msg_id":"`+ids.New().String()+`","content_md":"The export fails"}}`, key()...),
		http.StatusCreated, "")
	th := m["thread"].(map[string]any)
	if th["title_source"] != "auto" {
		w.t.Fatalf("the new chat is not auto: %v", th)
	}
	thread = th["id"].(string)
	held = w.heldVersion(thread, int64(th["version"].(float64)))
	w.agentAnswers(thread)
	return thread, held
}

// heldVersion returns the highest version of the thread among atLeast and the thread events of the
// log: what a client holds that is caught up.
func (w *titleWorld) heldVersion(thread string, atLeast int64) int64 {
	w.t.Helper()
	for _, e := range w.eventsAfter(0) {
		if strings.HasPrefix(e.Type, "thread.") && e.Resource == thread && e.Version > atLeast {
			atLeast = e.Version
		}
	}
	return atLeast
}

// agentAnswers lets Lotse run for the newest message of a person in the thread and answer there:
// the delivery, the start, an agent message and the completed end. The log must tell a client of
// the messages but not of the thread: the stored version moved by one per message.created of the
// thread (03 section 6.2) and by nothing else, and no thread event carries it.
func (w *titleWorld) agentAnswers(thread string) {
	w.t.Helper()
	rs := w.runsWhere(`thread_id = $1 AND state = 'queued'`, thread)
	if len(rs) != 1 {
		w.t.Fatalf("queued runs of the thread: %+v", rs)
	}
	_, _, stored := w.threadRow(thread)
	before := w.maxSeq()
	w.wantOK(w.start("lotse", rs[0].ID, w.deliver(rs[0].ID)))
	w.post("lotse", ids.MustParse(thread), "The March filter is the cause")
	w.wantOK(w.finish("lotse", rs[0].ID, `"state":"completed","summary":"Found the cause"`))
	var messages int64
	for _, e := range w.eventsAfter(before) {
		if strings.HasPrefix(e.Type, "thread.") && e.Resource == thread {
			w.t.Fatalf("%s tells clients of the new thread version", e.Type)
		}
		if e.Type == "message.created" && e.Thread == thread {
			messages++
		}
	}
	if _, _, now := w.threadRow(thread); messages == 0 || now != stored+messages {
		w.t.Fatalf("thread version %d after %d, with %d message.created of the thread", now, stored, messages)
	}
}

// TestSetThreadTitle_AfterAgentActivity (FIX-TITLE-EDIT-412, 06 section 8 point 2): every message
// raises the version of its thread (03 section 6.2) without a thread event, so the version a client
// holds in an active chat is behind the stored one. Its title edit names that version in If-Match
// and succeeds on the first try, because the title did not change since: the title becomes manual
// with one thread.updated.
func TestSetThreadTitle_AfterAgentActivity(t *testing.T) {
	t.Parallel()
	w := newTitleWorld(t)
	id, held := w.activeChat()
	_, _, stored := w.threadRow(id)
	if held >= stored {
		t.Fatalf("the client holds %d, the server stores %d: the sequence does not reproduce the report", held, stored)
	}
	r := w.setTitle("alex", id, "Export fails for March", held)
	if r.StatusCode != http.StatusOK {
		t.Fatalf("title edit with the held version %d against %d: %d %s", held, stored, r.StatusCode, r.Body)
	}
	if title, source, version := w.threadRow(id); title != "Export fails for March" || source != "manual" || version != stored+1 {
		t.Fatalf("stored %q %s %d", title, source, version)
	}
	if tag := r.Header.Get("ETag"); tag != `"`+strconvI(stored+1)+`"` {
		t.Fatalf("ETag %s", tag)
	}
	if evs := w.threadEvents("thread.updated", id); len(evs) != 1 || evs[0]["title"] != "Export fails for March" {
		t.Fatalf("thread.updated %v", evs)
	}
	// The next edit of the same client names the version of that answer, again behind after
	// another answer of the agent.
	w.post("alex", ids.MustParse(id), "And for April?")
	w.agentAnswers(id)
	if r := w.setTitle("alex", id, "Export fails for March and April", stored+1); r.StatusCode != http.StatusOK {
		t.Fatalf("second title edit: %d %s", r.StatusCode, r.Body)
	}
}

// TestSetThreadTitle_StaleTitleStillConflicts (FIX-TITLE-EDIT-412): the precondition still refuses
// an edit that is based on a title another person replaced in between. The owner sets a title in
// the active chat; Alex's client has not applied that thread.updated and sends the version it held
// before: 412 version_conflict with the current thread and no change. Sending again with the
// version of current succeeds, and a version the thread never had is refused.
func TestSetThreadTitle_StaleTitleStillConflicts(t *testing.T) {
	t.Parallel()
	w := newTitleWorld(t)
	id, held := w.activeChat()
	if r := w.setTitle("owner", id, "Export problem", held); r.StatusCode != http.StatusOK {
		t.Fatalf("the owner's title: %d %s", r.StatusCode, r.Body)
	}
	w.post("alex", ids.MustParse(id), "Still broken")
	w.agentAnswers(id)
	_, _, stored := w.threadRow(id)
	before := w.maxSeq()
	p := w.want(w.setTitle("alex", id, "Export fails for March", held), http.StatusPreconditionFailed, "version_conflict")
	current, _ := p["current"].(map[string]any)
	if current["title"] != "Export problem" || int64(current["version"].(float64)) != stored {
		t.Fatalf("current %v, stored version %d", current, stored)
	}
	w.want(w.setTitle("alex", id, "Export fails for March", stored+1), http.StatusPreconditionFailed, "version_conflict")
	if title, source, version := w.threadRow(id); title != "Export problem" || source != "manual" || version != stored {
		t.Fatalf("stored %q %s %d after the refusals", title, source, version)
	}
	if n := len(w.eventsAfter(before)); n != 0 {
		t.Fatalf("%d events after the refusals", n)
	}
	if r := w.setTitle("alex", id, "Export fails for March", stored); r.StatusCode != http.StatusOK {
		t.Fatalf("sending again with the current version: %d %s", r.StatusCode, r.Body)
	}
}

// TestMarkThreadUnread_AfterAgentActivity (FIX-TITLE-EDIT-412, 04 section 14): the other write of
// the thread view with If-Match names the version of the person's read state, which only the
// person's own writes raise. Messages of the agent leave it alone, so the version a client holds
// stays current in an active chat.
func TestMarkThreadUnread_AfterAgentActivity(t *testing.T) {
	t.Parallel()
	w := newTitleWorld(t)
	id, _ := w.activeChat()
	path := "/api/v1/threads/" + id + "/read-state"
	rs := w.wantOK(w.do("alex", http.MethodPut, path, `{"last_read_seq":2}`))
	held := int64(rs["version"].(float64))
	w.post("alex", ids.MustParse(id), "And for April?")
	w.agentAnswers(id)
	if stored := w.count(`SELECT version FROM read_states WHERE user_id = $1 AND thread_id = $2`, w.alex.String(), id); stored != held {
		t.Fatalf("the read state moved from %d to %d without a write of Alex", held, stored)
	}
	w.wantOK(w.do("alex", http.MethodPost, path+"/mark-unread", `{"last_read_seq":1}`, ifMatch(held)...))
}
