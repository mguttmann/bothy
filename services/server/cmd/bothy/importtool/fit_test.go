// SPDX-License-Identifier: Apache-2.0

package importtool

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/OWNER/bothy/services/server/api/restclient"
	"github.com/OWNER/bothy/services/server/cmd/bothy/importtool/bothyapi/bothytest"
	"github.com/OWNER/bothy/services/server/cmd/bothy/importtool/discord"
	"github.com/OWNER/bothy/services/server/cmd/bothy/importtool/discord/discordtest"
	"github.com/OWNER/bothy/services/server/internal/kernel/textcheck"
)

// The characters of the display text rule of the server (SEC-IN-4), built from their code points
// so the source holds no invisible character.
var (
	bidiOverride  = string(rune(0x202E))
	bidiMark      = string(rune(0x200F))
	c1Control     = string(rune(0x0085))
	zeroWidthJoin = string(rune(0x200D))
)

// TestDisplayText: a text that fits is returned as it is; control characters become spaces, bidi
// controls and invalid bytes are dropped, the ends are trimmed, and every result passes the check
// of the server (textcheck.DisplayText).
func TestDisplayText(t *testing.T) {
	tests := []struct {
		name, in, want string
		changed        bool
	}{
		{"fits", "Release planning", "Release planning", false},
		{"fits with spaces at the ends", " kept as it is ", " kept as it is ", false},
		{"empty", "", "", false},
		{"emoji sequence kept", "team " + "\U0001F469" + zeroWidthJoin + "\U0001F4BB", "team " + "\U0001F469" + zeroWidthJoin + "\U0001F4BB", false},
		{"line break and tab", "first\nsecond\tthird", "first second third", true},
		{"carriage return", "first\r\nsecond", "first  second", true},
		{"control at the end", "Release planning\a", "Release planning", true},
		{"delete and C1", "a\x7fb" + c1Control + "c", "a b c", true},
		{"bidi override", "Release" + bidiOverride + " planning", "Release planning", true},
		{"bidi mark at the end", "open" + bidiMark, "open", true},
		{"invalid bytes", "caf\xff\xfe", "caf", true},
		{"only controls", "\n" + bidiMark + "\t", "", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, changed := displayText(tc.in)
			if got != tc.want || changed != tc.changed {
				t.Errorf("displayText(%q) = %q, %v; want %q, %v", tc.in, got, changed, tc.want, tc.changed)
			}
			if err := textcheck.DisplayText(got); err != nil {
				t.Errorf("the result %q breaks the rule of the server: %v", got, err)
			}
		})
	}
}

// TestFitTopic: a topic is trimmed as before, fitted like a display text and cut to TopicMaxChars
// characters (not bytes); only a fit or a cut counts as changed.
func TestFitTopic(t *testing.T) {
	long := strings.Repeat("\u00e4", TopicMaxChars+50)
	tests := []struct {
		name, in, want string
		changed        bool
	}{
		{"fits", "Everyday talk", "Everyday talk", false},
		{"trimmed only", "  Everyday talk \n", "Everyday talk", false},
		{"blank", " \n\t ", "", false},
		{"line breaks", "Rules:\n1. be kind\n2. no spam", "Rules: 1. be kind 2. no spam", true},
		{"at the limit", long[:len("\u00e4")*TopicMaxChars], long[:len("\u00e4")*TopicMaxChars], false},
		{"above the limit", long, long[:len("\u00e4")*TopicMaxChars], true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, changed := fitTopic(tc.in)
			if got != tc.want || changed != tc.changed {
				t.Errorf("fitTopic = %d characters %q..., %v; want %d characters, %v", utf8.RuneCountInString(got),
					got[:min(len(got), 40)], changed, utf8.RuneCountInString(tc.want), tc.changed)
			}
			if utf8.RuneCountInString(got) > TopicMaxChars || textcheck.DisplayText(got) != nil {
				t.Errorf("the topic does not fit the server: %d characters", utf8.RuneCountInString(got))
			}
		})
	}
}

// TestFakeTopicLimitEqualsTheTool: the fake server accepts exactly the topic length the tool cuts
// to, so the unit tests see the limit the real server has by default (a test in test/importtool
// holds TopicMaxChars equal to that default).
func TestFakeTopicLimitEqualsTheTool(t *testing.T) {
	if bothytest.TopicMaxChars != TopicMaxChars {
		t.Errorf("the fake accepts topics of %d characters, the tool cuts to %d", bothytest.TopicMaxChars, TopicMaxChars)
	}
}

