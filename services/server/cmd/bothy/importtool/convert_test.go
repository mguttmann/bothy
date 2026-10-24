// SPDX-License-Identifier: Apache-2.0

package importtool

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/cmd/bothy/importtool/discord"
)

// convAlex, convSam and convRobin are the white-label authors and mentioned users of the
// conversion tests; Robin has no global_name so the username fallback is covered.
var (
	convAlex  = discord.User{ID: "100000001", Username: "alex", GlobalName: "Alex"}
	convSam   = discord.User{ID: "100000002", Username: "sam", GlobalName: "Sam"}
	convRobin = discord.User{ID: "100000003", Username: "robin"}
)

// convEnv returns the lookups of the conversion tests: one role, one channel posted in this
// source with an in-app path, one channel known by name only and one with "[" and "]" in its name.
func convEnv() ConvertEnv {
	return ConvertEnv{
		Roles: map[string]string{"200000001": "Maintainers"},
		Channel: func(id string) (string, string) {
			switch id {
			case "300000001":
				return "general", "/c/0b9a4c1e-5d2f-4e7a-9c3b-1f2e3d4c5b6a"
			case "300000002":
				return "off-topic", ""
			case "300000003":
				return "plans [draft]", "/c/7d1e2f3a-4b5c-4d6e-8f70-8192a3b4c5d6"
			}
			return "", ""
		},
	}
}

// convMsg builds a type 0 message by Alex with the given content that mentions Sam and Robin.
func convMsg(content string) discord.Message {
	return discord.Message{
		ID: "400000001", ChannelID: "300000001", Type: 0,
		Author: convAlex, Mentions: []discord.User{convSam, convRobin}, Content: content,
	}
}

// TestConvertContentTable holds one row per line of the content table of 22 6.3.
func TestConvertContentTable(t *testing.T) {
	poll := &discord.Poll{
		Question: discord.PollMedia{Text: "Lunch on Friday?"},
		Answers: []discord.PollAnswer{
			{AnswerID: 1, PollMedia: discord.PollMedia{Text: "Pizza"}},
			{AnswerID: 2, PollMedia: discord.PollMedia{Text: "Salad"}},
		},
	}
	tests := []struct {
		name string
		msg  discord.Message
		want string
	}{
		{"user mention", convMsg("hi <@100000002>"), "hi @Sam"},
		{"user mention nick form", convMsg("hi <@!100000002>"), "hi @Sam"},
		{"user mention username fallback", convMsg("hi <@100000003>"), "hi @robin"},
		{"user mention unknown", convMsg("hi <@100000009>"), "hi @unknown-user"},
		{"role mention", convMsg("ping <@&200000001>"), "ping @Maintainers"},
		{"role mention unknown", convMsg("ping <@&200000009>"), "ping @unknown-role"},
		{"channel with path", convMsg("see <#300000001>"),
			"see [#general](/c/0b9a4c1e-5d2f-4e7a-9c3b-1f2e3d4c5b6a)"},
		{"channel without path", convMsg("see <#300000002>"), "see #off-topic"},
		{"channel unknown", convMsg("see <#300000009>"), "see #unknown-channel"},
		{"channel link text escaped", convMsg("see <#300000003>"),
			`see [#plans \[draft\]](/c/7d1e2f3a-4b5c-4d6e-8f70-8192a3b4c5d6)`},
		{"static emoji", convMsg("nice <:party_parrot:500000001>"), "nice :party_parrot:"},
		{"animated emoji", convMsg("nice <a:dance:500000002>"), "nice :dance:"},
		{"timestamp without style", convMsg("at <t:1700000000>"), "at 2023-11-14T22:13:20Z"},
		{"timestamp with style", convMsg("at <t:1700000000:R>"), "at 2023-11-14T22:13:20Z"},
		{"malformed tokens stay", convMsg("<@abc> <#x1> <t:soon> <:e:12a>"), "<@abc> <#x1> <t:soon> <:e:12a>"},
		{"small text first line", convMsg("-# quiet\nloud"), "quiet\nloud"},
		{"small text later line", convMsg("loud\n-# quiet\n-#no space"), "loud\nquiet\n-#no space"},
		{"markdown kept", convMsg("**bold** _it_ `code`"), "**bold** _it_ `code`"},
		{"embed", convEmbedMsg(), "look\n\n> [Release \\[1.0\\]](https://example.com/notes)\n> line one @Sam\n>\n> line two"},
		{"embed title only", withEmbeds(convMsg(""), discord.Embed{Title: "Notice"}), "> Notice"},
		{"sticker", withStickers(convMsg(""), discord.Sticker{ID: "600000001", Name: "wave"}), "[Sticker: wave]"},
		{"poll", withPoll(convMsg(""), poll), "Poll: Lunch on Friday?\n- Pizza\n- Salad"},
		{"forward snapshot", convForwardMsg(), "fwd\n\n> from @Sam\n>\n> [Sticker: wave]"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := ConvertContent(tc.msg, convEnv()); got != tc.want {
				t.Fatalf("ConvertContent:\n got %q\nwant %q", got, tc.want)
			}
		})
	}
}

