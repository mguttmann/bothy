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

// releaser records the runs it was asked for.
type releaser struct{ got []ids.ID }

func (r *releaser) InvocationHoldTimeout(_ context.Context, id ids.ID) error {
	r.got = append(r.got, id)
	return nil
}

// TestInvocationHoldTimeoutJob: the kind runs.invocation_hold_timeout on the queue default with
// arguments of IDs only; the handler runs the use case of its run; an unbound scheduler refuses.
func TestInvocationHoldTimeoutJob(t *testing.T) {
	t.Parallel()
	reg := jobs.NewRegistry()
	r := &releaser{}
	if err := runsjobs.RegisterHold(reg, r); err != nil {
		t.Fatal(err)
	}
	if q, ok := reg.Queue(runsjobs.KindInvocationHoldTimeout); !ok || q != jobs.QueueDefault || runsjobs.KindInvocationHoldTimeout != "runs.invocation_hold_timeout" {
		t.Fatalf("queue %q %v", q, ok)
	}
	if err := jobs.ValidateArgsType(reflect.TypeFor[runsjobs.InvocationHoldTimeout]()); err != nil {
		t.Fatal(err)
	}
	run := ids.New()
	err := (runsjobs.HoldHandler{Releaser: r}).Work(context.Background(), jobs.Job[runsjobs.InvocationHoldTimeout]{Args: runsjobs.InvocationHoldTimeout{RunID: run}})
	if err != nil || len(r.got) != 1 || r.got[0] != run {
		t.Fatalf("Work: %v %v", err, r.got)
	}
	var s runsjobs.HoldScheduler
	if err := s.ScheduleInvocationHoldTimeout(context.Background(), run, time.Now()); errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("unbound scheduler: %v", err)
	}
}
