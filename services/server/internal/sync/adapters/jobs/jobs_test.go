// SPDX-License-Identifier: Apache-2.0

package jobs_test

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/platform/jobs"
	syncjobs "github.com/OWNER/bothy/services/server/internal/sync/adapters/jobs"
	"github.com/OWNER/bothy/services/server/internal/sync/app"
)

// fakeMaintenance counts runs and returns err.
type fakeMaintenance struct {
	runs int
	err  error
}

func (f *fakeMaintenance) Run(context.Context) (app.MaintenanceResult, error) {
	f.runs++
	return app.MaintenanceResult{}, f.err
}

func TestRegister_PeriodicOnTheMaintenanceQueue(t *testing.T) {
	t.Parallel()
	reg := jobs.NewRegistry()
	if err := syncjobs.Register(reg, &fakeMaintenance{}, 10*time.Minute); err != nil {
		t.Fatal(err)
	}
	if q, ok := reg.Queue(syncjobs.KindPartitionMaintenance); !ok || q != jobs.QueueMaintenance {
		t.Fatalf("queue = %q, %v", q, ok)
	}
	p := reg.Periodic()
	if len(p) != 1 || p[0].Args.Kind() != "sync.partition_maintenance" || p[0].Interval != 10*time.Minute || !p[0].RunOnStart {
		t.Fatalf("periodic = %+v", p)
	}
	if err := jobs.ValidateArgsType(reflect.TypeFor[syncjobs.PartitionMaintenance]()); err != nil {
		t.Fatal(err)
	}
	if err := syncjobs.Register(jobs.NewRegistry(), &fakeMaintenance{}, 0); err == nil {
		t.Fatal("an interval of 0 was accepted")
	}
}

func TestHandler_RunsTheUseCase(t *testing.T) {
	t.Parallel()
	m := &fakeMaintenance{}
	h := syncjobs.Handler{Maintenance: m}
	if err := h.Work(context.Background(), jobs.Job[syncjobs.PartitionMaintenance]{}); err != nil || m.runs != 1 {
		t.Fatalf("Work = %v, runs %d", err, m.runs)
	}
	m.err = errors.New("lock timeout")
	if err := h.Work(context.Background(), jobs.Job[syncjobs.PartitionMaintenance]{}); !errors.Is(err, m.err) {
		t.Fatalf("Work = %v, want the error so River retries", err)
	}
}

// fakeCleanup counts runs and returns err.
type fakeCleanup struct {
	runs int
	err  error
}

func (f *fakeCleanup) Run(context.Context) (app.CleanupResult, error) {
	f.runs++
	return app.CleanupResult{}, f.err
}

func TestRegisterIdempotencyCleanup_PeriodicOnTheMaintenanceQueue(t *testing.T) {
	t.Parallel()
	reg := jobs.NewRegistry()
	if err := syncjobs.RegisterIdempotencyCleanup(reg, &fakeCleanup{}, time.Hour); err != nil {
		t.Fatal(err)
	}
	if q, ok := reg.Queue(syncjobs.KindIdempotencyCleanup); !ok || q != jobs.QueueMaintenance {
		t.Fatalf("queue = %q, %v", q, ok)
	}
	p := reg.Periodic()
	if len(p) != 1 || p[0].Args.Kind() != "sync.idempotency_cleanup" || p[0].Interval != time.Hour {
		t.Fatalf("periodic = %+v", p)
	}
	if err := jobs.ValidateArgsType(reflect.TypeFor[syncjobs.IdempotencyCleanup]()); err != nil {
		t.Fatal(err)
	}
	if err := syncjobs.RegisterIdempotencyCleanup(jobs.NewRegistry(), &fakeCleanup{}, 0); err == nil {
		t.Fatal("an interval of 0 was accepted")
	}
}

func TestCleanupHandler_RunsTheUseCase(t *testing.T) {
	t.Parallel()
	c := &fakeCleanup{}
	h := syncjobs.CleanupHandler{Cleanup: c}
	if err := h.Work(context.Background(), jobs.Job[syncjobs.IdempotencyCleanup]{}); err != nil || c.runs != 1 {
		t.Fatalf("Work = %v, runs %d", err, c.runs)
	}
	c.err = context.Canceled
	if err := h.Work(context.Background(), jobs.Job[syncjobs.IdempotencyCleanup]{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("Work = %v, want ctx.Err() so River retries (02 \"Rules for jobs\" rule 2)", err)
	}
}
