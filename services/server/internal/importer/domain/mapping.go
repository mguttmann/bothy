// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"encoding/json"
	"strings"
	"unicode/utf8"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/textcheck"
)

// Values of the mapping file (22 5.2).
const (
	targetSkip        = "skip"
	targetPlaceholder = "placeholder"
	operatorNone      = "none"
	prefixUser        = "user:"
	prefixAgent       = "agent:"

	// Member names that occur in several objects of the schemas.
	memberName          = "name"
	memberTarget        = "target"
	memberUnmappedBot   = "unmapped_bot"
	memberUnmappedHuman = "unmapped_human"

	// LeadDiscord and LeadHere are the leads of a mapped channel (22 12.1).
	LeadDiscord = "discord"
	LeadHere    = "bothy"
	// DefaultPlaceholder and DefaultSkip are the values of defaults.unmapped_human and
	// defaults.unmapped_bot.
	DefaultPlaceholder = "placeholder"
	DefaultSkip        = "skip"
)

// Target is the target of a mapped space or channel: exactly one of Skip, a new object with
// CreateName or an Existing one.
type Target struct {
	Skip       bool
	CreateName string
	Existing   ids.ID
}

// Creates reports a target {create: {name}}.
func (t Target) Creates() bool { return !t.Skip && t.Existing.IsZero() }

// SpaceEntry maps a Discord category to a space.
type SpaceEntry struct {
	Category string
	Target   Target
}

// ChannelEntry maps a Discord channel to a channel: its target, the planned operator (agent
// handle, "" for none) and its lead.
type ChannelEntry struct {
	Channel         string
	Target          Target
	PlannedOperator string
	Lead            string
}

// UserKind is the target kind of a mapped Discord user.
type UserKind string

// Targets of a mapped Discord user (22 5.3).
const (
	UserPerson      UserKind = "user"
	UserAgent       UserKind = "agent"
	UserPlaceholder UserKind = "placeholder"
)

// UserEntry maps a Discord user to a person, an agent (by Handle) or a placeholder.
type UserEntry struct {
	User   string
	Kind   UserKind
	Handle string
}

// Defaults are the rules for what the mapping does not name.
type Defaults struct {
	// UnmappedHuman and UnmappedBot are DefaultPlaceholder or DefaultSkip.
	UnmappedHuman, UnmappedBot string
	// UncategorizedSpace is "skip" or the category of a space entry.
	UncategorizedSpace string
}

// Mapping is the mapping of schema import_source.discord.v1 (22 5.2), stored normalized in
// import_sources.config.
type Mapping struct {
	Version  int
	GuildID  string
	Defaults Defaults
	Spaces   []SpaceEntry
	Channels []ChannelEntry
	Users    []UserEntry
}

// RefKind is the kind of a reference the use case resolves through the ports of identity and chat.
type RefKind string

// The references of a mapping.
const (
	RefPerson  RefKind = "user"
	RefAgent   RefKind = "agent"
	RefSpace   RefKind = "space"
	RefChannel RefKind = "channel"
)

// Ref is one reference of the mapping to an existing principal or object, with the pointer of
// its member: the use case looks it up and records unknown_handle, not_active, unknown_space or
// unknown_channel there.
type Ref struct {
	Pointer string
	Kind    RefKind
	Handle  string
	ID      ids.ID
}

// Limits are the name limits the mapping is checked against.
type Limits struct {
	// SpaceName is space.name_max_chars, ChannelName channel.name_max_chars.
	SpaceName, ChannelName int
}

// ParseMapping decodes raw as import_source.discord.v1 and records every violation of the schema
// in p (unknown members, required members, Discord IDs that are no digits, duplicates within a
// list, names over the limits, an uncategorized_space that names no category). It returns the
// mapping and the references the caller still has to resolve. The config pointer is "/config".
func ParseMapping(raw json.RawMessage, lim Limits, p *Problems) (Mapping, []Ref) {
	const base = "/config"
	var out Mapping
	var refs []Ref
	m := object(raw, base, []string{"version", "guild_id", "defaults", "spaces", "channels", "users"}, p)
	if m == nil {
		return out, nil
	}
	out.Version = parseVersion(m, base, p)
	out.GuildID, _ = discordID(m, "guild_id", base, p)
	out.Defaults = parseDefaults(m, base, p)
	out.Spaces, refs = parseSpaces(m, base, lim, p, refs)
	out.Channels, refs = parseChannels(m, base, lim, p, refs)
	out.Users, refs = parseUsers(m, base, p, refs)
	if u := out.Defaults.UncategorizedSpace; u != "" && u != targetSkip && out.Space(u) == nil {
		p.Add(base+"/defaults/uncategorized_space", FieldUnknownCategory, "uncategorized_space is skip or a category of spaces")
	}
	return out, refs
}

