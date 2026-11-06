// SPDX-License-Identifier: Apache-2.0

package runs_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// The tests of F-PROJEKT-BEREIT (09 3.8 and 18.16): a project forum needs a runner as its
// operator, a binding under another operator explains itself, and a message that failed for it is
// triggered again once the owner fixed the cause.

// bindProject binds forum as the owner to a new empty folder ref.
func (w *world) bindProject(forum ids.ID, ref string) {
	w.t.Helper()
	w.wantOK(w.do("owner", http.MethodPut, "/api/v1/channels/"+forum.String()+"/project-binding",
		`{"workspace_ref":"`+ref+`","provision":"init","reason":"New project"}`))
}

// fieldCode returns pointer and code of the first field error of the problem p.
func fieldCode(p map[string]any) (string, string) {
	list, _ := p["errors"].([]any)
	if len(list) == 0 {
		return "", ""
	}
	f, _ := list[0].(map[string]any)
	pointer, _ := f["pointer"].(string)
	code, _ := f["code"].(string)
	return pointer, code
}

// readyReason reads ready and ready_reason of the binding of forum through getChannelBehavior.
func (w *world) readyReason(forum ids.ID) (bool, string) {
	w.t.Helper()
	b := w.wantOK(w.do("owner", http.MethodGet, "/api/v1/channels/"+forum.String()+"/behavior", ""))
	pb, _ := b["project_binding"].(map[string]any)
	if pb == nil {
		w.t.Fatalf("no project binding in %v", b)
	}
	ready, _ := pb["ready"].(bool)
	reason, _ := pb["ready_reason"].(string)
	return ready, reason
}

// forumPost creates a forum post in forum as name and returns its thread and first message.
func (w *world) forumPost(name string, forum ids.ID, text string) (ids.ID, map[string]any) {
	w.t.Helper()
	b, err := json.Marshal(text)
	must(w.t, err)
	m := w.want(w.do(name, http.MethodPost, "/api/v1/channels/"+forum.String()+"/threads",
		`{"title":"Build","first_message":{"client_msg_id":"`+ids.New().String()+`","content_md":`+string(b)+`}}`, key()...), http.StatusCreated, "")
	return ids.MustParse(m["thread"].(map[string]any)["id"].(string)), m["first_message"].(map[string]any)
}

// TestProjectOperator_Refused (09 18.16 item 1): a bound forum refuses an operator that cannot
// serve a project, and a forum operated by such an agent cannot be bound; nothing is written.
func TestProjectOperator_Refused(t *testing.T) {
	t.Parallel()
	w := runsWorld(t)
	w.bindProject(w.projektA, "website")
	before := w.channelVersion(w.projektA)
	p := w.want(w.setChannelOperator(w.projektA, `{"mode":"agent","agent_id":"`+w.texter.String()+`","reason":"Chat agent"}`),
		http.StatusBadRequest, "validation_failed")
	if pointer, code := fieldCode(p); pointer != "/agent_id" || code != "operator_cannot_serve_project" {
		t.Fatalf("agent: %s %s", pointer, code)
	}
	p = w.want(w.setChannelOperator(w.projektA, `{"mode":"none","reason":"No operator"}`), http.StatusBadRequest, "validation_failed")
	if pointer, code := fieldCode(p); pointer != "/mode" || code != "operator_cannot_serve_project" {
		t.Fatalf("none: %s %s", pointer, code)
	}
	if after := w.channelVersion(w.projektA); after != before {
		t.Fatalf("a refused operator change wrote the channel: %d -> %d", before, after)
	}
	// A forum of the chat agent cannot be bound.
	chatForum := w.channel(w.engineering, "forum", "Projekt-C")
	w.wantOK(w.setChannelOperator(chatForum, `{"mode":"agent","agent_id":"`+w.texter.String()+`","reason":"Chat agent"}`))
	p = w.want(w.do("owner", http.MethodPut, "/api/v1/channels/"+chatForum.String()+"/project-binding",
		`{"workspace_ref":"chat","provision":"init","reason":"New project"}`), http.StatusBadRequest, "validation_failed")
	if pointer, code := fieldCode(p); pointer != "#path/channel_id" || code != "operator_cannot_serve_project" {
		t.Fatalf("binding: %s %s", pointer, code)
	}
	if n := w.count(`SELECT count(*) FROM project_bindings WHERE channel_id = $1`, chatForum.String()); n != 0 {
		t.Fatalf("a refused binding wrote %d rows", n)
	}
}

