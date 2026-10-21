// SPDX-License-Identifier: Apache-2.0

package runs_test

import (
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// TestAG_16_AlwaysAllowNonOwner403 (AG-16: "Always allow" by a non-owner gives 403): Lotse posts
// the approval card of 09 11.5 for Alex's run; Alex clicks "Always allow": 403 forbidden with
// reason component_restricted, nothing changes, one deny audit entry; Alex then clicks "Allow
// once": 201, the other three are disabled, one allow audit entry with choice once; on a second
// card the Owner clicks "Always allow": 201, audit choice always.
func TestAG_16_AlwaysAllowNonOwner403(t *testing.T) {
	t.Parallel()
	w := runsWorld(t)
	r, _, _ := w.waitingRun()
	thread := w.threadOf(r.ID)
	m, byID := w.card("lotse", thread, r.ID, approvalCard)
	card := m["id"].(string)
	if byID["once"]["restricted_to"] != "run_starter" || byID["always"]["restricted_to"] != "owner" {
		t.Fatalf("restrictions %v %v", byID["once"]["restricted_to"], byID["always"]["restricted_to"])
	}
	before := w.componentsOf(card)
	w.wantReason(w.click("alex", byID["always"]["id"].(string), ""), http.StatusForbidden, "forbidden", "component_restricted")
	if after := w.componentsOf(card); !slices.Equal(after, before) {
		t.Fatalf("components changed: %+v", after)
	}
	if d := w.denies("interaction.create", "component_restricted"); len(d) != 1 {
		t.Fatalf("deny audit %+v", d)
	}
	ix := w.want(w.click("alex", byID["once"]["id"].(string), ""), http.StatusCreated, "")
	states := w.statesOf(card)
	if states["once"] != "used" || states["session"] != "disabled" || states["always"] != "disabled" || states["deny"] != "disabled" {
		t.Fatalf("states %v", states)
	}
	allows := allowRows(w.audits("interaction.create"))
	if len(allows) != 1 || allows[0].Details["choice"] != "once" || allows[0].Details["interaction_id"] != ix["id"] ||
		allows[0].Details["required_capability"] != "approve_actions" || allows[0].Details["restricted_to"] != "run_starter" ||
		allows[0].Details["component_type"] != "button" || allows[0].Details["run_id"] != r.ID {
		t.Fatalf("allow audit %+v", allows)
	}
	for _, forbidden := range []string{"custom_id", "label", "Allow once"} {
		raw, _ := json.Marshal(allows[0].Details)
		if strings.Contains(string(raw), forbidden) {
			t.Fatalf("audit details carry %s: %s", forbidden, raw)
		}
	}
	// A second card of the same run: the Owner may choose "Always allow".
	_, second := w.card("lotse", thread, r.ID, approvalCard)
	w.want(w.click("owner", second["always"]["id"].(string), ""), http.StatusCreated, "")
	allows = allowRows(w.audits("interaction.create"))
	if len(allows) != 2 || allows[1].Details["choice"] != "always" {
		t.Fatalf("allow audit %+v", allows)
	}
}

// allowRows keeps the allow rows.
func allowRows(list []auditRow) []auditRow {
	var out []auditRow
	for _, r := range list {
		if r.Decision == "allow" {
			out = append(out, r)
		}
	}
	return out
}

// TestAZ_M42_RestrictedToRunStarter (AZ-M42): the Owner triggers Lotse; Lotse starts the run and
// posts a card with restricted_to run_starter; Alex, who holds approve_actions, gets 403 forbidden
// with reason component_restricted.
func TestAZ_M42_RestrictedToRunStarter(t *testing.T) {
	t.Parallel()
	w := runsWorld(t)
	th := w.newThread(w.allgemein)
	m := w.post("owner", th, "Please check the export")
	r, ok := w.runOf(m["id"].(string))
	if !ok {
		t.Fatal("no run")
	}
	w.wantOK(w.start("lotse", r.ID, w.deliver(r.ID)))
	_, byID := w.card("lotse", th, r.ID, approvalCard)
	w.wantReason(w.click("alex", byID["once"]["id"].(string), ""), http.StatusForbidden, "forbidden", "component_restricted")
}

// TestAZ_M43_ApproveActionsMissing (AZ-M43): Sam triggers Texter; Texter posts an approval card for
// Sam's run; Sam, without approve_actions in Marketing, gets 403 forbidden with reason
// capability_missing.
func TestAZ_M43_ApproveActionsMissing(t *testing.T) {
	t.Parallel()
	w := runsWorld(t)
	th := w.newThread(w.kampagnen)
	m := w.post("sam", th, "Draft the newsletter")
	r, ok := w.runOf(m["id"].(string))
	if !ok {
		t.Fatal("no run of Texter")
	}
	w.wantOK(w.start("texter", r.ID, w.deliver(r.ID)))
	_, byID := w.card("texter", th, r.ID, approvalCard)
	w.wantReason(w.click("sam", byID["once"]["id"].(string), ""), http.StatusForbidden, "forbidden", "capability_missing")
}

// formCard is a form with one field of every type.
const formCard = `[{"custom_id":"details","type":"form","definition":{"title":"Refund details","fields":[` +
	`{"name":"reason","label":"Reason","type":"text","required":true,"max_length":20},` +
	`{"name":"notes","label":"Notes","type":"textarea"},` +
	`{"name":"amount","label":"Amount","type":"number"},` +
	`{"name":"urgent","label":"Urgent","type":"boolean"},` +
	`{"name":"channel","label":"Channel","type":"choice","choices":["mail","phone"]}]}}]`

// TestCreateInteraction_FormValuesValidated (plan row: form values checked against the
// definition): a valid form gives 201 with the values stored as sent; each invalid case gives 400
// validation_failed with interaction_values_invalid and the pointer, and no detail echoes a value;
// values whose event would exceed sync.event_payload_max_bytes give too_long on /values.
func TestCreateInteraction_FormValuesValidated(t *testing.T) {
	t.Parallel()
	w := runsWorld(t)
	r, _, _ := w.waitingRun()
	thread := w.threadOf(r.ID)
	for _, c := range []struct{ name, values, pointer string }{
		{"unknown key", `{"reason":"Late","other":"x"}`, "/values"},
		{"missing required", `{"notes":"n"}`, "/values/reason"},
		{"white space only", `{"reason":"   "}`, "/values/reason"},
		{"line break in text", `{"reason":"a\nb"}`, "/values/reason"},
		{"too long", `{"reason":"` + strings.Repeat("y", 21) + `"}`, "/values/reason"},
		{"string for number", `{"reason":"Late","amount":"12"}`, "/values/amount"},
		{"number for boolean", `{"reason":"Late","urgent":1}`, "/values/urgent"},
		{"choice outside", `{"reason":"Late","channel":"fax"}`, "/values/channel"},
	} {
		_, byID := w.card("lotse", thread, r.ID, formCard)
		p := w.want(w.click("alex", byID["details"]["id"].(string), c.values), http.StatusBadRequest, "validation_failed")
		list := p["errors"].([]any)
		first := list[0].(map[string]any)
		if first["code"] != "interaction_values_invalid" || first["pointer"] != c.pointer {
			t.Errorf("%s: %v", c.name, list)
		}
		for _, e := range list {
			if d, _ := e.(map[string]any)["detail"].(string); strings.Contains(d, "fax") || strings.Contains(d, "yyyy") || strings.Contains(d, "Late") {
				t.Errorf("%s: detail echoes a value: %s", c.name, d)
			}
		}
	}
	_, byID := w.card("lotse", thread, r.ID, formCard)
	values := `{"reason":"Late delivery","notes":"Second line\nthird","amount":12.5,"urgent":true,"channel":"phone"}`
	ix := w.want(w.click("alex", byID["details"]["id"].(string), values), http.StatusCreated, "")
	got, _ := json.Marshal(ix["values"])
	var want map[string]any
	must(t, json.Unmarshal([]byte(values), &want))
	wantJSON, _ := json.Marshal(want)
	if string(got) != string(wantJSON) {
		t.Fatalf("values %s, want %s", got, wantJSON)
	}
	if stored := w.text(`SELECT "values"::text FROM interactions WHERE id = $1`, ix["id"].(string)); !strings.Contains(stored, "Late delivery") {
		t.Fatalf("stored values %s", stored)
	}
	// A form value larger than the event allows.
	_, byID = w.card("lotse", thread, r.ID, `[{"custom_id":"long","type":"form","definition":{"title":"Notes","fields":[{"name":"notes","label":"Notes","type":"textarea"}]}}]`)
	p := w.want(w.click("alex", byID["long"]["id"].(string), `{"notes":"`+strings.Repeat("z", 70000)+`"}`), http.StatusBadRequest, "validation_failed")
	first := p["errors"].([]any)[0].(map[string]any)
	if first["pointer"] != "/values" || first["code"] != "too_long" {
		t.Fatalf("too large: %v", p["errors"])
	}
}

// TestCreateInteraction_SelectValues: selections within and outside min_values and max_values.
func TestCreateInteraction_SelectValues(t *testing.T) {
	t.Parallel()
	w := runsWorld(t)
	r, _, _ := w.waitingRun()
	thread := w.threadOf(r.ID)
	sel := `[{"custom_id":"pick","type":"select","single_use":false,"definition":{"options":[{"label":"One","value":"one"},` +
		`{"label":"Two","value":"two"},{"label":"Three","value":"three"}],"min_values":1,"max_values":2}}]`
	_, byID := w.card("lotse", thread, r.ID, sel)
	id := byID["pick"]["id"].(string)
	for _, c := range []struct{ values, pointer string }{
		{`{"selected":[]}`, "/values/selected"},
		{`{"selected":["one","two","three"]}`, "/values/selected"},
		{`{"selected":["four"]}`, "/values/selected/0"},
		{`{"selected":["one","one"]}`, "/values/selected/1"},
	} {
		p := w.want(w.click("alex", id, c.values), http.StatusBadRequest, "validation_failed")
		first := p["errors"].([]any)[0].(map[string]any)
		if first["code"] != "interaction_values_invalid" || first["pointer"] != c.pointer {
			t.Errorf("%s: %v", c.values, p["errors"])
		}
	}
	w.want(w.click("alex", id, `{"selected":["two"]}`), http.StatusCreated, "")
	w.want(w.click("alex", id, `{"selected":["one","three"]}`), http.StatusCreated, "")
	// A component without single_use stays active and unchanged (D-M2-15-13).
	if rows := w.componentsOf(w.text(`SELECT message_id::text FROM message_components WHERE id = $1`, id)); rows[0].State != "active" || rows[0].Version != 1 {
		t.Fatalf("component %+v", rows[0])
	}
}

// TestCreateInteraction_Replay: the same request twice gives 201 then 200 with
// Idempotent-Replay, one row, one event; different values with the same client_interaction_id give
// 422; two identical requests behind a barrier give one 201 and one 200; a replay after Alex lost
// read access gives 404.
func TestCreateInteraction_Replay(t *testing.T) {
	t.Parallel()
	w := runsWorld(t)
	r, _, _ := w.waitingRun()
	thread := w.threadOf(r.ID)
	sel := `[{"custom_id":"pick","type":"select","single_use":false,"definition":{"options":[{"label":"One","value":"one"},{"label":"Two","value":"two"}]}}]`
	_, byID := w.card("lotse", thread, r.ID, sel)
	id := byID["pick"]["id"].(string)
	client := ids.New().String()
	seq := w.maxSeq()
	first := w.want(w.clickWith("alex", id, client, `{"selected":["one"]}`), http.StatusCreated, "")
	resp := w.clickWith("alex", id, client, `{ "selected" : ["one"] }`)
	again := w.wantOK(resp)
	if resp.Header.Get("Idempotent-Replay") != "true" || again["id"] != first["id"] {
		t.Fatalf("replay %v %v", resp.Header, again)
	}
	if n := w.count(`SELECT count(*) FROM interactions WHERE client_interaction_id = $1`, client); n != 1 {
		t.Fatalf("%d rows", n)
	}
	if c := countType(w.eventsAfter(seq), "interaction.created"); c != 1 {
		t.Fatalf("%d interaction.created", c)
	}
	w.want(w.clickWith("alex", id, client, `{"selected":["two"]}`), http.StatusUnprocessableEntity, "idempotency_key_reuse")
	// Two identical requests behind a barrier.
	twin := ids.New().String()
	start := make(chan struct{})
	codes := make([]int, 2)
	var wg sync.WaitGroup
	for i := range codes {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			codes[i] = w.clickWith("alex", id, twin, `{"selected":["two"]}`).StatusCode
		}()
	}
	close(start)
	wg.Wait()
	slices.Sort(codes)
	if codes[0] != http.StatusOK || codes[1] != http.StatusCreated {
		t.Fatalf("twins %v", codes)
	}
	// Alex loses read access on Support: the replay is the 404 of an unknown component.
	w.wantOK(w.do("owner", http.MethodPost, "/api/v1/grants/"+w.g1.String()+"/revoke", `{"reason":"Moved team"}`, key()...))
	w.want(w.clickWith("alex", id, client, `{"selected":["one"]}`), http.StatusNotFound, "not_found")
}

