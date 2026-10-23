// SPDX-License-Identifier: Apache-2.0

package importtool

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
)

// testGuild is the invented Discord server ID of the mapping tests.
const testGuild = "100"

// existingSpaceID is an invented UUIDv7 of an existing space.
const existingSpaceID = "0192f4a0-7b1c-78d2-8a6a-ec465435b8a0"

// validMapping uses every form of 22 5.2 with white-label data.
const validMapping = `version: 1
guild_id: "100"
defaults:
  unmapped_human: placeholder
  unmapped_bot: skip
  uncategorized_space: "200"
spaces:
  - discord_category: "200"
    target: {create: {name: " Support "}}
  - discord_category: "201"
    target: {existing: "0192F4A0-7B1C-78D2-8A6A-EC465435B8A0"}
  - discord_category: 202
    target: skip
channels:
  - discord_channel: "300"
    target: {create: {name: tickets}}
    planned_operator: "agent:support-bot"
  - discord_channel: "301"
    target: skip
    planned_operator: none
    lead: bothy
users:
  - discord_user: "400"
    target: "user:alex"
  - discord_user: "401"
    target: "agent:support-bot"
  - discord_user: "402"
    target: placeholder
`

// validNormalized is the JSON the server stores for validMapping (domain.Mapping.JSON).
const validNormalized = `{
  "version": 1, "guild_id": "100",
  "defaults": {"unmapped_human": "placeholder", "unmapped_bot": "skip", "uncategorized_space": "200"},
  "spaces": [
    {"discord_category": "200", "target": {"create": {"name": "Support"}}},
    {"discord_category": "201", "target": {"existing": "0192f4a0-7b1c-78d2-8a6a-ec465435b8a0"}},
    {"discord_category": "202", "target": "skip"}
  ],
  "channels": [
    {"discord_channel": "300", "target": {"create": {"name": "tickets"}}, "planned_operator": "agent:support-bot", "lead": "discord"},
    {"discord_channel": "301", "target": "skip", "planned_operator": "none", "lead": "bothy"}
  ],
  "users": [
    {"discord_user": "400", "target": "user:alex"},
    {"discord_user": "401", "target": "agent:support-bot"},
    {"discord_user": "402", "target": "placeholder"}
  ]
}`

// decodeJSON decodes text into a generic JSON value.
func decodeJSON(t *testing.T, text string) map[string]any {
	t.Helper()
	var v map[string]any
	if err := json.Unmarshal([]byte(text), &v); err != nil {
		t.Fatal(err)
	}
	return v
}

