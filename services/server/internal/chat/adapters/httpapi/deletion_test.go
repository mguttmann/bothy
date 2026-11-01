// SPDX-License-Identifier: Apache-2.0

package httpapi_test

import (
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
)

// undoWindow is the default of chat.delete_undo_window, which the world leaves unset.
const undoWindow = 30 * time.Second

// postEmpty sends POST path as name without a body and wants 200.
func (w *world) postEmpty(name, path string) map[string]any {
	w.t.Helper()
	return w.want(w.do(name, http.MethodPost, path, "", key()...), http.StatusOK, "")
}

// wantConflict checks a 409 conflict with reason.
func (w *world) wantConflict(r *httpservertest.Response, reason string) {
	w.t.Helper()
	if r.StatusCode != http.StatusConflict {
		w.t.Fatalf("status %d, want 409: %s", r.StatusCode, r.Body)
	}
	if p := r.Problem(w.t); p["code"] != "conflict" || p["reason"] != reason {
		w.t.Fatalf("problem %v, want conflict/%s", p, reason)
	}
}

// TestDeleteSpace (07 4.5a, D-DEL-2, D-DEL-4, D-DEL-5, D-DEL-9): only an owner with step-up deletes
// a space; the delete is a rights change, travels as space.archived with deleted_at to every
// reader, is audited, and a second delete answers the same deletion. A restore within the undo
// window brings the space back with space.updated without deleted_at; after the window it is 409
// undo_window_passed.
func TestDeleteSpace(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	alex, sam := w.person("alex", false), w.person("sam", false)
	w.session("owner-stale", w.ownerID, start.Add(-time.Hour))
	support := w.create("Support")
	w.grant(alex, support, "trigger")
	w.grant(sam, support, "read")
	path := spacePath(support)

	w.want(w.do("alex", http.MethodPost, path+"/delete", "", key()...), http.StatusForbidden, "forbidden")
	w.want(w.do("owner-stale", http.MethodPost, path+"/delete", "", key()...), http.StatusUnauthorized, "step_up_required")
	v := w.permissionVersion()
	info := w.postEmpty("owner", path+"/delete")
	deletedAt, _ := time.Parse(time.RFC3339, info["deleted_at"].(string))
	undo, _ := time.Parse(time.RFC3339, info["undo_until"].(string))
	if info["state"] != "pending" || info["target_kind"] != "space" || info["target_id"] != support.String() || !undo.Equal(deletedAt.Add(undoWindow)) {
		t.Fatalf("deletion %v", info)
	}
	if w.permissionVersion() != v+1 {
		t.Fatal("the delete is not a rights change")
	}
	evs := w.events("space.archived")
	if len(evs) != 1 || evs[0].ReceiveAction != "space.read" || evs[0].Payload["space"].(map[string]any)["deleted_at"] == nil ||
		evs[0].Payload["space"].(map[string]any)["archived_at"] == nil {
		t.Fatalf("space.archived %+v", evs)
	}
	if a := w.allows("space.delete"); len(a) != 1 || a[0].ResourceID != support.String() {
		t.Fatalf("audit %+v", a)
	}
	for _, who := range []string{"alex", "sam"} {
		got := w.want(w.do(who, http.MethodGet, path, ""), http.StatusOK, "")
		if got["space"].(map[string]any)["deleted_at"] == nil || slices.Contains(w.listed(who), "Support") {
			t.Errorf("%s: %v", who, got)
		}
	}
	if again := w.postEmpty("owner", path+"/delete"); again["deleted_at"] != info["deleted_at"] || len(w.events("space.archived")) != 1 {
		t.Fatalf("second delete %v", again)
	}

	// The deleted item is archived, so a member without the right meets the archive cap first.
	w.want(w.do("alex", http.MethodPost, path+"/restore", "", key()...), http.StatusForbidden, "")
	restored := w.postEmpty("owner", path+"/restore")
	if _, ok := restored["deleted_at"]; ok || restored["archived_at"] != nil || restored["version"].(float64) <= evs[0].Payload["space"].(map[string]any)["version"].(float64) {
		t.Fatalf("restored %v", restored)
	}
	upd := w.events("space.updated")
	if len(upd) == 0 || upd[len(upd)-1].Payload["space"].(map[string]any)["deleted_at"] != nil {
		t.Fatalf("space.updated %+v", upd)
	}
	if a := w.allows("space.restore"); len(a) != 1 {
		t.Fatalf("audit %+v", a)
	}
	if !slices.Contains(w.listed("alex"), "Support") {
		t.Fatal("the restored space is not listed")
	}
	if again := w.postEmpty("owner", path+"/restore"); again["version"] != restored["version"] {
		t.Fatalf("a restore of a live space changed it: %v", again)
	}

	w.postEmpty("owner", path+"/delete")
	w.clk.Advance(undoWindow)
	r := w.do("owner", http.MethodPost, path+"/restore", "", key()...)
	w.wantConflict(r, chat.ReasonUndoWindowPassed)
	if got := w.want(w.do("alex", http.MethodGet, path+"/deletion", ""), http.StatusOK, ""); got["state"] != "final" {
		t.Fatalf("deletion after the window %v", got)
	}
}

