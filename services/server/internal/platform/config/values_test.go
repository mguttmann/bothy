// SPDX-License-Identifier: Apache-2.0

package config_test

import (
	"net/netip"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/platform/config"
)

func TestValuesAccessors(t *testing.T) {
	t.Parallel()
	v := mustLoad(t, allKinds(t), config.Sources{Env: []string{"BOTHY_EGRESS__ALLOWED_HOSTS=a, b"}})
	if got := v.Int("http.max_json_body_bytes"); got != 1048576 {
		t.Errorf("Int = %d", got)
	}
	if got := v.Float("status.error_ratio_degraded"); got != 0.05 {
		t.Errorf("Float = %v", got)
	}
	if got := v.Bool("auth.local_password_enabled"); !got {
		t.Errorf("Bool = %v", got)
	}
	if got := v.Duration("sync.event_retention"); got != 720*time.Hour {
		t.Errorf("Duration = %v", got)
	}
	if got := v.Strings("egress.allowed_hosts"); !reflect.DeepEqual(got, []string{"a", "b"}) {
		t.Errorf("Strings = %q", got)
	}
	if got := v.Prefixes("server.trusted_proxies"); !reflect.DeepEqual(got, []netip.Prefix{netip.MustParsePrefix("127.0.0.1/32"), netip.MustParsePrefix("::1/128")}) {
		t.Errorf("Prefixes = %v", got)
	}
	for k, want := range map[string]string{
		"server.role": "all", "server.listen_addr": "127.0.0.1:8080", "backup.pg.full_schedule": "0 2 * * 0",
		"instance.min_server_version": "0.1.0", "webhook.callback_url": "https://hooks.example.com/in",
	} {
		if got := v.String(k); got != want {
			t.Errorf("String(%s) = %q, want %q", k, got, want)
		}
	}
	if got := v.String("database.password_ref"); got != "file:/run/bothy/secrets/db_app_password" {
		t.Errorf("String(database.password_ref) = %q", got)
	}
	if got := v.String("secrets.dir"); got != "/run/bothy/secrets" {
		t.Errorf("String(secrets.dir) = %q", got)
	}
	for k, want := range map[string]string{
		"http.max_json_body_bytes": "1048576", "status.error_ratio_degraded": "0.05", "auth.local_password_enabled": "true",
		"sync.event_retention": "30d", "egress.allowed_hosts": "a, b", "server.trusted_proxies": "127.0.0.1/32, ::1/128",
	} {
		if got := v.Text(k); got != want {
			t.Errorf("Text(%s) = %q, want %q", k, got, want)
		}
	}
}

func TestValuesSlicesAreCopies(t *testing.T) {
	t.Parallel()
	v := mustLoad(t, allKinds(t), config.Sources{Env: []string{"BOTHY_EGRESS__ALLOWED_HOSTS=a, b"}})
	v.Strings("egress.allowed_hosts")[0] = "changed"
	v.Prefixes("server.trusted_proxies")[0] = netip.Prefix{}
	if v.Strings("egress.allowed_hosts")[0] != "a" || !v.Prefixes("server.trusted_proxies")[0].IsValid() {
		t.Fatal("accessors return shared slices")
	}
}

func TestAccessorPanicsOnUnknownKeyOrKind(t *testing.T) {
	t.Parallel()
	v := mustLoad(t, allKinds(t), config.Sources{})
	cases := map[string]func(){
		"String unknown":       func() { v.String("server.rol") },
		"String of int":        func() { v.String("http.max_json_body_bytes") },
		"Int of string":        func() { v.Int("server.role") },
		"Float of int":         func() { v.Float("http.max_json_body_bytes") },
		"Bool of string":       func() { v.Bool("secrets.dir") },
		"Duration of int":      func() { v.Duration("http.max_json_body_bytes") },
		"Strings of cidr list": func() { v.Strings("server.trusted_proxies") },
		"Prefixes of strings":  func() { v.Prefixes("egress.allowed_hosts") },
		"Text unknown":         func() { v.Text("nope.nope") },
		"Source unknown":       func() { v.Source("nope.nope") },
	}
	for name, fn := range cases {
		t.Run(name, func(t *testing.T) {
			defer func() {
				r := recover()
				if r == nil {
					t.Fatal("no panic")
				}
				if msg, ok := r.(string); !ok || !strings.HasPrefix(msg, "config: ") {
					t.Fatalf("panic value %#v, want a string starting with config:", r)
				}
			}()
			fn()
		})
	}
}

func TestSourceString(t *testing.T) {
	t.Parallel()
	for src, want := range map[config.Source]string{
		{Kind: config.SourceDefault}:                          "default",
		{Kind: config.SourceFile, Name: "/etc/bothy.yaml"}:    "file /etc/bothy.yaml",
		{Kind: config.SourceEnv, Name: "BOTHY_A__B"}:          "env BOTHY_A__B",
		{Kind: config.SourceFlag, Name: "--role"}:             "flag --role",
		{Kind: config.SourceKind(9)}:                          "SourceKind(9)",
		{Kind: config.SourceFile, Name: "/etc/a\nb.yaml"}:     `file "/etc/a\nb.yaml"`,
		{Kind: config.SourceFile, Name: "/etc/my bothy.yaml"}: "file /etc/my bothy.yaml",
		{Kind: config.SourceEnv, Name: "BOTHY_\xff"}:          `env "BOTHY_\xff"`,
	} {
		if got := src.String(); got != want {
			t.Errorf("%+v.String() = %q, want %q", src, got, want)
		}
	}
}
