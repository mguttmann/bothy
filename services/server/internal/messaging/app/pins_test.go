// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"encoding/json"
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
	mw "github.com/OWNER/bothy/services/server/test/messagingworld"
)

// pin sends pinMessage as name.
func pin(w *mw.World, name, id string) *httpservertest.Response {
	return w.Do(name, http.MethodPut, messagePath(id)+"/pin", "")
}

// unpin sends unpinMessage as name.
func unpin(w *mw.World, name, id string) *httpservertest.Response {
	return w.Do(name, http.MethodDelete, messagePath(id)+"/pin", "")
}

// listPins requests listPins of thread as name.
func listPins(w *mw.World, name string, thread ids.ID) *httpservertest.Response {
	return w.Do(name, http.MethodGet, "/api/v1/threads/"+thread.String()+"/pins", "")
}

// jsonOf renders a decoded JSON value again.
func jsonOf(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// pinned returns the message ids of a PinList in order.
func pinned(body map[string]any) []string {
	var out []string
	for _, it := range body["items"].([]any) {
		out = append(out, it.(map[string]any)["message_id"].(string))
	}
	return out
}

// TestPinMessage_RequiresManage: Alex with write but without manage gets 403, the owner 204, the
// operator agent 403; Alex on a Marketing message gets the 404 of a random id.
func TestPinMessage_RequiresManage(t *testing.T) {
	t.Parallel()
	w, _, ch, _, _, msg := world(t)
	op := w.Agent("operator-agent")
	w.SetOperator(ch, op)
	w.Want(pin(w, "alex", msg.id), http.StatusForbidden, "forbidden")
	w.Want(pin(w, "operator-agent", msg.id), http.StatusForbidden, "forbidden")
	w.Want(pin(w, "owner", msg.id), http.StatusNoContent, "")
	w.Want(unpin(w, "alex", msg.id), http.StatusForbidden, "forbidden")
	marketing := w.MainThread(w.Channel(w.Space("Marketing"), "text", "general"))
	foreign := post(w, "owner", marketing, "marketing plan")
	mw.Same404(t, pin(w, "alex", foreign.id), pin(w, "alex", ids.New().String()))
	mw.Same404(t, unpin(w, "alex", foreign.id), unpin(w, "alex", ids.New().String()))
}

// TestPinMessage_IdempotentAndEvents: two pins give one pin.added; two unpins one pin.removed
// carrying the former pin.
func TestPinMessage_IdempotentAndEvents(t *testing.T) {
	t.Parallel()
	w, _, _, main, _, msg := world(t)
	w.Want(pin(w, "owner", msg.id), http.StatusNoContent, "")
	w.Want(pin(w, "owner", msg.id), http.StatusNoContent, "")
	added := w.Events("pin.added")
	if len(added) != 1 {
		t.Fatalf("pin.added %+v", added)
	}
	p := added[0].Payload["pin"].(map[string]any)
	if p["message_id"] != msg.id || p["thread_id"] != main.String() || p["pinned_by"] != w.OwnerID.String() || p["pinned_at"] == nil {
		t.Fatalf("pin %v", p)
	}
	validate(t, "PinPayload", added[0].Raw)
	w.Want(unpin(w, "owner", msg.id), http.StatusNoContent, "")
	w.Want(unpin(w, "owner", msg.id), http.StatusNoContent, "")
	removed := w.Events("pin.removed")
	if len(removed) != 1 || jsonOf(removed[0].Payload) != jsonOf(added[0].Payload) {
		t.Fatalf("pin.removed %+v, added %s", removed, added[0].Raw)
	}
	if n := w.Count(`SELECT count(*)::int FROM pins`); n != 0 {
		t.Fatalf("rows %d", n)
	}
}

// TestPinMessage_DeletedMessage410: a tombstone refuses a pin with 410; unpinning is 204.
func TestPinMessage_DeletedMessage410(t *testing.T) {
	t.Parallel()
	w, _, _, _, _, msg := world(t)
	w.Want(del(w, "alex", msg.id), http.StatusNoContent, "")
	w.Want(pin(w, "owner", msg.id), http.StatusGone, "message_deleted")
	w.Want(unpin(w, "owner", msg.id), http.StatusNoContent, "")
	if evs := w.Events("pin.added", "pin.removed"); len(evs) != 0 {
		t.Fatalf("events %+v", evs)
	}
}

// TestPinMessage_ReadOnlyChannelAllowsManage: in a read-only channel the owner still pins and
// unpins (05 rule 1.8: manage actions stay allowed).
func TestPinMessage_ReadOnlyChannelAllowsManage(t *testing.T) {
	t.Parallel()
	w, _, ch, _, _, msg := world(t)
	w.SQL(`UPDATE channels SET read_only = true, version = version + 1 WHERE id = $1`, ch.String())
	w.Want(pin(w, "owner", msg.id), http.StatusNoContent, "")
	w.Want(unpin(w, "owner", msg.id), http.StatusNoContent, "")
	if n := len(w.Events("pin.added", "pin.removed")); n != 2 {
		t.Fatalf("events %d", n)
	}
}

// TestListPins_OrderAndVisibility: newest pin first, equal times by message id descending; the pin
// of a tombstone is absent; an invisible thread is the 404 of a random id.
func TestListPins_OrderAndVisibility(t *testing.T) {
	t.Parallel()
	w, _, _, main, _, msg := world(t)
	second, third := post(w, "alex", main, "second"), post(w, "alex", main, "third")
	w.Want(pin(w, "owner", msg.id), http.StatusNoContent, "")
	w.Clock.Advance(time.Second)
	w.Want(pin(w, "owner", second.id), http.StatusNoContent, "")
	w.Want(pin(w, "owner", third.id), http.StatusNoContent, "")
	tied := []string{second.id, third.id}
	slices.Sort(tied)
	want := []string{tied[1], tied[0], msg.id}
	if got := pinned(w.Want(listPins(w, "owner", main), http.StatusOK, "")); !slices.Equal(got, want) {
		t.Fatalf("pins %v, want %v", got, want)
	}
	w.Want(del(w, "alex", second.id), http.StatusNoContent, "")
	want = slices.DeleteFunc(want, func(id string) bool { return id == second.id })
	if got := pinned(w.Want(listPins(w, "alex", main), http.StatusOK, "")); !slices.Equal(got, want) {
		t.Fatalf("pins after delete %v, want %v", got, want)
	}
	marketing := w.MainThread(w.Channel(w.Space("Marketing"), "text", "general"))
	mw.Same404(t, listPins(w, "alex", marketing), listPins(w, "alex", ids.New()))
}

// TestDeleteMessage_RemovesReactionsAndPin: deleting a message removes its reactions and its pin
// in the same transaction; the tombstone shows none (M1-59 design note 9).
func TestDeleteMessage_RemovesReactionsAndPin(t *testing.T) {
	t.Parallel()
	w, _, _, _, _, msg := world(t)
	w.Want(react(w, "alex", msg.id, thumbsUp), http.StatusNoContent, "")
	w.Want(react(w, "owner", msg.id, party), http.StatusNoContent, "")
	w.Want(pin(w, "owner", msg.id), http.StatusNoContent, "")
	w.Want(del(w, "alex", msg.id), http.StatusNoContent, "")
	if n := w.Count(`SELECT (SELECT count(*) FROM reactions WHERE message_id = $1) + (SELECT count(*) FROM pins WHERE message_id = $1)`, msg.id); n != 0 {
		t.Fatalf("rows left %d", n)
	}
	m := w.Want(w.Do("owner", http.MethodGet, messagePath(msg.id), ""), http.StatusOK, "")
	if len(m["reactions"].([]any)) != 0 || mw.JSONList(m["my_reactions"]) != "[]" || m["pinned"] != false {
		t.Fatalf("tombstone %v", m)
	}
}
