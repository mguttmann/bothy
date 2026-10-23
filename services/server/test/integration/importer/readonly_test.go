// SPDX-License-Identifier: Apache-2.0

package importer_test

import (
	"encoding/json"
	"net/http"
	"strconv"
	"sync"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/test/harness"
)

// imported runs the structure batch on a running source and returns the source and the targets.
func (w *iw) imported() (string, map[string]item) {
	w.t.Helper()
	id := w.running()
	return id, byKey(w.mustBatch(id, structure()...))
}

// mainThread returns the main thread of a text channel as Robin sees it.
func (w *iw) mainThread(channel string) string {
	w.t.Helper()
	_, ch, _ := w.channel(w.robin, channel)
	s, _ := ch["main_thread_id"].(string)
	if s == "" {
		w.t.Fatalf("channel %s has no main thread: %v", channel, ch)
	}
	return s
}

// activeRun gives the text channel the operator Lotse and opens a thread whose first message
// queues a run of Lotse (09 3.2); it returns the run.
func (w *iw) activeRun(channel string) string {
	w.t.Helper()
	_, _, etag := w.channel(w.robin, channel)
	w.must(w.robin, http.MethodPut, "/channels/"+channel+"/operator", map[string]any{"mode": "agent", "agent_id": w.lotse.PrincipalID,
		"reason": "switch-over"}, ifMatch(etag), http.StatusOK)
	parent := w.id(w.must(w.robin, http.MethodPost, "/threads/"+w.mainThread(channel)+"/messages",
		map[string]any{"client_msg_id": ids.New().String(), "content_md": "question"}, nil, http.StatusCreated))
	var th struct {
		Thread struct {
			ID string `json:"id"`
		} `json:"thread"`
	}
	w.must(w.robin, http.MethodPost, "/channels/"+channel+"/threads", map[string]any{"title": "Work", "parent_message_id": parent,
		"first_message": map[string]any{"client_msg_id": ids.New().String(), "content_md": "please help"}}, nil, http.StatusCreated).JSON(w.t, &th)
	var run string
	if err := w.s.AdminDB().QueryRow(w.ctx, `SELECT id::text FROM runs WHERE thread_id = $1 AND state IN ('queued', 'running', 'waiting_input')`,
		th.Thread.ID).Scan(&run); err != nil {
		w.t.Fatalf("no active run in the thread: %v", err)
	}
	return run
}

// cancelRequested reports run.cancel_requested of run with reason channel_read_only among ev.
func cancelRequested(ev []event, run string) bool {
	for _, e := range ev {
		if e.Type != "run.cancel_requested" || e.ResourceID != run {
			continue
		}
		var p struct {
			Reason string `json:"reason"`
		}
		if json.Unmarshal(e.Payload, &p) == nil && p.Reason == "channel_read_only" {
			return true
		}
	}
	return false
}

// TestNewChannelNoGrants (22 5.1 rules 2 and 3): Alex does not see the imported channel; Robin
// does; it is read-only with lead discord and has no operator.
func TestNewChannelNoGrants(t *testing.T) {
	t.Parallel()
	w := newIW(t)
	_, items := w.imported()
	text := items["channel:"+chText].TargetID
	if status, _, _ := w.channel(w.alex, text); status != http.StatusNotFound {
		t.Errorf("Alex reads the imported channel: %d", status)
	}
	status, ch, _ := w.channel(w.robin, text)
	op, _ := ch["operator"].(map[string]any)
	if status != http.StatusOK || ch["read_only"] != true || op["mode"] != "none" {
		t.Errorf("Robin: %d %v", status, ch)
	}
	if _, ch, _ := w.channel(w.robin, items["channel:"+chHere].TargetID); ch["read_only"] != false {
		t.Errorf("a channel led here is writable: %v", ch)
	}
	if n := w.count(`SELECT count(*) FROM grants WHERE target_channel_id = $1`, text); n != 0 {
		t.Errorf("%d grants on the imported channel", n)
	}
}