// parseVersion reads version, which must be 1.
func parseVersion(m map[string]json.RawMessage, base string, p *Problems) int {
	raw, ok := m["version"]
	if !ok {
		p.Add(base+"/version", FieldRequired, "the member is required")
		return 0
	}
	var v int
	if err := json.Unmarshal(raw, &v); err != nil || v != 1 {
		p.Add(base+"/version", FieldInvalid, "version is 1")
		return 0
	}
	return v
}

// parseDefaults reads defaults with its three required members.
func parseDefaults(m map[string]json.RawMessage, base string, p *Problems) Defaults {
	ptr := base + "/defaults"
	raw, ok := m["defaults"]
	if !ok {
		p.Add(ptr, FieldRequired, "the member is required")
		return Defaults{}
	}
	d := object(raw, ptr, []string{memberUnmappedHuman, memberUnmappedBot, "uncategorized_space"}, p)
	if d == nil {
		return Defaults{}
	}
	var out Defaults
	for _, f := range []struct {
		key string
		dst *string
	}{{memberUnmappedHuman, &out.UnmappedHuman}, {memberUnmappedBot, &out.UnmappedBot}} {
		s, ok := str(d, f.key, ptr, true, p)
		if ok && s != DefaultPlaceholder && s != DefaultSkip {
			p.Add(Pointer(ptr, f.key), FieldInvalid, "placeholder or skip")
			continue
		}
		*f.dst = s
	}
	if s, ok := str(d, "uncategorized_space", ptr, true, p); ok {
		if s != targetSkip && !Digits(s) {
			p.Add(ptr+"/uncategorized_space", FieldNotDigits, "skip or a Discord category ID")
		} else {
			out.UncategorizedSpace = s
		}
	}
	return out
}

// list decodes the optional array member key; nil when absent or not an array (recorded).
func list(m map[string]json.RawMessage, key, base string, p *Problems) []json.RawMessage {
	raw, ok := m[key]
	if !ok || isNull(raw) {
		return nil
	}
	var out []json.RawMessage
	if err := json.Unmarshal(raw, &out); err != nil {
		p.Add(Pointer(base, key), FieldInvalid, "an array is required")
		return nil
	}
	return out
}

// parseTarget reads a target: "skip", {create: {name}} or {existing: <uuid>}. maxName is the
// name limit of a new object; refKind the reference of an existing one.
func parseTarget(raw json.RawMessage, ptr string, maxName int, refKind RefKind, p *Problems, refs []Ref) (Target, []Ref, bool) {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		if s != targetSkip {
			p.Add(ptr, FieldInvalid, "skip, {create: {name}} or {existing: <uuid>}")
			return Target{}, refs, false
		}
		return Target{Skip: true}, refs, true
	}
	m := object(raw, ptr, []string{"create", "existing"}, p)
	if m == nil {
		return Target{}, refs, false
	}
	_, hasCreate := m["create"]
	_, hasExisting := m["existing"]
	if hasCreate == hasExisting {
		p.Add(ptr, FieldInvalid, "exactly one of create and existing")
		return Target{}, refs, false
	}
	if hasExisting {
		v, ok := str(m, "existing", ptr, true, p)
		if !ok {
			return Target{}, refs, false
		}
		id, err := ids.Parse(v)
		if err != nil {
			p.Add(ptr+"/existing", FieldInvalid, "a UUID is required")
			return Target{}, refs, false
		}
		return Target{Existing: id}, append(refs, Ref{Pointer: ptr + "/existing", Kind: refKind, ID: id}), true
	}
	c := object(m["create"], ptr+"/create", []string{memberName}, p)
	if c == nil {
		return Target{}, refs, false
	}
	name, ok := str(c, memberName, ptr+"/create", true, p)
	if !ok {
		return Target{}, refs, false
	}
	name = strings.TrimSpace(name)
	switch {
	case name == "":
		p.Add(ptr+"/create/name", FieldRequired, "the name is required")
		return Target{}, refs, false
	case utf8.RuneCountInString(name) > maxName:
		p.Add(ptr+"/create/name", FieldTooLong, "the name is longer than its configured limit")
		return Target{}, refs, false
	case textcheck.DisplayText(name) != nil:
		// The rules of a space or channel name (SEC-IN-4) hold here already, so a stored mapping
		// never names an object chat would refuse at the first batch.
		p.Add(ptr+"/create/name", FieldInvalid, "the name contains control or bidi characters")
		return Target{}, refs, false
	}
	return Target{CreateName: name}, refs, true
}

