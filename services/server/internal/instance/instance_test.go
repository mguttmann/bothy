// SPDX-License-Identifier: Apache-2.0

package instance_test

import (
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/instance"
)

func TestRecord_SetupCompleted(t *testing.T) {
	t.Parallel()
	if (instance.Record{}).SetupCompleted() {
		t.Error("a record without setup_completed_at reports setup completed")
	}
	r := instance.Record{SetupCompletedAt: time.Date(2026, 10, 12, 8, 0, 0, 0, time.UTC)}
	if !r.SetupCompleted() {
		t.Error("a record with setup_completed_at reports setup not completed")
	}
}

func TestCounterNames(t *testing.T) {
	t.Parallel()
	for name, want := range map[instance.CounterName]string{
		instance.CounterEventSeq: "event_seq", instance.CounterAuditSeq: "audit_seq",
		instance.CounterPermissionVersion: "permission_version",
	} {
		if string(name) != want {
			t.Errorf("counter %q, want %q", name, want)
		}
	}
}
