// SPDX-License-Identifier: Apache-2.0

package importtool

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/cmd/bothy/importtool/bothyapi/bothytest"
	"github.com/OWNER/bothy/services/server/cmd/bothy/importtool/discord/discordtest"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

// The threads of the basic fixture the sessions tests name: an active thread and an archived one
// of general, the active forum post of support, and an ID of no thread of the server.
const (
	fxArchived  = "13000311"
	fxForumPost = "13000302"
	fxNoThread  = "13999999"
)

// The invented session references of the tests; none may appear in any output or log line.
const (
	refGeneral = "sess-invented-general-7f3a"
	refForum   = "sess-invented-forum-91c2"
	refOther   = "sess-invented-archived-55d0"
	refOutside = "sess-invented-outside-0b1e"
)

// sessionsMapping is fixtureMapping with Lotse as the planned operator of general and the agent of
// Robin as the planned operator of the forum support.
var sessionsMapping = strings.Replace(strings.Replace(fixtureMapping,
	"target: {create: {name: \"general\"}}", "target: {create: {name: \"general\"}}\n    planned_operator: \"agent:lotse\"", 1),
	"target: {create: {name: \"support\"}}", "target: {create: {name: \"support\"}}\n    planned_operator: \"agent:robin-agent\"", 1)

// sessionsLines is a sessions file: a line taken over in general (with a baseline), one in the
// forum, one whose agent is not the planned operator of general (discarded) and one for a thread
// outside the server (ignored).
var sessionsLines = strings.Join([]string{
	`{"discord_thread": "` + fxThread + `", "agent": "lotse", "external_session_ref": "` + refGeneral + `", "cost_baseline_micros": 1500000, "currency": "USD"}`,
	`{"discord_thread": "` + fxForumPost + `", "agent": "robin-agent", "external_session_ref": "` + refForum + `"}`,
	``,
	`{"discord_thread": "` + fxArchived + `", "agent": "robin-agent", "external_session_ref": "` + refOther + `"}`,
	`{"discord_thread": "` + fxNoThread + `", "agent": "lotse", "external_session_ref": "` + refOutside + `"}`,
}, "\n") + "\n"

// writeSessions writes a sessions file into a fresh directory and returns its path.
func (e *testEnv) writeSessions(text string) string {
	e.t.Helper()
	p := filepath.Join(e.t.TempDir(), "sessions.jsonl")
	if err := os.WriteFile(p, []byte(text), 0o600); err != nil {
		e.t.Fatal(err)
	}
	return p
}

// newSessionsEnv is the env of the sessions tests: the basic fixture and the two agents.
func newSessionsEnv(t *testing.T) (*testEnv, map[string]string) {
	t.Helper()
	e := newTestEnv(t, discordtest.BasicFixture(t))
	agents := map[string]string{"lotse": e.bothy.AddAgent("lotse", "Lotse", true), "robin-agent": e.bothy.AddAgent("robin-agent", "Robin", true)}
	return e, agents
}

// totals decodes the total of a JSON report.
func totals(t *testing.T, out string) (map[string]any, []any) {
	t.Helper()
	var rep struct {
		Total map[string]any `json:"total"`
		Notes []any          `json:"notes"`
	}
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		t.Fatalf("report %q: %v", out, err)
	}
	return rep.Total, rep.Notes
}

// assertNoRefs fails when a reference of the file appears in stdout, stderr or the debug log of
// the tool (requirement 16, SEC-AUD-5).
func assertNoRefs(t *testing.T, e *testEnv) {
	t.Helper()
	for _, out := range []string{e.stdout.String(), e.stderr.String(), e.debug.String()} {
		for _, ref := range []string{refGeneral, refForum, refOther, refOutside} {
			if strings.Contains(out, ref) {
				t.Errorf("an external session reference appears in the output or log of the tool")
			}
		}
	}
}

