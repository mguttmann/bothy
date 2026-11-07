// SPDX-License-Identifier: Apache-2.0

package jobs

import (
	"context"
	stdsync "sync"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/jobs"
	"github.com/OWNER/bothy/services/server/internal/runs/app"
)

// KindWakeDue is the kind of the job that fires a due at wake-up, fires a wake-up that waited for
// the end of a run, and expires a wake-up past its deadline (09 5.8.4, AR-04; 02 12.3): scheduled
// per wake-up at its not_before and at its deadline, and at the end of a run for the wake-ups of
// its thread that could fire, instead of a periodic job whose interval would delay every fire
// (ruling D-AR-04-2).
const KindWakeDue = "runs.wake_due"

// WakeDue are the arguments of the job: only the wake-up (02 12.1: IDs only).
type WakeDue struct {
	WakeID ids.ID `json:"wake_id"`
}

// Kind returns KindWakeDue.
func (WakeDue) Kind() string { return KindWakeDue }

// WakeDuer is the use case the job runs (app.Service.WakeDue).
type WakeDuer interface {
	// WakeDue fires or expires the wake-up when it is due; its error makes River retry the job.
	WakeDue(ctx context.Context, wakeID ids.ID) error
}

// WakeDueHandler works the job once; the use case locks the wake-up and is idempotent (an ended
// wake-up is nothing), so a retry is safe (02 12.4 rule 1).
type WakeDueHandler struct {
	Duer WakeDuer
}

// Work runs the use case for the wake-up of the job.
func (h WakeDueHandler) Work(ctx context.Context, j jobs.Job[WakeDue]) error {
	return h.Duer.WakeDue(ctx, j.Args.WakeID)
}

// RegisterWakeDue adds the handler on the queue default: a fire writes a run, an event and a
// note, so the job pauses in maintenance mode with every queue except the deliveries (ADR 0065
// point 4). Every role builds the same registry. d may be nil only where no job runs (tests of the
// registration). The error is the rejected registration.
func RegisterWakeDue(reg *jobs.Registry, d WakeDuer) error {
	return jobs.Register[WakeDue](reg, WakeDueHandler{Duer: d}, jobs.HandlerOptions{Queue: jobs.QueueDefault})
}

// WakeScheduler implements app.WakeScheduler over jobs.Inserter.InsertTx. The use cases are built
// before the job runtime of the role, so internal/wire binds the inserter once the runtime exists
// (Bind); a schedule before that is an internal error. It is safe for concurrent use.
type WakeScheduler struct {
	mu  stdsync.RWMutex
	ins jobs.Inserter
}

// Compile-time check: WakeScheduler is the outgoing port of the use cases.
var _ app.WakeScheduler = (*WakeScheduler)(nil)

// Bind sets the inserter of the job runtime.
func (s *WakeScheduler) Bind(ins jobs.Inserter) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ins = ins
}

// ScheduleWakeDue inserts runs.wake_due of the wake-up in the ambient unit of work, to run at or
// after at.
func (s *WakeScheduler) ScheduleWakeDue(ctx context.Context, wakeID ids.ID, at time.Time) error {
	s.mu.RLock()
	ins := s.ins
	s.mu.RUnlock()
	if ins == nil {
		return errs.New(errs.CodeInternalError, "runs: the wake-up scheduler has no job runtime")
	}
	return ins.InsertTx(ctx, WakeDue{WakeID: wakeID}, &jobs.InsertOptions{ScheduledAt: at})
}
