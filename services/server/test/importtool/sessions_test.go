// SPDX-License-Identifier: Apache-2.0

package importtool

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/OWNER/bothy/services/server/cmd/bothy/importtool"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
	"github.com/OWNER/bothy/services/server/test/harness"
)

// The invented session references of these tests; none may appear in a log line of the tool.
const (
	refRelease = "sess-invented-release-4c1d"
	refFresh   = "sess-invented-fresh-8b2e"
	// releaseBaseline is the cost baseline of the line of thRelease in micros (USD).
	releaseBaseline = 1500000
)

// sessionsMapping is mapping() with Lotse as the planned operator of general and its lead
// `bothy`, so the imported thread thRelease has Lotse as operator and Alex may write there after
// the import (a channel with lead `discord` is a read-only mirror, R-IM-07).
func sessionsMapping() map[string]any {
	m := mapping()
	for _, c := range m["channels"].([]any) {
		e := c.(map[string]any)
		if e["discord_channel"] == chGeneral {
			e["planned_operator"], e["lead"] = "agent:lotse", "bothy"
		}
	}
	return m
}

// sessionsFile writes the sessions file with the line of thRelease and returns its path.
func (w *world) sessionsFile() string {
	w.t.Helper()
	line := `{"discord_thread": "` + thRelease + `", "agent": "lotse", "external_session_ref": "` + refRelease +
		`", "cost_baseline_micros": ` + strconv.Itoa(releaseBaseline) + `, "currency": "USD"}` + "\n"
	path := filepath.Join(w.t.TempDir(), "sessions.jsonl")
	if err := os.WriteFile(path, []byte(line), 0o600); err != nil {
		w.t.Fatal(err)
	}
	return path
}

// sessionsWorld imports the fixture with the sessions file and gives Alex the trigger level and
// Lotse the write level on the imported general channel (I1 and I2, 09 3). It returns the world,
// the source, the session file of the tool, the sessions file and the imported thread thRelease.
func sessionsWorld(t *testing.T) (*world, string, string, string, string) {
	t.Helper()
	cfg := harness.RaisedRateLimits()
	cfg["telemetry.logs.level"] = "debug"
	logs := &syncBuffer{}
	w := newWorldWith(t, harness.Options{EgressDenyAll: true, Config: cfg, LogWriter: logs})
	// Requirement 16: the server never logs an external session reference, at any level.
	t.Cleanup(func() {
		if strings.Contains(logs.String(), refRelease) || strings.Contains(logs.String(), refFresh) {
			t.Error("an external session reference appears in the server log")
		}
	})
	src := w.sourceWith(sessionsMapping())
	session, file := w.login(), w.sessionsFile()
	res := w.mustApply(run{session: session, mapFile: w.mapFile(sessionsMapping()), extra: []string{"--sessions", file, "--report", "json"}},
		importtool.ExitOK)
	if strings.Contains(res.stdout+res.stderr, refRelease) {
		t.Error("the reference appears in the output or the debug log of the tool")
	}
	if got := reportTotal(t, res.stdout)["sessions_taken"]; got != float64(1) {
		t.Errorf("sessions_taken = %v, want 1", got)
	}
	general := w.target(src, "channel:"+chGeneral)
	for _, g := range []map[string]any{
		{"subject_kind": "user", "subject_id": w.alex.PrincipalID, "level": "trigger"},
		{"subject_kind": "agent", "subject_id": w.lotse.PrincipalID, "level": "write"},
	} {
		g["target_kind"], g["target_id"], g["reason"] = "channel", general, "imported channel"
		w.must(w.s.Owner, http.MethodPost, "/grants", g, nil, http.StatusCreated)
	}
	// The first poll creates the default consumer of Lotse at the head of the log, so it delivers
	// the invocations of the triggers that follow.
	w.must(w.lotse, http.MethodGet, "/agents/me/consumers/default/events", nil, nil, http.StatusOK)
	return w, src, session, file, w.target(src, "thread:"+thRelease)
}

// reportTotal decodes the member total of a JSON report.
func reportTotal(t *testing.T, out string) map[string]any {
	t.Helper()
	var rep struct {
		Total map[string]any `json:"total"`
	}
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		t.Fatalf("report %q: %v", out, err)
	}
	return rep.Total
}

// sessRow is one row of agent_sessions as these tests read it.
type sessRow struct {
	ID, Ref, EndReason, Currency string
	Current, Started             bool
	Baseline, Version            int64
}

