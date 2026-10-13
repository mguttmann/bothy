// SPDX-License-Identifier: Apache-2.0

package config_test

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/platform/config"
)

// canary is a value no message may ever contain (brief M1-07, TestTypeErrorNeverEchoesValue).
const canary = "canary-7f3a"

// allKinds returns a schema with one key of every kind and no required key.
func allKinds(t *testing.T) *config.Schema {
	t.Helper()
	bounded := key("http.max_json_body_bytes", config.KindInt, "1048576")
	bounded.Min = "1"
	retention := key("sync.event_retention", config.KindDuration, "30d")
	retention.Min, retention.Max = "1d", "3650d"
	ratio := key("status.error_ratio_degraded", config.KindFloat, "0.05")
	ratio.Min, ratio.Max = "0", "1"
	return mustSchema(t,
		enumKey(),
		key("server.listen_addr", config.KindListenAddr, "127.0.0.1:8080"),
		bounded, retention, ratio,
		key("secrets.dir", config.KindString, "/run/bothy/secrets"),
		key("database.password_ref", config.KindSecretRef, "file:<secrets.dir>/db_app_password"),
		key("server.trusted_proxies", config.KindCIDRList, "127.0.0.1/32, ::1/128"),
		key("egress.allowed_hosts", config.KindStringList, ""),
		key("auth.local_password_enabled", config.KindBool, "true"),
		key("backup.pg.full_schedule", config.KindCron, "0 2 * * 0"),
		key("instance.min_server_version", config.KindSemVer, "0.1.0"),
		key("webhook.callback_url", config.KindURL, "https://hooks.example.com/in"),
	)
}

// writeFile writes content to a file in a fresh temporary directory and returns its path.
func writeFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "bothy.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// problems loads and returns the problems; it fails the test when Load succeeds or returns
// another error type.
func problems(t *testing.T, s *config.Schema, src config.Sources) []config.Problem {
	t.Helper()
	v, err := config.Load(s, src)
	if err == nil {
		t.Fatalf("Load succeeded, want problems")
	}
	if v != nil {
		t.Errorf("Load returned values together with an error")
	}
	var le *config.LoadError
	if !errors.As(err, &le) {
		t.Fatalf("Load error is %T, want *config.LoadError", err)
	}
	return le.Problems
}

// mustLoad loads or stops the test.
func mustLoad(t *testing.T, s *config.Schema, src config.Sources) *config.Values {
	t.Helper()
	v, err := config.Load(s, src)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return v
}

// wantProblem checks that exactly one problem renders as want.
func wantProblem(t *testing.T, got []config.Problem, want string) {
	t.Helper()
	var lines []string
	for _, p := range got {
		lines = append(lines, p.String())
	}
	if len(got) != 1 || lines[0] != want {
		t.Fatalf("problems =\n%s\nwant exactly\n%s", strings.Join(lines, "\n"), want)
	}
}

func TestLoadDefaults(t *testing.T) {
	t.Parallel()
	v := mustLoad(t, allKinds(t), config.Sources{Env: []string{"PATH=/usr/bin", "OTEL_SERVICE_NAME=bothy"}})
	if got := v.String("server.role"); got != "all" {
		t.Errorf("server.role = %q", got)
	}
	if got := v.String("database.password_ref"); got != "file:/run/bothy/secrets/db_app_password" {
		t.Errorf("database.password_ref = %q, want the default with <secrets.dir> expanded", got)
	}
	if got := v.Duration("sync.event_retention"); got != 30*24*time.Hour {
		t.Errorf("sync.event_retention = %v", got)
	}
	if got := v.Source("server.role"); got != (config.Source{Kind: config.SourceDefault}) || got.String() != "default" {
		t.Errorf("Source(server.role) = %+v (%s)", got, got)
	}
	if v.File() != "" {
		t.Errorf("File() = %q, want empty without a file", v.File())
	}
}

