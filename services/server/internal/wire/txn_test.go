// SPDX-License-Identifier: Apache-2.0

package wire_test

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.yaml.in/yaml/v3"

	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

// update rewrites testdata/txn_registrations.golden instead of comparing with it.
var update = flag.Bool("update", false, "rewrite testdata/txn_registrations.golden")

// goldenPath is the golden file of the txn registrations.
const goldenPath = "testdata/txn_registrations.golden"

// moduleDeps reads depends_on of every module from architecture/modules.yaml, the module
// register (ADR 0050).
func moduleDeps(t *testing.T) wire.ModuleDeps {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "architecture", "modules.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var reg struct {
		Modules map[string]struct {
			DependsOn []string `yaml:"depends_on"`
		} `yaml:"modules"`
	}
	if err := yaml.Unmarshal(data, &reg); err != nil {
		t.Fatal(err)
	}
	deps := wire.ModuleDeps{}
	for name, m := range reg.Modules {
		deps[name] = m.DependsOn
	}
	if len(deps) == 0 || deps["chat"] == nil {
		t.Fatalf("the module register has no modules with dependencies: %v", deps)
	}
	return deps
}

func TestTxnRegistrationsGolden(t *testing.T) {
	t.Parallel()
	reg, err := wire.NewTxnRegistry(wire.NewModules(nil, nil))
	if err != nil {
		t.Fatal(err)
	}
	if !reg.Sealed() {
		t.Fatal("NewTxnRegistry did not seal the registry")
	}
	if err := wire.CheckTxnDirections(reg.Registrations(), moduleDeps(t)); err != nil {
		t.Fatal(err)
	}
	got := wire.TxnRegistrations(reg)
	if *update {
		if err := os.WriteFile(goldenPath, []byte(got), 0o600); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatal(err)
	}
	if got != string(want) {
		t.Fatalf("txn registrations differ from %s (regenerate with -update):\n--- got\n%s--- want\n%s", goldenPath, got, want)
	}
}

func TestCheckTxnDirections_ReverseSubscriptionFails(t *testing.T) {
	t.Parallel()
	deps := moduleDeps(t)
	reverse := []txn.Registration{{
		Kind: txn.KindSubscribe, Event: "chat.ThreadCreated", Publisher: "chat", Subscriber: "instance",
		Handler: "example.com/server/internal/instance/app.(*Service).OnThreadCreated-fm",
	}}
	err := wire.CheckTxnDirections(reverse, deps)
	if err == nil || !strings.Contains(err.Error(), "instance") || !strings.Contains(err.Error(), "chat") {
		t.Fatalf("CheckTxnDirections = %v, want an error naming instance and chat", err)
	}
	allowed := []txn.Registration{
		{Kind: txn.KindSubscribe, Event: "identity.UserUpdated", Publisher: "identity", Subscriber: "chat", Handler: "h1"},
		{Kind: txn.KindSubscribe, Event: "kernel.AccessChanged", Publisher: "kernel", Subscriber: "runs", Handler: "h2"},
	}
	if err := wire.CheckTxnDirections(allowed, deps); err != nil {
		t.Fatalf("allowed subscriptions: %v", err)
	}
	for _, bad := range []txn.Registration{
		{Kind: txn.KindSubscribe, Event: "chat.X", Publisher: "chat", Subscriber: "chat", Handler: "h"},
		{Kind: txn.KindSubscribe, Event: "nomodule.X", Publisher: "nomodule", Subscriber: "chat", Handler: "h"},
		{Kind: txn.KindSubscribe, Event: "instance.X", Publisher: "instance", Subscriber: "nomodule", Handler: "h"},
		{Kind: "other", Subscriber: "chat", Handler: "h"},
		// A wrapper in a nested internal package of wire or platform would pass a closure of that
		// package off as one of the module it is named after.
		{Kind: txn.KindSubscribe, Event: "identity.X", Publisher: "identity", Subscriber: "chat",
			Handler: "example.com/server/internal/wire/internal/chat.Handler[...].func1"},
		{Kind: txn.KindHook, Phase: txn.PhaseSync, Subscriber: "sync",
			Handler: "example.com/server/internal/platform/x/internal/sync.Hook.func1"},
	} {
		if err := wire.CheckTxnDirections([]txn.Registration{bad}, deps); err == nil {
			t.Errorf("CheckTxnDirections accepted %+v", bad)
		}
	}
}

func TestCheckTxnDirections_WrongHookOwnerFails(t *testing.T) {
	t.Parallel()
	deps := moduleDeps(t)
	good := []txn.Registration{
		{Kind: txn.KindHook, Phase: txn.PhaseRightsBegin, Subscriber: "authz", Handler: "a"},
		{Kind: txn.KindHook, Phase: txn.PhaseRightsCheck, Subscriber: "authz", Handler: "b"},
		{Kind: txn.KindHook, Phase: txn.PhaseSync, Subscriber: "sync", Handler: "c"},
		{Kind: txn.KindHook, Phase: txn.PhaseAudit, Subscriber: "audit", Handler: "d"},
	}
	if err := wire.CheckTxnDirections(good, deps); err != nil {
		t.Fatalf("the hooks of 02 section 8.1: %v", err)
	}
	for _, bad := range []txn.Registration{
		{Kind: txn.KindHook, Phase: txn.PhaseSync, Subscriber: "chat", Handler: "x"},
		{Kind: txn.KindHook, Phase: txn.PhaseRightsBegin, Subscriber: "sync", Handler: "x"},
		{Kind: txn.KindHook, Phase: txn.PhaseAudit, Subscriber: "authz", Handler: "x"},
		{Kind: txn.KindHook, Phase: txn.PhaseWork, Subscriber: "chat", Handler: "x"},
	} {
		err := wire.CheckTxnDirections([]txn.Registration{bad}, deps)
		if err == nil || !strings.Contains(err.Error(), bad.Subscriber) || !strings.Contains(err.Error(), bad.Phase.String()) {
			t.Errorf("CheckTxnDirections(%+v) = %v, want an error naming owner and phase", bad, err)
		}
	}
}

func TestUnitConfigBindsDefaults(t *testing.T) {
	t.Parallel()
	cfg, err := wire.UnitConfig(loadValues(t, schema(t)))
	if err != nil {
		t.Fatal(err)
	}
	want := pg.UnitConfig{RetryMax: 2, StatementTimeout: 5 * time.Second, LockTimeout: 2 * time.Second, IdleInTxTimeout: 10 * time.Second}
	if cfg != want {
		t.Fatalf("UnitConfig = %+v, want %+v", cfg, want)
	}
	cfg, err = wire.UnitConfig(loadValues(t, schema(t), "BOTHY_DATABASE__TX_RETRY_MAX=5"))
	if err != nil || cfg.RetryMax != 5 {
		t.Fatalf("UnitConfig = %+v, %v", cfg, err)
	}
	if _, err := wire.UnitConfig(nil); err == nil {
		t.Fatal("UnitConfig without values reported no error")
	}
}