// sessions reads the rows of agent_sessions of thread, oldest first.
func (w *world) sessions(thread string) []sessRow {
	w.t.Helper()
	rows, err := w.s.AdminDB().Query(w.ctx, `SELECT id::text, external_session_ref, coalesce(end_reason, ''), currency, is_current, started,
		cost_baseline_micros, version FROM agent_sessions WHERE thread_id = $1 ORDER BY created_at, id`, thread)
	if err != nil {
		w.t.Fatal(err)
	}
	defer rows.Close()
	var out []sessRow
	for rows.Next() {
		var r sessRow
		if err := rows.Scan(&r.ID, &r.Ref, &r.EndReason, &r.Currency, &r.Current, &r.Started, &r.Baseline, &r.Version); err != nil {
			w.t.Fatal(err)
		}
		out = append(out, r)
	}
	return out
}

// current returns the current row of thread.
func (w *world) current(thread string) (sessRow, bool) {
	for _, r := range w.sessions(thread) {
		if r.Current {
			return r, true
		}
	}
	return sessRow{}, false
}

// post lets Alex write text into thread and returns the message ID.
func (w *world) post(thread, text string) string {
	w.t.Helper()
	var m struct {
		ID string `json:"id"`
	}
	w.must(w.alex, http.MethodPost, "/threads/"+thread+"/messages", map[string]any{"client_msg_id": ids.New().String(), "content_md": text},
		nil, http.StatusCreated).JSON(w.t, &m)
	return m.ID
}

// runs returns the IDs of the runs of thread, oldest first.
func (w *world) runs(thread string) []string {
	w.t.Helper()
	rows, err := w.s.AdminDB().Query(w.ctx, `SELECT id::text FROM runs WHERE thread_id = $1 ORDER BY queued_at, id`, thread)
	if err != nil {
		w.t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			w.t.Fatal(err)
		}
		out = append(out, id)
	}
	return out
}

// payloads returns the payloads of the events of type typ of thread, in log order.
func (w *world) payloads(typ, thread string) []map[string]any {
	w.t.Helper()
	rows, err := w.s.AdminDB().Query(w.ctx, `SELECT payload::text FROM event_log WHERE type = $1 AND thread_id = $2 ORDER BY seq`, typ, thread)
	if err != nil {
		w.t.Fatal(err)
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			w.t.Fatal(err)
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(raw), &m); err != nil {
			w.t.Fatal(err)
		}
		out = append(out, m)
	}
	return out
}

// invocation returns the payload of agent.invocation of run.
func (w *world) invocation(thread, run string) map[string]any {
	w.t.Helper()
	for _, p := range w.payloads("agent.invocation", thread) {
		if p["run_id"] == run {
			return p
		}
	}
	w.t.Fatalf("no agent.invocation of run %s", run)
	return nil
}

// bindAudits returns the details of the allow entries of agent_session.bind on thread and their
// actors.
func (w *world) bindAudits(thread string) ([]map[string]any, []string) {
	w.t.Helper()
	rows, err := w.s.AdminDB().Query(w.ctx, `SELECT coalesce(actor_principal_id::text, ''), details::text FROM audit_events
		WHERE action = 'agent_session.bind' AND decision = 'allow' AND resource_id = $1 ORDER BY seq`, thread)
	if err != nil {
		w.t.Fatal(err)
	}
	defer rows.Close()
	var details []map[string]any
	var actors []string
	for rows.Next() {
		var actor, raw string
		if err := rows.Scan(&actor, &raw); err != nil {
			w.t.Fatal(err)
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(raw), &m); err != nil {
			w.t.Fatal(err)
		}
		details, actors = append(details, m), append(actors, actor)
	}
	return details, actors
}

// invocationToken polls the default consumer of Lotse until it delivers agent.invocation of run
// and returns its token; every page is acknowledged.
func (w *world) invocationToken(run string) string {
	w.t.Helper()
	token := ""
	testkit.Until(w.t, "the consumer of Lotse delivers agent.invocation of the run", func() bool {
		var page struct {
			Events []struct {
				Type     string `json:"type"`
				Token    string `json:"invocation_token"`
				Resource struct {
					ID string `json:"id"`
				} `json:"resource"`
			} `json:"events"`
			NextCursor int64 `json:"next_cursor"`
		}
		w.must(w.lotse, http.MethodGet, "/agents/me/consumers/default/events", nil, nil, http.StatusOK).JSON(w.t, &page)
		for _, e := range page.Events {
			if e.Type == "agent.invocation" && e.Resource.ID == run && e.Token != "" {
				token = e.Token
			}
		}
		if len(page.Events) > 0 {
			w.must(w.lotse, http.MethodPost, "/agents/me/consumers/default/ack", map[string]any{"seq": page.NextCursor}, nil, http.StatusOK)
		}
		return token != ""
	})
	return token
}

