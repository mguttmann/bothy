// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"encoding/json"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// SpaceData is the data of a space item (schema ImportSpaceData).
type SpaceData struct {
	Name, SortKey   string
	ExistingSpaceID ids.ID
}

// ChannelData is the data of a channel item (schema ImportChannelData).
type ChannelData struct {
	SpaceOriginKey                     string
	ExistingSpaceID, ExistingChannelID ids.ID
	Name, Kind, Topic, SortKey         string
}

// ForumTagData is the data of a forum_tag item (schema ImportForumTagData); emoji is accepted and
// not stored.
type ForumTagData struct {
	ChannelOriginKey, Name string
}

// Discord author kinds of a user item.
const (
	DiscordUser    = "user"
	DiscordBot     = "bot"
	DiscordWebhook = "webhook"
)

// UserData is the data of a user item (schema ImportUserData).
type UserData struct {
	DiscordKind, Username, GlobalName string
}

// optionalID reads an optional UUID member.
func optionalID(m map[string]json.RawMessage, key, ptr string, p *Problems) ids.ID {
	s, ok := str(m, key, ptr, false, p)
	if !ok {
		return ids.ID{}
	}
	id, err := ids.Parse(s)
	if err != nil {
		p.Add(Pointer(ptr, key), FieldInvalid, "a UUID is required")
		return ids.ID{}
	}
	return id
}

// ParseSpaceData checks the data of a space item at ptr: sort_key required, exactly one of name
// and existing_space_id.
func ParseSpaceData(raw json.RawMessage, ptr string, p *Problems) SpaceData {
	m := object(raw, ptr, []string{memberName, "sort_key", "existing_space_id"}, p)
	if m == nil {
		return SpaceData{}
	}
	var d SpaceData
	d.Name, _ = str(m, memberName, ptr, false, p)
	d.SortKey, _ = str(m, "sort_key", ptr, true, p)
	d.ExistingSpaceID = optionalID(m, "existing_space_id", ptr, p)
	_, hasName := m[memberName]
	_, hasExisting := m["existing_space_id"]
	if hasName == hasExisting {
		p.Add(ptr, FieldInvalid, "exactly one of name and existing_space_id")
	}
	return d
}

// ParseChannelData checks the data of a channel item at ptr: kind text or forum and sort_key
// required, name required unless existing_channel_id is given.
func ParseChannelData(raw json.RawMessage, ptr string, p *Problems) ChannelData {
	m := object(raw, ptr, []string{"space_origin_key", "existing_space_id", memberName, "kind", "topic", "sort_key", "existing_channel_id"}, p)
	if m == nil {
		return ChannelData{}
	}
	var d ChannelData
	d.SpaceOriginKey, _ = str(m, "space_origin_key", ptr, false, p)
	d.ExistingSpaceID = optionalID(m, "existing_space_id", ptr, p)
	d.ExistingChannelID = optionalID(m, "existing_channel_id", ptr, p)
	d.Name, _ = str(m, memberName, ptr, d.ExistingChannelID.IsZero(), p)
	d.Topic, _ = str(m, "topic", ptr, false, p)
	d.SortKey, _ = str(m, "sort_key", ptr, true, p)
	if k, ok := str(m, "kind", ptr, true, p); ok {
		if k != "text" && k != "forum" {
			p.Add(ptr+"/kind", FieldInvalid, "text or forum")
		}
		d.Kind = k
	}
	if d.SpaceOriginKey != "" {
		if _, code := ParseOriginKey(KindSpace, d.SpaceOriginKey); code != "" {
			p.Add(ptr+"/space_origin_key", code, "space:<category_id>")
		}
	}
	return d
}

// ParseForumTagData checks the data of a forum_tag item at ptr.
func ParseForumTagData(raw json.RawMessage, ptr string, p *Problems) ForumTagData {
	m := object(raw, ptr, []string{"channel_origin_key", memberName, "emoji"}, p)
	if m == nil {
		return ForumTagData{}
	}
	var d ForumTagData
	d.ChannelOriginKey, _ = str(m, "channel_origin_key", ptr, true, p)
	d.Name, _ = str(m, memberName, ptr, true, p)
	_, _ = str(m, "emoji", ptr, false, p)
	if d.ChannelOriginKey != "" {
		if _, code := ParseOriginKey(KindChannel, d.ChannelOriginKey); code != "" {
			p.Add(ptr+"/channel_origin_key", code, "channel:<channel_id>")
		}
	}
	return d
}

// ParseUserData checks the data of a user item at ptr.
func ParseUserData(raw json.RawMessage, ptr string, p *Problems) UserData {
	m := object(raw, ptr, []string{"discord_kind", "username", "global_name"}, p)
	if m == nil {
		return UserData{}
	}
	var d UserData
	if k, ok := str(m, "discord_kind", ptr, true, p); ok {
		if k != DiscordUser && k != DiscordBot && k != DiscordWebhook {
			p.Add(ptr+"/discord_kind", FieldInvalid, "user, bot or webhook")
		}
		d.DiscordKind = k
	}
	d.Username, _ = str(m, "username", ptr, true, p)
	d.GlobalName, _ = str(m, "global_name", ptr, false, p)
	return d
}

// UserTarget is the resolved target of a user item: a mapped person or agent (Kind and Handle),
// a placeholder, or Skip.
type UserTarget struct {
	Kind   UserKind
	Handle string
	Skip   bool
}

// ResolveUser returns the target of the Discord user with data d under the mapping (22 5.3): a
// webhook author is always a placeholder per webhook ID; a mapped user follows its entry; an
// unmapped person or bot follows defaults.unmapped_human or defaults.unmapped_bot.
func (m Mapping) ResolveUser(discordID string, d UserData) UserTarget {
	if d.DiscordKind == DiscordWebhook {
		return UserTarget{Kind: UserPlaceholder}
	}
	if e := m.User(discordID); e != nil {
		return UserTarget{Kind: e.Kind, Handle: e.Handle}
	}
	def := m.Defaults.UnmappedHuman
	if d.DiscordKind == DiscordBot {
		def = m.Defaults.UnmappedBot
	}
	if def == DefaultSkip {
		return UserTarget{Skip: true}
	}
	return UserTarget{Kind: UserPlaceholder}
}
