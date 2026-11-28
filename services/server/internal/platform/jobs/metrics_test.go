// SPDX-License-Identifier: Apache-2.0

package jobs_test

import (
	"errors"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/jobs"
	"github.com/OWNER/bothy/services/server/internal/platform/jobs/internal/fakemodule"
	"github.com/OWNER/bothy/services/server/internal/platform/otel/metrics/metricstest"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
)

// TestJobsMetrics_DurationAndQueue: every attempt records bothy.job.duration with its kind and
// outcome; bothy.job.queue observes per registered kind the jobs in the states available,
// scheduled and retryable (a scheduled job counts, a completed one does not).
func TestJobsMetrics_DurationAndQueue(t *testing.T) {
	metricstest.Strict(t)
	e := newEnv(t)
	mp, reader := metricstest.Provider(t)
	cfg := testConfig(t, time.Second)
	cfg.MeterProvider, cfg.StatementTimeout = mp, 5*time.Second
	ran := make(chan jobs.Job[fakemodule.Echo], 1)
	rt := e.runtime(t, cfg, jobs.RoleWorker, registry(t, recorder(ran), jobs.HandlerOptions{}))
	e.insert(t, rt.Inserter(), fakemodule.Echo{Target: ids.New(), Seq: 1}, &jobs.InsertOptions{ScheduledAt: time.Now().Add(time.Hour)})
	e.insert(t, rt.Inserter(), fakemodule.Echo{Target: ids.New(), Seq: 2}, nil)
	receive(t, ran)

	deadline := testkit.Deadline()
	for {
		got := metricstest.Collect(t, reader)
		done := metricstest.Sum(got["bothy.job.duration"], map[string]string{"bothy.job.kind": "fakemodule.echo", "bothy.outcome": "completed"})
		queued := metricstest.Sum(got["bothy.job.queue"], map[string]string{"bothy.job.kind": "fakemodule.echo"})
		if done == 1 && queued == 1 {
			for _, p := range metricstest.Points(got["bothy.job.duration"]) {
				if len(p.Attrs) != 2 {
					t.Errorf("duration attributes %v, want bothy.job.kind and bothy.outcome", p.Attrs)
				}
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("job duration completed = %v, queue = %v; want 1 and 1", done, queued)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// TestJobsMetrics_DiscardedOnLastFailedAttempt (AUD-PKG-R2-owner-server, Refs #1456): a failed
// attempt with attempts left counts nothing in bothy.job.discarded; a failed last attempt, also a
// panicking one, counts one with bothy.job.kind, the series BothyJobFinallyFailed reads.
func TestJobsMetrics_DiscardedOnLastFailedAttempt(t *testing.T) {
	metricstest.Strict(t)
	mp, reader := metricstest.Provider(t)
	if err := jobs.WorkFailingWithMetricsForTest(mp, errors.New("x"), 1, 3); err == nil {
		t.Fatal("Work = nil")
	}
	got := metricstest.Collect(t, reader)
	if len(metricstest.Points(got["bothy.job.discarded"])) != 1 {
		t.Fatalf("discarded points %v, want the series of the kind at 0", metricstest.Points(got["bothy.job.discarded"]))
	}
	if n := metricstest.Sum(got["bothy.job.discarded"], map[string]string{"bothy.job.kind": "jobs.timeout"}); n != 0 {
		t.Fatalf("discarded after a retried attempt = %v, want 0", n)
	}
	if err := jobs.WorkFailingWithMetricsForTest(mp, errors.New("x"), 3, 3); err == nil {
		t.Fatal("Work = nil")
	}
	if err := jobs.WorkFailingWithMetricsForTest(mp, nil, 2, 2); err == nil {
		t.Fatal("Work of a panic = nil")
	}
	got = metricstest.Collect(t, reader)
	if n := metricstest.Sum(got["bothy.job.discarded"], map[string]string{"bothy.job.kind": "jobs.timeout"}); n != 2 {
		t.Fatalf("discarded after two failed last attempts = %v, want 2", n)
	}
}

// TestJobsMetrics_SnoozeIsSnoozed: a snoozed last attempt records bothy.job.duration with the
// outcome snoozed and counts nothing in bothy.job.discarded, because River keeps the job.
func TestJobsMetrics_SnoozeIsSnoozed(t *testing.T) {
	metricstest.Strict(t)
	mp, reader := metricstest.Provider(t)
	if err := jobs.WorkFailingWithMetricsForTest(mp, jobs.Snooze(time.Second), 3, 3); err == nil {
		t.Fatal("Work = nil, want River's snooze")
	}
	got := metricstest.Collect(t, reader)
	if n := metricstest.Sum(got["bothy.job.duration"], map[string]string{"bothy.job.kind": "jobs.timeout", "bothy.outcome": "snoozed"}); n != 1 {
		t.Errorf("snoozed durations = %v, want 1", n)
	}
	if n := metricstest.Sum(got["bothy.job.discarded"], map[string]string{"bothy.job.kind": "jobs.timeout"}); n != 0 {
		t.Errorf("discarded after a snooze = %v, want 0", n)
	}
}