// TestConvertContentNilChannelLookup checks that a missing Channel lookup gives the fallback.
func TestConvertContentNilChannelLookup(t *testing.T) {
	if got := ConvertContent(convMsg("<#300000001>"), ConvertEnv{}); got != "#unknown-channel" {
		t.Fatalf("got %q", got)
	}
}

// convEmbedMsg is a message with text and one rich embed whose description has two paragraphs
// and a mention.
func convEmbedMsg() discord.Message {
	return withEmbeds(convMsg("look"), discord.Embed{
		Type: "rich", Title: "Release [1.0]", URL: "https://example.com/notes",
		Description: "line one <@100000002>\n\nline two",
	})
}

// convForwardMsg is a forward whose snapshot has a mention and a sticker.
func convForwardMsg() discord.Message {
	m := convMsg("fwd")
	m.MessageReference = &discord.MessageRef{Type: 1, MessageID: "400000009"}
	m.MessageSnapshots = []discord.Snapshot{{Message: discord.SnapshotMessage{
		Content:      "from <@100000002>",
		StickerItems: []discord.Sticker{{ID: "600000001", Name: "wave"}},
	}}}
	return m
}

// withEmbeds sets the embeds of m.
func withEmbeds(m discord.Message, e ...discord.Embed) discord.Message {
	m.Embeds = e
	return m
}

// withStickers sets the stickers of m.
func withStickers(m discord.Message, s ...discord.Sticker) discord.Message {
	m.StickerItems = s
	return m
}

// withPoll sets the poll of m.
func withPoll(m discord.Message, p *discord.Poll) discord.Message {
	m.Poll = p
	return m
}

// TestTakeOver holds the type and empty rules of 22 6.1.
func TestTakeOver(t *testing.T) {
	typed := func(typ int) discord.Message {
		m := convMsg("text")
		m.Type = typ
		return m
	}
	tests := []struct {
		name   string
		msg    discord.Message
		take   bool
		reason string
	}{
		{"default", typed(0), true, ""},
		{"reply", typed(19), true, ""},
		{"chat input command", typed(20), true, ""},
		{"thread starter", typed(21), true, ""},
		{"context menu command", typed(23), true, ""},
		{"pin notice", typed(6), false, SkipType},
		{"member join", typed(7), false, SkipType},
		{"thread created", typed(18), false, SkipType},
		{"empty", convMsg(""), false, SkipEmpty},
		{"whitespace only", convMsg(" \n "), false, SkipEmpty},
		{"attachment only", convWith(func(m *discord.Message) {
			m.Attachments = []discord.Attachment{{ID: "700000001", Filename: "plan.pdf", Size: 10}}
		}), true, ""},
		{"sticker only", withStickers(convMsg(""), discord.Sticker{ID: "600000001", Name: "wave"}), true, ""},
		{"poll only", withPoll(convMsg(""), &discord.Poll{Question: discord.PollMedia{Text: "Q"}}), true, ""},
		{"embed only", withEmbeds(convMsg(""), discord.Embed{Title: "Notice"}), true, ""},
		{"snapshot only", convWith(func(m *discord.Message) {
			m.MessageSnapshots = []discord.Snapshot{{Message: discord.SnapshotMessage{Content: "x"}}}
		}), true, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			take, reason := TakeOver(tc.msg)
			if take != tc.take || reason != tc.reason {
				t.Fatalf("TakeOver = (%v, %q), want (%v, %q)", take, reason, tc.take, tc.reason)
			}
		})
	}
}

// convWith returns an empty type 0 message changed by f.
func convWith(f func(*discord.Message)) discord.Message {
	m := convMsg("")
	f(&m)
	return m
}

// TestDisplayName checks the global_name first rule and the username fallback.
func TestDisplayName(t *testing.T) {
	if got := DisplayName(convAlex); got != "Alex" {
		t.Fatalf("got %q", got)
	}
	if got := DisplayName(convRobin); got != "robin" {
		t.Fatalf("got %q", got)
	}
}