func TestPrecedenceDefaultFileEnvFlag(t *testing.T) {
	t.Parallel()
	s := allKinds(t)
	file := writeFile(t, "server:\n  role: worker\n")
	env := []string{"BOTHY_SERVER__ROLE=api"}
	flags := map[string]string{"server.role": "all"}

	v := mustLoad(t, s, config.Sources{File: file, Env: env, Flags: flags})
	if v.String("server.role") != "all" || v.Source("server.role") != (config.Source{Kind: config.SourceFlag, Name: "--role"}) {
		t.Errorf("with flag: %q from %s, want all from flag --role", v.String("server.role"), v.Source("server.role"))
	}
	if got := v.Source("server.role").String(); got != "flag --role" {
		t.Errorf("Source().String() = %q, want %q", got, "flag --role")
	}

	v = mustLoad(t, s, config.Sources{File: file, Env: env})
	if v.String("server.role") != "api" || v.Source("server.role").String() != "env BOTHY_SERVER__ROLE" {
		t.Errorf("without flag: %q from %s, want api from env BOTHY_SERVER__ROLE", v.String("server.role"), v.Source("server.role"))
	}

	v = mustLoad(t, s, config.Sources{File: file})
	if v.String("server.role") != "worker" || v.Source("server.role") != (config.Source{Kind: config.SourceFile, Name: file}) {
		t.Errorf("without env: %q from %s, want worker from file", v.String("server.role"), v.Source("server.role"))
	}
	if got := v.Source("server.role").String(); got != "file "+file {
		t.Errorf("Source().String() = %q", got)
	}
	if v.Source("server.listen_addr").Kind != config.SourceDefault {
		t.Errorf("a key no source sets keeps its default")
	}
	if v.File() != file {
		t.Errorf("File() = %q, want %q", v.File(), file)
	}
}

func TestInvalidLowerSourceIsReported(t *testing.T) {
	t.Parallel()
	file := writeFile(t, "server:\n  role: nobody\n")
	got := problems(t, allKinds(t), config.Sources{File: file, Env: []string{"BOTHY_SERVER__ROLE=api"}})
	wantProblem(t, got, "config: server.role: must be one of api, worker, all (source: file "+file+")")
}

func TestUnknownEnvVariableNamesVariable(t *testing.T) {
	t.Parallel()
	got := problems(t, allKinds(t), config.Sources{Env: []string{"BOTHY_SERVER__ROLES=api", "HOME=/tmp"}})
	wantProblem(t, got, "config: unknown variable BOTHY_SERVER__ROLES (source: environment)")

	// Not a key name at all, and a name with characters that would break a log line.
	got = problems(t, allKinds(t), config.Sources{Env: []string{"BOTHY_=x", "BOTHY_A\nB=y"}})
	if len(got) != 2 || got[0].String() != `config: unknown variable "BOTHY_A\nB" (source: environment)` ||
		got[1].String() != "config: unknown variable BOTHY_ (source: environment)" {
		t.Fatalf("problems = %v", got)
	}
}

func TestVariablesWithoutPrefixAreIgnored(t *testing.T) {
	t.Parallel()
	mustLoad(t, allKinds(t), config.Sources{Env: []string{"OTEL_EXPORTER_OTLP_ENDPOINT=http://127.0.0.1:4318", "bothy_server__role=x", "NOEQUALS", "=C:=C:\\"}})
}

func TestDuplicateEnvVariable(t *testing.T) {
	t.Parallel()
	got := problems(t, allKinds(t), config.Sources{Env: []string{"BOTHY_SERVER__ROLE=api", "BOTHY_SERVER__ROLE=worker"}})
	wantProblem(t, got, "config: variable BOTHY_SERVER__ROLE is set more than once (source: environment)")
}

func TestConfigFileVariableIsAllowed(t *testing.T) {
	t.Parallel()
	file := writeFile(t, "server:\n  role: worker\n")
	v := mustLoad(t, allKinds(t), config.Sources{Env: []string{"BOTHY_CONFIG_FILE=" + file}})
	if v.String("server.role") != "worker" || v.File() != file {
		t.Errorf("BOTHY_CONFIG_FILE not used: role %q, file %q", v.String("server.role"), v.File())
	}

	// Sources.File wins over the variable.
	other := writeFile(t, "server:\n  role: api\n")
	v = mustLoad(t, allKinds(t), config.Sources{File: other, Env: []string{"BOTHY_CONFIG_FILE=" + file}})
	if v.String("server.role") != "api" || v.File() != other {
		t.Errorf("--config does not win: role %q, file %q", v.String("server.role"), v.File())
	}

	// Set but empty means no file.
	v = mustLoad(t, allKinds(t), config.Sources{Env: []string{"BOTHY_CONFIG_FILE="}})
	if v.File() != "" {
		t.Errorf("File() = %q, want empty", v.File())
	}
}

func TestConfigFileMissing(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "missing.yaml")
	got := problems(t, allKinds(t), config.Sources{File: path})
	wantProblem(t, got, "config: cannot read the configuration file: it does not exist (source: file "+path+")")

	dir := t.TempDir()
	got = problems(t, allKinds(t), config.Sources{File: dir})
	wantProblem(t, got, "config: cannot read the configuration file: is a directory (source: file "+dir+")")
}

