// SPDX-License-Identifier: Apache-2.0

package wire_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/agents/domain/definition"
	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

// TestAgentDefinitionLimitsEqualCorpus (A1-02): the defaults of agents.definition.* and
// agents.mcp_stdio_runners, bound through AgentsConfig, equal tests/agent-definitions/limits.json,
// so the shared corpus runs against the limits a server starts with, and every key of Limits has
// a schema key of the same name.
func TestAgentDefinitionLimitsEqualCorpus(t *testing.T) {
	t.Parallel()
	vals, err := config.Load(schema(t), config.Sources{Env: testEnv()})
	if err != nil {
		t.Fatalf("defaults: %v", err)
	}
	cfg, err := wire.AgentsConfig(vals)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "tests", "agent-definitions", "limits.json"))
	if err != nil {
		t.Fatal(err)
	}
	var byKey map[string]json.RawMessage
	if err := json.Unmarshal(raw, &byKey); err != nil {
		t.Fatal(err)
	}
	var want definition.Limits
	v := reflect.ValueOf(&want).Elem()
	for i := range v.NumField() {
		key := v.Type().Field(i).Tag.Get("config")
		if err := json.Unmarshal(byKey[key], v.Field(i).Addr().Interface()); err != nil {
			t.Fatalf("limits.json %s: %v", key, err)
		}
	}
	if got := cfg.DefinitionLimits(); !reflect.DeepEqual(got, want) {
		t.Fatalf("limits from the defaults %+v, corpus %+v", got, want)
	}
}
