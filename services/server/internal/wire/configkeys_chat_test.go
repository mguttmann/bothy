// SPDX-License-Identifier: Apache-2.0

package wire_test

import (
	"slices"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/chat/domain/title"
	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

// TestConfigKey_PlaceholderTitles (brief M2-18 note 3): thread.placeholder_titles is a
// list(string) of chapter 09 whose default is rendered from the word lists; a configured list
// reaches the chat Config unchanged and replaces the default.
func TestConfigKey_PlaceholderTitles(t *testing.T) {
	t.Parallel()
	i := slices.IndexFunc(schema(t).Keys(), func(k config.Key) bool { return k.Name == "thread.placeholder_titles" })
	if i < 0 {
		t.Fatal("thread.placeholder_titles is not registered")
	}
	k := schema(t).Keys()[i]
	if k.Kind != config.KindStringList || k.Chapter != "09-agents.md" || k.Default != title.DefaultConfigText() {
		t.Fatalf("key %+v", k)
	}
	for env, want := range map[string][]string{
		"": title.DefaultEntries(),
		"BOTHY_THREAD__PLACEHOLDER_TITLES=Draft, Sketch": {"Draft", "Sketch"},
	} {
		e := testEnv()
		if env != "" {
			e = append(e, env)
		}
		v, err := config.Load(schema(t), config.Sources{Env: e})
		if err != nil {
			t.Fatal(err)
		}
		c, err := wire.ChatConfig(v)
		if err != nil || !slices.Equal(c.PlaceholderTitles, want) {
			t.Fatalf("%q: %q, %v", env, c.PlaceholderTitles, err)
		}
	}
}
