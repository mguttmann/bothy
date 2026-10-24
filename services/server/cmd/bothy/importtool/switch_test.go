// SPDX-License-Identifier: Apache-2.0

package importtool

import (
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/cmd/bothy/importtool/discord/discordtest"
)

// switchMapping is fixtureMapping with the planned operator Lotse for general, the channel the
// switch tests move.
var switchMapping = strings.Replace(fixtureMapping, `    target: {create: {name: "general"}}`,
	`    target: {create: {name: "general"}}
    planned_operator: "agent:lotse"`, 1)

// busyReaction is the reactions member of a message an agent still works on (a default of
// import.follow.busy_reactions).
var busyReaction = map[string]any{"reactions": []any{map[string]any{"count": 1, "emoji": map[string]any{"name": "\U0001F440"}}}}

// switchEnv starts the fakes with the agent Lotse, stores the source of switchMapping with the
// stored mapping stored (switchMapping when ""), imports the fixture with --apply and returns the
// env, the source and the mapping file.
func switchEnv(t *testing.T, stored string) (*testEnv, string, string) {
	t.Helper()
	e := newTestEnv(t, discordtest.BasicFixture(t))
	e.bothy.AddAgent("lotse", "Lotse", true)
	if stored == "" {
		stored = switchMapping
	}
	src := e.startSource(stored)
	mapFile := e.writeMap(switchMapping)
	if code := e.run("--guild", fxGuild, "--map", mapFile, "--apply"); code != ExitOK {
		t.Fatalf("import exit %d: %s", code, e.stderr.String())
	}
	return e, src, mapFile
}

// switchArgs are the arguments of switch for general.
func switchArgs(mapFile string, extra ...string) []string {
	return append([]string{"switch", "--guild", fxGuild, "--map", mapFile, "--channel", fxGeneral}, extra...)
}

// TestSwitchNeedsDiscordLocked (brief design note 1): without --discord-locked switch ends with
// exit 2 and the owner's step 1, before any request to either fake.
func TestSwitchNeedsDiscordLocked(t *testing.T) {
	e := newTestEnv(t, discordtest.BasicFixture(t))
	e.startSource(switchMapping)
	mapFile := e.writeMap(switchMapping)
	discordBase, bothyBase := len(e.discord.Requests()), len(e.bothy.Requests())
	if code := e.run(switchArgs(mapFile)...); code != ExitUsage {
		t.Fatalf("exit %d, want 2: %s", code, e.stderr.String())
	}
	if !strings.Contains(e.stderr.String(), "--discord-locked") || !strings.Contains(e.stderr.String(), "read-only in Discord") {
		t.Errorf("stderr does not name step 1 and the flag:\n%s", e.stderr.String())
	}
	if len(e.discord.Requests()) != discordBase || len(e.bothy.Requests()) != bothyBase {
		t.Error("a request reached a fake")
	}
}

