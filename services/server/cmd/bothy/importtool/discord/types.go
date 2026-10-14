// SPDX-License-Identifier: Apache-2.0

package discord

import (
	"fmt"
	"io"
	"net/url"
	"strconv"
)

// The typed Discord objects hold only the fields the later tickets map (22 5 and 6; [R3],
// [R4]); unknown fields are ignored. Every ID is a snowflake as a digit string (22 7.1): the
// client checks each one after decoding and returns ErrInvalidID naming the JSON field. An
// optional ID that Discord sends as null is the empty string.

// User is a Discord user, also a bot or the author of a webhook message ([R1]).
type User struct {
	// ID is the user ID.
	ID string `json:"id"`
	// Username is the unique user name.
	Username string `json:"username"`
	// GlobalName is the display name, empty when unset.
	GlobalName string `json:"global_name"`
	// Bot is set for bot accounts.
	Bot bool `json:"bot"`
}

// Role is a role of a guild.
type Role struct {
	// ID is the role ID; the role @everyone has the ID of the guild.
	ID string `json:"id"`
	// Name is the role name.
	Name string `json:"name"`
}

// Guild is a Discord server ([R2]).
type Guild struct {
	// ID is the guild ID.
	ID string `json:"id"`
	// Name is the server name for the report.
	Name string `json:"name"`
	// Roles are the roles of the guild.
	Roles []Role `json:"roles"`
}

// Overwrite stands for one permission overwrite; the mapping only counts them.
type Overwrite struct{}

// Tag is a tag a forum offers.
type Tag struct {
	// ID is the tag ID, unique within its forum.
	ID string `json:"id"`
	// Name is the tag name.
	Name string `json:"name"`
	// EmojiName is the unicode emoji of the tag, empty when none.
	EmojiName string `json:"emoji_name"`
}

// ThreadMetadata is the thread part of a channel object.
type ThreadMetadata struct {
	// Archived is a pointer so that a missing field and false differ (22 5.1.1).
	Archived *bool `json:"archived"`
	// ArchiveTimestamp is the ISO 8601 time of the last archive change; the archived lists page
	// by it.
	ArchiveTimestamp string `json:"archive_timestamp"`
	// Locked is set when only moderators may unarchive the thread.
	Locked bool `json:"locked"`
}

// Channel is a category, a channel or a thread ([R3]).
type Channel struct {
	// ID is the channel ID.
	ID string `json:"id"`
	// Type is the Discord channel type (0 text, 2 voice, 4 category, 5 announcement, 11 and 12
	// threads, 15 forum, ...).
	Type int `json:"type"`
	// GuildID is the guild, empty when Discord omits it.
	GuildID string `json:"guild_id"`
	// ParentID is the category of a channel or the channel of a thread, empty for none.
	ParentID string `json:"parent_id"`
	// Name is the channel or thread name.
	Name string `json:"name"`
	// Position is the sort position.
	Position int `json:"position"`
	// Topic is the channel topic.
	Topic string `json:"topic"`
	// LastMessageID is the newest message, empty for none.
	LastMessageID string `json:"last_message_id"`
	// PermissionOverwrites has one entry per overwrite; only the count matters.
	PermissionOverwrites []Overwrite `json:"permission_overwrites"`
	// AvailableTags are the tags of a forum.
	AvailableTags []Tag `json:"available_tags"`
	// AppliedTags are the tag IDs of a forum post.
	AppliedTags []string `json:"applied_tags"`
	// ThreadMetadata is set for threads.
	ThreadMetadata *ThreadMetadata `json:"thread_metadata"`
	// OwnerID is the creator of a thread.
	OwnerID string `json:"owner_id"`
}

// SignedURL is a URL that can carry a signature in its query (attachment links of the CDN,
// [R5]). String, GoString and Format print scheme, host and path only, so a log line, an error
// or %+v of a result never shows the signature (REUSE L22); Raw returns the full URL for the
// download (M8-02).
type SignedURL string

// Raw returns the full URL with its query.
func (u SignedURL) Raw() string { return string(u) }

// String returns scheme, host and path only, or "" when the URL does not parse.
func (u SignedURL) String() string {
	p, err := url.Parse(string(u))
	if err != nil {
		return ""
	}
	return (&url.URL{Scheme: p.Scheme, Host: p.Host, Path: p.Path}).String()
}

// GoString is String in quotes, also for %#v.
func (u SignedURL) GoString() string { return strconv.Quote(u.String()) }

// Format prints String for every verb.
func (u SignedURL) Format(f fmt.State, _ rune) { _, _ = io.WriteString(f, u.String()) }

