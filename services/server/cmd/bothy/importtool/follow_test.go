// SPDX-License-Identifier: Apache-2.0

package importtool

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/cmd/bothy/importtool/discord/discordtest"
)

// pathGuildChannels is the path of GET /guilds/{id}/channels of the fixture guild in the fake.
const pathGuildChannels = discordtest.APIPath + "/guilds/" + fxGuild + "/channels"

// newMessage is a raw Discord message of Alex (mapped) of type 0 in channel, written at.
func newMessage(id, channel string, at time.Time) json.RawMessage {
	b, err := json.Marshal(map[string]any{"id": id, "channel_id": channel, "type": 0, "content": "a new message",
		"timestamp": at.UTC().Format(time.RFC3339Nano), "pinned": false, "mentions": []any{},
		"author": map[string]any{"id": "13000011", "username": "alex", "global_name": "Alex", "bot": false}})
	if err != nil {
		panic(err)
	}
	return b
}

// sleeps is the FollowSleep of a test: it records every wait, runs the hook of the call (1 based)
// and advances the fake clock by the wait, so no test sleeps on the wall clock.
type sleeps struct {
	e     *testEnv
	waits []time.Duration
	hook  func(call int)
}

// sleep implements Sleeper.
func (s *sleeps) sleep(ctx context.Context, d time.Duration) error {
	s.waits = append(s.waits, d)
	if s.hook != nil {
		s.hook(len(s.waits))
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	s.e.clk.Advance(d)
	return nil
}

// follower returns a test env whose FollowSleep is a sleeps with hook and a context that the hook
// may cancel through stop.
func follower(t *testing.T, hook func(call int, stop context.CancelFunc)) (*testEnv, *sleeps, context.Context) {
	t.Helper()
	e := newTestEnv(t, discordtest.BasicFixture(t))
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	s := &sleeps{e: e, hook: func(call int) { hook(call, cancel) }}
	e.follow = s.sleep
	return e, s, ctx
}

// count returns how often key occurs in list.
func count(list []string, key string) int {
	n := 0
	for _, k := range list {
		if k == key {
			n++
		}
	}
	return n
}

// TestFailedRoundWaitsAndContinues (plan acceptance, 22 12.2 item 5): a round that fails on a 5xx
// of Discord is followed by a wait of the backoff, recorded by the sleeper, and by a successful
// round that copies the new message; a cancel (SIGINT) ends the mirror with exit 0 and a released
// lease.
func TestFailedRoundWaitsAndContinues(t *testing.T) {
	var e *testEnv
	e, s, ctx := follower(t, func(call int, stop context.CancelFunc) {
		switch call {
		case 1:
			e.discord.Inject(pathGuildChannels, discordtest.Status(http.StatusBadGateway))
		case 2:
			e.discord.AddMessage(fxGeneral, newMessage("13009001", fxGeneral, e.clk.Now().Add(-time.Hour)))
		case 4:
			stop()
		}
	})
	src := e.startSource(fixtureMapping)
	if code := e.runCtx(ctx, "--guild", fxGuild, "--map", e.writeMap(fixtureMapping), "--apply", "--follow"); code != ExitOK {
		t.Fatalf("exit %d: %s", code, e.stderr.String())
	}
	poll := DefaultConfig().FollowPollInterval
	if want := []time.Duration{poll, poll, poll, poll}; !slices.Equal(s.waits, want) {
		t.Errorf("waits %v, want %v (poll, then 2 * poll in two pieces after the failed round)", s.waits, want)
	}
	if !strings.Contains(e.stderr.String(), "mirror round failed (1 in a row)") {
		t.Errorf("stderr does not report the failed round: %s", e.stderr.String())
	}
	if n := count(e.bothy.ThreadMessages(src, "channel:"+fxGeneral), "message:13009001"); n != 1 {
		t.Errorf("the new message was copied %d times, want 1", n)
	}
	if _, held := e.bothy.SourceCursor(src)["run"]; held {
		t.Error("the lease is still held after the signal")
	}
}

// TestBackoffCapped (brief design note 3): poll_interval after a success, doubled per failed round,
// never above import.follow.max_backoff.
func TestBackoffCapped(t *testing.T) {
	cfg := DefaultConfig()
	cfg.FollowPollInterval, cfg.FollowMaxBackoff = 30*time.Second, 5*time.Minute
	for failures, want := range map[int]time.Duration{0: 30 * time.Second, 1: time.Minute, 2: 2 * time.Minute, 3: 4 * time.Minute,
		4: 5 * time.Minute, 64: 5 * time.Minute, 1 << 20: 5 * time.Minute} {
		if got := backoff(cfg, failures); got != want {
			t.Errorf("backoff after %d failures = %s, want %s", failures, got, want)
		}
	}
}

// TestLeaseFreshDuringLongBackoff (22 7.4, brief design note 3): while every round fails, the waits
// grow to import.follow.max_backoff, and at every step of the fake clock the lease is younger than
// import.lease_stale_after, so no second run could take it from the living mirror.
func TestLeaseFreshDuringLongBackoff(t *testing.T) {
	cfg := DefaultConfig()
	var e *testEnv
	var src string
	rounds := map[int]time.Duration{} // GuildChannels requests seen so far -> time slept since
	e, _, ctx := follower(t, func(call int, stop context.CancelFunc) {
		if call == 1 {
			e.discord.Always(pathGuildChannels, discordtest.Status(http.StatusServiceUnavailable))
		}
		run, _ := e.bothy.SourceCursor(src)["run"].(map[string]any)
		at, err := time.Parse(time.RFC3339Nano, run["renewed_at"].(string))
		if err != nil {
			t.Fatalf("lease: %v", err)
		}
		if age := e.clk.Now().Sub(at); age >= cfg.LeaseStaleAfter {
			t.Errorf("call %d: the lease is %s old, not younger than import.lease_stale_after", call, age)
		}
		n := 0
		for _, r := range e.discord.Requests() {
			if r.Path == pathGuildChannels {
				n++
			}
		}
		rounds[n] += cfg.FollowPollInterval
		if rounds[n] > cfg.FollowMaxBackoff+cfg.FollowPollInterval || n > 8 {
			stop()
		}
	})
	src = e.startSource(fixtureMapping)
	if code := e.runCtx(ctx, "--guild", fxGuild, "--map", e.writeMap(fixtureMapping), "--apply", "--follow"); code != ExitOK {
		t.Fatalf("exit %d: %s", code, e.stderr.String())
	}
	// Request 1 is the preflight, 2 the successful first round; the waits after the failed
	// rounds 3 to 6 double up to the cap.
	poll := cfg.FollowPollInterval
	for n, want := range map[int]time.Duration{2: poll, 3: 2 * poll, 4: 4 * poll, 5: 8 * poll, 6: cfg.FollowMaxBackoff} {
		if rounds[n] != want {
			t.Errorf("wait after request %d = %s, want %s (all %v)", n, rounds[n], want, rounds)
		}
	}
}

// TestThreadErrorDoesNotBlockOthers (brief design note 3): a 409 of Bothy for the batch of one
// thread is a note; the other threads and the main thread are copied, and the failed thread keeps
// its cursor for the next round.
func TestThreadErrorDoesNotBlockOthers(t *testing.T) {
	e := newTestEnv(t, discordtest.BasicFixture(t))
	src := e.startSource(fixtureMapping)
	e.bothy.FailBatch("thread:"+fxThread, http.StatusConflict, "item_conflict", 1)
	if code := e.run("--guild", fxGuild, "--map", e.writeMap(fixtureMapping), "--apply", "--once"); code != ExitOK {
		t.Fatalf("exit %d: %s", code, e.stderr.String())
	}
	if got := e.bothy.ThreadMessages(src, "thread:"+fxThread); len(got) != 0 {
		t.Errorf("the failed thread has messages %v", got)
	}
	if got := e.bothy.ThreadMessages(src, "channel:"+fxGeneral); len(got) == 0 {
		t.Error("the main thread of general was not copied")
	}
	if th, _ := e.bothy.SourceCursor(src)["threads"].(map[string]any); th[fxThread] != nil {
		t.Errorf("the failed thread advanced its cursor to %v", th[fxThread])
	}
	if !strings.Contains(e.stdout.String(), noteThreadFailed) {
		t.Errorf("the report does not note the failed thread:\n%s", e.stdout.String())
	}
	if code := e.run("--guild", fxGuild, "--map", e.writeMap(fixtureMapping), "--apply", "--once"); code != ExitOK {
		t.Fatalf("second round exit %d: %s", code, e.stderr.String())
	}
	if got := e.bothy.ThreadMessages(src, "thread:"+fxThread); len(got) == 0 {
		t.Error("the next round did not copy the thread")
	}
}

// leadMapping is fixtureMapping with lead bothy for general (the stored form; the file never
// needs the lead, 22 12.1).
var leadMapping = strings.Replace(fixtureMapping, `    target: {create: {name: "general"}}`,
	`    target: {create: {name: "general"}}
    lead: bothy`, 1)

// TestLeadBothyWarnsAndCopiesNothing (22 12.2 item 4): a channel with lead bothy whose Discord
// side shows messages after the cursor gets the warning row, and none of its messages is copied.
func TestLeadBothyWarnsAndCopiesNothing(t *testing.T) {
	e := newTestEnv(t, discordtest.BasicFixture(t))
	src := e.startSource(leadMapping)
	if code := e.run("--guild", fxGuild, "--map", e.writeMap(fixtureMapping), "--apply", "--once"); code != ExitOK {
		t.Fatalf("exit %d: %s", code, e.stderr.String())
	}
	for _, key := range []string{"channel:" + fxGeneral, "thread:" + fxThread} {
		if got := e.bothy.ThreadMessages(src, key); len(got) != 0 {
			t.Errorf("%s of the lead=bothy channel got messages %v", key, got)
		}
	}
	if !strings.Contains(e.stdout.String(), noteAfterSwitch) {
		t.Errorf("no warning row:\n%s", e.stdout.String())
	}
}

// TestLeadsRereadEachRound (brief design note 2 b): the lead changed in the web console between
// two rounds holds in the next round.
func TestLeadsRereadEachRound(t *testing.T) {
	var e *testEnv
	var src string
	e, _, ctx := follower(t, func(call int, stop context.CancelFunc) {
		switch call {
		case 1:
			m, err := ParseMapping([]byte(leadMapping), fxGuild)
			if err != nil {
				t.Fatal(err)
			}
			e.bothy.SetConfig(src, m.Normalized())
			e.discord.AddMessage(fxGeneral, newMessage("13009001", fxGeneral, e.clk.Now().Add(-time.Hour)))
		case 2:
			stop()
		}
	})
	src = e.startSource(fixtureMapping)
	if code := e.runCtx(ctx, "--guild", fxGuild, "--map", e.writeMap(fixtureMapping), "--apply", "--follow"); code != ExitOK {
		t.Fatalf("exit %d: %s", code, e.stderr.String())
	}
	main := e.bothy.ThreadMessages(src, "channel:"+fxGeneral)
	if len(main) == 0 || slices.Contains(main, "message:13009001") {
		t.Errorf("main thread %v: want the first round copied and the message after the switch-over not", main)
	}
	if !strings.Contains(e.stdout.String(), noteAfterSwitch) {
		t.Errorf("no warning row after the lead changed:\n%s", e.stdout.String())
	}
}

// TestDryMirrorWritesNothing (plan acceptance, 22 10.1): --follow without --apply runs a reading
// round per interval: Bothy logs only GET, Discord only GET, the working and temp folders stay
// empty, and every round prints "would copy".
func TestDryMirrorWritesNothing(t *testing.T) {
	e, _, ctx := follower(t, func(call int, stop context.CancelFunc) {
		if call == 2 {
			stop()
		}
	})
	e.startSource(fixtureMapping)
	mapFile := e.writeMap(fixtureMapping)
	work, tmp := t.TempDir(), t.TempDir()
	t.Chdir(work)
	t.Setenv("TMPDIR", tmp)
	base := len(e.bothy.Requests())
	if code := e.runCtx(ctx, "--guild", fxGuild, "--map", mapFile, "--follow"); code != ExitOK {
		t.Fatalf("exit %d: %s", code, e.stderr.String())
	}
	for _, r := range e.bothy.Requests()[base:] {
		if r.Method != http.MethodGet {
			t.Errorf("Bothy got %s %s in a dry mirror", r.Method, r.Path)
		}
	}
	if m := e.discord.Methods(); !slices.Equal(m, []string{http.MethodGet}) {
		t.Errorf("Discord methods %v", m)
	}
	emptyDir(t, work)
	emptyDir(t, tmp)
	if n := strings.Count(e.stdout.String(), "would copy"); n != 2*2 {
		t.Errorf("%d mentions of would copy, want two rounds with two columns each:\n%s", n, e.stdout.String())
	}
	if !strings.Contains(e.stdout.String(), "Mirror round (dry run)") {
		t.Errorf("no mirror round header:\n%s", e.stdout.String())
	}
}

// TestOnceExit3WhenLeaseHeld (22 10.3): --apply --once ends with exit 3 and writes nothing while
// another run's lease is fresh.
func TestOnceExit3WhenLeaseHeld(t *testing.T) {
	e := newTestEnv(t, discordtest.BasicFixture(t))
	src := e.startSource(fixtureMapping)
	e.bothy.SetCursor(src, map[string]any{"version": 1, "channels": map[string]any{}, "threads": map[string]any{},
		"run": map[string]any{"id": "another-run", "renewed_at": e.clk.Now().Add(-time.Second).Format(time.RFC3339Nano)}})
	base := e.bothy.Writes()
	if code := e.run("--guild", fxGuild, "--map", e.writeMap(fixtureMapping), "--apply", "--once"); code != ExitLease {
		t.Fatalf("exit %d, want 3: %s", code, e.stderr.String())
	}
	if w := e.bothy.Writes() - base; w != 0 {
		t.Errorf("writes = %d, want 0", w)
	}
}

// TestContradictoryOptionsExit2 (22 10.2 rule 3): --follow with --once and --only-thread with
// --follow end with exit 2 without a call to Discord or Bothy.
func TestContradictoryOptionsExit2(t *testing.T) {
	e := newTestEnv(t, discordtest.BasicFixture(t))
	mapFile := e.writeMap(fixtureMapping)
	for _, args := range [][]string{
		{"--follow", "--once"}, {"--apply", "--follow", "--once"}, {"--follow", "--only-thread", fxThread},
		{"--apply", "--follow", "--only-thread", fxThread},
	} {
		discordBase, bothyBase := len(e.discord.Requests()), len(e.bothy.Requests())
		if code := e.run(append([]string{"--guild", fxGuild, "--map", mapFile}, args...)...); code != ExitUsage {
			t.Errorf("%v: exit %d, want 2", args, code)
		}
		if len(e.discord.Requests()) != discordBase || len(e.bothy.Requests()) != bothyBase {
			t.Errorf("%v: a call reached a fake", args)
		}
	}
}

// TestPausedSourceIsNoop (brief design note 3): after the owner paused the source the round copies
// nothing and notes it; the lease is not cleared, because a paused source refuses the write.
func TestPausedSourceIsNoop(t *testing.T) {
	var e *testEnv
	var src string
	e, _, ctx := follower(t, func(call int, stop context.CancelFunc) {
		switch call {
		case 1:
			e.bothy.SetState(src, "paused")
			e.discord.AddMessage(fxGeneral, newMessage("13009001", fxGeneral, e.clk.Now().Add(-time.Hour)))
		case 2:
			stop()
		}
	})
	src = e.startSource(fixtureMapping)
	if code := e.runCtx(ctx, "--guild", fxGuild, "--map", e.writeMap(fixtureMapping), "--apply", "--follow"); code != ExitOK {
		t.Fatalf("exit %d: %s", code, e.stderr.String())
	}
	if slices.Contains(e.bothy.ThreadMessages(src, "channel:"+fxGeneral), "message:13009001") {
		t.Error("a message was copied into the paused source")
	}
	if !strings.Contains(e.stdout.String(), noteSourcePaused) {
		t.Errorf("no note of the paused source:\n%s", e.stdout.String())
	}
}

// TestCompletedSourceEndsMirror (brief design note 3): a completed source ends --follow with
// exit 0, a failed one with exit 1.
func TestCompletedSourceEndsMirror(t *testing.T) {
	for state, want := range map[string]int{"completed": ExitOK, "failed": ExitAborted} {
		t.Run(state, func(t *testing.T) {
			var e *testEnv
			var src string
			e, _, ctx := follower(t, func(call int, stop context.CancelFunc) {
				if call == 1 {
					e.bothy.SetState(src, state)
				}
				if call > 1 {
					stop()
				}
			})
			src = e.startSource(fixtureMapping)
			if code := e.runCtx(ctx, "--guild", fxGuild, "--map", e.writeMap(fixtureMapping), "--apply", "--follow"); code != want {
				t.Fatalf("exit %d, want %d: %s", code, want, e.stderr.String())
			}
		})
	}
}

// TestMirrorThreadChanges (22 12.2 item 3): a rename re-posts the thread item once; an unchanged
// thread without new messages sends no batch; an archived thread is confirmed with GET
// /channels/{id} and posted as done.
func TestMirrorThreadChanges(t *testing.T) {
	var e *testEnv
	e, _, ctx := follower(t, func(call int, stop context.CancelFunc) {
		switch call {
		case 1:
			e.discord.RenameThread(fxThread, "Release planning, week two")
		case 3:
			e.discord.ArchiveThread(fxThread, e.clk.Now().Format(time.RFC3339Nano))
		case 5:
			stop()
		}
	})
	src := e.startSource(fixtureMapping)
	if code := e.runCtx(ctx, "--guild", fxGuild, "--map", e.writeMap(fixtureMapping), "--apply", "--follow"); code != ExitOK {
		t.Fatalf("exit %d: %s", code, e.stderr.String())
	}
	n := 0
	for _, b := range e.bothy.Batches(src) {
		if b.ThreadOriginKey == "thread:"+fxThread && len(b.Keys) > 0 && b.Keys[0] == "thread:"+fxThread {
			n++
		}
	}
	// Round 1 posts the thread, round 2 the rename, round 3 nothing, round 4 the archive, round 5
	// nothing.
	if n != 3 {
		t.Errorf("%d thread batches of %s, want 3", n, fxThread)
	}
	confirm := 0
	for _, r := range e.discord.Requests() {
		if r.Path == discordtest.APIPath+"/channels/"+fxThread {
			confirm++
		}
	}
	if confirm != 1 {
		t.Errorf("GET /channels/{id} of the archived thread %d times, want 1", confirm)
	}
	if fp, _ := e.bothy.SourceCursor(src)["mirrored"].(map[string]any); !strings.HasPrefix(fp[fxThread].(string), "c:") {
		t.Errorf("the stored fingerprint %v is not done", fp[fxThread])
	}
}

// TestMirrorSettle (22 12.3): a busy message holds the thread and the messages after it; once the
// reaction is gone and the quiet time passed, the next round copies the rest in order, nothing
// twice.
func TestMirrorSettle(t *testing.T) {
	var e *testEnv
	const settled, busyID, after = "13009001", "13009002", "13009003"
	e, _, ctx := follower(t, func(call int, stop context.CancelFunc) {
		switch call {
		case 1:
			now := e.clk.Now()
			e.discord.AddMessage(fxGeneral, newMessage(settled, fxGeneral, now.Add(-time.Hour)))
			e.discord.AddMessage(fxGeneral, newMessage(busyID, fxGeneral, now.Add(-time.Hour)))
			e.discord.EditMessage(fxGeneral, busyID, map[string]any{"reactions": []any{map[string]any{"count": 1, "emoji": map[string]any{"name": "\U0001F440"}}}})
			e.discord.AddMessage(fxGeneral, newMessage(after, fxGeneral, now))
		case 2:
			e.discord.EditMessage(fxGeneral, busyID, map[string]any{"reactions": nil})
			e.clk.Advance(DefaultConfig().FollowSettleAfter)
		case 3:
			stop()
		}
	})
	src := e.startSource(fixtureMapping)
	if code := e.runCtx(ctx, "--guild", fxGuild, "--map", e.writeMap(fixtureMapping), "--apply", "--follow"); code != ExitOK {
		t.Fatalf("exit %d: %s", code, e.stderr.String())
	}
	main := e.bothy.ThreadMessages(src, "channel:"+fxGeneral)
	tail := main[len(main)-3:]
	if want := []string{"message:" + settled, "message:" + busyID, "message:" + after}; !slices.Equal(tail, want) {
		t.Errorf("tail of the main thread %v, want %v", tail, want)
	}
	for _, b := range e.bothy.Batches(src) {
		if b.ThreadOriginKey == "channel:"+fxGeneral && slices.Contains(b.Keys, "message:"+busyID) && slices.Contains(b.Keys, "message:"+settled) {
			t.Errorf("the busy message was copied in the round of the settled one: %v", b.Keys)
		}
	}
}
