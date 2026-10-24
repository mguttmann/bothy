// SPDX-License-Identifier: Apache-2.0

package importtool

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/cmd/bothy/importtool"
	"github.com/OWNER/bothy/services/server/cmd/bothy/importtool/discord/discordtest"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
)

// crashBatch is the thread batch after which the hooks of the resume tests act: the second, so
// the run has advanced a cursor before (22 16 "No duplicate after a crash").
const crashBatch = 2

// TestApplyImportsFixture (22 5, 6, 16): one --apply run takes the fixture over: the spaces,
// channels and forum tags of the mapping, the threads open and done by the archive rule, every
// taken message with origin import and silent, the pins, the placeholders, the attachment within
// import.attachment_max_bytes as a file of its message and the one above it as a jump-link line;
// the lease is released at the end.
func TestApplyImportsFixture(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	src := w.source()
	res := w.mustApply(run{session: w.login()}, importtool.ExitOK)
	w.checkComplete(src)
	if t.Failed() {
		t.Log(res.stderr)
	}

	if got := w.text(`SELECT string_agg(s.name, ',' ORDER BY s.name) FROM import_items i JOIN spaces s ON s.id = i.target_id
		WHERE i.source_id = $1 AND i.item_kind = 'space'`, src); got != "Community,Projects" {
		t.Errorf("spaces %q", got)
	}
	if got := w.text(`SELECT string_agg(c.name || ':' || c.kind, ',' ORDER BY c.name) FROM import_items i JOIN channels c ON c.id = i.target_id
		WHERE i.source_id = $1 AND i.item_kind = 'channel'`, src); got != "general:text,news:text,random:text,support:forum" {
		t.Errorf("channels %q", got)
	}
	if n := w.count(`SELECT count(*) FROM forum_tags WHERE channel_id = $1 AND deleted_at IS NULL`, w.target(src, "channel:"+chSupport)); n != 2 {
		t.Errorf("%d forum tags, want 2", n)
	}
	for state, want := range map[string]int{"open": wantThreadsOpen, "closed": wantThreadsDone} {
		if n := w.count(`SELECT count(*) FROM import_items i JOIN threads t ON t.id = i.target_id
			WHERE i.source_id = $1 AND i.item_kind = 'thread' AND t.state = $2`, src, state); n != want {
			t.Errorf("%d %s threads, want %d", n, state, want)
		}
	}
	for _, th := range []string{thRelease, thLogin} {
		if got := w.text(`SELECT state FROM threads WHERE id = $1`, w.target(src, "thread:"+th)); got != "open" {
			t.Errorf("thread %s is %s, want open (active)", th, got)
		}
	}
	if n := w.count(`SELECT count(*) FROM thread_tags WHERE thread_id = ANY($1::uuid[])`,
		[]string{w.target(src, "thread:"+thLogin), w.target(src, "thread:"+thExport)}); n != 2 {
		t.Errorf("%d thread tags, want 2", n)
	}
	if n := w.count(`SELECT count(*) FROM messages WHERE origin = 'import' AND (NOT silent OR trigger_state IS NOT NULL)`); n != 0 {
		t.Errorf("%d imported messages not silent or with a trigger state", n)
	}
	if n := w.count(`SELECT count(*) FROM pins WHERE message_id = ANY($1::uuid[])`,
		[]string{w.target(src, "message:"+msgPinned), w.target(src, "message:"+msgForumPin)}); n != wantPins {
		t.Errorf("%d pins, want %d", n, wantPins)
	}
	if n := w.count(`SELECT count(*) FROM import_items i JOIN principals p ON p.id = i.target_id
		WHERE i.source_id = $1 AND i.item_kind = 'user' AND p.status = 'disabled'`, src); n != wantPlaceholders {
		t.Errorf("%d placeholders, want %d", n, wantPlaceholders)
	}
	w.checkAttachments(src)
	w.checkReplies(src)
	c := w.cursor(src)
	if c.Run != nil {
		t.Errorf("the lease is still held: %+v", c.Run)
	}
	if !strings.Contains(w.text(`SELECT stats::text FROM import_sources WHERE id = $1`, src), `"tool"`) {
		t.Error("no tool counters in the stats of the source")
	}
}

