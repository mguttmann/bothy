// SPDX-License-Identifier: Apache-2.0

package runs_test

import (
	"net/http"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// helpdeskForum adds the forum Marketing/Anfragen: its effective operator is Texter, which holds no
// grant there (write only through the operator rule, 07 section 2.9), and Sam has trigger on the
// space. Posts answer without a mention, as the support desk forum does.
func (w *titleWorld) helpdeskForum() string {
	w.t.Helper()
	forum := w.channel(w.marketing, "forum", "Anfragen")
	w.behavior(forum, "")
	return forum.String()
}

// operatorPost sends createThread as the operator agent Texter in forum with a first message, the
// way the Hermes adapter opens a forum post for a report, and returns the thread of the 201 body.
func (w *titleWorld) operatorPost(forum string, title string) map[string]any {
	w.t.Helper()
	m := w.want(w.do("texter", http.MethodPost, "/api/v1/channels/"+forum+"/threads",
		`{"title":"`+title+`","first_message":{"client_msg_id":"`+ids.New().String()+`","content_md":"New ticket"}}`, key()...), http.StatusCreated, "")
	return m["thread"].(map[string]any)
}

// threadState reads the state and closed_by of a thread.
func (w *titleWorld) threadState(id string) (state, closedBy string) {
	w.t.Helper()
	return w.text(`SELECT state FROM threads WHERE id = $1`, id),
		w.text(`SELECT coalesce(closed_by::text, '') FROM threads WHERE id = $1`, id)
}

// TestOperatorAgentTicketFlow (FEAT-hermes-channel-skills): the support desk flow of an
// operator agent in its forum. The agent opens a post, sets its status title more than once,
// marks it done (the state of the app's Done button), a person's answer in the done post is
// accepted, reopens it and starts a run of the agent, the agent writes into a done post and
// reopens one explicitly.
func TestOperatorAgentTicketFlow(t *testing.T) {
	t.Parallel()
	w := newTitleWorld(t)
	th := w.operatorPost(w.helpdeskForum(), "[OPEN] Printer on floor 2")
	id := th["id"].(string)
	if th["title_source"] != "auto" {
		t.Fatalf("operator post %v", th)
	}

	// Status titles: the operator agent replaces its own auto title again and again.
	for _, title := range []string{"[IN PROGRESS] Printer on floor 2", "[DONE] Printer on floor 2"} {
		_, _, v := w.threadRow(id)
		got := w.wantOK(w.setTitle("texter", id, title, v))
		if got["title"] != title || got["title_source"] != "auto" {
			t.Fatalf("title %v", got)
		}
	}

	// Done: the same state as the Done button of the app, closed by the agent.
	closed := w.wantOK(w.do("texter", http.MethodPost, "/api/v1/threads/"+id+"/close", "", key()...))
	if closed["state"] != "closed" {
		t.Fatalf("close %v", closed)
	}
	if state, by := w.threadState(id); state != "closed" || by != w.texter.String() {
		t.Fatalf("after close %s %s", state, by)
	}
	if n := len(w.threadEvents("thread.closed", id)); n != 1 {
		t.Fatalf("%d thread.closed", n)
	}

	// A person's answer in the done post: accepted, the post reopens, and the agent gets a run.
	answer := w.post("sam", ids.MustParse(id), "It fails again")["id"].(string)
	if state, _ := w.threadState(id); state != "open" {
		t.Fatalf("after the answer %s", state)
	}
	if r, ok := w.runOf(answer); !ok || r.Agent != w.texter.String() {
		t.Fatalf("no run of the operator for the answer: %+v %v", r, ok)
	}
}

// TestOperatorAgentWritesAndReopensDonePost (FEAT-hermes-channel-skills): closing never blocks the
// operator agent: a report into a done post is accepted and reopens it, and the agent reopens a
// done post explicitly (the "important answer" path of the support desk flow).
func TestOperatorAgentWritesAndReopensDonePost(t *testing.T) {
	t.Parallel()
	w := newTitleWorld(t)
	id := w.operatorPost(w.helpdeskForum(), "[OPEN] Mailbox full")["id"].(string)

	w.wantOK(w.do("texter", http.MethodPost, "/api/v1/threads/"+id+"/close", "", key()...))
	w.post("texter", ids.MustParse(id), "The requester answered: it works again")
	if state, _ := w.threadState(id); state != "open" {
		t.Fatalf("after the agent's message %s", state)
	}

	w.wantOK(w.do("texter", http.MethodPost, "/api/v1/threads/"+id+"/close", "", key()...))
	reopened := w.wantOK(w.do("texter", http.MethodPost, "/api/v1/threads/"+id+"/reopen", "", key()...))
	if reopened["state"] != "open" {
		t.Fatalf("reopen %v", reopened)
	}
	if n := len(w.threadEvents("thread.reopened", id)); n != 2 {
		t.Fatalf("%d thread.reopened", n)
	}
}
