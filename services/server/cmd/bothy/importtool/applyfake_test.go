// SPDX-License-Identifier: Apache-2.0

package importtool

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/cmd/bothy/importtool/discord/discordtest"
)

// TestApplyAgainstFake drives an --apply run against the fake Bothy: structure first, every
// thread in Discord order, the cursor at the last message, the lease released, and a second run
// adds no message.
func TestApplyAgainstFake(t *testing.T) {
	e := newTestEnv(t, discordtest.BasicFixture(t))
	src := e.startSource(fixtureMapping)
	mapFile := e.writeMap(fixtureMapping)
	if code := e.run("--guild", fxGuild, "--map", mapFile, "--apply"); code != ExitOK {
		t.Fatalf("exit %d: %s", code, e.stderr.String())
	}
	keys := map[string]int{}
	for _, it := range e.bothy.Items(src) {
		keys[it.OriginKey]++
	}
	for _, k := range []string{"space:" + fxProjects, "space:" + fxCommunity, "channel:" + fxGeneral, "channel:" + fxSupport,
		"thread:" + fxThread, "user:13000011"} {
		if keys[k] != 1 {
			t.Errorf("item %s = %d, want 1", k, keys[k])
		}
	}
	main := e.bothy.ThreadMessages(src, "channel:"+fxGeneral)
	if len(main) == 0 || !slices.IsSortedFunc(main, func(a, b string) int {
		return compareIDs(strings.TrimPrefix(a, keyMessage), strings.TrimPrefix(b, keyMessage))
	}) {
		t.Errorf("main thread messages %v are not in Discord order", main)
	}
	cur := e.bothy.SourceCursor(src)
	if _, held := cur["run"]; held {
		t.Errorf("the lease is still held after the run: %v", cur)
	}
	if ch, _ := cur["channels"].(map[string]any); ch[fxGeneral] != "13001012" {
		t.Errorf("cursor of general = %v, want the last message", ch[fxGeneral])
	}
	if tool, _ := e.bothy.SourceStats(src)["tool"].(map[string]any); tool["messages_taken"] == nil {
		t.Errorf("stats.tool = %v, want the counters", tool)
	}
	n := len(e.bothy.Items(src))
	if code := e.run("--guild", fxGuild, "--map", mapFile, "--apply"); code != ExitOK {
		t.Fatalf("second run exit %d: %s", code, e.stderr.String())
	}
	if m := len(e.bothy.Items(src)); m != n {
		t.Errorf("the second run added %d items", m-n)
	}
	if got := e.bothy.ThreadMessages(src, "channel:"+fxGeneral); !slices.Equal(got, main) {
		t.Errorf("second run changed the main thread: %v", got)
	}
}

// TestApplyNeedsRunningSource (brief design note 3): without a source, with a paused source and
// with a mapping that differs from the stored one, --apply ends with exit 2 before any write.
func TestApplyNeedsRunningSource(t *testing.T) {
	for _, tc := range []struct {
		name, state, stored, want string
	}{
		{"no source", "", "", "create and start it in the web console"},
		{"paused", "paused", fixtureMapping, "start the source in the web console"},
		{"differs", "running", strings.Replace(fixtureMapping, `name: "news"`, `name: "updates"`, 1), "/channels/1/target/create/name"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newTestEnv(t, discordtest.BasicFixture(t))
			if tc.state != "" {
				id := e.startSource(tc.stored)
				e.bothy.SetState(id, tc.state)
			}
			base := e.bothy.Writes()
			if code := e.run("--guild", fxGuild, "--map", e.writeMap(fixtureMapping), "--apply"); code != ExitUsage {
				t.Fatalf("exit %d, want 2: %s", code, e.stderr.String())
			}
			if !strings.Contains(e.stderr.String(), tc.want) {
				t.Errorf("stderr %q does not name %q", e.stderr.String(), tc.want)
			}
			if w := e.bothy.Writes() - base; w != 0 {
				t.Errorf("writes = %d, want 0", w)
			}
		})
	}
}

// TestApplyLeaseHeld (22 7.4): a fresh lease of another run ends --apply with exit 3 before any
// batch; --takeover runs.
func TestApplyLeaseHeld(t *testing.T) {
	e := newTestEnv(t, discordtest.BasicFixture(t))
	src := e.startSource(fixtureMapping)
	mapFile := e.writeMap(fixtureMapping)
	second := -1
	e.hook = func(n int) error {
		if n == 1 {
			e.hook = nil
			second = Main(t.Context(), []string{"--guild", fxGuild, "--map", mapFile, "--apply"}, e.env())
			return errors.New("stop after the first batch")
		}
		return nil
	}
	if code := e.run("--guild", fxGuild, "--map", mapFile, "--apply"); code != ExitAborted {
		t.Fatalf("first run exit %d, want 1: %s", code, e.stderr.String())
	}
	if second != ExitLease {
		t.Errorf("second run during the lease exit %d, want 3", second)
	}
	if code := e.run("--guild", fxGuild, "--map", mapFile, "--apply", "--takeover"); code != ExitOK {
		t.Fatalf("takeover exit %d: %s", code, e.stderr.String())
	}
	if _, held := e.bothy.SourceCursor(src)["run"]; held {
		t.Error("the lease is still held after the takeover run")
	}
}
