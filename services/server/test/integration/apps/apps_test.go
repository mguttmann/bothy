// SPDX-License-Identifier: Apache-2.0

package apps_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/gateway/adapters/wstest"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
	"github.com/OWNER/bothy/services/server/test/apps/fakeapp"
	"github.com/OWNER/bothy/services/server/test/harness"
)

// envelope is the part of an event envelope the tests read.
type envelope struct {
	ID              string          `json:"id"`
	Type            string          `json:"type"`
	Seq             int64           `json:"seq"`
	SyncEpoch       string          `json:"sync_epoch"`
	InvocationToken string          `json:"invocation_token"`
	Payload         json.RawMessage `json:"payload"`
	Resource        struct {
		Kind    string `json:"kind"`
		ID      string `json:"id"`
		Version *int64 `json:"version"`
	} `json:"resource"`
	Audience *struct {
		Kind string `json:"kind"`
	} `json:"audience"`
}

// nextDispatch reads frames of g until a dispatch of type typ arrives and returns its envelope; the
// envelopes of every other dispatch before it are returned in seen.
func nextDispatch(t *testing.T, g *harness.GatewayConn, typ string) (envelope, []envelope) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), testkit.Patience)
	defer cancel()
	var seen []envelope
	for {
		f, err := g.Next(ctx)
		if err != nil {
			t.Fatalf("waiting for %s: %v (seen %d dispatches)", typ, err, len(seen))
		}
		if f.Op != "dispatch" {
			continue
		}
		var e envelope
		if err := json.Unmarshal(f.D, &e); err != nil {
			t.Fatal(err)
		}
		if e.Type == typ {
			return e, seen
		}
		seen = append(seen, e)
	}
}

