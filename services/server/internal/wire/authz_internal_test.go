// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"testing"
	"time"
)

func TestAuthzConfig_FromValues(t *testing.T) {
	t.Parallel()
	cfg, err := AuthzConfig(loadValues(t))
	if err != nil || cfg.SnapshotCacheMaxEntries != 10000 || cfg.StepUpMaxAge != 5*time.Minute {
		t.Fatalf("defaults: %+v %v", cfg, err)
	}
	cfg, err = AuthzConfig(loadValues(t, "BOTHY_AUTHZ__SNAPSHOT_CACHE_MAX_ENTRIES=3", "BOTHY_AUTH__STEP_UP_MAX_AGE=90s"))
	if err != nil || cfg.SnapshotCacheMaxEntries != 3 || cfg.StepUpMaxAge != 90*time.Second {
		t.Fatalf("values: %+v %v", cfg, err)
	}
	s := &server{}
	if s.authorizerOf() != nil {
		t.Fatal("no authorizer before the start step")
	}
}