func TestEnvSecretRefVariableIsAllowed(t *testing.T) {
	t.Parallel()
	env := []string{"BOTHY_DATABASE__PASSWORD_REF=env:BOTHY_SECRET_DB_APP_PASSWORD", "BOTHY_SECRET_DB_APP_PASSWORD=" + canary}
	v := mustLoad(t, allKinds(t), config.Sources{Env: env})
	if got := v.String("database.password_ref"); got != "env:BOTHY_SECRET_DB_APP_PASSWORD" {
		t.Errorf("database.password_ref = %q", got)
	}

	// From the file as well.
	file := writeFile(t, "database:\n  password_ref: env:BOTHY_SECRET_DB_APP_PASSWORD\n")
	mustLoad(t, allKinds(t), config.Sources{File: file, Env: env[1:]})

	// Only the effective value counts: a file reference that the environment overrides exempts
	// nothing.
	got := problems(t, allKinds(t), config.Sources{File: file, Env: []string{"BOTHY_DATABASE__PASSWORD_REF=file:/run/bothy/secrets/x", env[1]}})
	wantProblem(t, got, "config: unknown variable BOTHY_SECRET_DB_APP_PASSWORD (source: environment)")
	for _, p := range got {
		if strings.Contains(p.String(), canary) {
			t.Fatalf("problem %q contains the secret", p)
		}
	}
}

func TestTypeErrorNeverEchoesValue(t *testing.T) {
	t.Parallel()
	s := allKinds(t)
	var env []string
	var file strings.Builder
	sections := map[string][]string{}
	for _, k := range s.Keys() {
		if k.Kind == config.KindString || k.Kind == config.KindStringList {
			continue // any text is valid for these kinds
		}
		env = append(env, config.EnvName(k.Name)+"="+canary)
		section, leaf, _ := strings.Cut(k.Name, ".")
		sections[section] = append(sections[section], leaf)
	}
	for section, leaves := range sections {
		file.WriteString(section + ":\n")
		for _, leaf := range leaves {
			// Nested leaves (backup.pg.full_schedule) as a dotted remainder are rejected anyway;
			// write them nested.
			parts := strings.Split(leaf, ".")
			for i, p := range parts {
				file.WriteString(strings.Repeat("  ", i+1) + p + ":")
				if i < len(parts)-1 {
					file.WriteString("\n")
				}
			}
			file.WriteString(" " + canary + "\n")
		}
	}
	file.WriteString("unknown:\n  key: " + canary + "\n")
	env = append(env, "BOTHY_UNKNOWN__KEY="+canary)
	path := writeFile(t, file.String())

	got := problems(t, s, config.Sources{File: path, Env: env, Flags: map[string]string{"server.role": canary}})
	all := (&config.LoadError{Problems: got}).Error()
	if strings.Contains(all, canary) {
		t.Fatalf("a message echoes the value:\n%s", all)
	}
	// Every invalid source is named: env, file and flag of each key.
	for _, k := range s.Keys() {
		if k.Kind == config.KindString || k.Kind == config.KindStringList {
			continue
		}
		if !strings.Contains(all, "config: "+k.Name+": ") || !strings.Contains(all, "env "+config.EnvName(k.Name)) {
			t.Errorf("no problem for %s from the environment in:\n%s", k.Name, all)
		}
	}
	if !strings.Contains(all, "(source: flag --role)") || !strings.Contains(all, "(source: file "+path+")") {
		t.Errorf("flag or file source missing in:\n%s", all)
	}
}

func TestRequiredKeyMissing(t *testing.T) {
	t.Parallel()
	req := key("server.public_url", config.KindURL, "")
	req.Required = true
	note := key("server.note", config.KindString, "")
	note.Required = true
	s := mustSchema(t, req, note)

	got := problems(t, s, config.Sources{Env: []string{"BOTHY_SERVER__PUBLIC_URL=https://chat.example.com"}})
	wantProblem(t, got, "config: server.note: is required; set it in the configuration file or with BOTHY_SERVER__NOTE (source: none)")

	// An empty value does not satisfy a required key, even for a kind that accepts empty text.
	got = problems(t, s, config.Sources{Env: []string{"BOTHY_SERVER__PUBLIC_URL=https://chat.example.com", "BOTHY_SERVER__NOTE="}})
	wantProblem(t, got, "config: server.note: is required and must not be empty (source: env BOTHY_SERVER__NOTE)")

	v := mustLoad(t, s, config.Sources{Env: []string{"BOTHY_SERVER__PUBLIC_URL=https://chat.example.com", "BOTHY_SERVER__NOTE=x"}})
	if v.String("server.public_url") != "https://chat.example.com" {
		t.Errorf("server.public_url = %q", v.String("server.public_url"))
	}
}

