// SPDX-License-Identifier: Apache-2.0

package integration_test

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// The project binding of a deleted forum (FIX-delete-releases-binding, D-DEL-REL-1 to
// D-DEL-REL-5): it stays during the undo window, so a restore gets it back, and it is released
// once the deletion is final and no folder move waits for the runner; an owner may remove the
// binding of an archived or deleted forum; the purge job releases the bindings of forums purged
// before this rule existed.

// released returns whether the binding of channel is released ("-" without a binding).
func (w *pbWorld) released(channel ids.ID) string {
	w.t.Helper()
	return w.text(`SELECT coalesce((SELECT (released_at IS NOT NULL)::text FROM project_bindings WHERE channel_id = $1), '-')`, channel.String())
}

// restoreChannel undoes the deletion of channel as the owner.
func (w *pbWorld) restoreChannel(channel ids.ID) map[string]any {
	w.t.Helper()
	return w.wantOK(w.do("owner", http.MethodPost, "/api/v1/channels/"+channel.String()+"/restore", "", key()...))
}

// TestDeleteForum_ReleasesBindingWhenFinal: during the undo window the deleted forum keeps its
// folder (a new binding of the same name is 409) and a restore brings the binding back; once the
// deletion is final the folder can be bound to another forum, while the deleted forum's row stays.
func TestDeleteForum_ReleasesBindingWhenFinal(t *testing.T) {
	t.Parallel()
	w := newPBWorld(t)
	w.bind(w.projektA, "website")

	w.wantOK(w.deleteChannel("owner", w.projektA, false))
	w.want(w.put("owner", w.projektB, bindBody("Website")), http.StatusConflict, "conflict")
	restored := w.restoreChannel(w.projektA)
	if restored["project_bound"] != true || w.released(w.projektA) != "false" {
		t.Fatalf("restore lost the binding: %v, released %s", restored, w.released(w.projektA))
	}
	if ref := w.text(`SELECT workspace_ref FROM project_bindings WHERE channel_id = $1`, w.projektA.String()); ref != "website" {
		t.Fatalf("workspace_ref after the restore %q", ref)
	}

	w.wantOK(w.deleteChannel("owner", w.projektA, false))
	w.clk.Advance(31 * time.Second)
	w.want(w.put("owner", w.projektB, bindBody("Website")), http.StatusConflict, "conflict")
	w.purgeDue()
	if w.released(w.projektA) != "true" {
		t.Fatalf("the purge did not release the binding: %s", w.released(w.projektA))
	}
	w.bind(w.projektB, "Website")
	info := w.wantOK(w.do("owner", http.MethodGet, "/api/v1/channels/"+w.projektA.String()+"/deletion", ""))
	if pf, _ := info["project_folder"].(map[string]any); info["state"] != "final" || pf == nil || pf["workspace_ref"] != "website" {
		t.Fatalf("deletion of the released forum %v", info)
	}
	// The released row no longer counts as a bound folder of a runner (M4-44).
	got, err := w.chat.BoundProjectFolders(context.Background(), []string{"website", "WEBSITE", "docs"})
	must(t, err)
	if strings.Join(got, ",") != "website,WEBSITE" {
		t.Fatalf("bound folders %v", got)
	}
	if r := w.del("owner", w.projektB); r.StatusCode != http.StatusNoContent {
		t.Fatalf("unbind: %d %s", r.StatusCode, r.Body)
	}
	if got, err := w.chat.BoundProjectFolders(context.Background(), []string{"website"}); err != nil || len(got) != 0 {
		t.Fatalf("a released binding still counts as bound: %v %v", got, err)
	}
}

// TestDeleteSpace_ReleasesBindingsWhenFinal: the forums of a purged space give their folders free.
func TestDeleteSpace_ReleasesBindingsWhenFinal(t *testing.T) {
	t.Parallel()
	w := newPBWorld(t)
	labor := w.space("Labor")
	forum := w.channel(labor, "forum", "Alt")
	w.bind(forum, "lab-site")
	w.wantOK(w.do("owner", http.MethodPost, "/api/v1/spaces/"+labor.String()+"/delete", "", key()...))
	w.clk.Advance(31 * time.Second)
	w.purgeDue()
	if w.released(forum) != "true" {
		t.Fatalf("the purge of the space did not release the binding: %s", w.released(forum))
	}
	w.bind(w.projektB, "lab-site")
}