// TestCreateInteraction_RightsAndLeaks: Robin (no right on Support) gets 404 identical in status,
// body and headers (except correlation_id and instance) to an unknown component ID; Sam (read
// through Leser) gets 403 forbidden with level_insufficient; an archived channel gives 403
// channel_read_only with reason channel_archived; an agent token gets 403.
func TestCreateInteraction_RightsAndLeaks(t *testing.T) {
	t.Parallel()
	w := runsWorld(t)
	r, _, _ := w.waitingRun()
	thread := w.threadOf(r.ID)
	_, byID := w.card("lotse", thread, r.ID, twoButtons)
	id := byID["approve"]["id"].(string)
	robin := w.click("robin", id, "")
	unknown := w.click("robin", ids.New().String(), "")
	if robin.StatusCode != http.StatusNotFound || unknown.StatusCode != http.StatusNotFound {
		t.Fatalf("robin %d, unknown %d", robin.StatusCode, unknown.StatusCode)
	}
	if a, b := sameProblem(t, robin.Body), sameProblem(t, unknown.Body); a != b {
		t.Fatalf("bodies differ:\n%s\n%s", robin.Body, unknown.Body)
	}
	for _, h := range []string{"Content-Type", "Cache-Control", "Content-Length"} {
		if robin.Header.Get(h) != unknown.Header.Get(h) && h != "Content-Length" {
			t.Fatalf("header %s differs: %q %q", h, robin.Header.Get(h), unknown.Header.Get(h))
		}
	}
	w.wantReason(w.click("sam", id, ""), http.StatusForbidden, "forbidden", "level_insufficient")
	w.want(w.click("lotse", id, ""), http.StatusForbidden, "")
	w.wantOK(w.do("owner", http.MethodPost, "/api/v1/channels/"+w.allgemein.String()+"/archive", `{"reason":"Closed"}`,
		ifMatch(w.channelVersion(w.allgemein))...))
	w.wantReason(w.click("alex", id, ""), http.StatusForbidden, "channel_read_only", "channel_archived")
}

