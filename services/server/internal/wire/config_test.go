// SPDX-License-Identifier: Apache-2.0

package wire_test

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
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
		"server.tls_cert_ref":                {Kind: config.KindSecretRef, Default: "", Chapter: "16-operations.md"},
		"server.tls_key_ref":                 {Kind: config.KindSecretRef, Default: "", Chapter: "16-operations.md"},
		"http.hsts_max_age":                  {Kind: config.KindDuration, Default: "31536000s", Min: "0s", Chapter: "19-security.md"},
		"http.max_json_body_bytes":           {Kind: config.KindInt, Default: "1048576", Min: "1", Chapter: "02-architecture.md"},
		"http.read_header_timeout":           {Kind: config.KindDuration, Default: "5s", Min: "1ms", Chapter: "02-architecture.md"},
		"http.idle_timeout":                  {Kind: config.KindDuration, Default: "120s", Min: "1ms", Chapter: "02-architecture.md"},
		"http.max_header_bytes":              {Kind: config.KindInt, Default: "65536", Min: "4096", Max: "16777216", Chapter: "02-architecture.md"},
		"http.max_json_depth":                {Kind: config.KindInt, Default: "32", Min: "1", Max: "1000", Chapter: "02-architecture.md"},
		"http.request_timeout":               {Kind: config.KindDuration, Default: "15s", Min: "1ms", Chapter: "02-architecture.md"},
		"http.transfer_timeout":              {Kind: config.KindDuration, Default: "10m", Min: "1ms", Chapter: "02-architecture.md"},
		"http.unavailable_retry_after":       {Kind: config.KindDuration, Default: "5s", Min: "1s", Max: "1h", Chapter: "02-architecture.md"},
		"file.max_upload_bytes":              {Kind: config.KindInt, Default: "104857600", Min: "1", Chapter: "03-data-model.md"},
		"file.storage.driver":                {Kind: config.KindEnum, Default: "fs", Enum: []string{"fs", "s3"}, Chapter: "12-files.md"},
		"file.fs.root":                       {Kind: config.KindString, Default: "/var/lib/bothy/files", Chapter: "12-files.md"},
		"file.s3.endpoint":                   {Kind: config.KindString, Default: "", Chapter: "12-files.md"},
		"file.s3.region":                     {Kind: config.KindString, Default: "us-east-1", Chapter: "12-files.md"},
		"file.s3.bucket":                     {Kind: config.KindString, Default: "", Chapter: "12-files.md"},
		"file.s3.prefix":                     {Kind: config.KindString, Default: "", Chapter: "12-files.md"},
		"file.s3.use_path_style":             {Kind: config.KindBool, Default: "true", Chapter: "12-files.md"},
		"file.s3.checksum_mode":              {Kind: config.KindEnum, Default: "when_required", Enum: []string{"when_required", "when_supported"}, Chapter: "12-files.md"},
		"file.s3.access_key_ref":             {Kind: config.KindSecretRef, Default: "file:<secrets.dir>/s3_access_key", Chapter: "12-files.md"},
		"file.s3.secret_key_ref":             {Kind: config.KindSecretRef, Default: "file:<secrets.dir>/s3_secret_key", Chapter: "12-files.md"},
		"api.cors_allowed_origins":           {Kind: config.KindStringList, Default: "", Chapter: "06-api.md"},
		"server.public_url":                  {Kind: config.KindURL, Required: true, Chapter: "16-operations.md"},
		"server.trusted_proxies":             {Kind: config.KindCIDRList, Default: "127.0.0.1/32, ::1/128", Chapter: "16-operations.md"},
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
		"principal.handle_max_chars":         {Kind: config.KindInt, Default: "32", Min: "1", Chapter: "03-data-model.md"},
		"principal.display_name_max_chars":   {Kind: config.KindInt, Default: "64", Min: "1", Chapter: "03-data-model.md"},
		"auth.dpop_nonce_ttl":                {Kind: config.KindDuration, Default: "5m", Min: "1s", Chapter: "08-identity.md"},
		"auth.dpop_proof_max_age":            {Kind: config.KindDuration, Default: "60s", Min: "1s", Chapter: "08-identity.md"},
		"auth.clock_leeway":                  {Kind: config.KindDuration, Default: "30s", Min: "0s", Chapter: "08-identity.md"},
		"auth.token_pepper_ref":              {Kind: config.KindSecretRef, Default: "file:<secrets.dir>/token_pepper", Chapter: "08-identity.md"},
		"api.page_size_default":              {Kind: config.KindInt, Default: "50", Min: "1", Chapter: "06-api.md"},
		"api.page_size_max":                  {Kind: config.KindInt, Default: "200", Min: "1", Chapter: "06-api.md"},
		"authz.snapshot_cache_max_entries":   {Kind: config.KindInt, Default: "10000", Min: "1", Chapter: "07-permissions.md"},
		"auth.step_up_max_age":               {Kind: config.KindDuration, Default: "5m", Min: "1s", Chapter: "03-data-model.md"},
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

		"audit.deny_aggregate_window":    {Kind: config.KindDuration, Default: "60s", Min: "1ms", Chapter: "05-actions.md"},
		"audit.deny_aggregate_max_keys":  {Kind: config.KindInt, Default: "10000", Min: "1", Chapter: "05-actions.md"},
		"privacy.audit_ip_key_period":    {Kind: config.KindDuration, Default: "30d", Min: "1s", Chapter: "20-privacy.md"},
		"privacy.audit_ip_key_retention": {Kind: config.KindDuration, Default: "90d", Min: "1s", Chapter: "20-privacy.md"},
		"privacy.audit_ip_kek_ref":       {Kind: config.KindSecretRef, Default: "file:<secrets.dir>/audit_ip_kek", Chapter: "20-privacy.md"},
		"ratelimit.max_tracked_keys":     {Kind: config.KindInt, Default: "100000", Min: "1", Max: "100000000", Chapter: "02-architecture.md"},
		"ratelimit.sweep_interval":       {Kind: config.KindDuration, Default: "1m", Min: "1s", Max: "1h", Chapter: "02-architecture.md"},

		"sync.event_log_partition_size":       {Kind: config.KindInt, Default: "1000000", Min: "1", Max: "1000000000000", Chapter: "03-data-model.md"},
		"sync.event_retention":                {Kind: config.KindDuration, Default: "30d", Min: "1s", Chapter: "04-sync-protocol.md"},
		"sync.event_payload_max_bytes":        {Kind: config.KindInt, Default: "65536", Min: "3", Max: "1048576", Chapter: "04-sync-protocol.md"},
		"sync.log_poll_interval":              {Kind: config.KindDuration, Default: "1s", Min: "1ms", Chapter: "02-architecture.md"},
		"sync.tail_batch_size":                {Kind: config.KindInt, Default: "1000", Min: "1", Max: "2147483647", Chapter: "02-architecture.md"},
		"sync.partition_maintenance_interval": {Kind: config.KindDuration, Default: "10m", Min: "1s", Chapter: "02-architecture.md"},
		"sync.idempotency_key_ttl":            {Kind: config.KindDuration, Default: "24h", Min: "1s", Chapter: "04-sync-protocol.md"},
		"sync.idempotency_cleanup_interval":   {Kind: config.KindDuration, Default: "1h", Min: "1s", Chapter: "02-architecture.md"},
		"sync.idempotency_cleanup_batch_size": {Kind: config.KindInt, Default: "1000", Min: "1", Max: "2147483647", Chapter: "02-architecture.md"},
		"sync.event_log_partitions_ahead":     {Kind: config.KindInt, Default: "3", Min: "1", Max: "1000", Chapter: "02-architecture.md"},
	}
	// Two keys per class of 05 "Rate-limit classes" (M1-19), with the defaults of 05 section 10.
	for class, def := range map[string][2]string{
		"rl.public_auth": {"30", "10"}, "rl.gateway_connect": {"12", "4"}, "rl.read": {"600", "100"},
		"rl.write": {"120", "30"}, "rl.message": {"60", "20"}, "rl.trigger": {"30", "10"},
		"rl.interaction": {"60", "20"}, "rl.upload": {"30", "10"}, "rl.search": {"60", "20"},
		"rl.typing": {"30", "5"}, "rl.admin": {"60", "20"}, "rl.export": {"2", "2"},
		"rl.agent_runtime": {"1200", "200"}, "rl.webhook_in": {"60", "20"}, "rl.import": {"120", "30"},
		"rl.terminal": {"20", "5"}, "rl.host": {"120", "30"},
	} {
		want["ratelimit."+class+".per_minute"] = config.Key{Kind: config.KindInt, Default: def[0], Min: "1", Max: "1000000", Chapter: "05-actions.md"}
		want["ratelimit."+class+".burst"] = config.Key{Kind: config.KindInt, Default: def[1], Min: "1", Max: "1000000", Chapter: "05-actions.md"}
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
		case "no default, required":
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
		// Shorthand rows of 05: "`ratelimit.rl.read.per_minute` / `.burst`" with "`600` / `100`".
		if names := strings.Split(cells[0], " / "); len(names) > 1 && column < len(cells) {
			values := strings.Split(cells[column], " / ")
			first := strings.Trim(names[0], "`")
			base := first[:strings.LastIndex(first, ".")]
			for i, n := range names {
				n = strings.Trim(n, "`")
				if i > 0 {
					n = base + n
				}
				if n == name && len(values) == len(names) {
					return values[i], true
				}
			}
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
	v, err := config.Load(schema(t), config.Sources{Env: testEnv()})
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
	if !slices.ContainsFunc(env, func(e string) bool { return strings.HasPrefix(e, "BOTHY_SERVER__PUBLIC_URL=") }) {
		env = append(testEnv(), env...)
	}
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

// testPublicURL sets server.public_url, the one key without a default (16 "Configuration values of this chapter").
const testPublicURL = "BOTHY_SERVER__PUBLIC_URL=https://chat.example.com"

// testEnv is the smallest environment that loads: the defaults plus server.public_url.
func testEnv() []string { return []string{testPublicURL} }

// TestPublicURLAndCORSOriginsAreOrigins: server.public_url and every CORS entry must be an origin
// (the chain builds the DPoP htu from it and compares the Origin header exactly).
func TestPublicURLAndCORSOriginsAreOrigins(t *testing.T) {
	t.Parallel()
	if got := load(t); got != nil {
		t.Fatalf("defaults with public_url: %v", got)
	}
	cases := map[string]string{
		"BOTHY_SERVER__PUBLIC_URL=https://chat.example.com/app":                          "server.public_url",
		"BOTHY_API__CORS_ALLOWED_ORIGINS=https://app.example.com,https://x.example.com/": "api.cors_allowed_origins",
	}
	for env, key := range cases {
		got := load(t, env)
		if len(got) != 1 || !strings.Contains(got[0].String(), key) {
			t.Errorf("%s: problems %v", env, got)
		}
	}
	if got := load(t, "BOTHY_API__CORS_ALLOWED_ORIGINS=https://app.example.com"); got != nil {
		t.Errorf("valid origin: %v", got)
	}
	_, err := config.Load(schema(t), config.Sources{})
	if err == nil || !strings.Contains(err.Error(), "server.public_url") {
		t.Errorf("without server.public_url: %v", err)
	}
}

// TestTLSKeysAndCrossChecks holds Scope 2 of brief M1-21: server.tls_cert_ref and
// server.tls_key_ref are optional secret references (env BOTHY_SERVER__TLS_CERT_REF and
// BOTHY_SERVER__TLS_KEY_REF), both set or both empty, and with TLS server.public_url uses https.
// The problems name keys and sources, never a value.
func TestTLSKeysAndCrossChecks(t *testing.T) {
	t.Parallel()
	const (
		cert = "BOTHY_SERVER__TLS_CERT_REF=file:/run/bothy/secrets/tls_cert"
		key  = "BOTHY_SERVER__TLS_KEY_REF=file:/run/bothy/secrets/tls_key"
	)
	if got := load(t); got != nil {
		t.Fatalf("no TLS: %v", got)
	}
	if got := load(t, cert, key); got != nil {
		t.Fatalf("TLS with an https public URL: %v", got)
	}
	if got := load(t, "BOTHY_SERVER__TLS_CERT_REF=", "BOTHY_SERVER__TLS_KEY_REF="); got != nil {
		t.Fatalf("both empty: %v", got)
	}
	pair := "config: server.tls_cert_ref, server.tls_key_ref: tls_pair: " +
		"server.tls_cert_ref and server.tls_key_ref must both be set or both be empty"
	for _, env := range []string{cert, key} {
		got := load(t, env)
		if len(got) != 1 || !strings.HasPrefix(got[0].String(), pair) {
			t.Errorf("%s alone: problems %v, want %s", env, got, pair)
		}
		noValues(t, got, "/run/bothy/secrets")
	}
	got := load(t, "BOTHY_SERVER__PUBLIC_URL=http://chat.example.com", cert, key)
	want := "config: server.public_url, server.tls_cert_ref, server.tls_key_ref: tls_https: " +
		"with server.tls_cert_ref and server.tls_key_ref set, server.public_url must use https"
	if len(got) != 1 || !strings.HasPrefix(got[0].String(), want) {
		t.Errorf("TLS with an http public URL: problems %v, want %s", got, want)
	}
	noValues(t, got, "chat.example.com", "/run/bothy/secrets")
}

// TestHSTSMaxAgeWholeSeconds holds http.hsts_max_age (19 "Configuration values of this chapter", SEC-TLS-3): a
// duration in whole seconds, 0 or more; max-age has no fractions.
func TestHSTSMaxAgeWholeSeconds(t *testing.T) {
	t.Parallel()
	for _, ok := range []string{"0s", "1s", "31536000s", "2m"} {
		if got := load(t, "BOTHY_HTTP__HSTS_MAX_AGE="+ok); got != nil {
			t.Errorf("%s: problems %v, want none", ok, got)
		}
	}
	for _, bad := range []string{"1500ms", "-1s", "1ms"} {
		got := load(t, "BOTHY_HTTP__HSTS_MAX_AGE="+bad)
		if len(got) != 1 || !strings.Contains(got[0].String(), "http.hsts_max_age") {
			t.Errorf("%s: problems %v, want one about http.hsts_max_age", bad, got)
		}
		noValues(t, got, bad)
	}
}

// TestAuditIPKeyRetentionAtLeastPeriod holds the cross-check of M1-24: a key must outlive its own
// period, so privacy.audit_ip_key_retention below privacy.audit_ip_key_period is refused.
func TestAuditIPKeyRetentionAtLeastPeriod(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		period, retention string
		ok                bool
	}{
		{"30d", "90d", true},
		{"30d", "30d", true},
		{"30d", "29d", false},
	} {
		got := load(t, "BOTHY_PRIVACY__AUDIT_IP_KEY_PERIOD="+c.period, "BOTHY_PRIVACY__AUDIT_IP_KEY_RETENTION="+c.retention)
		if (len(got) == 0) != c.ok {
			t.Errorf("period %s retention %s: problems %v, want ok %v", c.period, c.retention, got, c.ok)
		}
		if len(got) > 0 && !strings.Contains(got[0].String(), "privacy.audit_ip_key_retention") {
			t.Errorf("the problem does not name the key: %v", got)
		}
	}
}

// TestRateLimitConfig_InvalidValuesStopTheStart: a value below 1 never loads (exit code 2).
func TestRateLimitConfig_InvalidValuesStopTheStart(t *testing.T) {
	t.Parallel()
	for _, env := range []string{
		"BOTHY_RATELIMIT__RL__READ__PER_MINUTE=0", "BOTHY_RATELIMIT__RL__READ__BURST=0",
		"BOTHY_RATELIMIT__RL__HOST__BURST=-1", "BOTHY_RATELIMIT__MAX_TRACKED_KEYS=0",
		"BOTHY_RATELIMIT__SWEEP_INTERVAL=0s", "BOTHY_RATELIMIT__RL__TYPING__PER_MINUTE=1000001",
	} {
		if problems := load(t, env); len(problems) == 0 {
			t.Errorf("%s loaded", env)
		}
	}
}
