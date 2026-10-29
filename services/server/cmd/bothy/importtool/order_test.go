// SPDX-License-Identifier: Apache-2.0

package importtool

import (
	"cmp"
	"slices"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/cmd/bothy/importtool/bothyapi/bothytest"
	"github.com/OWNER/bothy/services/server/cmd/bothy/importtool/discord/discordtest"
)

// sidebarOrder returns the names in the order of the sidebar: sort key as bytes, then ID.
func sidebarOrder(placed []bothytest.Placed) []string {
	slices.SortStableFunc(placed, func(a, b bothytest.Placed) int {
		if c := strings.Compare(a.SortKey, b.SortKey); c != 0 {
			return c
		}
		return cmp.Compare(a.ID, b.ID)
	})
	out := make([]string, 0, len(placed))
	for _, p := range placed {
		out = append(out, p.Name)
	}
	return out
}

// keysByName returns the sort key of every placed object by name.
func keysByName(placed []bothytest.Placed) map[string]string {
	out := map[string]string{}
	for _, p := range placed {
		out[p.Name] = p.SortKey
	}
	return out
}

// applyRun runs --apply with the mapping text against a running source and fails on another exit.
func (e *testEnv) applyRun(text string) {
	e.t.Helper()
	if code := e.run("--guild", fxGuild, "--map", e.writeMap(text), "--apply"); code != ExitOK {
		e.t.Fatalf("exit %d: %s", code, e.stderr.String())
	}
}

// TestNewSpacesFollowExistingSpaces: on a server that already has spaces, the new spaces form one
// block after the last existing sort key, in Discord order, instead of interleaving with them.
func TestNewSpacesFollowExistingSpaces(t *testing.T) {
	e := newTestEnv(t, discordtest.BasicFixture(t))
	e.bothy.AddSpace("Alpha")
	beta := e.bothy.AddSpace("Beta")
	e.bothy.SetSpaceSortKey(beta, "a5")
	e.startSource(fixtureMapping)
	e.applyRun(fixtureMapping)
	want := []string{"Alpha", "Beta", "Projects", "Community"}
	if got := sidebarOrder(e.bothy.Spaces()); !slices.Equal(got, want) {
		t.Errorf("space order = %v, want %v", got, want)
	}
	keys := keysByName(e.bothy.Spaces())
	if keys["Projects"] <= "a5" || keys["Community"] <= keys["Projects"] {
		t.Errorf("keys %v: the new spaces are not after a5 in Discord order", keys)
	}
}

// TestNewChannelsFollowExistingChannels: new channels in an existing target space follow its last
// channel in Discord order; the channels of a new space keep Discord order among themselves.
func TestNewChannelsFollowExistingChannels(t *testing.T) {
	e := newTestEnv(t, discordtest.BasicFixture(t))
	existing := e.bothy.AddSpace("Existing")
	e.bothy.AddChannel(existing, "one")
	e.bothy.AddChannel(existing, "two")
	text := strings.Replace(fixtureMapping, `{create: {name: "Projects"}}`, `{existing: "`+existing+`"}`, 1)
	e.startSource(text)
	e.applyRun(text)
	// general (position 0), support (2) and random (4, no category, uncategorized_space Projects).
	want := []string{"one", "two", "general", "support", "random"}
	if got := sidebarOrder(e.bothy.Channels(existing)); !slices.Equal(got, want) {
		t.Errorf("channel order = %v, want %v", got, want)
	}
	if got := sidebarOrder(e.bothy.Spaces()); !slices.Equal(got, []string{"Existing", "Community"}) {
		t.Errorf("space order = %v, want Existing, Community", got)
	}
}

// TestCategoriesSharingExistingSpaceKeepDiscordOrder: two categories mapped to one existing space
// draw their channels from one block after its last channel, in Discord order, instead of each
// category starting its own block at the same key.
func TestCategoriesSharingExistingSpaceKeepDiscordOrder(t *testing.T) {
	e := newTestEnv(t, discordtest.BasicFixture(t))
	existing := e.bothy.AddSpace("Existing")
	e.bothy.AddChannel(existing, "one")
	text := strings.Replace(fixtureMapping, `{create: {name: "Projects"}}`, `{existing: "`+existing+`"}`, 1)
	text = strings.Replace(text, `{create: {name: "Community"}}`, `{existing: "`+existing+`"}`, 1)
	e.startSource(text)
	e.applyRun(text)
	placed := e.bothy.Channels(existing)
	keys := keysByName(placed)
	seen := map[string]string{}
	for name, k := range keys {
		if other, dup := seen[k]; dup {
			t.Errorf("%s and %s share the sort key %s", other, name, k)
		}
		seen[k] = name
	}
	// Discord order of the fixture: general (0), news (1), support (2), random (4).
	want := []string{"one", "general", "news", "support", "random"}
	if got := sidebarOrder(placed); !slices.Equal(got, want) {
		t.Errorf("channel order = %v, want %v", got, want)
	}
}

// TestRerunKeepsPlacedOrder: a second run after the server order changed moves nothing the first
// run placed.
func TestRerunKeepsPlacedOrder(t *testing.T) {
	e := newTestEnv(t, discordtest.BasicFixture(t))
	alpha := e.bothy.AddSpace("Alpha")
	e.startSource(fixtureMapping)
	e.applyRun(fixtureMapping)
	before := keysByName(e.bothy.Spaces())
	e.bothy.SetSpaceSortKey(alpha, "b00")
	e.applyRun(fixtureMapping)
	after := keysByName(e.bothy.Spaces())
	for _, name := range []string{"Projects", "Community"} {
		if after[name] != before[name] {
			t.Errorf("rerun moved %s from %s to %s", name, before[name], after[name])
		}
	}
	if n := len(e.bothy.Spaces()); n != 3 {
		t.Errorf("spaces after the rerun = %d, want 3", n)
	}
}

// TestNextKey checks the integer step after a key against the vectors of the fractional index
// scheme of the server (docs/decisions.d/M1-52.md and the vectors of fracindex_test.go in the chat module).
func TestNextKey(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"", "a0"},
		{"a0", "a1"},
		{"a0V", "a1"},
		{"az", "b00"},
		{"b0z", "b10"},
		{"bzz", "c000"},
		{"Zz", "a0"},
		{"Yzz", "Z0"},
		{"A" + strings.Repeat("0", 26) + "V", "A" + strings.Repeat("0", 25) + "1"},
	} {
		got, err := nextKey(tc.in)
		if err != nil || got != tc.want {
			t.Errorf("nextKey(%q) = %q, %v; want %q", tc.in, got, err, tc.want)
		}
	}
	for _, bad := range []string{"z" + strings.Repeat("z", 26), "a", "a0!", "a00"} {
		if _, err := nextKey(bad); err == nil {
			t.Errorf("nextKey(%q) has no error", bad)
		}
	}
}

// TestLastKey: the greatest valid key wins; invalid keys are left out.
func TestLastKey(t *testing.T) {
	if got := lastKey([]string{"a1", "b10", "a9", "not a key"}); got != "b10" {
		t.Errorf("lastKey = %q, want b10", got)
	}
	if got := lastKey(nil); got != "" {
		t.Errorf("lastKey(nil) = %q, want empty", got)
	}
}
