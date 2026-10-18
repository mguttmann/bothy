// SPDX-License-Identifier: Apache-2.0

package jobs_test

import (
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/jobs"
	"github.com/OWNER/bothy/services/server/internal/platform/jobs/internal/fakemodule"
	"github.com/OWNER/bothy/services/server/internal/platform/otel/metrics/metricstest"
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

	deadline := time.Now().Add(waitLong)
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