// TestSwitchHeldBackExit1 (brief design note 2, 22 13.3): a busy message in the main thread and an
// unsettled one in a thread hold the switch-over: exit 1 with the counts per reason and no step 3.
// Once the reaction is gone and import.follow.settle_after passed, switch copies both once and
// prints step 3 with the planned operator and the probe of step 4.
func TestSwitchHeldBackExit1(t *testing.T) {
	e, src, mapFile := switchEnv(t, "")
	const busyID, unsettledID = "13009001", "13009002"
	now := e.clk.Now()
	e.discord.AddMessage(fxGeneral, newMessage(busyID, now.Add(-time.Hour)))
	e.discord.EditMessage(fxGeneral, busyID, busyReaction)
	e.discord.AddMessage(fxThread, newMessage(unsettledID, now))
	if code := e.run(switchArgs(mapFile, "--discord-locked")...); code != ExitAborted {
		t.Fatalf("exit %d, want 1: %s", code, e.stderr.String())
	}
	for _, want := range []string{"2 messages are still held back", "busy 1", "unsettled 1", "import.follow.settle_after"} {
		if !strings.Contains(e.stderr.String(), want) {
			t.Errorf("stderr lacks %q:\n%s", want, e.stderr.String())
		}
	}
	if strings.Contains(e.stderr.String(), "Step 3") {
		t.Errorf("step 3 printed while messages are held:\n%s", e.stderr.String())
	}
	if _, held := e.bothy.SourceCursor(src)["run"]; held {
		t.Error("the lease is still held after switch")
	}

	e.discord.EditMessage(fxGeneral, busyID, map[string]any{"reactions": nil})
	e.clk.Advance(DefaultConfig().FollowSettleAfter)
	if code := e.run(switchArgs(mapFile, "--discord-locked")...); code != ExitOK {
		t.Fatalf("exit %d, want 0: %s", code, e.stderr.String())
	}
	if n := count(e.bothy.ThreadMessages(src, "channel:"+fxGeneral), "message:"+busyID); n != 1 {
		t.Errorf("the busy message was copied %d times, want 1", n)
	}
	if n := count(e.bothy.ThreadMessages(src, "thread:"+fxThread), "message:"+unsettledID); n != 1 {
		t.Errorf("the unsettled message was copied %d times, want 1", n)
	}
	for _, want := range []string{"Step 3", `choose "Lead here"`, "agent:lotse", "Step 4"} {
		if !strings.Contains(e.stderr.String(), want) {
			t.Errorf("stderr lacks %q:\n%s", want, e.stderr.String())
		}
	}
}

// TestSwitchNewMessagesExit1 (brief design note 2): messages after the cursor that the settle rule
// does not hold but the last round did not copy (here a thread whose batch the server refused once)
// end switch with exit 1 and the hint that the Discord lock of step 1 is evidently not in place.
func TestSwitchNewMessagesExit1(t *testing.T) {
	e, _, mapFile := switchEnv(t, "")
	e.discord.AddMessage(fxThread, newMessage("13009001", e.clk.Now().Add(-time.Hour)))
	e.bothy.FailBatch("thread:"+fxThread, http.StatusConflict, "item_conflict", 1)
	if code := e.run(switchArgs(mapFile, "--discord-locked")...); code != ExitAborted {
		t.Fatalf("exit %d, want 1: %s", code, e.stderr.String())
	}
	if !strings.Contains(e.stderr.String(), "run switch again") || !strings.Contains(e.stderr.String(), "not copied") {
		t.Errorf("stderr lacks the hint:\n%s", e.stderr.String())
	}
	if strings.Contains(e.stderr.String(), "Step 3") {
		t.Errorf("step 3 printed:\n%s", e.stderr.String())
	}
}

// TestSwitchWrongLeadExit2 (brief design note 1): a channel whose stored lead is already bothy, a
// channel not imported yet and a source that is not running end with exit 2 naming the condition,
// without any write.
func TestSwitchWrongLeadExit2(t *testing.T) {
	t.Run("lead bothy", func(t *testing.T) {
		e, _, mapFile := switchEnv(t, strings.Replace(switchMapping, `    planned_operator: "agent:lotse"`,
			`    planned_operator: "agent:lotse"
    lead: bothy`, 1))
		base := e.bothy.Writes()
		if code := e.run(switchArgs(mapFile, "--discord-locked")...); code != ExitUsage {
			t.Fatalf("exit %d, want 2: %s", code, e.stderr.String())
		}
		if !strings.Contains(e.stderr.String(), "lead") {
			t.Errorf("stderr does not name the lead:\n%s", e.stderr.String())
		}
		if w := e.bothy.Writes() - base; w != 0 {
			t.Errorf("writes = %d, want 0", w)
		}
	})
	t.Run("not imported", func(t *testing.T) {
		e := newTestEnv(t, discordtest.BasicFixture(t))
		e.bothy.AddAgent("lotse", "Lotse", true)
		e.startSource(switchMapping)
		base := e.bothy.Writes()
		if code := e.run(switchArgs(e.writeMap(switchMapping), "--discord-locked")...); code != ExitUsage {
			t.Fatalf("exit %d, want 2: %s", code, e.stderr.String())
		}
		if !strings.Contains(e.stderr.String(), "not imported") {
			t.Errorf("stderr does not name the missing import:\n%s", e.stderr.String())
		}
		if w := e.bothy.Writes() - base; w != 0 {
			t.Errorf("writes = %d, want 0", w)
		}
	})
	t.Run("paused source", func(t *testing.T) {
		e, src, mapFile := switchEnv(t, "")
		e.bothy.SetState(src, "paused")
		base := e.bothy.Writes()
		if code := e.run(switchArgs(mapFile, "--discord-locked")...); code != ExitUsage {
			t.Fatalf("exit %d, want 2: %s", code, e.stderr.String())
		}
		if w := e.bothy.Writes() - base; w != 0 {
			t.Errorf("writes = %d, want 0", w)
		}
	})
}