// TestSessionsFileRules holds design note 1: one case per validation rule; each ends the run with
// exit 2 before any write, names the line and never the reference.
func TestSessionsFileRules(t *testing.T) {
	good := `{"discord_thread": "` + fxThread + `", "agent": "lotse", "external_session_ref": "` + refGeneral + `"}`
	for _, tc := range []struct {
		name, line, want string
	}{
		{"not an object", `[1]`, "line 2: not a JSON object"},
		{"broken JSON", `{"discord_thread": `, "line 2: not a JSON object"},
		{"unknown member", `{"discord_thread": "1", "agent": "lotse", "external_session_ref": "` + refOther + `", "model": "x"}`, `line 2: unknown member "model"`},
		{"thread not digits", `{"discord_thread": "x1", "agent": "lotse", "external_session_ref": "` + refOther + `"}`, "line 2: discord_thread"},
		{"thread missing", `{"agent": "lotse", "external_session_ref": "` + refOther + `"}`, "line 2: discord_thread"},
		{"thread a number", `{"discord_thread": 13000312, "agent": "lotse", "external_session_ref": "` + refOther + `"}`, "line 2: discord_thread"},
		{"agent missing", `{"discord_thread": "13000312", "external_session_ref": "` + refOther + `"}`, "line 2: agent"},
		{"reference empty", `{"discord_thread": "13000312", "agent": "lotse", "external_session_ref": " "}`, "line 2: external_session_ref"},
		{"reference with a control character", `{"discord_thread": "13000312", "agent": "lotse", "external_session_ref": "` + refOther + `\u0007"}`, "line 2: external_session_ref"},
		{"reference too long", `{"discord_thread": "13000312", "agent": "lotse", "external_session_ref": "` + refOther + strings.Repeat("x", sessionRefMaxBytes) + `"}`, "line 2: external_session_ref"},
		{"negative baseline", `{"discord_thread": "13000312", "agent": "lotse", "external_session_ref": "` + refOther + `", "cost_baseline_micros": -1, "currency": "USD"}`, "line 2: cost_baseline_micros"},
		{"fractional baseline", `{"discord_thread": "13000312", "agent": "lotse", "external_session_ref": "` + refOther + `", "cost_baseline_micros": 1.5, "currency": "USD"}`, "line 2: cost_baseline_micros"},
		{"baseline above 2^53", `{"discord_thread": "13000312", "agent": "lotse", "external_session_ref": "` + refOther + `", "cost_baseline_micros": 9007199254740992, "currency": "USD"}`, "line 2: cost_baseline_micros"},
		{"baseline without currency", `{"discord_thread": "13000312", "agent": "lotse", "external_session_ref": "` + refOther + `", "cost_baseline_micros": 5}`, "line 2: currency"},
		{"lower-case currency", `{"discord_thread": "13000312", "agent": "lotse", "external_session_ref": "` + refOther + `", "currency": "usd"}`, "line 2: currency"},
		{"duplicate thread", `{"discord_thread": "` + fxThread + `", "agent": "lotse", "external_session_ref": "` + refOther + `"}`, "line 2: discord_thread " + fxThread + " is already named in line 1"},
		{"unknown agent", `{"discord_thread": "13000312", "agent": "nobody", "external_session_ref": "` + refOther + `"}`, "line 2: agent nobody is no agent of the server"},
	} {
		t.Run(tc.name, func(t *testing.T) {
					e, _ := newSessionsEnv(t)
			e.startSource(sessionsMapping)
			base := e.bothy.Writes()
			file := e.writeSessions(good + "\n" + tc.line + "\n")
			if code := e.run("--guild", fxGuild, "--map", e.writeMap(sessionsMapping), "--apply", "--sessions", file); code != ExitUsage {
				t.Fatalf("exit %d, want 2: %s", code, e.stderr.String())
			}
			if !strings.Contains(e.stderr.String(), "sessions file "+tc.want) {
				t.Errorf("stderr %q does not name %q", e.stderr.String(), tc.want)
			}
			if w := e.bothy.Writes() - base; w != 0 {
				t.Errorf("writes = %d, want 0", w)
			}
			assertNoRefs(t, e)
		})
	}
}

// TestSessionsDryRun: a dry run with --sessions validates and counts (would take over, discarded,
// ignored) and writes nothing.
func TestSessionsDryRun(t *testing.T) {
	e, _ := newSessionsEnv(t)
	base := e.bothy.Writes()
	code := e.run("--guild", fxGuild, "--map", e.writeMap(sessionsMapping), "--sessions", e.writeSessions(sessionsLines), "--report", "json")
	if code != ExitOK {
		t.Fatalf("exit %d: %s", code, e.stderr.String())
	}
	total, notes := totals(t, e.stdout.String())
	if total["sessions_taken"] != float64(2) || total["sessions_discarded"] != float64(1) {
		t.Errorf("total %v, want 2 would take over and 1 discarded", total)
	}
	if !containsNote(notes, noteSessionsIgnored(1)) {
		t.Errorf("notes %v miss the ignored line", notes)
	}
	if w := e.bothy.Writes() - base; w != 0 {
		t.Errorf("a dry run wrote %d times", w)
	}
	assertNoRefs(t, e)
}

