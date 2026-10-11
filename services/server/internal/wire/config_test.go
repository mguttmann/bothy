// SPDX-License-Identifier: Apache-2.0

package wire_test

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

func schema(t *testing.T) *config.Schema {
	t.Helper()
	s, err := wire.ConfigSchema()
	if err != nil {
		t.Fatalf("ConfigSchema: %v", err)
	}
	return s
}

func TestConfigSchemaBuilds(t *testing.T) {
	t.Parallel()
	got := map[string]config.Key{}
	for _, k := range schema(t).Keys() {
		got[k.Name] = k
	}
	want := map[string]config.Key{
		"server.role":                 {Kind: config.KindEnum, Default: "all", Enum: []string{"api", "worker", "all"}, Chapter: "16-betrieb.md"},
		"server.listen_addr":          {Kind: config.KindListenAddr, Default: "127.0.0.1:8080", Chapter: "16-betrieb.md"},
		"server.internal_listen_addr": {Kind: config.KindListenAddr, Default: "127.0.0.1:8081", Chapter: "16-betrieb.md"},
		"http.max_json_body_bytes":    {Kind: config.KindInt, Default: "1048576", Min: "1", Chapter: "02-architektur.md"},
		"message.max_content_bytes":   {Kind: config.KindInt, Default: "262144", Min: "1", Chapter: "03-datenmodell.md"},
		"egress.allowed_hosts":        {Kind: config.KindStringList, Default: "", Chapter: "19-sicherheit.md"},
		"egress.timeout":              {Kind: config.KindDuration, Default: "15s", Min: "1ms", Chapter: "19-sicherheit.md"},
		"egress.max_response_bytes":   {Kind: config.KindInt, Default: "1048576", Min: "1", Chapter: "19-sicherheit.md"},
		"secrets.dir":                 {Kind: config.KindString, Default: "/run/bothy/secrets", Chapter: "16-betrieb.md"},
		"secrets.cache_ttl":           {Kind: config.KindDuration, Default: "5m", Min: "1s", Chapter: "16-betrieb.md"},
	}
	if len(got) != len(want) {
		t.Errorf("schema has %d keys, want %d", len(got), len(want))
	}
	for name, w := range want {
		g, ok := got[name]
		if !ok {
			t.Errorf("key %s missing", name)
			continue
		}
		if g.Doc == "" || !strings.HasSuffix(g.Doc, ".") {
			t.Errorf("%s: Doc %q is not one sentence", name, g.Doc)
		}
		g.Name, g.Doc = "", ""
		if !reflect.DeepEqual(g, w) {
			t.Errorf("%s = %+v, want %+v", name, g, w)
		}
	}
}

// TestConfigKeysMatchChapterTables holds fixed requirement 9 of the brief: every key in the
// schema has a row in the table "Konfigurationswerte dieses Kapitels" of its chapter with the
// same default (M1-08 later checks all keys of all chapters).
func TestConfigKeysMatchChapterTables(t *testing.T) {
	t.Parallel()
	for _, k := range schema(t).Keys() {
		cell, ok := chapterDefault(t, k.Chapter, k.Name)
		if !ok {
			t.Errorf("%s: no row in the table of %s", k.Name, k.Chapter)
			continue
		}
		want := strings.ReplaceAll(cell, "`", "")
		switch cell {
		case "leer":
			want = ""
		case "keiner, Pflicht":
			want = ""
			if !k.Required {
				t.Errorf("%s: chapter says required, schema does not", k.Name)
			}
		}
		if k.Default != want {
			t.Errorf("%s: schema default %q, chapter %s says %q", k.Name, k.Default, k.Chapter, cell)
		}
	}
}