// sameProblem returns the problem body without correlation_id and instance.
func sameProblem(t *testing.T, body []byte) string {
	t.Helper()
	var m map[string]any
	must(t, json.Unmarshal(body, &m))
	delete(m, "correlation_id")
	delete(m, "instance")
	out, _ := json.Marshal(m)
	return string(out)
}

// TestCreateInteraction_Expired: the clock passes expires_at, the job has not run: 409
// component_used; the job then sets expired, raises the versions and writes one message.updated.
func TestCreateInteraction_Expired(t *testing.T) {
	t.Parallel()
	w := runsWorld(t)
	r, _, _ := w.waitingRun()
	thread := w.threadOf(r.ID)
	exp := w.clk.Now().Add(10 * time.Minute).UTC().Format("2006-01-02T15:04:05.000Z")
	comps := strings.ReplaceAll(twoButtons, `"restricted_to":"any"}`, `"restricted_to":"any","expires_at":"`+exp+`"}`)
	m, byID := w.card("lotse", thread, r.ID, comps)
	card := m["id"].(string)
	w.clk.Advance(11 * time.Minute)
	w.want(w.click("alex", byID["approve"]["id"].(string), ""), http.StatusConflict, "component_used")
	before := w.messageVersion(card)
	seq := w.maxSeq()
	must(t, w.runs.ExpireComponents(t.Context(), ids.MustParse(card)))
	rows := w.componentsOf(card)
	if rows[0].State != "expired" || rows[1].State != "expired" || rows[0].Version != 2 {
		t.Fatalf("components %+v", rows)
	}
	if v := w.messageVersion(card); v != before+1 {
		t.Fatalf("message version %d", v)
	}
	if got := types(w.eventsAfter(seq)); !slices.Equal(got, []string{"message.updated"}) {
		t.Fatalf("events %v", got)
	}
	// A second run of the job changes nothing.
	must(t, w.runs.ExpireComponents(t.Context(), ids.MustParse(card)))
	if n := len(w.eventsAfter(seq)); n != 1 {
		t.Fatalf("%d events after the second run", n)
	}
}

