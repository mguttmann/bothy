// SPDX-License-Identifier: Apache-2.0

package importtool

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/OWNER/bothy/services/server/cmd/bothy/importtool/discord"
)

// The conversion of a Discord message to the Markdown dialect content_format 1 (22 6.1 to 6.3,
// 6.5 item 6). Every function here is pure: no I/O, no clock. The user's Markdown is kept as it
// is, because Discord Markdown is close to the target; only the tokens of the 22 6.3 table
// change. A token whose ID is not a digit string is no token and stays as text.

// Skip reasons of a message (22 6.1, report column "skipped by reason").
const (
	// SkipType is a Discord message type other than 0, 19, 20, 21 and 23.
	SkipType = "type"
	// SkipEmpty is a message without content, attachment, embed, sticker, poll or snapshot.
	SkipEmpty = "empty"
	// SkipStarter is a type 21 message whose starter message is in the main thread.
	SkipStarter = "starter_in_main"
	// SkipAuthor is a message whose author maps to skip; the caller sets it.
	SkipAuthor = "author"
)

// takenTypes are the Discord message types taken over (22 6.1): DEFAULT, REPLY,
// CHAT_INPUT_COMMAND, THREAD_STARTER_MESSAGE and CONTEXT_MENU_COMMAND.
var takenTypes = map[int]bool{0: true, 19: true, 20: true, 21: true, 23: true}

// Fallback texts of tokens whose target is not known.
const (
	// unknownUser replaces a user mention whose user is not in the message's mentions.
	unknownUser = "unknown-user"
	// unknownRole replaces a role mention whose role is not in the guild roles.
	unknownRole = "unknown-role"
	// unknownChannel replaces a channel mention whose channel is not known.
	unknownChannel = "unknown-channel"
)

// smallTextMarker is the Discord small-text prefix of a line; it is removed (22 6.3).
const smallTextMarker = "-# "

// isoUTC is the layout of a converted <t:...> token: ISO 8601 in UTC, whole seconds.
const isoUTC = "2006-01-02T15:04:05Z"

// tokenPattern matches the Discord tokens of the 22 6.3 table. Groups: 1 the mention kind
// ("@", "@!", "@&" or "#") and 2 its ID; 3 the emoji name and 4 its ID; 5 the seconds of a
// timestamp, with an optional known style letter. IDs and seconds must be digits, so a
// malformed token does not match and stays as text.
var tokenPattern = regexp.MustCompile(
	`<(@!?|@&|#)([0-9]+)>|<a?:([A-Za-z0-9_~]+):([0-9]+)>|<t:([0-9]+)(?::[tTdDfFR])?>`)

// ErrDiscordTime is returned by ParseDiscordTime for a value that is no RFC 3339 time. It
// carries no part of the value.
var ErrDiscordTime = errors.New("importtool: invalid discord timestamp")

// ConvertEnv are the lookups of a conversion.
type ConvertEnv struct {
	// Roles maps a guild role ID to its name (Guild.Roles).
	Roles map[string]string
	// Channel returns the Discord name of a guild channel or thread ID ("" when unknown) and the
	// in-app path "/c/<bothy channel uuid>" when the channel is posted in this source ("" otherwise).
	// A nil Channel knows no channel.
	Channel func(id string) (name, path string)
}

// TakeOver reports whether m is taken over by the type and empty rules of 22 6.1 and else the
// reason (SkipType or SkipEmpty). The starter rule (SkipStarter) needs the plan and is the
// caller's, as is SkipAuthor. Content of only white space counts as no content.
func TakeOver(m discord.Message) (bool, string) {
	if !takenTypes[m.Type] {
		return false, SkipType
	}
	empty := strings.TrimSpace(m.Content) == "" && len(m.Attachments) == 0 && len(m.Embeds) == 0 &&
		len(m.StickerItems) == 0 && m.Poll == nil && len(m.MessageSnapshots) == 0
	if empty {
		return false, SkipEmpty
	}
	return true, ""
}

// ConvertContent converts the text parts of m to the Markdown dialect content_format 1 (22 6.3).
// The parts, in this order and joined by a blank line, are: the content (tokens replaced, the
// small-text marker removed per line), one quote block per embed (title as a link when it has
// one, then the description; no image), one line "[Sticker: name]" per sticker, the poll as the
// line "Poll: <question>" followed by one "- <answer>" line per answer, and one quote block per
// forwarded snapshot with its content, embeds and stickers converted by the same rules (mentions
// resolved from m.Mentions, because a snapshot carries none). Empty parts are left out. The
// reply quote and the attachment lines are the caller's.
func ConvertContent(m discord.Message, env ConvertEnv) string {
	parts := convertBody(m.Content, m.Embeds, m.StickerItems, m.Mentions, env)
	if p := pollPart(m.Poll); p != "" {
		parts = append(parts, p)
	}
	for _, s := range m.MessageSnapshots {
		sm := s.Message
		inner := convertBody(sm.Content, sm.Embeds, sm.StickerItems, m.Mentions, env)
		if len(inner) > 0 {
			parts = append(parts, quoteBlock(strings.Join(inner, "\n\n")))
		}
	}
	return strings.Join(parts, "\n\n")
}