// mustParse parses text for testGuild and fails the test on an error.
func mustParse(t *testing.T, text string) *Mapping {
	t.Helper()
	m, err := ParseMapping([]byte(text), testGuild)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// TestParseMappingValid parses every form and fills the defaults.
func TestParseMappingValid(t *testing.T) {
	t.Parallel()
	m := mustParse(t, validMapping)
	if m.Version != 1 || m.GuildID != testGuild {
		t.Errorf("header %d %q", m.Version, m.GuildID)
	}
	if m.Defaults != (MappingDefaults{UnmappedHuman: "placeholder", UnmappedBot: "skip", UncategorizedSpace: "200"}) {
		t.Errorf("defaults %+v", m.Defaults)
	}
	wantSpaces := []SpaceEntry{
		{Category: "200", Target: Target{CreateName: "Support"}, Line: 8},
		{Category: "201", Target: Target{Existing: existingSpaceID}, Line: 10},
		{Category: "202", Target: Target{Skip: true}, Line: 12},
	}
	if !reflect.DeepEqual(m.Spaces, wantSpaces) {
		t.Errorf("spaces\n got %+v\nwant %+v", m.Spaces, wantSpaces)
	}
	wantChannels := []ChannelEntry{
		{Channel: "300", Target: Target{CreateName: "tickets"}, PlannedOperator: "support-bot", Lead: LeadDiscord, Line: 15},
		{Channel: "301", Target: Target{Skip: true}, Lead: LeadBothy, Line: 18},
	}
	if !reflect.DeepEqual(m.Channels, wantChannels) {
		t.Errorf("channels\n got %+v\nwant %+v", m.Channels, wantChannels)
	}
	wantUsers := []UserEntry{
		{User: "400", Kind: "user", Handle: "alex", Line: 23},
		{User: "401", Kind: "agent", Handle: "support-bot", Line: 25},
		{User: "402", Kind: "placeholder", Line: 27},
	}
	if !reflect.DeepEqual(m.Users, wantUsers) {
		t.Errorf("users\n got %+v\nwant %+v", m.Users, wantUsers)
	}
	if m.Space("201") == nil || m.Space("999") != nil || m.Channel("301") == nil || m.Channel("999") != nil ||
		m.User("402") == nil || m.User("999") != nil {
		t.Error("lookups")
	}
}

// TestParseMappingMinimal: absent lists are empty, the normalized form writes them as [].
func TestParseMappingMinimal(t *testing.T) {
	t.Parallel()
	m := mustParse(t, "version: 1\nguild_id: \"100\"\ndefaults: {unmapped_human: skip, unmapped_bot: skip, uncategorized_space: skip}\nspaces:\n")
	want := decodeJSON(t, `{"version": 1, "guild_id": "100",
		"defaults": {"unmapped_human": "skip", "unmapped_bot": "skip", "uncategorized_space": "skip"},
		"spaces": [], "channels": [], "users": []}`)
	if got := m.Normalized(); !reflect.DeepEqual(got, want) {
		t.Errorf("normalized\n got %#v\nwant %#v", got, want)
	}
}

// replaceLine returns validMapping with the 1-based line n replaced by text (several lines
// allowed).
func replaceLine(n int, text string) string {
	lines := strings.Split(validMapping, "\n")
	lines[n-1] = text
	return strings.Join(lines, "\n")
}

// TestParseMappingErrors has one row per error of design note 3; each is a *MappingError with
// the YAML line and without any value of the file.
func TestParseMappingErrors(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, text string
		guild      string
		line       int
	}{
		{"empty file", "", testGuild, 0},
		{"only a comment", "# nothing\n", testGuild, 0},
		{"null document", "~\n", testGuild, 0},
		{"two documents", validMapping + "---\nversion: 1\n", testGuild, 29},
		{"invalid YAML", "version: 1\nguild_id: [\n", testGuild, 2},
		{"root not a mapping", "- 1\n", testGuild, 1},
		{"unknown top-level member", replaceLine(1, "version: 1\ntoken: secretvalue"), testGuild, 2},
		{"unknown member in an entry", replaceLine(17, "    planned_operator: \"agent:support-bot\"\n    topic: secretvalue"), testGuild, 18},
		{"unknown member in defaults", replaceLine(4, "  unmapped_human: placeholder\n  extra: secretvalue"), testGuild, 5},
		{"unknown member in a target", replaceLine(16, "    target: {create: {name: tickets, topic: secretvalue}}"), testGuild, 16},
		{"duplicate member", replaceLine(1, "version: 1\nversion: 1"), testGuild, 2},
		{"version missing", replaceLine(1, "# no version"), testGuild, 2},
		{"version 2", replaceLine(1, "version: 2"), testGuild, 1},
		{"version as text", replaceLine(1, "version: \"1\""), testGuild, 1},
		{"guild_id missing", replaceLine(2, "# none"), testGuild, 1},
		{"guild_id not digits", replaceLine(2, "guild_id: \"10a\""), testGuild, 2},
		{"guild_id too long", replaceLine(2, "guild_id: \"123456789012345678901\""), testGuild, 2},
		{"guild_id above uint64", replaceLine(2, "guild_id: \"99999999999999999999\""), testGuild, 2},
		{"guild_id differs from --guild", validMapping, "101", 2},
		{"defaults missing", strings.Replace(validMapping, "defaults:\n  unmapped_human: placeholder\n  unmapped_bot: skip\n  uncategorized_space: \"200\"\n", "", 1), testGuild, 1},
		{"defaults not a mapping", strings.Replace(validMapping, "defaults:\n  unmapped_human: placeholder\n  unmapped_bot: skip\n  uncategorized_space: \"200\"\n", "defaults: skip\n", 1), testGuild, 3},
		{"defaults member missing", replaceLine(5, "  # none"), testGuild, 4},
		{"unmapped_human value", replaceLine(4, "  unmapped_human: drop"), testGuild, 4},
		{"unmapped_bot wrong type", replaceLine(5, "  unmapped_bot: [skip]"), testGuild, 5},
		{"uncategorized_space neither skip nor ID", replaceLine(6, "  uncategorized_space: general"), testGuild, 6},
		{"uncategorized_space names no category", replaceLine(6, "  uncategorized_space: \"299\""), testGuild, 6},
		{"spaces not a list", strings.Replace(validMapping, "spaces:\n", "spaces: {}\nx_spaces:\n", 1), testGuild, 7},
		{"category not digits", replaceLine(10, "  - discord_category: \"2x\""), testGuild, 10},
		{"category missing", replaceLine(10, "  -"), testGuild, 11},
		{"category duplicate", replaceLine(10, "  - discord_category: \"200\""), testGuild, 10},
		{"target missing", replaceLine(9, "    # none"), testGuild, 8},
		{"target unknown string", replaceLine(9, "    target: keep"), testGuild, 9},
		{"target both forms", replaceLine(9, "    target: {create: {name: a}, existing: \""+existingSpaceID+"\"}"), testGuild, 9},
		{"target neither form", replaceLine(9, "    target: {}"), testGuild, 9},
		{"target a list", replaceLine(9, "    target: [skip]"), testGuild, 9},
		{"existing not a UUID", replaceLine(11, "    target: {existing: \"nope\"}"), testGuild, 11},
		{"existing not UUIDv7", replaceLine(11, "    target: {existing: \"0192f4a0-7b1c-48d2-8a6a-ec465435b8a0\"}"), testGuild, 11},
		{"create name empty after trim", replaceLine(9, "    target: {create: {name: \"   \"}}"), testGuild, 9},
		{"create name missing", replaceLine(9, "    target: {create: {}}"), testGuild, 9},
		{"create name not text", replaceLine(9, "    target: {create: {name: [a]}}"), testGuild, 9},
		{"create name with a control character", replaceLine(9, "    target: {create: {name: \"a\\u0007b\"}}"), testGuild, 9},
		{"create name with a bidi control", replaceLine(9, "    target: {create: {name: \"Sup\\u200Fport\"}}"), testGuild, 9},
		{"channel not digits", replaceLine(15, "  - discord_channel: \"general\""), testGuild, 15},
		{"channel duplicate", replaceLine(18, "  - discord_channel: \"300\""), testGuild, 18},
		{"planned_operator a user", replaceLine(17, "    planned_operator: \"user:alex\""), testGuild, 17},
		{"planned_operator empty handle", replaceLine(17, "    planned_operator: \"agent:\""), testGuild, 17},
		{"lead neither discord nor bothy", replaceLine(21, "    lead: both"), testGuild, 21},
		{"user not digits", replaceLine(23, "  - discord_user: \"alex\""), testGuild, 23},
		{"user duplicate", replaceLine(25, "  - discord_user: \"400\""), testGuild, 25},
		{"user target group", replaceLine(24, "    target: \"group:x\""), testGuild, 24},
		{"user target empty handle", replaceLine(24, "    target: \"user:\""), testGuild, 24},
		{"user target missing", replaceLine(24, "    # none"), testGuild, 23},
		{"user target a mapping", replaceLine(24, "    target: {create: {name: alex}}"), testGuild, 24},
	}
	for _, c := range cases {
		m, err := ParseMapping([]byte(c.text), c.guild)
		var me *MappingError
		if !errors.As(err, &me) {
			t.Errorf("%s: want *MappingError, got %v (mapping %v)", c.name, err, m != nil)
			continue
		}
		if m != nil {
			t.Errorf("%s: a mapping next to the error", c.name)
		}
		if me.Line != c.line {
			t.Errorf("%s: line %d, want %d (%s)", c.name, me.Line, c.line, me.Msg)
		}
		if strings.Contains(err.Error(), "secretvalue") || strings.Contains(err.Error(), "alex") ||
			strings.Contains(err.Error(), "Support") {
			t.Errorf("%s: the error repeats a value of the file: %s", c.name, err)
		}
		if !strings.HasPrefix(err.Error(), "mapping file line ") {
			t.Errorf("%s: error text %q", c.name, err)
		}
	}
}

