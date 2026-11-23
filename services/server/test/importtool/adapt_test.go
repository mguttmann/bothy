// SPDX-License-Identifier: Apache-2.0

package importtool

import (
	"encoding/json"
	"maps"
	"slices"
	"strconv"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"

	"github.com/OWNER/bothy/services/server/cmd/bothy/importtool"
	"github.com/OWNER/bothy/services/server/cmd/bothy/importtool/discord/discordtest"
	"github.com/OWNER/bothy/services/server/internal/kernel/textcheck"
	"github.com/OWNER/bothy/services/server/test/harness"
)

// Messages the tests of this file add to the channel news (after its one fixture message).
const (
	msgNewsA = "13001102"
	msgNewsB = "13001103"
	msgNewsC = "13001104"
	msgNewsD = "13001105"
	msgNewsE = "13001106"
)

// The data of the fixture that the server refuses as it stands (22 5.1, 5.1.1, 6.1, 6.6): a topic
// with line breaks, a bidi control and more characters than channel.topic_max_chars, names with
// control and bidi characters.
const (
	rightToLeftOverride = "\u202e"
	rightToLeftMark     = "\u200f"
	rawTopic            = "Everyday talk\nwith a second line\tand a tab " + rightToLeftOverride
	cleanTopicStart     = "Everyday talk with a second line and a tab "
)

// reportJSON is the part of the JSON report (22 11) the tests of this file read.
type reportJSON struct {
	Channels []struct {
		Channel string `json:"channel"`
		Counts  struct {
			MessagesTaken        int            `json:"messages_taken"`
			MessagesSkipped      map[string]int `json:"messages_skipped"`
			AttachmentsForwarded int            `json:"attachments_forwarded"`
			PinsTaken            int            `json:"pins_taken"`
			PinsNotTaken         int            `json:"pins_not_taken"`
		} `json:"counts"`
		Notes []string `json:"notes"`
	} `json:"channels"`
}

// channelRow returns the row of the Discord channel name of the JSON report in out.
func channelRow(t *testing.T, out, name string) (row struct {
	Empty, Taken, Forwarded, Pins, PinsNotTaken int
	Notes                                       []string
}) {
	t.Helper()
	var rep reportJSON
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		t.Fatalf("the report is not one JSON object: %v\n%s", err, out)
	}
	for _, c := range rep.Channels {
		if c.Channel == name {
			row.Empty, row.Taken, row.Forwarded = c.Counts.MessagesSkipped["empty"], c.Counts.MessagesTaken, c.Counts.AttachmentsForwarded
			row.Pins, row.PinsNotTaken, row.Notes = c.Counts.PinsTaken, c.Counts.PinsNotTaken, c.Notes
			return row
		}
	}
	t.Fatalf("the report has no row %s:\n%s", name, out)
	return row
}

// withMembers returns the raw Discord object with members set.
func withMembers(t *testing.T, object json.RawMessage, members map[string]any) json.RawMessage {
	t.Helper()
	var o map[string]any
	if err := json.Unmarshal(object, &o); err != nil {
		t.Fatal(err)
	}
	maps.Copy(o, members)
	return raw(t, o)
}

// edit sets members of the object with id in list.
func edit(t *testing.T, list []json.RawMessage, id string, members map[string]any) {
	t.Helper()
	for i, o := range list {
		var head struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(o, &head); err != nil {
			t.Fatal(err)
		}
		if head.ID == id {
			list[i] = withMembers(t, o, members)
			return
		}
	}
	t.Fatalf("the fixture has no object %s", id)
}

// jsonRun is an --apply run of w against fake that prints the JSON report.
func jsonRun(session string, fake *discordtest.Fake) run {
	return run{session: session, fake: fake, extra: []string{"--report", "json"}}
}

// checkDisplayText holds that got passes the display text rule of the server (SEC-IN-4).
func checkDisplayText(t *testing.T, what, got string) {
	t.Helper()
	if err := textcheck.DisplayText(got); err != nil {
		t.Errorf("%s %q breaks the display text rule: %v", what, got, err)
	}
}

