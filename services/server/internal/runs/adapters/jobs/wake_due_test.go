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

// wakeDuer records the wake-ups it was asked for.
type wakeDuer struct{ got []ids.ID }

func (d *wakeDuer) WakeDue(_ context.Context, id ids.ID) error {
	d.got = append(d.got, id)
	return nil
}

// TestWakeDueJob: the kind runs.wake_due on the queue default with only the wake-up as argument
// (02 12.1); the handler runs the use case of its wake-up; an unbound scheduler refuses (AR-04).
func TestWakeDueJob(t *testing.T) {
	t.Parallel()
	reg := jobs.NewRegistry()
	d := &wakeDuer{}
	if err := runsjobs.RegisterWakeDue(reg, d); err != nil {
		t.Fatal(err)
	}
	if q, ok := reg.Queue(runsjobs.KindWakeDue); !ok || q != jobs.QueueDefault {
		t.Fatalf("queue %q %v", q, ok)
	}
	if err := jobs.ValidateArgsType(reflect.TypeFor[runsjobs.WakeDue]()); err != nil {
		t.Fatal(err)
	}
	wake := ids.New()
	if err := (runsjobs.WakeDueHandler{Duer: d}).Work(context.Background(), jobs.Job[runsjobs.WakeDue]{Args: runsjobs.WakeDue{WakeID: wake}}); err != nil ||
		len(d.got) != 1 || d.got[0] != wake {
		t.Fatalf("Work: %v %v", err, d.got)
	}
	var s runsjobs.WakeScheduler
	if err := s.ScheduleWakeDue(context.Background(), wake, time.Now()); errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("unbound scheduler: %v", err)
	}
}
