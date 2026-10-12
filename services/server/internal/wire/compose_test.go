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
	v, err := config.Load(schema(t), config.Sources{})
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