// TestApps runs the scenarios of the shared world in order; each subtest leaves the world as the
// next one expects it.
func TestApps(t *testing.T) {
	w := newWorld(t, nil)
	alex, sam, robin := w.m.People["Alex"], w.m.People["Sam"], w.m.People["Robin"]

	t.Run("TestInstallCreatesManifestCommands", func(t *testing.T) {
		w.t = t
		if w.text(`SELECT scope_kind FROM commands WHERE id = $1`, w.link) != "instance" ||
			w.text(`SELECT owner_principal_id::text FROM commands WHERE id = $1`, w.link) != w.linker {
			t.Fatal("link is not an instance-wide command of Linker")
		}
		if n := w.count(`SELECT count(*) FROM event_log WHERE type = 'command.changed' AND resource_id = $1`, w.link); n != 1 {
			t.Fatalf("command.changed of link: %d", n)
		}
		if n := w.count(`SELECT count(*) FROM audit_events WHERE action = 'app.install' AND resource_id = $1 AND details::text LIKE '%' || $2 || '%'`,
			w.linker, w.link); n != 1 {
			t.Fatalf("app.install entries naming link in commands_added: %d", n)
		}
		list := w.do(alex, harness.Request{Method: http.MethodGet, Path: "/channels/" + w.m.Channels["Support/Allgemein"] + "/commands"})
		w.want(list, http.StatusOK)
		if !strings.Contains(string(list.Body), w.link) {
			t.Fatalf("link not offered in Support/Allgemein: %s", list.Body)
		}
		list = w.do(sam, harness.Request{Method: http.MethodGet, Path: "/channels/" + w.m.Channels["Marketing/Kampagnen"] + "/commands"})
		w.want(list, http.StatusOK)
		if strings.Contains(string(list.Body), w.link) {
			t.Fatalf("link offered in Marketing/Kampagnen: %s", list.Body)
		}
	})

	t.Run("TestAppPersonalEventsReachApp_Gateway", func(t *testing.T) {
		w.t = t
		// The gateway of an app ends at ready: sync.snapshot.read has no app scope in the core
		// contract (docs/DECISIONS.md M1-64/11, D-M2-25), so Linker is closed with 4018. The
		// admission of command.invoked and interaction.created to the audience app is proved on the
		// hub's rule (authz TestAgentEventsConsume_ReceiveRuleForApps, sync
		// TestAdmit_AppPersonalEventCarriesAudience); here the person side: the events never reach
		// a reader of the channel.
		ctx, cancel := w.patience()
		defer cancel()
		_, err := w.linkerC.Gateway(ctx)
		var ce *wstest.CloseError
		if !errors.As(err, &ce) || ce.Code != 4018 {
			t.Fatalf("gateway of Linker: %v, want 4018 until the contract gives sync.snapshot.read an app scope", err)
		}
		sg, err := sam.Gateway(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer sg.Close()
		resp := w.invoke(alex, "Support/Allgemein", w.link, "", nil)
		w.want(resp, http.StatusAccepted)
		var inv map[string]any
		resp.JSON(t, &inv)
		if inv["accepted"] != true || inv["run_id"] != nil || inv["thread_id"] != w.mainThread("Support/Allgemein") {
			t.Fatalf("invocation %s", resp.Body)
		}
		thread := w.mainThread("Support/Allgemein")
		_, comps, cardResp := w.postCard(w.linkerC, thread, "Pick one", oneButton)
		w.want(cardResp, http.StatusCreated)
		w.want(w.click(alex, comps["go"], ""), http.StatusCreated)
		ev := w.text(`SELECT payload::text FROM event_log WHERE type = 'command.invoked' AND audience_principal_id = $1 ORDER BY seq DESC LIMIT 1`, w.linker)
		if ev == "" || strings.Contains(ev, "run_id") || w.count(`SELECT count(*) FROM event_log WHERE type = 'interaction.created' AND audience_principal_id = $1`, w.linker) != 1 {
			t.Fatalf("the personal events of Linker are not stored as specified")
		}
		// Barrier: a message Sam must receive comes after both events; none of them reached Sam.
		w.want(w.do(w.s.Owner, harness.Request{Method: http.MethodPost, Path: "/threads/" + thread + "/messages",
			Body: map[string]any{"client_msg_id": ids.New().String(), "content_md": "barrier"}}), http.StatusCreated)
		_, before := nextDispatch(t, sg, "message.created")
		for _, e := range before {
			if e.Type == "command.invoked" || e.Type == "interaction.created" {
				t.Fatalf("Sam received %s", e.Type)
			}
		}
	})

	t.Run("TestAppPersonalEventsReachApp_CatchUp", func(t *testing.T) {
		w.t = t
		w.want(w.invoke(alex, "Support/Allgemein", w.link, "", nil), http.StatusAccepted)
		id := w.text(`SELECT id::text FROM event_log WHERE type = 'command.invoked' AND audience_principal_id = $1 ORDER BY seq DESC LIMIT 1`, w.linker)
		page := w.do(w.linkerC, harness.Request{Method: http.MethodGet, Path: "/events?after=0&limit=500"})
		w.want(page, http.StatusOK)
		if !strings.Contains(string(page.Body), id) {
			t.Fatalf("listEvents of Linker lacks the event")
		}
		w.want(w.do(w.s.Owner, harness.Request{Method: http.MethodPut, Path: "/apps/" + w.linker + "/scopes", Body: map[string]any{
			"granted_scopes": []string{"commands:write", "components:write", "messages:write"}, "reason": "test"}}), http.StatusOK)
		w.wantProblem(w.do(w.linkerC, harness.Request{Method: http.MethodGet, Path: "/events?after=0&limit=5"}), http.StatusForbidden, "scope_missing", "scope_missing")
		w.want(w.do(w.s.Owner, harness.Request{Method: http.MethodPut, Path: "/apps/" + w.linker + "/scopes", Body: map[string]any{
			"granted_scopes":      []string{"commands:write", "components:write", "events:subscribe", "messages:write"},
			"event_subscriptions": []string{"command.invoked"}, "reason": "test"}}), http.StatusOK)
	})

	t.Run("TestAppPersonalEventsReachApp_WebhookSubscription", func(t *testing.T) {
		w.t = t
		w.want(w.do(w.s.Owner, harness.Request{Method: http.MethodPost, Path: "/apps/" + w.linker + "/webhooks", Body: map[string]any{
			"url": w.fake.URLFor(hooksHost, "/hooks/events"), "event_types": []string{"command.invoked"}}}), http.StatusCreated)
		before := len(w.fake.Requests())
		w.want(w.invoke(alex, "Support/Allgemein", w.link, "", nil), http.StatusAccepted)
		id := w.text(`SELECT id::text FROM event_log WHERE type = 'command.invoked' AND audience_principal_id = $1 ORDER BY seq DESC LIMIT 1`, w.linker)
		ctx, cancel := w.patience()
		defer cancel()
		var got fakeapp.Request
		for got.Path == "" {
			reqs, err := w.fake.WaitRequests(ctx, before+1)
			if err != nil {
				t.Fatal(err)
			}
			for _, r := range reqs[before:] {
				if r.Host == w.fake.HostPort(hooksHost) && r.Header.Get("webhook-id") == id {
					got = r
				}
			}
			before = len(reqs)
		}
		if !got.Verified {
			t.Fatal("the delivery is not signed with v1a")
		}
	})

	t.Run("TestAppPersonalEventsReachApp_InteractionsURL", func(t *testing.T) {
		w.t = t
		thread := w.mainThread("Support/Eskalation")
		w.fake.Respond(fakeapp.Response{Body: []byte(`{"client_msg_id":"` + ids.New().String() + `","content_md":"Ticket opened"}`)})
		before := len(w.fake.Requests())
		resp := w.invoke(alex, "Support/Eskalation", w.ticket, "", map[string]string{"title": "Printer"})
		w.want(resp, http.StatusAccepted)
		ctx, cancel := w.patience()
		defer cancel()
		reqs, err := w.fake.WaitRequests(ctx, before+1)
		if err != nil {
			t.Fatal(err)
		}
		r := reqs[before]
		var env envelope
		_ = json.Unmarshal(r.Body, &env)
		if !r.Verified || r.HasV1 || r.Header.Get("webhook-id") != env.ID || env.Type != "command.invoked" || env.SyncEpoch == "" ||
			env.InvocationToken != "" || r.Path != "/hooks/interactions" || r.Header.Get("Authorization") != "" {
			t.Fatalf("request: verified %v v1 %v type %s", r.Verified, r.HasV1, env.Type)
		}
		w.until("the reply of Tracker", func() bool {
			return w.count(`SELECT count(*) FROM messages WHERE author_principal_id = $1 AND thread_id = $2 AND origin = 'app'`, w.tracker, thread) == 1
		})
		if n := w.count(`SELECT count(*) FROM runs WHERE thread_id = $1 AND queued_at > now() - interval '1 hour' AND triggering_command_id = $2`,
			thread, w.ticket); n != 0 {
			t.Fatal("an app command opened a run")
		}
		if n := w.count(`SELECT count(*) FROM event_log WHERE type LIKE 'agent.%' AND payload::text LIKE '%' || $1 || '%'`, w.ticket); n != 0 {
			t.Fatal("an agent event names the app command")
		}
		// 202: no message.
		w.fake.Respond(fakeapp.Response{Status: http.StatusAccepted})
		before = len(w.fake.Requests())
		w.want(w.invoke(alex, "Support/Eskalation", w.ticket, "", map[string]string{"title": "Scanner"}), http.StatusAccepted)
		if _, err := w.fake.WaitRequests(ctx, before+1); err != nil {
			t.Fatal(err)
		}
		w.until("the job of the 202", func() bool {
			return w.count(`SELECT count(*) FROM jobs.river_job WHERE kind = 'runs.app_dispatch' AND state NOT IN ('completed', 'discarded', 'cancelled')`) == 0
		})
		if n := w.count(`SELECT count(*) FROM messages WHERE author_principal_id = $1 AND thread_id = $2`, w.tracker, thread); n != 1 {
			t.Fatalf("messages of Tracker after 202: %d", n)
		}
	})

	t.Run("TestAppInteractionViaInteractionsURL", func(t *testing.T) {
		w.t = t
		thread := w.mainThread("Support/Eskalation")
		_, comps, resp := w.postCard(w.trackerC, thread, "Ticket", buttonAndForm)
		w.want(resp, http.StatusCreated)
		if w.text(`SELECT origin FROM messages WHERE author_principal_id = $1 AND kind = 'card' ORDER BY created_at DESC LIMIT 1`, w.tracker) != "app" {
			t.Fatal("the card has no origin app")
		}
		w.fake.Respond(fakeapp.Response{Status: http.StatusAccepted})
		before := len(w.fake.Requests())
		click := w.click(alex, comps["details"], `{"note":"Paper jam"}`)
		w.want(click, http.StatusCreated)
		var ix map[string]any
		click.JSON(t, &ix)
		id, _ := ix["id"].(string)
		if ix["state"] != "received" || ix["run_id"] != nil {
			t.Fatalf("interaction %s", click.Body)
		}
		ctx, cancel := w.patience()
		defer cancel()
		reqs, err := w.fake.WaitRequests(ctx, before+1)
		if err != nil {
			t.Fatal(err)
		}
		var env envelope
		_ = json.Unmarshal(reqs[before].Body, &env)
		if env.Type != "interaction.created" || !strings.Contains(string(env.Payload), "Paper jam") {
			t.Fatalf("request type %s", env.Type)
		}
		w.until("dispatched", func() bool { return w.interactionState(id) == "dispatched" })
		w.want(w.do(w.trackerC, harness.Request{Method: http.MethodPost, Path: "/interactions/" + id + "/respond",
			Body: map[string]any{"state": "acknowledged"}}), http.StatusOK)
		if w.interactionState(id) != "acknowledged" {
			t.Fatal("respondToInteraction did not win")
		}
		// 200 with a reply: acknowledged and the reply in the card's thread.
		w.fake.Respond(fakeapp.Response{Body: []byte(`{"client_msg_id":"` + ids.New().String() + `","content_md":"Thanks"}`)})
		click = w.click(alex, comps["ok"], "")
		w.want(click, http.StatusCreated)
		click.JSON(t, &ix)
		id, _ = ix["id"].(string)
		w.until("acknowledged by the reply", func() bool { return w.interactionState(id) == "acknowledged" })
		if n := w.count(`SELECT count(*) FROM messages WHERE author_principal_id = $1 AND thread_id = $2 AND content_md = 'Thanks'`, w.tracker, thread); n != 1 {
			t.Fatalf("reply messages %d", n)
		}
	})

	t.Run("TestAppCommandsSkipI2", func(t *testing.T) {
		w.t = t
		// A fixture that simulates an I1 defect (OQ-01): Alex gets trigger on Research while
		// Assistent stays personal with the owner as holder.
		fixture := w.sqlGrant(alex.PrincipalID, "Research", "trigger")
		defer w.dropGrant(fixture)
		w.fake.Respond(fakeapp.Response{Status: http.StatusAccepted})
		before := len(w.fake.Requests())
		denials := w.count(`SELECT count(*) FROM audit_events WHERE decision = 'deny' AND actor_principal_id = $1`, alex.PrincipalID)
		w.want(w.invoke(alex, "Research/Notizen", w.ticket, "", map[string]string{"title": "Notes"}), http.StatusAccepted)
		ctx, cancel := w.patience()
		defer cancel()
		if _, err := w.fake.WaitRequests(ctx, before+1); err != nil {
			t.Fatal(err)
		}
		if n := w.count(`SELECT count(*) FROM audit_events WHERE decision = 'deny' AND actor_principal_id = $1`, alex.PrincipalID); n != denials {
			t.Fatal("the app command wrote a deny entry")
		}
		assistent := w.m.Agents["Assistent"]
		var c struct {
			ID string `json:"id"`
		}
		harness.MustDo(t, assistent, harness.Request{Method: http.MethodPost, Path: "/commands", Body: map[string]any{
			"name": "summarize", "description": "Summarize", "scope_kind": "channel", "scope_id": w.m.Channels["Research/Notizen"], "control": true}},
			http.StatusCreated).JSON(t, &c)
		w.wantProblem(w.invoke(alex, "Research/Notizen", c.ID, "", nil), http.StatusForbidden, "agent_invocation_denied", "billing_mode_personal_not_holder")
		w.until("one deny entry", func() bool {
			return w.count(`SELECT count(*) FROM audit_events WHERE decision = 'deny' AND actor_principal_id = $1`, alex.PrincipalID) == denials+1
		})
		_, comps, resp := w.postCard(w.trackerC, w.mainThread("Research/Notizen"), "Notes", oneButton)
		w.want(resp, http.StatusCreated)
		w.fake.Respond(fakeapp.Response{Status: http.StatusAccepted})
		w.want(w.click(alex, comps["go"], ""), http.StatusCreated)
		// The click reaches Tracker while the fixture still holds (the re-check of the job).
		if _, err := w.fake.WaitRequests(ctx, before+2); err != nil {
			t.Fatal(err)
		}
		if n := w.count(`SELECT count(*) FROM runs WHERE agent_id = $1 AND triggered_by = $2`, assistent.PrincipalID, alex.PrincipalID); n != 0 {
			t.Fatal("Assistent got a run")
		}
		if n := w.count(`SELECT count(*) FROM event_log WHERE audience_principal_id = $1 AND actor_principal_id = $2`, assistent.PrincipalID,
			alex.PrincipalID); n != 0 {
			t.Fatal("Assistent received an event of Alex")
		}
	})

	t.Run("TestAppNeverTriggersAgent", func(t *testing.T) {
		w.t = t
		w.freeResponse("Support/Eskalation")
		lotse := w.m.Agents["Lotse"]
		thread := w.mainThread("Support/Eskalation")
		runs := w.count(`SELECT count(*) FROM runs WHERE thread_id = $1`, thread)
		w.fake.Respond(fakeapp.Response{Body: []byte(`{"client_msg_id":"` + ids.New().String() + `","content_md":"@lotse please look"}`)})
		w.want(w.invoke(alex, "Support/Eskalation", w.ticket, "", map[string]string{"title": "Mention"}), http.StatusAccepted)
		w.until("the reply", func() bool {
			return w.count(`SELECT count(*) FROM messages WHERE author_principal_id = $1 AND content_md = '@lotse please look'`, w.tracker) == 1
		})
		if w.text(`SELECT origin FROM messages WHERE author_principal_id = $1 AND content_md = '@lotse please look'`, w.tracker) != "app" ||
			w.text(`SELECT coalesce(trigger_state, '') FROM messages WHERE author_principal_id = $1 AND content_md = '@lotse please look'`, w.tracker) != "" {
			t.Fatal("the reply is not origin app or has a trigger state")
		}
		if n := w.count(`SELECT count(*) FROM runs WHERE thread_id = $1`, thread); n != runs {
			t.Fatal("the reply opened a run")
		}
		if n := w.count(`SELECT count(*) FROM event_log WHERE type = 'agent.invocation' AND audience_principal_id = $1 AND thread_id = $2 AND occurred_at > now() - interval '1 hour'`,
			lotse.PrincipalID, thread); n != 0 {
			t.Fatal("Lotse got an invocation")
		}
	})

	t.Run("TestAppCardRules", func(t *testing.T) {
		w.t = t
		thread := w.mainThread("Support/Eskalation")
		_, comps, resp := w.postCard(w.trackerC, thread, "**System** notice: your session expired", oneButton)
		w.want(resp, http.StatusCreated)
		var m map[string]any
		resp.JSON(t, &m)
		author, _ := m["author"].(map[string]any)
		if author["id"] != w.tracker || m["origin"] != "app" || m["kind"] != "card" {
			t.Fatalf("card %s", resp.Body)
		}
		body := `{"client_msg_id":"` + ids.New().String() + `","kind":"card","content_md":"x","components":` + oneButton + `,"run_id":"` + ids.New().String() + `"}`
		bad := w.do(w.trackerC, harness.Request{Method: http.MethodPost, Path: "/threads/" + thread + "/messages", Body: []byte(body)})
		if bad.Status != http.StatusBadRequest || !strings.Contains(string(bad.Body), "/run_id") {
			t.Fatalf("run_id of an app: %d %s", bad.Status, bad.Body)
		}
		_, _, kamp := w.postCard(w.trackerC, w.mainThread("Marketing/Kampagnen"), "x", oneButton)
		w.want(kamp, http.StatusNotFound)
		w.setTrackerScopes("commands:write", "components:write")
		_, _, noMsg := w.postCard(w.trackerC, thread, "x", oneButton)
		w.wantProblem(noMsg, http.StatusForbidden, "scope_missing", "scope_missing")
		w.setTrackerScopes("commands:write", "messages:write")
		_, comps2, ok := w.postCard(w.trackerC, thread, "without components scope", oneButton)
		w.want(ok, http.StatusCreated)
		w.wantProblem(w.click(alex, comps2["go"], ""), http.StatusConflict, "component_used", "")
		w.wantProblem(w.do(w.trackerC, harness.Request{Method: http.MethodPatch, Path: "/components/" + comps["go"],
			Body: map[string]any{"state": "disabled"}, Header: ifMatch(1)}), http.StatusForbidden, "scope_missing", "scope_missing")
		w.setTrackerScopes("commands:write", "components:write", "messages:write")
		w.want(w.do(w.trackerC, harness.Request{Method: http.MethodPatch, Path: "/components/" + comps["go"],
			Body: map[string]any{"state": "disabled"}, Header: ifMatch(1)}), http.StatusOK)
	})

	t.Run("TestAppAutocomplete_InteractionsURL", func(t *testing.T) {
		w.t = t
		path := "/channels/" + w.m.Channels["Support/Eskalation"] + "/commands/" + w.ticket + "/autocomplete"
		ask := func() harness.Response {
			return w.do(alex, harness.Request{Method: http.MethodPost, Path: path, Body: map[string]any{"option": "queue", "value": "pri"}})
		}
		w.fake.Respond(fakeapp.Response{Body: []byte(`{"choices":[{"name":"Printer","value":"printer"}]}`)})
		before := len(w.fake.Requests())
		resp := ask()
		w.want(resp, http.StatusOK)
		if !strings.Contains(string(resp.Body), `"printer"`) {
			t.Fatalf("choices %s", resp.Body)
		}
		r := w.fake.Requests()[before]
		var env envelope
		_ = json.Unmarshal(r.Body, &env)
		if !r.Verified || env.Type != "command.autocomplete_requested" || env.InvocationToken != "" {
			t.Fatalf("request %s verified %v", env.Type, r.Verified)
		}
		w.fake.Respond(fakeapp.Response{Body: []byte(`{"choices":[{"name":"","value":"x"}]}`)})
		if resp := ask(); resp.Status != http.StatusOK || !strings.Contains(string(resp.Body), `"choices":[]`) {
			t.Fatalf("invalid answer %d %s", resp.Status, resp.Body)
		}
		// 202 and then answerAutocomplete by Tracker.
		answered := make(chan harness.Response, 1)
		w.fake.Handle(func(r fakeapp.Request) fakeapp.Response {
			var e envelope
			_ = json.Unmarshal(r.Body, &e)
			var p struct {
				RequestID string `json:"request_id"`
			}
			_ = json.Unmarshal(e.Payload, &p)
			go func() {
				answered <- w.trackerC.Do(context.Background(), harness.Request{Method: http.MethodPost, Path: "/command-autocompletions/" + p.RequestID + "/answer",
					Body: map[string]any{"choices": []any{map[string]any{"name": "Scanner", "value": "scanner"}}}})
			}()
			return fakeapp.Response{Status: http.StatusAccepted}
		})
		resp = ask()
		w.fake.Handle(nil)
		if a := <-answered; a.Status/100 != 2 {
			t.Fatalf("answerAutocomplete %d %s", a.Status, a.Body)
		}
		if resp.Status != http.StatusOK || !strings.Contains(string(resp.Body), `"scanner"`) {
			t.Fatalf("deferred answer %d %s", resp.Status, resp.Body)
		}
	})

	t.Run("TestAppAutocomplete_Gateway", func(t *testing.T) {
		w.t = t
		// Linker has no live gateway connection (M1-64/11): the relay finds none, so Alex gets the
		// empty list at once, never after command.autocomplete_timeout (R-AG-14).
		resp := w.do(alex, harness.Request{Method: http.MethodPost,
			Path: "/channels/" + w.m.Channels["Support/Allgemein"] + "/commands/" + w.link + "/autocomplete",
			Body: map[string]any{"option": "target", "value": "is"}})
		if resp.Status != http.StatusOK || !strings.Contains(string(resp.Body), `"choices":[]`) {
			t.Fatalf("choices %d %s", resp.Status, resp.Body)
		}
		if timeout, _ := time.ParseDuration(w.s.ConfigValue("command.autocomplete_timeout")); resp.Elapsed >= timeout {
			t.Fatalf("the person waited %v", resp.Elapsed)
		}
	})

	t.Run("TestReceiveRuleForApps", func(t *testing.T) {
		w.t = t
		poll := w.do(w.linkerC, harness.Request{Method: http.MethodGet, Path: "/agents/me/consumers/default/events"})
		if poll.Status == http.StatusOK {
			t.Fatalf("an app polled the agent events: %s", poll.Body)
		}
		w.setTrackerScopes("commands:write", "components:write", "messages:write", "events:subscribe")
		// A token takes the granted scopes of its creation (M2-24): Tracker needs a new one.
		tracker := w.s.NewTokenClient(t, &harness.Client{PrincipalID: w.tracker})
		w.fake.Respond(fakeapp.Response{Status: http.StatusAccepted})
		before := len(w.fake.Requests())
		w.want(w.invoke(alex, "Support/Allgemein", w.link, "", nil), http.StatusAccepted)
		w.want(w.invoke(alex, "Support/Eskalation", w.ticket, "", map[string]string{"title": "Own"}), http.StatusAccepted)
		ctx, cancel := w.patience()
		defer cancel()
		if _, err := w.fake.WaitRequests(ctx, before+1); err != nil {
			t.Fatal(err)
		}
		for _, c := range []struct {
			client      *harness.Client
			own, others string
		}{{tracker, w.ticket, w.link}, {w.linkerC, w.link, w.ticket}} {
			page := w.do(c.client, harness.Request{Method: http.MethodGet, Path: "/events?after=0&limit=500"})
			w.want(page, http.StatusOK)
			if !strings.Contains(string(page.Body), c.own) || strings.Contains(string(page.Body), `"command_id":"`+c.others+`"`) {
				t.Fatalf("catch-up of %s mixes the personal events of the apps", c.client.PrincipalID)
			}
		}
		w.setTrackerScopes("commands:write", "components:write", "messages:write")
	})

	t.Run("TestDeniedAppActionsAudited", func(t *testing.T) {
		w.t = t
		count := func(actor string) int {
			return w.count(`SELECT count(*) FROM audit_events WHERE decision = 'deny' AND action = 'command.invoke' AND actor_principal_id = $1`, actor)
		}
		samBefore, robinBefore := count(sam.PrincipalID), count(robin.PrincipalID)
		jobs := w.count(`SELECT count(*) FROM jobs.river_job WHERE kind = 'runs.app_dispatch'`)
		w.want(w.invoke(sam, "Support/Allgemein", w.ticket, "", map[string]string{"title": "x"}), http.StatusNotFound)
		w.want(w.invoke(sam, "Support/Eskalation", w.ticket, "", map[string]string{"title": "x"}), http.StatusNotFound)
		w.until("Sam's denials", func() bool { return count(sam.PrincipalID) >= samBefore+1 })
		w.want(w.invoke(robin, "Engineering/Projekt-A", w.link, w.m.Threads["Engineering/Projekt-A"], nil), http.StatusNotFound)
		w.until("Robin's denial", func() bool { return count(robin.PrincipalID) == robinBefore+1 })
		if n := w.count(`SELECT count(*) FROM jobs.river_job WHERE kind = 'runs.app_dispatch'`); n != jobs {
			t.Fatal("a denied invocation inserted a job")
		}
	})

	t.Run("TestAppCommandHiddenWithoutScopeOrWrite", func(t *testing.T) {
		w.t = t
		list := func() string {
			return string(w.do(alex, harness.Request{Method: http.MethodGet, Path: "/channels/" + w.m.Channels["Support/Allgemein"] + "/commands"}).Body)
		}
		get := func(id string) harness.Response {
			return w.do(alex, harness.Request{Method: http.MethodGet, Path: "/commands/" + id})
		}
		w.setLinkerScopes("components:write", "events:subscribe", "messages:write")
		if strings.Contains(list(), w.link) {
			t.Fatal("link listed without commands:write")
		}
		hidden, unknown := w.invoke(alex, "Support/Allgemein", w.link, "", nil), w.invoke(alex, "Support/Allgemein", ids.New().String(), "", nil)
		w.want(hidden, http.StatusNotFound)
		if !sameMap(stripCorrelation(hidden.Body), stripCorrelation(unknown.Body)) {
			t.Fatalf("hidden %s, unknown %s", hidden.Body, unknown.Body)
		}
		hiddenGet, unknownGet := get(w.link), get(ids.New().String())
		w.want(hiddenGet, http.StatusNotFound)
		if !sameMap(stripCorrelation(hiddenGet.Body), stripCorrelation(unknownGet.Body)) {
			t.Fatal("getCommand of a hidden command differs from an unknown one")
		}
		w.setLinkerScopes("commands:write", "components:write", "events:subscribe", "messages:write")
		if !strings.Contains(list(), w.link) {
			t.Fatal("link not listed with commands:write")
		}
		w.want(get(w.link), http.StatusOK)
		w.want(w.invoke(alex, "Support/Allgemein", w.link, "", nil), http.StatusAccepted)
		grant := w.text(`SELECT id::text FROM grants WHERE subject_principal_id = $1 AND target_channel_id = $2 AND revoked_at IS NULL`,
			w.linker, w.m.Channels["Support/Allgemein"])
		w.want(w.do(w.s.Owner, harness.Request{Method: http.MethodPost, Path: "/grants/" + grant + "/revoke", Body: map[string]any{"reason": "test"}}),
			http.StatusOK)
		if strings.Contains(list(), w.link) {
			t.Fatal("link listed without write")
		}
		w.want(w.invoke(alex, "Support/Allgemein", w.link, "", nil), http.StatusNotFound)
		w.want(get(w.link), http.StatusOK)
		w.grant(w.linker, "Support/Allgemein")
	})

	t.Run("TestUpgradeAppliesCommandDiff", func(t *testing.T) {
		w.t = t
		own := w.mustRegister(w.linkerC, "own")
		archive := map[string]any{"name": "archive", "description": "Archive an issue"}
		next := w.linkerManifest("1.1.0", archive)
		next["commands"].([]any)[0].(map[string]any)["description"] = "Link an issue of the tracker"
		version := w.count(`SELECT version FROM commands WHERE id = $1`, w.link)
		w.upgrade(w.linker, next)
		archiveID := w.commandID(w.linker, "archive")
		if w.count(`SELECT version FROM commands WHERE id = $1`, w.link) != version+1 {
			t.Fatal("the changed link kept its version")
		}
		w.upgrade(w.linker, w.linkerManifest("1.2.0"))
		if w.count(`SELECT count(*) FROM commands WHERE id = $1 AND removed_at IS NOT NULL`, archiveID) != 1 {
			t.Fatal("archive was not removed")
		}
		if w.count(`SELECT count(*) FROM event_log WHERE type = 'command.changed' AND resource_id = $1`, archiveID) != 2 {
			t.Fatal("archive lacks its two command.changed")
		}
		if w.count(`SELECT count(*) FROM commands WHERE id = $1 AND removed_at IS NULL`, own) != 1 {
			t.Fatal("a command registered through the API was removed")
		}
	})

	t.Run("TestSuspendHidesCommandsAndDisablesComponents", func(t *testing.T) {
		w.t = t
		card, comps, resp := w.postCard(w.linkerC, w.mainThread("Support/Allgemein"), "Suspend me", oneButton)
		w.want(resp, http.StatusCreated)
		version := w.count(`SELECT version FROM messages WHERE id = $1`, card)
		updates := w.count(`SELECT count(*) FROM event_log WHERE type = 'message.updated' AND resource_id = $1`, card)
		w.want(w.do(w.s.Owner, harness.Request{Method: http.MethodPost, Path: "/apps/" + w.linker + "/suspend"}), http.StatusNoContent)
		if w.text(`SELECT state FROM message_components WHERE id = $1`, comps["go"]) != "disabled" ||
			w.count(`SELECT version FROM messages WHERE id = $1`, card) != version+1 ||
			w.count(`SELECT count(*) FROM event_log WHERE type = 'message.updated' AND resource_id = $1`, card) != updates+1 {
			t.Fatal("the card of the suspended app is not disabled with one message.updated")
		}
		w.want(w.invoke(alex, "Support/Allgemein", w.link, "", nil), http.StatusNotFound)
		w.wantProblem(w.do(w.linkerC, harness.Request{Method: http.MethodGet, Path: "/events?after=0&limit=5"}), http.StatusUnauthorized, "unauthenticated", "")
		w.want(w.do(w.s.Owner, harness.Request{Method: http.MethodPost, Path: "/apps/" + w.linker + "/resume"}), http.StatusNoContent)
		w.want(w.invoke(alex, "Support/Allgemein", w.link, "", nil), http.StatusAccepted)
		if w.text(`SELECT state FROM message_components WHERE id = $1`, comps["go"]) != "disabled" {
			t.Fatal("resume brought a component back")
		}
		w.wantProblem(w.click(alex, comps["go"], ""), http.StatusConflict, "component_used", "")
	})

	t.Run("TestLifecycleRacesInvoke", func(t *testing.T) {
		w.t = t
		start := make(chan struct{})
		got := make(chan harness.Response, 1)
		go func() {
			<-start
			got <- alex.Do(context.Background(), harness.Request{Method: http.MethodPost,
				Path: "/channels/" + w.m.Channels["Support/Allgemein"] + "/commands/" + w.link + "/invoke", Body: map[string]any{}})
		}()
		close(start)
		w.want(w.do(w.s.Owner, harness.Request{Method: http.MethodPost, Path: "/apps/" + w.linker + "/suspend"}), http.StatusNoContent)
		resp := <-got
		if resp.Status != http.StatusAccepted && resp.Status != http.StatusNotFound {
			t.Fatalf("invoke during suspend: %d %s", resp.Status, resp.Body)
		}
		w.until("no pending dispatch", func() bool {
			return w.count(`SELECT count(*) FROM jobs.river_job WHERE kind = 'runs.app_dispatch' AND state NOT IN ('completed', 'discarded', 'cancelled')`) == 0
		})
		w.want(w.do(w.s.Owner, harness.Request{Method: http.MethodPost, Path: "/apps/" + w.linker + "/resume"}), http.StatusNoContent)
	})

	t.Run("TestUninstallRemovesCommandsKeepsMessages", func(t *testing.T) {
		w.t = t
		card, comps, resp := w.postCard(w.linkerC, w.mainThread("Support/Allgemein"), "Uninstall me", oneButton)
		w.want(resp, http.StatusCreated)
		w.want(w.do(w.s.Owner, harness.Request{Method: http.MethodPost, Path: "/apps/" + w.linker + "/uninstall", Body: map[string]any{"reason": "test"}}),
			http.StatusNoContent)
		if w.count(`SELECT count(*) FROM commands WHERE id = $1 AND removed_at IS NOT NULL`, w.link) != 1 {
			t.Fatal("link was not removed")
		}
		if w.text(`SELECT state FROM message_components WHERE id = $1`, comps["go"]) != "disabled" ||
			w.text(`SELECT author_principal_id::text FROM messages WHERE id = $1`, card) != w.linker {
			t.Fatal("the card is not disabled or lost its author")
		}
		details := w.text(`SELECT details::text FROM audit_events WHERE action = 'app.uninstall' AND resource_id = $1 ORDER BY occurred_at DESC LIMIT 1`, w.linker)
		if !strings.Contains(details, w.link) || !strings.Contains(details, "components_disabled") {
			t.Fatalf("audit details of the uninstall lack the command or the count")
		}
	})
}

// sameMap compares two decoded JSON objects.
func sameMap(a, b map[string]any) bool {
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	return string(x) == string(y)
}

// ifMatch is an If-Match header for version.
func ifMatch(version int64) http.Header {
	h := http.Header{}
	h.Set("If-Match", `"`+strings.TrimSpace(string(mustJSON(version)))+`"`)
	return h
}

// mustJSON encodes v.
func mustJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

// sqlGrant gives the person a level on the space through raw SQL, beside the rights checks: a
// fixture that simulates an I1 defect (OQ-01), as M2-15 does. It returns the grant; the caller
// drops it again, because every later rights change refuses a world that breaks I1.
func (w *world) sqlGrant(person, space, level string) string {
	w.t.Helper()
	id := ids.New().String()
	w.exec(`INSERT INTO grants (id, subject_kind, subject_principal_id, target_kind, target_space_id, level, capabilities, reason, granted_by)
		VALUES ($1, 'user', $2, 'space', $3, $4, '{}', 'I1 defect fixture', $5)`, id, person, w.m.Spaces[space], level, w.s.Owner.PrincipalID)
	// The cache of the permission snapshots is keyed by permission_version.
	w.exec(`UPDATE sequence_counters SET value = value + 1 WHERE name = 'permission_version'`)
	return id
}

// dropGrant deletes the fixture grant of sqlGrant and raises the permission version.
func (w *world) dropGrant(id string) {
	w.t.Helper()
	w.exec(`DELETE FROM grants WHERE id = $1`, id)
	w.exec(`UPDATE sequence_counters SET value = value + 1 WHERE name = 'permission_version'`)
}

// exec runs a statement on the admin connection.
func (w *world) exec(q string, args ...any) {
	w.t.Helper()
	if _, err := w.s.AdminDB().Exec(w.t.Context(), q, args...); err != nil {
		w.t.Fatal(err)
	}
}

// freeResponse sets free_response on the channel key through putChannelBehavior.
func (w *world) freeResponse(channel string) {
	w.t.Helper()
	path := "/channels/" + w.m.Channels[channel] + "/behavior"
	cur := harness.MustDo(w.t, w.s.Owner, harness.Request{Method: http.MethodGet, Path: path}, http.StatusOK)
	var b map[string]any
	cur.JSON(w.t, &b)
	delete(b, "settings_version")
	b["free_response"] = true
	h := http.Header{}
	h.Set("If-Match", cur.Header.Get("ETag"))
	harness.MustDo(w.t, w.s.Owner, harness.Request{Method: http.MethodPut, Path: path, Body: b, Header: h}, http.StatusOK)
}

// setTrackerScopes replaces the granted scopes of Tracker.
func (w *world) setTrackerScopes(scopes ...string) { w.setScopes(w.tracker, scopes) }

// setLinkerScopes replaces the granted scopes of Linker and keeps its event subscription when
// events:subscribe stays.
func (w *world) setLinkerScopes(scopes ...string) { w.setScopes(w.linker, scopes) }

// setScopes replaces the granted scopes of app.
func (w *world) setScopes(app string, scopes []string) {
	w.t.Helper()
	body := map[string]any{"granted_scopes": scopes, "reason": "test"}
	if app == w.linker && slices.Contains(scopes, "events:subscribe") {
		body["event_subscriptions"] = []string{"command.invoked"}
	}
	harness.MustDo(w.t, w.s.Owner, harness.Request{Method: http.MethodPut, Path: "/apps/" + app + "/scopes", Body: body}, http.StatusOK)
}

// upgrade replaces the manifest of app.
func (w *world) upgrade(app string, manifest map[string]any) {
	w.t.Helper()
	body := map[string]any{"manifest": manifest, "reason": "test",
		"granted_scopes": []string{"commands:write", "components:write", "events:subscribe", "messages:write"}, "event_subscriptions": []string{"command.invoked"}}
	harness.MustDo(w.t, w.s.Owner, harness.Request{Method: http.MethodPut, Path: "/apps/" + app + "/manifest", Body: body}, http.StatusOK)
}

// mustRegister registers an instance-wide command name as c through registerCommand.
func (w *world) mustRegister(c *harness.Client, name string) string {
	w.t.Helper()
	var cmd struct {
		ID string `json:"id"`
	}
	harness.MustDo(w.t, c, harness.Request{Method: http.MethodPost, Path: "/commands", Body: map[string]any{"name": name, "description": "Own command"}},
		http.StatusCreated).JSON(w.t, &cmd)
	return cmd.ID
}