func TestBounds(t *testing.T) {
	t.Parallel()
	s := allKinds(t)
	cases := map[string]string{
		"BOTHY_HTTP__MAX_JSON_BODY_BYTES=0":       "config: http.max_json_body_bytes: must be at least 1 (source: env BOTHY_HTTP__MAX_JSON_BODY_BYTES)",
		"BOTHY_SYNC__EVENT_RETENTION=23h":         "config: sync.event_retention: must be at least 1d (source: env BOTHY_SYNC__EVENT_RETENTION)",
		"BOTHY_SYNC__EVENT_RETENTION=3651d":       "config: sync.event_retention: must be at most 3650d (source: env BOTHY_SYNC__EVENT_RETENTION)",
		"BOTHY_STATUS__ERROR_RATIO_DEGRADED=1.5":  "config: status.error_ratio_degraded: must be at most 1 (source: env BOTHY_STATUS__ERROR_RATIO_DEGRADED)",
		"BOTHY_STATUS__ERROR_RATIO_DEGRADED=-0.1": "config: status.error_ratio_degraded: must be at least 0 (source: env BOTHY_STATUS__ERROR_RATIO_DEGRADED)",
	}
	for env, want := range cases {
		wantProblem(t, problems(t, s, config.Sources{Env: []string{env}}), want)
	}
	for _, env := range []string{"BOTHY_HTTP__MAX_JSON_BODY_BYTES=1", "BOTHY_SYNC__EVENT_RETENTION=24h", "BOTHY_SYNC__EVENT_RETENTION=3650d", "BOTHY_STATUS__ERROR_RATIO_DEGRADED=1"} {
		mustLoad(t, s, config.Sources{Env: []string{env}})
	}
}

func TestEnum(t *testing.T) {
	t.Parallel()
	s := allKinds(t)
	for _, role := range []string{"api", "worker", "all"} {
		if got := mustLoad(t, s, config.Sources{Env: []string{"BOTHY_SERVER__ROLE=" + role}}).String("server.role"); got != role {
			t.Errorf("role %q loaded as %q", role, got)
		}
	}
	got := problems(t, s, config.Sources{Env: []string{"BOTHY_SERVER__ROLE=API"}})
	wantProblem(t, got, "config: server.role: must be one of api, worker, all (source: env BOTHY_SERVER__ROLE)")
}

func TestFlags(t *testing.T) {
	t.Parallel()
	s := allKinds(t)
	got := problems(t, s, config.Sources{Flags: map[string]string{"server.listen_addr": "127.0.0.1:0"}})
	wantProblem(t, got, "config: server.listen_addr: no command-line flag sets this key (source: command line)")

	noRole := mustSchema(t, key("server.listen_addr", config.KindListenAddr, "127.0.0.1:8080"))
	got = problems(t, noRole, config.Sources{Flags: map[string]string{"server.role": "api"}})
	wantProblem(t, got, "config: server.role: unknown key (source: flag --role)")
}

func TestSecretRefDefaultPlaceholder(t *testing.T) {
	t.Parallel()
	s := allKinds(t)
	v := mustLoad(t, s, config.Sources{Env: []string{"BOTHY_SECRETS__DIR=/var/run/secrets/bothy"}})
	if got := v.String("database.password_ref"); got != "file:/var/run/secrets/bothy/db_app_password" {
		t.Errorf("database.password_ref = %q, want the default expanded with the effective secrets.dir", got)
	}
	if v.Source("database.password_ref").Kind != config.SourceDefault {
		t.Errorf("source = %s, want default", v.Source("database.password_ref"))
	}

	// An invalid secrets.dir is a problem of secrets.dir alone (ruling M1-09/15), not of the
	// references whose defaults name it.
	got := problems(t, s, config.Sources{Env: []string{"BOTHY_SECRETS__DIR=relative/<secrets.dir>"}})
	wantProblem(t, got, "config: secrets.dir: must be an absolute path (source: env BOTHY_SECRETS__DIR)")

	// With another string key, inserted text is not scanned again, and an expansion that yields
	// an invalid reference is a problem of the reference key with source default.
	other := mustSchema(t, key("files.root", config.KindString, "/srv/EXAMPLE"),
		key("files.key_ref", config.KindSecretRef, "file:<files.root>/key"))
	got = problems(t, other, config.Sources{Env: []string{"BOTHY_FILES__ROOT=relative/<files.root>"}})
	wantProblem(t, got, "config: files.key_ref: default after expanding <files.root>: file reference must be an absolute path (source: default)")

	// A configured value expands <secrets.dir> as well (ruling M1-09/4); the details are in
	// TestSecretsDirExpandedInConfiguredRefs.
	v = mustLoad(t, s, config.Sources{Env: []string{"BOTHY_DATABASE__PASSWORD_REF=file:<secrets.dir>/x", "BOTHY_SECRETS__DIR=/var/run/secrets/bothy"}})
	if got := v.String("database.password_ref"); got != "file:/var/run/secrets/bothy/x" {
		t.Errorf("configured database.password_ref = %q, want <secrets.dir> expanded", got)
	}
}