// Attachment is a file of a message.
type Attachment struct {
	// ID is the attachment ID.
	ID string `json:"id"`
	// Filename is the original file name.
	Filename string `json:"filename"`
	// Size is the size in bytes.
	Size int64 `json:"size"`
	// URL is the signed CDN link.
	URL SignedURL `json:"url"`
	// ContentType is the media type, empty when unknown.
	ContentType string `json:"content_type"`
}

// Embed is a rich embed of a message.
type Embed struct {
	// Type is the embed type ("rich", "link", ...).
	Type string `json:"type"`
	// Title is the title.
	Title string `json:"title"`
	// Description is the text.
	Description string `json:"description"`
	// URL is the link of the title.
	URL SignedURL `json:"url"`
}

// Sticker is a sticker item of a message.
type Sticker struct {
	// ID is the sticker ID.
	ID string `json:"id"`
	// Name is the sticker name.
	Name string `json:"name"`
	// FormatType is the Discord format type.
	FormatType int `json:"format_type"`
}

// PollMedia is the text of a poll question or answer.
type PollMedia struct {
	// Text is the text.
	Text string `json:"text"`
}

// PollAnswer is one answer of a poll.
type PollAnswer struct {
	// AnswerID is the number of the answer within the poll.
	AnswerID int `json:"answer_id"`
	// PollMedia is the answer text.
	PollMedia PollMedia `json:"poll_media"`
}

// Poll is the poll of a message.
type Poll struct {
	// Question is the question.
	Question PollMedia `json:"question"`
	// Answers are the answers.
	Answers []PollAnswer `json:"answers"`
}

// MessageRef is the reference of a reply (type 0), a forward (type 1) or a system message.
type MessageRef struct {
	// Type is 0 for a reply and 1 for a forward.
	Type int `json:"type"`
	// MessageID is the referenced message, empty when unknown.
	MessageID string `json:"message_id"`
	// ChannelID is the channel of the referenced message.
	ChannelID string `json:"channel_id"`
	// GuildID is the guild of the referenced message.
	GuildID string `json:"guild_id"`
}

// SnapshotMessage is the copied part of a forwarded message.
type SnapshotMessage struct {
	// Type is the message type of the original.
	Type int `json:"type"`
	// Content is the text of the original.
	Content string `json:"content"`
	// Timestamp is the time of the original.
	Timestamp string `json:"timestamp"`
	// EditedTimestamp is the edit time of the original, empty when never edited.
	EditedTimestamp string `json:"edited_timestamp"`
	// Attachments are the files of the original.
	Attachments []Attachment `json:"attachments"`
	// Embeds are the embeds of the original.
	Embeds []Embed `json:"embeds"`
	// StickerItems are the stickers of the original.
	StickerItems []Sticker `json:"sticker_items"`
}

// Snapshot is one entry of message_snapshots of a forward.
type Snapshot struct {
	// Message is the copied message.
	Message SnapshotMessage `json:"message"`
}

// Emoji is the emoji of a reaction; ID is empty for a unicode emoji.
type Emoji struct {
	// ID is the ID of a custom emoji.
	ID string `json:"id"`
	// Name is the unicode emoji or the custom emoji name.
	Name string `json:"name"`
}

// Reaction is the count of one emoji on a message.
type Reaction struct {
	// Count is the number of reactions.
	Count int `json:"count"`
	// Emoji is the emoji.
	Emoji Emoji `json:"emoji"`
}

// Message is a Discord message ([R4]).
type Message struct {
	// ID is the message ID.
	ID string `json:"id"`
	// ChannelID is the channel or thread.
	ChannelID string `json:"channel_id"`
	// Type is the message type (22 6.1).
	Type int `json:"type"`
	// Author is the author; for a webhook message a pseudo user.
	Author User `json:"author"`
	// Mentions are the mentioned users.
	Mentions []User `json:"mentions"`
	// WebhookID is set for webhook messages.
	WebhookID string `json:"webhook_id"`
	// Content is the text.
	Content string `json:"content"`
	// Timestamp is the creation time (ISO 8601).
	Timestamp string `json:"timestamp"`
	// EditedTimestamp is the edit time, empty when never edited.
	EditedTimestamp string `json:"edited_timestamp"`
	// Pinned is set for pinned messages.
	Pinned bool `json:"pinned"`
	// Attachments are the files.
	Attachments []Attachment `json:"attachments"`
	// Embeds are the embeds.
	Embeds []Embed `json:"embeds"`
	// StickerItems are the stickers.
	StickerItems []Sticker `json:"sticker_items"`
	// Poll is the poll, nil for none.
	Poll *Poll `json:"poll"`
	// MessageReference is the reference of a reply or forward, nil for none.
	MessageReference *MessageRef `json:"message_reference"`
	// MessageSnapshots are the copied messages of a forward.
	MessageSnapshots []Snapshot `json:"message_snapshots"`
	// Reactions are the reaction counts.
	Reactions []Reaction `json:"reactions"`
	// Thread is the thread started from this message, nil for none.
	Thread *Channel `json:"thread"`
}