// seen records a duplicate Discord ID of one list at ptr.
func seen(set map[string]bool, id, ptr string, p *Problems) bool {
	if set[id] {
		p.Add(ptr, FieldDuplicate, "the Discord ID is named twice")
		return true
	}
	set[id] = true
	return false
}

// parseSpaces reads spaces.
func parseSpaces(m map[string]json.RawMessage, base string, lim Limits, p *Problems, refs []Ref) ([]SpaceEntry, []Ref) {
	var out []SpaceEntry
	set := map[string]bool{}
	for i, raw := range list(m, "spaces", base, p) {
		ptr := Pointer(base+"/spaces", i)
		e := object(raw, ptr, []string{"discord_category", memberTarget}, p)
		if e == nil {
			continue
		}
		id, okID := discordID(e, "discord_category", ptr, p)
		if okID && seen(set, id, ptr+"/discord_category", p) {
			okID = false
		}
		var t Target
		okT := hasTarget(e, ptr, p)
		if okT {
			t, refs, okT = parseTarget(e[memberTarget], ptr+"/target", lim.SpaceName, RefSpace, p, refs)
		}
		if okID && okT {
			out = append(out, SpaceEntry{Category: id, Target: t})
		}
	}
	return out, refs
}

// hasTarget records a missing target member and reports a present one.
func hasTarget(e map[string]json.RawMessage, ptr string, p *Problems) bool {
	if _, ok := e[memberTarget]; !ok {
		p.Add(ptr+"/target", FieldRequired, "the member is required")
		return false
	}
	return true
}

// parseChannels reads channels with planned_operator (default none) and lead (default discord).
func parseChannels(m map[string]json.RawMessage, base string, lim Limits, p *Problems, refs []Ref) ([]ChannelEntry, []Ref) {
	var out []ChannelEntry
	set := map[string]bool{}
	for i, raw := range list(m, "channels", base, p) {
		ptr := Pointer(base+"/channels", i)
		e := object(raw, ptr, []string{"discord_channel", memberTarget, "planned_operator", "lead"}, p)
		if e == nil {
			continue
		}
		id, okID := discordID(e, "discord_channel", ptr, p)
		if okID && seen(set, id, ptr+"/discord_channel", p) {
			okID = false
		}
		var t Target
		okT := hasTarget(e, ptr, p)
		if okT {
			t, refs, okT = parseTarget(e[memberTarget], ptr+"/target", lim.ChannelName, RefChannel, p, refs)
		}
		entry := ChannelEntry{Channel: id, Target: t, Lead: LeadDiscord}
		var okOp bool
		refs, okOp = parseChannelOptions(e, ptr, &entry, p, refs)
		if okID && okT && okOp {
			out = append(out, entry)
		}
	}
	return out, refs
}

// parseChannelOptions reads planned_operator and lead of one channel entry into entry.
func parseChannelOptions(e map[string]json.RawMessage, ptr string, entry *ChannelEntry, p *Problems, refs []Ref) ([]Ref, bool) {
	ok := true
	if s, present := str(e, "planned_operator", ptr, false, p); present && s != operatorNone {
		h, isAgent := strings.CutPrefix(s, prefixAgent)
		if !isAgent || h == "" {
			p.Add(ptr+"/planned_operator", FieldInvalid, "agent:<handle> or none")
			ok = false
		} else {
			entry.PlannedOperator = h
			refs = append(refs, Ref{Pointer: ptr + "/planned_operator", Kind: RefAgent, Handle: h})
		}
	}
	if s, present := str(e, "lead", ptr, false, p); present {
		if s != LeadDiscord && s != LeadHere {
			p.Add(ptr+"/lead", FieldInvalid, "discord or "+LeadHere)
			ok = false
		}
		entry.Lead = s
	}
	return refs, ok
}

// parseUsers reads users.
func parseUsers(m map[string]json.RawMessage, base string, p *Problems, refs []Ref) ([]UserEntry, []Ref) {
	var out []UserEntry
	set := map[string]bool{}
	for i, raw := range list(m, "users", base, p) {
		ptr := Pointer(base+"/users", i)
		e := object(raw, ptr, []string{"discord_user", memberTarget}, p)
		if e == nil {
			continue
		}
		id, okID := discordID(e, "discord_user", ptr, p)
		if okID && seen(set, id, ptr+"/discord_user", p) {
			okID = false
		}
		s, ok := str(e, memberTarget, ptr, true, p)
		if !ok {
			continue
		}
		entry := UserEntry{User: id}
		switch {
		case s == targetPlaceholder:
			entry.Kind = UserPlaceholder
		case strings.HasPrefix(s, prefixUser) && len(s) > len(prefixUser):
			entry.Kind, entry.Handle = UserPerson, s[len(prefixUser):]
			refs = append(refs, Ref{Pointer: ptr + "/target", Kind: RefPerson, Handle: entry.Handle})
		case strings.HasPrefix(s, prefixAgent) && len(s) > len(prefixAgent):
			entry.Kind, entry.Handle = UserAgent, s[len(prefixAgent):]
			refs = append(refs, Ref{Pointer: ptr + "/target", Kind: RefAgent, Handle: entry.Handle})
		default:
			p.Add(ptr+"/target", FieldInvalid, "user:<handle>, agent:<handle> or placeholder")
			continue
		}
		if okID {
			out = append(out, entry)
		}
	}
	return out, refs
}