// TestRevertOnlyReads (brief design note 3, 22 13.4, R-IM-12): revert sends only GET to the server
// and nothing to Discord; it counts per server thread the messages written on the server (origin other
// than import, kind other than system), prints one line per thread with a count, the total and
// the owner's steps, and never a name.
func TestRevertOnlyReads(t *testing.T) {
	e, src, _ := switchEnv(t, "")
	stored := strings.Replace(switchMapping, `    planned_operator: "agent:lotse"`, `    planned_operator: "agent:lotse"
    lead: bothy`, 1)
	m, err := ParseMapping([]byte(stored), fxGuild)
	if err != nil {
		t.Fatal(err)
	}
	e.bothy.SetConfig(src, m.Normalized())
	var channel string
	for _, it := range e.bothy.Items(src) {
		if it.OriginKey == "channel:"+fxGeneral {
			channel = it.TargetID
		}
	}
	main := e.bothy.MainThread(channel)
	for _, mk := range [][2]string{{"human", "text"}, {"agent", "text"}, {"system", "system"}, {"import", "text"}} {
		e.bothy.AddThreadMessage(main, mk[0], mk[1])
	}
	written := e.bothy.AddThread(channel)
	e.bothy.AddThreadMessage(written, "human", "text")
	quiet := e.bothy.AddThread(channel)
	e.bothy.AddThreadMessage(quiet, "import", "text")
	discordBase, bothyBase := len(e.discord.Requests()), len(e.bothy.Requests())

	if code := e.run("revert", "--guild", fxGuild, "--channel", fxGeneral); code != ExitOK {
		t.Fatalf("exit %d: %s", code, e.stderr.String())
	}
	if len(e.discord.Requests()) != discordBase {
		t.Error("revert sent a request to Discord")
	}
	for _, r := range e.bothy.Requests()[bothyBase:] {
		if r.Method != http.MethodGet {
			t.Errorf("revert sent %s %s", r.Method, r.Path)
		}
	}
	out := e.stdout.String()
	for _, want := range []string{main + ": 2", written + ": 1", "Total: 3", "Discord leads", "add the agent"} {
		if !strings.Contains(out, want) {
			t.Errorf("revert output lacks %q:\n%s", want, out)
		}
	}
	for _, unwanted := range []string{quiet, "general", "Alex", "Lotse"} {
		if strings.Contains(out, unwanted) {
			t.Errorf("revert output contains %q:\n%s", unwanted, out)
		}
	}

	// A channel whose stored lead is discord already: the same output, with the console step done.
	e.bothy.SetConfig(src, mustNormalized(t, switchMapping))
	if code := e.run("revert", "--guild", fxGuild, "--channel", fxGeneral); code != ExitOK {
		t.Fatalf("exit %d: %s", code, e.stderr.String())
	}
	if out := e.stdout.String(); !strings.Contains(out, "Total: 3") || !strings.Contains(out, "already discord") {
		t.Errorf("revert of a channel led by Discord:\n%s", out)
	}
}

