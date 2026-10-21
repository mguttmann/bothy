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

// expirer records the runs it was asked for.
type expirer struct{ got []ids.ID }

func (e *expirer) ExpireQueued(_ context.Context, id ids.ID) error {
	e.got = append(e.got, id)
	return nil
}

// TestQueuedExpiryJob: the kind runs.queued_expiry on the queue default with arguments of IDs only;
// the handler runs the use case of its run; an unbound scheduler refuses.
func TestQueuedExpiryJob(t *testing.T) {
	t.Parallel()
	reg := jobs.NewRegistry()
	e := &expirer{}
	if err := runsjobs.Register(reg, e); err != nil {
		t.Fatal(err)
	}
	if q, ok := reg.Queue(runsjobs.KindQueuedExpiry); !ok || q != jobs.QueueDefault {
		t.Fatalf("queue %q %v", q, ok)
	}
	if err := jobs.ValidateArgsType(reflect.TypeFor[runsjobs.QueuedExpiry]()); err != nil {
		t.Fatal(err)
	}
	run := ids.New()
	if err := (runsjobs.Handler{Expirer: e}).Work(context.Background(), jobs.Job[runsjobs.QueuedExpiry]{Args: runsjobs.QueuedExpiry{RunID: run}}); err != nil ||
		len(e.got) != 1 || e.got[0] != run {
		t.Fatalf("Work: %v %v", err, e.got)
	}
	var s runsjobs.Scheduler
	if err := s.ScheduleQueuedExpiry(context.Background(), run, time.Now()); errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("unbound scheduler: %v", err)
	}
}
