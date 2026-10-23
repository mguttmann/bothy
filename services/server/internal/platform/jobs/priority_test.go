// SPDX-License-Identifier: Apache-2.0

package jobs_test

import (
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/jobs"
	"github.com/OWNER/bothy/services/server/internal/platform/jobs/internal/fakemodule"
)

// TestInsertTx_PriorityFetchOrder holds InsertOptions.Priority (M8-11 design note 6 c): a job
// inserted with PriorityLowest waits behind a job inserted later without options, because a worker
// with one free slot fetches the available jobs of its queue by priority first; the stored
// priorities are 4 and 1.
func TestInsertTx_PriorityFetchOrder(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ran := make(chan jobs.Job[fakemodule.Echo], 2)
	reg := registry(t, recorder(ran), jobs.HandlerOptions{})
	api := e.runtime(t, testConfig(t, time.Second), jobs.RoleAPI, reg)
	e.insert(t, api.Inserter(), fakemodule.Echo{Target: ids.New(), Seq: 1}, &jobs.InsertOptions{Priority: jobs.PriorityLowest})
	e.insert(t, api.Inserter(), fakemodule.Echo{Target: ids.New(), Seq: 2}, nil)
	var low, high int
	e.query(t, "SELECT min(priority), max(priority) FROM "+table("river_job"), &high, &low)
	if low != jobs.PriorityLowest || high != jobs.PriorityHighest {
		t.Fatalf("stored priorities %d and %d", low, high)
	}
	cfg := testConfig(t, time.Second)
	cfg.MaxWorkers[jobs.QueueDefault] = 1
	e.runtime(t, cfg, jobs.RoleWorker, registry(t, recorder(ran), jobs.HandlerOptions{}))
	if first := receive(t, ran); first.Args.Seq != 2 {
		t.Fatalf("the worker fetched job %d first, want the job of the highest priority", first.Args.Seq)
	}
	if second := receive(t, ran); second.Args.Seq != 1 {
		t.Fatalf("second job %d", second.Args.Seq)
	}
}
