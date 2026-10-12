// SPDX-License-Identifier: Apache-2.0

package wire_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/platform/jobs"
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
		cfg.Tail.ReconnectBackoffCap != 10*time.Second || cfg.MaintenanceInterval != 10*time.Minute {
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
	reg, err := wire.NewJobRegistry(&wire.JobModules{Partitions: nil, PartitionInterval: cfg.MaintenanceInterval})
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
