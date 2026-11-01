// SPDX-License-Identifier: Apache-2.0

package integration_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
)

// deleteChannel sends deleteChannel as name with trash_project_folder trash.
func (w *pbWorld) deleteChannel(name string, channel ids.ID, trash bool) *httpservertest.Response {
	w.t.Helper()
	body := `{"trash_project_folder":false}`
	if trash {
		body = `{"trash_project_folder":true}`
	}
	return w.do(name, http.MethodPost, "/api/v1/channels/"+channel.String()+"/delete", body, key()...)
}

// trashCheck reports the folder check of channel as the runner through a heartbeat.
func (w *pbWorld) trashCheck(channel ids.ID, check string) {
	w.t.Helper()
	w.wantOK(w.beat("runner", `{"state":"online","project_readiness":[{"channel_id":"`+channel.String()+`","ready":true,"trash_check":`+check+`}]}`))
}

// trashState returns trash_state:trash_reason:trash_agent_id of the binding of channel.
func (w *pbWorld) trashState(channel ids.ID) string {
	w.t.Helper()
	return w.text(`SELECT coalesce(trash_state, '-') || ':' || coalesce(trash_reason, '-') || ':' || coalesce(trash_agent_id::text, '-')
		FROM project_bindings WHERE channel_id = $1`, channel.String())
}

// purgeDue runs the chat part of messaging.purge_deleted for every due item (the forums of these
// tests hold no messages).
func (w *pbWorld) purgeDue() {
	w.t.Helper()
	due, err := w.chat.DuePurges(context.Background(), 10)
	must(w.t, err)
	for _, t := range due {
		must(w.t, w.chat.FinishPurge(context.Background(), t))
	}
}

