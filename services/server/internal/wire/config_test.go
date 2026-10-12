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
		"server.role":                        {Kind: config.KindEnum, Default: "all", Enum: []string{"api", "worker", "all"}, Chapter: "16-operations.md"},
		"server.listen_addr":                 {Kind: config.KindListenAddr, Default: "127.0.0.1:8080", Chapter: "16-operations.md"},
		"server.internal_listen_addr":        {Kind: config.KindListenAddr, Default: "127.0.0.1:8081", Chapter: "16-operations.md"},
		"http.max_json_body_bytes":           {Kind: config.KindInt, Default: "1048576", Min: "1", Chapter: "02-architecture.md"},
		"http.read_header_timeout":           {Kind: config.KindDuration, Default: "5s", Min: "1ms", Chapter: "02-architecture.md"},
		"http.idle_timeout":                  {Kind: config.KindDuration, Default: "120s", Min: "1ms", Chapter: "02-architecture.md"},
		"http.max_header_bytes":              {Kind: config.KindInt, Default: "65536", Min: "4096", Max: "16777216", Chapter: "02-architecture.md"},
		"shutdown.drain_delay":               {Kind: config.KindDuration, Default: "5s", Min: "0s", Max: "1h", Chapter: "02-architecture.md"},
		"shutdown.http_grace":                {Kind: config.KindDuration, Default: "20s", Min: "1ms", Max: "1h", Chapter: "02-architecture.md"},
		"shutdown.telemetry_flush_timeout":   {Kind: config.KindDuration, Default: "5s", Min: "1ms", Max: "1h", Chapter: "02-architecture.md"},
		"shutdown.total_timeout":             {Kind: config.KindDuration, Default: "60s", Min: "1s", Max: "1h", Chapter: "02-architecture.md"},
		"shutdown.jobs_soft_timeout":         {Kind: config.KindDuration, Default: "20s", Min: "1ms", Max: "1h", Chapter: "02-architecture.md"},
		"jobs.schema":                        {Kind: config.KindString, Default: "jobs", Chapter: "02-architecture.md"},
		"jobs.queue.default.max_workers":     {Kind: config.KindInt, Default: "8", Min: "1", Max: "10000", Chapter: "02-architecture.md"},
		"jobs.queue.files.max_workers":       {Kind: config.KindInt, Default: "2", Min: "1", Max: "10000", Chapter: "02-architecture.md"},
		"jobs.queue.delivery.max_workers":    {Kind: config.KindInt, Default: "16", Min: "1", Max: "10000", Chapter: "02-architecture.md"},
		"jobs.queue.maintenance.max_workers": {Kind: config.KindInt, Default: "2", Min: "1", Max: "10000", Chapter: "02-architecture.md"},
		"jobs.queue.privacy.max_workers":     {Kind: config.KindInt, Default: "1", Min: "1", Max: "10000", Chapter: "02-architecture.md"},
		"message.max_content_bytes":          {Kind: config.KindInt, Default: "262144", Min: "1", Chapter: "03-data-model.md"},
		"egress.allowed_hosts":               {Kind: config.KindStringList, Default: "", Chapter: "19-security.md"},
		"egress.timeout":                     {Kind: config.KindDuration, Default: "15s", Min: "1ms", Chapter: "19-security.md"},
		"egress.max_response_bytes":          {Kind: config.KindInt, Default: "1048576", Min: "1", Chapter: "19-security.md"},
		"secrets.dir":                        {Kind: config.KindString, Default: "/run/bothy/secrets", Chapter: "16-operations.md"},
		"secrets.cache_ttl":                  {Kind: config.KindDuration, Default: "5m", Min: "1s", Chapter: "16-operations.md"},

		"database.host":                         {Kind: config.KindString, Default: "localhost", Chapter: "16-operations.md"},
		"database.port":                         {Kind: config.KindInt, Default: "5432", Min: "1", Max: "65535", Chapter: "16-operations.md"},
		"database.name":                         {Kind: config.KindString, Default: "bothy", Chapter: "16-operations.md"},
		"database.user":                         {Kind: config.KindString, Default: "bothy_app", Chapter: "16-operations.md"},
		"database.password_ref":                 {Kind: config.KindSecretRef, Default: "file:<secrets.dir>/db_app_password", Chapter: "16-operations.md"},
		"database.sslmode":                      {Kind: config.KindEnum, Default: "verify-full", Enum: []string{"disable", "verify-full"}, Chapter: "16-operations.md"},
		"database.sslrootcert":                  {Kind: config.KindString, Default: "", Chapter: "16-operations.md"},
		"database.max_conns":                    {Kind: config.KindInt, Default: "20", Min: "1", Max: "2147483647", Chapter: "16-operations.md"},
		"database.acquire_timeout":              {Kind: config.KindDuration, Default: "2s", Min: "1ms", Chapter: "02-architecture.md"},
		"database.statement_timeout":            {Kind: config.KindDuration, Default: "5s", Min: "1ms", Max: "24d", Chapter: "02-architecture.md"},
		"database.lock_timeout":                 {Kind: config.KindDuration, Default: "2s", Min: "1ms", Max: "24d", Chapter: "02-architecture.md"},
		"database.idle_in_tx_timeout":           {Kind: config.KindDuration, Default: "10s", Min: "1ms", Max: "24d", Chapter: "02-architecture.md"},
		"database.tx_retry_max":                 {Kind: config.KindInt, Default: "2", Min: "0", Chapter: "02-architecture.md"},
		"database.listen_reconnect_backoff_cap": {Kind: config.KindDuration, Default: "10s", Min: "1ms", Chapter: "02-architecture.md"},
		"startup.dependency_wait":               {Kind: config.KindDuration, Default: "60s", Min: "0s", Chapter: "02-architecture.md"},
		"migrate.lock_timeout":                  {Kind: config.KindDuration, Default: "5s", Min: "1ms", Max: "24d", Chapter: "16-operations.md"},
		"migrate.bootstrap_user":                {Kind: config.KindString, Default: "postgres", Chapter: "16-operations.md"},
		"migrate.bootstrap_password_ref":        {Kind: config.KindSecretRef, Default: "file:<secrets.dir>/db_superuser_password", Chapter: "16-operations.md"},
		"migrate.owner_password_ref":            {Kind: config.KindSecretRef, Default: "file:<secrets.dir>/db_owner_password", Chapter: "16-operations.md"},
		"migrate.app_password_ref":              {Kind: config.KindSecretRef, Default: "file:<secrets.dir>/db_app_password", Chapter: "16-operations.md"},
		"migrate.maint_password_ref":            {Kind: config.KindSecretRef, Default: "file:<secrets.dir>/db_maint_password", Chapter: "16-operations.md"},
		"migrate.monitor_password_ref":          {Kind: config.KindSecretRef, Default: "file:<secrets.dir>/db_monitor_password", Chapter: "16-operations.md"},

		"instance.environment":                 {Kind: config.KindEnum, Default: "prod", Enum: []string{"dev", "staging", "prod"}, Chapter: "16-operations.md"},
		"telemetry.enabled":                    {Kind: config.KindBool, Default: "true", Chapter: "17-observability.md"},
		"telemetry.traces.sampler_ratio":       {Kind: config.KindFloat, Default: "1.0", Min: "0", Max: "1", Chapter: "17-observability.md"},
		"telemetry.traces.trust_remote_parent": {Kind: config.KindBool, Default: "false", Chapter: "17-observability.md"},
		"telemetry.logs.exporter":              {Kind: config.KindEnum, Default: "both", Enum: []string{"otlp", "stdout", "both"}, Chapter: "17-observability.md"},
		"telemetry.logs.level":                 {Kind: config.KindEnum, Default: "info", Enum: []string{"debug", "info", "warn", "error"}, Chapter: "17-observability.md"},
		"telemetry.pseudonym_key_ref":          {Kind: config.KindSecretRef, Default: "file:<secrets.dir>/telemetry_pseudonym_key", Chapter: "17-observability.md"},
		"privacy.telemetry_pseudonym_rotation": {Kind: config.KindDuration, Default: "30d", Min: "1s", Chapter: "20-privacy.md"},
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
// schema has a row in the table "Configuration values of this chapter" of its chapter with the
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
		case "empty":
			want = ""
		case "none, required":
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

