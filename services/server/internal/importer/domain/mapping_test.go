// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/importer/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

const existingSpace = "0192f4a0-7b1c-78d2-8a6a-ec465435b8a0"

var limits = domain.Limits{SpaceName: 10, ChannelName: 8}

// valid is a mapping without a violation.
const valid = `{
  "version": 1, "guild_id": "100",
  "defaults": {"unmapped_human": "placeholder", "unmapped_bot": "skip", "uncategorized_space": "200"},
  "spaces": [
    {"discord_category": "200", "target": {"create": {"name": " Support "}}},
    {"discord_category": "201", "target": {"existing": "` + existingSpace + `"}},
    {"discord_category": "202", "target": "skip"}
  ],
  "channels": [
    {"discord_channel": "300", "target": {"create": {"name": "tickets"}}, "planned_operator": "agent:lotse"},
    {"discord_channel": "301", "target": "skip", "lead": "bothy"}
  ],
  "users": [
    {"discord_user": "400", "target": "user:alex"},
    {"discord_user": "401", "target": "agent:lotse"},
    {"discord_user": "402", "target": "placeholder"}
  ]
}`

func parse(t *testing.T, raw string) (domain.Mapping, []domain.Ref, []errs.FieldError) {
	t.Helper()
	var p domain.Problems
	m, refs := domain.ParseMapping(json.RawMessage(raw), limits, &p)
	return m, refs, p.Fields()
}

// TestMappingValidation_Valid parses a mapping without violation, fills the defaults, lists the
// references and normalizes the JSON.
func TestMappingValidation_Valid(t *testing.T) {
	t.Parallel()
	m, refs, fields := parse(t, valid)
	if len(fields) != 0 {
		t.Fatalf("fields %+v", fields)
	}
	if m.Space("200").Target.CreateName != "Support" || m.Space("201").Target.Existing != ids.MustParse(existingSpace) || !m.Space("202").Target.Skip {
		t.Errorf("spaces %+v", m.Spaces)
	}
	if c := m.Channel("300"); c.Lead != domain.LeadDiscord || c.PlannedOperator != "lotse" {
		t.Errorf("channel 300 %+v", c)
	}
	if c := m.Channel("301"); c.Lead != domain.LeadBothy || c.PlannedOperator != "" || !c.Target.Skip {
		t.Errorf("channel 301 %+v", c)
	}
	wantRefs := []domain.Ref{
		{Pointer: "/config/spaces/1/target/existing", Kind: domain.RefSpace, ID: ids.MustParse(existingSpace)},
		{Pointer: "/config/channels/0/planned_operator", Kind: domain.RefAgent, Handle: "lotse"},
		{Pointer: "/config/users/0/target", Kind: domain.RefPerson, Handle: "alex"},
		{Pointer: "/config/users/1/target", Kind: domain.RefAgent, Handle: "lotse"},
	}
	if !reflect.DeepEqual(refs, wantRefs) {
		t.Errorf("refs\n got %+v\nwant %+v", refs, wantRefs)
	}
	raw, err := m.JSON()
	if err != nil {
		t.Fatal(err)
	}
	back, ok := domain.StoredMapping(raw)
	if !ok || !reflect.DeepEqual(back, m) {
		t.Errorf("stored round trip %v:\n got %+v\nwant %+v", ok, back, m)
	}
	if !strings.Contains(string(raw), `"planned_operator":"none"`) || !strings.Contains(string(raw), `"lead":"discord"`) {
		t.Errorf("normalized json %s", raw)
	}
}