// chapterDefault returns the cell of the column "Standard" in the row of name in the section
// "Konfigurationswerte dieses Kapitels" of chapter.
func chapterDefault(t *testing.T, chapter, name string) (string, bool) {
	t.Helper()
	text := readSpec(t, chapter)
	start := strings.Index(text, "Konfigurationswerte dieses Kapitels\n")
	if start < 0 {
		t.Fatalf("%s has no section Konfigurationswerte dieses Kapitels", chapter)
	}
	column := -1
	for _, line := range strings.Split(text[start:], "\n") {
		if !strings.HasPrefix(line, "|") {
			continue
		}
		cells := strings.Split(strings.Trim(line, "|"), "|")
		for i := range cells {
			cells[i] = strings.TrimSpace(cells[i])
		}
		if column < 0 {
			for i, c := range cells {
				if c == "Standard" {
					column = i
				}
			}
			continue
		}
		if cells[0] == "`"+name+"`" && column < len(cells) {
			return cells[column], true
		}
	}
	return "", false
}

func readSpec(t *testing.T, chapter string) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if info, err := os.Stat(filepath.Join(dir, "docs", "spec")); err == nil && info.IsDir() {
			root, err := os.OpenRoot(filepath.Join(dir, "docs", "spec"))
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = root.Close() }()
			data, err := root.ReadFile(chapter)
			if err != nil {
				t.Fatal(err)
			}
			return string(data)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("no directory with docs/spec above the working directory")
		}
		dir = parent
	}
}

func TestDefaultsPassCrossChecks(t *testing.T) {
	t.Parallel()
	v, err := config.Load(schema(t), config.Sources{})
	if err != nil {
		t.Fatalf("Load with defaults only: %v", err)
	}
	if v.String("server.role") != "all" {
		t.Errorf("server.role = %q", v.String("server.role"))
	}
}

// load returns the problems of loading the server schema with env (nil when it loads).
func load(t *testing.T, env ...string) []config.Problem {
	t.Helper()
	_, err := config.Load(schema(t), config.Sources{Env: env})
	if err == nil {
		return nil
	}
	var le *config.LoadError
	if !errors.As(err, &le) {
		t.Fatalf("Load error %T", err)
	}
	return le.Problems
}

func TestListenersDistinct(t *testing.T) {
	t.Parallel()
	cases := []struct {
		public, internal string
		conflict         bool
	}{
		{"127.0.0.1:8080", "127.0.0.1:8080", true},
		{":8080", "127.0.0.1:8080", true},
		{"127.0.0.1:8080", ":8080", true},
		{"[::]:8080", "127.0.0.1:8080", true},
		{"0.0.0.0:8080", "[::1]:8080", true},
		{"[::ffff:127.0.0.1]:8080", "127.0.0.1:8080", true},
		{"[::1]:8080", "[::1]:8080", true},
		{"[::%eth0]:8080", "127.0.0.1:8080", true},
		{"127.0.0.1:8080", "[::%1]:8080", true},
		{"[fe80::1%eth0]:8080", "[fe80::1%eth0]:8080", true},
		{"[fe80::1%eth0]:8080", "[fe80::1%eth1]:8080", false},
		{"127.0.0.1:0", "127.0.0.1:0", false},
		{":0", ":0", false},
		{"127.0.0.1:8080", "127.0.0.1:8081", false},
		{"127.0.0.1:8080", "127.0.0.2:8080", false},
		{"127.0.0.1:8080", "[::1]:8080", false},
	}
	for _, c := range cases {
		got := load(t, "BOTHY_SERVER__LISTEN_ADDR="+c.public, "BOTHY_SERVER__INTERNAL_LISTEN_ADDR="+c.internal)
		if !c.conflict {
			if got != nil {
				t.Errorf("%s and %s: problems %v, want none", c.public, c.internal, got)
			}
			continue
		}
		want := "config: server.listen_addr, server.internal_listen_addr: listeners_distinct: " +
			"the public and the internal listener must not share a port on the same or an unspecified address; only port 0 may repeat " +
			"(source: server.listen_addr from env BOTHY_SERVER__LISTEN_ADDR, server.internal_listen_addr from env BOTHY_SERVER__INTERNAL_LISTEN_ADDR)"
		if len(got) != 1 || got[0].String() != want {
			t.Errorf("%s and %s: problems %v, want\n%s", c.public, c.internal, got, want)
		}
		noValues(t, got, c.public, c.internal)
	}
}

