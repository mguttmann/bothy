// SPDX-License-Identifier: Apache-2.0

package importtool

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/cmd/bothy/importtool/discord/discordtest"
)

// emptied returns the basic fixture with every message of the general channel stripped of its
// content, attachments, embeds, stickers, poll and snapshots, as Discord delivers them without
// the intent MESSAGE_CONTENT.
func emptied(t testing.TB) *discordtest.Fixture {
	t.Helper()
	f := discordtest.BasicFixture(t)
	for i, raw := range f.Messages[fxGeneral] {
		var m map[string]any
		if err := json.Unmarshal(raw, &m); err != nil {
			t.Fatal(err)
		}
		m["content"] = ""
		for _, k := range []string{"attachments", "embeds", "sticker_items", "message_snapshots"} {
			m[k] = []any{}
		}
		delete(m, "poll")
		b, err := json.Marshal(m)
		if err != nil {
			t.Fatal(err)
		}
		f.Messages[fxGeneral][i] = b
	}
	return f
}

// notBot returns the basic fixture whose token belongs to a user account.
func notBot(t testing.TB) *discordtest.Fixture {
	t.Helper()
	f := discordtest.BasicFixture(t)
	f.Me = json.RawMessage(`{"id": "13000002", "username": "alex", "global_name": "Alex", "bot": false}`)
	return f
}

// TestPreflightFindings (22 4.4, 22 16 row "Intent probe"): every finding ends the run with exit 2
// before any write, also a dry run.
func TestPreflightFindings(t *testing.T) {
	for _, tc := range []struct {
		name    string
		fixture func(testing.TB) *discordtest.Fixture
		mapping string
		setup   func(*testEnv)
		want    string
	}{
		{name: "intent probe", fixture: emptied, mapping: fixtureMapping, want: "MESSAGE_CONTENT"},
		{name: "not a bot token", fixture: notBot, mapping: fixtureMapping, want: "not a bot token"},
		{name: "not an owner", fixture: discordtest.BasicFixture, mapping: fixtureMapping,
			setup: func(e *testEnv) { e.bothy.SetMe(false, true) }, want: "not an owner"},
		{name: "unknown handle", fixture: discordtest.BasicFixture,
			mapping: strings.Replace(fixtureMapping, `"user:alex"`, `"user:sam"`, 1), want: "user:sam"},
		{name: "unknown planned operator", fixture: discordtest.BasicFixture,
			mapping: strings.Replace(fixtureMapping, `name: "general"}}`, `name: "general"}}`+"\n    planned_operator: \"agent:lotse\"", 1),
			want:    "agent:lotse"},
		{name: "space name collision", fixture: discordtest.BasicFixture, mapping: fixtureMapping,
			setup: func(e *testEnv) { e.bothy.AddSpace("projects") }, want: "space name"},
		{name: "channel name collision", fixture: discordtest.BasicFixture, mapping: fixtureMapping,
			setup: func(e *testEnv) {
				// The mapping adopts a space that holds a channel named like a new one.
				id := e.bothy.AddSpace("Existing")
				e.bothy.AddChannel(id, "General")
				e.mapText = strings.Replace(fixtureMapping, `{create: {name: "Projects"}}`, `{existing: "`+id+`"}`, 1)
			}, want: "channel name"},
		{name: "unknown channel", fixture: discordtest.BasicFixture,
			mapping: strings.Replace(fixtureMapping, `"13000104"`, `"13000199"`, 1), want: "does not exist"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newTestEnv(t, tc.fixture(t))
			if tc.setup != nil {
				tc.setup(e)
			}
			text := tc.mapping
			if e.mapText != "" {
				text = e.mapText
			}
			base := e.bothy.Writes()
			if code := e.run("--guild", fxGuild, "--map", e.writeMap(text)); code != ExitUsage {
				t.Fatalf("exit %d, want 2: %s", code, e.stderr.String())
			}
			if !strings.Contains(e.stderr.String(), tc.want) {
				t.Errorf("stderr %q does not say %q", e.stderr.String(), tc.want)
			}
			if w := e.bothy.Writes() - base; w != 0 {
				t.Errorf("writes = %d, want 0", w)
			}
		})
	}
}
