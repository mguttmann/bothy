// SPDX-License-Identifier: Apache-2.0

package config_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

func TestEnvNameRoundTrip(t *testing.T) {
	t.Parallel()
	pairs := map[string]string{
		"a.b_c":                          "BOTHY_A__B_C",
		"sync.event_retention":           "BOTHY_SYNC__EVENT_RETENTION",
		"jobs.queue.default.max_workers": "BOTHY_JOBS__QUEUE__DEFAULT__MAX_WORKERS",
		"a1.b2_c3.d":                     "BOTHY_A1__B2_C3__D",
	}
	for k, env := range pairs {
		if got := config.EnvName(k); got != env {
			t.Errorf("EnvName(%q) = %q, want %q", k, got, env)
		}
		if got, ok := config.KeyFromEnv(env); !ok || got != k {
			t.Errorf("KeyFromEnv(%q) = %q, %v; want %q, true", env, got, ok, k)
		}
	}
	for _, v := range []string{
		"BOTHY_A___B", "BOTHY_a__b", "BOTHY_A__B_", "BOTHY__A__B", "BOTHY_A____B", "BOTHY_", "BOTHY_A",
		"BOTHY_CONFIG_FILE", "OTHER_A__B", "bothy_a__b", "BOTHY_A__1B", "BOTHY_A__B__", "BOTHY_A-B__C",
	} {
		if got, ok := config.KeyFromEnv(v); ok {
			t.Errorf("KeyFromEnv(%q) = %q, true; want false", v, got)
		}
	}
	// Every key of the server schema maps to its variable and back.
	server, err := wire.ConfigSchema()
	if err != nil {
		t.Fatalf("wire.ConfigSchema: %v", err)
	}
	for _, k := range server.Keys() {
		env := config.EnvName(k.Name)
		if got, ok := config.KeyFromEnv(env); !ok || got != k.Name {
			t.Errorf("KeyFromEnv(EnvName(%q)) = %q, %v", k.Name, got, ok)
		}
	}
}

func TestInvalidKeyNames(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"a__b.c", "a._b", "A.b", "single", "a.b.", ".a.b", "a..b", "a.b_", "a.1b", "a.b-c", "a.b c", ""} {
		_, err := config.NewSchema([]config.Key{key(name, config.KindString, "")})
		wantSchemaError(t, err, "invalid key name")
	}
}

func TestRefNamingRule(t *testing.T) {
	t.Parallel()
	_, err := config.NewSchema([]config.Key{key("x.y_ref", config.KindString, "")})
	wantSchemaError(t, err, "x.y_ref", "secret_ref")
	_, err = config.NewSchema([]config.Key{key("x.y", config.KindSecretRef, "file:/run/x")})
	wantSchemaError(t, err, "x.y", "_ref")
	mustSchema(t, key("x.y_ref", config.KindSecretRef, "file:/run/x"))
}