// TestApplyFitsTopicToServerRules (22 5.1): a Discord topic with line breaks, a tab and a bidi
// control, longer than channel.topic_max_chars, and a forum topic above that limit do not stop the
// import: the channels get the topic without those characters, cut to the limit, and the report
// notes it, also in the dry run. Before, the server refused the structure batch and every run
// ended with exit 1 at the same place.
func TestApplyFitsTopicToServerRules(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	f := w.fixture()
	edit(t, f.Channels, chGeneral, map[string]any{"topic": rawTopic + strings.Repeat("x", 1100)})
	edit(t, f.Channels, chSupport, map[string]any{"topic": strings.Repeat("y", 2000)})
	fake := discordtest.New(t, f)
	session := w.login()

	dry := w.invoke(jsonRun(session, fake), "--guild", guildID, "--map", w.mapFile(mapping()), "--report", "json")
	if dry.code != importtool.ExitOK {
		t.Fatalf("dry run: exit %d\n%s", dry.code, dry.stderr)
	}
	src := w.source()
	res := w.mustApply(jsonRun(session, fake), importtool.ExitOK)

	limit, err := strconv.Atoi(w.s.ConfigValue("channel.topic_max_chars"))
	if err != nil {
		t.Fatal(err)
	}
	general := w.text(`SELECT topic FROM channels WHERE id = $1`, w.target(src, "channel:"+chGeneral))
	checkDisplayText(t, "the topic of general", general)
	if !strings.HasPrefix(general, cleanTopicStart+"xxx") || utf8.RuneCountInString(general) != limit {
		t.Errorf("the topic of general has %d characters and starts %q; want %d characters starting %q",
			utf8.RuneCountInString(general), general[:min(len(general), 60)], limit, cleanTopicStart)
	}
	if got := w.text(`SELECT topic FROM channels WHERE id = $1`, w.target(src, "channel:"+chSupport)); got != strings.Repeat("y", limit) {
		t.Errorf("the topic of the forum has %d characters, want it cut to %d", utf8.RuneCountInString(got), limit)
	}
	for name, out := range map[string]string{"dry run": dry.stdout, "apply": res.stdout} {
		for _, ch := range []string{"#general", "#support"} {
			if row := channelRow(t, out, ch); !slices.ContainsFunc(row.Notes, func(n string) bool { return strings.Contains(n, "topic") }) {
				t.Errorf("%s: the row %s does not note the changed topic: %v", name, ch, row.Notes)
			}
		}
	}
	w.checkComplete(src)
}

// TestApplyFitsThreadNamesToServerRules (22 5.1.1): a thread name with a bidi control or a control
// character does not stop the import: the thread gets the name without them, and the report notes
// it. Before, the server refused the thread batch and every run ended with exit 1 at the same
// place.
func TestApplyFitsThreadNamesToServerRules(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	f := w.fixture()
	edit(t, f.ActiveThreads, thRelease, map[string]any{"name": "Release" + rightToLeftOverride + " planning\u0007"})
	edit(t, f.ActiveThreads, thLogin, map[string]any{"name": "Login fails\ton the second device"})
	fake := discordtest.New(t, f)
	src := w.source()
	res := w.mustApply(jsonRun(w.login(), fake), importtool.ExitOK)

	for thread, want := range map[string]string{thRelease: "Release planning", thLogin: "Login fails on the second device"} {
		got := w.text(`SELECT title FROM threads WHERE id = $1`, w.target(src, "thread:"+thread))
		checkDisplayText(t, "the title of thread "+thread, got)
		if got != want {
			t.Errorf("thread %s has the title %q, want %q", thread, got, want)
		}
	}
	for _, ch := range []string{"#general", "#support"} {
		if row := channelRow(t, res.stdout, ch); !slices.ContainsFunc(row.Notes, func(n string) bool { return strings.Contains(n, "name") }) {
			t.Errorf("the row %s does not note the changed name: %v", ch, row.Notes)
		}
	}
	w.checkComplete(src)
}