// TestNormalizedEqualsServerForm: the normalized form is the JSON the server stores.
func TestNormalizedEqualsServerForm(t *testing.T) {
	t.Parallel()
	got := mustParse(t, validMapping).Normalized()
	want := decodeJSON(t, validNormalized)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("normalized\n got %#v\nwant %#v", got, want)
	}
}

// TestDiffStored compares with the stored mapping after normalizing, ignoring the leads.
func TestDiffStored(t *testing.T) {
	t.Parallel()
	m := mustParse(t, validMapping)
	if d := m.DiffStored(decodeJSON(t, validNormalized)); d != "" {
		t.Errorf("equal: %q", d)
	}
	cases := []struct {
		name, stored, want string
	}{
		{"lead ignored", strings.Replace(validNormalized, `"lead": "bothy"`, `"lead": "discord"`, 1), ""},
		{"lead absent ignored", strings.Replace(validNormalized, `, "lead": "bothy"`, "", 1), ""},
		{"planned operator absent means none", strings.Replace(validNormalized, `, "planned_operator": "none"`, "", 1), ""},
		{"existing in upper case", strings.Replace(validNormalized, "0192f4a0-7b1c", "0192F4A0-7B1C", 1), ""},
		{"stored name not trimmed", strings.Replace(validNormalized, `"name": "Support"`, `"name": " Support"`, 1), ""},
		{"differing channel name", strings.Replace(validNormalized, `"name": "tickets"`, `"name": "help"`, 1), "/channels/0/target/create/name"},
		{"differing version", strings.Replace(validNormalized, `"version": 1`, `"version": 2`, 1), "/version"},
		{"differing guild", strings.Replace(validNormalized, `"guild_id": "100"`, `"guild_id": "101"`, 1), "/guild_id"},
		{"differing default", strings.Replace(validNormalized, `"unmapped_bot": "skip"`, `"unmapped_bot": "placeholder"`, 1), "/defaults/unmapped_bot"},
		{"differing target form", strings.Replace(validNormalized, `"target": "skip", "planned_operator"`, `"target": {"create": {"name": "x"}}, "planned_operator"`, 1), "/channels/1/target"},
		{"differing user", strings.Replace(validNormalized, `"user:alex"`, `"user:sam"`, 1), "/users/0/target"},
		{"extra stored entry", strings.Replace(validNormalized, `{"discord_user": "402", "target": "placeholder"}`, `{"discord_user": "402", "target": "placeholder"}, {"discord_user": "403", "target": "placeholder"}`, 1), "/users/3"},
		{"missing stored entry", strings.Replace(validNormalized, `,
    {"discord_user": "402", "target": "placeholder"}`, "", 1), "/users/2"},
		{"missing stored list", strings.Replace(validNormalized, `"version": 1,`, `"version": 1, "x": 1,`, 1), "/x"},
	}
	for _, c := range cases {
		stored := decodeJSON(t, c.stored)
		if got := m.DiffStored(stored); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
	noUsers := decodeJSON(t, validNormalized)
	delete(noUsers, "users")
	if got := m.DiffStored(noUsers); got != "/users" {
		t.Errorf("stored without users: %q", got)
	}
	// The pointer escapes "~" and "/" (RFC 6901).
	odd := decodeJSON(t, validNormalized)
	odd["a/b~c"] = true
	if got := m.DiffStored(odd); got != "/a~1b~0c" {
		t.Errorf("escaped pointer: %q", got)
	}
}

// TestLeadsOf reads the leads of the stored mapping; a channel without lead is discord.
func TestLeadsOf(t *testing.T) {
	t.Parallel()
	stored := decodeJSON(t, strings.Replace(validNormalized, `, "lead": "discord"`, "", 1))
	want := map[string]string{"300": LeadDiscord, "301": LeadBothy}
	if got := LeadsOf(stored); !reflect.DeepEqual(got, want) {
		t.Errorf("leads %v, want %v", got, want)
	}
	if got := LeadsOf(map[string]any{}); len(got) != 0 {
		t.Errorf("no channels: %v", got)
	}
}

// TestResolveUser checks the targets of 22 5.3 as the server resolves them.
func TestResolveUser(t *testing.T) {
	t.Parallel()
	m := mustParse(t, validMapping)
	cases := []struct {
		id, kind string
		want     UserTarget
	}{
		{"400", "user", UserTarget{Kind: "user", Handle: "alex"}},
		{"401", "bot", UserTarget{Kind: "agent", Handle: "support-bot"}},
		{"402", "user", UserTarget{Kind: "placeholder"}},
		{"400", "webhook", UserTarget{Kind: "placeholder"}},
		{"999", "webhook", UserTarget{Kind: "placeholder"}},
		{"999", "user", UserTarget{Kind: "placeholder"}},
		{"999", "bot", UserTarget{Skip: true}},
	}
	for _, c := range cases {
		if got := m.ResolveUser(c.id, c.kind); got != c.want {
			t.Errorf("%s %s = %+v, want %+v", c.id, c.kind, got, c.want)
		}
	}
	m.Defaults.UnmappedHuman = "skip"
	if got := m.ResolveUser("999", "user"); got != (UserTarget{Skip: true}) {
		t.Errorf("unmapped human with skip: %+v", got)
	}
}