// TestDeleteForum_FolderMoveHoldsBindingUntilOutcome: a requested folder move keeps the folder
// bound after the purge until the runner reports its outcome, so no new forum can take a folder
// that is about to move to the trash.
func TestDeleteForum_FolderMoveHoldsBindingUntilOutcome(t *testing.T) {
	t.Parallel()
	w := newPBWorld(t)
	w.bind(w.projektA, "website")
	w.trashCheck(w.projektA, `{"ok":true}`)
	w.wantOK(w.deleteChannel("owner", w.projektA, true))
	w.clk.Advance(31 * time.Second)
	w.purgeDue()
	must(t, func() error { _, err := w.chat.ReleaseFinalBindings(context.Background()); return err }())
	if w.released(w.projektA) != "false" {
		t.Fatalf("a binding with a requested move was released: %s", w.released(w.projektA))
	}
	w.want(w.put("owner", w.projektB, bindBody("website")), http.StatusConflict, "conflict")
	w.wantOK(w.beat("runner", `{"state":"online","project_trash_results":[{"channel_id":"`+w.projektA.String()+`","result":"trashed"}]}`))
	if w.released(w.projektA) != "true" {
		t.Fatalf("the outcome did not release the binding: %s", w.released(w.projektA))
	}
	w.bind(w.projektB, "website")
	final := w.wantOK(w.do("owner", http.MethodGet, "/api/v1/channels/"+w.projektA.String()+"/deletion", ""))
	if pf := final["project_folder"].(map[string]any); pf["trash_state"] != "trashed" {
		t.Fatalf("the outcome is gone after the release: %v", final)
	}
}

// TestDeleteProjectBinding_ArchivedOrDeletedForum: the owner removes the binding of an archived
// forum, of a forum of an archived space and of a deleted forum (which drops a requested folder
// move); the channel.updated of a deleted forum keeps deleted_at, so no client reads it as an undo.
func TestDeleteProjectBinding_ArchivedOrDeletedForum(t *testing.T) {
	t.Parallel()
	w := newPBWorld(t)
	w.bind(w.projektB, "website")
	w.wantOK(w.do("owner", http.MethodPost, "/api/v1/channels/"+w.projektB.String()+"/archive", `{"reason":"Done"}`, key()...))
	if r := w.del("owner", w.projektB); r.StatusCode != http.StatusNoContent {
		t.Fatalf("unbind archived forum: %d %s", r.StatusCode, r.Body)
	}
	if rows := w.allows(actionBind); len(rows) != 2 || rows[1].Details["op"] != "deleted" {
		t.Fatalf("audit %+v", rows)
	}

	archive := w.space("Archiv")
	old := w.channel(archive, "forum", "Alt")
	w.bind(old, "old-site")
	w.wantOK(w.do("owner", http.MethodPost, "/api/v1/spaces/"+archive.String()+"/archive", `{"reason":"Done"}`, key()...))
	if r := w.del("owner", old); r.StatusCode != http.StatusNoContent {
		t.Fatalf("unbind forum of an archived space: %d %s", r.StatusCode, r.Body)
	}

	w.bind(w.projektA, "Website")
	w.trashCheck(w.projektA, `{"ok":true}`)
	w.wantOK(w.deleteChannel("owner", w.projektA, true))
	seq := w.headSeq()
	if r := w.del("owner", w.projektA); r.StatusCode != http.StatusNoContent {
		t.Fatalf("unbind deleted forum: %d %s", r.StatusCode, r.Body)
	}
	if w.bindingState(w.projektA) != "-" {
		t.Fatalf("binding left: %s", w.bindingState(w.projektA))
	}
	updated := w.updatedSince(seq)
	if len(updated) != 1 || !strings.Contains(updated[0].Payload, `"deleted_at"`) || !strings.Contains(strings.ReplaceAll(updated[0].Payload, " ", ""), `"project_bound":false`) {
		t.Fatalf("channel.updated of the deleted forum %+v", updated)
	}
	w.clk.Advance(31 * time.Second)
	w.purgeDue()
	if ack := w.wantOK(w.beat("runner", `{"state":"online"}`)); ack["project_trash"] != nil {
		t.Fatalf("a removed binding still asks for a folder move: %v", ack)
	}
	// Non-owners still cannot.
	other := w.channel(w.engineering, "forum", "Projekt-C")
	w.bind(other, "docs")
	w.want(w.del("alex", other), http.StatusForbidden, "forbidden")
}

// TestReleaseFinalBindings_Repair: a binding of a forum purged before this rule (released_at
// NULL) is released by the job's sweep, which also fixes a folder that a purged forum blocks.
func TestReleaseFinalBindings_Repair(t *testing.T) {
	t.Parallel()
	w := newPBWorld(t)
	w.bind(w.projektA, "website")
	w.wantOK(w.deleteChannel("owner", w.projektA, false))
	w.clk.Advance(31 * time.Second)
	w.purgeDue()
	w.sql(`UPDATE project_bindings SET released_at = NULL WHERE channel_id = $1`, w.projektA.String())
	w.want(w.put("owner", w.projektB, bindBody("website")), http.StatusConflict, "conflict")

	n, err := w.chat.ReleaseFinalBindings(context.Background())
	must(t, err)
	if n != 1 || w.released(w.projektA) != "true" {
		t.Fatalf("released %d, state %s", n, w.released(w.projektA))
	}
	if n, err := w.chat.ReleaseFinalBindings(context.Background()); err != nil || n != 0 {
		t.Fatalf("second sweep released %d (%v)", n, err)
	}
	w.bind(w.projektB, "website")
}