// TestCreateInteraction_I2PersonalAgent (red team: a personal subscription triggered by another
// person through an interaction): a raw SQL statement gives Alex trigger on Research while
// Assistent stays personal with the Owner as holder (a fixture that simulates an I1 defect,
// OQ-01); Alex clicks a card of Assistent: 403 agent_invocation_denied with reason
// billing_mode_personal_not_holder; the component stays active; one deny audit entry.
func TestCreateInteraction_I2PersonalAgent(t *testing.T) {
	t.Parallel()
	w := runsWorld(t)
	th := w.newThread(w.notizen)
	m := w.post("owner", th, "Summarise my notes")
	r, ok := w.runOf(m["id"].(string))
	if !ok {
		t.Fatal("no run of Assistent")
	}
	w.wantOK(w.startWith("assistent", r.ID, w.deliver(r.ID), "personal", "chat"))
	_, byID := w.card("assistent", th, r.ID, twoButtons)
	w.sql(`INSERT INTO grants (id, subject_kind, subject_principal_id, target_kind, target_space_id, level, capabilities, reason, granted_by)
		VALUES ($1, 'user', $2, 'space', $3, 'trigger', '{}', 'I1 defect fixture', $4)`, ids.New().String(), w.alex.String(), w.research.String(), w.owner.String())
	// The cache of the permission snapshots is keyed by permission_version.
	w.sql(`UPDATE sequence_counters SET value = value + 1 WHERE name = 'permission_version'`)
	w.wantReason(w.click("alex", byID["approve"]["id"].(string), ""), http.StatusForbidden, "agent_invocation_denied", "billing_mode_personal_not_holder")
	if st := w.text(`SELECT state FROM message_components WHERE id = $1`, byID["approve"]["id"].(string)); st != "active" {
		t.Fatalf("component %s", st)
	}
	if d := w.denies("interaction.create", "billing_mode_personal_not_holder"); len(d) != 1 {
		t.Fatalf("deny audit %+v", d)
	}
}

// TestCreateInteraction_NewRun: a run-less card of Lotse, no active run: 201; a new run queued
// with trigger_kind interaction, triggering_interaction_id and triggered_by Alex; the events in
// the order of design note 10 i; the delivery of interaction.created and then of agent.invocation
// carries tokens with trigger_kind interaction and the interaction_id; startRun with the
// agent.invocation token gives 200.
func TestCreateInteraction_NewRun(t *testing.T) {
	t.Parallel()
	w := runsWorld(t)
	r, _, _ := w.startedRun()
	thread := w.threadOf(r.ID)
	_, byID := w.card("lotse", thread, "", twoButtons)
	w.wantOK(w.finish("lotse", r.ID, `"state":"completed"`))
	seq := w.maxSeq()
	ix := w.want(w.click("alex", byID["approve"]["id"].(string), ""), http.StatusCreated, "")
	run := ix["run_id"].(string)
	var kind, triggering, by, state string
	must(t, w.db.QueryRow(t.Context(), `SELECT trigger_kind, coalesce(triggering_interaction_id::text, ''), triggered_by::text, state
		FROM runs WHERE id = $1`, run).Scan(&kind, &triggering, &by, &state))
	if kind != "interaction" || triggering != ix["id"] || by != w.alex.String() || state != "queued" {
		t.Fatalf("run %s %s %s %s", kind, triggering, by, state)
	}
	got := types(w.eventsAfter(seq))
	want := []string{"message.updated", "message.created", "run.state_changed", "interaction.created", "agent.invocation"}
	if !slices.Equal(got, want) {
		t.Fatalf("events %v, want %v", got, want)
	}
	validateSchema(t, "AgentInvocationPayload", w.eventPayloadOf("agent.invocation", run))
	out := w.deliverInteraction(ix["id"].(string))
	if !out.Send {
		t.Fatal("interaction.created was not sent")
	}
	if c := claimsOf(t, out.InvocationToken); c["trigger_kind"] != "interaction" || c["interaction_id"] != ix["id"] {
		t.Fatalf("claims %v", c)
	}
	tok := w.deliver(run)
	if c := claimsOf(t, tok); c["trigger_kind"] != "interaction" || c["interaction_id"] != ix["id"] || c["message_id"] != nil {
		t.Fatalf("claims %v", c)
	}
	w.wantOK(w.start("lotse", run, tok))
}