func TestNewSchemaRejects(t *testing.T) {
	t.Parallel()
	withEnum := func(k config.Key, enum ...string) config.Key { k.Enum = enum; return k }
	withBounds := func(k config.Key, lo, hi string) config.Key { k.Min, k.Max = lo, hi; return k }
	required := func(k config.Key) config.Key { k.Required = true; return k }
	cases := []struct {
		name string
		keys []config.Key
		want []string
	}{
		{"duplicate", []config.Key{key("a.b", config.KindString, ""), key("a.b", config.KindInt, "1")}, []string{"a.b", "registered twice"}},
		{"no kind", []config.Key{key("a.b", 0, "")}, []string{"a.b", "kind"}},
		{"unknown kind", []config.Key{key("a.b", config.Kind(99), "")}, []string{"a.b", "kind"}},
		{"default does not parse", []config.Key{key("a.b", config.KindInt, "ten")}, []string{"a.b", "default", "whole number"}},
		{"empty default of int", []config.Key{key("a.b", config.KindInt, "")}, []string{"a.b", "empty default"}},
		{"empty default of duration", []config.Key{key("a.b", config.KindDuration, "")}, []string{"a.b", "empty default"}},
		{"required with default", []config.Key{required(key("a.b", config.KindString, "x"))}, []string{"a.b", "required"}},
		{"enum without values", []config.Key{key("a.b", config.KindEnum, "x")}, []string{"a.b", "Enum"}},
		{"enum on string", []config.Key{withEnum(key("a.b", config.KindString, "x"), "x")}, []string{"a.b", "Enum"}},
		{"enum with empty value", []config.Key{withEnum(key("a.b", config.KindEnum, "x"), "x", "")}, []string{"a.b", "Enum"}},
		{"enum with duplicate", []config.Key{withEnum(key("a.b", config.KindEnum, "x"), "x", "x")}, []string{"a.b", "Enum"}},
		{"enum default not allowed", []config.Key{withEnum(key("a.b", config.KindEnum, "z"), "x", "y")}, []string{"a.b", "default"}},
		{"bounds on string", []config.Key{withBounds(key("a.b", config.KindString, ""), "1", "")}, []string{"a.b", "bounds"}},
		{"min does not parse", []config.Key{withBounds(key("a.b", config.KindInt, "5"), "x", "")}, []string{"a.b", "Min"}},
		{"max does not parse", []config.Key{withBounds(key("a.b", config.KindDuration, "5s"), "", "1x")}, []string{"a.b", "Max"}},
		{"min above max", []config.Key{withBounds(key("a.b", config.KindFloat, "0.5"), "1", "0.1")}, []string{"a.b", "Min", "Max"}},
		{"default below min", []config.Key{withBounds(key("a.b", config.KindInt, "0"), "1", "")}, []string{"a.b", "default", "at least 1"}},
		{"default above max", []config.Key{withBounds(key("a.b", config.KindDuration, "2h"), "", "1h")}, []string{"a.b", "default", "at most 1h"}},
		{"no doc", []config.Key{func() config.Key { k := key("a.b", config.KindString, ""); k.Doc = " "; return k }()}, []string{"a.b", "Doc"}},
		{"bad chapter", []config.Key{func() config.Key { k := key("a.b", config.KindString, ""); k.Chapter = "betrieb.md"; return k }()}, []string{"a.b", "Chapter"}},
		{"key is a section of another key", []config.Key{key("a.b", config.KindString, ""), key("a.b.c", config.KindString, "")}, []string{"a.b", "section"}},
		{"placeholder names unknown key", []config.Key{key("a.b_ref", config.KindSecretRef, "file:<secrets.dir>/x")}, []string{"a.b_ref", "<secrets.dir>"}},
		{"placeholder names non-string key", []config.Key{key("secrets.dir", config.KindInt, "1"), key("a.b_ref", config.KindSecretRef, "file:<secrets.dir>/x")}, []string{"a.b_ref", "string key"}},
		{"expanded default does not parse", []config.Key{key("secrets.dir", config.KindString, "relative"), key("a.b_ref", config.KindSecretRef, "file:<secrets.dir>/x")}, []string{"a.b_ref", "absolute"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			_, err := config.NewSchema(c.keys)
			wantSchemaError(t, err, c.want...)
		})
	}
}

func TestNewSchemaReportsEveryProblem(t *testing.T) {
	t.Parallel()
	_, err := config.NewSchema([]config.Key{key("Bad", config.KindString, ""), key("a.b", config.KindInt, "x")})
	wantSchemaError(t, err, `"Bad"`, "a.b")
	if n := strings.Count(err.Error(), "\n") + 1; n != 2 {
		t.Errorf("error has %d lines, want 2: %q", n, err)
	}
}

func TestNewSchemaAccepts(t *testing.T) {
	t.Parallel()
	req := key("server.public_url", config.KindURL, "")
	req.Required = true
	bounded := key("http.max_json_body_bytes", config.KindInt, "1048576")
	bounded.Min = "1"
	mustSchema(t,
		enumKey(), req, bounded,
		key("secrets.dir", config.KindString, "/run/bothy/secrets"),
		key("database.password_ref", config.KindSecretRef, "file:<secrets.dir>/db_app_password"),
		key("server.trusted_proxies", config.KindCIDRList, "127.0.0.1/32, ::1/128"),
		key("egress.allowed_hosts", config.KindStringList, ""),
		key("database.sslrootcert", config.KindString, ""),
		key("backup.pg.full_schedule", config.KindCron, "0 2 * * 0"),
		key("instance.min_server_version", config.KindSemVer, "0.1.0"),
		key("status.error_ratio_degraded", config.KindFloat, "0.05"),
		key("auth.local_password_enabled", config.KindBool, "true"),
		key("secrets.cache_ttl", config.KindDuration, "5m"),
		key("server.listen_addr", config.KindListenAddr, "127.0.0.1:8080"),
		key("jobs.queue.default.max_workers", config.KindInt, "8"),
	)
}

