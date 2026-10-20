// SPDX-License-Identifier: Apache-2.0

package stack

import (
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

// overlayKeys are the keys each test overlay may set on a service of the installation (Design
// note 3); every other service of an overlay is test only.
var overlayKeys = map[string]map[string][]string{
	"test/compose.terminal.yaml": {"api": {"env_file", "environment", "ports"}, "worker": {"env_file"}},
	"test/compose.terminal-twonode.yaml": {
		"api-2": {"image", "pull_policy", "env_file", "environment", "ports"}, "edge": {"ports"},
	},
}

// TestComposeTerminalOverlay renders docker compose config for each profile and proves Design note
// 3: the installation file comes first and the overlays change it only in the named keys; the only
// published port is 127.0.0.1::8443 of tls-front; privileged only on termhost, which is only on
// termhosts; in twonode api and api-2 run with equal image, command, volumes and environment
// (cluster.mode multi in both); default and limits have neither api-2 nor edge; no Tailscale
// service.
func TestComposeTerminalOverlay(t *testing.T) {
	root, err := RepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	d := &Docker{}
	if err := Preflight(t.Context(), d); err != nil {
		t.Fatal(err)
	}
	for _, p := range []Profile{ProfileDefault, ProfileLimits, ProfileTwoNode} {
		files := composeFiles(p)
		if files[0] != "compose.yaml" {
			t.Fatalf("%s: the first file is %s, want the installation file", p, files[0])
		}
		for _, f := range files {
			allowed, ok := overlayKeys[f]
			if !ok {
				continue
			}
			checkOverlayKeys(t, filepath.Join(root, "infra", "compose", f), allowed)
		}
		cfg := renderConfig(t, root, p)
		services := cfg.Services
		var published []string
		for name, svc := range services {
			for _, port := range svc.Ports {
				published = append(published, name+":"+port.HostIP+":"+port.Published+":"+port.Target.String())
			}
			if svc.Privileged && name != "termhost" {
				t.Errorf("%s: %s is privileged", p, name)
			}
			if strings.Contains(name, "tailscale") {
				t.Errorf("%s: the stack holds the Tailscale service %s", p, name)
			}
		}
		if want := []string{"tls-front:127.0.0.1::8443"}; !slices.Equal(published, want) {
			t.Errorf("%s: published ports %v, want %v", p, published, want)
		}
		th, ok := services["termhost"]
		if !ok || !th.Privileged {
			t.Fatalf("%s: termhost missing or not privileged", p)
		}
		if nets := slices.Sorted(maps.Keys(th.Networks)); !slices.Equal(nets, []string{"termhosts"}) {
			t.Errorf("%s: termhost is on %v, want only termhosts", p, nets)
		}
		_, api2 := services["api-2"]
		_, edge := services["edge"]
		if p != ProfileTwoNode {
			if api2 || edge {
				t.Errorf("%s: api-2 %v, edge %v, want neither", p, api2, edge)
			}
			continue
		}
		a, b := services["api"], services["api-2"]
		if a.Image != b.Image || !reflect.DeepEqual(a.Command, b.Command) || !reflect.DeepEqual(a.Volumes, b.Volumes) || !reflect.DeepEqual(a.Environment, b.Environment) {
			t.Errorf("twonode: api and api-2 differ in image, command, volumes or environment")
		}
		for _, svc := range []string{"api", "api-2"} {
			if services[svc].Environment["BOTHY_CLUSTER__MODE"] != "multi" {
				t.Errorf("twonode: %s runs cluster.mode %q", svc, services[svc].Environment["BOTHY_CLUSTER__MODE"])
			}
		}
	}
}

// composeConfig is the part of docker compose config --format json the test reads.
type composeConfig struct {
	Services map[string]struct {
		Image       string            `json:"image"`
		Command     any               `json:"command"`
		Volumes     any               `json:"volumes"`
		Environment map[string]string `json:"environment"`
		Privileged  bool              `json:"privileged"`
		Networks    map[string]any    `json:"networks"`
		Ports       []struct {
			HostIP    string      `json:"host_ip"`
			Published string      `json:"published"`
			Target    json.Number `json:"target"`
		} `json:"ports"`
	} `json:"services"`
}

// renderConfig renders the Compose configuration of profile p with the services of every profile
// of the suite active (termhost included) and placeholder values for the suite variables.
func renderConfig(t *testing.T, root string, p Profile) composeConfig {
	t.Helper()
	tmp := t.TempDir()
	env := filepath.Join(tmp, "server.env")
	if err := os.WriteFile(env, []byte("BOTHY_CLUSTER__MODE=multi\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	s := &Stack{Project: "termsuite-overlay-test", Root: root, Profile: p, docker: &Docker{}, files: composeFiles(p), env: []string{
		"COMPOSE_PROFILES=direct,termhost", "BOTHY_PUBLIC_URL=" + InstanceOrigin, "SMOKE_SERVER_IMAGE=termsuite-server:test",
		"SMOKE_SECRETS_INPUT=" + tmp, "SMOKE_RUN_ID=test", "SMOKE_OUT=" + tmp, "SMOKE_TMP=" + tmp, "SMOKE_CANARY=test",
		"TERMSUITE_SERVER_ENV=" + env, "TERMSUITE_FRONT_DIR=" + tmp, "TERMSUITE_HOST_IMAGE=termsuite-host:test",
		"TERMSUITE_UID=1000", "TERMSUITE_GID=1000", "TERMSUITE_TRUSTED_PROXIES=192.0.2.0/24",
	}}
	out, err := s.Compose(t.Context(), "config", "--format", "json")
	if err != nil {
		t.Fatalf("%s: compose config: %v", p, err)
	}
	var cfg composeConfig
	if err := json.Unmarshal(out, &cfg); err != nil {
		t.Fatal(err)
	}
	return cfg
}

// checkOverlayKeys fails when an overlay sets a key of an installation service outside allowed.
func checkOverlayKeys(t *testing.T, path string, allowed map[string][]string) {
	t.Helper()
	b, err := os.ReadFile(path) //nolint:gosec // G304: an overlay of the repository
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Services map[string]map[string]yaml.Node `yaml:"services"`
	}
	if err := yaml.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	installation := map[string]bool{"api": true, "worker": true, "migrate": true, "postgres": true, "secrets-init": true, "otel-collector": true,
		"backup": true, "api-2": true, "edge": true}
	for name, keys := range doc.Services {
		if !installation[name] {
			continue
		}
		for k := range keys {
			if !slices.Contains(allowed[name], k) {
				t.Errorf("%s redefines %s.%s of the installation", filepath.Base(path), name, k)
			}
		}
	}
}
