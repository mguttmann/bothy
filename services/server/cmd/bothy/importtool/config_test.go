// SPDX-License-Identifier: Apache-2.0

package importtool

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/platform/config"
)

// writeConfigFile writes text into a fresh file below t.TempDir and returns its path.
func writeConfigFile(t *testing.T, text string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "import.yaml")
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// wantLoadError asserts that err is a *config.LoadError and returns its text.
func wantLoadError(t *testing.T, err error) string {
	t.Helper()
	var le *config.LoadError
	if !errors.As(err, &le) {
		t.Fatalf("want a *config.LoadError, got %T %v", err, err)
	}
	return le.Error()
}

// TestDefaultConfigEqualsLoadWithoutSources: the defaults of 22 19 are what LoadConfig returns
// without a file and without variables.
func TestDefaultConfigEqualsLoadWithoutSources(t *testing.T) {
	t.Parallel()
	got, err := LoadConfig("", nil)
	if err != nil {
		t.Fatal(err)
	}
	want := DefaultConfig()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("LoadConfig(\"\", nil)\n got %+v\nwant %+v", got, want)
	}
	spot := Config{
		DiscordAPIBase:         "https://discord.com/api/v10",
		DiscordCDNHosts:        []string{"cdn.discordapp.com", "media.discordapp.net"},
		DiscordPageSize:        100,
		DiscordMax429Retries:   5,
		DiscordMaxRateWait:     time.Minute,
		DiscordPreflightSample: 20,
		DiscordClosedTags:      []string{},
		AttachmentMaxBytes:     0,
		HTTPTimeout:            time.Minute,
		LoginPollInterval:      5 * time.Second,
		BatchMaxItems:          100,
		ReplyQuoteMaxChars:     100,
		FollowPollInterval:     30 * time.Second,
		FollowSettleAfter:      time.Minute,
		FollowBusyReactions:    []string{"\U0001F440"},
		FollowBusyLookback:     10,
		FollowMaxBackoff:       5 * time.Minute,
		LeaseStaleAfter:        90 * time.Second,
	}
	if !reflect.DeepEqual(want, spot) {
		t.Errorf("DefaultConfig\n got %+v\nwant %+v", want, spot)
	}
}

// TestConfigSchemaHoldsExactlyTheChapterKeys: the schema has the keys of 22 19, all of chapter
// 22, and nothing outside import.*.
func TestConfigSchemaHoldsExactlyTheChapterKeys(t *testing.T) {
	t.Parallel()
	s, err := ConfigSchema()
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"import.attachment_max_bytes", "import.batch_max_items", "import.discord.api_base",
		"import.discord.cdn_hosts", "import.discord.closed_tags", "import.discord.max_429_retries",
		"import.discord.max_rate_wait", "import.discord.page_size", "import.discord.preflight_sample",
		"import.follow.busy_lookback", "import.follow.busy_reactions", "import.follow.max_backoff",
		"import.follow.poll_interval", "import.follow.settle_after", "import.http_timeout",
		"import.lease_stale_after", "import.login_poll_interval", "import.reply_quote_max_chars",
	}
	var got []string
	for _, k := range s.Keys() {
		got = append(got, k.Name)
		if k.Chapter != "22-import-migration.md" {
			t.Errorf("%s: chapter %q", k.Name, k.Chapter)
		}
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("keys\n got %v\nwant %v", got, want)
	}
}

// TestConfigPrecedence: the environment wins over the file, the file over the default; one key
// per source.
func TestConfigPrecedence(t *testing.T) {
	t.Parallel()
	file := writeConfigFile(t, "import:\n  batch_max_items: 7\n  follow:\n    poll_interval: 20s\n")
	env := []string{"BOTHY_IMPORT__FOLLOW__POLL_INTERVAL=40s", "BOTHY_IMPORT__DISCORD__PAGE_SIZE=50"}
	got, err := LoadConfig(file, env)
	if err != nil {
		t.Fatal(err)
	}
	if got.BatchMaxItems != 7 {
		t.Errorf("file over default: batch_max_items %d", got.BatchMaxItems)
	}
	if got.FollowPollInterval != 40*time.Second {
		t.Errorf("environment over file: follow.poll_interval %v", got.FollowPollInterval)
	}
	if got.DiscordPageSize != 50 {
		t.Errorf("environment over default: discord.page_size %d", got.DiscordPageSize)
	}
	if got.ReplyQuoteMaxChars != DefaultConfig().ReplyQuoteMaxChars {
		t.Errorf("default: reply_quote_max_chars %d", got.ReplyQuoteMaxChars)
	}
}

// TestConfigFileShapeErrors: a flat dot key and a key outside import.* are errors of the file.
func TestConfigFileShapeErrors(t *testing.T) {
	t.Parallel()
	cases := []struct{ name, text string }{
		{"flat dot key", "import.batch_max_items: 5\n"},
		{"key outside import", "server:\n  listen_addr: \"127.0.0.1:0\"\n"},
		{"unknown import key", "import:\n  batch_size: 5\n"},
	}
	for _, c := range cases {
		_, err := LoadConfig(writeConfigFile(t, c.text), nil)
		if err == nil {
			t.Errorf("%s: no error", c.name)
			continue
		}
		wantLoadError(t, err)
	}
}