// exampleSecret stands for a secret value in the tests of M1-09; no problem may contain it.
const exampleSecret = "EXAMPLE-secret-5d21"

// noSecret fails the test when a problem contains secret.
func noSecret(t *testing.T, got []config.Problem, secret string) {
	t.Helper()
	for _, p := range got {
		if strings.Contains(p.String(), secret) {
			t.Fatalf("problem %q contains the secret", p)
		}
	}
}

// TestLoaderAllowsEnvVariablesNamedByRefs holds brief M1-09 D2: the variable an env: reference
// names is no unknown BOTHY_ variable, and every other unknown BOTHY_ variable still aborts.
func TestLoaderAllowsEnvVariablesNamedByRefs(t *testing.T) {
	t.Parallel()
	// The name is a constant because gitleaks' rule generic-api-key reads the literal
	// "BOTHY_SECRET_X=" after another literal ending in SECRET_X as an assigned key.
	const variable = "BOTHY_SECRET_X"
	env := []string{"BOTHY_DATABASE__PASSWORD_REF=env:" + variable, variable + "=" + exampleSecret}
	v := mustLoad(t, allKinds(t), config.Sources{Env: env})
	if got := v.String("database.password_ref"); got != "env:BOTHY_SECRET_X" {
		t.Errorf("database.password_ref = %q, want env:BOTHY_SECRET_X", got)
	}

	got := problems(t, allKinds(t), config.Sources{Env: append(slices.Clone(env), "BOTHY_SECRET_Y="+exampleSecret)})
	wantProblem(t, got, "config: unknown variable BOTHY_SECRET_Y (source: environment)")
	noSecret(t, got, exampleSecret)
}