// start starts run as Lotse with the token of its invocation.
func (w *world) start(run string) {
	w.t.Helper()
	w.must(w.lotse, http.MethodPost, "/runs/"+run+"/start", map[string]any{"invocation_token": w.invocationToken(run),
		"billing_mode_used": "organization", "tool_class_used": "chat"}, nil, http.StatusOK)
}

// finish ends run as Lotse with completed.
func (w *world) finish(run string) {
	w.t.Helper()
	w.must(w.lotse, http.MethodPost, "/runs/"+run+"/finish", map[string]any{"state": "completed"}, nil, http.StatusOK)
}

// checkBound holds the import binding of thread after the first trigger (22 9 steps 2 and 3, 09
// 9.5): exactly one current row with the imported reference, started and with the baseline,
// beside the import row; one allow audit agent_session.bind with cause import by Alex; one
// agent_session.changed; the invocation of the first run with session_mode bind and the bound row
// carrying the baseline and its currency. It returns the first run and the bound row.
func (w *world) checkBound(thread string) (string, sessRow) {
	w.t.Helper()
	rows := w.sessions(thread)
	if len(rows) != 2 || rows[0].EndReason != "import" || rows[0].Current {
		w.t.Fatalf("rows %+v, want the import row and the bound row", rows)
	}
	bound := rows[1]
	if !bound.Current || bound.Ref != refRelease || !bound.Started || bound.Baseline != releaseBaseline || bound.Currency != "USD" {
		w.t.Fatalf("bound row %+v", bound)
	}
	details, actors := w.bindAudits(thread)
	if len(details) != 1 || details[0]["cause"] != "import" || details[0]["session_id"] != bound.ID || actors[0] != w.alex.PrincipalID {
		w.t.Fatalf("audit entries %v by %v", details, actors)
	}
	if strings.Contains(w.text(`SELECT coalesce(string_agg(details::text, ''), '') FROM audit_events WHERE resource_id = $1`, thread), refRelease) {
		w.t.Error("an audit entry carries the external session reference")
	}
	if ev := w.payloads("agent_session.changed", thread); len(ev) != 1 || ev[0]["agent_session"].(map[string]any)["id"] != bound.ID {
		w.t.Fatalf("agent_session.changed %v", ev)
	}
	runs := w.runs(thread)
	if len(runs) == 0 {
		w.t.Fatal("the trigger opened no run")
	}
	p := w.invocation(thread, runs[0])
	s, _ := p["agent_session"].(map[string]any)
	if p["session_mode"] != "bind" || s == nil || s["id"] != bound.ID || s["external_session_ref"] != refRelease ||
		s["cost_baseline_micros"] != float64(releaseBaseline) || s["currency"] != "USD" {
		w.t.Fatalf("invocation %v", p)
	}
	return runs[0], bound
}

// TestImportedSessionBindsOnce (plan acceptance, 22 16 row "Binding once"): the import writes the
// earlier row; two simultaneous first triggers by Alex in the imported thread give exactly one
// current row with the imported reference, one audit agent_session.bind with cause import and one
// agent_session.changed, and the invocation carries session_mode bind and the baseline.
func TestImportedSessionBindsOnce(t *testing.T) {
	t.Parallel()
	w, _, _, _, thread := sessionsWorld(t)
	rows := w.sessions(thread)
	if len(rows) != 1 || rows[0].Current || !rows[0].Started || rows[0].EndReason != "import" || rows[0].Ref != refRelease ||
		rows[0].Baseline != releaseBaseline || rows[0].Currency != "USD" {
		t.Fatalf("earlier rows %+v", rows)
	}
	var wg sync.WaitGroup
	gate := make(chan struct{})
	for i := range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-gate
			w.post(thread, "Please continue with the release plan, part "+strconv.Itoa(i+1))
		}()
	}
	close(gate)
	wg.Wait()
	w.checkBound(thread)
	if n := w.count(`SELECT count(*) FROM agent_sessions WHERE thread_id = $1 AND is_current`, thread); n != 1 {
		t.Fatalf("%d current rows", n)
	}
}