// checkAttachments holds the attachment rules of 22 6.5: the attachment within the limit is the
// file of its message, the one above it was never fetched and its message carries the line with
// the jump link instead.
func (w *world) checkAttachments(src string) {
	w.t.Helper()
	small := w.target(src, "message:"+msgSmallFile)
	if n := w.count(`SELECT count(*) FROM message_attachments a JOIN import_items i ON i.target_id = a.file_id
		WHERE a.message_id = $1 AND i.source_id = $2 AND i.origin_key = $3`, small, src, "file:"+attSmall); n != 1 {
		w.t.Errorf("the attachment within the limit is no file of its message (%d): content %q, CDN requests %+v", n,
			w.text(`SELECT content_md FROM messages WHERE id = $1`, small), w.cdn.Requests())
	}
	big := w.target(src, "message:"+msgBigFile)
	link := "https://discord.com/channels/" + guildID + "/" + chGeneral + "/" + msgBigFile
	if got := w.text(`SELECT content_md FROM messages WHERE id = $1`, big); !strings.Contains(got, link) {
		w.t.Errorf("the message of the attachment above the limit has no jump link: %q", got)
	}
	if n := w.count(`SELECT count(*) FROM message_attachments WHERE message_id = $1`, big); n != 0 {
		w.t.Errorf("the attachment above the limit was attached (%d)", n)
	}
	for _, r := range w.cdn.Requests() {
		if r.Path == filePath(attBig) {
			w.t.Error("the attachment above the limit was fetched")
		}
	}
}

// checkReplies holds the reply rules of 22 6.2: a reply to a message of the same thread points to
// it, a reply across threads carries the quote line instead.
func (w *world) checkReplies(src string) {
	w.t.Helper()
	same := w.text(`SELECT coalesce(reply_to_message_id::text, '') FROM messages WHERE id = $1`, w.target(src, "message:"+msgSameReply))
	if want := w.target(src, "message:"+msgReplied); same != want {
		w.t.Errorf("same-thread reply points to %q, want %q", same, want)
	}
	var to, content string
	if err := w.s.AdminDB().QueryRow(w.ctx, `SELECT coalesce(reply_to_message_id::text, ''), content_md FROM messages WHERE id = $1`,
		w.target(src, "message:"+msgCrossRepl)).Scan(&to, &content); err != nil {
		w.t.Fatal(err)
	}
	if to != "" || !strings.HasPrefix(content, "> @") {
		w.t.Errorf("cross-thread reply: reply_to %q, content %q; want no reply_to and a quote line", to, content)
	}
}

// TestCrashBetweenBatchAndCursor (plan acceptance, 22 16 "No duplicate after a crash"): run 1
// ends after thread batch crashBatch, before its cursor (exit 1); run 2 takes the lease run 1 left
// over with --takeover and completes; every item exists once and every thread keeps Discord
// order.
func TestCrashBetweenBatchAndCursor(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	src := w.source()
	session := w.login()
	crash := errors.New("simulated crash between batch and cursor")
	w.mustApply(run{session: session, afterBatch: func(n int) error {
		if n == crashBatch {
			return crash
		}
		return nil
	}}, importtool.ExitAborted)
	if c := w.cursor(src); c.Run == nil {
		t.Fatal("run 1 left no lease")
	}
	before := w.count(`SELECT count(*) FROM messages WHERE origin = 'import'`)
	if before == 0 || before >= wantMessages {
		t.Fatalf("run 1 imported %d messages, want some but not all", before)
	}
	w.mustApply(run{session: session, takeover: true}, importtool.ExitOK)
	w.checkComplete(src)
}

