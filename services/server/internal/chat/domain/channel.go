// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"strings"
	"time"
	"unicode/utf8"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Field error codes of the channel and tag rules (docs/DECISIONS.md M1-53/1).
const (
	// FieldNotAllowedForKind marks a member the channel kind does not have, for example
	// forum_default_sort on a text channel.
	FieldNotAllowedForKind = "not_allowed_for_kind"
	// FieldNotASibling marks an after_channel_id or after_tag_id that is no live sibling.
	FieldNotASibling = "not_a_sibling"
	// FieldNotAForum marks a forum tag call on a channel that is not a forum.
	FieldNotAForum = "not_a_forum"
	// FieldNotAllowedWithColor marks clear_color sent together with color.
	FieldNotAllowedWithColor = "not_allowed_with_color"
)

// ChannelKind is channels.kind (03 section 4.5).
type ChannelKind string

// The channel kinds; only text and forum can be created through channel.create (R-DM-10).
const (
	ChannelText  ChannelKind = "text"
	ChannelForum ChannelKind = "forum"
	ChannelDM    ChannelKind = "dm"
)

// OperatorMode is the read form of the operator columns of a channel (OpenAPI OperatorMode).
type OperatorMode string

// The operator modes: inherit takes the default operator of the space, agent names an own agent,
// none has no operator (M1-54 sets them).
const (
	OperatorInherit OperatorMode = "inherit"
	OperatorNone    OperatorMode = "none"
	OperatorAgent   OperatorMode = "agent"
)

// The values of channels.forum_default_sort and channels.default_notification_level.
var (
	forumSorts         = map[string]bool{"last_activity": true, "created": true}
	notificationLevels = map[string]bool{"all": true, "mentions": true, "mute": true}
)

// Limits are the text limits of channels and tags, bound from channel.name_max_chars,
// channel.topic_max_chars and forum_tag.name_max_chars.
type Limits struct {
	ChannelName, ChannelTopic, TagName int
}

// Channel is one row of channels (03 section 4.5) as the chat module reads it; MainThreadID comes
// from threads. Every change of the columns of channel.update, of the order or of a tag bumps
// Version once.
type Channel struct {
	ID, SpaceID ids.ID
	Kind        ChannelKind
	// Name is trimmed; Topic may be empty.
	Name, Topic string
	// SortKey is the fractional index key of the order in the space.
	SortKey string
	// OperatorInherit and OperatorAgentID are the operator columns (written by SetOperator, M1-54).
	OperatorInherit bool
	OperatorAgentID ids.ID
	// RequireMention, ThreadRequireMention, FreeResponse, AutoThread and AllowMentionedAgents are
	// the behaviour columns (written by M2-02); the defaults of 03 for a new channel.
	RequireMention, ThreadRequireMention, FreeResponse, AutoThread, AllowMentionedAgents bool
	// HistoryBackfillLimit is channels.history_backfill_limit.
	HistoryBackfillLimit int
	// ForumDefaultSort and DefaultNotificationLevel are "" for none.
	ForumDefaultSort, DefaultNotificationLevel string
	// ReadOnly is channels.read_only (set by M8-03).
	ReadOnly bool
	// MainThreadID is the main thread of a text channel; zero for a forum.
	MainThreadID ids.ID
	// CreatedBy is the creator; CreatedAt the creation time.
	CreatedBy ids.ID
	CreatedAt time.Time
	// ArchivedAt is zero while the channel is not archived.
	ArchivedAt time.Time
	// Version is the ETag; SettingsVersion the counter of behaviour and prompt (M2-02).
	Version, SettingsVersion int64
}

// NewChannel returns a new channel with the defaults of 03: version and settings version 1,
// operator inherited, the mention defaults, history_backfill_limit backfill. The arguments are
// checked already.
func NewChannel(id, space ids.ID, kind ChannelKind, name, topic, sortKey string, by ids.ID, backfill int) Channel {
	return Channel{
		ID: id, SpaceID: space, Kind: kind, Name: name, Topic: topic, SortKey: sortKey, OperatorInherit: true,
		RequireMention: true, AutoThread: true, HistoryBackfillLimit: backfill, CreatedBy: by, Version: 1, SettingsVersion: 1,
	}
}

// Archived reports an archived channel; the space may be archived on its own.
func (c Channel) Archived() bool { return !c.ArchivedAt.IsZero() }

// Operator is the read form of the operator of a channel (OpenAPI ChannelOperator): zero IDs
// are absent members.
type Operator struct {
	Mode             OperatorMode
	AgentID          ids.ID
	EffectiveAgentID ids.ID
}

// Operator returns the operator of the channel with spaceDefault, the default operator of its
// space (03: effective operator = operator_inherit ? default : operator_agent_id).
func (c Channel) Operator(spaceDefault ids.ID) Operator {
	a := c.Assignment()
	return Operator{Mode: a.Mode, AgentID: a.AgentID, EffectiveAgentID: EffectiveOperator(c.OperatorInherit, c.OperatorAgentID, spaceDefault)}
}

