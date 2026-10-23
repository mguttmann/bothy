// SPDX-License-Identifier: Apache-2.0

package importtool

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/cmd/bothy/importtool/discord/discordtest"
)

// The distinctive values of the secret test: message text, file name and display names that must
// never reach an output.
const (
	secretText     = "zq-content-marker-7731 about the invoice"
	secretFile     = "zq-attachment-name-5521.pdf"
	secretDisplay  = "Sam Zq-Display-Marker" // #nosec G101 -- an invented test marker
	secretUsername = "zq-username-marker"
	secretPath     = "/attachments/13000103/13009001/" + secretFile
)

// secretFixture returns the basic fixture with a message of a distinctive text and author and a
// message with an attachment on the fake CDN whose link carries the signed query.
func secretFixture(t *testing.T, cdn *discordtest.CDN, f *discordtest.Fixture) {
	t.Helper()
	text, err := json.Marshal(map[string]any{"id": "13001013", "channel_id": fxGeneral, "type": 0,
		"author":  map[string]any{"id": "13000017", "username": secretUsername, "global_name": secretDisplay, "bot": false},
		"content": secretText + " <@13000017>", "timestamp": "2026-03-01T10:01:00.000000+00:00", "pinned": true,
		"mentions": []any{map[string]any{"id": "13000017", "username": secretUsername, "global_name": secretDisplay}}})
	if err != nil {
		t.Fatal(err)
	}
	link := cdn.URL(discordtest.CDNHost, secretPath, discordtest.SignedQuery)
	f.Messages[fxGeneral] = append(f.Messages[fxGeneral], text,
		discordtest.AttachmentMessage("13001014", fxGeneral, "13009001", secretFile, 64, link))
}

// TestNoSecretInOutput (22 16 row "No secret in output", 22 10.2 rule 4, REUSE 4.3): with fakes
// that echo the token and a signed query into headers and error bodies, and a fixture with
// distinctive message texts, file names and display names, none of them appears in standard
// output, standard error, the report (text and JSON) or the debug log, in a dry run and in an
// --apply run that fails at its end.
func TestNoSecretInOutput(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
	}{
		{"dry run text", nil},
		{"dry run json", []string{"--report", "json"}},
		{"apply text", []string{"--apply"}},
		{"apply json", []string{"--apply", "--report", "json"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := discordtest.BasicFixture(t)
			e := newTestEnv(t, f)
			secretFixture(t, e.cdn, f)
			e.discord.Always(discordtest.AnyPath, discordtest.Echo(0))
			e.cdn.Always(secretPath, discordtest.CDNEcho(http.StatusForbidden))
			e.bothy.Echo(true)
			src := e.startSource(fixtureMapping)
			apply := len(tc.args) > 0 && tc.args[0] == "--apply"
			// After the batch of the main thread of general, the next stored cursor fails with an
			// echoed body, so the apply run ends with an error text.
			e.hook = func(int) error {
				if len(e.bothy.ThreadMessages(src, "channel:"+fxGeneral)) > 0 {
					e.bothy.Fail("/api/v1/imports/"+src+"/cursor", http.StatusInternalServerError, "", "", 1)
				}
				return nil
			}
			args := append([]string{"--guild", fxGuild, "--map", e.writeMap(fixtureMapping)}, tc.args...)
			code := e.run(args...)
			if want := map[bool]int{false: ExitOK, true: ExitAborted}[apply]; code != want {
				t.Fatalf("exit %d, want %d: %s", code, want, e.stderr.String())
			}
			if apply {
				if len(e.cdn.Requests()) == 0 {
					t.Errorf("the attachment was never fetched: %s", e.debug.String())
				}
				if d := e.bothy.MessageData(src, "message:13001013"); !strings.Contains(d["content_md"].(string), secretText) {
					t.Errorf("the distinctive message was not imported: %v", d)
				}
				if d := e.bothy.MessageData(src, "message:13001014"); !strings.Contains(d["content_md"].(string), secretFile) {
					t.Errorf("the failed attachment has no line: %v", d)
				}
			}
			outputs := map[string]string{"stdout": e.stdout.String(), "stderr": e.stderr.String(), "debug log": e.debug.String()}
			secrets := []string{testToken, discordtest.SignedQuery, "zq-content-marker", secretFile, "zq-attachment-name",
				secretDisplay, "Zq-Display", secretUsername, "Alex", "Robin"}
			if r := e.bothy.CurrentRefresh(); r != "" {
				secrets = append(secrets, r)
			}
			for where, out := range outputs {
				for _, s := range secrets {
					if strings.Contains(out, s) {
						t.Errorf("%s contains %q:\n%s", where, s, out)
					}
				}
			}
		})
	}
}