// mustNormalized returns the stored form of a mapping text.
func mustNormalized(t *testing.T, text string) map[string]any {
	t.Helper()
	m, err := ParseMapping([]byte(text), fxGuild)
	if err != nil {
		t.Fatal(err)
	}
	return m.Normalized()
}

// TestStatusFlags (brief design note 4, 22 12.1): status prints per mapped channel the lead, the
// read-only state of the server channel and how its operator compares with the planned operator,
// flags a lead that disagrees with the read-only flag as inconsistent, and names nothing.
func TestStatusFlags(t *testing.T) {
	stored := strings.Replace(switchMapping, `    planned_operator: "agent:lotse"`, `    planned_operator: "agent:lotse"
    lead: bothy`, 1)
	stored = strings.Replace(stored, `    target: {create: {name: "news"}}`, `    target: {create: {name: "news"}}
    planned_operator: "agent:lotse"
    lead: bothy`, 1)
	// lounge is mapped in the stored mapping only, so it was never imported.
	stored = strings.Replace(stored, "users:", `  - discord_channel: "13000106"
    target: {create: {name: "lounge"}}
users:`, 1)
	e, src, _ := switchEnv(t, "")
	e.bothy.SetConfig(src, mustNormalized(t, stored))
	targets := map[string]string{}
	for _, it := range e.bothy.Items(src) {
		targets[it.OriginKey] = it.TargetID
	}
	pilot := e.bothy.AddAgent("pilot", "Pilot", true)
	for _, a := range []struct {
		channel, agent string
		readOnly       bool
	}{
		{fxGeneral, "", false}, // lead bothy, writable, operator Lotse (set below)
		{fxNews, pilot, true},  // lead bothy but read-only: inconsistent; another agent
		{fxRandom, "", true},   // lead discord, read-only, no operator
		{fxSupport, "", false}, // lead discord but writable: inconsistent
	} {
		e.bothy.SetChannelReadOnly(targets["channel:"+a.channel], a.readOnly)
		e.bothy.SetChannelOperator(targets["channel:"+a.channel], a.agent)
	}
	e.bothy.SetChannelOperator(targets["channel:"+fxGeneral], agentID(t, e, "lotse"))

	if code := e.run("status", "--guild", fxGuild); code != ExitOK {
		t.Fatalf("status exit %d: %s", code, e.stderr.String())
	}
	out := e.stdout.String()
	lines := strings.Split(out, "\n")
	want := map[string][]string{
		fxGeneral: {"bothy", "writable", "operator ok"},
		fxNews:    {"bothy", "read-only", "operator differs from planned", "inconsistent"},
		fxRandom:  {"discord", "read-only", "no operator"},
		fxSupport: {"discord", "writable", "no operator", "inconsistent"},
		fxLounge:  {"discord", "not imported"},
	}
	for id, parts := range want {
		i := slices.IndexFunc(lines, func(l string) bool { return strings.HasPrefix(strings.TrimSpace(l), id+": ") })
		if i < 0 {
			t.Errorf("no line of %s:\n%s", id, out)
			continue
		}
		for _, p := range parts {
			if !strings.Contains(lines[i], p) {
				t.Errorf("line of %s lacks %q: %s", id, p, lines[i])
			}
		}
		if id != fxNews && id != fxSupport && strings.Contains(lines[i], "inconsistent") {
			t.Errorf("line of %s flagged: %s", id, lines[i])
		}
		if id != fxLounge && !strings.Contains(lines[i], targets["channel:"+id]) {
			t.Errorf("line of %s lacks the server channel: %s", id, lines[i])
		}
	}
	for _, name := range []string{"general", "news", "Projects", "Lotse", "Pilot", "Alex"} {
		if strings.Contains(out, name) {
			t.Errorf("status names %q:\n%s", name, out)
		}
	}
}

