// SPDX-License-Identifier: Apache-2.0

package routes_test

import (
	"context"
	"os"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/api/routes"
)

// stubServer implements the enabled operations without behaviour; Build only registers them.
type stubServer struct{}

func (stubServer) GetDevice(context.Context, rest.GetDeviceRequestObject) (rest.GetDeviceResponseObject, error) {
	return nil, nil
}

func (stubServer) ListMyDevices(context.Context, rest.ListMyDevicesRequestObject) (rest.ListMyDevicesResponseObject, error) {
	return nil, nil
}

func (stubServer) ListUserDevices(context.Context, rest.ListUserDevicesRequestObject) (rest.ListUserDevicesResponseObject, error) {
	return nil, nil
}

// TestRealTable_Enabled pins the enabled operations (P-04): since M1-33 the three device reads.
// A handler ticket that enables an operation changes this test together with api/operations.yaml.
func TestRealTable_Enabled(t *testing.T) {
	want := map[string]string{
		"getDevice":       "GET /api/v1/devices/{device_id}",
		"listMyDevices":   "GET /api/v1/me/devices",
		"listUserDevices": "GET /api/v1/users/{user_id}/devices",
	}
	ops := routes.Enabled()
	if len(ops) != len(want) {
		t.Fatalf("Enabled() = %d operations, want %d", len(ops), len(want))
	}
	for _, op := range ops {
		if want[op.ID] != op.Pattern || op.Action != "device.list" || op.RateLimitClass != "rl.read" {
			t.Errorf("operation %+v", op)
		}
		if got, ok := routes.ByPattern(op.Pattern); !ok || got.ID != op.ID {
			t.Errorf("ByPattern(%s) = %+v, %v", op.Pattern, got, ok)
		}
	}
	got, err := routes.Build(stubServer{}, routes.Options{}, routes.SpecCatalog())
	if err != nil || len(got) != len(want) {
		t.Fatalf("Build = %d routes, %v", len(got), err)
	}
	if err := routes.Verify(nil, routes.SpecCatalog()); err == nil {
		t.Error("Verify(nothing) passed although three operations are enabled")
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
