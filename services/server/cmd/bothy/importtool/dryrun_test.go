// SPDX-License-Identifier: Apache-2.0

package importtool

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/cmd/bothy/importtool/bothyapi/bothytest"
	"github.com/OWNER/bothy/services/server/cmd/bothy/importtool/discord/discordtest"
)

// emptyDir fails the test when dir holds any entry.
func emptyDir(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		t.Errorf("%s holds %s after the dry run", dir, e.Name())
	}
}

// TestDryRunOnlyGET (22 16 row "Dry run only GET", 22 10.2 rules 1 and 5, R-IM-02, REUSE 4.7):
// with a fresh access token both fakes log only GET; with an expired one Bothy additionally logs
// exactly one refreshToken; the working directory and TMPDIR stay empty; the session file changes
// only in the refresh case; the CDN logs no request.
func TestDryRunOnlyGET(t *testing.T) {
	for _, expired := range []bool{false, true} {
		name := "fresh token"
		if expired {
			name = "expired token"
		}
		t.Run(name, func(t *testing.T) {
			e := newTestEnv(t, discordtest.BasicFixture(t))
			e.startSource(fixtureMapping)
			mapFile := e.writeMap(fixtureMapping)
			work, tmp := t.TempDir(), t.TempDir()
			t.Chdir(work)
			t.Setenv("TMPDIR", tmp)
			before, err := os.ReadFile(e.session)
			if err != nil {
				t.Fatal(err)
			}
			if expired {
				e.bothy.ExpireAccess()
			}
			base := len(e.bothy.Requests())
			if code := e.run("--guild", fxGuild, "--map", mapFile); code != ExitOK {
				t.Fatalf("exit %d: %s", code, e.stderr.String())
			}
			refresh := 0
			for _, r := range e.bothy.Requests()[base:] {
				switch {
				case r.Method == http.MethodGet:
				case r.Method == http.MethodPost && r.Path == bothytest.PathRefresh:
					refresh++
				default:
					t.Errorf("Bothy got %s %s in a dry run", r.Method, r.Path)
				}
			}
			if want := map[bool]int{false: 0, true: 1}[expired]; refresh != want {
				t.Errorf("refreshToken calls = %d, want %d", refresh, want)
			}
			for m := range e.discordMethods() {
				if m != http.MethodGet {
					t.Errorf("Discord got %s", m)
				}
			}
			if n := len(e.cdn.Requests()); n != 0 {
				t.Errorf("CDN requests = %d, want 0", n)
			}
			emptyDir(t, work)
			emptyDir(t, tmp)
			after, err := os.ReadFile(e.session)
			if err != nil {
				t.Fatal(err)
			}
			if changed := !bytes.Equal(before, after); changed != expired {
				t.Errorf("session file changed = %v, want %v", changed, expired)
			}
			if !strings.Contains(e.stdout.String(), "would take over") {
				t.Errorf("the report does not say would take over:\n%s", e.stdout.String())
			}
		})
	}
}

// TestDryRunReportJSON: --report json prints one object with a version member and counts that
// follow the fixture.
func TestDryRunReportJSON(t *testing.T) {
	e := newTestEnv(t, discordtest.BasicFixture(t))
	if code := e.run("--guild", fxGuild, "--map", e.writeMap(fixtureMapping), "--report", "json"); code != ExitOK {
		t.Fatalf("exit %d: %s", code, e.stderr.String())
	}
	var out struct {
		Version  int    `json:"version"`
		Mode     string `json:"mode"`
		Channels []struct {
			Channel string         `json:"channel"`
			Counts  map[string]any `json:"counts"`
		} `json:"channels"`
		Total           map[string]any `json:"total"`
		ChannelsSkipped map[string]int `json:"channels_skipped"`
	}
	if err := json.Unmarshal([]byte(e.stdout.String()), &out); err != nil {
		t.Fatalf("report is not one JSON object: %v\n%s", err, e.stdout.String())
	}
	if out.Version != reportVersion || out.Mode != "dry run" {
		t.Errorf("version %d mode %q", out.Version, out.Mode)
	}
	if out.ChannelsSkipped[skipVoice] != 1 {
		t.Errorf("channels_skipped = %v, want the voice channel", out.ChannelsSkipped)
	}
	if got := out.Total["messages_taken"].(float64); got == 0 {
		t.Errorf("messages_taken = %v, want some", got)
	}
}