// TestApplyFitsTagNameToServerRules (22 6.6): a forum tag name with a bidi control does not stop
// the import: the tag gets the name without it and the forum post keeps its tag. Before, the server
// refused the structure batch of the tag and every run ended with exit 1 at the same place.
func TestApplyFitsTagNameToServerRules(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	f := w.fixture()
	edit(t, f.Channels, chSupport, map[string]any{"available_tags": []any{
		map[string]any{"id": "13000201", "name": "open" + rightToLeftMark},
		map[string]any{"id": "13000202", "name": "done", "emoji_name": "\u2705"},
	}})
	fake := discordtest.New(t, f)
	src := w.source()
	res := w.mustApply(jsonRun(w.login(), fake), importtool.ExitOK)

	tags := w.text(`SELECT string_agg(name, ',' ORDER BY name) FROM forum_tags WHERE channel_id = $1 AND deleted_at IS NULL`,
		w.target(src, "channel:"+chSupport))
	if tags != "done,open" {
		t.Errorf("forum tags %q, want done,open", tags)
	}
	if n := w.count(`SELECT count(*) FROM thread_tags WHERE thread_id = $1`, w.target(src, "thread:"+thLogin)); n != 1 {
		t.Errorf("the forum post has %d tags, want its tag open", n)
	}
	if row := channelRow(t, res.stdout, "#support"); !slices.ContainsFunc(row.Notes, func(n string) bool { return strings.Contains(n, "name") }) {
		t.Errorf("the row of the forum does not note the changed name: %v", row.Notes)
	}
	w.checkComplete(src)
}

// imageEmbed is an embed without title, link, description, author, field and footer: it converts
// to no text (22 6.3: images in embeds are not loaded).
var imageEmbed = []any{map[string]any{"type": "rich", "image": map[string]any{"url": "https://cdn.example.com/embed/picture.png"}}}

// TestApplySkipsMessagesWithoutContent (22 6.1): a message whose only part is an embed that
// converts to no text, and a forward whose snapshot converts to none, are counted as skipped
// (empty) in the dry run and in the import, and the messages after them are taken over; a reply to
// such a message carries the quote line instead of the reply. Before, the server refused the empty
// content_md, the batch was lost and every run ended with exit 1 at the same place.
func TestApplySkipsMessagesWithoutContent(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	f := w.fixture()
	at := func(minute string) string { return "2026-03-02T09:" + minute + ":00.000000+00:00" }
	f.Messages[chNews] = append(f.Messages[chNews],
		message(msgNewsA, chNews, 0, robin, "", at("01"), map[string]any{"embeds": imageEmbed}),
		message(msgNewsB, chNews, 0, alex, "", at("02"), map[string]any{
			"message_reference": map[string]any{"type": 1, "message_id": "13009001", "channel_id": chRandom, "guild_id": guildID},
			"message_snapshots": []any{map[string]any{"message": map[string]any{"type": 0, "content": "", "timestamp": at("00"),
				"attachments": []any{}, "embeds": imageEmbed}}}}),
		message(msgNewsC, chNews, 19, alex, "Nice picture.", at("03"), replyTo(chNews, msgNewsA)),
		message(msgNewsD, chNews, 0, robin, "After the pictures.", at("04"), nil))
	fake := discordtest.New(t, f)
	session := w.login()

	dry := w.invoke(jsonRun(session, fake), "--guild", guildID, "--map", w.mapFile(mapping()), "--report", "json")
	if dry.code != importtool.ExitOK {
		t.Fatalf("dry run: exit %d\n%s", dry.code, dry.stderr)
	}
	src := w.source()
	res := w.mustApply(jsonRun(session, fake), importtool.ExitOK)

	for name, out := range map[string]string{"dry run": dry.stdout, "apply": res.stdout} {
		if row := channelRow(t, out, "#news"); row.Empty != 2 || row.Taken != 3 {
			t.Errorf("%s: news has %d messages skipped as empty and %d taken over, want 2 and 3", name, row.Empty, row.Taken)
		}
	}
	if n := w.count(`SELECT count(*) FROM import_items WHERE source_id = $1 AND origin_key = ANY($2::text[])`, src,
		[]string{"message:" + msgNewsA, "message:" + msgNewsB}); n != 0 {
		t.Errorf("%d items of the messages without content, want none", n)
	}
	var replyTarget, content string
	if err := w.s.AdminDB().QueryRow(w.ctx, `SELECT coalesce(reply_to_message_id::text, ''), content_md FROM messages WHERE id = $1`,
		w.target(src, "message:"+msgNewsC)).Scan(&replyTarget, &content); err != nil {
		t.Fatal(err)
	}
	if replyTarget != "" || !strings.HasPrefix(content, "> @Robin") || !strings.HasSuffix(content, "Nice picture.") {
		t.Errorf("the reply to a skipped message: reply_to %q, content %q; want the quote line and no reply", replyTarget, content)
	}
	if w.target(src, "message:"+msgNewsD) == "" {
		t.Error("the message after the skipped ones was not taken over")
	}
	if c := w.cursor(src); c.Channels[chNews] != msgNewsD {
		t.Errorf("the cursor of news is %q, want the last message %s", c.Channels[chNews], msgNewsD)
	}
	if n := w.count(`SELECT count(*) FROM messages WHERE origin = 'import' AND content_md !~ '\S'
		AND NOT EXISTS (SELECT 1 FROM message_attachments a WHERE a.message_id = messages.id)`); n != 0 {
		t.Errorf("%d imported messages without a visible character and without a file", n)
	}
}

