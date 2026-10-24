// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"slices"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/platform/jobs"
)

// TestStartSequenceModuleSteps: every role starts the module steps 6a to 6e in the order Serve
// documents (audit, sync, authz, settings, identity; identity reads locale.default through the
// settings, M1-46), then the signing keys (M1-32) and the routes of step
// 7, so the device handlers of identity (M1-33) and the key handlers exist when the routes are
// mounted (docs/DECISIONS.md D-M2-01-11), and the keys before the jobs that rotate them. The session
// part of identity follows the signing keys, which sign its access tokens, and precedes the routes,
// whose chain authenticates with it (M1-34).
func TestStartSequenceModuleSteps(t *testing.T) {
	t.Parallel()
	want := []string{"audit", "sync", "authz", "settings", "identity", "signing_keys", "audit_log", "sessions", "routes", "jobs"}
	for _, role := range []Role{RoleAPI, RoleWorker, RoleAll} {
		names := componentNames(t, role)
		last := -1
		for _, name := range want {
			i := slices.Index(names, name)
			if i < 0 || i <= last {
				t.Fatalf("%s: start sequence %v, want %v in this order", role, names, want)
			}
			last = i
		}
		if got := slices.Index(names, "settings"); got != slices.Index(names, "authz")+1 {
			t.Errorf("%s: settings is not right after authz: %v", role, names)
		}
		if got := slices.Index(names, "identity"); got != slices.Index(names, "settings")+1 {
			t.Errorf("%s: identity is not right after settings: %v", role, names)
		}
	}
}

// TestStartSequenceStatsMeasure (M6-13): every role builds the own measurement in its step
// stats_measure right after the schema and before the first module that records (audit is the
// first module step), so every module receives the recorder; and the job registry of every role
// holds stats.retention on the queue maintenance, hourly and at the start (River runs it in
// worker and all).
func TestStartSequenceStatsMeasure(t *testing.T) {
	t.Parallel()
	for _, role := range []Role{RoleAPI, RoleWorker, RoleAll} {
		names := componentNames(t, role)
		i := slices.Index(names, stepStatsMeasure)
		if i < 0 || names[i-1] != "schema" || i > slices.Index(names, "audit") {
			t.Fatalf("%s: start sequence %v, want stats_measure right after schema and before audit", role, names)
		}
	}
	reg, err := NewJobRegistry(&JobModules{TerminalSweepInterval: time.Minute, PartitionInterval: time.Minute, CleanupInterval: time.Hour,
		ExpiryInterval: time.Minute, KeyRotationInterval: time.Minute, AuthCleanupInterval: time.Hour, ChainVerifyInterval: time.Hour,
		StallInterval: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	if q, ok := reg.Queue("stats.retention"); !ok || q != jobs.QueueMaintenance {
		t.Fatalf("stats.retention: queue %q, %v", q, ok)
	}
	if !slices.ContainsFunc(reg.Periodic(), func(p jobs.PeriodicJob) bool {
		return p.Args.Kind() == "stats.retention" && p.Interval == time.Hour && p.RunOnStart
	}) {
		t.Fatal("stats.retention is not periodic hourly with RunOnStart")
	}
}