// TestRunLease (22 7.4, brief design note 8): the lease keeps two writing runs apart.
func TestRunLease(t *testing.T) {
	t.Parallel()
	t.Run("fresh lease and takeover", testLeaseTakeover)
	t.Run("renewed during a rate-limit wait", testLeaseRenewedInWait)
}

// testLeaseTakeover: while run 1 holds a fresh lease, a second run exits 3; a run with
// --takeover runs and completes; run 1, whose lease was taken over, stops at its next write with
// exit 3 and writes nothing twice.
func testLeaseTakeover(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	src := w.source()
	first, second, third := w.login(), w.login(), w.login()
	blocked, took := -1, -1
	w.mustApply(run{session: first, afterBatch: func(n int) error {
		if n == crashBatch {
			blocked = w.apply(run{session: second}).code
			took = w.apply(run{session: third, takeover: true}).code
		}
		return nil
	}}, importtool.ExitLease)
	if blocked != importtool.ExitLease {
		t.Errorf("a second run during a fresh lease exited %d, want %d", blocked, importtool.ExitLease)
	}
	if took != importtool.ExitOK {
		t.Errorf("the --takeover run exited %d, want %d", took, importtool.ExitOK)
	}
	w.checkComplete(src)
}

// testLeaseRenewedInWait: run 1 waits in a Discord rate-limit wait longer than
// import.lease_stale_after (on the fake clock); the ticker renews the lease every
// import.follow.poll_interval during the wait, so a second run started in the wait exits 3.
func testLeaseRenewedInWait(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	src := w.source()
	first, second := w.login(), w.login()
	const retryAfter = 3 * time.Second
	if retryAfter <= leaseStaleAfter || retryAfter > maxRateWait {
		t.Fatal("the wait must lie between import.lease_stale_after and import.discord.max_rate_wait")
	}
	w.fake.Inject(discordtest.APIPath+"/channels/"+thLogin+"/messages", discordtest.RateLimitBody("3"))
	tick := make(chan struct{})
	leaseSleep := func(ctx context.Context, _ time.Duration) error {
		select {
		case <-tick:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	waited, blocked := time.Duration(0), -1
	sleep := func(ctx context.Context, d time.Duration) error {
		for left := d; left > 0; {
			step := min(left, pollInterval)
			w.clk.Advance(step)
			left -= step
			waited += step
			select {
			case tick <- struct{}{}:
			case <-ctx.Done():
				return ctx.Err()
			}
			now := w.clk.Now()
			testkit.Until(t, "the lease renewed at the fake time", func() bool {
				c := w.cursor(src)
				if c.Run == nil {
					return false
				}
				at, err := time.Parse(time.RFC3339Nano, c.Run.RenewedAt)
				return err == nil && at.Equal(now)
			})
		}
		blocked = w.apply(run{session: second}).code
		return ctx.Err()
	}
	w.mustApply(run{session: first, sleep: sleep, leaseSleep: leaseSleep}, importtool.ExitOK)
	if waited != retryAfter {
		t.Errorf("the rate-limit wait took %s on the fake clock, want %s", waited, retryAfter)
	}
	if blocked != importtool.ExitLease {
		t.Errorf("a second run during the wait exited %d, want %d", blocked, importtool.ExitLease)
	}
	w.checkComplete(src)
}

// TestPausedSourceExit1 (22 10.3, brief design note 1): the owner pauses the source during a run;
// the run ends with exit 1 at its next write; after start a new run, once the lease of the first
// is older than import.lease_stale_after, resumes without --takeover and completes.
func TestPausedSourceExit1(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	src := w.source()
	session := w.login()
	res := w.mustApply(run{session: session, afterBatch: func(n int) error {
		if n == crashBatch {
			w.must(w.robin, http.MethodPost, "/imports/"+src+"/pause", nil, nil, http.StatusOK)
		}
		return nil
	}}, importtool.ExitAborted)
	if !strings.Contains(res.stderr, "import_not_running") && !strings.Contains(res.stderr, "not running") {
		t.Errorf("stderr does not name the paused source: %s", res.stderr)
	}
	w.must(w.robin, http.MethodPost, "/imports/"+src+"/start", nil, nil, http.StatusAccepted)
	w.clk.Advance(leaseStaleAfter + time.Second)
	w.mustApply(run{session: session}, importtool.ExitOK)
	w.checkComplete(src)
}

// TestMappingDiffersFromStoredExit2 (brief design note 3): a --map file whose channel name differs
// from the stored mapping ends with exit 2 naming the JSON pointer, before any write.
func TestMappingDiffersFromStoredExit2(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	src := w.source()
	m := mapping()
	m["channels"].([]any)[1].(map[string]any)["target"] = map[string]any{"create": map[string]any{"name": "announcements"}}
	res := w.mustApply(run{session: w.login(), mapFile: w.mapFile(m)}, importtool.ExitUsage)
	if !strings.Contains(res.stderr, "/channels/1/target/create/name") {
		t.Errorf("stderr does not name the JSON pointer: %s", res.stderr)
	}
	if n := w.count(`SELECT count(*) FROM import_items WHERE source_id = $1`, src); n != 0 {
		t.Errorf("%d import items written", n)
	}
	if got := w.text(`SELECT cursor::text FROM import_sources WHERE id = $1`, src); got != "{}" {
		t.Errorf("the cursor was written: %s", got)
	}
}

// TestImportedMessagesNeverTrigger (22 16 "Import never triggers", ADR 0043): random is imported
// with lead `bothy` and the planned operator Lotse; after the owner sets Lotse as the operator of the
// channel, a second run takes new messages over (one mentions the agent) and none of them creates
// a run, a follow-up, a trigger state or an agent.invocation.
func TestImportedMessagesNeverTrigger(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	src := w.source()
	session := w.login()
	w.mustApply(run{session: session}, importtool.ExitOK)

	channel := w.target(src, "channel:"+chRandom)
	r := w.must(w.robin, http.MethodGet, "/channels/"+channel, nil, nil, http.StatusOK)
	h := http.Header{}
	h.Set("If-Match", r.Header.Get("ETag"))
	w.must(w.robin, http.MethodPut, "/channels/"+channel+"/operator", map[string]any{"mode": "agent", "agent_id": w.lotse.PrincipalID,
		"reason": "switch-over"}, h, http.StatusOK)
	seq := w.count(`SELECT coalesce(max(seq), 0) FROM event_log`)

	f := w.fixture()
	f.Messages[chRandom] = append(f.Messages[chRandom],
		message("13001202", chRandom, 0, sam, "@lotse are you there?", "2026-03-03T09:00:00.000000+00:00", nil),
		message("13001203", chRandom, 0, alex, "One more question for the agent", "2026-03-03T09:05:00.000000+00:00", nil))
	w.mustApply(run{session: session, fake: discordtest.New(t, f)}, importtool.ExitOK)

	if n := w.count(`SELECT count(*) FROM messages m JOIN threads t ON t.id = m.thread_id WHERE t.channel_id = $1 AND m.origin = 'import'`,
		channel); n != 3 {
		t.Fatalf("%d imported messages in random, want 3", n)
	}
	for _, q := range []string{`SELECT count(*) FROM runs`, `SELECT count(*) FROM run_followups`,
		`SELECT count(*) FROM messages WHERE origin = 'import' AND (trigger_state IS NOT NULL OR NOT silent)`} {
		if n := w.count(q); n != 0 {
			t.Errorf("%s = %d, want 0", q, n)
		}
	}
	rows, err := w.s.AdminDB().Query(w.ctx, `SELECT type FROM event_log WHERE seq > $1`, seq)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var typ string
		if err := rows.Scan(&typ); err != nil {
			t.Fatal(err)
		}
		switch typ {
		case "agent.invocation", "agent.followup", "run.state_changed", "message.created":
			t.Errorf("the second run appended %s", typ)
		}
	}
}
