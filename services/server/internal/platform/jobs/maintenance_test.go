// SPDX-License-Identifier: Apache-2.0

package jobs_test

import (
	"errors"
	"testing"
	"time"

	"github.com/riverqueue/river"

	"github.com/OWNER/bothy/services/server/internal/platform/jobs"
)

// TestMaintenance_WorkerHoldsOtherKinds (M7-08, 02 12.2): in maintenance mode a kind that does not
// run is held with River's snooze for the pause and its handler never runs; a kind that runs gets
// the write exemption of maintenance mode; outside the mode the handler runs without it.
func TestMaintenance_WorkerHoldsOtherKinds(t *testing.T) {
	t.Parallel()
	ran, _, err := jobs.WorkInMaintenanceForTest(&jobs.Maintenance{Queues: []string{jobs.QueueDelivery}, Kinds: []string{"other.kind"}, Pause: time.Minute})
	var snooze *river.JobSnoozeError
	if ran || !errors.As(err, &snooze) || snooze.Duration != time.Minute {
		t.Fatalf("held kind: ran %v, error %v; want a snooze of one minute", ran, err)
	}
	ran, exempt, err := jobs.WorkInMaintenanceForTest(&jobs.Maintenance{Queues: []string{jobs.QueueDelivery}, Kinds: []string{"jobs.timeout"}, Pause: time.Minute})
	if err != nil || !ran || !exempt {
		t.Fatalf("running kind: error %v, ran %v, exempt %v", err, ran, exempt)
	}
	if ran, exempt, err = jobs.WorkInMaintenanceForTest(nil); err != nil || !ran || exempt {
		t.Fatalf("outside the mode: error %v, ran %v, exempt %v", err, ran, exempt)
	}
}

// TestMaintenance_ConfigRefuses: a pause that is not positive or an unknown queue is a start error.
func TestMaintenance_ConfigRefuses(t *testing.T) {
	t.Parallel()
	base := jobs.Config{Schema: "jobs", SoftStopTimeout: time.Second, MaxWorkers: map[string]int{}}
	for _, q := range jobs.Queues() {
		base.MaxWorkers[q] = 1
	}
	if err := jobs.ValidateForTest(base); err != nil {
		t.Fatalf("valid config: %v", err)
	}
	for name, m := range map[string]*jobs.Maintenance{
		"no pause":      {Queues: []string{jobs.QueueDelivery}},
		"unknown queue": {Queues: []string{"elsewhere"}, Pause: time.Second},
	} {
		c := base
		c.Maintenance = m
		if err := jobs.ValidateForTest(c); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
}