// TestHollow: a message of a taken type is hollow when its converted text has no visible
// character and it has no attachment, its own or one of a forward.
func TestHollow(t *testing.T) {
	image := discord.Embed{Type: "rich"}
	file := []discord.Attachment{{ID: "700000001", Filename: "plan.pdf", Size: 10}}
	tests := []struct {
		name string
		msg  discord.Message
		want bool
	}{
		{"text", convMsg("hello"), false},
		{"embed with a title", withEmbeds(convMsg(""), discord.Embed{Title: "Notice"}), false},
		{"embed with a field", withEmbeds(convMsg(""), discord.Embed{Fields: []discord.EmbedField{{Name: "State", Value: "done"}}}), false},
		{"embed with only an image", withEmbeds(convMsg(""), image), true},
		{"white space and an image embed", withEmbeds(convMsg(" \n\t"), image), true},
		{"image embed and a file", discord.Message{Embeds: []discord.Embed{image}, Attachments: file}, false},
		{"forward of an image embed", discord.Message{MessageSnapshots: []discord.Snapshot{{Message: discord.SnapshotMessage{
			Embeds: []discord.Embed{image}}}}}, true},
		{"forward of a file", discord.Message{MessageSnapshots: []discord.Snapshot{{Message: discord.SnapshotMessage{Attachments: file}}}}, false},
		{"sticker", withStickers(convMsg(""), discord.Sticker{ID: "600000001", Name: "wave"}), false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if ok, _ := TakeOver(tc.msg); !ok {
				t.Fatalf("the message is not of a taken kind; the case is no case of hollow")
			}
			if got := hollow(tc.msg, ConvertContent(tc.msg, convEnv())); got != tc.want {
				t.Errorf("hollow = %v, want %v (converted %q)", got, tc.want, ConvertContent(tc.msg, convEnv()))
			}
		})
	}
}

// pinBatch is a thread batch of two messages with their pins and one message without.
func pinBatch() restclient.ImportBatch {
	item := func(kind restclient.ImportObjectItemKind, key string) restclient.ImportObject {
		return restclient.ImportObject{ItemKind: kind, OriginKey: key}
	}
	return restclient.ImportBatch{Items: []restclient.ImportObject{
		item(restclient.ImportObjectItemKindMessage, "message:1"), item(restclient.ImportObjectItemKindPin, "pin:1"),
		item(restclient.ImportObjectItemKindMessage, "message:2"), item(restclient.ImportObjectItemKindPin, "pin:2"),
		item(restclient.ImportObjectItemKindMessage, "message:3"), item(restclient.ImportObjectItemKindPin, "pin:3"),
	}}
}

// TestDropPinsAtLimit: only the refusal of a pin item for pin.max_per_thread removes pins, from
// the refused one on; every other error leaves the batch alone.
func TestDropPinsAtLimit(t *testing.T) {
	refusal := func(status int, pointer, code string) error {
		return &APIError{Op: "applyImportBatch", Status: status, Code: "validation_failed", Fields: []FieldProblem{{Pointer: pointer, Code: code}}}
	}
	keys := func(b restclient.ImportBatch) string {
		var out []string
		for _, it := range b.Items {
			out = append(out, it.OriginKey)
		}
		return strings.Join(out, " ")
	}
	const all = "message:1 pin:1 message:2 pin:2 message:3 pin:3"
	tests := []struct {
		name    string
		err     error
		dropped int
		want    string
	}{
		{"the limit at the second pin", refusal(http.StatusBadRequest, "/items/3/data/message_origin_key", "limit_reached"), 2,
			"message:1 pin:1 message:2 message:3"},
		{"the limit at the first pin", refusal(http.StatusBadRequest, "/items/1/data/message_origin_key", "limit_reached"), 3,
			"message:1 message:2 message:3"},
		{"another rule of the pin", refusal(http.StatusBadRequest, "/items/3/data/message_origin_key", "not_imported"), 0, all},
		{"the limit of a message", refusal(http.StatusBadRequest, "/items/2/data/attachment_file_ids", "limit_reached"), 0, all},
		{"an index beyond the batch", refusal(http.StatusBadRequest, "/items/9/data/message_origin_key", "limit_reached"), 0, all},
		{"no item pointer", refusal(http.StatusBadRequest, "/thread_origin_key", "limit_reached"), 0, all},
		{"another status", refusal(http.StatusConflict, "/items/3/data/message_origin_key", "limit_reached"), 0, all},
		{"another error", errors.New("network"), 0, all},
		{"no error", nil, 0, all},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			b := pinBatch()
			if n := dropPinsAtLimit(tc.err, &b); n != tc.dropped || keys(b) != tc.want {
				t.Errorf("dropPinsAtLimit = %d, batch %q; want %d, %q", n, keys(b), tc.dropped, tc.want)
			}
		})
	}
}

