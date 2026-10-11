// SPDX-License-Identifier: Apache-2.0

package routes_test

import (
	"os"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"

	"github.com/OWNER/bothy/services/server/api/routes"
)

// TestRealTable_Empty pins the state of this ticket: no operation has a handler yet, so the real
// table is empty and Build mounts nothing (P-04). A handler ticket that enables an operation
// changes this test together with api/operations.yaml.
func TestRealTable_Empty(t *testing.T) {
	if ops := routes.Enabled(); len(ops) != 0 {
		t.Fatalf("Enabled() = %d operations, want none at this ticket", len(ops))
	}
	got, err := routes.Build(struct{}{}, routes.Options{}, routes.SpecCatalog())
	if err != nil || len(got) != 0 {
		t.Fatalf("Build = %v, %v; want no routes and no error", got, err)
	}
	if err := routes.Verify(nil, routes.SpecCatalog()); err != nil {
		t.Errorf("Verify(nothing) = %v", err)
	}
	if err := routes.Verify([]string{"GET /api/v1/discovery"}, routes.SpecCatalog()); err == nil {
		t.Error("Verify accepted a pattern of no enabled operation")
	}
	if _, ok := routes.ByID("getDiscovery"); ok {
		t.Error("ByID found an operation that is not enabled")
	}
	if _, ok := routes.ByPattern("GET /api/v1/discovery"); ok {
		t.Error("ByPattern found an operation that is not enabled")
	}
}

// TestRealTable_PathsBelowBaseURL holds W6 for every enabled operation.
func TestRealTable_PathsBelowBaseURL(t *testing.T) {
	for _, op := range routes.Enabled() {
		if !strings.HasPrefix(op.Path, routes.BaseURL+"/") || !strings.Contains(op.Pattern, " "+routes.BaseURL+"/") {
			t.Errorf("%s: path %q, pattern %q not below %s", op.ID, op.Path, op.Pattern, routes.BaseURL)
		}
	}
}

// TestSpecCatalog_MatchesActionsYAML: the generated catalogue knows every action of
// docs/spec/actions.yaml that a route may carry, with its class, and no system action and no
// action without a class.
func TestSpecCatalog_MatchesActionsYAML(t *testing.T) {
	data, err := os.ReadFile("../../../../docs/spec/actions.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var file struct {
		Actions []struct {
			ID      string `yaml:"id"`
			Right   string `yaml:"right"`
			RLClass string `yaml:"rl_class"`
		} `yaml:"actions"`
	}
	if err := yaml.Unmarshal(data, &file); err != nil {
		t.Fatal(err)
	}
	if len(file.Actions) == 0 {
		t.Fatal("no actions read")
	}
	cat := routes.SpecCatalog()
	for _, a := range file.Actions {
		class, ok := cat.Class(a.ID)
		unreachable := a.Right == "system" || a.RLClass == "none"
		switch {
		case unreachable && ok:
			t.Errorf("action %s (right %q, class %q) is known to the route catalogue", a.ID, a.Right, a.RLClass)
		case !unreachable && (!ok || class != a.RLClass):
			t.Errorf("action %s: catalogue gives %q (known %v), actions.yaml %q", a.ID, class, ok, a.RLClass)
		}
	}
}