// TestReadOnlyRejectsOwner_ImportApplySucceeds (plan acceptance, 22 16 "Read-only in the mirror"):
// Robin's createMessage in the read-only channel is 403 channel_read_only; a forum tag batch into
// the read-only forum succeeds.
func TestReadOnlyRejectsOwner_ImportApplySucceeds(t *testing.T) {
	t.Parallel()
	w := newIW(t)
	id, items := w.imported()
	w.wantProblem(w.message(w.robin, w.mainThread(items["channel:"+chText].TargetID)), http.StatusForbidden, "channel_read_only", "")
	tag := w.mustBatch(id, obj("forum_tag:"+chForum+":10", "forum_tag", map[string]any{"channel_origin_key": "channel:" + chForum, "name": "feature"}))
	if tag[0].State != "posted" || tag[0].TargetKind != "forum_tag" {
		t.Errorf("tag %+v", tag)
	}
	if n := w.count(`SELECT count(*) FROM forum_tags WHERE channel_id = $1 AND deleted_at IS NULL`, items["channel:"+chForum].TargetID); n != 2 {
		t.Errorf("%d tags in the forum", n)
	}
}

// TestSetImportLead_ReadOnlyInSameTransaction (plan acceptance, 22 12.1, 13.3, 13.4): lead `bothy`
// lifts read-only and lead discord sets it, each with channel.updated and import.progress in the
// same commit; setting it cancels the active run of the channel with channel_read_only.
func TestSetImportLead_ReadOnlyInSameTransaction(t *testing.T) {
	t.Parallel()
	w := newIW(t)
	id, items := w.imported()
	text := items["channel:"+chText].TargetID
	lead := func(value string) []event {
		seq := w.maxSeq()
		w.must(w.robin, http.MethodPut, "/imports/"+id+"/leads", map[string]any{"source_channel": chText, "lead": value}, nil, http.StatusOK)
		return w.events(seq)
	}
	sameCommit := func(ev []event, types ...string) {
		t.Helper()
		commit := ""
		for _, typ := range types {
			found := false
			for _, e := range ev {
				if e.Type == typ && (commit == "" || e.Commit == commit) {
					commit, found = e.Commit, true
				}
			}
			if !found {
				t.Errorf("%s missing or in another commit: %+v", typ, ev)
			}
		}
	}
	ev := lead("bothy")
	sameCommit(ev, "channel.updated", "import.progress")
	if _, ch, _ := w.channel(w.robin, text); ch["read_only"] != false {
		t.Fatalf("lead here keeps read-only: %v", ch)
	}
	if s := w.get(id); s.Config["channels"].([]any)[0].(map[string]any)["lead"] != "bothy" {
		t.Errorf("stored lead %v", s.Config["channels"])
	}
	run := w.activeRun(text)
	if ev := lead("bothy"); len(ev) != 0 {
		t.Errorf("an equal lead appended %+v", ev)
	}
	ev = lead("discord")
	sameCommit(ev, "channel.updated", "import.progress", "run.cancel_requested")
	if !cancelRequested(ev, run) {
		t.Errorf("no run.cancel_requested with channel_read_only for %s in %+v", run, ev)
	}
	if _, ch, _ := w.channel(w.robin, text); ch["read_only"] != true {
		t.Errorf("lead discord keeps the channel writable: %v", ch)
	}
	w.wantProblem(w.must(w.robin, http.MethodPut, "/imports/"+id+"/leads", map[string]any{"source_channel": "999", "lead": "bothy"}, nil,
		http.StatusNotFound), http.StatusNotFound, "not_found", "")
	w.wantProblem(w.must(w.robin, http.MethodPut, "/imports/"+id+"/leads", map[string]any{"source_channel": chSkip, "lead": "bothy"}, nil,
		http.StatusNotFound), http.StatusNotFound, "not_found", "")
	w.wantProblem(w.must(w.robin, http.MethodPut, "/imports/"+id+"/leads", map[string]any{"source_channel": chCollide, "lead": "bothy"}, nil,
		http.StatusConflict), http.StatusConflict, "conflict", "channel_not_imported")
	if n := w.count(`SELECT count(*) FROM audit_events WHERE action = 'import.manage' AND resource_id = $1 AND details->>'op' = 'lead'`, id); n < 3 {
		t.Errorf("%d audit entries of lead changes", n)
	}
}