// convertBody returns the non-empty parts of a message or snapshot body: the converted content,
// one quote block per embed and one line per sticker.
func convertBody(content string, embeds []discord.Embed, stickers []discord.Sticker,
	mentions []discord.User, env ConvertEnv,
) []string {
	var parts []string
	if text := convertText(content, mentions, env); strings.TrimSpace(text) != "" {
		parts = append(parts, text)
	}
	for _, e := range embeds {
		if p := embedPart(e, mentions, env); p != "" {
			parts = append(parts, p)
		}
	}
	for _, s := range stickers {
		parts = append(parts, "[Sticker: "+s.Name+"]")
	}
	return parts
}

// convertText removes the small-text marker at the start of every line and replaces the tokens
// of the 22 6.3 table. The replacement runs in one pass, so a replaced name is never read as a
// token again.
func convertText(s string, mentions []discord.User, env ConvertEnv) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimPrefix(l, smallTextMarker)
	}
	return tokenPattern.ReplaceAllStringFunc(strings.Join(lines, "\n"), func(tok string) string {
		return replaceToken(tok, mentions, env)
	})
}

// replaceToken returns the text of one matched token; a token it cannot convert (a timestamp
// out of range) stays as it is.
func replaceToken(tok string, mentions []discord.User, env ConvertEnv) string {
	g := tokenPattern.FindStringSubmatch(tok)
	switch {
	case g[1] == "@" || g[1] == "@!":
		return "@" + mentionName(g[2], mentions)
	case g[1] == "@&":
		return "@" + roleName(g[2], env.Roles)
	case g[1] == "#":
		return channelText(g[2], env)
	case g[3] != "":
		return ":" + g[3] + ":"
	default:
		return timestampText(tok, g[5])
	}
}

// mentionName returns the display name of the mentioned user id, or the fallback unknownUser.
func mentionName(id string, mentions []discord.User) string {
	for _, u := range mentions {
		if u.ID == id {
			if n := DisplayName(u); n != "" {
				return n
			}
		}
	}
	return unknownUser
}

// roleName returns the name of the role id, or the fallback unknownRole.
func roleName(id string, roles map[string]string) string {
	if n := roles[id]; n != "" {
		return n
	}
	return unknownRole
}

// channelText returns the link [#name](path) of a channel posted in this source, else #name,
// with the fallback unknownChannel for an unknown channel.
func channelText(id string, env ConvertEnv) string {
	var name, path string
	if env.Channel != nil {
		name, path = env.Channel(id)
	}
	if name == "" {
		return "#" + unknownChannel
	}
	if path == "" {
		return "#" + name
	}
	return "[" + escapeLinkText("#"+name) + "](" + path + ")"
}

// timestampText returns the ISO 8601 UTC text of the seconds of a <t:...> token, or tok when the
// seconds do not fit into a four-digit year.
func timestampText(tok, seconds string) string {
	sec, err := strconv.ParseInt(seconds, 10, 64)
	if err != nil {
		return tok
	}
	ts := time.Unix(sec, 0).UTC()
	if ts.Year() > 9999 {
		return tok
	}
	return ts.Format(isoUTC)
}