// TestReplyQuote checks the flattening and the cut to import.reply_quote_max_chars.
func TestReplyQuote(t *testing.T) {
	tests := []struct {
		name, text string
		max        int
		want       string
	}{
		{"no cut", "see you\nat noon", 40, "> @Sam: see you at noon"},
		{"exact length", "abcde", 5, "> @Sam: abcde"},
		{"cut", "abcdefgh", 5, "> @Sam: abcde..."},
		{"cut counts runes", "αβγδεζ", 3, "> @Sam: αβγ..."},
		{"crlf", "a\r\nb", 40, "> @Sam: a b"},
		{"empty", "", 40, "> @Sam"},
		{"blank", " \n ", 40, "> @Sam"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := ReplyQuote("Sam", tc.text, tc.max); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// TestAttachmentLine checks the line of an attachment that was not taken over (22 6.5 item 6).
func TestAttachmentLine(t *testing.T) {
	const jump = "https://discord.com/channels/800000001/300000001/400000001"
	tests := []struct {
		name   string
		size   int64
		reason string
		want   string
	}{
		{"one byte", 1, "failed", "Attachment not taken over: plan.pdf (1 byte, failed) " + jump},
		{"bytes", 512, "failed", "Attachment not taken over: plan.pdf (512 bytes, failed) " + jump},
		{"kilobytes", 1536, "failed", "Attachment not taken over: plan.pdf (1.5 KB, failed) " + jump},
		{"megabytes", 52_428_800, "too large", "Attachment not taken over: plan.pdf (50.0 MB, too large) " + jump},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			a := discord.Attachment{ID: "700000001", Filename: "plan.pdf", Size: tc.size}
			got := AttachmentLine(a, tc.reason, "800000001", "300000001", "400000001")
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// TestContentSHA256 checks the exact form, the stability under attachment order and that
// different content or attachments give a different hash.
func TestContentSHA256(t *testing.T) {
	a := ContentSHA256("hi", []string{"2", "1"})
	if b := ContentSHA256("hi", []string{"1", "2"}); a != b {
		t.Fatalf("order changes the hash: %s != %s", a, b)
	}
	if a != convSHA("hi\n1,2") {
		t.Fatalf("form: got %s", a)
	}
	if a == ContentSHA256("ho", []string{"1", "2"}) {
		t.Fatal("different content, same hash")
	}
	if a == ContentSHA256("hi", []string{"1", "3"}) {
		t.Fatal("different attachments, same hash")
	}
	if ContentSHA256("hi", nil) != convSHA("hi\n") {
		t.Fatal("form without attachments")
	}
	ids := []string{"2", "1"}
	ContentSHA256("hi", ids)
	if ids[0] != "2" {
		t.Fatal("the input slice was sorted in place")
	}
}

// TestSnowflakeTime checks the creation time of the example snowflake of the Discord reference.
func TestSnowflakeTime(t *testing.T) {
	want := time.Date(2016, 4, 30, 11, 18, 25, 796_000_000, time.UTC)
	// The reference snowflake: 41944705796 ms after the Discord epoch, worker, process and
	// increment bits 131079. Built from its parts, because the white-label rule of
	// tools/denylist/generic.rules flags 17 to 20 digit literals as real snowflakes.
	ref := strconv.FormatUint(41944705796<<22|131079, 10)
	if got := SnowflakeTime(ref); !got.Equal(want) || got.Location() != time.UTC {
		t.Fatalf("got %v, want %v", got, want)
	}
	if got := SnowflakeTime("not-a-number"); !got.IsZero() {
		t.Fatalf("invalid ID: got %v", got)
	}
}

// TestParseDiscordTime checks the fraction form, the offset conversion to UTC and an error.
func TestParseDiscordTime(t *testing.T) {
	got, err := ParseDiscordTime("2023-11-14T23:13:20.123456+01:00")
	if err != nil {
		t.Fatal(err)
	}
	want := time.Date(2023, 11, 14, 22, 13, 20, 123_456_000, time.UTC)
	if !got.Equal(want) || got.Location() != time.UTC {
		t.Fatalf("got %v, want %v", got, want)
	}
	if _, err := ParseDiscordTime("yesterday"); err == nil {
		t.Fatal("no error for an invalid time")
	}
}

// convSHA is the hex SHA-256 of s, computed independently of ContentSHA256.
func convSHA(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}