// TestApplyKeepsEmbedFieldsAndForwardedAttachments (22 6.3, 6.5 item 7): an embed whose statement
// lies in its fields, its author and its footer is taken over with them as text, and a forward
// that carries only an attachment is taken over with a line that names the file, its size and the
// jump link; the file is not loaded, and the report counts it.
func TestApplyKeepsEmbedFieldsAndForwardedAttachments(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	f := w.fixture()
	at := func(minute string) string { return "2026-03-02T09:" + minute + ":00.000000+00:00" }
	f.Messages[chNews] = append(f.Messages[chNews],
		message(msgNewsA, chNews, 0, robin, "", at("01"), map[string]any{"embeds": []any{map[string]any{"type": "rich",
			"author": map[string]any{"name": "Build bot"},
			"fields": []any{map[string]any{"name": "Tests", "value": "412 passed", "inline": true}},
			"footer": map[string]any{"text": "runner 3"}}}}),
		message(msgNewsB, chNews, 0, alex, "", at("02"), map[string]any{
			"message_reference": map[string]any{"type": 1, "message_id": "13009001", "channel_id": chRandom, "guild_id": guildID},
			"message_snapshots": []any{map[string]any{"message": map[string]any{"type": 0, "content": "", "timestamp": at("00"), "embeds": []any{},
				"attachments": []any{map[string]any{"id": "13005009", "filename": "checklist.pdf", "size": 2048,
					"url": w.cdn.URL(discordtest.CDNHost, "/attachments/"+chRandom+"/13005009/checklist.pdf", "ex=1&is=2&hm=3")}}}}}}))
	fake := discordtest.New(t, f)
	src := w.source()
	res := w.mustApply(jsonRun(w.login(), fake), importtool.ExitOK)

	if got := w.text(`SELECT content_md FROM messages WHERE id = $1`, w.target(src, "message:"+msgNewsA)); got != "> Build bot\n> Tests: 412 passed\n> runner 3" {
		t.Errorf("the embed arrived as %q, want its author, its field and its footer", got)
	}
	forward := w.target(src, "message:"+msgNewsB)
	want := "Attachment not taken over: checklist.pdf (2.0 KB, of a forwarded message) https://discord.com/channels/" + guildID + "/" + chNews + "/" + msgNewsB
	if got := w.text(`SELECT content_md FROM messages WHERE id = $1`, forward); got != want {
		t.Errorf("the forward arrived as %q, want %q", got, want)
	}
	if n := w.count(`SELECT count(*) FROM message_attachments WHERE message_id = $1`, forward); n != 0 {
		t.Errorf("the forward has %d files, want none", n)
	}
	if row := channelRow(t, res.stdout, "#news"); row.Forwarded != 1 || row.Empty != 0 || row.Taken != 3 {
		t.Errorf("news: %d forwarded attachments, %d empty, %d taken; want 1, 0 and 3", row.Forwarded, row.Empty, row.Taken)
	}
	for _, r := range w.cdn.Requests() {
		if strings.Contains(r.Path, "13005009") {
			t.Errorf("the attachment of the forwarded message was fetched: %s", r.Path)
		}
	}
}