// TestSetChannelReadOnly (05 channel.read_only.set): step-up, If-Match, audit with the reason,
// active runs cancelled; on a channel of a running import 409 import_lead_controls, allowed again
// after the source is completed.
func TestSetChannelReadOnly(t *testing.T) {
	t.Parallel()
	w := newIW(t)
	plain := w.existingChannel
	body := map[string]any{"read_only": true, "reason": "maintenance"}
	w.wantProblem(w.do(w.robin, http.MethodPut, "/channels/"+plain+"/read-only", body, nil), http.StatusPreconditionRequired, "precondition_required", "")
	run := w.activeRun(plain)
	_, _, etag := w.channel(w.robin, plain)
	fresh, err := w.s.UserSession(w.ctx, w.robin.PrincipalID)
	if err != nil {
		t.Fatal(err)
	}
	w.wantProblem(w.do(fresh, http.MethodPut, "/channels/"+plain+"/read-only", body, ifMatch(etag)), http.StatusUnauthorized, "step_up_required", "")
	seq := w.maxSeq()
	w.must(w.robin, http.MethodPut, "/channels/"+plain+"/read-only", body, ifMatch(etag), http.StatusOK)
	if !cancelRequested(w.events(seq), run) {
		t.Errorf("the run %s was not cancelled", run)
	}
	if n := w.count(`SELECT count(*) FROM audit_events WHERE action = 'channel.read_only.set' AND resource_id = $1 AND details->>'reason' = 'maintenance'`,
		plain); n != 1 {
		t.Errorf("%d audit entries", n)
	}
	w.wantProblem(w.message(w.robin, w.existingMainThrd), http.StatusForbidden, "channel_read_only", "")
	if status, _, _ := w.channel(w.alex, plain); status != http.StatusNotFound {
		t.Errorf("Alex sees the channel: %d", status)
	}

	id, items := w.imported()
	text := items["channel:"+chText].TargetID
	_, _, etag = w.channel(w.robin, text)
	w.wantProblem(w.must(w.robin, http.MethodPut, "/channels/"+text+"/read-only", map[string]any{"read_only": false, "reason": "x"}, ifMatch(etag),
		http.StatusConflict), http.StatusConflict, "conflict", "import_lead_controls")
	for _, ch := range []string{chText, chForum} {
		w.must(w.robin, http.MethodPut, "/imports/"+id+"/leads", map[string]any{"source_channel": ch, "lead": "bothy"}, nil, http.StatusOK)
	}
	w.must(w.robin, http.MethodPost, "/imports/"+id+"/complete", nil, nil, http.StatusOK)
	_, _, etag = w.channel(w.robin, text)
	w.must(w.robin, http.MethodPut, "/channels/"+text+"/read-only", map[string]any{"read_only": true, "reason": "archive copy"}, ifMatch(etag), http.StatusOK)
}

// TestPlaceholderNeverLogsInOrTriggers (22 5.3, R-IM-06): the placeholders are disabled people
// without identity, credential, grant, group or owner row; they cannot be signed in and the owner
// cannot mint a token for them. That authorize refuses a disabled principal message.create and
// agent.invoke is TestPlaceholderNeverTriggers of internal/authz/app.
func TestPlaceholderNeverLogsInOrTriggers(t *testing.T) {
	t.Parallel()
	w := newIW(t)
	_, items := w.imported()
	for key, want := range map[string]struct{ handle, name string }{
		"user:" + userSam:  {"imported-sam.example", "Sam"},
		"user:" + userHook: {"imported-deploy-hook", "Deploy Hook"},
	} {
		id := items[key].TargetID
		var u struct {
			Handle      string `json:"handle"`
			DisplayName string `json:"display_name"`
			Status      string `json:"status"`
		}
		w.must(w.robin, http.MethodGet, "/users/"+id, nil, nil, http.StatusOK).JSON(t, &u)
		if u.Handle != want.handle || u.DisplayName != want.name || u.Status != "disabled" {
			t.Errorf("%s: %+v", key, u)
		}
		for _, q := range []string{
			`SELECT count(*) FROM user_identities WHERE user_id = $1`,
			`SELECT count(*) FROM local_credentials c JOIN user_identities i ON i.id = c.identity_id WHERE i.user_id = $1`,
			`SELECT count(*) FROM owners WHERE user_id = $1`, `SELECT count(*) FROM api_tokens WHERE principal_id = $1`,
			`SELECT count(*) FROM devices WHERE user_id = $1`, `SELECT count(*) FROM grants WHERE subject_principal_id = $1`,
			`SELECT count(*) FROM group_memberships WHERE user_id = $1`,
		} {
			if n := w.count(q, id); n != 0 {
				t.Errorf("%s: %d rows of %s", key, n, q)
			}
		}
		if _, err := w.s.UserSession(w.ctx, id); err == nil {
			t.Errorf("%s could be signed in", key)
		}
	}
}

