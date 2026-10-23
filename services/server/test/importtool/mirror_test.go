// SPDX-License-Identifier: Apache-2.0

package importtool

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/cmd/bothy/importtool"
	"github.com/OWNER/bothy/services/server/cmd/bothy/importtool/discord/discordtest"
)

// IDs of the read mirror test: a forum post with a placeholder name (title_source auto) and the
// three messages written in general after the import.
const (
	thAuto        = "13000303"
	msgSettled    = "13001020"
	msgBusy       = "13001021"
	msgUnsettled  = "13001022" // also the starter of the thread thNew created on it
	thNew         = msgUnsettled
	msgInNew      = "13001023"
	busyEmoji     = "\U0001F440"
	mirrorTimeout = 10 * time.Minute // the settle time passed between the two rounds, above import.follow.settle_after
)

// TestReadMirrorAgainstRealServer (22 12, 16 row "Settle rule"): after an import, Discord shows
// three new messages in general (one settled, one busy, one unsettled), a rename of a thread with
// an automatic and of one with a manual title, and an archived forum post. One --once round copies
// only the settled message, renames only the automatic thread and closes the archived one; after
// the fake clock passed import.follow.settle_after and the busy reaction is gone, the next round
// copies the rest in Discord order, and no message exists twice.
func TestReadMirrorAgainstRealServer(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	f := w.fixture()
	f.ActiveThreads = append(f.ActiveThreads, raw(t, map[string]any{"id": thAuto, "type": 11, "guild_id": guildID,
		"parent_id": chSupport, "owner_id": userAlex, "name": "Untitled", "last_message_id": thAuto, "applied_tags": []any{},
		"thread_metadata": map[string]any{"archived": false, "archive_timestamp": "2026-03-11T08:00:00.000000+00:00", "locked": false}}))
	f.Messages[thAuto] = append(f.Messages[thAuto], message(thAuto, thAuto, 0, alex, "The tablet cannot sign in.",
		"2026-03-11T08:00:00.000000+00:00", nil))
	fake := discordtest.New(t, f)
	src := w.source()
	session := w.login()
	w.mustApply(run{session: session, fake: fake}, importtool.ExitOK)
	imported := w.count(`SELECT count(*) FROM messages WHERE origin = 'import'`)

	now := w.clk.Now()
	stamp := func(at time.Time) string { return at.UTC().Format("2006-01-02T15:04:05.000000+00:00") }
	busyReaction := map[string]any{"reactions": []any{map[string]any{"count": 1, "emoji": map[string]any{"name": busyEmoji}}}}
	fake.AddMessage(chGeneral, message(msgSettled, chGeneral, 0, alex, "Settled answer.", stamp(now.Add(-mirrorTimeout)), nil))
	fake.AddMessage(chGeneral, message(msgBusy, chGeneral, 0, sam, "Working on it", stamp(now.Add(-mirrorTimeout)), busyReaction))
	fake.AddMessage(chGeneral, message(msgUnsettled, chGeneral, 0, robin, "Typing still", stamp(now), nil))
	fake.Mutate(func(fx *discordtest.Fixture) {
		fx.ActiveThreads = append(fx.ActiveThreads, raw(t, map[string]any{"id": thNew, "type": 11, "guild_id": guildID,
			"parent_id": chGeneral, "owner_id": userRobin, "name": "Typing still", "last_message_id": msgInNew,
			"thread_metadata": map[string]any{"archived": false, "archive_timestamp": stamp(now), "locked": false}}))
	})
	fake.AddMessage(thNew, message(msgInNew, thNew, 0, alex, "Answer in the thread.", stamp(now.Add(-mirrorTimeout)), nil))
	fake.RenameThread(thAuto, "Login on tablets")
	fake.RenameThread(thRelease, "Release planning, week two")
	fake.ArchiveThread(thLogin, stamp(now))

	once := run{session: session, fake: fake, extra: []string{"--once"}}
	w.mustApply(once, importtool.ExitOK)
	general := w.target(src, "channel:"+chGeneral)
	if n := w.count(`SELECT count(*) FROM import_items WHERE source_id = $1 AND origin_key = ANY($2::text[])`, src,
		[]string{"message:" + msgSettled, "message:" + msgBusy, "message:" + msgUnsettled}); n != 1 {
		t.Errorf("the first round copied %d of the three new messages, want only the settled one", n)
	}
	if w.target(src, "message:"+msgSettled) == "" {
		t.Error("the settled message was not copied")
	}
	if got := w.text(`SELECT title FROM threads WHERE id = $1`, w.target(src, "thread:"+thAuto)); got != "Login on tablets" {
		t.Errorf("the automatic title is %q, want the new name", got)
	}
	if got := w.text(`SELECT title FROM threads WHERE id = $1`, w.target(src, "thread:"+thRelease)); got != "Release planning" {
		t.Errorf("the manual title is %q, want it unchanged", got)
	}
	if got := w.text(`SELECT state FROM threads WHERE id = $1`, w.target(src, "thread:"+thLogin)); got != "closed" {
		t.Errorf("the archived forum post is %s, want closed", got)
	}

	fake.EditMessage(chGeneral, msgBusy, map[string]any{"reactions": nil})
	w.clk.Advance(mirrorTimeout)
	w.mustApply(once, importtool.ExitOK)
	if n := w.count(`SELECT count(*) FROM messages WHERE origin = 'import'`); n != imported+4 {
		t.Errorf("%d imported messages, want %d (the import, the three new ones and the one of the new thread)", n, imported+4)
	}
	var all, distinct int
	if err := w.s.AdminDB().QueryRow(w.ctx, `SELECT count(*), count(DISTINCT import_item_id) FROM messages WHERE origin = 'import'`).
		Scan(&all, &distinct); err != nil {
		t.Fatal(err)
	}
	if all != distinct {
		t.Errorf("%d imported messages for %d items: a message was copied twice", all, distinct)
	}
	w.checkOrder()
	if got := w.text(`SELECT coalesce(parent_message_id::text, '') FROM threads WHERE id = $1`, w.target(src, "thread:"+thNew)); got !=
		w.target(src, "message:"+msgUnsettled) {
		t.Errorf("the thread created on the held message has parent %q, want its starter", got)
	}
	if n := w.count(`SELECT count(*) FROM messages m JOIN threads t ON t.id = m.thread_id
		WHERE t.channel_id = $1 AND t.kind = 'main' AND m.origin = 'import'`, general); n == 0 {
		t.Error("no imported message in the main thread of general")
	}
	if c := w.cursor(src); c.Run != nil || c.Channels[chGeneral] != msgUnsettled {
		t.Errorf("cursor %+v: want no lease and general at the last new message", c)
	}
}

// raw marshals a Discord object of the fixture.
func raw(t *testing.T, v map[string]any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