// TestDeleteSpace_ArchivedBeforeStaysArchived (D-DEL-2): a space archived before its deletion stays
// archived after the restore.
func TestDeleteSpace_ArchivedBeforeStaysArchived(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	support := w.create("Support")
	archived := w.want(w.do("owner", http.MethodPost, spacePath(support)+"/archive", `{"reason":"Old"}`, key()...), http.StatusOK, "")
	w.clk.Advance(time.Second)
	w.postEmpty("owner", spacePath(support)+"/delete")
	restored := w.postEmpty("owner", spacePath(support)+"/restore")
	if restored["archived_at"] != archived["archived_at"] {
		t.Fatalf("archive before %v, after the restore %v", archived["archived_at"], restored["archived_at"])
	}
}

// TestDeleteChannel (D-DEL-4, D-DEL-5): manage with step-up deletes a channel; a member without
// manage gets 403; the deletion travels as channel.archived with deleted_at to the readers; a
// folder move of a channel without a binding is refused; a restore within the window gives
// channel.updated, and while the space is deleted it is 409 space_deleted.
func TestDeleteChannel(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	alex, sam := w.person("alex", false), w.person("sam", false)
	w.session("owner-stale", w.ownerID, start.Add(-time.Hour))
	support := w.create("Support")
	ch := w.channel(support, "text", "general")
	w.grant(alex, support, "trigger")
	w.manage(sam, support)
	path := channelPath(ch)

	w.want(w.do("alex", http.MethodPost, path+"/delete", "", key()...), http.StatusForbidden, "forbidden")
	w.want(w.do("owner-stale", http.MethodPost, path+"/delete", "", key()...), http.StatusUnauthorized, "step_up_required")
	w.want(w.do("sam", http.MethodPost, path+"/delete", `{"trash_project_folder":true}`, key()...), http.StatusBadRequest, "validation_failed")
	info := w.want(w.do("sam", http.MethodPost, path+"/delete", `{"trash_project_folder":false}`, key()...), http.StatusOK, "")
	if info["state"] != "pending" || info["target_kind"] != "channel" || info["contents"].(map[string]any)["channels"] != float64(1) {
		t.Fatalf("deletion %v", info)
	}
	evs := w.events("channel.archived")
	if len(evs) != 1 || evs[0].ReceiveAction != "channel.read" || evs[0].Payload["channel"].(map[string]any)["deleted_at"] == nil {
		t.Fatalf("channel.archived %+v", evs)
	}
	if a := w.allows("channel.delete"); len(a) != 1 || a[0].Details["trash_project_folder"] != false {
		t.Fatalf("audit %+v", a)
	}
	if names := w.channelNames("alex", support); slices.Contains(names, "general") {
		t.Fatalf("listed %v", names)
	}
	// The deleted item is archived, so a member without the right meets the archive cap first.
	w.want(w.do("alex", http.MethodPost, path+"/restore", "", key()...), http.StatusForbidden, "")
	restored := w.postEmpty("sam", path+"/restore")
	if _, ok := restored["deleted_at"]; ok || restored["archived_at"] != nil {
		t.Fatalf("restored %v", restored)
	}
	if upd := w.events("channel.updated"); len(upd) == 0 || upd[len(upd)-1].Payload["channel"].(map[string]any)["deleted_at"] != nil {
		t.Fatalf("channel.updated %+v", upd)
	}
	if a := w.allows("channel.restore"); len(a) != 1 {
		t.Fatalf("audit %+v", a)
	}

	w.postEmpty("owner", path+"/delete")
	w.postEmpty("owner", spacePath(support)+"/delete")
	r := w.do("owner", http.MethodPost, path+"/restore", "", key()...)
	w.wantConflict(r, chat.ReasonSpaceDeleted)
}