// TestProjectNotReady_ExplainedAndTriggeredAgain (09 3.8, 18.16 items 2 and 3; the reported
// case): a bound forum whose operator became a chat agent reads ready_reason
// operator_cannot_serve_project, a new post fails with that reason instead of a bare
// project_not_ready, and after the owner made the runner the operator again and the runner
// reported the project ready, the author triggers the post again: its thread takes the runner and
// a run opens; a second retrigger is 409, and nobody but the author may retrigger.
func TestProjectNotReady_ExplainedAndTriggeredAgain(t *testing.T) {
	t.Parallel()
	w := runsWorld(t)
	// The forum inherits from Engineering, whose default is the runner while the forum is bound.
	w.wantOK(w.setChannelOperator(w.projektA, `{"mode":"inherit","reason":"Inherit"}`))
	w.wantOK(w.setSpaceOperator(w.engineering, `{"mode":"agent","agent_id":"`+w.runner.String()+`","reason":"Runner"}`))
	w.bindProject(w.projektA, "website")
	// The space default changes to the chat agent: the space stays free (18.16 item 1).
	w.wantOK(w.setSpaceOperator(w.engineering, `{"mode":"agent","agent_id":"`+w.texter.String()+`","reason":"Chat agent"}`))
	if ready, reason := w.readyReason(w.projektA); ready || reason != "operator_cannot_serve_project" {
		t.Fatalf("readiness %v %q", ready, reason)
	}
	thread, first := w.forumPost("owner", w.projektA, "Set up the build")
	msg := first["id"].(string)
	if st, reason, _ := w.triggerOf(msg); st != "failed" || reason != "operator_cannot_serve_project" {
		t.Fatalf("trigger %s %s", st, reason)
	}
	if n := len(w.runsWhere(`thread_id = $1`, thread.String())); n != 0 {
		t.Fatalf("runs %d", n)
	}

	// The owner fixes the operator; the runner reports the project ready.
	w.wantOK(w.setSpaceOperator(w.engineering, `{"mode":"agent","agent_id":"`+w.runner.String()+`","reason":"Runner again"}`))
	must(t, w.chat.ApplyProjectReadiness(context.Background(), w.runner, []chat.ProjectReadiness{{ChannelID: w.projektA, Ready: true}}))
	if ready, reason := w.readyReason(w.projektA); !ready || reason != "" {
		t.Fatalf("readiness after the fix %v %q", ready, reason)
	}

	// Only the author may trigger the message again.
	p := w.want(w.do("robin", http.MethodPost, "/api/v1/messages/"+msg+"/retrigger", "", key()...), http.StatusForbidden, "forbidden")
	if p["reason"] != "not_self" {
		t.Fatalf("problem %v", p)
	}
	before := w.text(`SELECT operator_agent_id::text || ':' || version FROM threads WHERE id = $1`, thread.String())
	m := w.wantOK(w.do("owner", http.MethodPost, "/api/v1/messages/"+msg+"/retrigger", "", key()...))
	if m["id"] != msg || m["trigger_state"] != "received" {
		t.Fatalf("message after the retrigger %v", m)
	}
	after := w.text(`SELECT operator_agent_id::text || ':' || version FROM threads WHERE id = $1`, thread.String())
	if before == after || after[:36] != w.runner.String() {
		t.Fatalf("thread operator %s -> %s, want the runner", before, after)
	}
	rows := w.runsWhere(`thread_id = $1`, thread.String())
	if len(rows) != 1 || rows[0].Agent != w.runner.String() || rows[0].Triggering != msg || rows[0].State != "queued" {
		t.Fatalf("runs %+v", rows)
	}
	w.want(w.do("owner", http.MethodPost, "/api/v1/messages/"+msg+"/retrigger", "", key()...), http.StatusConflict, "invalid_state_transition")
}
