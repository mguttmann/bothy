// SPDX-License-Identifier: Apache-2.0

package runs_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// sizedCard is a card of two buttons whose approve label has n bytes.
func sizedCard(n int) string {
	return `[{"custom_id":"approve","type":"button","definition":{"label":"` + strings.Repeat("l", n) + `","value":"approve"}},` +
		`{"custom_id":"deny","type":"button","definition":{"label":"Deny","value":"deny"}}]`
}

// TestCardAtSizeLimit: the largest card createMessage accepts still changes state in every
// message.updated it can get (D-M2-15-26): after reactions with the most distinct symbols, a click
// gives 201 and the end of the run after cancelRun disables the rest of the card; a definition that makes the card too large
// for that reserve is 400 too_long on /definition in updateComponent, never 500.
func TestCardAtSizeLimit(t *testing.T) {
	t.Parallel()
	w := runsWorld(t)
	r := w.waitingRun()
	thread := w.threadOf(r.ID)
	// The largest accepted label, found by bisection; card is the last accepted one, of label lo.
	lo, hi := 1000, 70000
	var card map[string]any
	for lo < hi {
		mid := (lo + hi + 1) / 2
		resp := w.postCard("lotse", thread, r.ID, sizedCard(mid))
		if resp.StatusCode == http.StatusCreated {
			lo = mid
			card = w.want(resp, http.StatusCreated, "")
			continue
		}
		w.want(resp, http.StatusBadRequest, "validation_failed")
		hi = mid - 1
	}
	if card == nil || componentsByID(card)["approve"] == nil {
		t.Fatal("no card accepted")
	}
	id := card["id"].(string)
	byID := componentsByID(card)
	emoji := []string{"👍", "👎", "😀", "😃", "😄", "😁", "😆", "😅", "😂", "🤣", "😊", "😇", "🙂", "🙃", "😉", "😌", "😍", "🥰", "😘", "😗"}
	for _, e := range emoji {
		resp := w.do("owner", http.MethodPut, "/api/v1/messages/"+id+"/reactions/"+url.PathEscape(e), "")
		if resp.StatusCode/100 != 2 {
			t.Fatalf("reaction %s: %d %s", e, resp.StatusCode, resp.Body)
		}
	}
	// A larger definition does not fit the reserve.
	deny := byID["deny"]["id"].(string)
	p := w.want(w.do("lotse", http.MethodPatch, "/api/v1/components/"+deny,
		`{"definition":{"label":"`+strings.Repeat("d", 200)+`","value":"deny"}}`, ifMatchOnly(1)...), http.StatusBadRequest, "validation_failed")
	if first := p["errors"].([]any)[0].(map[string]any); first["pointer"] != "/definition" || first["code"] != "too_long" {
		t.Fatalf("update errors %v", p["errors"])
	}
	w.want(w.click("alex", byID["approve"]["id"].(string), ""), http.StatusCreated, "")
	w.want(w.cancel("alex", r.ID), http.StatusAccepted, "")
	w.clk.Advance(31 * time.Second)
	must(t, w.runs.CancelTimeout(t.Context(), ids.MustParse(r.ID)))
	if st := w.text(`SELECT state FROM runs WHERE id = $1`, r.ID); st != "cancelled" {
		t.Fatalf("run %s", st)
	}
	if st := w.statesOf(id); st["deny"] != "disabled" || st["approve"] != "used" {
		t.Fatalf("states %v", st)
	}
}

// TestInteraction_NULRefused: U+0000, which PostgreSQL cannot store, in a form value or in
// response.text is 400 validation_failed with its pointer (D-M2-15-27), never 500.
func TestInteraction_NULRefused(t *testing.T) {
	t.Parallel()
	w := runsWorld(t)
	r := w.waitingRun()
	thread := w.threadOf(r.ID)
	_, byID := w.card("lotse", thread, r.ID, `[{"custom_id":"long","type":"form","definition":{"title":"Notes","fields":[{"name":"notes","label":"Notes","type":"textarea"}]}}]`)
	p := w.want(w.click("alex", byID["long"]["id"].(string), `{"notes":"a\u0000b"}`), http.StatusBadRequest, "validation_failed")
	if first := p["errors"].([]any)[0].(map[string]any); first["pointer"] != "/values" || first["code"] != "interaction_values_invalid" {
		t.Fatalf("values errors %v", p["errors"])
	}
	_, byID = w.card("lotse", thread, r.ID, twoButtons)
	ix := w.want(w.click("alex", byID["approve"]["id"].(string), ""), http.StatusCreated, "")
	path := "/api/v1/interactions/" + ix["id"].(string) + "/respond"
	p = w.want(w.do("lotse", http.MethodPost, path, `{"state":"acknowledged","response":{"text":"a\u0000b"}}`, key()...), http.StatusBadRequest, "validation_failed")
	if first := p["errors"].([]any)[0].(map[string]any); first["pointer"] != "/response/text" || first["code"] != "invalid_characters" {
		t.Fatalf("respond errors %v", p["errors"])
	}
}