// escapeLinkText escapes the characters that would end or open the text of a Markdown link.
func escapeLinkText(s string) string {
	return strings.NewReplacer(`\`, `\\`, "[", `\[`, "]", `\]`).Replace(s)
}

// embedPart returns the quote block of an embed: the title (a link when the embed has a URL),
// or the URL alone, then the converted description. An embed without any of them gives "".
func embedPart(e discord.Embed, mentions []discord.User, env ConvertEnv) string {
	var lines []string
	link := e.URL.Raw()
	switch {
	case e.Title != "" && link != "":
		lines = append(lines, "["+escapeLinkText(e.Title)+"]("+link+")")
	case e.Title != "":
		lines = append(lines, e.Title)
	case link != "":
		lines = append(lines, link)
	}
	if d := convertText(e.Description, mentions, env); strings.TrimSpace(d) != "" {
		lines = append(lines, d)
	}
	if len(lines) == 0 {
		return ""
	}
	return quoteBlock(strings.Join(lines, "\n"))
}

// pollPart returns the line "Poll: <question>" and one "- <answer>" line per answer, or "" for
// no poll.
func pollPart(p *discord.Poll) string {
	if p == nil {
		return ""
	}
	lines := []string{"Poll: " + p.Question.Text}
	for _, a := range p.Answers {
		lines = append(lines, "- "+a.PollMedia.Text)
	}
	return strings.Join(lines, "\n")
}

// quoteBlock prefixes every line of s with "> ", and an empty line with ">", so that blank lines
// inside the block do not end it.
func quoteBlock(s string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		if l == "" {
			lines[i] = ">"
		} else {
			lines[i] = "> " + l
		}
	}
	return strings.Join(lines, "\n")
}

// DisplayName returns global_name or, when empty, username of u.
func DisplayName(u discord.User) string {
	if u.GlobalName != "" {
		return u.GlobalName
	}
	return u.Username
}

// ReplyQuote returns the first line "> @Name: <text>" of a reply whose target is not in the same
// thread: text is the referenced converted content flattened to one line (line breaks to
// spaces, trimmed), cut to max runes (import.reply_quote_max_chars) with "..." appended when
// cut; a max below one keeps the whole text. An empty text gives "> @Name".
func ReplyQuote(name, text string, max int) string {
	flat := strings.TrimSpace(strings.NewReplacer("\r\n", " ", "\n", " ", "\r", " ").Replace(text))
	if flat == "" {
		return "> @" + name
	}
	if max > 0 && utf8.RuneCountInString(flat) > max {
		flat = string([]rune(flat)[:max]) + "..."
	}
	return "> @" + name + ": " + flat
}

// AttachmentLine returns the line of an attachment that was not taken over (22 6.5 item 6):
// "Attachment not taken over: <file name> (<size>, <reason>) <jump link>", with the size in
// bytes, KB or MB (one decimal, base 1024), the reason ("too large" or "failed") and the jump
// link https://discord.com/channels/{guild}/{channel}/{message}. The file name is message
// content: the line goes into the message, never into output.
func AttachmentLine(a discord.Attachment, reason, guildID, channelID, messageID string) string {
	return fmt.Sprintf("Attachment not taken over: %s (%s, %s) https://discord.com/channels/%s/%s/%s",
		a.Filename, humanSize(a.Size), reason, guildID, channelID, messageID)
}

// humanSize returns n as "1 byte", "N bytes", "N.N KB" or "N.N MB" (base 1024).
func humanSize(n int64) string {
	const kib, mib = 1 << 10, 1 << 20
	switch {
	case n == 1:
		return "1 byte"
	case n < kib:
		return strconv.FormatInt(n, 10) + " bytes"
	case n < mib:
		return strconv.FormatFloat(float64(n)/kib, 'f', 1, 64) + " KB"
	default:
		return strconv.FormatFloat(float64(n)/mib, 'f', 1, 64) + " MB"
	}
}

// ContentSHA256 is the hex SHA-256 of the converted Markdown plus the sorted attachment IDs
// (brief note 6), exactly sha256(md + "\n" + strings.Join(sortedIDs, ",")): the IDs sorted as
// strings, so the order of the attachments does not change the hash; without attachments the
// input ends with "\n". The ids slice is not changed.
func ContentSHA256(md string, attachmentIDs []string) string {
	ids := slices.Clone(attachmentIDs)
	slices.Sort(ids)
	sum := sha256.Sum256([]byte(md + "\n" + strings.Join(ids, ",")))
	return hex.EncodeToString(sum[:])
}

// discordEpochMillis is the start of the Discord epoch (2015-01-01T00:00:00Z) in Unix
// milliseconds; a snowflake counts its milliseconds from there.
const discordEpochMillis = 1420070400000

// snowflakeTimeShift is the bit position of the timestamp within a snowflake.
const snowflakeTimeShift = 22

// SnowflakeTime returns the creation time of a Discord snowflake ((id >> 22) + 1420070400000 ms,
// UTC), or the zero time when id is no unsigned 64-bit number.
func SnowflakeTime(id string) time.Time {
	n, err := strconv.ParseUint(id, 10, 64)
	if err != nil {
		return time.Time{}
	}
	return time.UnixMilli(int64(n>>snowflakeTimeShift) + discordEpochMillis).UTC()
}

// ParseDiscordTime parses an ISO 8601 timestamp of Discord (RFC 3339 with fraction) to UTC. The
// error is ErrDiscordTime without the value.
func ParseDiscordTime(s string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return time.Time{}, ErrDiscordTime
	}
	return t.UTC(), nil
}
