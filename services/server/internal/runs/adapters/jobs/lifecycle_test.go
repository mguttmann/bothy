// SPDX-License-Identifier: Apache-2.0

package jobs_test

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/jobs"
	runsjobs "github.com/OWNER/bothy/services/server/internal/runs/adapters/jobs"
)

// lifecycle records the calls of the two lifecycle jobs.
type lifecycle struct {
	cancelled []ids.ID
	stalls    int
}

func (l *lifecycle) CancelTimeout(_ context.Context, id ids.ID) error {
	l.cancelled = append(l.cancelled, id)
	return nil
}

func (l *lifecycle) StallCheck(context.Context) error {
	l.stalls++
	return nil
}

// TestLifecycleJobs (M2-09): runs.cancel_timeout and runs.stall_check are on the queue
// maintenance with arguments of IDs only, each handler runs its use case, a second registration
// on the same registry and a non-positive interval are refused, and an unbound cancel scheduler
// is an internal error.
func TestLifecycleJobs(t *testing.T) {
	t.Parallel()
	reg := jobs.NewRegistry()
	l := &lifecycle{}
	if err := runsjobs.RegisterLifecycle(reg, l, time.Minute); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{runsjobs.KindCancelTimeout, runsjobs.KindStallCheck} {
		if q, ok := reg.Queue(kind); !ok || q != jobs.QueueMaintenance {
			t.Fatalf("%s: queue %q %v", kind, q, ok)
		}
	}
	for _, typ := range []reflect.Type{reflect.TypeFor[runsjobs.CancelTimeout](), reflect.TypeFor[runsjobs.StallCheck]()} {
		if err := jobs.ValidateArgsType(typ); err != nil {
			t.Fatal(err)
		}
	}
	if err := runsjobs.RegisterLifecycle(reg, l, time.Minute); err == nil {
		t.Fatal("a second registration on the same registry was accepted")
	}
	if err := runsjobs.RegisterLifecycle(jobs.NewRegistry(), l, 0); err == nil {
		t.Fatal("a zero interval was accepted")
	}
	run := ids.New()
	ctx := context.Background()
	if err := (runsjobs.CancelTimeoutHandler{Lifecycle: l}).Work(ctx, jobs.Job[runsjobs.CancelTimeout]{Args: runsjobs.CancelTimeout{RunID: run}}); err != nil ||
		len(l.cancelled) != 1 || l.cancelled[0] != run {
		t.Fatalf("cancel timeout: %v %v", err, l.cancelled)
	}
	if err := (runsjobs.StallCheckHandler{Lifecycle: l}).Work(ctx, jobs.Job[runsjobs.StallCheck]{}); err != nil || l.stalls != 1 {
		t.Fatalf("stall check: %v %d", err, l.stalls)
	}
	var s runsjobs.CancelScheduler
	if err := s.Schedule(ctx, run, time.Unix(0, 0)); errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("unbound scheduler: %v", err)
	}
}
