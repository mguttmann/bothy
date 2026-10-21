// SPDX-License-Identifier: Apache-2.0

package canary

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/OWNER/bothy/services/server/test/harness"
)

// formScope is the scope of the form value canaries: of all observers only the card's agent
// reads it, through interaction.created (SEC-T10, M2-15 design note 18). Sam reads the channel of
// the card and still may not see a value, so the channel cannot be the scope.
const formScope = "form values of a card"

// responseScope is the scope of the canary response.text: no observer reads it, because
// respondToInteraction writes no event and only answers the card's agent.
const responseScope = "response of an interaction"

// components drives the run lifecycle of M2-09 and the three operations of components and
// interactions (M2-15) on a run of Lotse in Support/Allgemein. Lotse's consumer reaches the head
// of the log; Alex posts a message, which opens a run of the operator Lotse; Lotse takes the
// invocation token from its consumer, starts the run, posts a card bound to it with a button, a
// select and a form whose title, labels, placeholder and option labels are canaries of the
// channel, and waits for input with a canary status line. Alex chooses an option and submits the
// form with a canary text and textarea value (201), then a form with a canary choice outside
// choices (400 interaction_values_invalid); Robin (no right on Support) gets 404 with canary
// values. Lotse polls its consumer again (a foreign record of Lotse: interaction.created carries
// the form values), renews the token of that delivery, resumes the run with the fresh one,
// answers the interaction with a canary response.text, sends updateComponent without If-Match
// (428) and with a canary label (200) and finishes the run; the owner cancels the run of Lotse in
// Support/Eskalation. Sam reads Support/Allgemein, so his frames and pages carry the card and its
// message.updated, never a form value.
func (sc *Scenario) components() {
	t := sc.T
	t.Helper()
	o, alex, robin, lotse := sc.W.Owner, sc.W.People["Alex"], sc.W.People["Robin"], sc.W.Agents["Lotse"]
	allgemein := sc.W.Channels["Support/Allgemein"]
	label := func(what string) string { return sc.Reg.New(ClassContent, allgemein, what) }
	// The consumer starts at the head of the log on its first poll; the run opens afterwards.
	sc.consume(lotse, "", "")
	thread, run := sc.openThread(alex, allgemein, label)
	if run == "" {
		t.Errorf("components: the new thread of Alex in Support/Allgemein opened no run of Lotse")
		return
	}
	start := sc.consume(lotse, "agent.invocation", run)
	if start == "" {
		t.Errorf("components: the consumer of Lotse delivered no agent.invocation with a token for the run")
		return
	}
	runPath := "/runs/" + run
	sc.post(lotse, runPath+"/start", map[string]any{"invocation_token": start, "billing_mode_used": "organization", "tool_class_used": "chat"}, ok...)

	components := []map[string]any{
		{"custom_id": "go", "type": "button", "single_use": false, "definition": map[string]any{"label": label("button label"), "value": "go"}},
		{"custom_id": "pick", "type": "select", "single_use": false, "definition": map[string]any{
			"placeholder": label("select placeholder"),
			"options": []map[string]any{
				{"label": label("option label one"), "value": "one"}, {"label": label("option label two"), "value": "two"},
			},
		}},
		{"custom_id": "ask", "type": "form", "single_use": false, "definition": map[string]any{
			"title": label("form title"),
			"fields": []map[string]any{
				{"name": "reason", "label": label("form field label text"), "type": "text", "required": true},
				{"name": "notes", "label": label("form field label textarea"), "type": "textarea"},
				{"name": "channel", "label": label("form field label choice"), "type": "choice", "choices": []string{"mail", "phone"}},
			},
		}},
	}
	card := obj(sc.post(lotse, "/threads/"+thread+"/messages", map[string]any{
		"client_msg_id": newID(), "kind": "card", "content_md": label("card text"), "run_id": run, "components": components,
	}, created...))
	byID := map[string]map[string]any{}
	list, _ := card["components"].([]any)
	for _, c := range list {
		if m, isObj := c.(map[string]any); isObj {
			byID[jsonText(m["custom_id"])] = m
		}
	}
	button, sel, form := jsonText(byID["go"]["id"]), jsonText(byID["pick"]["id"]), jsonText(byID["ask"]["id"])
	if button == "" || sel == "" || form == "" {
		t.Errorf("components: the card has no components: %s", clip([]byte(fmt.Sprint(card))))
		return
	}
	sc.post(lotse, runPath+"/await-input", map[string]any{"status_line": label("run status line")}, ok...)

	click := func(values map[string]any) map[string]any {
		return map[string]any{"client_interaction_id": newID(), "values": values}
	}
	value := func(what string) string { return sc.Reg.New(ClassContent, formScope, what) }
	sc.post(alex, "/components/"+sel+"/interactions", click(map[string]any{"selected": []string{"two"}}), created...)
	text := value("form value text")
	ix := obj(sc.post(alex, "/components/"+form+"/interactions", click(map[string]any{
		"reason": text, "notes": value("form value textarea") + "\nsecond line", "channel": "phone",
	}), created...))
	if str(ix, "values.reason") != text {
		t.Errorf("components: the 201 of createInteraction does not return the form values")
	}
	invalid := obj(sc.post(alex, "/components/"+form+"/interactions", click(map[string]any{
		"reason": value("form value text (400)"), "channel": value("form value choice (400)"),
	}), http.StatusBadRequest))
	if errList, _ := invalid["errors"].([]any); len(errList) == 0 || jsonText(errList[0].(map[string]any)["code"]) != "interaction_values_invalid" {
		t.Errorf("components: the 400 of a choice outside choices is not interaction_values_invalid: %v", invalid["errors"])
	}
	sc.post(robin, "/components/"+form+"/interactions", click(map[string]any{"reason": value("form value text (404)")}), http.StatusNotFound)

	interaction := str(ix, "id")
	resume := sc.consume(lotse, "interaction.created", interaction, text)
	if resume == "" {
		t.Errorf("components: the consumer of Lotse delivered no interaction.created with the form values and a token")
	} else {
		// The token of the delivery renews like the delivery it came with (D-M2-15-22); the
		// fresh token resumes the run.
		renewed := str(obj(sc.post(lotse, runPath+"/invocation-token", map[string]any{"invocation_token": resume}, ok...)), "invocation_token")
		sc.Reg.Register(renewed, ClassSecret, "", "renewed invocation token")
		sc.post(lotse, runPath+"/resume", map[string]any{"invocation_token": renewed, "billing_mode_used": "organization", "tool_class_used": "chat"}, ok...)
	}
	sc.post(lotse, "/interactions/"+interaction+"/respond", map[string]any{
		"state": "acknowledged", "response": map[string]any{"text": sc.Reg.New(ClassContent, responseScope, "interaction response text")},
	}, ok...)
	relabel := func(what string) map[string]any {
		return map[string]any{"definition": map[string]any{"label": label(what), "value": "go"}}
	}
	sc.call(lotse, req{method: http.MethodPatch, path: "/components/" + button, body: relabel("button label (428)")}, http.StatusPreconditionRequired)
	sc.call(lotse, req{method: http.MethodPatch, path: "/components/" + button, body: relabel("button label (updated)"),
		header: ifMatch(jsonText(byID["go"]["version"]))}, ok...)
	sc.post(lotse, runPath+"/finish", map[string]any{"state": "completed"}, ok...)

	// The run of Lotse that the first message in Support/Eskalation opened: the owner cancels it.
	if other := activeRun(sc, o, sc.W.Threads["Support/Eskalation"]); other != "" {
		sc.post(o, "/runs/"+other+"/cancel", map[string]any{}, http.StatusOK, http.StatusAccepted)
	} else {
		t.Errorf("components: the thread of Support/Eskalation has no active run to cancel")
	}
}

