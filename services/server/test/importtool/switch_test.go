// SPDX-License-Identifier: Apache-2.0

package importtool

import (
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/cmd/bothy/importtool"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/test/harness"
)

// IDs of the switch-over test: a message an agent still works on in Discord and a message written
// in Discord after the switch-over.
const (
	msgBusySwitch = "13001030"
	msgLate       = "13001031"
)

// switchMapping is mapping() with Lotse as the planned operator of general, which stays led by
// Discord until the switch-over.
func switchMapping() map[string]any {
	m := mapping()
	for _, c := range m["channels"].([]any) {
		e := c.(map[string]any)
		if e["discord_channel"] == chGeneral {
			e["planned_operator"] = "agent:lotse"
		}
	}
	return m
}

// switchWorld is the state of TestSwitchAndRevertAgainstRealServer: the world, the source, the
// session file of the tool, the mapping file, the Bothy channel of general and the imported thread
// thRelease.
type switchWorld struct {
	*world
	src, session, mapFile, channel, thread string
}

// tool runs bothy import discord with args and fails unless it exits with want.
func (s *switchWorld) tool(want int, args ...string) result {
	s.t.Helper()
	res := s.invoke(run{session: s.session}, args...)
	if res.code != want {
		s.t.Fatalf("%v: exit %d, want %d\nstdout: %s\nstderr: %s", args, res.code, want, res.stdout, res.stderr)
	}
	return res
}

// mirrorOnce runs one round of the read mirror.
func (s *switchWorld) mirrorOnce() result {
	s.t.Helper()
	return s.tool(importtool.ExitOK, "--guild", guildID, "--map", s.mapFile, "--apply", "--once")
}

// switchOver runs switch --discord-locked for general.
func (s *switchWorld) switchOver(want int) result {
	s.t.Helper()
	return s.tool(want, "switch", "--guild", guildID, "--map", s.mapFile, "--channel", chGeneral, "--discord-locked")
}

// lead sets the lead of general in the web console (setImportLead as Robin with step-up).
func (s *switchWorld) lead(value string) {
	s.t.Helper()
	s.must(s.robin, http.MethodPut, "/imports/"+s.src+"/leads", map[string]any{"source_channel": chGeneral, "lead": value}, nil, http.StatusOK)
}

// operator sets the operator of general in the web console (setChannelOperator as Robin with
// step-up and the version of the channel): Lotse, or no operator for "".
func (s *switchWorld) operator(agent string) {
	s.t.Helper()
	etag := s.must(s.robin, http.MethodGet, "/channels/"+s.channel, nil, nil, http.StatusOK).Header.Get("ETag")
	body := map[string]any{"mode": "none", "reason": "revert"}
	if agent != "" {
		body = map[string]any{"mode": "agent", "agent_id": agent, "reason": "switch-over"}
	}
	h := http.Header{}
	h.Set("If-Match", etag)
	s.must(s.robin, http.MethodPut, "/channels/"+s.channel+"/operator", body, h, http.StatusOK)
}

// dbNow returns the clock of the database, the time runs are queued with.
func (s *switchWorld) dbNow() time.Time {
	s.t.Helper()
	var at time.Time
	if err := s.s.AdminDB().QueryRow(s.ctx, `SELECT clock_timestamp()`).Scan(&at); err != nil {
		s.t.Fatal(err)
	}
	return at
}

// oneLeads holds the invariant of 22 12.1 after a step that began at since: the stored lead of
// general is discord exactly when its Bothy channel is read-only and no run of Lotse queued since
// is active there, and bothy exactly when the channel is writable.
func (s *switchWorld) oneLeads(step string, since time.Time) {
	s.t.Helper()
	lead := s.text(`SELECT coalesce(c->>'lead', 'discord') FROM import_sources s, jsonb_array_elements(s.config->'channels') c
		WHERE s.id = $1 AND c->>'discord_channel' = $2`, s.src, chGeneral)
	readOnly := s.text(`SELECT read_only::text FROM channels WHERE id = $1`, s.channel) == "true"
	active := s.count(`SELECT count(*) FROM runs r JOIN threads t ON t.id = r.thread_id WHERE t.channel_id = $1 AND r.agent_id = $2
		AND r.state IN ('queued', 'running', 'waiting_input') AND r.queued_at >= $3`, s.channel, s.lotse.PrincipalID, since)
	switch {
	case lead == "discord" && (!readOnly || active != 0):
		s.t.Fatalf("after %s: lead discord, but read-only %v and %d runs of Lotse started since", step, readOnly, active)
	case lead == "bothy" && readOnly:
		s.t.Fatalf("after %s: lead bothy, but the channel is read-only", step)
	case lead != "discord" && lead != "bothy":
		s.t.Fatalf("after %s: stored lead %q", step, lead)
	}
}

