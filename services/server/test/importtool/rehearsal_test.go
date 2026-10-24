// SPDX-License-Identifier: Apache-2.0

package importtool

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"go.yaml.in/yaml/v3"

	"github.com/OWNER/bothy/services/server/cmd/bothy/importtool"
	"github.com/OWNER/bothy/services/server/cmd/bothy/importtool/discord/discordtest"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
)

// guidePath is the migration guide whose rehearsal blocks the rehearsal runs, relative to this
// package.
const guidePath = "../../../../docs/operations/discord-migration.md"

// IDs the rehearsal adds to the fixture of world (invented digit strings): a media channel with
// one active post, the Discord bot of the agent Lotse, its messages and the messages of the
// parallel operation.
const (
	chGallery     = "13000108" // media channel (type 16), category Community
	thShot        = "13000331" // active post of gallery
	userLotseBot  = "13000018" // the Discord bot account of Lotse, mapped to agent:lotse
	msgBotAnswer  = "13001051" // a settled answer of Lotse's bot in general, imported as Lotse's
	msgShotAnswer = "13003311" // Lotse's bot answers in the gallery post
	msgMirrorBusy = "13001052" // parallel operation: Lotse's bot still works on it (busy reaction)
	msgMirrorSam  = "13001202" // parallel operation: settled message of Sam in random
	msgMirrorNew  = "13003003" // parallel operation: unsettled message of Alex in thLogin
	msgRobin      = "13001012" // Robin's last message in the main thread of general (fixture)
	thNewsQ       = "13001101" // active thread of news on its only message
	msgNewsQ      = "13002101" // Sam asks in thNewsQ
	thBuild       = "13001201" // active thread of random on its only message
	msgBuildDone  = "13002201" // Lotse's bot answers in thBuild
	refLogin      = "sess-invented-login-2a7c"
)

// rehearsalChannels are the mapped channels in the order the rehearsal switches them over, with
// the imported thread of each in which the probe triggers the operator (an operator answers in a
// thread without a mention, 09 require_mention) and whether the sessions file names a reference
// for that thread.
var rehearsalChannels = []struct {
	channel, thread string
	bound           bool
}{
	{chGeneral, thRelease, true},
	{chNews, thNewsQ, false},
	{chSupport, thLogin, true},
	{chRandom, thBuild, false},
	{chGallery, thShot, false},
}

// wantChannelMessages are the imported messages per channel after the parallel operation:
// general 13 of the 15 Discord messages of its main thread (the 13 of the fixture, the bot answer
// and the busy output; types 6 and 18 skipped), thRelease 3 (its type 21 starter is in the main
// thread) and the four archived threads 1 each; news 1 and thNewsQ 1; support thLogin 2 plus the
// unsettled message, thExport 1; random 1, thBuild 1 and Sam's message; gallery 2.
var wantChannelMessages = map[string]int{chGeneral: 13 + 3 + 4, chNews: 2, chSupport: 3 + 1, chRandom: 3, chGallery: 2}

// guideCommand is one command line of a rehearsal block of the guide, split into its words after
// "bothy import".
type guideCommand []string