// TestSecretsDirExpandedInConfiguredRefs holds ruling M1-09/4: <secrets.dir> is replaced by the
// effective secrets.dir in configured secret_ref values too, and in no other way.
func TestSecretsDirExpandedInConfiguredRefs(t *testing.T) {
	t.Parallel()
	const variable = "BOTHY_DATABASE__PASSWORD_REF"
	envSource := config.Source{Kind: config.SourceEnv, Name: variable}
	s := allKinds(t)
	wantRef := func(t *testing.T, v *config.Values, want string, src config.Source) {
		t.Helper()
		if got := v.String("database.password_ref"); got != want {
			t.Errorf("database.password_ref = %q, want %q", got, want)
		}
		if got := v.Text("database.password_ref"); got != want {
			t.Errorf("Text(database.password_ref) = %q, want %q", got, want)
		}
		if got := v.Source("database.password_ref"); got != src {
			t.Errorf("Source(database.password_ref) = %s, want %s", got, src)
		}
	}

	t.Run("env with default secrets.dir", func(t *testing.T) {
		t.Parallel()
		v := mustLoad(t, s, config.Sources{Env: []string{variable + "=file:<secrets.dir>/x"}})
		wantRef(t, v, "file:/run/bothy/secrets/x", envSource)
	})
	t.Run("env with secrets.dir from env", func(t *testing.T) {
		t.Parallel()
		v := mustLoad(t, s, config.Sources{Env: []string{variable + "=file:<secrets.dir>/x", "BOTHY_SECRETS__DIR=/var/run/secrets/bothy"}})
		wantRef(t, v, "file:/var/run/secrets/bothy/x", envSource)
	})
	t.Run("file with secrets.dir from env over file", func(t *testing.T) {
		t.Parallel()
		path := writeFile(t, "secrets:\n  dir: /srv/overridden\ndatabase:\n  password_ref: file:<secrets.dir>/y\n")
		v := mustLoad(t, s, config.Sources{File: path, Env: []string{"BOTHY_SECRETS__DIR=/var/run/secrets/bothy"}})
		wantRef(t, v, "file:/var/run/secrets/bothy/y", config.Source{Kind: config.SourceFile, Name: path})
	})
	t.Run("a placeholder in secrets.dir is a problem of secrets.dir", func(t *testing.T) {
		t.Parallel()
		// The expansion never produces a placeholder again: secrets.dir must not contain "<"
		// (docs/DECISIONS.md M1-09/15), and the reference adds no problem of its own.
		got := problems(t, s, config.Sources{Env: []string{variable + "=file:<secrets.dir>/x", "BOTHY_SECRETS__DIR=/run/<secrets.dir>"}})
		wantProblem(t, got, "config: secrets.dir: must not contain < (source: env BOTHY_SECRETS__DIR)")
	})
	t.Run("expansion of an overridden source is checked", func(t *testing.T) {
		t.Parallel()
		path := writeFile(t, "database:\n  password_ref: file:<secrets.dir>/../y\n")
		got := problems(t, s, config.Sources{File: path, Env: []string{variable + "=file:/run/bothy/secrets/x"}})
		wantProblem(t, got, "config: database.password_ref: after expanding <secrets.dir>: "+
			"file reference must be a clean path without . or .. elements and without repeated or trailing slashes (source: file "+path+")")
	})
	t.Run("relative secrets.dir is a problem of secrets.dir only", func(t *testing.T) {
		t.Parallel()
		got := problems(t, s, config.Sources{Env: []string{variable + "=file:<secrets.dir>/x", "BOTHY_SECRETS__DIR=relative"}})
		wantProblem(t, got, "config: secrets.dir: must be an absolute path (source: env BOTHY_SECRETS__DIR)")
	})
	t.Run("no other placeholder", func(t *testing.T) {
		t.Parallel()
		got := problems(t, s, config.Sources{Env: []string{variable + "=file:<server.role>/x"}})
		wantProblem(t, got, "config: database.password_ref: file reference must be an absolute path (source: env "+variable+")")
	})
	t.Run("failed secrets.dir adds no second problem", func(t *testing.T) {
		t.Parallel()
		path := writeFile(t, "secrets:\n  dir: /a\n  dir: /b\n")
		got := problems(t, s, config.Sources{File: path, Env: []string{variable + "=file:<secrets.dir>/x"}})
		wantProblem(t, got, `config: duplicate key "secrets.dir" at lines 2 and 3 (source: file `+path+")")

		// A list where a value is expected fails secrets.dir in the same way.
		path = writeFile(t, "secrets:\n  dir: [/a]\n")
		got = problems(t, s, config.Sources{File: path, Env: []string{variable + "=file:<secrets.dir>/x"}})
		wantProblem(t, got, "config: secrets.dir: expected a single value, found a list (source: file "+path+")")

		// A reference without the placeholder does not depend on secrets.dir and is still checked.
		path = writeFile(t, "secrets:\n  dir: /a\n  dir: /b\n")
		got = problems(t, s, config.Sources{File: path, Env: []string{variable + "=file:relative"}})
		if len(got) != 2 || got[1].String() != "config: database.password_ref: file reference must be an absolute path (source: env "+variable+")" {
			t.Fatalf("problems = %v, want the duplicate key and the relative reference", got)
		}
	})
	t.Run("schema without the string key secrets.dir keeps the text", func(t *testing.T) {
		t.Parallel()
		ref := key("database.password_ref", config.KindSecretRef, "file:/run/bothy/secrets/db_app_password")
		for _, s := range []*config.Schema{mustSchema(t, ref), mustSchema(t, ref, key("secrets.dir", config.KindInt, "1"))} {
			got := problems(t, s, config.Sources{Env: []string{variable + "=file:<secrets.dir>/x"}})
			wantProblem(t, got, "config: database.password_ref: file reference must be an absolute path (source: env "+variable+")")
		}
	})
}

func TestListFromEnvAndDefault(t *testing.T) {
	t.Parallel()
	v := mustLoad(t, allKinds(t), config.Sources{Env: []string{"BOTHY_EGRESS__ALLOWED_HOSTS=api.example.com:443, [2001:db8::10]:8443;allow_private"}})
	if got, want := v.Strings("egress.allowed_hosts"), []string{"api.example.com:443", "[2001:db8::10]:8443;allow_private"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Strings = %q, want %q", got, want)
	}
	if got := v.Text("server.trusted_proxies"); got != "127.0.0.1/32, ::1/128" {
		t.Errorf("Text(server.trusted_proxies) = %q", got)
	}
}