// TestCreateInteraction_ActiveRunRules: a run-less card of Lotse while Lotse's run is active gives
// 403 agent_invocation_denied with run_active; while a run of another agent is active,
// run_active_other_agent.
func TestCreateInteraction_ActiveRunRules(t *testing.T) {
	t.Parallel()
	w := runsWorld(t)
	r, _, _ := w.startedRun()
	thread := w.threadOf(r.ID)
	_, byID := w.card("lotse", thread, "", twoButtons)
	w.wantReason(w.click("alex", byID["approve"]["id"].(string), ""), http.StatusForbidden, "agent_invocation_denied", "run_active")
	// A run of Texter in the thread of the card: Texter is a guest in Eskalation.
	th := w.newThread(w.eskalation)
	_, card := w.card("lotse", th, "", twoButtons)
	m := w.post("robin", th, "@texter please draft a reply")
	if _, ok := w.runOf(m["id"].(string)); !ok {
		w.sql(`INSERT INTO runs (id, thread_id, agent_id, trigger_kind, triggered_by, billing_mode_expected, invocation_jti)
			VALUES ($1, $2, $3, 'message', $4, 'organization', $1)`, ids.New().String(), th.String(), w.texter.String(), w.owner.String())
	}
	w.wantReason(w.click("alex", card["approve"]["id"].(string), ""), http.StatusForbidden, "agent_invocation_denied", "run_active_other_agent")
}

// TestAG_31_InteractionResumesWaitingRun (AG-31 end to end: only the token of the
// interaction.created delivery sets a waiting run to running): Alex triggers Lotse; Lotse starts
// with S, posts an approval card bound to the run and calls awaitRunInput; renewInvocationToken(S)
// and resumeRun(S) give 403 invocation_token_invalid with superseded; Alex clicks "Allow once";
// the delivery of interaction.created carries token A and sets interactions.state dispatched;
// resumeRun(A) gives 200 and running.
func TestAG_31_InteractionResumesWaitingRun(t *testing.T) {
	t.Parallel()
	w := runsWorld(t)
	r, _, s := w.startedRun()
	thread := w.threadOf(r.ID)
	_, byID := w.card("lotse", thread, r.ID, approvalCard)
	w.wantOK(w.await("lotse", r.ID, "Waiting for approval"))
	w.wantReason(w.renew("lotse", r.ID, s), http.StatusForbidden, "invocation_token_invalid", "superseded")
	w.wantReason(w.resume("lotse", r.ID, s), http.StatusForbidden, "invocation_token_invalid", "superseded")
	ix := w.want(w.click("alex", byID["once"]["id"].(string), ""), http.StatusCreated, "")
	out := w.deliverInteraction(ix["id"].(string))
	if !out.Send || out.InvocationToken == "" {
		t.Fatal("interaction.created was not sent")
	}
	if st := w.interactionState(ix["id"].(string)); st != "dispatched" {
		t.Fatalf("interaction %s", st)
	}
	// The interaction token renews like the delivery it came with (D-M2-15-22); the fresh token
	// resumes the run.
	grant := w.wantOK(w.renew("lotse", r.ID, out.InvocationToken))
	fresh := grant["invocation_token"].(string)
	if c := claimsOf(t, fresh); c["trigger_kind"] != "interaction" || c["interaction_id"] != ix["id"] {
		t.Fatalf("renewed claims %v", c)
	}
	body := w.wantOK(w.resume("lotse", r.ID, fresh))
	if body["state"] != "running" {
		t.Fatalf("run %v", body["state"])
	}
}

// TestInteractionDelivery_Recheck: after Alex's click and before Lotse connects, the Owner revokes
// G-1: interaction.created is not sent, interactions.state failed, the run stays waiting_input,
// one deny audit entry with stage delivery; a variant revokes only approve_actions: the same with
// capability_missing.
func TestInteractionDelivery_Recheck(t *testing.T) {
	t.Parallel()
	for _, variant := range []string{"grant", "capability"} {
		t.Run(variant, func(t *testing.T) {
			t.Parallel()
			w := runsWorld(t)
			r, _, _ := w.waitingRun()
			thread := w.threadOf(r.ID)
			_, byID := w.card("lotse", thread, r.ID, approvalCard)
			ix := w.want(w.click("alex", byID["once"]["id"].(string), ""), http.StatusCreated, "")
			reason := "no_grant"
			w.wantOK(w.do("owner", http.MethodPost, "/api/v1/grants/"+w.g1.String()+"/revoke", `{"reason":"Moved team"}`, key()...))
			if variant == "capability" {
				reason = "capability_missing"
				w.grant(`"subject_kind":"user","subject_id":"` + w.alex.String() + `","target_kind":"space","target_id":"` + w.support.String() +
					`","level":"trigger","capabilities":["upload"],"reason":"Support lead without approvals"`)
			}
			out := w.deliverInteraction(ix["id"].(string))
			if out.Send {
				t.Fatal("interaction.created was sent after the revocation")
			}
			if st := w.interactionState(ix["id"].(string)); st != "failed" {
				t.Fatalf("interaction %s", st)
			}
			if st := w.text(`SELECT state FROM runs WHERE id = $1`, r.ID); st != "waiting_input" {
				t.Fatalf("run %s", st)
			}
			d := w.denies("agent.invoke", reason)
			if len(d) != 1 || d[0].Details["stage"] != "delivery" || d[0].Details["event_type"] != "interaction.created" {
				t.Fatalf("deny audit %+v", w.audits("agent.invoke"))
			}
		})
	}
}