// TestDeleteChannel_ProjectFolder (D-DEL-4, D-DEL-10, D-DEL-11; 09 18.15): moving the project
// folder needs channel.project.bind and an ok folder check of the serving runner; the request
// reaches only that runner and only once the deletion is final; its outcome is stored once and
// audited as channel.project.trash with the runner as actor.
func TestDeleteChannel_ProjectFolder(t *testing.T) {
	t.Parallel()
	w := newPBWorld(t)
	w.bind(w.projektA, "website")

	// alex has manage but not channel.project.bind.
	w.wantReason("trash without bind", w.deleteChannel("alex", w.projektA, true), http.StatusForbidden, "forbidden", "")
	// No check reported yet, then a refusing check.
	w.wantReason("no check", w.deleteChannel("owner", w.projektA, true), http.StatusConflict, "conflict", chat.ReasonProjectFolderRefused)
	w.trashCheck(w.projektA, `{"ok":false,"reason":"uncommitted_changes"}`)
	info := w.wantOK(w.do("owner", http.MethodGet, "/api/v1/channels/"+w.projektA.String()+"/deletion", ""))
	pf := info["project_folder"].(map[string]any)
	check := pf["trash_check"].(map[string]any)
	if pf["workspace_ref"] != "website" || check["ok"] != false || check["reason"] != "uncommitted_changes" || check["checked_at"] == nil ||
		info["state"] != "live" {
		t.Fatalf("deletion info %v", info)
	}
	if got := w.wantOK(w.do("alex", http.MethodGet, "/api/v1/channels/"+w.projektA.String()+"/deletion", "")); got["project_folder"] != nil {
		t.Fatalf("a person who is not an owner sees the folder: %v", got)
	}
	w.wantReason("refused check", w.deleteChannel("owner", w.projektA, true), http.StatusConflict, "conflict", chat.ReasonProjectFolderRefused)
	if w.trashState(w.projektA) != "-:-:-" {
		t.Fatalf("a refused delete wrote a request: %s", w.trashState(w.projektA))
	}

	w.trashCheck(w.projektA, `{"ok":true}`)
	got := w.wantOK(w.deleteChannel("owner", w.projektA, true))
	if got["state"] != "pending" || got["project_folder"].(map[string]any)["trash_state"] != "requested" {
		t.Fatalf("delete %v", got)
	}
	if want := "requested:-:" + w.runner.String(); w.trashState(w.projektA) != want {
		t.Fatalf("binding %s, want %s", w.trashState(w.projektA), want)
	}
	if rows := w.allows("channel.delete"); len(rows) != 1 || rows[0].Details["trash_project_folder"] != true {
		t.Fatalf("audit %+v", rows)
	}
	// Not handed out while the deletion can be undone, nor after the window before the purge.
	if ack := w.wantOK(w.beat("runner", `{"state":"online"}`)); ack["project_trash"] != nil {
		t.Fatalf("handed out while pending: %v", ack)
	}
	w.clk.Advance(31 * time.Second)
	if ack := w.wantOK(w.beat("runner", `{"state":"online"}`)); ack["project_trash"] != nil {
		t.Fatalf("handed out before the purge: %v", ack)
	}
	w.purgeDue()
	if ack := w.wantOK(w.beat("lotse", `{"state":"online"}`)); ack["project_trash"] != nil {
		t.Fatalf("handed to another runner: %v", ack)
	}
	ack := w.wantOK(w.beat("runner", `{"state":"online"}`))
	list, _ := ack["project_trash"].([]any)
	if len(list) != 1 || list[0].(map[string]any)["channel_id"] != w.projektA.String() || list[0].(map[string]any)["workspace_ref"] != "website" {
		t.Fatalf("project_trash %v", ack["project_trash"])
	}

	// Foreign and unknown results are ignored; the outcome is stored once and audited.
	w.wantOK(w.beat("lotse", `{"state":"online","project_trash_results":[{"channel_id":"`+w.projektA.String()+`","result":"trashed"}]}`))
	w.wantOK(w.beat("runner", `{"state":"online","project_trash_results":[{"channel_id":"`+w.projektA.String()+
		`","result":"refused","reason":"unpushed_commits"},{"channel_id":"`+ids.New().String()+`","result":"trashed"}]}`))
	w.wantOK(w.beat("runner", `{"state":"online","project_trash_results":[{"channel_id":"`+w.projektA.String()+`","result":"trashed"}]}`))
	if want := "refused:unpushed_commits:" + w.runner.String(); w.trashState(w.projektA) != want {
		t.Fatalf("binding %s, want %s", w.trashState(w.projektA), want)
	}
	rows := w.allows("channel.project.trash")
	if len(rows) != 1 || rows[0].ResourceID != w.projektA.String() || rows[0].Details["result"] != "refused" ||
		rows[0].Details["reason_code"] != "unpushed_commits" {
		t.Fatalf("audit %+v", rows)
	}
	if actor := w.text(`SELECT actor_principal_id::text FROM audit_events WHERE action = 'channel.project.trash'`); actor != w.runner.String() {
		t.Fatalf("audit actor %s", actor)
	}
	if ack := w.wantOK(w.beat("runner", `{"state":"online"}`)); ack["project_trash"] != nil {
		t.Fatalf("handed out after the outcome: %v", ack)
	}
	final := w.wantOK(w.do("owner", http.MethodGet, "/api/v1/channels/"+w.projektA.String()+"/deletion", ""))
	if fpf := final["project_folder"].(map[string]any); final["state"] != "final" || fpf["trash_state"] != "refused" || fpf["trash_reason"] != "unpushed_commits" {
		t.Fatalf("final deletion %v", final)
	}
}

// TestDeleteChannel_FolderFlagOffAndRestore: a deletion without the folder flag leaves
// trash_state NULL; a restore within the undo window drops a requested move.
func TestDeleteChannel_FolderFlagOffAndRestore(t *testing.T) {
	t.Parallel()
	w := newPBWorld(t)
	w.bind(w.projektA, "website")
	w.bind(w.projektB, "docs")
	w.trashCheck(w.projektB, `{"ok":true}`)
	w.wantOK(w.deleteChannel("owner", w.projektA, false))
	if w.trashState(w.projektA) != "-:-:-" {
		t.Fatalf("a delete without the flag wrote a request: %s", w.trashState(w.projektA))
	}
	w.wantOK(w.deleteChannel("owner", w.projektB, true))
	w.wantOK(w.do("owner", http.MethodPost, "/api/v1/channels/"+w.projektB.String()+"/restore", "", key()...))
	if w.trashState(w.projektB) != "-:-:-" {
		t.Fatalf("a restore kept the request: %s", w.trashState(w.projektB))
	}
}