// setMembers sets members of the raw fixture object with id in list.
func setMembers(t *testing.T, list []json.RawMessage, id string, members map[string]any) {
	t.Helper()
	for i, raw := range list {
		var o map[string]any
		if err := json.Unmarshal(raw, &o); err != nil {
			t.Fatal(err)
		}
		if o["id"] != id {
			continue
		}
		for k, v := range members {
			o[k] = v
		}
		b, err := json.Marshal(o)
		if err != nil {
			t.Fatal(err)
		}
		list[i] = b
		return
	}
	t.Fatalf("the fixture has no object %s", id)
}

// reportCounts decodes the JSON report in out and returns the counts and notes of the row of the
// Discord channel name.
func reportCounts(t *testing.T, out, name string) (map[string]any, []string) {
	t.Helper()
	var rep struct {
		Channels []struct {
			Channel string         `json:"channel"`
			Counts  map[string]any `json:"counts"`
			Notes   []string       `json:"notes"`
		} `json:"channels"`
	}
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		t.Fatalf("the report is not one JSON object: %v\n%s", err, out)
	}
	for _, c := range rep.Channels {
		if c.Channel == name {
			return c.Counts, c.Notes
		}
	}
	t.Fatalf("the report has no row %s:\n%s", name, out)
	return nil, nil
}

// TestApplyFitsDiscordValues drives a dry run and an --apply run against the fake server, which
// refuses what the real one refuses, with a fixture that holds every such value: a topic with a
// line break, a thread and a tag name with a bidi control, a message that converts to nothing and
// more pins than the thread may hold. Both runs end with exit 0, count the same messages as
// skipped, and the apply run posts everything else.
func TestApplyFitsDiscordValues(t *testing.T) {
	f := discordtest.BasicFixture(t)
	setMembers(t, f.Channels, fxGeneral, map[string]any{"topic": "Everyday talk\nand more"})
	setMembers(t, f.ActiveThreads, fxThread, map[string]any{"name": "Release" + bidiOverride + " planning"})
	setMembers(t, f.Channels, fxSupport, map[string]any{"available_tags": []any{
		map[string]any{"id": "13000201", "name": "open" + bidiMark}, map[string]any{"id": "13000202", "name": "done"}}})
	// The webhook embed of the fixture loses its text, and two more messages of general are pinned.
	setMembers(t, f.Messages[fxGeneral], "13001005", map[string]any{"embeds": []any{map[string]any{"type": "rich",
		"image": map[string]any{"url": "https://cdn.example.com/embed/picture.png"}}}})
	setMembers(t, f.Messages[fxGeneral], "13001004", map[string]any{"pinned": true})
	setMembers(t, f.Messages[fxGeneral], "13001012", map[string]any{"pinned": true})
	e := newTestEnv(t, f)
	e.bothy.SetPinLimit(2)
	mapFile := e.writeMap(fixtureMapping)

	if code := e.run("--guild", fxGuild, "--map", mapFile, "--report", "json"); code != ExitOK {
		t.Fatalf("dry run: exit %d: %s", code, e.stderr.String())
	}
	dry, dryNotes := reportCounts(t, e.stdout.String(), "#general")
	src := e.startSource(fixtureMapping)
	if code := e.run("--guild", fxGuild, "--map", mapFile, "--apply", "--report", "json"); code != ExitOK {
		t.Fatalf("apply: exit %d: %s", code, e.stderr.String())
	}
	applied, notes := reportCounts(t, e.stdout.String(), "#general")

	// Two messages count as empty: the thread starter of the fixture, which has no content, and the
	// webhook message whose embed converts to nothing. The fake stamps its items with the fake
	// clock, so the apply run counts what it posted as present.
	empty := func(counts map[string]any) any { return counts["messages_skipped"].(map[string]any)["empty"] }
	posted := applied["messages_taken"].(float64) + applied["messages_present"].(float64)
	if empty(dry) != float64(2) || empty(applied) != float64(2) || dry["messages_taken"] != posted {
		t.Errorf("dry run: %v empty, %v taken; apply: %v empty, %v posted; want two empty messages and equal counts",
			empty(dry), dry["messages_taken"], empty(applied), posted)
	}
	if applied["pins_taken"] != float64(2) || applied["pins_not_taken"] != float64(1) {
		t.Errorf("pins taken %v, not taken %v; want 2 and 1", applied["pins_taken"], applied["pins_not_taken"])
	}
	if got := e.bothy.Pins(src); len(got) != 2 || got[0] != "message:13001001" || got[1] != "message:13001004" {
		t.Errorf("pins %v, want the first two in Discord order", got)
	}
	for _, key := range []string{"message:13001005", "pin:13001012"} {
		for _, it := range e.bothy.Items(src) {
			if it.OriginKey == key {
				t.Errorf("the item %s was posted", key)
			}
		}
	}
	if got := e.bothy.ThreadMessages(src, "channel:"+fxGeneral); count(got, "message:13001012") != 1 {
		t.Errorf("the message of the pin above the limit is missing: %v", got)
	}
	for name, list := range map[string][]string{"dry run": dryNotes, "apply": notes} {
		for _, want := range []string{noteTopicFitted, noteNameFitted} {
			if count(list, want) != 1 {
				t.Errorf("%s: the row of general lacks the note %q: %v", name, want, list)
			}
		}
	}
	if count(notes, notePinLimit) != 1 || count(dryNotes, notePinLimit) != 0 {
		t.Errorf("the pin limit is noted %d times by the apply run and %d times by the dry run, want 1 and 0",
			count(notes, notePinLimit), count(dryNotes, notePinLimit))
	}
	if _, supportNotes := reportCounts(t, e.stdout.String(), "#support"); count(supportNotes, noteNameFitted) != 1 {
		t.Errorf("the row of the forum lacks the note of the tag name: %v", supportNotes)
	}
}