// TestRespondToInteraction: acknowledged with a hint gives 200; the same again gives 200 without
// change; failed afterwards gives 409 invalid_state_transition; Texter gets 404; Alex gets 403; a
// hint above message.max_content_bytes gives 400; a repeat with the same Idempotency-Key gives the
// stored answer.
func TestRespondToInteraction(t *testing.T) {
	t.Parallel()
	w := runsWorld(t)
	r, _, _ := w.waitingRun()
	thread := w.threadOf(r.ID)
	_, byID := w.card("lotse", thread, r.ID, twoButtons)
	ix := w.want(w.click("alex", byID["approve"]["id"].(string), ""), http.StatusCreated, "")
	path := "/api/v1/interactions/" + ix["id"].(string) + "/respond"
	w.want(w.do("lotse", http.MethodPost, path, `{"state":"acknowledged","response":{"text":"`+strings.Repeat("h", 4097)+`"}}`, key()...),
		http.StatusBadRequest, "validation_failed")
	k := key()
	first := w.wantOK(w.do("lotse", http.MethodPost, path, `{"state":"acknowledged","response":{"text":"Refund approved"}}`, k...))
	resp, _ := first["response"].(map[string]any)
	if first["state"] != "acknowledged" || resp["text"] != "Refund approved" {
		t.Fatalf("answer %v", first)
	}
	repeat := w.do("lotse", http.MethodPost, path, `{"state":"acknowledged","response":{"text":"Refund approved"}}`, k...)
	if w.wantOK(repeat)["state"] != "acknowledged" || repeat.Header.Get("Idempotent-Replay") != "true" {
		t.Fatalf("repeat %v", repeat.Header)
	}
	seq := w.maxSeq()
	same := w.wantOK(w.do("lotse", http.MethodPost, path, `{"state":"acknowledged"}`, key()...))
	if same["state"] != "acknowledged" || len(w.eventsAfter(seq)) != 0 {
		t.Fatalf("same state %v", same)
	}
	w.want(w.do("lotse", http.MethodPost, path, `{"state":"failed"}`, key()...), http.StatusConflict, "invalid_state_transition")
	w.want(w.do("texter", http.MethodPost, path, `{"state":"failed"}`, key()...), http.StatusNotFound, "not_found")
	w.want(w.do("alex", http.MethodPost, path, `{"state":"failed"}`, key()...), http.StatusForbidden, "forbidden")
}

