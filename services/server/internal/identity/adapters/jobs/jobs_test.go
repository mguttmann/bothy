// SPDX-License-Identifier: Apache-2.0

package jobs_test

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	identityjobs "github.com/OWNER/bothy/services/server/internal/identity/adapters/jobs"
	"github.com/OWNER/bothy/services/server/internal/platform/jobs"
)

// fakeCleanup counts the runs and returns err.
type fakeCleanup struct {
	runs int
	err  error
}

func (f *fakeCleanup) Run(context.Context) (int64, error) {
	f.runs++
	return 0, f.err
}

func TestRegister_PeriodicOnTheMaintenanceQueue(t *testing.T) {
	t.Parallel()
	reg := jobs.NewRegistry()
	if err := identityjobs.Register(reg, &fakeCleanup{}, time.Hour); err != nil {
		t.Fatal(err)
	}
	if q, ok := reg.Queue(identityjobs.KindAuthCleanup); !ok || q != jobs.QueueMaintenance {
		t.Fatalf("queue = %q, %v", q, ok)
	}
	p := reg.Periodic()
	if len(p) != 1 || p[0].Args.Kind() != "identity.auth_cleanup" || p[0].Interval != time.Hour {
		t.Fatalf("periodic = %+v", p)
	}
	if err := jobs.ValidateArgsType(reflect.TypeFor[identityjobs.AuthCleanup]()); err != nil {
		t.Fatal(err)
	}
	if err := identityjobs.Register(jobs.NewRegistry(), &fakeCleanup{}, 0); err == nil {
		t.Fatal("an interval of 0 was accepted")
	}
}

func TestHandler_RunsTheUseCase(t *testing.T) {
	t.Parallel()
	c := &fakeCleanup{}
	h := identityjobs.Handler{Cleanup: c}
	if err := h.Work(context.Background(), jobs.Job[identityjobs.AuthCleanup]{}); err != nil || c.runs != 1 {
		t.Fatalf("Work = %v, runs %d", err, c.runs)
	}
	c.err = context.Canceled
	if err := h.Work(context.Background(), jobs.Job[identityjobs.AuthCleanup]{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("Work = %v, want the error so River retries", err)
	}
}