// Space returns the space entry of the category, nil when the mapping does not name it.
func (m Mapping) Space(category string) *SpaceEntry {
	for i := range m.Spaces {
		if m.Spaces[i].Category == category {
			return &m.Spaces[i]
		}
	}
	return nil
}

// Channel returns the channel entry of the Discord channel, nil when the mapping does not name it.
func (m Mapping) Channel(channel string) *ChannelEntry {
	for i := range m.Channels {
		if m.Channels[i].Channel == channel {
			return &m.Channels[i]
		}
	}
	return nil
}

// User returns the user entry of the Discord user, nil when the mapping does not name it.
func (m Mapping) User(user string) *UserEntry {
	for i := range m.Users {
		if m.Users[i].User == user {
			return &m.Users[i]
		}
	}
	return nil
}

// SetLead changes the lead of the mapped channel; false when the mapping does not name it.
func (m *Mapping) SetLead(channel, lead string) bool {
	for i := range m.Channels {
		if m.Channels[i].Channel == channel {
			m.Channels[i].Lead = lead
			return true
		}
	}
	return false
}

// targetJSON is the normalized JSON form of a target.
func targetJSON(t Target) any {
	switch {
	case t.Skip:
		return targetSkip
	case !t.Existing.IsZero():
		return map[string]string{"existing": t.Existing.String()}
	}
	return map[string]map[string]string{"create": {memberName: t.CreateName}}
}

// JSON returns the normalized mapping stored in import_sources.config: every member written, the
// defaults of planned_operator (none) and lead (discord) filled in, names trimmed.
func (m Mapping) JSON() (json.RawMessage, error) {
	type space struct {
		Category string `json:"discord_category"`
		Target   any    `json:"target"`
	}
	type channel struct {
		Channel  string `json:"discord_channel"`
		Target   any    `json:"target"`
		Operator string `json:"planned_operator"`
		Lead     string `json:"lead"`
	}
	type user struct {
		User   string `json:"discord_user"`
		Target string `json:"target"`
	}
	out := struct {
		Version  int               `json:"version"`
		GuildID  string            `json:"guild_id"`
		Defaults map[string]string `json:"defaults"`
		Spaces   []space           `json:"spaces"`
		Channels []channel         `json:"channels"`
		Users    []user            `json:"users"`
	}{
		Version: m.Version, GuildID: m.GuildID,
		Defaults: map[string]string{
			"unmapped_human": m.Defaults.UnmappedHuman, "unmapped_bot": m.Defaults.UnmappedBot,
			"uncategorized_space": m.Defaults.UncategorizedSpace,
		},
		Spaces: []space{}, Channels: []channel{}, Users: []user{},
	}
	for _, s := range m.Spaces {
		out.Spaces = append(out.Spaces, space{Category: s.Category, Target: targetJSON(s.Target)})
	}
	for _, c := range m.Channels {
		op := operatorNone
		if c.PlannedOperator != "" {
			op = prefixAgent + c.PlannedOperator
		}
		out.Channels = append(out.Channels, channel{Channel: c.Channel, Target: targetJSON(c.Target), Operator: op, Lead: c.Lead})
	}
	for _, u := range m.Users {
		t := targetPlaceholder
		if u.Kind != UserPlaceholder {
			t = string(u.Kind) + ":" + u.Handle
		}
		out.Users = append(out.Users, user{User: u.User, Target: t})
	}
	return json.Marshal(out)
}

// StoredMapping decodes a mapping that ParseMapping accepted and JSON stored. Errors: a stored
// value that no longer parses (an internal error of the caller).
func StoredMapping(raw json.RawMessage) (Mapping, bool) {
	var p Problems
	m, _ := ParseMapping(raw, Limits{SpaceName: maxStoredName, ChannelName: maxStoredName}, &p)
	return m, p.Empty()
}

// maxStoredName lifts the name limits when a stored mapping is read back: the limits held when
// the source was created, and a lowered limit later must not make a stored source unreadable.
const maxStoredName = int(^uint(0) >> 1)