// TestApplyCountsPinsAboveTheLimit (22 6.6): pins beyond pin.max_per_thread of the server do not
// stop the import. The thread keeps the pins up to the limit in Discord order, the further ones
// are counted as not taken over, every message is taken over, and a later run that meets a new
// pinned message in the full thread goes on the same way. Before, the server refused the batch of
// the pin above the limit and every run ended with exit 1 at the same place.
func TestApplyCountsPinsAboveTheLimit(t *testing.T) {
	t.Parallel()
	cfg := harness.RaisedRateLimits()
	cfg["pin.max_per_thread"] = "2"
	w := newWorldWith(t, harness.Options{EgressDenyAll: true, Config: cfg})
	f := w.fixture()
	pinned := map[string]any{"pinned": true}
	at := func(minute string) string { return "2026-03-02T09:" + minute + ":00.000000+00:00" }
	f.Messages[chNews] = append(f.Messages[chNews],
		message(msgNewsA, chNews, 0, robin, "First pin.", at("01"), pinned),
		message(msgNewsB, chNews, 0, alex, "Second pin.", at("02"), pinned),
		message(msgNewsC, chNews, 0, robin, "Third pin.", at("03"), pinned),
		message(msgNewsD, chNews, 0, alex, "Fourth pin.", at("04"), pinned))
	fake := discordtest.New(t, f)
	src := w.source()
	session := w.login()
	res := w.mustApply(jsonRun(session, fake), importtool.ExitOK)

	news := w.text(`SELECT id::text FROM threads WHERE channel_id = $1 AND kind = 'main'`, w.target(src, "channel:"+chNews))
	pins := func() string {
		return w.text(`SELECT coalesce(string_agg(i.origin_key, ',' ORDER BY i.origin_key), '') FROM pins p
			JOIN messages m ON m.id = p.message_id JOIN import_items i ON i.id = m.import_item_id WHERE p.thread_id = $1`, news)
	}
	if got, want := pins(), "message:"+msgNewsA+",message:"+msgNewsB; got != want {
		t.Errorf("pins of news %q, want the first two %q", got, want)
	}
	if row := channelRow(t, res.stdout, "#news"); row.Pins != 2 || row.PinsNotTaken != 2 || row.Taken != 5 ||
		!slices.ContainsFunc(row.Notes, func(n string) bool { return strings.Contains(n, "pin.max_per_thread") }) {
		t.Errorf("news: %d pins taken, %d not taken, %d messages, notes %v; want 2, 2, 5 and a note on pin.max_per_thread",
			row.Pins, row.PinsNotTaken, row.Taken, row.Notes)
	}
	if n := w.count(`SELECT count(*) FROM import_items WHERE source_id = $1 AND item_kind = 'pin' AND origin_key = ANY($2::text[])`, src,
		[]string{"pin:" + msgNewsC, "pin:" + msgNewsD}); n != 0 {
		t.Errorf("%d pin items above the limit, want none", n)
	}
	if c := w.cursor(src); c.Channels[chNews] != msgNewsD {
		t.Errorf("the cursor of news is %q, want the last message %s", c.Channels[chNews], msgNewsD)
	}

	fake.AddMessage(chNews, message(msgNewsE, chNews, 0, robin, "A later pin.", at("05"), pinned))
	res = w.mustApply(jsonRun(session, fake), importtool.ExitOK)
	if row := channelRow(t, res.stdout, "#news"); row.Pins != 0 || row.PinsNotTaken != 1 || row.Taken != 1 {
		t.Errorf("second run, news: %d pins taken, %d not taken, %d messages; want 0, 1 and 1", row.Pins, row.PinsNotTaken, row.Taken)
	}
	if got, want := pins(), "message:"+msgNewsA+",message:"+msgNewsB; got != want {
		t.Errorf("pins of news after the second run %q, want %q", got, want)
	}
	if w.target(src, "message:"+msgNewsE) == "" {
		t.Error("the message of the pin above the limit was not taken over")
	}
}

// TestRefusedBatchNamesRuleAndItem: a batch the server refuses ends the run with exit 1 and a
// message that names the place (the JSON pointer), the rule (the field code) and the item (its
// origin key), never content. Here the server allows a shorter topic than the tool cuts to.
func TestRefusedBatchNamesRuleAndItem(t *testing.T) {
	t.Parallel()
	cfg := harness.RaisedRateLimits()
	cfg["channel.topic_max_chars"] = "5"
	w := newWorldWith(t, harness.Options{EgressDenyAll: true, Config: cfg})
	w.source()
	res := w.mustApply(run{session: w.login()}, importtool.ExitAborted)
	line := ""
	for _, l := range strings.Split(res.stderr, "\n") {
		if strings.HasPrefix(l, "bothy import discord: ") {
			line = l
		}
	}
	for _, want := range []string{"applyImportBatch", "status 400", "validation_failed", "/data/topic", "too_long", "channel:" + chGeneral} {
		if !strings.Contains(line, want) {
			t.Errorf("the error line lacks %q: %s", want, line)
		}
	}
	if strings.Contains(res.stderr, "Everyday talk") {
		t.Error("the output carries the topic")
	}
	if strings.IndexFunc(line, unicode.IsControl) >= 0 {
		t.Errorf("the error line carries a control character: %q", line)
	}
}
