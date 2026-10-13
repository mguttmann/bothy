// SPDX-License-Identifier: Apache-2.0

package wire_test

import (
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/platform/jobs"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

// TestWireJobArgsOnlyIDs: every kind of the real registry has argument types that carry only IDs
// (02 "Einsatz"). Register checks it at the start; this test makes a bad type fail in the gate
// already. Empty until M1-16 registers the first handler.
func TestWireJobArgsOnlyIDs(t *testing.T) {
	t.Parallel()
	reg, err := wire.NewJobRegistry(&wire.JobModules{PartitionInterval: time.Minute, CleanupInterval: time.Hour, ExpiryInterval: time.Minute, KeyRotationInterval: time.Minute, AuthCleanupInterval: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	for kind, typ := range reg.ArgTypes() {
		if err := jobs.ValidateArgsType(typ); err != nil {
			t.Errorf("kind %s: %v", kind, err)
		}
	}
}

func TestJobsConfigBindsDefaults(t *testing.T) {
	t.Parallel()
	v, err := config.Load(schema(t), config.Sources{Env: []string{testPublicURL, "BOTHY_JOBS__QUEUE__FILES__MAX_WORKERS=5"}})
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := wire.JobsConfig(v)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]int{"default": 8, "files": 5, "delivery": 16, "maintenance": 2, "privacy": 1}
	if cfg.Schema != "jobs" || cfg.SoftStopTimeout != 20*time.Second || len(cfg.MaxWorkers) != len(want) {
		t.Fatalf("JobsConfig = %+v", cfg)
	}
	for q, n := range want {
		if cfg.MaxWorkers[q] != n {
			t.Errorf("MaxWorkers[%s] = %d, want %d", q, cfg.MaxWorkers[q], n)
		}
	}
	for _, q := range jobs.Queues() {
		if _, ok := cfg.MaxWorkers[q]; !ok {
			t.Errorf("queue %s has no parallelism", q)
		}
	}
}

func TestJobRoleMapping(t *testing.T) {
	t.Parallel()
	for role, want := range map[wire.Role]jobs.Role{wire.RoleAPI: jobs.RoleAPI, wire.RoleWorker: jobs.RoleWorker, wire.RoleAll: jobs.RoleAll} {
		if got := wire.JobRole(role); got != want {
			t.Errorf("JobRole(%s) = %s, want %s", role, got, want)
		}
	}
}

// TestJobRegistryHasAuthCleanup: the registry of every role holds identity.auth_cleanup on the
// queue maintenance, periodic every auth.cleanup_interval (02 section 12.3, M1-35), and refuses a
// missing interval.
func TestJobRegistryHasAuthCleanup(t *testing.T) {
	t.Parallel()
	m := wire.JobModules{PartitionInterval: time.Minute, CleanupInterval: time.Hour, ExpiryInterval: time.Minute, KeyRotationInterval: time.Minute,
		AuthCleanupInterval: 3 * time.Hour}
	reg, err := wire.NewJobRegistry(&m)
	if err != nil {
		t.Fatal(err)
	}
	if q, ok := reg.Queue("identity.auth_cleanup"); !ok || q != jobs.QueueMaintenance {
		t.Fatalf("queue = %q, %v", q, ok)
	}
	found := false
	for _, p := range reg.Periodic() {
		found = found || p.Args.Kind() == "identity.auth_cleanup" && p.Interval == 3*time.Hour
	}
	if !found {
		t.Fatalf("periodic jobs %+v lack identity.auth_cleanup every 3h", reg.Periodic())
	}
	m.AuthCleanupInterval = 0
	if _, err := wire.NewJobRegistry(&m); err == nil {
		t.Fatal("a registry without auth.cleanup_interval was accepted")
	}
}