// openThread lets c post a message into the main thread of the text channel ch, with texts from
// text; auto_thread of the behaviour step makes it the parent of a new thread, and the message
// opens a run of the channel's operator (M2-05). It returns the thread of that run and the run
// ("" when none opened in time).
func (sc *Scenario) openThread(c *harness.Client, ch string, text func(string) string) (string, string) {
	sc.T.Helper()
	main := str(obj(sc.get(c, "/channels/"+ch, ok...)), "channel.main_thread_id")
	parent := id(sc.post(c, "/threads/"+main+"/messages", map[string]any{"client_msg_id": newID(), "content_md": text("thread parent text")}, created...))
	deadline := time.Now().Add(runWait)
	for parent != "" && time.Now().Before(deadline) {
		threads := []string{main}
		for _, th := range items(sc.get(c, "/channels/"+ch+"/threads", ok...)) {
			if jsonText(th["parent_message_id"]) == parent {
				threads = append(threads, jsonText(th["id"]))
			}
		}
		for _, th := range threads {
			if run := activeRun(sc, c, th); run != "" {
				return th, run
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	return "", ""
}

// runWait bounds the wait of openThread for the run of a new thread.
const runWait = 10 * time.Second

// activeRun returns the ID of the active run of thread as c lists it, or "".
func activeRun(sc *Scenario, c *harness.Client, thread string) string {
	sc.T.Helper()
	for _, r := range items(sc.get(c, "/threads/"+thread+"/runs", ok...)) {
		switch jsonText(r["state"]) {
		case "queued", "running", "waiting_input":
			return jsonText(r["id"])
		}
	}
	return ""
}

// maxConsumerPolls bounds the polls of consume.
const maxConsumerPolls = 20

// consume polls the default consumer of agent (pollAgentEvents) until the event of typ on
// resource came or a poll is empty, registers every invocation token as a secret, adds every
// event without its token to the foreign sink as a record of the agent and acks each page up to
// next_cursor. It returns
// the invocation token of that event, or "" when none came; every want must appear in its payload.
func (sc *Scenario) consume(agent *harness.Client, typ, resource string, want ...string) string {
	sc.T.Helper()
	token := ""
	for poll := 1; poll <= maxConsumerPolls && token == ""; poll++ {
		resp := sc.get(agent, "/agents/me/consumers/default/events", ok...)
		var body struct {
			Events     []map[string]any `json:"events"`
			NextCursor int64            `json:"next_cursor"`
		}
		_ = json.Unmarshal(resp.Body, &body)
		for _, e := range body.Events {
			tok := jsonText(e["invocation_token"])
			sc.Reg.Register(tok, ClassSecret, "", "invocation token")
			// The token is the agent's own credential for this delivery (a secret of its success
			// body, like its access token); the foreign record holds the rest of the event.
			delete(e, "invocation_token")
			page, _ := json.Marshal(e)
			sc.Sinks.AddJSON(SinkForeign, fmt.Sprintf("Lotse pollAgentEvents %s %d %v", typ, poll, e["type"]), "Lotse", "", page)
			e["invocation_token"] = tok
			res, _ := e["resource"].(map[string]any)
			if jsonText(e["type"]) != typ || jsonText(res["id"]) != resource || tok == "" {
				continue
			}
			raw, _ := json.Marshal(e["payload"])
			found := true
			for _, w := range want {
				found = found && strings.Contains(string(raw), w)
			}
			if found {
				token = tok
			}
		}
		if len(body.Events) == 0 {
			break
		}
		sc.post(agent, "/agents/me/consumers/default/ack", map[string]any{"seq": body.NextCursor}, ok...)
	}
	return token
}
