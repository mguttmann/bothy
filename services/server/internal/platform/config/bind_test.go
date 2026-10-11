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

// fullConfig binds one field of every allowed field type.
type fullConfig struct {
	Role        string         `config:"server.role"`
	Listen      string         `config:"server.listen_addr"`
	MaxBody     int64          `config:"http.max_json_body_bytes"`
	MaxBodyInt  int            `config:"http.max_json_body_bytes"`
	Ratio       float64        `config:"status.error_ratio_degraded"`
	Local       bool           `config:"auth.local_password_enabled"`
	Retention   time.Duration  `config:"sync.event_retention"`
	Hosts       []string       `config:"egress.allowed_hosts"`
	Proxies     []netip.Prefix `config:"server.trusted_proxies"`
	Schedule    string         `config:"backup.pg.full_schedule"`
	MinVersion  string         `config:"instance.min_server_version"`
	DatabaseRef string         `config:"database.password_ref"`
	Callback    string         `config:"webhook.callback_url"`
	BaseDir     string         `config:"secrets.dir"`
	unexported  string
}

func TestBind(t *testing.T) {
	t.Parallel()
	v := mustLoad(t, allKinds(t), config.Sources{Env: []string{"BOTHY_SERVER__ROLE=worker", "BOTHY_EGRESS__ALLOWED_HOSTS=a,b"}})
	c := fullConfig{unexported: "kept"}
	if err := config.Bind(v, &c); err != nil {
		t.Fatalf("Bind: %v", err)
	}
	want := fullConfig{
		unexported: "kept",
		Role:       "worker", Listen: "127.0.0.1:8080", MaxBody: 1048576, MaxBodyInt: 1048576, Ratio: 0.05, Local: true,
		Retention: 720 * time.Hour, Hosts: []string{"a", "b"},
		Proxies:  []netip.Prefix{netip.MustParsePrefix("127.0.0.1/32"), netip.MustParsePrefix("::1/128")},
		Schedule: "0 2 * * 0", MinVersion: "0.1.0", DatabaseRef: "file:/run/bothy/secrets/db_app_password",
		Callback: "https://hooks.example.com/in", BaseDir: "/run/bothy/secrets",
	}
	if !reflect.DeepEqual(c, want) {
		t.Fatalf("Bind filled\n%+v\nwant\n%+v", c, want)
	}
	c.Hosts[0] = "changed"
	if v.Strings("egress.allowed_hosts")[0] != "a" {
		t.Fatal("Bind shares the slice of Values")
	}
}

func TestBindRejects(t *testing.T) {
	t.Parallel()
	v := mustLoad(t, allKinds(t), config.Sources{})
	type untagged struct {
		Role string `config:"server.role"`
		Port int
	}
	type unknownKey struct {
		Role string `config:"server.rol"`
	}
	type mismatch struct {
		Role int64          `config:"server.role"`
		Max  int32          `config:"http.max_json_body_bytes"`
		TTL  int64          `config:"sync.event_retention"`
		List []netip.Prefix `config:"egress.allowed_hosts"`
	}
	type namedType string
	type named struct {
		Role namedType `config:"server.role"`
	}
	type emptyTag struct {
		Role string `config:""`
	}
	cases := []struct {
		name string
		dst  any
		want []string
	}{
		{"untagged exported field", &untagged{}, []string{"untagged.Port", "no config tag"}},
		{"unknown key", &unknownKey{}, []string{"unknownKey.Role", `unknown key "server.rol"`}},
		{"type mismatch", &mismatch{}, []string{"mismatch.Role", "mismatch.Max", "mismatch.TTL", "mismatch.List", "does not fit"}},
		{"named type", &named{}, []string{"named.Role", "does not fit"}},
		{"empty tag", &emptyTag{}, []string{"emptyTag.Role", "no config tag"}},
		{"not a pointer", fullConfig{}, []string{"pointer to a struct"}},
		{"nil pointer", (*fullConfig)(nil), []string{"pointer to a struct"}},
		{"pointer to non-struct", new(string), []string{"pointer to a struct"}},
		{"nil", nil, []string{"pointer to a struct"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			err := config.Bind(v, c.dst)
			if err == nil {
				t.Fatal("Bind succeeded")
			}
			for _, w := range c.want {
				if !strings.Contains(err.Error(), w) {
					t.Errorf("error %q does not contain %q", err, w)
				}
			}
		})
	}
	if err := config.Bind(nil, &fullConfig{}); err == nil {
		t.Error("Bind(nil values) succeeded")
	}
}

func TestBindIsAllOrNothing(t *testing.T) {
	t.Parallel()
	v := mustLoad(t, allKinds(t), config.Sources{})
	type partly struct {
		Role string `config:"server.role"`
		Bad  int64  `config:"server.listen_addr"`
	}
	p := partly{Role: "untouched"}
	if err := config.Bind(v, &p); err == nil {
		t.Fatal("Bind succeeded")
	}
	if p.Role != "untouched" {
		t.Fatalf("Bind changed a field before failing: %+v", p)
	}
}
