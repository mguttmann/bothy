// SPDX-License-Identifier: Apache-2.0

package jobs_test

import (
	"context"
	"errors"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/platform/jobs"
	statsjobs "github.com/OWNER/bothy/services/server/internal/stats/adapters/jobs"
	"github.com/OWNER/bothy/services/server/internal/stats/app"
)

// fakeRetention counts runs and returns err.
type fakeRetention struct {
	runs int
	err  error
}

func (f *fakeRetention) Run(context.Context) (app.RetentionResult, error) {
	f.runs++
	return app.RetentionResult{}, f.err
}

// TestRegister_HourlyOnTheMaintenanceQueue holds the registration of stats.retention: queue
// maintenance, periodic every hour and at the start.
func TestRegister_HourlyOnTheMaintenanceQueue(t *testing.T) {
	t.Parallel()
	reg := jobs.NewRegistry()
	if err := statsjobs.Register(reg, &fakeRetention{}); err != nil {
		t.Fatal(err)
	}
	if q, ok := reg.Queue("stats.retention"); !ok || q != jobs.QueueMaintenance {
		t.Fatalf("queue = %q, %v", q, ok)
	}
	p := reg.Periodic()
	if len(p) != 1 || p[0].Args.Kind() != "stats.retention" || p[0].Interval != statsjobs.RetentionInterval || !p[0].RunOnStart {
		t.Fatalf("periodic = %+v", p)
	}
}

// TestHandler_RunsTheUseCase shows that Work runs the use case once and passes its error on.
func TestHandler_RunsTheUseCase(t *testing.T) {
	t.Parallel()
	f := &fakeRetention{err: errors.New("lock timeout")}
	h := statsjobs.Handler{Retention: f}
	if err := h.Work(context.Background(), jobs.Job[statsjobs.Retention]{}); err == nil || f.runs != 1 {
		t.Fatalf("Work = %v after %d runs", err, f.runs)
	}
	if err := (statsjobs.Handler{}).Work(context.Background(), jobs.Job[statsjobs.Retention]{}); err == nil {
		t.Fatal("Work without a use case returned no error")
	}
}