// TestImportedSessionMissingFailsOpen (plan acceptance, 22 9 step 5, 09 9.5 item 4): Lotse reports
// bind_failed session_missing for the bound row; a new current row follows, exactly one system
// line agent_session.resume_failed is written, and Alex's message reached the agent (the
// invocation names it).
func TestImportedSessionMissingFailsOpen(t *testing.T) {
	t.Parallel()
	w, _, _, _, thread := sessionsWorld(t)
	msg := w.post(thread, "Please continue with the release plan")
	first, bound := w.checkBound(thread)
	if p := w.invocation(thread, first); p["message_id"] != msg {
		t.Fatalf("the invocation names message %v, want Alex's %s", p["message_id"], msg)
	}
	w.start(first)
	h := http.Header{}
	h.Set("If-Match", `"`+strconv.FormatInt(w.sessionVersion(bound.ID), 10)+`"`)
	w.must(w.lotse, http.MethodPost, "/threads/"+thread+"/agent-sessions/report",
		map[string]any{"external_session_ref": refFresh, "started": true, "bind_failed": "session_missing"}, h, http.StatusOK)
	cur, ok := w.current(thread)
	if !ok || cur.ID == bound.ID || cur.Ref != refFresh {
		t.Fatalf("current row %+v, want a new row", cur)
	}
	for _, r := range w.sessions(thread) {
		if r.ID == bound.ID && (r.Current || r.EndReason != "rebind") {
			t.Fatalf("the bound row %+v did not end with rebind", r)
		}
	}
	if n := w.count(`SELECT count(*) FROM messages WHERE thread_id = $1 AND kind = 'system' AND system_code = 'agent_session.resume_failed'`,
		thread); n != 1 {
		t.Fatalf("%d system lines agent_session.resume_failed, want 1", n)
	}
	if n := w.count(`SELECT count(*) FROM messages WHERE id = $1 AND author_principal_id = $2`, msg, w.alex.PrincipalID); n != 1 {
		t.Fatal("Alex's message is lost")
	}
	w.finish(first)
}

// sessionVersion reads the version of a session row.
func (w *world) sessionVersion(id string) int64 {
	w.t.Helper()
	return int64(w.count(`SELECT version FROM agent_sessions WHERE id = $1`, id))
}

// TestResetNeverBindsAgain (plan acceptance, 22 9 step 6): after resetAgentSession the next
// trigger starts a new session; a second import run with the same file reports the line late and
// writes nothing.
func TestResetNeverBindsAgain(t *testing.T) {
	t.Parallel()
	w, _, session, file, thread := sessionsWorld(t)
	w.post(thread, "Please continue with the release plan")
	first, _ := w.checkBound(thread)
	w.start(first)
	w.finish(first)
	w.must(w.alex, http.MethodPost, "/threads/"+thread+"/agent-sessions/reset", nil, nil, http.StatusCreated)
	if _, ok := w.current(thread); ok {
		t.Fatal("a row is current after the reset")
	}
	w.post(thread, "Start over, please")
	runs := w.runs(thread)
	if p := w.invocation(thread, runs[len(runs)-1]); p["session_mode"] != "new" || p["agent_session"] != nil {
		t.Fatalf("after the reset %v", p)
	}
	before := w.sessions(thread)
	res := w.mustApply(run{session: session, mapFile: w.mapFile(sessionsMapping()), extra: []string{"--sessions", file, "--report", "json"}},
		importtool.ExitOK)
	if total := reportTotal(t, res.stdout); total["sessions_late"] != float64(1) || total["sessions_taken"] != float64(0) {
		t.Errorf("second run total %v, want one late line", total)
	}
	if after := w.sessions(thread); len(after) != len(before) {
		t.Fatalf("the second run wrote rows: %+v, were %+v", after, before)
	}
	if details, _ := w.bindAudits(thread); len(details) != 1 {
		t.Fatalf("%d bind audits, want 1", len(details))
	}
}

// TestSecondRunAlreadyPresent (22 9 step 1, requirement 13): a second import run with the same file
// before any trigger answers already present and adds no row; the stats of the server count it.
func TestSecondRunAlreadyPresent(t *testing.T) {
	t.Parallel()
	w, src, session, file, thread := sessionsWorld(t)
	res := w.mustApply(run{session: session, mapFile: w.mapFile(sessionsMapping()), extra: []string{"--sessions", file, "--report", "json"}},
		importtool.ExitOK)
	if total := reportTotal(t, res.stdout); total["sessions_present"] != float64(1) || total["sessions_taken"] != float64(0) {
		t.Errorf("second run total %v, want one present line", total)
	}
	if rows := w.sessions(thread); len(rows) != 1 {
		t.Fatalf("%d rows after the second run, want 1", len(rows))
	}
	stats := w.text(`SELECT stats::text FROM import_sources WHERE id = $1`, src)
	for _, want := range []string{`"sessions_taken_over": 1`, `"sessions_already_present": 1`} {
		if !strings.Contains(stats, want) {
			t.Errorf("stats %s miss %s", stats, want)
		}
	}
}