// TestMappingValidation has one row per rule of design note 2; every row's violation is reported
// with its pointer, and a mapping with several violations reports all of them at once.
func TestMappingValidation(t *testing.T) {
	t.Parallel()
	base := func(spaces, channels, users string) string {
		return `{"version": 1, "guild_id": "100", "defaults": {"unmapped_human": "placeholder", "unmapped_bot": "placeholder", "uncategorized_space": "skip"},
			"spaces": [` + spaces + `], "channels": [` + channels + `], "users": [` + users + `]}`
	}
	cases := []struct {
		name, raw, pointer, code string
	}{
		{"not an object", `[]`, "/config", domain.FieldInvalid},
		{"unknown member", `{"version": 1, "guild_id": "1", "defaults": {"unmapped_human": "skip", "unmapped_bot": "skip", "uncategorized_space": "skip"}, "token": "x"}`, "/config/token", domain.FieldUnknownMember},
		{"version", `{"version": 2, "guild_id": "1", "defaults": {"unmapped_human": "skip", "unmapped_bot": "skip", "uncategorized_space": "skip"}}`, "/config/version", domain.FieldInvalid},
		{"guild digits", `{"version": 1, "guild_id": "12a", "defaults": {"unmapped_human": "skip", "unmapped_bot": "skip", "uncategorized_space": "skip"}}`, "/config/guild_id", domain.FieldNotDigits},
		{"defaults missing", `{"version": 1, "guild_id": "1"}`, "/config/defaults", domain.FieldRequired},
		{"unmapped value", `{"version": 1, "guild_id": "1", "defaults": {"unmapped_human": "drop", "unmapped_bot": "skip", "uncategorized_space": "skip"}}`, "/config/defaults/unmapped_human", domain.FieldInvalid},
		{"uncategorized unknown", `{"version": 1, "guild_id": "1", "defaults": {"unmapped_human": "skip", "unmapped_bot": "skip", "uncategorized_space": "9"}}`, "/config/defaults/uncategorized_space", domain.FieldUnknownCategory},
		{"space id digits", base(`{"discord_category": "x", "target": "skip"}`, "", ""), "/config/spaces/0/discord_category", domain.FieldNotDigits},
		{"space duplicate", base(`{"discord_category": "1", "target": "skip"}, {"discord_category": "1", "target": "skip"}`, "", ""), "/config/spaces/1/discord_category", domain.FieldDuplicate},
		{"target both", base(`{"discord_category": "1", "target": {"create": {"name": "a"}, "existing": "`+existingSpace+`"}}`, "", ""), "/config/spaces/0/target", domain.FieldInvalid},
		{"target string", base(`{"discord_category": "1", "target": "keep"}`, "", ""), "/config/spaces/0/target", domain.FieldInvalid},
		{"existing uuid", base(`{"discord_category": "1", "target": {"existing": "nope"}}`, "", ""), "/config/spaces/0/target/existing", domain.FieldInvalid},
		{"space name long", base(`{"discord_category": "1", "target": {"create": {"name": "abcdefghijk"}}}`, "", ""), "/config/spaces/0/target/create/name", domain.FieldTooLong},
		{"name with a bidi control", base(`{"discord_category": "1", "target": {"create": {"name": "Sup\u200fport"}}}`, "", ""), "/config/spaces/0/target/create/name", domain.FieldInvalid},
		{"name with a control character", base("", `{"discord_channel": "1", "target": {"create": {"name": "a\u0007b"}}}`, ""), "/config/channels/0/target/create/name", domain.FieldInvalid},
		{"channel name long", base("", `{"discord_channel": "1", "target": {"create": {"name": "abcdefghi"}}}`, ""), "/config/channels/0/target/create/name", domain.FieldTooLong},
		{"create unknown member", base("", `{"discord_channel": "1", "target": {"create": {"name": "a", "topic": "b"}}}`, ""), "/config/channels/0/target/create/topic", domain.FieldUnknownMember},
		{"channel duplicate", base("", `{"discord_channel": "1", "target": "skip"}, {"discord_channel": "1", "target": "skip"}`, ""), "/config/channels/1/discord_channel", domain.FieldDuplicate},
		{"operator form", base("", `{"discord_channel": "1", "target": "skip", "planned_operator": "user:alex"}`, ""), "/config/channels/0/planned_operator", domain.FieldInvalid},
		{"lead", base("", `{"discord_channel": "1", "target": "skip", "lead": "both"}`, ""), "/config/channels/0/lead", domain.FieldInvalid},
		{"target missing", base("", `{"discord_channel": "1"}`, ""), "/config/channels/0/target", domain.FieldRequired},
		{"user target", base("", "", `{"discord_user": "1", "target": "group:x"}`), "/config/users/0/target", domain.FieldInvalid},
		{"user duplicate", base("", "", `{"discord_user": "1", "target": "placeholder"}, {"discord_user": "1", "target": "placeholder"}`), "/config/users/1/discord_user", domain.FieldDuplicate},
		{"user unknown member", base("", "", `{"discord_user": "1", "target": "placeholder", "note": "x"}`), "/config/users/0/note", domain.FieldUnknownMember},
	}
	for _, c := range cases {
		_, _, fields := parse(t, c.raw)
		if !slices.ContainsFunc(fields, func(f errs.FieldError) bool { return f.Pointer == c.pointer && f.Code == c.code }) {
			t.Errorf("%s: want %s at %s, got %+v", c.name, c.code, c.pointer, fields)
		}
	}
	// All errors at once.
	_, _, fields := parse(t, base(`{"discord_category": "x", "target": "skip"}`, `{"discord_channel": "y", "target": "skip"}`, `{"discord_user": "z", "target": "placeholder"}`))
	if len(fields) != 3 {
		t.Errorf("want three violations at once, got %+v", fields)
	}
	var p domain.Problems
	if p.Err() != nil {
		t.Error("no problems is no error")
	}
	p.Add("/config", domain.FieldInvalid, "x")
	if e, ok := errs.As(p.Err()); !ok || e.Code != errs.CodeValidationFailed || len(e.Fields) != 1 {
		t.Errorf("Err() = %v", p.Err())
	}
}

// TestResolveUser checks the targets of 22 5.3: webhook always placeholder, mapped entries, and
// the defaults for unmapped people and bots.
func TestResolveUser(t *testing.T) {
	t.Parallel()
	m, _, _ := parse(t, valid)
	cases := []struct {
		id   string
		kind string
		want domain.UserTarget
	}{
		{"400", domain.DiscordUser, domain.UserTarget{Kind: domain.UserPerson, Handle: "alex"}},
		{"401", domain.DiscordBot, domain.UserTarget{Kind: domain.UserAgent, Handle: "lotse"}},
		{"400", domain.DiscordWebhook, domain.UserTarget{Kind: domain.UserPlaceholder}},
		{"999", domain.DiscordUser, domain.UserTarget{Kind: domain.UserPlaceholder}},
		{"999", domain.DiscordBot, domain.UserTarget{Skip: true}},
	}
	for _, c := range cases {
		if got := m.ResolveUser(c.id, domain.UserData{DiscordKind: c.kind}); got != c.want {
			t.Errorf("%s %s = %+v, want %+v", c.id, c.kind, got, c.want)
		}
	}
	if !m.SetLead("300", domain.LeadBothy) || m.Channel("300").Lead != domain.LeadBothy || m.SetLead("999", domain.LeadBothy) {
		t.Error("SetLead")
	}
}