// chapterDefault returns the cell of the column "Default" in the row of name in the section
// "Configuration values of this chapter" of chapter.
func chapterDefault(t *testing.T, chapter, name string) (string, bool) {
	t.Helper()
	text := readSpec(t, chapter)
	start := strings.Index(text, "Configuration values of this chapter\n")
	if start < 0 {
		t.Fatalf("%s has no section Configuration values of this chapter", chapter)
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
				if c == "Default" {
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

// TestJobsSchemaIdentifier: jobs.schema must be an unquoted identifier other than public and
// without the prefix pg_ (cross-check jobs_schema); the problem names the key, never the value.
func TestJobsSchemaIdentifier(t *testing.T) {
	t.Parallel()
	for _, ok := range []string{"jobs", "_jobs", "jobs_it2", strings.Repeat("a", 63)} {
		if problems := load(t, "BOTHY_JOBS__SCHEMA="+ok); problems != nil {
			t.Errorf("jobs.schema %q: %v", ok, problems)
		}
	}
	for _, bad := range []string{"public", "information_schema", "pg_jobs", "Jobs", "2jobs", "jobs-x", `jobs"x`, "jobs.x", strings.Repeat("a", 64)} {
		problems := load(t, "BOTHY_JOBS__SCHEMA="+bad)
		if len(problems) != 1 || !strings.Contains(problems[0].Message, "jobs.schema") || strings.Contains(problems[0].Message, bad) {
			t.Errorf("jobs.schema %q: problems %v, want one naming the key and not the value", bad, problems)
		}
	}
}
