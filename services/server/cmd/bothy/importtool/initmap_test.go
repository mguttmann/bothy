// SPDX-License-Identifier: Apache-2.0

package importtool

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/cmd/bothy/importtool/discord/discordtest"
)

// TestInitMap (22 5.2, brief design note 10): init-map writes exactly the file --out, a mapping
// that parses, with every category and channel created under its Discord name, the voice channel
// as a comment and every author as a placeholder without a name; a second call refuses to
// overwrite it; the server is never called.
func TestInitMap(t *testing.T) {
	e := newTestEnv(t, discordtest.BasicFixture(t))
	dir := t.TempDir()
	out := filepath.Join(dir, "map.yaml")
	base := len(e.bothy.Requests())
	if code := e.run("init-map", "--guild", fxGuild, "--out", out); code != ExitOK {
		t.Fatalf("exit %d: %s", code, e.stderr.String())
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("dir holds %v (%v), want only the mapping", entries, err)
	}
	data, err := os.ReadFile(out) // #nosec G304 -- a path below t.TempDir()
	if err != nil {
		t.Fatal(err)
	}
	m, err := ParseMapping(data, fxGuild)
	if err != nil {
		t.Fatalf("the skeleton does not parse: %v\n%s", err, data)
	}
	if len(m.Spaces) != 2 || m.Channel(fxGeneral) == nil || m.Channel(fxGeneral).Target.CreateName != "general" {
		t.Errorf("skeleton %+v", m)
	}
	if m.Channel(fxLounge) != nil || !strings.Contains(string(data), "# voice or stage channel \""+fxLounge+"\"") {
		t.Errorf("the voice channel is not a comment:\n%s", data)
	}
	if u := m.User("13000016"); u == nil || u.Kind != kindPlaceholder {
		t.Errorf("author 13000016 = %+v, want a placeholder", u)
	}
	for _, name := range []string{"Alex", "Sam", "Robin", "alex", "helper-bot", "Deploy hook"} {
		if strings.Contains(string(data), name) {
			t.Errorf("the skeleton names a person: %q", name)
		}
	}
	if !strings.Contains(string(data), "# bot") || !strings.Contains(string(data), "# webhook") {
		t.Errorf("the skeleton lacks the author kind comments:\n%s", data)
	}
	if len(e.bothy.Requests()) != base {
		t.Error("init-map called the server")
	}
	for m := range e.discordMethods() {
		if m != http.MethodGet {
			t.Errorf("Discord got %s", m)
		}
	}
	if code := e.run("init-map", "--guild", fxGuild, "--out", out); code != ExitUsage {
		t.Errorf("second init-map exit %d, want 2", code)
	}
}

// TestStatus (22 10.1): status prints the source, its state, the ownership list by Discord
// channel ID and the counters, and no name.
func TestStatus(t *testing.T) {
	e := newTestEnv(t, discordtest.BasicFixture(t))
	src := e.startSource(fixtureMapping)
	if code := e.run("--guild", fxGuild, "--map", e.writeMap(fixtureMapping), "--apply"); code != ExitOK {
		t.Fatalf("apply exit %d: %s", code, e.stderr.String())
	}
	if code := e.run("status", "--guild", fxGuild); code != ExitOK {
		t.Fatalf("status exit %d: %s", code, e.stderr.String())
	}
	out := e.stdout.String()
	for _, want := range []string{src, "State: running", fxGeneral + ": discord", "tool.messages_taken"} {
		if !strings.Contains(out, want) {
			t.Errorf("status lacks %q:\n%s", want, out)
		}
	}
	for _, name := range []string{"general", "Projects", "Alex", "Example"} {
		if strings.Contains(out, name) {
			t.Errorf("status names %q:\n%s", name, out)
		}
	}
}