// ChannelChange is the body of updateChannel after the text checks: nil fields stay unchanged.
type ChannelChange struct {
	Name, Topic, ForumDefaultSort, DefaultNotificationLevel *string
	// ClearDefaultNotificationLevel removes the notification default.
	ClearDefaultNotificationLevel bool
}

// Check validates the combination of c on a channel of kind: at least one member,
// forum_default_sort only on a forum, a known sort and level, and not both the level and its
// clearing. Errors: validation_failed with the pointer of the member ("" for an empty body).
func (c ChannelChange) Check(kind ChannelKind) error {
	switch {
	case c.Name == nil && c.Topic == nil && c.ForumDefaultSort == nil && c.DefaultNotificationLevel == nil && !c.ClearDefaultNotificationLevel:
		return fieldError("", FieldRequired, "the body changes nothing")
	case c.DefaultNotificationLevel != nil && c.ClearDefaultNotificationLevel:
		return fieldError("/clear_default_notification_level", FieldNotAllowed, "default_notification_level and clear_default_notification_level exclude each other")
	case c.DefaultNotificationLevel != nil && !notificationLevels[*c.DefaultNotificationLevel]:
		return fieldError("/default_notification_level", FieldInvalid, "the notification level is unknown")
	}
	return checkForumSort(kind, c.ForumDefaultSort)
}

// Update applies c, bumps the version once when a value changed and returns the names of the
// changed fields in the order name, topic, forum_default_sort, default_notification_level.
func (c *Channel) Update(ch ChannelChange) []string {
	var changed []string
	set := func(field string, dst *string, v *string) {
		if v != nil && *v != *dst {
			*dst = *v
			changed = append(changed, field)
		}
	}
	set("name", &c.Name, ch.Name)
	set("topic", &c.Topic, ch.Topic)
	set("forum_default_sort", &c.ForumDefaultSort, ch.ForumDefaultSort)
	if ch.ClearDefaultNotificationLevel {
		empty := ""
		set("default_notification_level", &c.DefaultNotificationLevel, &empty)
	} else {
		set("default_notification_level", &c.DefaultNotificationLevel, ch.DefaultNotificationLevel)
	}
	if len(changed) > 0 {
		c.Version++
	}
	return changed
}

// Move sets the sort key and bumps the version.
func (c *Channel) Move(key string) {
	c.SortKey = key
	c.Version++
}

// Touch bumps the version for a change of the forum tags or of the inherited operator (03
// forum_tags; M1-54: a new space default changes the effective operator). Every tag change raises
// channels.version.
func (c *Channel) Touch() { c.Version++ }

// Archive sets the archive time and bumps the version; there is no way back (05 has no unarchive).
func (c *Channel) Archive(at time.Time) {
	c.ArchivedAt = at
	c.Version++
}

// CheckCreateKind validates kind and forum_default_sort of createChannel: text or forum (a dm
// channel is never created here, R-DM-10), a sort only for a forum and only a known one. Errors:
// validation_failed on /kind or /forum_default_sort.
func CheckCreateKind(kind ChannelKind, sort *string) error {
	if kind != ChannelText && kind != ChannelForum {
		return fieldError("/kind", FieldInvalid, "the kind must be text or forum")
	}
	return checkForumSort(kind, sort)
}

// checkForumSort allows a known forum_default_sort only on a forum.
func checkForumSort(kind ChannelKind, sort *string) error {
	switch {
	case sort == nil:
		return nil
	case kind != ChannelForum:
		return fieldError("/forum_default_sort", FieldNotAllowedForKind, "forum_default_sort applies to forums only")
	case !forumSorts[*sort]:
		return fieldError("/forum_default_sort", FieldInvalid, "the forum sort is unknown")
	}
	return nil
}

// ChannelName trims name and checks it like a space name: not blank, at most maxChars Unicode
// characters (channel.name_max_chars), no control or bidi characters (SEC-IN-4). Errors:
// validation_failed on /name; the error never repeats the input.
func ChannelName(name string, maxChars int) (string, error) {
	return trimmedName("/name", name, maxChars)
}

// ChannelTopic checks topic: at most maxChars Unicode characters (channel.topic_max_chars), no
// control or bidi characters; empty is allowed and the topic is kept as sent. Errors:
// validation_failed on /topic.
func ChannelTopic(topic string, maxChars int) (string, error) {
	if err := displayText("/topic", "topic", topic); err != nil {
		return "", err
	}
	if utf8.RuneCountInString(topic) > maxChars {
		return "", fieldError("/topic", FieldTooLong, "the topic is too long")
	}
	return topic, nil
}

// TagName trims and checks a forum tag name like a channel name with forum_tag.name_max_chars.
// Errors: validation_failed on /name.
func TagName(name string, maxChars int) (string, error) {
	return trimmedName("/name", name, maxChars)
}

// trimmedName is the shared rule of channel and tag names.
func trimmedName(pointer, name string, maxChars int) (string, error) {
	if err := displayText(pointer, "name", name); err != nil {
		return "", err
	}
	name = strings.TrimSpace(name)
	switch {
	case name == "":
		return "", fieldError(pointer, FieldRequired, "the name is required")
	case utf8.RuneCountInString(name) > maxChars:
		return "", fieldError(pointer, FieldTooLong, "the name is too long")
	}
	return name, nil
}