// TestSetCursor_ReplaceAndStatsNamespaces (design note 10): the cursor is replaced, the tool's
// counters are merged under tool and never touch server.
func TestSetCursor_ReplaceAndStatsNamespaces(t *testing.T) {
	t.Parallel()
	w := newIW(t)
	id, _ := w.imported()
	before := w.serverStats(id)
	put := func(body map[string]any) source {
		var s source
		w.must(w.robin, http.MethodPut, "/imports/"+id+"/cursor", body, nil, http.StatusOK).JSON(t, &s)
		return s
	}
	put(map[string]any{"cursor": map[string]any{"version": 1, "channels": map[string]any{chText: "5"}}, "stats": map[string]any{"messages": 3, "server": 9}})
	s := put(map[string]any{"cursor": map[string]any{"version": 1, "run": map[string]any{"id": "r1"}}, "stats": map[string]any{"threads": 2}})
	if _, ok := s.Cursor["channels"]; ok || s.Cursor["run"] == nil {
		t.Errorf("the cursor was not replaced: %v", s.Cursor)
	}
	tool, _ := s.Stats["tool"].(map[string]any)
	if tool["messages"] != float64(3) || tool["threads"] != float64(2) || tool["server"] != float64(9) {
		t.Errorf("tool stats %v", tool)
	}
	b1, _ := json.Marshal(before)
	b2, _ := json.Marshal(s.Stats["server"])
	if string(b1) != string(b2) {
		t.Errorf("server stats changed: %s then %s", b1, b2)
	}
}

// TestGetImportReturnsCursor: the stored cursor comes back unchanged from getImport and
// listImports.
func TestGetImportReturnsCursor(t *testing.T) {
	t.Parallel()
	w := newIW(t)
	id := w.running()
	cursor := map[string]any{"version": float64(1), "run": map[string]any{"id": "r1", "renewed_at": "2026-10-14T09:00:00.000Z"},
		"threads": map[string]any{"1029384700": "1029384799"}}
	w.must(w.robin, http.MethodPut, "/imports/"+id+"/cursor", map[string]any{"cursor": cursor}, nil, http.StatusOK)
	got, _ := json.Marshal(w.get(id).Cursor)
	want, _ := json.Marshal(cursor)
	if string(got) != string(want) {
		t.Errorf("cursor %s, want %s", got, want)
	}
	var page struct {
		Items []source `json:"items"`
	}
	w.must(w.robin, http.MethodGet, "/imports", nil, nil, http.StatusOK).JSON(t, &page)
	if len(page.Items) != 1 || page.Items[0].ID != id || page.Items[0].Cursor["run"] == nil {
		t.Errorf("list %+v", page.Items)
	}
}

// TestPlaceholderConcurrentSources (design note 8, red team finding M1): two running sources
// import the same new author at the same time; both batches succeed, each with its own
// placeholder and handle, because a handle the other batch took moves on to the next counter.
func TestPlaceholderConcurrentSources(t *testing.T) {
	t.Parallel()
	w := newIW(t)
	sources := []string{w.running(), w.running()}
	for round := range 4 {
		key := "user:" + strconv.Itoa(600+round)
		data := map[string]any{"discord_kind": "user", "username": "kim" + strconv.Itoa(round)}
		var wg sync.WaitGroup
		results := make([]harness.Response, len(sources))
		for i, id := range sources {
			wg.Add(1)
			go func(i int, id string) {
				defer wg.Done()
				results[i] = w.robin.Do(w.ctx, harness.Request{Method: http.MethodPost, Path: "/imports/" + id + "/batches",
					Body: map[string]any{"items": []any{obj(key, "user", data)}}})
			}(i, id)
		}
		wg.Wait()
		targets := map[string]bool{}
		for i, r := range results {
			if r.Status != http.StatusOK {
				t.Fatalf("round %d batch %d = %d %s", round, i, r.Status, r.Body)
			}
			var b struct {
				Items []item `json:"items"`
			}
			r.JSON(t, &b)
			targets[b.Items[0].TargetID] = true
		}
		if len(targets) != 2 {
			t.Errorf("round %d: targets %v", round, targets)
		}
	}
}
