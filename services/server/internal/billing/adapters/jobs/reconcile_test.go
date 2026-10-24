// SPDX-License-Identifier: Apache-2.0

package jobs_test

import (
	"context"
	"reflect"
	"slices"
	"testing"

	billingjobs "github.com/OWNER/bothy/services/server/internal/billing/adapters/jobs"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/jobs"
)

// applier records the runs it was asked for.
type applier struct{ got [][]ids.ID }

func (a *applier) ApplyReconciliation(_ context.Context, runIDs []ids.ID) error {
	a.got = append(a.got, runIDs)
	return nil
}

// TestReconcileJob (M2-28 design note 6): the kind billing.reconcile on the queue default with
// arguments of IDs only; the handler applies its runs; an unbound scheduler refuses.
func TestReconcileJob(t *testing.T) {
	t.Parallel()
	reg := jobs.NewRegistry()
	a := &applier{}
	if err := billingjobs.Register(reg, a); err != nil {
		t.Fatal(err)
	}
	if q, ok := reg.Queue(billingjobs.KindReconcile); !ok || q != jobs.QueueDefault {
		t.Fatalf("queue %q %v", q, ok)
	}
	if err := jobs.ValidateArgsType(reflect.TypeFor[billingjobs.Reconcile]()); err != nil {
		t.Fatal(err)
	}
	runs := []ids.ID{ids.New(), ids.New()}
	job := jobs.Job[billingjobs.Reconcile]{Args: billingjobs.Reconcile{RunIDs: runs}}
	if err := (billingjobs.Handler{Applier: a}).Work(context.Background(), job); err != nil || len(a.got) != 1 || !slices.Equal(a.got[0], runs) {
		t.Fatalf("Work: %v %v", err, a.got)
	}
	var s billingjobs.Scheduler
	if err := s.Schedule(context.Background(), runs); errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("unbound scheduler: %v", err)
	}
}
