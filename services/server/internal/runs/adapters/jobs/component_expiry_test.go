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

// componentExpirer records the cards it was asked for.
type componentExpirer struct{ got []ids.ID }

func (e *componentExpirer) ExpireComponents(_ context.Context, id ids.ID) error {
	e.got = append(e.got, id)
	return nil
}

// TestComponentExpiryJob: the kind runs.component_expiry on the queue maintenance with only the
// card message as argument (02 12.1); the handler runs the use case of its card; an unbound
// scheduler refuses (M2-15 design note 15 a).
func TestComponentExpiryJob(t *testing.T) {
	t.Parallel()
	reg := jobs.NewRegistry()
	e := &componentExpirer{}
	if err := runsjobs.RegisterComponentExpiry(reg, e); err != nil {
		t.Fatal(err)
	}
	if q, ok := reg.Queue(runsjobs.KindComponentExpiry); !ok || q != jobs.QueueMaintenance {
		t.Fatalf("queue %q %v", q, ok)
	}
	if err := jobs.ValidateArgsType(reflect.TypeFor[runsjobs.ComponentExpiry]()); err != nil {
		t.Fatal(err)
	}
	card := ids.New()
	if err := (runsjobs.ComponentExpiryHandler{Expirer: e}).Work(context.Background(),
		jobs.Job[runsjobs.ComponentExpiry]{Args: runsjobs.ComponentExpiry{MessageID: card}}); err != nil || len(e.got) != 1 || e.got[0] != card {
		t.Fatalf("Work: %v %v", err, e.got)
	}
	var s runsjobs.ComponentScheduler
	if err := s.ScheduleComponentExpiry(context.Background(), card, time.Now()); errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("unbound scheduler: %v", err)
	}
}