// agentID returns the UUID of the agent handle of the fake.
func agentID(t *testing.T, e *testEnv, handle string) string {
	t.Helper()
	api, err := newBothy(t.Context(), DefaultConfig(), e.session, e.env())
	if err != nil {
		t.Fatal(err)
	}
	agents, err := bothyReads{api: api}.agents(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	return agents[handle].id.String()
}

// TestSwitchUsageExit2 (22 10.2 rule 3, 10.3): a missing or malformed argument of switch ends with
// exit 2 before any request to either fake.
func TestSwitchUsageExit2(t *testing.T) {
	e := newTestEnv(t, discordtest.BasicFixture(t))
	mapFile := e.writeMap(switchMapping)
	for _, args := range [][]string{
		{"switch", "--guild", fxGuild, "--channel", fxGeneral, "--discord-locked"},
		{"switch", "--guild", fxGuild, "--map", mapFile, "--channel", "general", "--discord-locked"},
		{"switch", "--guild", fxGuild, "--map", mapFile, "--channel", fxGeneral, "--discord-locked", "--report", "xml"},
		{"switch", "--guild", fxGuild, "--map", mapFile, "--channel", fxGeneral, "--discord-locked", "--unknown"},
		{"revert", "--guild", fxGuild},
		{"revert", "--guild", fxGuild, "--channel", fxGeneral, "extra"},
	} {
		discordBase, bothyBase := len(e.discord.Requests()), len(e.bothy.Requests())
		if code := e.run(args...); code != ExitUsage {
			t.Errorf("%v: exit %d, want 2", args, code)
		}
		if len(e.discord.Requests()) != discordBase || len(e.bothy.Requests()) != bothyBase {
			t.Errorf("%v: a request reached a fake", args)
		}
	}
}

// TestRevertRefusesExit2 (brief design note 3): revert ends with exit 2 for a guild without an open
// source, a channel the stored mapping does not name and a channel that is not imported; status of
// a guild without a source says so.
func TestRevertRefusesExit2(t *testing.T) {
	e := newTestEnv(t, discordtest.BasicFixture(t))
	base := e.bothy.Writes()
	if code := e.run("status", "--guild", fxGuild); code != ExitOK || !strings.Contains(e.stdout.String(), "No open import source") {
		t.Errorf("status without a source: exit %d, %s", code, e.stdout.String())
	}
	if code := e.run("revert", "--guild", fxGuild, "--channel", fxGeneral); code != ExitUsage {
		t.Errorf("without a source: exit %d, want 2", code)
	}
	e.bothy.AddAgent("lotse", "Lotse", true)
	e.startSource(switchMapping)
	for channel, want := range map[string]string{fxLounge: "not a mapped channel", fxGeneral: "not imported"} {
		if code := e.run("revert", "--guild", fxGuild, "--channel", channel); code != ExitUsage {
			t.Errorf("%s: exit %d, want 2", channel, code)
		}
		if !strings.Contains(e.stderr.String(), want) {
			t.Errorf("%s: stderr lacks %q: %s", channel, want, e.stderr.String())
		}
	}
	if w := e.bothy.Writes() - base; w != 0 {
		t.Errorf("revert wrote %d times", w)
	}
}

// TestSwitchWithoutPlannedOperator (brief design note 2): a channel whose mapping plans no operator
// is switched like any other; step 3 then says that the mapping plans none.
func TestSwitchWithoutPlannedOperator(t *testing.T) {
	e, _, mapFile := switchEnv(t, "")
	if code := e.run("switch", "--guild", fxGuild, "--map", mapFile, "--channel", fxRandom, "--discord-locked"); code != ExitOK {
		t.Fatalf("exit %d: %s", code, e.stderr.String())
	}
	if !strings.Contains(e.stderr.String(), "plans no operator") {
		t.Errorf("stderr lacks the note on the operator:\n%s", e.stderr.String())
	}
}
