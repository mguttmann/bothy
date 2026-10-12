// SPDX-License-Identifier: Apache-2.0

package wire_test

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/platform/egress"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

func TestEgressEnvNames(t *testing.T) {
	t.Parallel()
	for key, env := range map[string]string{
		"egress.allowed_hosts":      "BOTHY_EGRESS__ALLOWED_HOSTS",
		"egress.timeout":            "BOTHY_EGRESS__TIMEOUT",
		"egress.max_response_bytes": "BOTHY_EGRESS__MAX_RESPONSE_BYTES",
	} {
		if got := config.EnvName(key); got != env {
			t.Errorf("EnvName(%s) = %s, want %s", key, got, env)
		}
	}
}

func TestEgressConfigDefaults(t *testing.T) {
	t.Parallel()
	v, err := config.Load(schema(t), config.Sources{Env: testEnv()})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	got, err := wire.EgressConfig(v)
	if err != nil {
		t.Fatalf("EgressConfig: %v", err)
	}
	want := egress.Config{AllowedHosts: []egress.Rule{}, Timeout: 15 * time.Second, MaxResponseBytes: 1048576}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("EgressConfig = %+v, want %+v", got, want)
	}
}

func TestEgressConfigFromEnvironment(t *testing.T) {
	t.Parallel()
	v, err := config.Load(schema(t), config.Sources{Env: []string{testPublicURL,
		"BOTHY_EGRESS__ALLOWED_HOSTS=API.example.com:443, [2001:db8::10]:8443;allow_private,stt.example.com;allow_private",
		"BOTHY_EGRESS__TIMEOUT=30s",
		"BOTHY_EGRESS__MAX_RESPONSE_BYTES=2048",
	}})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	got, err := wire.EgressConfig(v)
	if err != nil {
		t.Fatalf("EgressConfig: %v", err)
	}
	want := egress.Config{
		AllowedHosts: []egress.Rule{
			{Host: "api.example.com", Port: 443},
			{Host: "2001:db8::10", Port: 8443, AllowPrivate: true},
			{Host: "stt.example.com", AllowPrivate: true},
		},
		Timeout:          30 * time.Second,
		MaxResponseBytes: 2048,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("EgressConfig = %+v, want %+v", got, want)
	}
	if _, err := egress.New(got, egress.Options{Purpose: "test"}, egress.Deps{}); err != nil {
		t.Fatalf("New with the loaded configuration: %v", err)
	}
}

func TestEgressConfigFromFile(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "bothy.yaml")
	text := "egress:\n  allowed_hosts:\n    - api.example.com\n    - \"192.0.2.10:8443;allow_private\"\n  timeout: 2m\n"
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	v, err := config.Load(schema(t), config.Sources{File: path, Env: testEnv()})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	got, err := wire.EgressConfig(v)
	if err != nil {
		t.Fatalf("EgressConfig: %v", err)
	}
	want := []egress.Rule{{Host: "api.example.com"}, {Host: "192.0.2.10", Port: 8443, AllowPrivate: true}}
	if !reflect.DeepEqual(got.AllowedHosts, want) || got.Timeout != 2*time.Minute {
		t.Fatalf("EgressConfig = %+v", got)
	}
}

func TestEgressAllowedHostsRejectedAtLoad(t *testing.T) {
	t.Parallel()
	cases := []struct {
		value   string
		message string
	}{
		{"api.example.com, *.example.com", "egress_allowed_hosts: element 2: egress: invalid rule: "},
		{"https://api.example.com", "egress_allowed_hosts: element 1: egress: invalid rule: "},
		{"api.example.com;allow_everything", "egress_allowed_hosts: element 1: egress: invalid rule: "},
		{"api.example.com:443, a.example.com, API.example.com:443;allow_private", "egress_allowed_hosts: elements 1 and 3 name the same host and port"},
		// A list whose first entry starts with "[" is a JSON array in the environment.
		{`["[2001:db8::10]:8443", "2001:DB8::10", "[2001:db8::10]:8443;allow_private"]`, "egress_allowed_hosts: elements 1 and 3 name the same host and port"},
	}
	for _, c := range cases {
		got := load(t, "BOTHY_EGRESS__ALLOWED_HOSTS="+c.value)
		if len(got) != 1 || got[0].Key != "egress.allowed_hosts" || !strings.HasPrefix(got[0].Message, c.message) ||
			got[0].Source != "egress.allowed_hosts from env BOTHY_EGRESS__ALLOWED_HOSTS" {
			t.Errorf("%q: problems %v, want one with message %q", c.value, got, c.message)
			continue
		}
		for _, part := range strings.Split(c.value, ",") {
			noValues(t, got, strings.TrimSpace(part))
		}
	}
}

// TestEgressIPv6FirstNeedsJSONArray holds the list grammar of the configuration reference: in
// the environment a text that starts with "[" is a JSON array, so a first entry with a bracketed
// IPv6 literal needs the JSON form.
func TestEgressIPv6FirstNeedsJSONArray(t *testing.T) {
	t.Parallel()
	got := load(t, "BOTHY_EGRESS__ALLOWED_HOSTS=[2001:db8::10]:8443, api.example.com")
	if len(got) != 1 || got[0].Message != "must be a JSON array of strings or a comma-separated list" {
		t.Errorf("comma form with a bracket first: problems %v", got)
	}
	v, err := config.Load(schema(t), config.Sources{Env: []string{testPublicURL, `BOTHY_EGRESS__ALLOWED_HOSTS=["[2001:db8::10]:8443", "api.example.com"]`}})
	if err != nil {
		t.Fatalf("JSON form: %v", err)
	}
	if text := v.Text("egress.allowed_hosts"); text != `["[2001:db8::10]:8443","api.example.com"]` {
		t.Errorf("canonical text %s", text)
	}
}

func TestEgressLimitsMustBePositive(t *testing.T) {
	t.Parallel()
	for _, env := range []string{"BOTHY_EGRESS__TIMEOUT=0s", "BOTHY_EGRESS__TIMEOUT=0ms", "BOTHY_EGRESS__MAX_RESPONSE_BYTES=0", "BOTHY_EGRESS__MAX_RESPONSE_BYTES=-1"} {
		got := load(t, env)
		if len(got) != 1 || !strings.HasPrefix(got[0].Key, "egress.") || !strings.HasPrefix(got[0].Message, "must be at least") {
			t.Errorf("%s: problems %v, want one bound problem", env, got)
		}
	}
	if got := load(t, "BOTHY_EGRESS__TIMEOUT=1ms", "BOTHY_EGRESS__MAX_RESPONSE_BYTES=1"); got != nil {
		t.Errorf("smallest valid limits: problems %v", got)
	}
}

func TestEgressConfigNeedsTheEgressKeys(t *testing.T) {
	t.Parallel()
	s, err := config.NewSchema([]config.Key{{Name: "server.role", Kind: config.KindString, Default: "all", Doc: "Role.", Chapter: "16-betrieb.md"}})
	if err != nil {
		t.Fatal(err)
	}
	v, err := config.Load(s, config.Sources{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := wire.EgressConfig(v); err == nil {
		t.Fatal("EgressConfig without the egress keys: no error")
	}
}