func TestKeysSortedCopies(t *testing.T) {
	t.Parallel()
	s := mustSchema(t, key("z.a", config.KindString, ""), enumKey(), key("a.z", config.KindString, ""))
	keys := s.Keys()
	var names []string
	for _, k := range keys {
		names = append(names, k.Name)
	}
	if want := []string{"a.z", "server.role", "z.a"}; !reflect.DeepEqual(names, want) {
		t.Fatalf("Keys() names = %q, want %q", names, want)
	}
	keys[1].Enum[0] = "changed"
	keys[1].Default = "changed"
	if again := s.Keys(); again[1].Enum[0] != "api" || again[1].Default != "all" {
		t.Fatalf("Keys() returned shared state: %+v", again[1])
	}
}

func TestNewSchemaCopiesInput(t *testing.T) {
	t.Parallel()
	in := []config.Key{enumKey()}
	s := mustSchema(t, in...)
	in[0].Enum[0] = "changed"
	if s.Keys()[0].Enum[0] != "api" {
		t.Fatal("NewSchema keeps the caller's Enum slice")
	}
}

func TestAddCheckRejects(t *testing.T) {
	t.Parallel()
	noop := func(*config.Values) error { return nil }
	s := mustSchema(t, enumKey(), key("server.listen_addr", config.KindListenAddr, "127.0.0.1:8080"))
	if err := s.AddCheck("role_known", []string{"server.role"}, noop); err != nil {
		t.Fatalf("AddCheck: %v", err)
	}
	cases := []struct {
		name    string
		check   string
		keys    []string
		fn      func(*config.Values) error
		wantErr string
	}{
		{"duplicate name", "role_known", []string{"server.role"}, noop, "registered twice"},
		{"bad name", "Role-Known", []string{"server.role"}, noop, "invalid check name"},
		{"empty name", "", []string{"server.role"}, noop, "invalid check name"},
		{"no keys", "other", nil, noop, "no keys"},
		{"unknown key", "other", []string{"server.rol"}, noop, `unknown key "server.rol"`},
		{"key twice", "other", []string{"server.role", "server.role"}, noop, "twice"},
		{"nil function", "other", []string{"server.role"}, nil, "no function"},
	}
	for _, c := range cases {
		err := s.AddCheck(c.check, c.keys, c.fn)
		if err == nil || !strings.Contains(err.Error(), c.wantErr) {
			t.Errorf("%s: AddCheck error = %v, want one containing %q", c.name, err, c.wantErr)
		}
	}
}

func TestProblemAndLoadErrorText(t *testing.T) {
	t.Parallel()
	p := config.Problem{Key: "server.role", Source: "env BOTHY_SERVER__ROLE", Message: "must be one of api, worker, all"}
	if got, want := p.String(), "config: server.role: must be one of api, worker, all (source: env BOTHY_SERVER__ROLE)"; got != want {
		t.Errorf("Problem.String() = %q, want %q", got, want)
	}
	u := config.Problem{Source: "environment", Message: "unknown variable BOTHY_X"}
	if got, want := u.String(), "config: unknown variable BOTHY_X (source: environment)"; got != want {
		t.Errorf("Problem.String() = %q, want %q", got, want)
	}
	e := &config.LoadError{Problems: []config.Problem{u, p}}
	if got, want := e.Error(), u.String()+"\n"+p.String(); got != want {
		t.Errorf("LoadError.Error() = %q, want %q", got, want)
	}
}

// TestOptionalSecretRefDefault holds ruling M1-21/1: a secret_ref key whose chapter cell is
// "leer" (server.tls_cert_ref, server.tls_key_ref) registers with an empty default, which means
// "no reference" and switches the feature that reads the key off.
func TestOptionalSecretRefDefault(t *testing.T) {
	t.Parallel()
	mustSchema(t, key("server.tls_cert_ref", config.KindSecretRef, ""))
}
