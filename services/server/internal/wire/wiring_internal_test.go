// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"

	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
)

// notWiredYet are the modules of architecture/modules.yaml that `bothy serve` does not wire yet.
// The ticket that wires one removes it here and adds its module_<name>.go.
var notWiredYet = []string{"privacy", "status"}

// registerModules returns the module names of architecture/modules.yaml, the module register
// (ADR 0050).
func registerModules(t *testing.T) []string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "architecture", "modules.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var reg struct {
		Modules map[string]any `yaml:"modules"`
	}
	if err := yaml.Unmarshal(data, &reg); err != nil {
		t.Fatal(err)
	}
	var names []string
	for name := range reg.Modules {
		names = append(names, name)
	}
	slices.Sort(names)
	if len(names) == 0 {
		t.Fatal("the module register has no modules")
	}
	return names
}

// TestEveryRegisteredModuleIsWiredOnce: every module of the module register has exactly one
// wiring (its module_<name>.go) unless it is listed in notWiredYet, no wiring names a module
// outside the register, and no listed module is wired (PL/28).
func TestEveryRegisteredModuleIsWiredOnce(t *testing.T) {
	t.Parallel()
	count := map[string]int{}
	for _, w := range moduleWirings() {
		count[w.module]++
	}
	registered := registerModules(t)
	for _, name := range registered {
		pending := slices.Contains(notWiredYet, name)
		switch {
		case pending && count[name] > 0:
			t.Errorf("module %s is wired; remove it from notWiredYet", name)
		case !pending && count[name] != 1:
			t.Errorf("module %s is wired %d times, want exactly once (module_%s.go)", name, count[name], name)
		}
	}
	for name := range count {
		if !slices.Contains(registered, name) {
			t.Errorf("wiring of %q, which is not a module of architecture/modules.yaml", name)
		}
	}
	for _, name := range notWiredYet {
		if !slices.Contains(registered, name) {
			t.Errorf("notWiredYet lists %q, which is not a module of architecture/modules.yaml", name)
		}
	}
}

// TestWiringRules: the production wiring passes checkWirings (unique order keys of parts and
// mounts, the parts of a step adjacent, every part with a step and a start).
func TestWiringRules(t *testing.T) {
	t.Parallel()
	if err := checkWirings(allWirings()); err != nil {
		t.Fatal(err)
	}
}

// TestCheckWiringsRefuses: checkWirings names each kind of violation.
func TestCheckWiringsRefuses(t *testing.T) {
	t.Parallel()
	start := func(*server, context.Context) error { return nil }
	cases := map[string]struct {
		ws   []moduleWiring
		want string
	}{
		"module twice": {
			ws:   []moduleWiring{{module: "chat"}, {module: "chat"}},
			want: `module "chat" is wired twice`,
		},
		"order twice": {
			ws: []moduleWiring{
				{module: "a", parts: []part{{order: 10, step: "a", start: start}}},
				{module: "b", parts: []part{{order: 10, step: "b", start: start}}},
			},
			want: "order 10 is used by",
		},
		"step not adjacent": {
			ws: []moduleWiring{
				{module: "a", parts: []part{{order: 10, step: "a", start: start}, {order: 30, step: "a", start: start}}},
				{module: "b", parts: []part{{order: 20, step: "b", start: start}}},
			},
			want: `the parts of step "a" are not adjacent`,
		},
		"no start": {
			ws:   []moduleWiring{{module: "a", parts: []part{{order: 10, step: "a"}}}},
			want: "has no step or no start",
		},
		"mount order twice": {
			ws: []moduleWiring{
				{module: "a", mounts: []moduleMount{{order: 1}}},
				{module: "b", mounts: []moduleMount{{order: 1}}},
			},
			want: "mount order 1 is used by",
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			err := checkWirings(c.ws)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("checkWirings = %v, want an error with %q", err, c.want)
			}
		})
	}
}