func TestJSONLimitAboveMessage(t *testing.T) {
	t.Parallel()
	cases := []struct {
		json, message string
		ok            bool
	}{
		{"262144", "262144", false},
		{"262143", "262144", false},
		{"262145", "262144", true},
		{"1048576", "1048577", false},
	}
	for _, c := range cases {
		got := load(t, "BOTHY_HTTP__MAX_JSON_BODY_BYTES="+c.json, "BOTHY_MESSAGE__MAX_CONTENT_BYTES="+c.message)
		if c.ok {
			if got != nil {
				t.Errorf("%s > %s: problems %v", c.json, c.message, got)
			}
			continue
		}
		want := "config: http.max_json_body_bytes, message.max_content_bytes: json_limit_above_message: " +
			"http.max_json_body_bytes must be greater than message.max_content_bytes " +
			"(source: http.max_json_body_bytes from env BOTHY_HTTP__MAX_JSON_BODY_BYTES, message.max_content_bytes from env BOTHY_MESSAGE__MAX_CONTENT_BYTES)"
		if len(got) != 1 || got[0].String() != want {
			t.Errorf("%s vs %s: problems %v, want\n%s", c.json, c.message, got, want)
		}
		noValues(t, got, c.json, c.message)
	}
}

// TestSecretsDirCleanAbsolute holds that bothy config check rejects every secrets.dir the secret
// provider rejects (absolute and clean like a file: reference, not the root, without NUL and
// "<"): the loader checks the key itself (docs/DECISIONS.md M1-09/15), so config check reports
// what the start would.
func TestSecretsDirCleanAbsolute(t *testing.T) {
	t.Parallel()
	const (
		notAbsolute = "must be an absolute path"
		hasNUL      = "must not contain a NUL character"
		isRoot      = "must not be the root directory"
		hasAngle    = "must not contain <"
		notClean    = "must be a clean path without . or .. elements and without repeated or trailing slashes"
	)
	cases := []struct {
		dir, want string
	}{
		{"/run/bothy/secrets", ""},
		{"/srv/bothy-secrets", ""},
		{"", notAbsolute},
		{"run/bothy/secrets", notAbsolute},
		{"./secrets", notAbsolute},
		{"/", isRoot},
		{"/run/bothy/\x00secrets", hasNUL},
		{"/run/<secrets.dir>", hasAngle},
		{"/run/bothy/secrets/", notClean},
		{"/run//bothy/secrets", notClean},
		{"/run/bothy/./secrets", notClean},
		{"/run/bothy/../secrets", notClean},
	}
	for _, c := range cases {
		got := load(t, "BOTHY_SECRETS__DIR="+c.dir)
		if c.want == "" {
			if got != nil {
				t.Errorf("secrets.dir %q: problems %v, want none", c.dir, got)
			}
			continue
		}
		want := "config: secrets.dir: " + c.want + " (source: env BOTHY_SECRETS__DIR)"
		if len(got) != 1 || got[0].String() != want {
			t.Errorf("secrets.dir %q: problems %v, want\n%s", c.dir, got, want)
		}
		if c.dir != "" && c.dir != "/" {
			noValues(t, got, c.dir)
		}
	}
}

// TestSecretsCacheTTLAtLeastOneSecond holds the lower bound of secrets.cache_ttl
// (docs/DECISIONS.md M1-09/24): with 0 every call would read the secret, and while a refresh
// fails every call would log a line and count a failure instead of one per TTL.
func TestSecretsCacheTTLAtLeastOneSecond(t *testing.T) {
	t.Parallel()
	if got := load(t, "BOTHY_SECRETS__CACHE_TTL=1s"); got != nil {
		t.Errorf("1s: problems %v, want none", got)
	}
	got := load(t, "BOTHY_SECRETS__CACHE_TTL=0s")
	if want := "config: secrets.cache_ttl: must be at least 1s (source: env BOTHY_SECRETS__CACHE_TTL)"; len(got) != 1 || got[0].String() != want {
		t.Errorf("0s: problems %v, want\n%s", got, want)
	}
}

func noValues(t *testing.T, problems []config.Problem, values ...string) {
	t.Helper()
	text := (&config.LoadError{Problems: problems}).Error()
	for _, v := range values {
		if strings.Contains(text, v) {
			t.Errorf("problem text echoes the value %q:\n%s", v, text)
		}
	}
}