// guideBlocks reads the fenced blocks of the guide that a line "<!-- rehearsal: <name> -->"
// marks: every line of such a block is a command "bothy import ..." or a comment. Words are
// separated by spaces; a quote is refused, so the test runs exactly what the guide prints.
func guideBlocks(t *testing.T) map[string][]guideCommand {
	t.Helper()
	data, err := os.ReadFile(guidePath)
	if err != nil {
		t.Fatalf("the guide: %v", err)
	}
	marker := regexp.MustCompile(`^<!-- rehearsal: ([a-z-]+) -->$`)
	blocks := map[string][]guideCommand{}
	sc := bufio.NewScanner(bytes.NewReader(data))
	name, state := "", 0 // 0 outside, 1 after the marker, 2 inside the fence
	for sc.Scan() {
		line := sc.Text()
		switch {
		case state == 0:
			if m := marker.FindStringSubmatch(line); m != nil {
				if _, dup := blocks[m[1]]; dup {
					t.Fatalf("the guide marks the rehearsal block %q twice", m[1])
				}
				name, state = m[1], 1
				blocks[name] = nil
			}
		case state == 1:
			if line != "```sh" {
				t.Fatalf("the rehearsal marker %q is not followed by a ```sh fence: %q", name, line)
			}
			state = 2
		case line == "```":
			if len(blocks[name]) == 0 {
				t.Fatalf("the rehearsal block %q has no command", name)
			}
			state = 0
		case strings.HasPrefix(line, "#") || strings.TrimSpace(line) == "":
		case strings.HasPrefix(line, "bothy import ") && !strings.ContainsAny(line, `"'\`):
			blocks[name] = append(blocks[name], strings.Fields(strings.TrimPrefix(line, "bothy import ")))
		default:
			t.Fatalf("the rehearsal block %q has a line the test cannot run literally: %q", name, line)
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	if state != 0 {
		t.Fatalf("the rehearsal block %q is not closed", name)
	}
	return blocks
}

// rehearsal is the state of TestMigrationRehearsal: the world, the blocks of the guide and the
// ones run, the files of the guide's commands, the directory that outlives the subtests, the
// session of the tool and the source.
type rehearsal struct {
	*world
	blocks            map[string][]guideCommand
	used              map[string]bool
	files             map[string]string
	dir, session, src string
}

// guide runs the rehearsal block name of the guide with the placeholders of vars and the file
// names of the guide replaced, and fails unless every command exits with 0. It returns the
// results.
func (r *rehearsal) guide(name string, vars map[string]string) []result {
	r.t.Helper()
	return r.guideExit(name, vars, importtool.ExitOK)
}

// block returns the commands of the rehearsal block name and marks it as run.
func (r *rehearsal) block(name string) []guideCommand {
	r.t.Helper()
	cmds, ok := r.blocks[name]
	if !ok {
		r.t.Fatalf("the guide has no rehearsal block %q", name)
	}
	r.used[name] = true
	return cmds
}

// args replaces the placeholders of vars and the file names of the guide in cmd of block name; an
// unknown placeholder fails the test.
func (r *rehearsal) args(name string, cmd guideCommand, vars map[string]string) []string {
	r.t.Helper()
	args := make([]string, len(cmd))
	for i, word := range cmd {
		switch v, isVar := vars[word]; {
		case isVar:
			args[i] = v
		case r.files[word] != "":
			args[i] = r.files[word]
		case word != ">" && strings.ContainsAny(word, "<>"):
			r.t.Fatalf("block %q: the placeholder %s has no rehearsal value", name, word)
		default:
			args[i] = word
		}
	}
	return args
}

// guideExit is guide with the exit code want of every command.
func (r *rehearsal) guideExit(name string, vars map[string]string, want int) []result {
	r.t.Helper()
	var results []result
	for _, cmd := range r.block(name) {
		args := r.args(name, cmd, vars)
		// "> file" at the end keeps the standard output in that file of the guide.
		out := ""
		if n := len(args); n > 2 && cmd[n-2] == ">" {
			out, args = args[n-1], args[:n-2]
		}
		var res result
		switch args[0] {
		case "login":
			if len(args) != 3 || args[1] != "--server" {
				r.t.Fatalf("block %q: login %v, want login --server <server_url>", name, args[1:])
			}
			r.session = r.loginTo(args[2], r.dir)
			res = result{code: importtool.ExitOK}
		case "discord":
			res = r.invoke(run{session: r.session}, args[1:]...)
		default:
			r.t.Fatalf("block %q: bothy import %s is not a command the rehearsal runs", name, args[0])
		}
		if res.code != want {
			r.t.Fatalf("block %q: bothy import %s: exit %d, want %d\nstdout: %s\nstderr: %s", name, strings.Join(cmd, " "), res.code,
				want, res.stdout, res.stderr)
		}
		if out != "" {
			if err := os.WriteFile(out, []byte(res.stdout), 0o600); err != nil {
				r.t.Fatal(err)
			}
		}
		results = append(results, res)
	}
	return results
}

// follow starts the guide's long-running --follow mirror in the background, as the service of an
// installation runs it, waits until it holds the lease of the source and returns its stop: the
// cancelled context of Ctrl-C or a stopped service, after which the mirror must end with exit 0
// and free the lease. Its FollowSleep waits for the stop, so the lease is not renewed in between
// and stays fresh on the fake clock.
func (r *rehearsal) follow(vars map[string]string) func() {
	r.t.Helper()
	cmds := r.block("follow")
	if len(cmds) != 1 || cmds[0][0] != "discord" {
		r.t.Fatalf("the follow block %v is not one bothy import discord command", cmds)
	}
	args := r.args("follow", cmds[0], vars)
	ctx, cancel := context.WithCancel(r.ctx)
	done := make(chan result, 1)
	go func() { done <- r.invoke(run{session: r.session, ctx: ctx, followSleep: untilDone}, args[1:]...) }()
	testkit.Until(r.t, "the --follow mirror holds the lease of the source", func() bool { return r.cursor(r.src).Run != nil })
	return func() {
		r.t.Helper()
		cancel()
		if res := <-done; res.code != importtool.ExitOK {
			r.t.Fatalf("the stopped --follow mirror: exit %d, want 0\nstderr: %s", res.code, res.stderr)
		}
		if c := r.cursor(r.src); c.Run != nil {
			r.t.Fatalf("the stopped --follow mirror kept the lease %+v", c.Run)
		}
	}
}

// file reads the file of the guide named name.
func (r *rehearsal) file(name string) string {
	r.t.Helper()
	data, err := os.ReadFile(r.files[name])
	if err != nil {
		r.t.Fatalf("the guide's file %s: %v", name, err)
	}
	return string(data)
}

// rehearsalFixture is the fixture of world with a media channel and its post, the Discord bot of
// Lotse with answers in general, in the post and in a thread of random, and a thread of news.
func (w *world) rehearsalFixture() *discordtest.Fixture {
	f := w.fixture()
	lotseBot := map[string]any{"id": userLotseBot, "username": "lotse-bot", "global_name": nil, "bot": true}
	f.Channels = append(f.Channels, raw(w.t, map[string]any{"id": chGallery, "type": 16, "guild_id": guildID, "parent_id": catCommunity,
		"name": "gallery", "position": 5, "permission_overwrites": []any{}, "available_tags": []any{}}))
	f.ActiveThreads = append(f.ActiveThreads, raw(w.t, map[string]any{"id": thShot, "type": 11, "guild_id": guildID,
		"parent_id": chGallery, "owner_id": userAlex, "name": "Dashboard screenshot", "last_message_id": msgShotAnswer, "applied_tags": []any{},
		"thread_metadata": map[string]any{"archived": false, "archive_timestamp": "2026-03-12T08:00:00.000000+00:00", "locked": false}}))
	f.Messages[thShot] = []json.RawMessage{
		message(thShot, thShot, 0, alex, "The new dashboard, does it look right?", "2026-03-12T08:00:00.000000+00:00", nil),
		message(msgShotAnswer, thShot, 0, lotseBot, "The chart legend overlaps the axis.", "2026-03-12T08:01:00.000000+00:00", nil),
	}
	for _, th := range []struct{ id, parent, name, msg string }{{thNewsQ, chNews, "Release notes", msgNewsQ}, {thBuild, chRandom, "Build",
		msgBuildDone}} {
		f.ActiveThreads = append(f.ActiveThreads, raw(w.t, map[string]any{"id": th.id, "type": 11, "guild_id": guildID, "parent_id": th.parent,
			"owner_id": userAlex, "name": th.name, "last_message_id": th.msg,
			"thread_metadata": map[string]any{"archived": false, "archive_timestamp": "2026-03-13T08:00:00.000000+00:00", "locked": false}}))
	}
	f.Messages[thNewsQ] = []json.RawMessage{message(msgNewsQ, thNewsQ, 0, sam, "Where are the release notes?",
		"2026-03-02T09:00:00.000000+00:00", nil)}
	f.Messages[thBuild] = []json.RawMessage{message(msgBuildDone, thBuild, 0, lotseBot, "The build is green again.",
		"2026-03-03T09:00:00.000000+00:00", nil)}
	f.Messages[chGeneral] = append(f.Messages[chGeneral],
		message(msgBotAnswer, chGeneral, 0, lotseBot, "Build summary: all checks passed.", "2026-03-01T11:00:00.000000+00:00", nil))
	return f
}

// completeMapping is the owner's edit of the skeleton init-map wrote (22 13.2 step 1): the
// channel without category goes into Projects, every channel gets Lotse as its planned operator,
// Alex, Robin and Lotse's Discord bot are mapped, every other author stays a placeholder.
func (r *rehearsal) completeMapping() map[string]any {
	r.t.Helper()
	data, err := os.ReadFile(r.files["mapping.yaml"])
	if err != nil {
		r.t.Fatal(err)
	}
	var m map[string]any
	if err := yaml.Unmarshal(data, &m); err != nil {
		r.t.Fatalf("the skeleton: %v", err)
	}
	m["defaults"].(map[string]any)["uncategorized_space"] = catProjects
	channels := map[string]bool{}
	for _, c := range m["channels"].([]any) {
		e := c.(map[string]any)
		e["planned_operator"] = "agent:lotse"
		channels[e["discord_channel"].(string)] = true
	}
	for _, c := range rehearsalChannels {
		if !channels[c.channel] {
			r.t.Fatalf("the skeleton lacks channel %s: %v", c.channel, m["channels"])
		}
	}
	targets := map[string]string{userAlex: "user:alex", userRobin: "user:robin", userLotseBot: "agent:lotse"}
	seen := 0
	for _, u := range m["users"].([]any) {
		e := u.(map[string]any)
		if target, ok := targets[e["discord_user"].(string)]; ok {
			e["target"] = target
			seen++
		} else if e["target"] != "placeholder" {
			r.t.Fatalf("skeleton user %v is not a placeholder", e)
		}
	}
	if seen != len(targets) {
		r.t.Fatalf("the skeleton names %d of the %d mapped authors: %v", seen, len(targets), m["users"])
	}
	out, err := yaml.Marshal(m)
	if err != nil {
		r.t.Fatal(err)
	}
	if err := os.WriteFile(r.files["mapping.yaml"], out, 0o600); err != nil {
		r.t.Fatal(err)
	}
	return m
}

// channelMessages counts the imported messages in every thread of the server channel of the
// Discord channel ch.
func (r *rehearsal) channelMessages(ch string) int {
	r.t.Helper()
	return r.count(`SELECT count(*) FROM messages m JOIN threads t ON t.id = m.thread_id WHERE t.channel_id = $1 AND m.origin = 'import'`,
		r.target(r.src, "channel:"+ch))
}

// probeThread is the server thread of the imported Discord thread.
func (r *rehearsal) probeThread(thread string) string {
	r.t.Helper()
	return r.target(r.src, "thread:"+thread)
}

// lead sets the lead of ch in the web console (setImportLead as Robin with step-up).
func (r *rehearsal) lead(ch, value string) {
	r.t.Helper()
	r.must(r.robin, http.MethodPut, "/imports/"+r.src+"/leads", map[string]any{"source_channel": ch, "lead": value}, nil, http.StatusOK)
}

// operator sets the operator of the server channel of ch in the web console: Lotse, or none.
func (r *rehearsal) operator(ch string, set bool) {
	r.t.Helper()
	channel := r.target(r.src, "channel:"+ch)
	etag := r.must(r.robin, http.MethodGet, "/channels/"+channel, nil, nil, http.StatusOK).Header.Get("ETag")
	body := map[string]any{"mode": "none", "reason": "revert"}
	if set {
		body = map[string]any{"mode": "agent", "agent_id": r.lotse.PrincipalID, "reason": "switch-over"}
	}
	h := http.Header{}
	h.Set("If-Match", etag)
	r.must(r.robin, http.MethodPut, "/channels/"+channel+"/operator", body, h, http.StatusOK)
}

// switchOver runs 22 13.3 for ch: the guide's switch command, then the console steps (lead
// bothy, the planned operator Lotse).
func (r *rehearsal) switchOver(ch string) {
	r.t.Helper()
	res := r.guide("switch", map[string]string{"<guild_id>": guildID, "<channel_id>": ch})
	if !strings.Contains(res[0].stderr, "agent:lotse") {
		r.t.Fatalf("switch %s printed no step 3 with the planned operator:\n%s", ch, res[0].stderr)
	}
	r.lead(ch, "bothy")
	r.operator(ch, true)
}

// probe is step 4 of 22 13.3 in ch: Alex writes in the imported thread; exactly one new run of
// Lotse starts, it binds the imported session where the sessions file names one, Lotse answers on
// the server and finishes.
func (r *rehearsal) probe(ch, thread string, bound bool) {
	r.t.Helper()
	th := r.probeThread(thread)
	before := r.runs(th)
	r.post(th, "Test trigger after the switch-over")
	runs := r.runs(th)
	if len(runs) != len(before)+1 {
		r.t.Fatalf("channel %s: %d new runs after the probe, want 1", ch, len(runs)-len(before))
	}
	run := runs[len(runs)-1]
	if p := r.invocation(th, run); bound && len(before) == 0 && p["session_mode"] != "bind" {
		r.t.Fatalf("channel %s: the probe has session_mode %v, want bind", ch, p["session_mode"])
	}
	r.start(run)
	if n := r.count(`SELECT count(*) FROM messages WHERE thread_id = $1 AND run_id = $2 AND author_principal_id = $3`, th, run,
		r.lotse.PrincipalID); n != 1 {
		r.t.Fatalf("channel %s: %d answers of Lotse, want 1", ch, n)
	}
	r.finish(run)
}

// TestMigrationRehearsal (PLAN 14 exit criterion 2, 22 13): the whole migration flow of the guide
// docs/operations/discord-migration.md on fakes against the real server, with the guide's
// commands run literally from its rehearsal blocks: login and init-map, a dry run over the whole
// source without abort, the console steps through the API, a pilot of two channels, the full
// import, parallel operation with mirror rounds and spot checks, the switch-over of every channel
// with a test trigger, one revert and switch-over again, and the completion: every channel with
// lead bothy, the source completed, Discord saw only GET and no message exists twice.
func TestMigrationRehearsal(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	w.fake = discordtest.New(t, w.rehearsalFixture())
	dir := t.TempDir()
	r := &rehearsal{world: w, dir: dir, blocks: guideBlocks(t), used: map[string]bool{}, files: map[string]string{
		"mapping.yaml": filepath.Join(dir, "mapping.yaml"), "sessions.jsonl": filepath.Join(dir, "sessions.jsonl"),
		"pilot-report.json": filepath.Join(dir, "pilot-report.json"), "import-report.json": filepath.Join(dir, "import-report.json")}}
	sessions := `{"discord_thread": "` + thRelease + `", "agent": "lotse", "external_session_ref": "` + refRelease + `"}` + "\n" +
		`{"discord_thread": "` + thLogin + `", "agent": "lotse", "external_session_ref": "` + refLogin + `"}` + "\n"
	if err := os.WriteFile(r.files["sessions.jsonl"], []byte(sessions), 0o600); err != nil {
		t.Fatal(err)
	}
	vars := map[string]string{"<server_url>": w.s.PublicURL, "<guild_id>": guildID}
	step := func(name string, fn func(t *testing.T)) {
		if !t.Run(name, func(t *testing.T) { r.t = t; fn(t) }) {
			t.FailNow()
		}
		r.t = t
	}

	step("a dry run over the whole source", func(t *testing.T) {
		r.guide("login", vars)
		r.guide("init-map", vars)
		r.completeMapping()
		res := r.guide("dry-run", vars)
		if !strings.Contains(res[0].stdout, "Check report (dry run)") {
			t.Fatalf("dry run report:\n%s", res[0].stdout)
		}
		if m := w.fake.Methods(); !slices.Equal(m, []string{http.MethodGet}) {
			t.Fatalf("Discord got methods %v, want only GET", m)
		}
		if n := w.count(`SELECT count(*) FROM import_sources`); n != 0 {
			t.Fatalf("the dry run left %d sources", n)
		}
	})

	step("b console creates and starts the source", func(t *testing.T) {
		r.src = w.sourceWith(r.completeMapping())
	})

	step("c pilot of two channels", func(t *testing.T) {
		pilot := map[string]string{"<guild_id>": guildID, "<channel_id>": chGeneral, "<second_channel_id>": chSupport}
		r.guide("pilot", pilot)
		rep := reportChannels(t, r.file("pilot-report.json"))
		if len(rep) != 2 || rep["#general"]["messages_taken"] != float64(13+3+4-1) || rep["#support"]["messages_taken"] != float64(3) {
			t.Fatalf("pilot report %v", rep)
		}
		for _, ch := range []string{chGeneral, chSupport} {
			if got, want := r.channelMessages(ch), wantChannelMessages[ch]-mirrorAdds(ch); got != want {
				t.Errorf("channel %s: %d imported messages, want %d", ch, got, want)
			}
		}
		// The read mirror of the pilot stays on the pilot channels.
		r.guide("pilot-mirror", pilot)
		if n := w.count(`SELECT count(*) FROM import_items WHERE source_id = $1 AND item_kind = 'channel'`, r.src); n != 2 {
			t.Fatalf("%d channels imported in the pilot, want 2", n)
		}
	})

	step("d full import and parallel operation with spot checks", func(t *testing.T) {
		r.guide("import", vars)
		rep := reportChannels(t, r.file("import-report.json"))
		// The pilot channels resume at their cursors: nothing is taken twice.
		for _, ch := range []string{"#general", "#support"} {
			if rep[ch]["messages_taken"] != float64(0) {
				t.Errorf("full import took messages of the pilot channel %s again: %v", ch, rep[ch])
			}
		}
		for ch, want := range map[string]int{"#news": 2, "#random": 2, "#gallery": 2} {
			if rep[ch]["messages_taken"] != float64(want) {
				t.Errorf("full import in %s: %v, want %d messages taken", ch, rep[ch], want)
			}
		}
		// Round 1: Lotse's bot still works on an output in general, Sam writes in random, Alex is
		// still typing in thLogin: only Sam's message is copied.
		now := w.clk.Now()
		stamp := func(at time.Time) string { return at.UTC().Format("2006-01-02T15:04:05.000000+00:00") }
		lotseBot := map[string]any{"id": userLotseBot, "username": "lotse-bot", "global_name": nil, "bot": true}
		busy := map[string]any{"reactions": []any{map[string]any{"count": 1, "emoji": map[string]any{"name": busyEmoji}}}}
		w.fake.AddMessage(chGeneral, message(msgMirrorBusy, chGeneral, 0, lotseBot, "Working on the release notes", stamp(now.Add(-mirrorTimeout)), busy))
		w.fake.AddMessage(chRandom, message(msgMirrorSam, chRandom, 0, sam, "Lunch at noon?", stamp(now.Add(-mirrorTimeout)), nil))
		w.fake.AddMessage(thLogin, message(msgMirrorNew, thLogin, 0, alex, "It works again after the update", stamp(now), nil))
		r.guide("mirror", vars)
		for id, want := range map[string]int{msgMirrorBusy: 0, msgMirrorSam: 1, msgMirrorNew: 0} {
			if n := w.count(`SELECT count(*) FROM import_items WHERE source_id = $1 AND origin_key = $2`, r.src, "message:"+id); n != want {
				t.Errorf("after round 1 message %s has %d items, want %d", id, n, want)
			}
		}
		// Round 2: the output is done and the settle time has passed: the rest is copied once.
		w.fake.EditMessage(chGeneral, msgMirrorBusy, map[string]any{"reactions": nil, "content": "Release notes are ready."})
		w.clk.Advance(mirrorTimeout)
		r.guide("mirror", vars)
		// A further round finds nothing new and copies nothing twice.
		r.guide("mirror", vars)
		r.spotChecks()
		res := r.guide("status", vars)
		for _, c := range rehearsalChannels {
			if !strings.Contains(res[0].stdout, c.channel+": discord") || strings.Contains(res[0].stdout, "inconsistent") {
				t.Fatalf("status in parallel operation:\n%s", res[0].stdout)
			}
		}
	})

	step("e switch every channel over with a test trigger", func(t *testing.T) {
		for _, s := range []string{catProjects, catCommunity} {
			w.must(w.s.Owner, http.MethodPost, "/grants", map[string]any{"subject_kind": "user", "subject_id": w.alex.PrincipalID,
				"level": "trigger", "target_kind": "space", "target_id": w.target(r.src, "space:"+s), "reason": "imported space"}, nil,
				http.StatusCreated)
		}
		w.must(w.lotse, http.MethodGet, "/agents/me/consumers/default/events", nil, nil, http.StatusOK)
		for _, c := range rehearsalChannels {
			w.must(w.s.Owner, http.MethodPost, "/grants", map[string]any{"subject_kind": "agent", "subject_id": w.lotse.PrincipalID,
				"level": "write", "target_kind": "channel", "target_id": w.target(r.src, "channel:"+c.channel), "reason": "imported channel"},
				nil, http.StatusCreated)
			if c.channel == chGeneral {
				// A running --follow mirror holds the lease: switch ends with exit 3 until the mirror
				// is stopped (the guide, section 7 "One writing run at a time").
				stop := r.follow(vars)
				r.guideExit("switch", map[string]string{"<guild_id>": guildID, "<channel_id>": c.channel}, importtool.ExitLease)
				stop()
			}
			r.switchOver(c.channel)
			r.probe(c.channel, c.thread, c.bound)
			if c.channel == chGeneral {
				r.revertAndSwitchAgain(c.channel, c.thread)
			}
		}
		for _, th := range []string{thRelease, thLogin} {
			if n := w.count(`SELECT count(*) FROM agent_sessions WHERE thread_id = $1 AND is_current AND external_session_ref = ANY($2::text[])`,
				w.target(r.src, "thread:"+th), []string{refRelease, refLogin}); n != 1 {
				t.Errorf("thread %s: %d bound imported sessions, want 1", th, n)
			}
		}
	})

	step("f completion", func(t *testing.T) {
		w.must(w.robin, http.MethodPost, "/imports/"+r.src+"/complete", nil, nil, http.StatusOK)
		if got := w.text(`SELECT state FROM import_sources WHERE id = $1`, r.src); got != "completed" {
			t.Fatalf("source %s, want completed", got)
		}
		if n := w.count(`SELECT count(*) FROM import_sources s, jsonb_array_elements(s.config->'channels') c
			WHERE s.id = $1 AND coalesce(c->>'lead', 'discord') <> 'bothy'`, r.src); n != 0 {
			t.Fatalf("%d channels without lead bothy", n)
		}
		if n := w.count(`SELECT count(*) FROM channels WHERE id = ANY($1::uuid[]) AND read_only`, r.serverChannels()); n != 0 {
			t.Fatalf("%d channels still read-only", n)
		}
		r.noDuplicates()
		if m := w.fake.Methods(); !slices.Equal(m, []string{http.MethodGet}) {
			t.Fatalf("Discord got methods %v, want only GET", m)
		}
		for _, req := range w.cdn.Requests() {
			if req.Method != http.MethodGet {
				t.Fatalf("the CDN got %s", req.Method)
			}
		}
		for name := range r.blocks {
			if !r.used[name] {
				t.Errorf("the rehearsal block %q of the guide is never run", name)
			}
		}
	})
}

// mirrorAdds is the number of messages of ch that the parallel operation adds after the import.
func mirrorAdds(ch string) int {
	switch ch {
	case chGeneral, chSupport, chRandom:
		return 1
	}
	return 0
}

// reportChannels decodes the channel rows of a JSON report into their counts by channel name.
func reportChannels(t *testing.T, out string) map[string]map[string]any {
	t.Helper()
	var rep struct {
		Channels []struct {
			Channel string         `json:"channel"`
			Counts  map[string]any `json:"counts"`
		} `json:"channels"`
	}
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		t.Fatalf("report %q: %v", out, err)
	}
	m := map[string]map[string]any{}
	for _, c := range rep.Channels {
		m[c.Channel] = c.Counts
	}
	return m
}

// spotChecks are the checks of parallel operation (22 13.2 step 3) against the fixture: per
// channel the imported messages equal the fixture, a sample of messages has the mapped author
// (a person, a placeholder, the agent of the bot), the attachments follow 22 6.5 and the replies
// 22 6.2, every thread keeps Discord order, and the imported session references exist.
func (r *rehearsal) spotChecks() {
	r.t.Helper()
	for _, c := range rehearsalChannels {
		if got := r.channelMessages(c.channel); got != wantChannelMessages[c.channel] {
			r.t.Errorf("channel %s: %d imported messages, want %d", c.channel, got, wantChannelMessages[c.channel])
		}
	}
	author := func(id string) (string, string) {
		var who, status string
		if err := r.s.AdminDB().QueryRow(r.ctx, `SELECT p.id::text, p.status FROM messages m JOIN principals p ON p.id = m.author_principal_id
			WHERE m.id = $1`, r.target(r.src, "message:"+id)).Scan(&who, &status); err != nil {
			r.t.Fatal(err)
		}
		return who, status
	}
	for id, want := range map[string]string{msgSmallFile: r.alex.PrincipalID, msgRobin: r.robin.PrincipalID, thLogin: "", msgForumPin: r.alex.PrincipalID,
		msgBotAnswer: r.lotse.PrincipalID, msgBuildDone: r.lotse.PrincipalID, msgNewsQ: "", msgMirrorBusy: r.lotse.PrincipalID, msgShotAnswer: r.lotse.PrincipalID, msgMirrorSam: ""} {
		who, status := author(id)
		switch {
		case want == "" && status != "disabled":
			r.t.Errorf("message %s: author %s is %s, want a placeholder", id, who, status)
		case want != "" && who != want:
			r.t.Errorf("message %s: author %s, want %s", id, who, want)
		}
	}
	if got := r.text(`SELECT content_md FROM messages WHERE id = $1`, r.target(r.src, "message:"+msgMirrorBusy)); got != "Release notes are ready." {
		r.t.Errorf("the busy output was copied as %q, want its settled text", got)
	}
	r.checkAttachments(r.src)
	r.checkReplies(r.src)
	r.checkOrder()
	if n := r.count(`SELECT count(*) FROM agent_sessions WHERE end_reason = 'import' AND external_session_ref = ANY($1::text[])`,
		[]string{refRelease, refLogin}); n != 2 {
		r.t.Errorf("%d imported session references, want 2", n)
	}
}

// revertAndSwitchAgain is 22 13.4 for ch after its probe: the guide's revert prints the two
// messages written on the server (the probe and Lotse's answer); the console gives the lead back
// and removes the operator; the guide's mirror round resumes; then the channel switches over again
// and a second probe passes.
func (r *rehearsal) revertAndSwitchAgain(ch, thread string) {
	r.t.Helper()
	res := r.guide("revert", map[string]string{"<guild_id>": guildID, "<channel_id>": ch})
	if !strings.Contains(res[0].stdout, r.probeThread(thread)+": 2\n") || !strings.Contains(res[0].stdout, "Total: 2\n") {
		r.t.Fatalf("revert output:\n%s", res[0].stdout)
	}
	r.lead(ch, "discord")
	r.operator(ch, false)
	if got := r.text(`SELECT read_only::text FROM channels WHERE id = $1`, r.target(r.src, "channel:"+ch)); got != "true" {
		r.t.Fatalf("channel %s is writable after the revert", ch)
	}
	r.guide("mirror", map[string]string{"<guild_id>": guildID})
	r.switchOver(ch)
	r.probe(ch, thread, false)
}

// serverChannels are the server channels of the mapped channels.
func (r *rehearsal) serverChannels() []string {
	r.t.Helper()
	out := make([]string, 0, len(rehearsalChannels))
	for _, c := range rehearsalChannels {
		out = append(out, r.target(r.src, "channel:"+c.channel))
	}
	return out
}

// noDuplicates holds that every origin key of the source exists once, no item failed, and every
// imported message belongs to one item (22 7).
func (r *rehearsal) noDuplicates() {
	r.t.Helper()
	if n := r.count(`SELECT count(*) FROM (SELECT origin_key FROM import_items WHERE source_id = $1 GROUP BY origin_key HAVING count(*) > 1) d`,
		r.src); n != 0 {
		r.t.Errorf("%d origin keys more than once", n)
	}
	if n := r.count(`SELECT count(*) FROM import_items WHERE source_id = $1 AND state <> 'posted'`, r.src); n != 0 {
		r.t.Errorf("%d items not posted", n)
	}
	var all, distinct int
	if err := r.s.AdminDB().QueryRow(r.ctx, `SELECT count(*), count(DISTINCT import_item_id) FROM messages WHERE origin = 'import'`).
		Scan(&all, &distinct); err != nil {
		r.t.Fatal(err)
	}
	total := 0
	for _, n := range wantChannelMessages {
		total += n
	}
	if all != distinct || all != total {
		r.t.Errorf("%d imported messages for %d items, want %d each", all, distinct, total)
	}
}