// TestSessionsApply: --apply sends earlier_session with the thread item of a line whose agent is
// the planned operator (with the agent UUID and the baseline), drops and counts the mismatch, and
// fills the column Sessions from the answers; a second run with the same file answers
// already_present and stores nothing new; a thread that has had a session since is late.
func TestSessionsApply(t *testing.T) {
	e, agents := newSessionsEnv(t)
	src := e.startSource(sessionsMapping)
	mapFile, file := e.writeMap(sessionsMapping), e.writeSessions(sessionsLines)
	if code := e.run("--guild", fxGuild, "--map", mapFile, "--apply", "--sessions", file, "--report", "json"); code != ExitOK {
		t.Fatalf("exit %d: %s", code, e.stderr.String())
	}
	total, notes := totals(t, e.stdout.String())
	if total["sessions_taken"] != float64(2) || total["sessions_discarded"] != float64(1) || total["sessions_present"] != float64(0) {
		t.Errorf("first run total %v", total)
	}
	if !containsNote(notes, noteSessionsIgnored(1)) {
		t.Errorf("notes %v miss the ignored line", notes)
	}
	want := map[string][]bothytest.EarlierSession{
		keyThread + fxThread:    {{AgentID: agents["lotse"], Ref: refGeneral, Baseline: 1500000, Currency: "USD"}},
		keyThread + fxForumPost: {{AgentID: agents["robin-agent"], Ref: refForum}},
		keyThread + fxArchived:  nil,
	}
	for key, w := range want {
		if got := e.bothy.EarlierSessions(src, key); !equalEarlier(got, w) {
			t.Errorf("%s: earlier sessions %+v, want %+v", key, got, w)
		}
	}
	if tool, _ := e.bothy.SourceStats(src)["tool"].(map[string]any); tool["sessions_taken"] != float64(2) || tool["sessions_discarded"] != float64(1) {
		t.Errorf("stats.tool %v, want the session counters", tool)
	}

	e.bothy.SetSessionLate(src, keyThread+fxForumPost)
	if code := e.run("--guild", fxGuild, "--map", mapFile, "--apply", "--sessions", file, "--report", "json"); code != ExitOK {
		t.Fatalf("second run exit %d: %s", code, e.stderr.String())
	}
	total, _ = totals(t, e.stdout.String())
	if total["sessions_taken"] != float64(0) || total["sessions_present"] != float64(1) || total["sessions_late"] != float64(1) ||
		total["sessions_discarded"] != float64(1) {
		t.Errorf("second run total %v", total)
	}
	if got := e.bothy.EarlierSessions(src, keyThread+fxThread); len(got) != 1 {
		t.Errorf("the second run stored %d earlier sessions of general, want 1", len(got))
	}
	if code := e.run("--guild", fxGuild, "--map", mapFile, "--apply", "--sessions", file); code != ExitOK ||
		!strings.Contains(e.stdout.String(), "Sessions taken/present/discarded/late") {
		t.Errorf("text report exit %d: %s", code, e.stdout.String())
	}
	assertNoRefs(t, e)
}

// TestSessionsOutsideSelection: with --only-thread a line of another thread is ignored and
// counted in the note, never sent.
func TestSessionsOutsideSelection(t *testing.T) {
	e, _ := newSessionsEnv(t)
	src := e.startSource(sessionsMapping)
	code := e.run("--guild", fxGuild, "--map", e.writeMap(sessionsMapping), "--apply", "--only-thread", fxThread,
		"--sessions", e.writeSessions(sessionsLines), "--report", "json")
	if code != ExitOK {
		t.Fatalf("exit %d: %s", code, e.stderr.String())
	}
	total, notes := totals(t, e.stdout.String())
	if total["sessions_taken"] != float64(1) || !containsNote(notes, noteSessionsIgnored(3)) {
		t.Errorf("total %v notes %v", total, notes)
	}
	if got := e.bothy.EarlierSessions(src, keyThread+fxForumPost); len(got) != 0 {
		t.Errorf("a thread outside --only-thread got %+v", got)
	}
}

// TestSessionRefMaxBytesIsTheKeyMaximum holds sessionRefMaxBytes equal to the maximum of
// agent.session_ref_max_bytes in the configuration schema of the server.
func TestSessionRefMaxBytesIsTheKeyMaximum(t *testing.T) {
	t.Parallel()
	s, err := wire.ConfigSchema()
	if err != nil {
		t.Fatal(err)
	}
	for _, k := range s.Keys() {
		if k.Name == "agent.session_ref_max_bytes" {
			if k.Max != strconv.Itoa(sessionRefMaxBytes) {
				t.Fatalf("sessionRefMaxBytes is %d, the key maximum %s", sessionRefMaxBytes, k.Max)
			}
			return
		}
	}
	t.Fatal("agent.session_ref_max_bytes is not a key of the server")
}

// containsNote reports whether notes holds n.
func containsNote(notes []any, n string) bool {
	for _, x := range notes {
		if x == n {
			return true
		}
	}
	return false
}

// equalEarlier compares two lists of earlier sessions.
func equalEarlier(a, b []bothytest.EarlierSession) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