// TestDeleteThread (D-DEL-3, D-DEL-4): the creator with write or a manager deletes a chat without
// step-up, anybody else with write gets 403; a main thread is 409 main_thread; the deletion travels
// as thread.updated with deleted_at; a restore within the window brings it back and after it is
// 409 undo_window_passed.
func TestDeleteThread(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	alex, sam := w.person("alex", false), w.person("sam", false)
	support := w.create("Support")
	forum := w.channel(support, "forum", "bugs")
	text := w.channel(support, "text", "general")
	w.grant(alex, support, "trigger")
	w.grant(sam, support, "write")
	title := "Alex's chat"
	own, err := w.newThread("alex", chat.CreateThreadCmd{ChannelID: forum, Title: &title})
	if err != nil {
		t.Fatal(err)
	}
	theirs := w.post(forum, "Owner's chat")

	w.want(w.do("alex", http.MethodPost, threadPath(theirs.ID)+"/delete", "", key()...), http.StatusForbidden, "forbidden")
	w.want(w.do("sam", http.MethodPost, threadPath(own.ID)+"/delete", "", key()...), http.StatusForbidden, "forbidden")
	r := w.do("owner", http.MethodPost, threadPath(w.mainThread(text))+"/delete", "", key()...)
	w.wantConflict(r, chat.ReasonMainThread)

	info := w.postEmpty("alex", threadPath(own.ID)+"/delete")
	if info["state"] != "pending" || info["contents"].(map[string]any)["threads"] != float64(1) {
		t.Fatalf("deletion %v", info)
	}
	evs := w.threadEvents("thread.updated", own.ID)
	if len(evs) != 1 || evs[0].ReceiveAction != "thread.read" || evs[0].Payload["thread"].(map[string]any)["deleted_at"] == nil {
		t.Fatalf("thread.updated %+v", evs)
	}
	if a := w.allows("thread.delete"); len(a) != 1 || a[0].ResourceID != own.ID.String() {
		t.Fatalf("audit %+v", a)
	}
	w.want(w.do("alex", http.MethodGet, threadPath(own.ID), ""), http.StatusNotFound, "not_found")
	if got := w.want(w.do("sam", http.MethodGet, threadPath(own.ID)+"/deletion", ""), http.StatusOK, ""); got["state"] != "pending" {
		t.Fatalf("deletion read by a reader %v", got)
	}
	list := w.want(w.do("alex", http.MethodGet, channelThreadsPath(forum), ""), http.StatusOK, "")
	for _, it := range list["items"].([]any) {
		if it.(map[string]any)["id"] == own.ID.String() {
			t.Fatal("a deleted thread is listed")
		}
	}
	restored := w.postEmpty("alex", threadPath(own.ID)+"/restore")
	if _, ok := restored["deleted_at"]; ok {
		t.Fatalf("restored %v", restored)
	}
	if evs := w.threadEvents("thread.updated", own.ID); len(evs) != 2 || evs[1].Payload["thread"].(map[string]any)["deleted_at"] != nil {
		t.Fatalf("thread.updated after the restore %+v", evs)
	}
	w.want(w.do("alex", http.MethodGet, threadPath(own.ID), ""), http.StatusOK, "")

	w.postEmpty("owner", threadPath(theirs.ID)+"/delete")
	w.clk.Advance(undoWindow + time.Second)
	r = w.do("owner", http.MethodPost, threadPath(theirs.ID)+"/restore", "", key()...)
	w.wantConflict(r, chat.ReasonUndoWindowPassed)
}

// TestDeletionInfo_Counts (D-DEL-6): the counts of a space, a channel and a thread: channels
// without a channel deleted on its own, chats without main threads and deleted chats, messages
// from message_count, files attached to their messages and the agent sessions of their threads.
func TestDeletionInfo_Counts(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	support := w.create("Support")
	forum := w.channel(support, "forum", "bugs")
	text := w.channel(support, "text", "general")
	gone := w.channel(support, "text", "gone")
	w.postEmpty("owner", channelPath(gone)+"/delete")
	a, b := w.post(forum, "One"), w.post(forum, "Two")
	deleted := w.post(forum, "Three")
	w.postEmpty("owner", threadPath(deleted.ID)+"/delete")
	w.sql(`UPDATE threads SET message_count = 2 WHERE id = $1`, a.ID.String())
	w.sql(`UPDATE threads SET message_count = 3 WHERE id = $1`, b.ID.String())
	w.sql(`UPDATE threads SET message_count = 1 WHERE channel_id = $1`, text.String())
	msg := w.message(a.ID)
	file := ids.New()
	w.sql(`INSERT INTO files (id, object_key, sha256, size_bytes, mime_type, original_name, uploaded_by, purpose, scope_channel_id,
		upload_state, scan_status) VALUES ($1, $2, '\x01', 10, 'text/plain', 'notes.txt', $3, 'attachment', $4, 'complete', 'clean')`,
		file.String(), "f/"+file.String(), w.ownerID.String(), forum.String())
	w.sql(`INSERT INTO message_attachments (message_id, file_id, position, thread_id, channel_id) VALUES ($1, $2, 0, $3, $4)`,
		msg.String(), file.String(), a.ID.String(), forum.String())
	agent := w.operatorAgent("lotse", "organization")
	w.sql(`INSERT INTO agent_sessions (id, thread_id, agent_id, external_session_ref) VALUES ($1, $2, $3, 'sess-a')`, ids.New().String(), b.ID.String(), agent.String())

	for _, c := range []struct {
		path string
		want map[string]float64
	}{
		{spacePath(support), map[string]float64{"channels": 2, "threads": 2, "messages": 6, "files": 1, "sessions": 1}},
		{channelPath(forum), map[string]float64{"channels": 1, "threads": 2, "messages": 5, "files": 1, "sessions": 1}},
		{threadPath(b.ID), map[string]float64{"channels": 0, "threads": 1, "messages": 3, "files": 0, "sessions": 1}},
	} {
		got := w.want(w.do("owner", http.MethodGet, c.path+"/deletion", ""), http.StatusOK, "")
		contents := got["contents"].(map[string]any)
		for k, v := range c.want {
			if contents[k] != v {
				t.Errorf("%s %s = %v, want %v", c.path, k, contents[k], v)
			}
		}
		if got["state"] != "live" || got["deleted_at"] != nil || got["undo_until"] != nil {
			t.Errorf("%s: %v", c.path, got)
		}
	}
}
