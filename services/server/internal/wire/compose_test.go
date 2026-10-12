// SPDX-License-Identifier: Apache-2.0

package wire_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.yaml.in/yaml/v3"

	"github.com/OWNER/bothy/services/server/internal/platform/config"
)

// TestComposeStopGraceAboveTotal holds 02 "Shutdown": the orchestrator waits longer than the
// server's whole shutdown budget, so Compose never kills a server that is still within
// shutdown.total_timeout. Every service of infra/compose/compose.yaml that runs `serve` needs a
// stop_grace_period above the default of the key (check D8 of make config-doc-check holds the
// same relation with the forms of entrypoint and command; this test reads the real file).
func TestComposeStopGraceAboveTotal(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "infra", "compose", "compose.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Services map[string]struct {
			Command         []string `yaml:"command"`
			StopGracePeriod string   `yaml:"stop_grace_period"`
		} `yaml:"services"`
	}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	v, err := config.Load(schema(t), config.Sources{Env: testEnv()})
	if err != nil {
		t.Fatal(err)
	}
	total := v.Duration("shutdown.total_timeout")
	servers := 0
	for name, svc := range doc.Services {
		if len(svc.Command) == 0 || svc.Command[0] != "serve" {
			continue
		}
		servers++
		grace, err := time.ParseDuration(svc.StopGracePeriod)
		if err != nil {
			t.Errorf("%s: stop_grace_period %q: %v", name, svc.StopGracePeriod, err)
			continue
		}
		if grace <= total {
			t.Errorf("%s: stop_grace_period %s is not above shutdown.total_timeout %s", name, grace, total)
		}
	}
	if servers < 2 || !strings.Contains(string(raw), "--role=worker") {
		t.Fatalf("found %d server services; the test no longer reads the file it should", servers)
	}
}

// TestComposeTrustedProxiesPassthrough holds Scope 6 of brief M1-21 (16 "4.3 Rand und
// Identität"): service api of profile direct passes server.trusted_proxies from the Compose
// variable BOTHY_TRUSTED_PROXIES with the loopback default of the key, so the operator can name
// the gateway of the Compose network that the reverse proxy appears as. Profile tailscale passes
// nothing: the server listens on loopback in the namespace of the sidecar, and the anchor
// x-server-env, which every server service merges, does not set the key either. The default in
// the file and in .env.example loads to the default of the schema.
func TestComposeTrustedProxiesPassthrough(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "infra", "compose", "compose.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		ServerEnv map[string]string `yaml:"x-server-env"`
		Services  map[string]struct {
			Environment map[string]string `yaml:"environment"`
		} `yaml:"services"`
	}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	const (
		key  = "BOTHY_SERVER__TRUSTED_PROXIES"
		want = "${BOTHY_TRUSTED_PROXIES:-127.0.0.1/32,::1/128}"
	)
	api, ok := doc.Services["api"]
	if !ok || api.Environment[key] != want {
		t.Fatalf("service api: %s = %q, want %q", key, api.Environment[key], want)
	}
	if v, ok := doc.Services["api-tailscale"].Environment[key]; ok {
		t.Errorf("service api-tailscale sets %s = %q; the sidecar profile keeps the loopback default", key, v)
	}
	if v, ok := doc.ServerEnv[key]; ok {
		t.Errorf("x-server-env sets %s = %q; only service api passes it", key, v)
	}
	example, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "infra", "compose", ".env.example"))
	if err != nil {
		t.Fatal(err)
	}
	var exampleValue string
	for line := range strings.Lines(string(example)) {
		if v, ok := strings.CutPrefix(strings.TrimSpace(line), "BOTHY_TRUSTED_PROXIES="); ok {
			exampleValue = v
		}
	}
	defaults, err := config.Load(schema(t), config.Sources{Env: testEnv()})
	if err != nil {
		t.Fatal(err)
	}
	fallback := strings.TrimSuffix(strings.TrimPrefix(want, "${BOTHY_TRUSTED_PROXIES:-"), "}")
	for name, value := range map[string]string{"compose.yaml fallback": fallback, ".env.example": exampleValue} {
		v, err := config.Load(schema(t), config.Sources{Env: append(testEnv(), key+"="+value)})
		if err != nil {
			t.Fatalf("%s %q: %v", name, value, err)
		}
		if got, def := v.Text("server.trusted_proxies"), defaults.Text("server.trusted_proxies"); got != def {
			t.Errorf("%s loads to %q, want the schema default %q", name, got, def)
		}
	}
}