// TestUpdateComponent: without If-Match 428; stale 412 version_conflict with current; active ->
// disabled gives 200 with both versions up by one and one message.updated; disabled -> active on a
// single-use component whose sibling is used gives 409; used -> disabled and an expired component
// to active give 409; a new label on a used component gives 200; a definition that breaks
// component.v1 gives 400; a body with required_capability, run_id or restricted_to gives 400; no
// change gives 200 without a new version or event; Texter gets 404; Alex gets 403 with
// subject_kind_not_allowed.
func TestUpdateComponent(t *testing.T) {
	t.Parallel()
	w := runsWorld(t)
	r, _, _ := w.waitingRun()
	thread := w.threadOf(r.ID)
	m, byID := w.card("lotse", thread, r.ID, twoButtons)
	card := m["id"].(string)
	approve, deny := byID["approve"]["id"].(string), byID["deny"]["id"].(string)
	path := func(id string) string { return "/api/v1/components/" + id }
	w.want(w.do("lotse", http.MethodPatch, path(approve), `{"state":"disabled"}`, key()...), http.StatusPreconditionRequired, "precondition_required")
	p := w.want(w.do("lotse", http.MethodPatch, path(approve), `{"state":"disabled"}`, ifMatchOnly(7)...), http.StatusPreconditionFailed, "version_conflict")
	if cur, _ := p["current"].(map[string]any); cur["id"] != approve || cur["version"] != float64(1) {
		t.Fatalf("current %v", p["current"])
	}
	before := w.messageVersion(card)
	seq := w.maxSeq()
	resp := w.do("lotse", http.MethodPatch, path(approve), `{"state":"disabled"}`, ifMatchOnly(1)...)
	c := w.wantOK(resp)
	if c["state"] != "disabled" || version(c) != 2 || resp.Header.Get("ETag") != `"2"` || w.messageVersion(card) != before+1 {
		t.Fatalf("disable %v %v", c, resp.Header)
	}
	if got := types(w.eventsAfter(seq)); !slices.Equal(got, []string{"message.updated"}) {
		t.Fatalf("events %v", got)
	}
	// Alex uses deny; approve may not come back (the single-use decision is made).
	w.want(w.click("alex", deny, ""), http.StatusCreated, "")
	w.want(w.do("lotse", http.MethodPatch, path(approve), `{"state":"active"}`, ifMatchOnly(2)...), http.StatusConflict, "invalid_state_transition")
	w.want(w.do("lotse", http.MethodPatch, path(deny), `{"state":"disabled"}`, ifMatchOnly(2)...), http.StatusConflict, "invalid_state_transition")
	c = w.wantOK(w.do("lotse", http.MethodPatch, path(deny), `{"definition":{"label":"Denied","value":"deny","style":"danger"}}`, ifMatchOnly(2)...))
	if c["state"] != "used" || version(c) != 3 || c["definition"].(map[string]any)["label"] != "Denied" {
		t.Fatalf("relabel %v", c)
	}
	p = w.want(w.do("lotse", http.MethodPatch, path(deny), `{"definition":{"label":"Denied"}}`, ifMatchOnly(3)...), http.StatusBadRequest, "validation_failed")
	if first := p["errors"].([]any)[0].(map[string]any); first["code"] != "required" {
		t.Fatalf("errors %v", p["errors"])
	}
	for _, member := range []string{`"required_capability":"approve_actions"`, `"run_id":"` + r.ID + `"`, `"restricted_to":"owner"`} {
		w.want(w.do("lotse", http.MethodPatch, path(deny), `{`+member+`}`, ifMatchOnly(3)...), http.StatusBadRequest, "validation_failed")
	}
	seq = w.maxSeq()
	c = w.wantOK(w.do("lotse", http.MethodPatch, path(deny), `{"definition":{"value":"deny","style":"danger","label":"Denied"}}`, ifMatchOnly(3)...))
	if version(c) != 3 || len(w.eventsAfter(seq)) != 0 {
		t.Fatalf("no change %v", c)
	}
	w.want(w.do("texter", http.MethodPatch, path(deny), `{"state":"disabled"}`, ifMatchOnly(3)...), http.StatusNotFound, "not_found")
	w.wantReason(w.do("alex", http.MethodPatch, path(deny), `{"state":"disabled"}`, ifMatchOnly(3)...), http.StatusForbidden, "forbidden", "subject_kind_not_allowed")
	// An expired component cannot become active again.
	exp := w.clk.Now().Add(time.Minute).UTC().Format("2006-01-02T15:04:05.000Z")
	_, other := w.card("lotse", thread, r.ID, `[{"custom_id":"later","type":"button","single_use":false,"expires_at":"`+exp+`","definition":{"label":"Later","value":"later"}}]`)
	later := other["later"]["id"].(string)
	w.wantOK(w.do("lotse", http.MethodPatch, path(later), `{"state":"disabled"}`, ifMatchOnly(1)...))
	w.clk.Advance(2 * time.Minute)
	w.want(w.do("lotse", http.MethodPatch, path(later), `{"state":"active"}`, ifMatchOnly(2)...), http.StatusConflict, "invalid_state_transition")
}

// TestEndOfRunDisablesComponents: finishRun with completed, and in variants cancelRun after
// agent.cancel_grace and a denial at delivery: every active component of the run is disabled with
// one message.updated per card; components of other runs and run-less cards are unchanged.
func TestEndOfRunDisablesComponents(t *testing.T) {
	t.Parallel()
	for _, variant := range []string{"finish", "cancel", "denial"} {
		t.Run(variant, func(t *testing.T) {
			t.Parallel()
			w := runsWorld(t)
			var r runRow
			if variant == "denial" {
				r, _ = w.openRun()
			} else {
				r, _, _ = w.startedRun()
			}
			thread := w.threadOf(r.ID)
			a, _ := w.card("lotse", thread, r.ID, twoButtons)
			b, _ := w.card("lotse", thread, r.ID, approvalCard)
			free, _ := w.card("lotse", thread, "", twoButtons)
			other, _, _ := w.startedRun()
			o, _ := w.card("lotse", w.threadOf(other.ID), other.ID, twoButtons)
			seq := w.maxSeq()
			switch variant {
			case "finish":
				w.wantOK(w.finish("lotse", r.ID, `"state":"completed"`))
			case "cancel":
				w.want(w.cancel("alex", r.ID), http.StatusAccepted, "")
				w.clk.Advance(31 * time.Second)
				must(t, w.runs.CancelTimeout(t.Context(), ids.MustParse(r.ID)))
			case "denial":
				w.wantOK(w.do("owner", http.MethodPost, "/api/v1/grants/"+w.g1.String()+"/revoke", `{"reason":"Moved team"}`, key()...))
				out, err := w.runs.Prepare(t.Context(), deliveryOf(w, r.ID))
				must(t, err)
				if out.Send {
					t.Fatal("sent after the revocation")
				}
			}
			if st := w.text(`SELECT state FROM runs WHERE id = $1`, r.ID); st != "completed" && st != "cancelled" && st != "failed" {
				t.Fatalf("run %s", st)
			}
			for _, card := range []map[string]any{a, b} {
				for _, c := range w.componentsOf(card["id"].(string)) {
					if c.State != "disabled" {
						t.Fatalf("%s: component %+v", variant, c)
					}
				}
			}
			for _, card := range []map[string]any{free, o} {
				for _, c := range w.componentsOf(card["id"].(string)) {
					if c.State != "active" {
						t.Fatalf("%s: unrelated component %+v", variant, c)
					}
				}
			}
			updated := 0
			for _, e := range w.eventsAfter(seq) {
				if e.Type == "message.updated" && (e.Resource == a["id"] || e.Resource == b["id"]) {
					updated++
				}
			}
			if updated != 2 {
				t.Fatalf("%s: %d message.updated for the two cards", variant, updated)
			}
		})
	}
}

