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

// KindInvocationHoldTimeout is the kind of the job that releases a held agent.invocation after
// file.invocation_hold_max (02 12.3 "Job catalogue", M3-07).
const KindInvocationHoldTimeout = "runs.invocation_hold_timeout"

// InvocationHoldTimeout are the arguments of runs.invocation_hold_timeout: only the run (02 12.1:
// IDs only). The release time is the run's invocation_held_until, which the use case reads.
type InvocationHoldTimeout struct {
	RunID ids.ID `json:"run_id"`
}

// Kind returns KindInvocationHoldTimeout.
func (InvocationHoldTimeout) Kind() string { return KindInvocationHoldTimeout }

// Releaser is the use case the job runs (app.Service.InvocationHoldTimeout).
type Releaser interface {
	// InvocationHoldTimeout releases the held agent.invocation of the run when it is still queued,
	// held and due, schedules the job again when it is not yet due, and does nothing otherwise; its
	// error makes River retry the job.
	InvocationHoldTimeout(ctx context.Context, runID ids.ID) error
}

// HoldHandler works the job once; the use case locks the run and clears the hold at the release,
// so a retry or a second run of the job has no second effect.
type HoldHandler struct {
	Releaser Releaser
}

// Work runs the use case for the run of the job.
func (h HoldHandler) Work(ctx context.Context, j jobs.Job[InvocationHoldTimeout]) error {
	return h.Releaser.InvocationHoldTimeout(ctx, j.Args.RunID)
}

// RegisterHold adds the handler of runs.invocation_hold_timeout on the queue default. Every role
// builds the same registry: the api role inserts the job at the acceptance, worker and all run
// it. r may be nil only where no job runs (tests of the registration). The error is the rejected
// registration.
func RegisterHold(reg *jobs.Registry, r Releaser) error {
	return jobs.Register[InvocationHoldTimeout](reg, HoldHandler{Releaser: r}, jobs.HandlerOptions{Queue: jobs.QueueDefault})
}

// HoldScheduler implements app.HoldScheduler over jobs.Inserter.InsertTx. The use cases are built
// before the job runtime of the role, so internal/wire binds the inserter once the runtime exists
// (Bind); a schedule before that is an internal error. It is safe for concurrent use.
type HoldScheduler struct {
	mu  stdsync.RWMutex
	ins jobs.Inserter
}

// Compile-time check: HoldScheduler is the outgoing port of the use cases.
var _ app.HoldScheduler = (*HoldScheduler)(nil)

// Bind sets the inserter of the job runtime.
func (s *HoldScheduler) Bind(ins jobs.Inserter) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ins = ins
}

// ScheduleInvocationHoldTimeout inserts runs.invocation_hold_timeout of the run in the ambient unit
// of work, to run at or after at, the release time of its held invocation.
func (s *HoldScheduler) ScheduleInvocationHoldTimeout(ctx context.Context, runID ids.ID, at time.Time) error {
	s.mu.RLock()
	ins := s.ins
	s.mu.RUnlock()
	if ins == nil {
		return errs.New(errs.CodeInternalError, "runs: the hold scheduler has no job runtime")
	}
	return ins.InsertTx(ctx, InvocationHoldTimeout{RunID: runID}, &jobs.InsertOptions{ScheduledAt: at})
}