// TestStartSequenceUnchanged pins the start components of each role with the default
// configuration as they were before the module files (PL/28: the split changed no behaviour). A
// ticket that adds or moves a step updates this list together with Serve's documentation: M7-17
// added ephemeral_listener after the public listener of role api, which holds it in every
// cluster.mode for the hints of a separate worker; M6-13 added stats_measure after the schema;
// M6-05 added statistics after apps in the roles api and all, which serve the statistics
// operations.
func TestStartSequenceUnchanged(t *testing.T) {
	t.Parallel()
	modules := []string{"telemetry", "secrets", "internal_listener", "database", "schema", "stats_measure", "instance", "audit", "sync", "authz",
		"settings", "identity", "agents", "chat", "files", "terminal", "apps"}
	rest := []string{"signing_keys", "audit_log", "sessions", "routes"}
	common := slices.Concat(modules, rest)
	served := slices.Concat(modules, []string{"statistics"}, rest)
	want := map[Role][]string{
		RoleAPI:    append(slices.Clone(served), "log_tail", "agent_presence", "jobs", "public_listener", "ephemeral_listener", "ready"),
		RoleWorker: append(slices.Clone(common), "jobs", "event_consumers", "scan_health", "ready"),
		RoleAll: append(slices.Clone(served), "log_tail", "agent_presence", "jobs", "event_consumers", "scan_health",
			"public_listener", "ready"),
	}
	for role, w := range want {
		if got := componentNames(t, role); !slices.Equal(got, w) {
			t.Errorf("%s: start sequence\n got %v\nwant %v", role, got, w)
		}
	}
}

// TestChatStepParts: the step chat runs the parts of chat, messaging, runs, the personal data of
// sync, the readiness of agents, catch-up and snapshot, notify, the APNs adapter (M3-20) and the
// FCM adapter (M3-21) of notify, stats and the importer (M8-03) in this order, as the single start
// function did before the module files.
func TestChatStepParts(t *testing.T) {
	t.Parallel()
	var got []string
	for _, p := range orderedParts(allWirings()) {
		if p.step == "chat" {
			got = append(got, p.module)
		}
	}
	want := []string{"chat", "messaging", "billing", "runs", "sync", "agents", "sync", "notify", "notify", "notify", "stats", "importer"}
	if !slices.Equal(got, want) {
		t.Fatalf("parts of step chat by module: %v, want %v", got, want)
	}
}

// TestModuleMountsAndBindingsOrder: the gateway mounts before the terminal (the control
// connection needs the gateway's publisher), and the route bindings keep the order gateway,
// host connect, terminal streams.
func TestModuleMountsAndBindingsOrder(t *testing.T) {
	t.Parallel()
	var mounts []string
	for _, w := range moduleWirings() {
		for range w.mounts {
			mounts = append(mounts, w.module)
		}
	}
	if !slices.Equal(mounts, []string{"gateway", "terminal"}) {
		t.Fatalf("modules with mounts: %v", mounts)
	}
	g, tm := gatewayWiring().mounts[0].order, terminalWiring().mounts[0].order
	if g >= tm {
		t.Fatalf("gateway mount order %d is not before terminal %d", g, tm)
	}
	bindings := moduleRouteBindings()
	want := append([]authorize.RouteBinding{GatewayRouteBinding(), HostConnectRouteBinding()}, TerminalStreamRouteBindings()...)
	if !slices.Equal(bindings, want) {
		t.Fatalf("module route bindings:\n got %v\nwant %v", bindings, want)
	}
}

// TestModulePartsRefuseMissingDependencies: a start part whose earlier steps did not run (no
// audit, sync, Authorizer, identity, chat and so on) fails the start with an error instead of
// panicking, and so do the job hooks of apps, identity and the signing keys (fail closed).
func TestModulePartsRefuseMissingDependencies(t *testing.T) {
	t.Parallel()
	values := loadValues(t)
	parts := map[string]func(*server, context.Context) error{
		"agents": (*server).startAgents, "billing": (*server).startBilling, "audit_log": (*server).startAuditLog, "chat": (*server).startChat,
		"files": (*server).startFiles, "messaging": (*server).startMessaging, "notify": (*server).startNotify,
		"personal": (*server).startPersonal, "runs": (*server).startRuns, "scan_health": (*server).startScanHealth,
		"sessions": (*server).startSessions, "settings": (*server).startSettings, "stats": (*server).startStats,
		"sync": (*server).startSync, "sync_reads": (*server).startSyncReads, "terminal": (*server).startTerminal,
	}
	for name, start := range parts {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			s := &server{values: values, log: newSwitchLogger(slog.New(slog.DiscardHandler))}
			if err := start(s, t.Context()); err == nil {
				t.Fatalf("part %s started without its dependencies", name)
			}
		})
	}
	for _, w := range []moduleWiring{appsWiring(), identityWiring(), instanceWiring()} {
		t.Run("jobs_"+w.module, func(t *testing.T) {
			t.Parallel()
			if err := w.jobs(&server{values: values}, &JobModules{}); err == nil {
				t.Fatalf("the job hook of %s accepted a server without the module", w.module)
			}
		})
	}
}
