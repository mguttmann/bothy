// SPDX-License-Identifier: Apache-2.0

package wire_test

import (
	"strings"
	"testing"
	"time"

	billingjobs "github.com/OWNER/bothy/services/server/internal/billing/adapters/jobs"
	"github.com/OWNER/bothy/services/server/internal/platform/jobs"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

// TestServerConfig_HasNoReconcilerKeys (M2-28 design note 3 k, ADR 0055 "Confirmation"): the
// configuration schema of the server has no key under reconciler. and no key whose name contains
// admin_key; the reconciler.* keys belong to the reconciliation app (place "Reconciliation app",
// 10 section 17), so the provider's admin key has no way into the server's configuration.
func TestServerConfig_HasNoReconcilerKeys(t *testing.T) {
	t.Parallel()
	schema, err := wire.ConfigSchema()
	if err != nil {
		t.Fatal(err)
	}
	keys := schema.Keys()
	if len(keys) == 0 {
		t.Fatal("the schema has no keys")
	}
	for _, k := range keys {
		if strings.HasPrefix(k.Name, "reconciler.") || strings.Contains(k.Name, "admin_key") {
			t.Errorf("server configuration key %q belongs to the reconciliation app", k.Name)
		}
	}
}

// TestJobs_BillingReconcileNotPeriodic (02 section 12.3, M2-28): the registry of every role has the
// kind billing.reconcile on the queue default and no periodic entry for it.
func TestJobs_BillingReconcileNotPeriodic(t *testing.T) {
	t.Parallel()
	reg, err := wire.NewJobRegistry(&wire.JobModules{TerminalSweepInterval: time.Minute, FilesGCInterval: time.Minute, PartitionInterval: time.Minute, CleanupInterval: time.Hour,
		ExpiryInterval: time.Minute, KeyRotationInterval: time.Minute, AuthCleanupInterval: time.Hour, ChainVerifyInterval: time.Hour,
		StallInterval: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	if q, ok := reg.Queue(billingjobs.KindReconcile); !ok || q != jobs.QueueDefault {
		t.Fatalf("billing.reconcile: queue %q, %v", q, ok)
	}
	for _, p := range reg.Periodic() {
		if p.Args.Kind() == billingjobs.KindReconcile {
			t.Fatalf("billing.reconcile is periodic: %+v", p)
		}
	}
}
