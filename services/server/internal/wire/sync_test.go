// SPDX-License-Identifier: Apache-2.0

package wire_test

import (
	"net/http"
	"reflect"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
	"github.com/OWNER/bothy/services/server/internal/platform/jobs"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	syncjobs "github.com/OWNER/bothy/services/server/internal/sync/adapters/jobs"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

// syncValues loads the configuration with extra environment entries.
func syncValues(t *testing.T, env ...string) *config.Values {
	t.Helper()
	v, err := config.Load(schema(t), config.Sources{Env: append([]string{testPublicURL}, env...)})
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestSyncConfigBindsDefaults(t *testing.T) {
	t.Parallel()
	cfg, err := wire.SyncConfigFrom(syncValues(t))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Appender.PayloadMaxBytes != 65536 || cfg.Partitions.Size != 1_000_000 || cfg.Partitions.Ahead != 3 ||
		cfg.Partitions.Retention != 30*24*time.Hour || cfg.Partitions.LockTimeout != 2*time.Second ||
		cfg.Tail.PollInterval != time.Second || cfg.Tail.BatchSize != 1000 ||
		cfg.Tail.ReconnectBackoffCap != 10*time.Second || cfg.MaintenanceInterval != 10*time.Minute ||
		cfg.Idempotency.TTL != 24*time.Hour || cfg.Cleanup.Interval != time.Hour || cfg.Cleanup.BatchSize != 1000 {
		t.Fatalf("SyncConfig = %+v", cfg)
	}
}

// TestPartitionMaintenanceJobRegistered: the job registry of every role holds
// sync.partition_maintenance on queue maintenance, periodic with
// sync.partition_maintenance_interval and RunOnStart (02 "Job catalogue"), and its arguments carry
// only IDs.
func TestPartitionMaintenanceJobRegistered(t *testing.T) {
	t.Parallel()
	cfg, err := wire.SyncConfigFrom(syncValues(t, "BOTHY_SYNC__PARTITION_MAINTENANCE_INTERVAL=7m"))
	if err != nil {
		t.Fatal(err)
	}
	reg, err := wire.NewJobRegistry(&wire.JobModules{Partitions: nil, PartitionInterval: cfg.MaintenanceInterval, CleanupInterval: cfg.Cleanup.Interval})
	if err != nil {
		t.Fatal(err)
	}
	if q, ok := reg.Queue(syncjobs.KindPartitionMaintenance); !ok || q != jobs.QueueMaintenance {
		t.Fatalf("queue of %s = %q, %v", syncjobs.KindPartitionMaintenance, q, ok)
	}
	var found bool
	for _, p := range reg.Periodic() {
		if p.Args.Kind() == syncjobs.KindPartitionMaintenance {
			found = p.Interval == 7*time.Minute && p.RunOnStart
		}
	}
	if !found {
		t.Fatalf("periodic jobs = %+v, want %s every 7m with RunOnStart", reg.Periodic(), syncjobs.KindPartitionMaintenance)
	}
	typ, ok := reg.ArgTypes()[syncjobs.KindPartitionMaintenance]
	if !ok || typ != reflect.TypeFor[syncjobs.PartitionMaintenance]() {
		t.Fatalf("argument type = %v", typ)
	}
	if err := jobs.ValidateArgsType(typ); err != nil {
		t.Fatal(err)
	}
}

// TestIdempotencyCleanupJobRegistered: the job registry of every role holds
// sync.idempotency_cleanup on queue maintenance, periodic every sync.idempotency_cleanup_interval
// (02 "Job catalogue"), with arguments that carry only IDs; only worker and all run it.
func TestIdempotencyCleanupJobRegistered(t *testing.T) {
	t.Parallel()
	cfg, err := wire.SyncConfigFrom(syncValues(t, "BOTHY_SYNC__IDEMPOTENCY_CLEANUP_INTERVAL=15m", "BOTHY_SYNC__IDEMPOTENCY_CLEANUP_BATCH_SIZE=50"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Cleanup.BatchSize != 50 {
		t.Fatalf("batch size %d, want 50", cfg.Cleanup.BatchSize)
	}
	reg, err := wire.NewJobRegistry(&wire.JobModules{PartitionInterval: cfg.MaintenanceInterval, CleanupInterval: cfg.Cleanup.Interval})
	if err != nil {
		t.Fatal(err)
	}
	if q, ok := reg.Queue(syncjobs.KindIdempotencyCleanup); !ok || q != jobs.QueueMaintenance {
		t.Fatalf("queue of %s = %q, %v", syncjobs.KindIdempotencyCleanup, q, ok)
	}
	var found bool
	for _, p := range reg.Periodic() {
		if p.Args.Kind() == syncjobs.KindIdempotencyCleanup {
			found = p.Interval == 15*time.Minute
		}
	}
	if !found {
		t.Fatalf("periodic jobs = %+v, want %s every 15m", reg.Periodic(), syncjobs.KindIdempotencyCleanup)
	}
	if err := jobs.ValidateArgsType(reg.ArgTypes()[syncjobs.KindIdempotencyCleanup]); err != nil {
		t.Fatal(err)
	}
}

// TestInstallIdempotency_FillsTheStage: the start installs StageIdempotency in the chain of api
// and all (02 "Kette im api-Knoten" step 10), so the slot is taken afterwards.
func TestInstallIdempotency_FillsTheStage(t *testing.T) {
	t.Parallel()
	v := syncValues(t)
	cfg, err := wire.SyncConfigFrom(v)
	if err != nil {
		t.Fatal(err)
	}
	chain, err := wire.NewChain(httpserver.DefaultChainConfig("https://chat.example.com"), nil)
	if err != nil {
		t.Fatal(err)
	}
	sy, err := wire.NewSync(nil, pg.NewUnitOfWork(nil, pg.UnitConfig{}, txn.NewRegistry()), cfg, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := wire.InstallIdempotency(chain, sy, cfg); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if recover() == nil {
			t.Error("StageIdempotency is still free after InstallIdempotency")
		}
	}()
	chain.Use(httpserver.StageIdempotency, func(next http.Handler) http.Handler { return next })
}