// TestConfigIgnoresForeignVariables: a server variable on the same machine neither aborts the
// tool nor names a file, and the token variable is not a key.
func TestConfigIgnoresForeignVariables(t *testing.T) {
	t.Parallel()
	missing := filepath.Join(t.TempDir(), "absent.yaml")
	env := []string{
		"BOTHY_DATABASE__URL=postgres://db.example.com/bothy",
		"BOTHY_CONFIG_FILE=" + missing,
		"BOTHY_IMPORT_DISCORD_TOKEN=invented-token",
		"HOME=/nonexistent",
	}
	got, err := LoadConfig("", env)
	if err != nil {
		t.Fatalf("foreign variables: %v", err)
	}
	if !reflect.DeepEqual(got, DefaultConfig()) {
		t.Errorf("foreign variables changed the values: %+v", got)
	}
}

// TestConfigUnknownImportVariable: a variable with the prefix of the tool keys that names no key
// is an error (a typo must not pass silently).
func TestConfigUnknownImportVariable(t *testing.T) {
	t.Parallel()
	_, err := LoadConfig("", []string{"BOTHY_IMPORT__BATCH_SIZE=5"})
	if err == nil {
		t.Fatal("no error")
	}
	wantLoadError(t, err)
}

// TestConfigAttachmentMaxBytes: "" means the server limit (0), a positive integer is taken, any
// other text is an error that does not repeat the value.
func TestConfigAttachmentMaxBytes(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		text string
		want int64
	}{{"", 0}, {"1024", 1024}} {
		got, err := LoadConfig("", []string{"BOTHY_IMPORT__ATTACHMENT_MAX_BYTES=" + c.text})
		if err != nil {
			t.Errorf("%q: %v", c.text, err)
			continue
		}
		if got.AttachmentMaxBytes != c.want {
			t.Errorf("%q: got %d, want %d", c.text, got.AttachmentMaxBytes, c.want)
		}
	}
	for _, bad := range []string{"x7secretish", "000", "-5", "1e3", "99999999999999999999"} {
		_, err := LoadConfig("", []string{"BOTHY_IMPORT__ATTACHMENT_MAX_BYTES=" + bad})
		if err == nil {
			t.Errorf("%q: no error", bad)
			continue
		}
		if msg := wantLoadError(t, err); strings.Contains(msg, bad) {
			t.Errorf("%q: the error repeats the value: %s", bad, msg)
		}
	}
}

// TestConfigLeaseLongerThanPoll: import.lease_stale_after must be larger than
// import.follow.poll_interval (22 19), else the lease would age between two renewals.
func TestConfigLeaseLongerThanPoll(t *testing.T) {
	t.Parallel()
	for _, lease := range []string{"30s", "10s"} {
		_, err := LoadConfig("", []string{"BOTHY_IMPORT__LEASE_STALE_AFTER=" + lease})
		if err == nil {
			t.Errorf("lease %s with poll 30s: no error", lease)
			continue
		}
		wantLoadError(t, err)
	}
	if _, err := LoadConfig("", []string{"BOTHY_IMPORT__LEASE_STALE_AFTER=31s"}); err != nil {
		t.Errorf("lease 31s: %v", err)
	}
}

// TestConfigBounds: the bounds of the schema hold (page size at most what Discord allows).
func TestConfigBounds(t *testing.T) {
	t.Parallel()
	for _, v := range []string{
		"BOTHY_IMPORT__DISCORD__PAGE_SIZE=101", "BOTHY_IMPORT__BATCH_MAX_ITEMS=0",
		"BOTHY_IMPORT__HTTP_TIMEOUT=0", "BOTHY_IMPORT__DISCORD__MAX_429_RETRIES=-1",
	} {
		if _, err := LoadConfig("", []string{v}); err == nil {
			t.Errorf("%s: no error", v)
		}
	}
	got, err := LoadConfig("", []string{"BOTHY_IMPORT__DISCORD__MAX_429_RETRIES=0"})
	if err != nil || got.DiscordMax429Retries != 0 {
		t.Errorf("max_429_retries 0: %v %d", err, got.DiscordMax429Retries)
	}
}

// TestConfigListsFromFile: list keys read a YAML sequence in the file.
func TestConfigListsFromFile(t *testing.T) {
	t.Parallel()
	file := writeConfigFile(t, "import:\n  discord:\n    closed_tags: [done, solved]\n    cdn_hosts: [cdn.example.com]\n")
	got, err := LoadConfig(file, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.DiscordClosedTags, []string{"done", "solved"}) ||
		!reflect.DeepEqual(got.DiscordCDNHosts, []string{"cdn.example.com"}) {
		t.Errorf("lists %v %v", got.DiscordClosedTags, got.DiscordCDNHosts)
	}
}
