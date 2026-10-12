// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"log/slog"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/api/routes"
	"github.com/OWNER/bothy/services/server/api/routes/routestest"
	authzapp "github.com/OWNER/bothy/services/server/internal/authz/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
)

func TestRouteBindings_ConvertsTheTable(t *testing.T) {
	ops := routestest.Operations()
	got := RouteBindings(ops)
	if len(got) != len(ops) {
		t.Fatalf("%d bindings for %d operations", len(got), len(ops))
	}
	for i, op := range ops {
		want := authorize.RouteBinding{OperationID: op.ID, Method: op.Method, Pattern: op.Pattern, Action: op.Action, RateLimitClass: op.RateLimitClass}
		if got[i] != want {
			t.Errorf("binding %d = %+v, want %+v", i, got[i], want)
		}
	}
	if len(RouteBindings(routes.Enabled())) != len(routes.Enabled()) {
		t.Fatal("real table not converted")
	}
}

func TestBuildRegistry_CallsEveryFunction(t *testing.T) {
	calls := 0
	reg := BuildRegistry([]func(*authorize.Registry){
		func(r *authorize.Registry) { calls++; r.Event("authz", authorize.EventAccessChanged) },
		func(r *authorize.Registry) { calls++; r.Op("gateway", authorize.OpTypingStart) },
	})
	if calls != 2 || len(reg.Events()) != 1 || len(reg.Ops()) != 1 {
		t.Fatalf("calls %d, events %v, ops %v", calls, reg.Events(), reg.Ops())
	}
	// nil is the production list of the module ports.
	if BuildRegistry(nil) == nil {
		t.Fatal("no production registry")
	}
}

// TestStartAbortsOnBadRegistration: the route step with an injected bad route binding and a bad
// op registration fails with one ERROR line per violation and mounts nothing, so the public
// listener, which starts after this step, never opens (02 15.1 step 7).
func TestStartAbortsOnBadRegistration(t *testing.T) {
	logs := &captureHandler{}
	mux := &recordingMux{}
	reg := authorize.NewRegistry()
	reg.Op("gateway", authorize.OpHello)
	bad := []authorize.RouteBinding{
		{OperationID: "bootstrapInstance", Method: "POST", Pattern: "POST /api/v1/bootstrap", Action: "instance.bootstrap"},
		{OperationID: "getThing", Method: "GET", Pattern: "GET /api/v1/things", Action: "thing.read", RateLimitClass: "rl.read"},
	}
	cat := authzapp.NewCatalog(slog.New(logs))

	mounted := false
	err := startRoutes(bad, reg, cat, slog.New(logs), func() error {
		mounted = true
		return mountRoutes(RoleAll, mux, fixtureBuild, cat, testChain(t), slog.New(logs))
	})

	if err == nil {
		t.Fatal("the start step accepted bad registrations")
	}
	for _, want := range []string{"bootstrapInstance", "getThing", `"hello"`} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not name %s", err, want)
		}
	}
	if mounted || len(mux.patterns) != 0 {
		t.Errorf("the mount step ran (%v) or mounted %q despite the failed check", mounted, mux.patterns)
	}
	var lines []logRecord
	for _, r := range logs.records {
		if r.msg == msgRegistrationCheckFailed {
			lines = append(lines, r)
		}
	}
	if len(lines) != 3 {
		t.Fatalf("got %d %q lines, want one per violation (3): %+v", len(lines), msgRegistrationCheckFailed, logs.records)
	}
	kinds := map[string]string{}
	for _, l := range lines {
		if l.level != slog.LevelError {
			t.Errorf("line %+v", l)
		}
		kinds[l.attrs["error.type"]] = l.attrs["bothy.action"]
	}
	if len(kinds) != 3 || kinds["system_action"] != "instance.bootstrap" {
		t.Errorf("violation kinds = %v", kinds)
	}
	if _, ok := kinds["unknown_action"]; !ok {
		t.Errorf("violation kinds = %v", kinds)
	}
	if _, ok := kinds["outbound_op"]; !ok {
		t.Errorf("violation kinds = %v", kinds)
	}
}

// TestStartRoutes_ValidRunsMount: without violations the mount step runs once.
func TestStartRoutes_ValidRunsMount(t *testing.T) {
	calls := 0
	err := startRoutes(nil, authorize.NewRegistry(), authzapp.NewCatalog(nil), nil, func() error { calls++; return nil })
	if err != nil || calls != 1 {
		t.Fatalf("err %v, calls %d", err, calls)
	}
}

// TestStartRoutes_WorkerChecksRegistrations: the worker serves no REST API but emits events, so
// its registrations are checked too.
func TestStartRoutes_WorkerChecksRegistrations(t *testing.T) {
	reg := authorize.NewRegistry()
	reg.Event("messaging", authorize.EventType("no.such.event"))
	err := startRoutes(nil, reg, authzapp.NewCatalog(slog.New(&captureHandler{})), slog.New(&captureHandler{}), nil)
	if err == nil || !strings.Contains(err.Error(), "no.such.event") {
		t.Fatalf("err = %v", err)
	}
}

// TestStartRoutes_MountsThroughTheAuthzCatalogue: with valid registrations the step mounts the
// fixture surface; its actions are unknown to the real catalogue, so the mount fails with the
// authz catalogue, which proves the catalogue is the one passed to the route check.
func TestStartRoutes_MountsThroughTheAuthzCatalogue(t *testing.T) {
	logs := &captureHandler{}
	cat := authzapp.NewCatalog(slog.New(logs))
	err := startRoutes(nil, authorize.NewRegistry(), cat, slog.New(logs), func() error {
		return mountRoutes(RoleAPI, &recordingMux{}, fixtureBuild, cat, testChain(t), slog.New(logs))
	})
	if err == nil || !strings.Contains(err.Error(), "thing.") {
		t.Fatalf("err = %v; the fixture actions must be unknown to the authz catalogue", err)
	}
	mux := &recordingMux{}
	if err := startRoutes(RouteBindings(routes.Enabled()), BuildRegistry(nil), cat, slog.New(logs), func() error {
		return MountRoutes(RoleAPI, mux, Handlers{}, cat, testChain(t), slog.New(logs))
	}); err != nil {
		t.Fatalf("real table: %v", err)
	}
}

func TestNewAuthzCatalog(t *testing.T) {
	cat, err := NewAuthzCatalog(slog.New(&captureHandler{}))
	if err != nil || cat == nil {
		t.Fatalf("NewAuthzCatalog = %v, %v", cat, err)
	}
	if _, err := cat.Spec(authorize.ActionMessageRead); err != nil {
		t.Fatal(err)
	}
}
