// SPDX-License-Identifier: Apache-2.0

package runs_test

import (
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// TestCreateMessage_Card: Lotse posts a card with a button, a select and a form bound to its
// running run: 201; the rows carry the defaults of design note 5; messages.run_id is NULL and
// every message_components.run_id is the run; message.created, getMessage and listThreadMessages
// show the components in position order; one runs.component_expiry job is scheduled at the
// earliest expires_at (M2-15).
func TestCreateMessage_Card(t *testing.T) {
	t.Parallel()
	w := runsWorld(t)
	r, _ := w.startedRun()
	thread := w.threadOf(r.ID)
	seq := w.maxSeq()
	m, byID := w.card("lotse", thread, r.ID, threeKinds)
	if m["kind"] != "card" || m["origin"] != "agent" || m["run_id"] != nil || len(byID) != 3 {
		t.Fatalf("answer %v", m)
	}
	id := m["id"].(string)
	rows := w.componentsOf(id)
	want := w.clk.Now().Add(24 * time.Hour)
	for i, c := range rows {
		if c.Position != i+1 || !c.SingleUse || c.Restricted != "any" || c.Capability != "" || c.State != "active" || c.Version != 1 ||
			c.Run != r.ID || !c.ExpiresAt.Equal(want) {
			t.Fatalf("component %d %+v", i, c)
		}
	}
	if got := []string{rows[0].CustomID, rows[1].CustomID, rows[2].CustomID}; !slices.Equal(got, []string{"go", "pick", "ask"}) {
		t.Fatalf("order %v", got)
	}
	if run := w.text(`SELECT coalesce(run_id::text, '') FROM messages WHERE id = $1`, id); run != "" {
		t.Fatalf("messages.run_id %q", run)
	}
	// The stored definition carries the defaults (D-M2-15-3).
	def := byID["go"]["definition"].(map[string]any)
	if def["style"] != "secondary" || def["label"] != "Go" {
		t.Fatalf("definition %v", def)
	}
	sel := byID["pick"]["definition"].(map[string]any)
	if sel["min_values"] != float64(1) || sel["max_values"] != float64(1) {
		t.Fatalf("select %v", sel)
	}
	created := w.eventPayloadOf("message.created", id)
	var p struct {
		Message struct {
			Components []struct {
				CustomID string `json:"custom_id"`
				Position int    `json:"position"`
				RunID    string `json:"run_id"`
			} `json:"components"`
		} `json:"message"`
	}
	must(t, json.Unmarshal(created, &p))
	if len(p.Message.Components) != 3 || p.Message.Components[0].CustomID != "go" || p.Message.Components[2].Position != 3 ||
		p.Message.Components[1].RunID != r.ID {
		t.Fatalf("message.created %s", created)
	}
	if !slices.Contains(types(w.eventsAfter(seq)), "message.created") {
		t.Fatal("no message.created")
	}
	got := w.wantOK(w.do("alex", http.MethodGet, "/api/v1/messages/"+id, ""))
	if c := componentsByID(got); len(c) != 3 || c["ask"]["position"] != float64(3) {
		t.Fatalf("getMessage %v", got["components"])
	}
	page := w.wantOK(w.do("alex", http.MethodGet, "/api/v1/threads/"+thread.String()+"/messages", ""))
	found := false
	for _, it := range page["items"].([]any) {
		im := it.(map[string]any)
		if im["id"] == id {
			found = len(im["components"].([]any)) == 3
		} else if len(im["components"].([]any)) != 0 {
			t.Fatalf("a message without components shows %v", im["components"])
		}
	}
	if !found {
		t.Fatal("listThreadMessages lacks the components of the card")
	}
	jobs := w.expiries.of(id)
	if len(jobs) != 1 || !jobs[0].At.Equal(want) {
		t.Fatalf("expiry jobs %+v", jobs)
	}
}

// TestCreateMessage_CardRules: components without kind card, a card without components, a card
// from a person, a run of Texter and a run of another thread (both the same run_mismatch on
// /run_id, the answer of M2-19), an ended own run (409 run_not_active) and a card whose event
// would exceed sync.event_payload_max_bytes (too_long): exact status, pointer and code; nothing is
// stored (D-M2-15-29).
func TestCreateMessage_CardRules(t *testing.T) {
	t.Parallel()
	w := runsWorld(t)
	r, _ := w.startedRun()
	thread := w.threadOf(r.ID)
	other, _ := w.startedRun()
	ended, _ := w.startedRun()
	w.wantOK(w.finish("lotse", ended.ID, `"state":"completed"`))
	// A run of Texter in a thread of Marketing.
	kth := w.newThread(w.kampagnen)
	tm := w.post("sam", kth, "Draft the newsletter")
	tr, ok := w.runOf(tm["id"].(string))
	if !ok {
		t.Fatal("no run of Texter")
	}
	messages := w.count(`SELECT count(*) FROM messages`)
	comps := w.count(`SELECT count(*) FROM message_components`)
	big := strings.Repeat("x", 4000)
	huge := `[`
	for i := 0; i < 25; i++ {
		if i > 0 {
			huge += ","
		}
		huge += `{"custom_id":"b` + string(rune('a'+i)) + `","type":"button","definition":{"label":"` + big + `","value":"v"}}`
	}
	huge += `]`
	for _, c := range []struct {
		name, who, body, pointer, code string
	}{
		{"components without card", "lotse", `{"client_msg_id":"` + ids.New().String() + `","content_md":"x","components":` + twoButtons + `}`, "/kind", "invalid"},
		{"card without components", "lotse", `{"client_msg_id":"` + ids.New().String() + `","kind":"card","content_md":"x","components":[]}`, "/components", "required"},
		{"card of a person", "alex", cardBody("x", "", twoButtons), "/components", "invalid"},
		{"run of Texter", "lotse", cardBody("x", tr.ID, twoButtons), "/run_id", "run_mismatch"},
		{"run of another thread", "lotse", cardBody("x", other.ID, twoButtons), "/run_id", "run_mismatch"},
		{"too large", "lotse", cardBody("x", r.ID, huge), "/components", "too_long"},
	} {
		resp := w.do(c.who, http.MethodPost, "/api/v1/threads/"+thread.String()+"/messages", c.body, key()...)
		p := w.want(resp, http.StatusBadRequest, "validation_failed")
		errsList := p["errors"].([]any)
		first := errsList[0].(map[string]any)
		if first["pointer"] != c.pointer || first["code"] != c.code {
			t.Errorf("%s: %v", c.name, errsList)
		}
	}
	p := w.want(w.do("lotse", http.MethodPost, "/api/v1/threads/"+thread.String()+"/messages", cardBody("x", ended.ID, twoButtons), key()...),
		http.StatusConflict, "conflict")
	if p["reason"] != "run_not_active" {
		t.Errorf("ended run: %v", p)
	}
	if n := w.count(`SELECT count(*) FROM messages`); n != messages {
		t.Fatalf("%d messages stored", n-messages)
	}
	if n := w.count(`SELECT count(*) FROM message_components`); n != comps {
		t.Fatalf("%d components stored", n-comps)
	}
}

// TestCreateMessage_CardSupportsComponentsOff: with operator_config.supports_components = false for
// Lotse, a card is 403 feature_disabled and nothing is stored (09 2.2, D-M2-15-5).
func TestCreateMessage_CardSupportsComponentsOff(t *testing.T) {
	t.Parallel()
	w := runsWorld(t)
	r, _ := w.startedRun()
	thread := w.threadOf(r.ID)
	w.wantOK(w.do("owner", http.MethodPatch, "/api/v1/agents/"+w.lotse.String(), `{"operator_config":{"supports_components":false}}`,
		ifMatch(w.agentVersion(w.lotse))...))
	comps := w.count(`SELECT count(*) FROM message_components`)
	w.want(w.postCard("lotse", thread, r.ID, twoButtons), http.StatusForbidden, "feature_disabled")
	if n := w.count(`SELECT count(*) FROM message_components`); n != comps {
		t.Fatalf("%d components stored", n-comps)
	}
}

// TestCreateMessage_CardReplayAndHash: the same card twice gives 201 then 200 with
// Idempotent-Replay and the same components; the same client_msg_id with a changed component gives
// 422 idempotency_key_reuse; a plain text message replays as before (bothy-msg-v1).
func TestCreateMessage_CardReplayAndHash(t *testing.T) {
	t.Parallel()
	w := runsWorld(t)
	r, _ := w.startedRun()
	thread := w.threadOf(r.ID)
	client := ids.New().String()
	body := func(comps string) string {
		return `{"client_msg_id":"` + client + `","kind":"card","content_md":"Approve?","run_id":"` + r.ID + `","components":` + comps + `}`
	}
	path := "/api/v1/threads/" + thread.String() + "/messages"
	first := w.want(w.do("lotse", http.MethodPost, path, body(twoButtons), key()...), http.StatusCreated, "")
	// Another key order and white space of the same request.
	reordered := `[{"type":"button","custom_id":"approve","restricted_to":"any","definition":{"value":"approve","style":"success","label":"Approve"}},` +
		` {"definition":{"style":"danger","label":"Deny","value":"deny"},"custom_id":"deny","type":"button","restricted_to":"any"}]`
	resp := w.do("lotse", http.MethodPost, path, body(reordered), key()...)
	again := w.wantOK(resp)
	if resp.Header.Get("Idempotent-Replay") != "true" || again["id"] != first["id"] || len(again["components"].([]any)) != 2 {
		t.Fatalf("replay %d %v", resp.StatusCode, again)
	}
	changed := strings.Replace(twoButtons, `"label":"Deny"`, `"label":"Reject"`, 1)
	w.want(w.do("lotse", http.MethodPost, path, body(changed), key()...), http.StatusUnprocessableEntity, "idempotency_key_reuse")
	text := ids.New().String()
	tb := `{"client_msg_id":"` + text + `","content_md":"Plain answer"}`
	w.want(w.do("lotse", http.MethodPost, path, tb, key()...), http.StatusCreated, "")
	resp = w.do("lotse", http.MethodPost, path, tb, key()...)
	if w.wantOK(resp); resp.Header.Get("Idempotent-Replay") != "true" {
		t.Fatal("a plain message does not replay")
	}
}

// TestCreateInteraction_Button: Alex clicks a single-use button on a card of Lotse's run in
// waiting_input: 201 with custom_id, thread_id, run_id, state received; the button is used with
// used_by Alex, its sibling disabled; the events message.updated then interaction.created;
// message.updated raises messages.version by one; the payload validates against
// InteractionCreatedPayload (M2-15 design note 10).
func TestCreateInteraction_Button(t *testing.T) {
	t.Parallel()
	w := runsWorld(t)
	r := w.waitingRun()
	thread := w.threadOf(r.ID)
	m, byID := w.card("lotse", thread, r.ID, twoButtons)
	card := m["id"].(string)
	before := w.messageVersion(card)
	seq := w.maxSeq()
	ix := w.want(w.click("alex", byID["approve"]["id"].(string), ""), http.StatusCreated, "")
	if ix["custom_id"] != "approve" || ix["thread_id"] != thread.String() || ix["run_id"] != r.ID || ix["state"] != "received" ||
		ix["response"] != nil || ix["principal_id"] != w.alex.String() {
		t.Fatalf("interaction %v", ix)
	}
	rows := w.componentsOf(card)
	if rows[0].State != "used" || rows[0].UsedBy != w.alex.String() || rows[0].Version != 2 || rows[1].State != "disabled" || rows[1].Version != 2 {
		t.Fatalf("components %+v", rows)
	}
	if v := w.messageVersion(card); v != before+1 {
		t.Fatalf("message version %d, want %d", v, before+1)
	}
	evs := w.eventsAfter(seq)
	if got := strings.Join(types(evs), " "); got != "message.updated interaction.created" {
		t.Fatalf("events %s", got)
	}
	if evs[0].Version != before+1 {
		t.Fatalf("message.updated version %d", evs[0].Version)
	}
	p := w.eventPayloadOf("interaction.created", ix["id"].(string))
	validateSchema(t, "InteractionCreatedPayload", p)
	if strings.Contains(string(p), "response") {
		t.Fatalf("payload carries a response: %s", p)
	}
}

// TestAG_15_ConcurrentClicksOneWins (AG-15: two people click different buttons of one card at the
// same time; exactly one wins, the rest is disabled): on a card of Lotse with two single-use
// buttons, Alex and the Owner click different buttons behind a barrier: exactly one 201 and one
// 409 component_used; one used, the other disabled; one interaction row; one interaction.created;
// one message.updated for the card; the loser's denial is in the audit with reason
// component_inactive.
func TestAG_15_ConcurrentClicksOneWins(t *testing.T) {
	t.Parallel()
	w := runsWorld(t)
	r := w.waitingRun()
	thread := w.threadOf(r.ID)
	m, byID := w.card("lotse", thread, r.ID, twoButtons)
	card := m["id"].(string)
	seq := w.maxSeq()
	start := make(chan struct{})
	results := make([]int, 2)
	done := make(chan struct{}, 2)
	for i, c := range []struct{ who, button string }{{"alex", "approve"}, {"owner", "deny"}} {
		go func() {
			<-start
			results[i] = w.click(c.who, byID[c.button]["id"].(string), "").StatusCode
			done <- struct{}{}
		}()
	}
	close(start)
	<-done
	<-done
	slices.Sort(results)
	if results[0] != http.StatusCreated || results[1] != http.StatusConflict {
		t.Fatalf("statuses %v", results)
	}
	states := w.statesOf(card)
	used, disabled := 0, 0
	for _, s := range states {
		switch s {
		case "used":
			used++
		case "disabled":
			disabled++
		}
	}
	if used != 1 || disabled != 1 {
		t.Fatalf("states %v", states)
	}
	if n := w.count(`SELECT count(*) FROM interactions WHERE message_id = $1`, card); n != 1 {
		t.Fatalf("%d interactions", n)
	}
	evs := w.eventsAfter(seq)
	if countType(evs, "interaction.created") != 1 || countType(evs, "message.updated") != 1 {
		t.Fatalf("events %v", types(evs))
	}
	if d := w.denies("interaction.create", "component_inactive"); len(d) != 1 {
		t.Fatalf("deny audit %+v", d)
	}
}