func TestCrossChecks(t *testing.T) {
	t.Parallel()
	s := allKinds(t)
	ran := 0
	err := s.AddCheck("ratio_below_retention", []string{"status.error_ratio_degraded", "sync.event_retention"}, func(v *config.Values) error {
		ran++
		if v.Float("status.error_ratio_degraded") > 0.5 {
			return errors.New("status.error_ratio_degraded must be at most 0.5 while sync.event_retention is set")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	mustLoad(t, s, config.Sources{})
	if ran != 1 {
		t.Fatalf("check ran %d times, want 1", ran)
	}

	got := problems(t, s, config.Sources{Env: []string{"BOTHY_STATUS__ERROR_RATIO_DEGRADED=0.9"}})
	wantProblem(t, got, "config: status.error_ratio_degraded, sync.event_retention: ratio_below_retention: "+
		"status.error_ratio_degraded must be at most 0.5 while sync.event_retention is set "+
		"(source: status.error_ratio_degraded from env BOTHY_STATUS__ERROR_RATIO_DEGRADED, sync.event_retention from default)")

	// A check whose key did not parse does not run; only the parse problem is reported.
	ran = 0
	got = problems(t, s, config.Sources{Env: []string{"BOTHY_SYNC__EVENT_RETENTION=forever"}})
	if ran != 0 || len(got) != 1 || got[0].Key != "sync.event_retention" {
		t.Fatalf("check ran %d times, problems %v", ran, got)
	}
}

func TestCheckSeesOnlyItsKeys(t *testing.T) {
	t.Parallel()
	s := allKinds(t)
	if err := s.AddCheck("peek", []string{"server.role"}, func(v *config.Values) error {
		_ = v.String("server.listen_addr")
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if recover() == nil {
			t.Fatal("a check that reads an undeclared key must panic")
		}
	}()
	_, _ = config.Load(s, config.Sources{})
}

func TestProblemsSortedAndComplete(t *testing.T) {
	t.Parallel()
	file := writeFile(t, "server:\n  role: nobody\n")
	got := problems(t, allKinds(t), config.Sources{File: file, Env: []string{
		"BOTHY_HTTP__MAX_JSON_BODY_BYTES=many",
		"BOTHY_SERVER__LISTEN_PORT=8080",
	}})
	var lines []string
	for _, p := range got {
		lines = append(lines, p.String())
	}
	want := []string{
		"config: unknown variable BOTHY_SERVER__LISTEN_PORT (source: environment)",
		"config: http.max_json_body_bytes: must be a whole number in decimal digits (source: env BOTHY_HTTP__MAX_JSON_BODY_BYTES)",
		"config: server.role: must be one of api, worker, all (source: file " + file + ")",
	}
	if !reflect.DeepEqual(lines, want) {
		t.Fatalf("problems =\n%s\nwant\n%s", strings.Join(lines, "\n"), strings.Join(want, "\n"))
	}
}

func TestPlaceholderOfFailedKeyAddsNoProblem(t *testing.T) {
	t.Parallel()
	// secrets.dir is set twice in the file: that is the one problem. The reference default that
	// names it must not add a second, misleading problem about its own expansion.
	path := writeFile(t, "secrets:\n  dir: /run/EXAMPLE/a\n  dir: /run/EXAMPLE/b\n")
	got := problems(t, allKinds(t), config.Sources{File: path})
	wantProblem(t, got, `config: duplicate key "secrets.dir" at lines 2 and 3 (source: file `+path+")")

	// A required key behind a placeholder: only the missing key is reported.
	dir := key("secrets.dir", config.KindString, "")
	dir.Required = true
	s := mustSchema(t, dir, key("database.password_ref", config.KindSecretRef, "file:<secrets.dir>/x"))
	wantProblem(t, problems(t, s, config.Sources{}),
		"config: secrets.dir: is required; set it in the configuration file or with BOTHY_SECRETS__DIR (source: none)")
}

// TestSecretsDirRule holds the rule of the key secrets.dir that no kind expresses (docs/DECISIONS.md
// M1-09/15): an absolute path in clean form, not the root directory, without NUL and without "<",
// because file:<secrets.dir>/<name> must give a clean file: reference and the expansion must not
// produce a placeholder again. The loader reports one problem on secrets.dir, never its value,
// and the references that use the placeholder report none of their own.
func TestSecretsDirRule(t *testing.T) {
	t.Parallel()
	s := mustSchema(t,
		key("secrets.dir", config.KindString, "/run/bothy/secrets"),
		key("database.password_ref", config.KindSecretRef, "file:<secrets.dir>/db_app_password"),
		key("migrate.owner_password_ref", config.KindSecretRef, "file:/run/EXAMPLE/db_owner_password"),
	)
	const (
		notAbsolute = "must be an absolute path"
		hasNUL      = "must not contain a NUL character"
		isRoot      = "must not be the root directory"
		hasAngle    = "must not contain <"
		notClean    = "must be a clean path without . or .. elements and without repeated or trailing slashes"
	)
	for dir, want := range map[string]string{
		"/run/bothy/secrets":   "",
		"/srv/EXAMPLE-secrets": "",
		"":                     notAbsolute,
		"run/EXAMPLE":          notAbsolute,
		"./EXAMPLE":            notAbsolute,
		"/":                    isRoot,
		"/run/EXAMPLE\x00x":    hasNUL,
		"/run/<secrets.dir>":   hasAngle,
		"/run/EXAMPLE<x":       hasAngle,
		"/run/EXAMPLE/":        notClean,
		"/run//EXAMPLE":        notClean,
		"/run/./EXAMPLE":       notClean,
		"/run/../EXAMPLE":      notClean,
	} {
		env := []string{"BOTHY_SECRETS__DIR=" + dir, "BOTHY_MIGRATE__OWNER_PASSWORD_REF=file:<secrets.dir>/db_owner_password"}
		v, err := config.Load(s, config.Sources{Env: env})
		if want == "" {
			if err != nil {
				t.Errorf("secrets.dir %q: %v", dir, err)
			} else if got := v.String("migrate.owner_password_ref"); got != "file:"+dir+"/db_owner_password" {
				t.Errorf("secrets.dir %q: migrate.owner_password_ref = %q", dir, got)
			}
			continue
		}
		got := problems(t, s, config.Sources{Env: env})
		wantProblem(t, got, "config: secrets.dir: "+want+" (source: env BOTHY_SECRETS__DIR)")
		if dir != "" && dir != "/" && strings.Contains(got[0].String(), dir) {
			t.Errorf("secrets.dir %q: the problem echoes the value", dir)
		}
	}
}

// TestOptionalSecretRef holds ruling M1-21/1: a secret_ref key with an empty default loads as the
// empty text when no source sets it or a source sets it to the empty text, and takes a reference
// like any other secret_ref key. A secret_ref key with a reference as default still refuses the
// empty text, because its feature cannot be switched off.
func TestOptionalSecretRef(t *testing.T) {
	t.Parallel()
	schema := func() *config.Schema {
		return mustSchema(t,
			key("secrets.dir", config.KindString, "/run/bothy/secrets"),
			key("database.password_ref", config.KindSecretRef, "file:<secrets.dir>/db_app_password"),
			key("server.tls_cert_ref", config.KindSecretRef, ""))
	}
	v := mustLoad(t, schema(), config.Sources{})
	if got := v.String("server.tls_cert_ref"); got != "" {
		t.Errorf("default server.tls_cert_ref = %q, want empty", got)
	}
	v = mustLoad(t, schema(), config.Sources{Env: []string{"BOTHY_SERVER__TLS_CERT_REF="}})
	if got := v.String("server.tls_cert_ref"); got != "" {
		t.Errorf("empty server.tls_cert_ref = %q, want empty", got)
	}
	v = mustLoad(t, schema(), config.Sources{Env: []string{"BOTHY_SERVER__TLS_CERT_REF=file:<secrets.dir>/tls_cert"}})
	if got := v.String("server.tls_cert_ref"); got != "file:/run/bothy/secrets/tls_cert" {
		t.Errorf("server.tls_cert_ref = %q", got)
	}
	got := problems(t, schema(), config.Sources{Env: []string{"BOTHY_SERVER__TLS_CERT_REF=" + canary}})
	if len(got) != 1 || strings.Contains(got[0].String(), canary) || !strings.Contains(got[0].String(), "server.tls_cert_ref") {
		t.Errorf("problems %v: want one that names the key and not the value", got)
	}
	got = problems(t, schema(), config.Sources{Env: []string{"BOTHY_DATABASE__PASSWORD_REF="}})
	if len(got) != 1 || !strings.Contains(got[0].String(), "database.password_ref") {
		t.Errorf("empty database.password_ref: problems %v, want one", got)
	}
}