// copies returns how often the Discord message id exists as a posted item and as an imported
// message.
func (s *switchWorld) copies(id string) (int, int) {
	s.t.Helper()
	items := s.count(`SELECT count(*) FROM import_items WHERE source_id = $1 AND origin_key = $2 AND state = 'posted'`, s.src, "message:"+id)
	msgs := s.count(`SELECT count(*) FROM messages m JOIN import_items i ON i.id = m.import_item_id WHERE i.origin_key = $1`, "message:"+id)
	return items, msgs
}

// TestSwitchAndRevertAgainstRealServer (plan acceptance, 22 13.3 and 13.4: "integration test:
// mirror, switchover, test trigger, revert; two places never lead at the same time"): against the
// real server, with Robin's step-up session for the web console and Lotse's client for the agent.
// After every step exactly one side leads (22 12.1).
func TestSwitchAndRevertAgainstRealServer(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	s := &switchWorld{world: w}

	// 1. Seed: import general with the session reference of thRelease and run the mirror; Alex
	// may trigger in the target space, Lotse may write in the channel.
	since := s.dbNow()
	s.src, s.session, s.mapFile = w.sourceWith(switchMapping()), w.login(), w.mapFile(switchMapping())
	w.mustApply(run{session: s.session, mapFile: s.mapFile, extra: []string{"--sessions", w.sessionsFile()}}, importtool.ExitOK)
	s.channel, s.thread = w.target(s.src, "channel:"+chGeneral), w.target(s.src, "thread:"+thRelease)
	for _, g := range []map[string]any{
		{"subject_kind": "user", "subject_id": w.alex.PrincipalID, "level": "trigger", "target_kind": "space",
			"target_id": w.target(s.src, "space:"+catProjects)},
		{"subject_kind": "agent", "subject_id": w.lotse.PrincipalID, "level": "write", "target_kind": "channel", "target_id": s.channel},
	} {
		g["reason"] = "imported channel"
		w.must(w.s.Owner, http.MethodPost, "/grants", g, nil, http.StatusCreated)
	}
	w.must(w.lotse, http.MethodGet, "/agents/me/consumers/default/events", nil, nil, http.StatusOK)
	s.mirrorOnce()
	var view struct {
		Channel struct {
			Operator struct {
				Mode string `json:"mode"`
			} `json:"operator"`
		} `json:"channel"`
	}
	w.must(w.robin, http.MethodGet, "/channels/"+s.channel, nil, nil, http.StatusOK).JSON(t, &view)
	if view.Channel.Operator.Mode != "none" {
		t.Fatalf("operator mode %s in the mirror, want none", view.Channel.Operator.Mode)
	}
	s.oneLeads("seed", since)

	// 2. Discord locked, but an agent still works on a message: switch holds; once the reaction
	// is gone, switch copies it once and prints step 3.
	since = s.dbNow()
	now := w.clk.Now()
	stamp := func(at time.Time) string { return at.UTC().Format("2006-01-02T15:04:05.000000+00:00") }
	busy := map[string]any{"reactions": []any{map[string]any{"count": 1, "emoji": map[string]any{"name": busyEmoji}}}}
	w.fake.AddMessage(chGeneral, message(msgBusySwitch, chGeneral, 0, sam, "Working on it", stamp(now.Add(-mirrorTimeout)), busy))
	res := s.switchOver(importtool.ExitAborted)
	if !strings.Contains(res.stderr, "1 messages are still held back (busy 1)") || strings.Contains(res.stderr, "Step 3") {
		t.Fatalf("switch with a busy message:\n%s", res.stderr)
	}
	if items, _ := s.copies(msgBusySwitch); items != 0 {
		t.Fatal("the busy message was copied")
	}
	s.oneLeads("switch held", since)
	w.fake.EditMessage(chGeneral, msgBusySwitch, map[string]any{"reactions": nil})
	res = s.switchOver(importtool.ExitOK)
	if !strings.Contains(res.stderr, "Step 3") || !strings.Contains(res.stderr, "agent:lotse") {
		t.Fatalf("switch printed no step 3:\n%s", res.stderr)
	}
	if items, msgs := s.copies(msgBusySwitch); items != 1 || msgs != 1 {
		t.Fatalf("the settled message: %d items, %d messages, want one each", items, msgs)
	}
	s.oneLeads("switch", since)

	// 3. The web console: Bothy leads, then the planned operator.
	since = s.dbNow()
	s.lead("bothy")
	s.oneLeads("lead bothy", since)
	s.operator(w.lotse.PrincipalID)
	s.oneLeads("operator set", since)

	// 4. Probe: Alex writes in the imported thread; exactly one run of Lotse, the imported session
	// binds, the answer of Lotse is in Bothy.
	since = s.dbNow()
	w.post(s.thread, "Please continue with the release plan")
	runs := w.runs(s.thread)
	if len(runs) != 1 {
		t.Fatalf("%d runs after the probe, want 1", len(runs))
	}
	if p := w.invocation(s.thread, runs[0]); p["session_mode"] != "bind" {
		t.Fatalf("the probe invocation has session_mode %v, want bind", p["session_mode"])
	}
	w.start(runs[0])
	if n := w.count(`SELECT count(*) FROM messages WHERE thread_id = $1 AND run_id = $2 AND author_principal_id = $3`, s.thread, runs[0],
		w.lotse.PrincipalID); n != 1 {
		t.Fatalf("%d answers of Lotse in the thread, want 1", n)
	}
	w.finish(runs[0])
	s.oneLeads("probe", since)

	// 5. A late Discord message in the switched channel: the round warns and copies nothing.
	since = s.dbNow()
	w.fake.AddMessage(chGeneral, message(msgLate, chGeneral, 0, alex, "Written in Discord after all", stamp(now.Add(-mirrorTimeout)), nil))
	if res := s.mirrorOnce(); !strings.Contains(res.stdout, "written in Discord after the switch-over") {
		t.Fatalf("no warning for the late message:\n%s", res.stdout)
	}
	if items, _ := s.copies(msgLate); items != 0 {
		t.Fatal("the late message was copied while Bothy leads")
	}
	s.oneLeads("late message", since)

	// 6. revert only reads: the thread with the two messages written in Bothy (Alex and Lotse).
	since = s.dbNow()
	before := w.count(`SELECT count(*) FROM audit_events`)
	res = s.tool(importtool.ExitOK, "revert", "--guild", guildID, "--channel", chGeneral)
	if !strings.Contains(res.stdout, s.thread+": 2\n") || !strings.Contains(res.stdout, "Total: 2\n") {
		t.Fatalf("revert output:\n%s", res.stdout)
	}
	if strings.Contains(res.stdout, "release plan") || strings.Contains(res.stdout, "Alex") {
		t.Fatalf("revert prints content or names:\n%s", res.stdout)
	}
	if after := w.count(`SELECT count(*) FROM audit_events`); after != before {
		t.Fatalf("revert wrote %d audit entries", after-before)
	}
	s.oneLeads("revert", since)

	// 7. A run of Lotse is active when the console gives the lead back: it is cancelled with
	// channel_read_only; then the console removes the operator, and Alex can no longer write.
	w.post(s.thread, "One more question")
	runs = w.runs(s.thread)
	active := runs[len(runs)-1]
	w.start(active)
	since = s.dbNow()
	s.lead("discord")
	cancelled := false
	for _, p := range w.payloads("run.cancel_requested", s.thread) {
		if p["reason"] == "channel_read_only" && strings.Contains(string(mustJSON(t, p)), active) {
			cancelled = true
		}
	}
	if !cancelled {
		t.Fatalf("run %s got no run.cancel_requested with channel_read_only", active)
	}
	s.oneLeads("lead discord", since)
	s.operator("")
	resp := w.alex.Do(w.ctx, harness.Request{Method: http.MethodPost, Path: "/threads/" + s.thread + "/messages",
		Body: map[string]any{"client_msg_id": ids.New().String(), "content_md": "Still there?"}})
	var problem struct {
		Code string `json:"code"`
	}
	_ = json.Unmarshal(resp.Body, &problem)
	if resp.Status != http.StatusForbidden || problem.Code != "channel_read_only" {
		t.Fatalf("Alex writes after the revert: %d %s", resp.Status, resp.Body)
	}
	s.oneLeads("operator removed", since)

	// 8. The mirror resumes at the cursor: the late message is copied once.
	since = s.dbNow()
	s.mirrorOnce()
	if items, msgs := s.copies(msgLate); items != 1 || msgs != 1 {
		t.Fatalf("the late message: %d items, %d messages, want one each", items, msgs)
	}
	s.oneLeads("mirror resumed", since)
	if m := w.fake.Methods(); !slices.Equal(m, []string{http.MethodGet}) {
		t.Fatalf("Discord got methods %v, want only GET", m)
	}
}

// mustJSON marshals v.
func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