// TestForwardedAttachmentsGetALine (22 6.5 item 7): the attachments of a forwarded message are
// not loaded; the message carries one line per attachment with its name, size and the jump link,
// and the dry run and the apply run count them.
func TestForwardedAttachmentsGetALine(t *testing.T) {
	f := discordtest.BasicFixture(t)
	setMembers(t, f.Messages[fxGeneral], "13001008", map[string]any{"message_snapshots": []any{map[string]any{"message": map[string]any{
		"type": 0, "content": "", "timestamp": "2026-02-28T17:00:00.000000+00:00", "embeds": []any{},
		"attachments": []any{map[string]any{"id": "13005009", "filename": "checklist.pdf", "size": 2048,
			"url": "https://cdn.example.com/attachments/13000107/13005009/checklist.pdf"}}}}}})
	e := newTestEnv(t, f)
	mapFile := e.writeMap(fixtureMapping)
	if code := e.run("--guild", fxGuild, "--map", mapFile, "--report", "json"); code != ExitOK {
		t.Fatalf("dry run: exit %d: %s", code, e.stderr.String())
	}
	dry, _ := reportCounts(t, e.stdout.String(), "#general")
	src := e.startSource(fixtureMapping)
	if code := e.run("--guild", fxGuild, "--map", mapFile, "--apply", "--report", "json"); code != ExitOK {
		t.Fatalf("apply: exit %d: %s", code, e.stderr.String())
	}
	applied, _ := reportCounts(t, e.stdout.String(), "#general")
	if dry["attachments_forwarded"] != float64(1) || applied["attachments_forwarded"] != float64(1) {
		t.Errorf("attachments_forwarded: dry run %v, apply %v; want 1 each", dry["attachments_forwarded"], applied["attachments_forwarded"])
	}
	want := "Attachment not taken over: checklist.pdf (2.0 KB, of a forwarded message) https://discord.com/channels/" +
		fxGuild + "/" + fxGeneral + "/13001008"
	if got, _ := e.bothy.MessageData(src, "message:13001008")["content_md"].(string); got != want {
		t.Errorf("the forward has the content %q, want %q", got, want)
	}
	for _, file := range e.bothy.Files(src) {
		if file.OriginKey == "file:13005009" {
			t.Error("the attachment of the forwarded message was uploaded")
		}
	}
	for _, r := range e.cdn.Requests() {
		if strings.Contains(r.Path, "13005009") {
			t.Errorf("the attachment of the forwarded message was fetched: %s", r.Path)
		}
	}
	if strings.Contains(e.stdout.String()+e.stderr.String(), "checklist.pdf") {
		t.Error("the output names the file of the attachment")
	}
}
