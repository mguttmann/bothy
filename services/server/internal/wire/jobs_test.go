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
// (02 "Usage"). Register checks it at the start; this test makes a bad type fail in the gate
// already. Empty until M1-16 registers the first handler.
func TestWireJobArgsOnlyIDs(t *testing.T) {
	t.Parallel()
	reg, err := wire.NewJobRegistry(&wire.JobModules{TerminalSweepInterval: time.Minute, PartitionInterval: time.Minute, CleanupInterval: time.Hour, ExpiryInterval: time.Minute, KeyRotationInterval: time.Minute, AuthCleanupInterval: time.Hour, ChainVerifyInterval: time.Hour, StallInterval: time.Minute})
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
	if cfg.Schema != "jobs" || cfg.SoftStopTimeout != 20*time.Second || cfg.StatementTimeout <= 0 || len(cfg.MaxWorkers) != len(want) {
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
	m := wire.JobModules{TerminalSweepInterval: time.Minute, PartitionInterval: time.Minute, CleanupInterval: time.Hour, ExpiryInterval: time.Minute, KeyRotationInterval: time.Minute,
		AuthCleanupInterval: 3 * time.Hour, ChainVerifyInterval: time.Hour, StallInterval: time.Minute}
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

// TestJobRegistryHasChainVerify: the registry of every role holds audit.chain_verify on the
// queue maintenance, periodic every audit.checkpoint_interval (02 section 12.3, M1-50), and
// refuses a missing interval.
func TestJobRegistryHasChainVerify(t *testing.T) {
	t.Parallel()
	m := wire.JobModules{TerminalSweepInterval: time.Minute, PartitionInterval: time.Minute, CleanupInterval: time.Hour, ExpiryInterval: time.Minute, KeyRotationInterval: time.Minute,
		AuthCleanupInterval: time.Hour, ChainVerifyInterval: 2 * time.Hour, StallInterval: time.Minute}
	reg, err := wire.NewJobRegistry(&m)
	if err != nil {
		t.Fatal(err)
	}
	if q, ok := reg.Queue("audit.chain_verify"); !ok || q != jobs.QueueMaintenance {
		t.Fatalf("queue = %q, %v", q, ok)
	}
	found := false
	for _, p := range reg.Periodic() {
		found = found || p.Args.Kind() == "audit.chain_verify" && p.Interval == 2*time.Hour
	}
	if !found {
		t.Fatalf("periodic jobs %+v lack audit.chain_verify every 2h", reg.Periodic())
	}
	m.ChainVerifyInterval = 0
	if _, err := wire.NewJobRegistry(&m); err == nil {
		t.Fatal("a registry without audit.checkpoint_interval was accepted")
	}
}

// TestWire_JobsAndSubscriptions: runs.cancel_timeout and runs.stall_check are on the queue
// maintenance, runs.stall_check is periodic every agent.presence_timeout and runs once at the
// start of a worker, and a registry without the interval is refused (M2-09).
func TestWire_JobsAndSubscriptions(t *testing.T) {
	t.Parallel()
	m := wire.JobModules{TerminalSweepInterval: time.Minute, PartitionInterval: time.Minute, CleanupInterval: time.Hour, ExpiryInterval: time.Minute, KeyRotationInterval: time.Minute,
		AuthCleanupInterval: time.Hour, ChainVerifyInterval: time.Hour, StallInterval: 90 * time.Second}
	reg, err := wire.NewJobRegistry(&m)
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"runs.cancel_timeout", "runs.stall_check"} {
		if q, ok := reg.Queue(kind); !ok || q != jobs.QueueMaintenance {
			t.Fatalf("%s: queue = %q, %v", kind, q, ok)
		}
	}
	found := false
	for _, p := range reg.Periodic() {
		found = found || p.Args.Kind() == "runs.stall_check" && p.Interval == 90*time.Second && p.RunOnStart
	}
	if !found {
		t.Fatalf("periodic jobs %+v lack runs.stall_check every 90s", reg.Periodic())
	}
	for _, p := range reg.Periodic() {
		if p.Args.Kind() == "runs.cancel_timeout" {
			t.Fatal("runs.cancel_timeout is scheduled per request, not periodic")
		}
	}
	m.StallInterval = 0
	if _, err := wire.NewJobRegistry(&m); err == nil {
		t.Fatal("a registry without agent.presence_timeout was accepted")
	}
}

// TestJobRegistryHasNotifyJobs: the six jobs of notify are on the queue delivery in every role
// (M3-19, notify.gateway_send M3-23, 02 12.2).
func TestJobRegistryHasNotifyJobs(t *testing.T) {
	t.Parallel()
	reg, err := wire.NewJobRegistry(&wire.JobModules{TerminalSweepInterval: time.Minute, PartitionInterval: time.Minute, CleanupInterval: time.Hour, ExpiryInterval: time.Minute,
		KeyRotationInterval: time.Minute, AuthCleanupInterval: time.Hour, ChainVerifyInterval: time.Hour, StallInterval: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"notify.push_send", "notify.gateway_send", "notify.coalesce_window", "notify.coalesce_flush", "notify.badge_sync", "notify.badge_sync_hold"} {
		if q, ok := reg.Queue(kind); !ok || q != jobs.QueueDelivery {
			t.Errorf("%s: queue %q, %v", kind, q, ok)
		}
	}
}