// TestMessageDeleteRemovesComponents: Lotse deletes its card after an interaction that opened a
// run: components and interactions are gone, runs.triggering_interaction_id is NULL, the redacted
// message.created and message.updated payloads have components []; createInteraction on the old
// component ID gives 404; the expiry job then does nothing.
func TestMessageDeleteRemovesComponents(t *testing.T) {
	t.Parallel()
	w := runsWorld(t)
	r, _, _ := w.startedRun()
	thread := w.threadOf(r.ID)
	m, byID := w.card("lotse", thread, "", twoButtons)
	card := m["id"].(string)
	w.wantOK(w.finish("lotse", r.ID, `"state":"completed"`))
	ix := w.want(w.click("alex", byID["approve"]["id"].(string), ""), http.StatusCreated, "")
	run := ix["run_id"].(string)
	w.want(w.do("lotse", http.MethodDelete, "/api/v1/messages/"+card, "", key()...), http.StatusNoContent, "")
	if n := w.count(`SELECT count(*) FROM message_components WHERE message_id = $1`, card) + w.count(`SELECT count(*) FROM interactions WHERE message_id = $1`, card); n != 0 {
		t.Fatalf("%d rows left", n)
	}
	if v := w.text(`SELECT coalesce(triggering_interaction_id::text, '') FROM runs WHERE id = $1`, run); v != "" {
		t.Fatalf("triggering_interaction_id %q", v)
	}
	rs, err := w.db.Query(t.Context(), `SELECT payload::text FROM event_log WHERE resource_id = $1 AND type IN ('message.created', 'message.updated')`, card)
	must(t, err)
	n := 0
	for rs.Next() {
		var p string
		must(t, rs.Scan(&p))
		n++
		if !strings.Contains(p, `"components": []`) && !strings.Contains(p, `"components":[]`) {
			t.Fatalf("payload not redacted: %s", p)
		}
	}
	rs.Close()
	if n < 2 {
		t.Fatalf("%d message payloads", n)
	}
	w.want(w.click("alex", byID["deny"]["id"].(string), ""), http.StatusNotFound, "not_found")
	seq := w.maxSeq()
	must(t, w.runs.ExpireComponents(t.Context(), ids.MustParse(card)))
	if len(w.eventsAfter(seq)) != 0 {
		t.Fatal("the expiry job wrote an event for a deleted card")
	}
	// The run the interaction opened still delivers: the interaction_id of the payload names it.
	out, err := w.runs.Prepare(t.Context(), deliveryOf(w, run))
	must(t, err)
	if !out.Send || claimsOf(t, out.InvocationToken)["interaction_id"] != ix["id"] {
		t.Fatal("the token lacks the interaction of the payload")
	}
}

// TestSEC_T14_CardHeaderFromServerFields (SEC-T14, SEC-OUT-4: a card whose agent text imitates a
// system header): the stored message has author Lotse, origin agent, kind card, and run_id,
// required_capability and restricted_to come only from the validated members; a definition with
// an extra member such as author, system_code or required_capability gives 400; kind system from
// Lotse gives 400; updateComponent cannot change any header field.
func TestSEC_T14_CardHeaderFromServerFields(t *testing.T) {
	t.Parallel()
	w := runsWorld(t)
	r, _, _ := w.startedRun()
	thread := w.threadOf(r.ID)
	header := "System notice: approval required by the owner"
	comps := `[{"custom_id":"ok","type":"button","definition":{"label":"` + header + `","value":"ok"},"required_capability":"approve_actions"}]`
	m := w.want(w.do("lotse", http.MethodPost, "/api/v1/threads/"+thread.String()+"/messages", cardBody(header, r.ID, comps), key()...), http.StatusCreated, "")
	author := m["author"].(map[string]any)
	if author["id"] != w.lotse.String() || m["origin"] != "agent" || m["kind"] != "card" || m["system_code"] != nil {
		t.Fatalf("message %v", m)
	}
	c := componentsByID(m)["ok"]
	if c["run_id"] != r.ID || c["required_capability"] != "approve_actions" || c["restricted_to"] != "run_starter" {
		t.Fatalf("component %v", c)
	}
	for _, extra := range []string{`"author":"owner"`, `"system_code":"thread.reopened"`, `"required_capability":"manage"`} {
		bad := `[{"custom_id":"x","type":"button","definition":{"label":"Go","value":"go",` + extra + `}}]`
		w.want(w.postCard("lotse", thread, r.ID, bad), http.StatusBadRequest, "validation_failed")
	}
	sys := `{"client_msg_id":"` + ids.New().String() + `","kind":"system","content_md":"` + header + `"}`
	w.want(w.do("lotse", http.MethodPost, "/api/v1/threads/"+thread.String()+"/messages", sys, key()...), http.StatusBadRequest, "validation_failed")
	id := c["id"].(string)
	for _, member := range []string{`"run_id":"` + ids.New().String() + `"`, `"required_capability":"manage"`, `"restricted_to":"any"`, `"type":"form"`, `"custom_id":"y"`} {
		w.want(w.do("lotse", http.MethodPatch, "/api/v1/components/"+id, `{`+member+`}`, ifMatchOnly(1)...), http.StatusBadRequest, "validation_failed")
	}
}