// idCheck collects the first invalid ID of an object with its JSON field path.
type idCheck struct {
	err error
}

// required checks an ID that must be present.
func (c *idCheck) required(field, id string) {
	if c.err == nil && !validSnowflake(id) {
		c.err = fmt.Errorf("%w: field %s", ErrInvalidID, field)
	}
}

// optional checks an ID that may be empty.
func (c *idCheck) optional(field, id string) {
	if id != "" {
		c.required(field, id)
	}
}

// at joins a field path.
func at(prefix, field string) string {
	if prefix == "" {
		return field
	}
	return prefix + "." + field
}

// indexed names element i of a list field.
func indexed(prefix, field string, i int) string {
	return at(prefix, field) + "[" + strconv.Itoa(i) + "]"
}

func (u *User) check(c *idCheck, p string) { c.required(at(p, "id"), u.ID) }

func (g *Guild) check(c *idCheck, p string) {
	c.required(at(p, "id"), g.ID)
	for i := range g.Roles {
		c.required(at(indexed(p, "roles", i), "id"), g.Roles[i].ID)
	}
}

func (ch *Channel) check(c *idCheck, p string) {
	c.required(at(p, "id"), ch.ID)
	c.optional(at(p, "guild_id"), ch.GuildID)
	c.optional(at(p, "parent_id"), ch.ParentID)
	c.optional(at(p, "last_message_id"), ch.LastMessageID)
	c.optional(at(p, "owner_id"), ch.OwnerID)
	for i := range ch.AvailableTags {
		c.required(at(indexed(p, "available_tags", i), "id"), ch.AvailableTags[i].ID)
	}
	for i, id := range ch.AppliedTags {
		c.required(indexed(p, "applied_tags", i), id)
	}
}

func (m *Message) check(c *idCheck, p string) {
	c.required(at(p, "id"), m.ID)
	c.optional(at(p, "channel_id"), m.ChannelID)
	m.Author.check(c, at(p, "author"))
	for i := range m.Mentions {
		m.Mentions[i].check(c, indexed(p, "mentions", i))
	}
	c.optional(at(p, "webhook_id"), m.WebhookID)
	checkFiles(c, p, m.Attachments, m.StickerItems)
	if r := m.MessageReference; r != nil {
		c.optional(at(p, "message_reference.message_id"), r.MessageID)
		c.optional(at(p, "message_reference.channel_id"), r.ChannelID)
		c.optional(at(p, "message_reference.guild_id"), r.GuildID)
	}
	for i := range m.MessageSnapshots {
		s := &m.MessageSnapshots[i].Message
		checkFiles(c, at(indexed(p, "message_snapshots", i), "message"), s.Attachments, s.StickerItems)
	}
	for i := range m.Reactions {
		c.optional(at(indexed(p, "reactions", i), "emoji.id"), m.Reactions[i].Emoji.ID)
	}
	if m.Thread != nil {
		m.Thread.check(c, at(p, "thread"))
	}
}

// checkFiles checks the IDs of attachments and stickers.
func checkFiles(c *idCheck, p string, files []Attachment, stickers []Sticker) {
	for i := range files {
		c.required(at(indexed(p, "attachments", i), "id"), files[i].ID)
	}
	for i := range stickers {
		c.required(at(indexed(p, "sticker_items", i), "id"), stickers[i].ID)
	}
}

// maxSnowflakeDigits is the length of the largest unsigned 64-bit number; a snowflake is one.
const maxSnowflakeDigits = 20

// validSnowflake reports whether s is a snowflake: 1 to 20 ASCII digits whose value fits into
// an unsigned 64-bit integer (22 7.1).
func validSnowflake(s string) bool {
	if s == "" || len(s) > maxSnowflakeDigits {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	_, err := strconv.ParseUint(s, 10, 64)
	return err == nil
}

// compareSnowflakes orders two valid snowflakes numerically.
func compareSnowflakes(a, b string) int {
	x, _ := strconv.ParseUint(a, 10, 64)
	y, _ := strconv.ParseUint(b, 10, 64)
	switch {
	case x < y:
		return -1
	case x > y:
		return 1
	}
	return 0
}
