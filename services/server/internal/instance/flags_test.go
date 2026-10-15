// SPDX-License-Identifier: Apache-2.0

package instance_test

import (
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/instance"
	"github.com/OWNER/bothy/services/server/internal/instance/domain"
)

// registerModules reads the module keys under modules: of architecture/modules.yaml (two-space
// indented keys), enough for this test without a YAML parser.
func registerModules(t *testing.T) []string {
	t.Helper()
	raw, err := os.ReadFile("../../architecture/modules.yaml")
	if err != nil {
		t.Fatal(err)
	}
	key := regexp.MustCompile(`^  ([a-z][a-z0-9_]*):`)
	var out []string
	in := false
	for _, line := range strings.Split(string(raw), "\n") {
		switch {
		case line == "modules:":
			in = true
		case in && line != "" && !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "#"):
			in = false
		case in:
			if m := key.FindStringSubmatch(line); m != nil {
				out = append(out, m[1])
			}
		}
	}
	if len(out) == 0 {
		t.Fatal("no modules in modules.yaml")
	}
	return out
}

// TestRegisteredFlagsValid holds the one registry to the registry rules (ADR 0064).
func TestRegisteredFlagsValid(t *testing.T) {
	t.Parallel()
	defs := instance.RegisteredFlags()
	specs := make([]domain.FlagSpec, 0, len(defs))
	for _, d := range defs {
		specs = append(specs, domain.FlagSpec{Key: d.Key, Kind: string(d.Kind), Description: d.Description, Module: d.Module, RemoveBy: d.RemoveBy, Default: d.Default})
	}
	if findings := domain.CheckRegistry(specs, registerModules(t)); len(findings) != 0 {
		t.Fatalf("findings: %q", findings)
	}
	keys := make([]string, 0, len(defs))
	for _, d := range defs {
		keys = append(keys, d.Key)
	}
	want := []string{instance.FlagDMEnabled, instance.FlagTokensPersonalEnabled, instance.FlagVoiceEnabled}
	if !slices.Equal(keys, want) {
		t.Fatalf("keys %q, want %q in key order", keys, want)
	}
	for _, d := range defs {
		if d.Kind != instance.FlagKindOption || d.Default {
			t.Errorf("%s: kind %s default %v, want option off (configuration.md 7)", d.Key, d.Kind, d.Default)
		}
	}
	defs[0].Key = "changed"
	if instance.RegisteredFlags()[0].Key == "changed" {
		t.Error("RegisteredFlags returns the registry itself")
	}
}
