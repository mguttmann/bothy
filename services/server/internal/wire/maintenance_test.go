// SPDX-License-Identifier: Apache-2.0

package wire_test

import (
	"slices"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/platform/jobs"
	runsjobs "github.com/OWNER/bothy/services/server/internal/runs/adapters/jobs"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

// TestMaintenanceFrom (M7-08): server.maintenance_mode is off by default and read_only turns the
// mode on; server.maintenance_retry_after binds as a duration.
func TestMaintenanceFrom(t *testing.T) {
	t.Parallel()
	for env, want := range map[string]wire.Maintenance{
		"": {RetryAfter: time.Minute},
		"BOTHY_SERVER__MAINTENANCE_MODE=read_only": {ReadOnly: true, RetryAfter: time.Minute},
		"BOTHY_SERVER__MAINTENANCE_MODE=off":       {RetryAfter: time.Minute},
	} {
		sources := []string{testPublicURL}
		if env != "" {
			sources = append(sources, env)
		}
		v, err := config.Load(schema(t), config.Sources{Env: sources})
		if err != nil {
			t.Fatal(err)
		}
		got, err := wire.MaintenanceFrom(v)
		if err != nil || got != want {
			t.Errorf("%q: %+v, %v; want %+v", env, got, err, want)
		}
	}
	if _, err := config.Load(schema(t), config.Sources{Env: []string{testPublicURL, "BOTHY_SERVER__MAINTENANCE_MODE=on"}}); err == nil {
		t.Error("server.maintenance_mode=on loaded")
	}
}

// TestMaintenanceJobs (02 12.2): in maintenance mode the worker fetches from the queues delivery
// and maintenance and runs exactly the kinds of the queue delivery and runs.cancel_timeout.
func TestMaintenanceJobs(t *testing.T) {
	t.Parallel()
	reg, err := wire.NewJobRegistry(&wire.JobModules{RetentionInterval: time.Hour, StatusProbeInterval: time.Minute, TerminalSweepInterval: time.Minute, FilesGCInterval: time.Minute, FilesExtractInterval: time.Hour, PartitionInterval: time.Minute, CleanupInterval: time.Hour, ExpiryInterval: time.Minute, KeyRotationInterval: time.Minute, AuthCleanupInterval: time.Hour, ChainVerifyInterval: time.Hour, StallInterval: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	m := wire.MaintenanceJobs(reg, time.Minute)
	if !slices.Equal(m.Queues, []string{jobs.QueueDelivery, jobs.QueueMaintenance}) || m.Pause != time.Minute {
		t.Fatalf("queues %v, pause %v", m.Queues, m.Pause)
	}
	for kind := range reg.ArgTypes() {
		q, _ := reg.Queue(kind)
		want := q == jobs.QueueDelivery || kind == runsjobs.KindCancelTimeout
		if got := slices.Contains(m.Kinds, kind); got != want {
			t.Errorf("kind %s (queue %s) runs %v, want %v", kind, q, got, want)
		}
	}
	for _, p := range reg.Periodic() {
		if slices.Contains(m.Kinds, p.Args.Kind()) {
			t.Errorf("periodic job %s runs in maintenance mode", p.Args.Kind())
		}
	}
}
