// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

// runConfig runs "bothy config <args>" against schema with env and returns exit code, stdout and
// stderr.
func runConfig(t *testing.T, schema func() (*config.Schema, error), env []string, args ...string) (int, string, string) {
	t.Helper()
	if s, err := schema(); err == nil && slices.ContainsFunc(s.Keys(), func(k config.Key) bool { return k.Name == "server.public_url" }) {
		env = withPublicURL(env)
	}
	var stdout, stderr bytes.Buffer
	code := configCommand(t.Context(), args, env, schema, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

// secretSchema is the test schema of the brief: a reference key whose default names an env
// variable, next to the role.
func secretSchema() (*config.Schema, error) {
	return config.NewSchema([]config.Key{
		{Name: "server.role", Kind: config.KindEnum, Default: "all", Enum: []string{"api", "worker", "all"}, Doc: "Role.", Chapter: "16-operations.md"},
		{Name: "database.password_ref", Kind: config.KindSecretRef, Default: "env:BOTHY_SECRET_TEST", Doc: "Password reference.", Chapter: "16-operations.md"},
		{Name: "database.name", Kind: config.KindString, Default: "bothy", Doc: "Database name.", Chapter: "16-operations.md"},
	})
}

func TestConfigCheckOK(t *testing.T) {
	t.Parallel()
	code, stdout, stderr := runConfig(t, wire.ConfigSchema, []string{"PATH=/usr/bin"}, "check")
	if code != 0 || stdout != "configuration ok\n" || stderr != "" {
		t.Fatalf("exit %d, stdout %q, stderr %q; want 0, %q, empty", code, stdout, stderr, "configuration ok\n")
	}

	// Registered in the command table: the same through run.
	var out, errOut bytes.Buffer
	if code := run(context.Background(), []string{"config", "check", "--role", "worker"}, withPublicURL(nil), &out, &errOut); code != 0 || out.String() != "configuration ok\n" {
		t.Fatalf("run config check: exit %d, stdout %q, stderr %q", code, out.String(), errOut.String())
	}
}

func TestConfigCheckReportsProblemsExit1(t *testing.T) {
	t.Parallel()
	env := []string{"BOTHY_SERVER__ROLE=canary-9b1c", "BOTHY_SERVER__LISTEN_ADR=127.0.0.1:1"}
	code, stdout, stderr := runConfig(t, wire.ConfigSchema, env, "check")
	want := "config: unknown variable BOTHY_SERVER__LISTEN_ADR (source: environment)\n" +
		"config: server.role: must be one of api, worker, all (source: env BOTHY_SERVER__ROLE)\n"
	if code != 1 || stdout != "" || stderr != want {
		t.Fatalf("exit %d, stdout %q, stderr\n%s\nwant 1, empty and\n%s", code, stdout, stderr, want)
	}
	if strings.Contains(stderr, "canary-9b1c") {
		t.Fatal("stderr echoes the invalid value")
	}

	// A file problem names the file; the flag is checked like every source.
	path := filepath.Join(t.TempDir(), "bothy.yaml")
	if err := os.WriteFile(path, []byte("server:\n  rol: api\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	code, _, stderr = runConfig(t, wire.ConfigSchema, nil, "check", "--config", path, "--role", "nobody")
	want = `config: unknown key "server.rol" (source: file ` + path + ")\n" +
		"config: server.role: must be one of api, worker, all (source: flag --role)\n"
	if code != 1 || stderr != want {
		t.Fatalf("exit %d, stderr\n%s\nwant 1 and\n%s", code, stderr, want)
	}
}

func TestConfigUsageExit2(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{
		{},
		{"frobnicate"},
		{"check", "--nope"},
		{"check", "extra"},
		{"check", "--redacted"},
		{"print", "--role"},
		{"print", "--config"},
		{"print", "--redacted=maybe"},
		{"print", "--redacted=false"},
		{"check", "--config", ""},
		{"print", "--config="},
	} {
		code, stdout, stderr := runConfig(t, wire.ConfigSchema, nil, args...)
		if code != 2 || stdout != "" || !strings.Contains(stderr, configUsage) {
			t.Errorf("config %q: exit %d, stdout %q, stderr %q; want 2, empty, usage", args, code, stdout, stderr)
		}
		if len(args) > 0 && !strings.HasPrefix(stderr, "bothy config: ") {
			t.Errorf("config %q: stderr does not start with an error line: %q", args, stderr)
		}
	}
}

func TestConfigUsageErrorStaysOneLine(t *testing.T) {
	t.Parallel()
	// The flag package quotes no flag name. A name with a line break would forge a problem line,
	// one with an escape sequence would reach the terminal (docs/DECISIONS.md M1-07/19).
	for _, arg := range []string{
		"-x\nconfig: server.role: forged (source: flag --role)",
		"-\x1b[31mx",
		"---x\nforged",
	} {
		code, _, stderr := runConfig(t, wire.ConfigSchema, nil, "check", arg)
		first, rest, _ := strings.Cut(stderr, "\n")
		if code != 2 || !strings.HasPrefix(first, "bothy config: \"") || !strings.HasPrefix(rest, configUsage) ||
			strings.Contains(stderr, "\x1b") {
			t.Errorf("config check %q: exit %d, stderr %q; want 2, one quoted error line, usage", arg, code, stderr)
		}
	}
	// A plain flag name stays readable.
	_, _, stderr := runConfig(t, wire.ConfigSchema, nil, "check", "--nope")
	if first, _, _ := strings.Cut(stderr, "\n"); first != "bothy config: flag provided but not defined: -nope" {
		t.Errorf("error line %q", first)
	}
}

// TestConfigCheckSecretsOnPrintIsUsage holds that --secrets belongs to check only: print never
// resolves a reference, so the flag there is a usage error, not a silent no-op.
func TestConfigCheckSecretsOnPrintIsUsage(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{{"print", "--secrets"}, {"print", "--secrets=false"}, {"check", "--secrets=maybe"}} {
		code, stdout, stderr := runConfig(t, wire.ConfigSchema, nil, args...)
		if code != 2 || stdout != "" || !strings.HasPrefix(stderr, "bothy config: ") || !strings.Contains(stderr, configUsage) {
			t.Errorf("config %q: exit %d, stdout %q, stderr %q; want 2, empty, error line and usage", args, code, stdout, stderr)
		}
	}
}

func TestConfigHelpExit0(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{{"help"}, {"-h"}, {"--help"}, {"check", "-h"}, {"print", "--help"}} {
		code, stdout, stderr := runConfig(t, wire.ConfigSchema, nil, args...)
		if code != 0 || stdout != configUsage || stderr != "" {
			t.Errorf("config %q: exit %d, stdout %q, stderr %q", args, code, stdout, stderr)
		}
	}
}

func TestConfigPrintShowsSourcePerKey(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "bothy.yaml")
	if err := os.WriteFile(path, []byte("server:\n  listen_addr: 127.0.0.1:9080\nmessage:\n  max_content_bytes: 1024\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	env := []string{"BOTHY_CONFIG_FILE=" + path, "BOTHY_MESSAGE__MAX_CONTENT_BYTES=2048"}
	code, stdout, stderr := runConfig(t, wire.ConfigSchema, env, "print", "--role", "worker")
	want := "# configuration file: " + path + "\n" +
		"agent.invocation_token_ttl = 1m  # default\n" +
		"agent.jwks_max_age = 1h  # default\n" +
		"api.cors_allowed_origins =   # default\n" +
		"api.page_size_default = 50  # default\n" +
		"api.page_size_max = 200  # default\n" +
		"audit.checkpoint_interval = 1h  # default\n" +
		"audit.checkpoint_sink =   # default\n" +
		"audit.deny_aggregate_max_keys = 10000  # default\n" +
		"audit.deny_aggregate_window = 1m  # default\n" +
		"auth.access_token_ttl = 10m  # default\n" +
		"auth.api_token_max_ttl = 365d  # default\n" +
		"auth.cleanup_interval = 1h  # default\n" +
		"auth.clock_leeway = 30s  # default\n" +
		"auth.dpop_nonce_ttl = 5m  # default\n" +
		"auth.dpop_proof_max_age = 1m  # default\n" +
		"auth.dpop_required_for = agent  # default\n" +
		"auth.login_fail_window = 15m  # default\n" +
		"auth.personal_token_max_ttl = 90d  # default\n" +
		"auth.refresh_reuse_grace = 30s  # default\n" +
		"auth.refresh_token_ttl = 14d  # default\n" +
		"auth.session_max_age = 30d  # default\n" +
		"auth.step_up_challenge_ttl = 2m  # default\n" +
		"auth.step_up_fail_limit = 5  # default\n" +
		"auth.step_up_max_age = 5m  # default\n" +
		"auth.token_pepper_ref = file:/run/bothy/secrets/token_pepper  # default\n" +
		"authz.expiry_reminder_lead = 2d  # default\n" +
		"authz.expiry_sweep_batch_max = 500  # default\n" +
		"authz.expiry_sweep_interval = 1m  # default\n" +
		"authz.snapshot_cache_max_entries = 10000  # default\n" +
		"consumer.batch_size = 500  # default\n" +
		"consumer.pending_alert_after = 15m  # default\n" +
		"consumer.poll_interval = 1s  # default\n" +
		"database.acquire_timeout = 2s  # default\n" +
		"database.host = localhost  # default\n" +
		"database.idle_in_tx_timeout = 10s  # default\n" +
		"database.listen_reconnect_backoff_cap = 10s  # default\n" +
		"database.lock_timeout = 2s  # default\n" +
		"database.max_conns = 20  # default\n" +
		"database.name = bothy  # default\n" +
		"database.password_ref = file:/run/bothy/secrets/db_app_password  # default\n" +
		"database.port = 5432  # default\n" +
		"database.sslmode = verify-full  # default\n" +
		"database.sslrootcert =   # default\n" +
		"database.statement_timeout = 5s  # default\n" +
		"database.tx_retry_max = 2  # default\n" +
		"database.user = bothy_app  # default\n" +
		"egress.allowed_hosts =   # default\n" +
		"egress.max_response_bytes = 1048576  # default\n" +
		"egress.timeout = 15s  # default\n" +
		"file.fs.root = /var/lib/bothy/files  # default\n" +
		"file.max_upload_bytes = 104857600  # default\n" +
		"file.s3.access_key_ref = file:/run/bothy/secrets/s3_access_key  # default\n" +
		"file.s3.bucket =   # default\n" +
		"file.s3.checksum_mode = when_required  # default\n" +
		"file.s3.endpoint =   # default\n" +
		"file.s3.prefix =   # default\n" +
		"file.s3.region = us-east-1  # default\n" +
		"file.s3.secret_key_ref = file:/run/bothy/secrets/s3_secret_key  # default\n" +
		"file.s3.use_path_style = true  # default\n" +
		"file.storage.driver = fs  # default\n" +
		"http.hsts_max_age = 365d  # default\n" +
		"http.idle_timeout = 2m  # default\n" +
		"http.max_header_bytes = 65536  # default\n" +
		"http.max_json_body_bytes = 1048576  # default\n" +
		"http.max_json_depth = 32  # default\n" +
		"http.read_header_timeout = 5s  # default\n" +
		"http.request_timeout = 15s  # default\n" +
		"http.transfer_timeout = 10m  # default\n" +
		"http.unavailable_retry_after = 5s  # default\n" +
		"identity.last_active_granularity = 5m  # default\n" +
		"instance.environment = prod  # default\n" +
		"jobs.queue.default.max_workers = 8  # default\n" +
		"jobs.queue.delivery.max_workers = 16  # default\n" +
		"jobs.queue.files.max_workers = 2  # default\n" +
		"jobs.queue.maintenance.max_workers = 2  # default\n" +
		"jobs.queue.privacy.max_workers = 1  # default\n" +
		"jobs.schema = jobs  # default\n" +
		"message.max_content_bytes = 2048  # env BOTHY_MESSAGE__MAX_CONTENT_BYTES\n" +
		"migrate.app_password_ref = file:/run/bothy/secrets/db_app_password  # default\n" +
		"migrate.bootstrap_password_ref = file:/run/bothy/secrets/db_superuser_password  # default\n" +
		"migrate.bootstrap_user = postgres  # default\n" +
		"migrate.lock_timeout = 5s  # default\n" +
		"migrate.maint_password_ref = file:/run/bothy/secrets/db_maint_password  # default\n" +
		"migrate.monitor_password_ref = file:/run/bothy/secrets/db_monitor_password  # default\n" +
		"migrate.owner_password_ref = file:/run/bothy/secrets/db_owner_password  # default\n" +
		"principal.display_name_max_chars = 64  # default\n" +
		"principal.handle_max_chars = 32  # default\n" +
		"privacy.audit_ip_kek_ref = file:/run/bothy/secrets/audit_ip_kek  # default\n" +
		"privacy.audit_ip_key_period = 30d  # default\n" +
		"privacy.audit_ip_key_retention = 90d  # default\n" +
		"privacy.telemetry_pseudonym_rotation = 30d  # default\n" +
		"ratelimit.max_tracked_keys = 100000  # default\n" +
		"ratelimit.rl.admin.burst = 20  # default\n" +
		"ratelimit.rl.admin.per_minute = 60  # default\n" +
		"ratelimit.rl.agent_runtime.burst = 200  # default\n" +
		"ratelimit.rl.agent_runtime.per_minute = 1200  # default\n" +
		"ratelimit.rl.export.burst = 2  # default\n" +
		"ratelimit.rl.export.per_minute = 2  # default\n" +
		"ratelimit.rl.gateway_connect.burst = 4  # default\n" +
		"ratelimit.rl.gateway_connect.per_minute = 12  # default\n" +
		"ratelimit.rl.host.burst = 30  # default\n" +
		"ratelimit.rl.host.per_minute = 120  # default\n" +
		"ratelimit.rl.import.burst = 30  # default\n" +
		"ratelimit.rl.import.per_minute = 120  # default\n" +
		"ratelimit.rl.interaction.burst = 20  # default\n" +
		"ratelimit.rl.interaction.per_minute = 60  # default\n" +
		"ratelimit.rl.message.burst = 20  # default\n" +
		"ratelimit.rl.message.per_minute = 60  # default\n" +
		"ratelimit.rl.public_auth.burst = 10  # default\n" +
		"ratelimit.rl.public_auth.per_minute = 30  # default\n" +
		"ratelimit.rl.read.burst = 100  # default\n" +
		"ratelimit.rl.read.per_minute = 600  # default\n" +
		"ratelimit.rl.search.burst = 20  # default\n" +
		"ratelimit.rl.search.per_minute = 60  # default\n" +
		"ratelimit.rl.terminal.burst = 5  # default\n" +
		"ratelimit.rl.terminal.per_minute = 20  # default\n" +
		"ratelimit.rl.trigger.burst = 10  # default\n" +
		"ratelimit.rl.trigger.per_minute = 30  # default\n" +
		"ratelimit.rl.typing.burst = 5  # default\n" +
		"ratelimit.rl.typing.per_minute = 30  # default\n" +
		"ratelimit.rl.upload.burst = 10  # default\n" +
		"ratelimit.rl.upload.per_minute = 30  # default\n" +
		"ratelimit.rl.webhook_in.burst = 20  # default\n" +
		"ratelimit.rl.webhook_in.per_minute = 60  # default\n" +
		"ratelimit.rl.write.burst = 30  # default\n" +
		"ratelimit.rl.write.per_minute = 120  # default\n" +
		"ratelimit.sweep_interval = 1m  # default\n" +
		"secrets.cache_ttl = 5m  # default\n" +
		"secrets.dir = /run/bothy/secrets  # default\n" +
		"secrets.signing_key_dir = /var/lib/bothy/signing-keys  # default\n" +
		"server.internal_listen_addr = 127.0.0.1:8081  # default\n" +
		"server.listen_addr = 127.0.0.1:9080  # file " + path + "\n" +
		"server.public_url = https://chat.example.com  # env BOTHY_SERVER__PUBLIC_URL\n" +
		"server.role = worker  # flag --role\n" +
		"server.tls_cert_ref =   # default\n" +
		"server.tls_key_ref =   # default\n" +
		"server.trusted_proxies = 127.0.0.1/32, ::1/128  # default\n" +
		"shutdown.drain_delay = 5s  # default\n" +
		"shutdown.http_grace = 20s  # default\n" +
		"shutdown.jobs_soft_timeout = 20s  # default\n" +
		"shutdown.telemetry_flush_timeout = 5s  # default\n" +
		"shutdown.total_timeout = 1m  # default\n" +
		"signing_key.access_token_rotation_interval = 30d  # default\n" +
		"signing_key.cache_ttl = 30s  # default\n" +
		"signing_key.invocation_rotation_interval = 30d  # default\n" +
		"signing_key.rotation_check_interval = 1m  # default\n" +
		"startup.dependency_wait = 1m  # default\n" +
		"sync.event_log_partition_size = 1000000  # default\n" +
		"sync.event_log_partitions_ahead = 3  # default\n" +
		"sync.event_payload_max_bytes = 65536  # default\n" +
		"sync.event_retention = 30d  # default\n" +
		"sync.idempotency_cleanup_batch_size = 1000  # default\n" +
		"sync.idempotency_cleanup_interval = 1h  # default\n" +
		"sync.idempotency_key_ttl = 1d  # default\n" +
		"sync.log_poll_interval = 1s  # default\n" +
		"sync.partition_maintenance_interval = 10m  # default\n" +
		"sync.tail_batch_size = 1000  # default\n" +
		"telemetry.enabled = true  # default\n" +
		"telemetry.logs.exporter = both  # default\n" +
		"telemetry.logs.level = info  # default\n" +
		"telemetry.pseudonym_key_ref = file:/run/bothy/secrets/telemetry_pseudonym_key  # default\n" +
		"telemetry.traces.sampler_ratio = 1  # default\n" +
		"telemetry.traces.trust_remote_parent = false  # default\n" +
		"webhook.delivery_timeout = 15s  # default\n"
	if code != 0 || stdout != want || stderr != "" {
		t.Fatalf("exit %d, stderr %q, stdout\n%s\nwant\n%s", code, stderr, stdout, want)
	}

	code, stdout, _ = runConfig(t, wire.ConfigSchema, nil, "print")
	if code != 0 || !strings.HasPrefix(stdout, "# configuration file: none\n") {
		t.Fatalf("without a file: exit %d, stdout\n%s", code, stdout)
	}

	// Problems go to stderr with exit 1, nothing to stdout.
	code, stdout, stderr = runConfig(t, wire.ConfigSchema, []string{"BOTHY_HTTP__MAX_JSON_BODY_BYTES=1"}, "print")
	if code != 1 || stdout != "" || !strings.Contains(stderr, "json_limit_above_message") {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, stdout, stderr)
	}
}

func TestConfigPrintShowsReferenceOnly(t *testing.T) {
	t.Parallel()
	env := []string{"BOTHY_SECRET_TEST=canary-9b1c"}
	code, stdout, stderr := runConfig(t, secretSchema, env, "print", "--redacted")
	if code != 0 || !strings.Contains(stdout, "database.password_ref = env:BOTHY_SECRET_TEST  # default\n") {
		t.Fatalf("exit %d, stderr %q, stdout\n%s", code, stderr, stdout)
	}
	// Set from the environment as well; check prints nothing about the secret either.
	env = append(env, "BOTHY_DATABASE__PASSWORD_REF=env:BOTHY_SECRET_TEST")
	code2, stdout2, stderr2 := runConfig(t, secretSchema, env, "print")
	code3, stdout3, stderr3 := runConfig(t, secretSchema, env, "check")
	if code2 != 0 || code3 != 0 || !strings.Contains(stdout2, "database.password_ref = env:BOTHY_SECRET_TEST  # env BOTHY_DATABASE__PASSWORD_REF\n") {
		t.Fatalf("exit %d/%d, stdout\n%s", code2, code3, stdout2)
	}
	for _, out := range []string{stdout, stderr, stdout2, stderr2, stdout3, stderr3} {
		if strings.Contains(out, "canary-9b1c") {
			t.Fatalf("output contains the secret:\n%s", out)
		}
	}
}

func TestConfigPrintRedactedFlagAccepted(t *testing.T) {
	t.Parallel()
	env := []string{"BOTHY_SERVER__ROLE=api"}
	code1, plain, _ := runConfig(t, wire.ConfigSchema, env, "print")
	code2, redacted, stderr := runConfig(t, wire.ConfigSchema, env, "print", "--redacted")
	if code1 != 0 || code2 != 0 || stderr != "" || plain != redacted {
		t.Fatalf("print and print --redacted differ: exit %d/%d\n%s\n---\n%s", code1, code2, plain, redacted)
	}
}

func TestConfigPrintQuotesUnprintableText(t *testing.T) {
	t.Parallel()
	env := []string{"BOTHY_DATABASE__NAME=bothy\nserver.role = api  # flag --role"}
	code, stdout, _ := runConfig(t, secretSchema, env, "print")
	want := `database.name = "bothy\nserver.role = api  # flag --role"  # env BOTHY_DATABASE__NAME` + "\n"
	if code != 0 || !strings.Contains(stdout, want) || strings.Count(stdout, "\n") != 4 {
		t.Fatalf("exit %d, stdout\n%s\nwant a line\n%s", code, stdout, want)
	}
	for text, quoted := range map[string]bool{"plain": false, " lead": true, "trail ": true, `"q"`: true, "tab\t": true, "": false, "ñïçé": false, "bad\xffbyte": true} {
		if got := printable(text) != text; got != quoted {
			t.Errorf("printable(%q) quoted = %v, want %v", text, got, quoted)
		}
	}
}

func TestConfigSchemaErrorExit1(t *testing.T) {
	t.Parallel()
	broken := func() (*config.Schema, error) { return nil, errors.New("config schema: key \"x\": broken") }
	code, stdout, stderr := runConfig(t, broken, nil, "check")
	if code != 1 || stdout != "" || stderr != "bothy config: config schema: key \"x\": broken\n" {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, stdout, stderr)
	}
}

// testPublicURL sets server.public_url, the one key of the server schema without a default.
const testPublicURL = "BOTHY_SERVER__PUBLIC_URL=https://chat.example.com"

// withPublicURL returns env with server.public_url set unless env sets it already.
func withPublicURL(env []string) []string {
	if slices.ContainsFunc(env, func(e string) bool { return strings.HasPrefix(e, "BOTHY_SERVER__PUBLIC_URL=") }) {
		return env
	}
	return append([]string{testPublicURL}, env...)
}

// TestSinkConfigRejectsUnknownScheme: bothy config check refuses an audit.checkpoint_sink with a
// scheme other than file:// and names the supported scheme, never the value (19 section 16,
// M1-50); file:// with an absolute path and the empty value pass.
func TestSinkConfigRejectsUnknownScheme(t *testing.T) {
	t.Parallel()
	code, stdout, stderr := runConfig(t, wire.ConfigSchema, []string{"BOTHY_AUDIT__CHECKPOINT_SINK=s3://bucket/prefix"}, "check")
	if code != 1 || stdout != "" || !strings.Contains(stderr, "audit.checkpoint_sink") || !strings.Contains(stderr, "file://") {
		t.Fatalf("exit %d, stdout %q, stderr %q; want 1 and a problem naming audit.checkpoint_sink and file://", code, stdout, stderr)
	}
	if strings.Contains(stderr, "bucket") {
		t.Errorf("the problem echoes the value: %q", stderr)
	}
	for _, ok := range []string{"file:///var/lib/bothy-checkpoints", ""} {
		if code, _, stderr := runConfig(t, wire.ConfigSchema, []string{"BOTHY_AUDIT__CHECKPOINT_SINK=" + ok}, "check"); code != 0 {
			t.Errorf("%q: exit %d, stderr %q", ok, code, stderr)
		}
	}
}
