// SPDX-License-Identifier: Apache-2.0

package importtool

import (
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/cmd/bothy/importtool/discord/discordtest"
)

// TestInvalidSelectionWritesNothing (22 16 row "Invalid selection", 22 10.2 rule 2): an unknown
// --only-thread or --channel among valid ones ends --apply with exit 2, and the fake Bothy logs no
// write, also not for the valid IDs of the same call.
func TestInvalidSelectionWritesNothing(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
	}{
		{"unknown thread", []string{"--only-thread", fxThread, "--only-thread", "99999999"}},
		{"unknown channel", []string{"--channel", fxGeneral, "--channel", "99999999"}},
		{"voice channel", []string{"--channel", fxLounge}},
		{"thread of another channel", []string{"--channel", fxSupport, "--only-thread", fxThread}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newTestEnv(t, discordtest.BasicFixture(t))
			src := e.startSource(fixtureMapping)
			base := e.bothy.Writes()
			args := append([]string{"--guild", fxGuild, "--map", e.writeMap(fixtureMapping), "--apply"}, tc.args...)
			if code := e.run(args...); code != ExitUsage {
				t.Fatalf("exit %d, want 2: %s", code, e.stderr.String())
			}
			if w := e.bothy.Writes() - base; w != 0 {
				t.Errorf("writes = %d, want 0", w)
			}
			if n := len(e.bothy.Items(src)); n != 0 {
				t.Errorf("items = %d, want 0", n)
			}
		})
	}
}

// TestSelectionNarrows: --only-thread imports exactly that thread with the structure of its
// channel, and not the main thread.
func TestSelectionNarrows(t *testing.T) {
	e := newTestEnv(t, discordtest.BasicFixture(t))
	src := e.startSource(fixtureMapping)
	if code := e.run("--guild", fxGuild, "--map", e.writeMap(fixtureMapping), "--apply", "--only-thread", fxThread); code != ExitOK {
		t.Fatalf("exit %d: %s", code, e.stderr.String())
	}
	keys := map[string]bool{}
	for _, it := range e.bothy.Items(src) {
		keys[it.OriginKey] = true
	}
	if !keys["thread:"+fxThread] || !keys["channel:"+fxGeneral] {
		t.Errorf("items %v lack the thread or its channel", keys)
	}
	for k := range keys {
		if strings.HasPrefix(k, "channel:") && k != "channel:"+fxGeneral {
			t.Errorf("item %s is outside the selection", k)
		}
	}
	if got := e.bothy.ThreadMessages(src, "channel:"+fxGeneral); len(got) != 0 {
		t.Errorf("main thread got %v, want nothing", got)
	}
}

// TestContradictoryOptions (22 10.2 rule 3): contradictory or malformed options end with exit 2
// without any call.
func TestContradictoryOptions(t *testing.T) {
	for _, args := range [][]string{
		{"--guild", fxGuild, "--map", "x.yaml", "--takeover"},
		{"--guild", fxGuild, "--map", "x.yaml", "--report", "xml"},
		{"--guild", "abc", "--map", "x.yaml"},
		{"--guild", fxGuild, "--map", "x.yaml", "--only-thread", "12a"},
		{"--guild", fxGuild},
	} {
		e := newTestEnv(t, discordtest.BasicFixture(t))
		dBase, bBase := len(e.discord.Requests()), len(e.bothy.Requests())
		if code := e.run(args...); code != ExitUsage {
			t.Errorf("%v: exit %d, want 2", args, code)
		}
		if len(e.discord.Requests()) != dBase || len(e.bothy.Requests()) != bBase {
			t.Errorf("%v: a call was made", args)
		}
	}
}
