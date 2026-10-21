// SPDX-License-Identifier: Apache-2.0

package wire_test

import (
	"testing"
	"time"

	authzjobs "github.com/OWNER/bothy/services/server/internal/authz/adapters/jobs"
	"github.com/OWNER/bothy/services/server/internal/platform/jobs"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

// TestAuthzExpiryConfig binds the three keys of the expiry jobs with their defaults (07 section
// 14) and with values from the environment.
func TestAuthzExpiryConfig(t *testing.T) {
	t.Parallel()
	cfg, err := wire.AuthzExpiryConfig(syncValues(t))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Interval != time.Minute || cfg.ReminderLead != 48*time.Hour || cfg.BatchMax != 500 {
		t.Fatalf("defaults %+v", cfg)
	}
	cfg, err = wire.AuthzExpiryConfig(syncValues(t, "BOTHY_AUTHZ__EXPIRY_SWEEP_BATCH_MAX=2", "BOTHY_AUTHZ__EXPIRY_SWEEP_INTERVAL=30s",
		"BOTHY_AUTHZ__EXPIRY_REMINDER_LEAD=24h"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Interval != 30*time.Second || cfg.ReminderLead != 24*time.Hour || cfg.BatchMax != 2 {
		t.Fatalf("set %+v", cfg)
	}
}

// TestAuthzExpiryJobsRegistered: the job registry of every role holds authz.expiry_sweep and
// authz.expiry_reminder on queue maintenance, periodic every authz.expiry_sweep_interval; only
// worker and all run them.
func TestAuthzExpiryJobsRegistered(t *testing.T) {
	t.Parallel()
	reg, err := wire.NewJobRegistry(&wire.JobModules{PartitionInterval: time.Minute, CleanupInterval: time.Hour, ExpiryInterval: 2 * time.Minute, KeyRotationInterval: time.Minute, AuthCleanupInterval: time.Hour, ChainVerifyInterval: time.Hour, StallInterval: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{authzjobs.KindExpirySweep, authzjobs.KindExpiryReminder} {
		if q, ok := reg.Queue(kind); !ok || q != jobs.QueueMaintenance {
			t.Fatalf("queue of %s = %q, %v", kind, q, ok)
		}
		found := false
		for _, p := range reg.Periodic() {
			if p.Args.Kind() == kind {
				found = p.Interval == 2*time.Minute
			}
		}
		if !found {
			t.Fatalf("%s not periodic every 2m: %+v", kind, reg.Periodic())
		}
	}
}
