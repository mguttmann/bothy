// SPDX-License-Identifier: Apache-2.0

// Package jobs is the adapter of the runs module to platform/jobs (M2-08): the scheduled job
// runs.queued_expiry on the queue default, which fails a run that nobody started within
// run.queued_max_age (docs/spec/09-agenten.md 5.5, D-AG-16; 02 "Job-Katalog"), and the
// ExpiryScheduler of the use cases over jobs.Inserter.InsertTx.
//
// Example:
//
//	sched := &jobs.Scheduler{}
//	svc, _ := app.New(cfg, app.Deps{Expiry: sched, ...})
//	_ = jobs.Register(reg, svc) // every role
//	sched.Bind(runtime.Inserter()) // after the runtime of the role exists
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

// KindQueuedExpiry is the kind of the job.
const KindQueuedExpiry = "runs.queued_expiry"

// QueuedExpiry are the arguments of the job: only the run (02 12.1: IDs only).
type QueuedExpiry struct {
	RunID ids.ID `json:"run_id"`
}

// Kind returns KindQueuedExpiry.
func (QueuedExpiry) Kind() string { return KindQueuedExpiry }

// Expirer is the use case the job runs (app.Service.ExpireQueued).
type Expirer interface {
	// ExpireQueued fails the run when it is still queued and due, schedules the job again when it
	// is queued and not yet due, and does nothing otherwise; its error makes River retry the job.
	ExpireQueued(ctx context.Context, runID ids.ID) error
}

// Handler works the job once; the use case locks the run and is idempotent, so a retry is safe.
type Handler struct {
	Expirer Expirer
}

// Work runs the use case for the run of the job.
func (h Handler) Work(ctx context.Context, j jobs.Job[QueuedExpiry]) error {
	return h.Expirer.ExpireQueued(ctx, j.Args.RunID)
}

// Register adds the handler on the queue default. Every role builds the same registry: the api
// role inserts the job at the acceptance, worker and all run it. e may be nil only where no job
// runs (tests of the registration). The error is the rejected registration.
func Register(reg *jobs.Registry, e Expirer) error {
	return jobs.Register[QueuedExpiry](reg, Handler{Expirer: e}, jobs.HandlerOptions{Queue: jobs.QueueDefault})
}

// Scheduler implements app.ExpiryScheduler over jobs.Inserter.InsertTx. The use cases are built
// before the job runtime of the role, so internal/wire binds the inserter once the runtime exists
// (Bind); a schedule before that is an internal error. It is safe for concurrent use.
type Scheduler struct {
	mu  stdsync.RWMutex
	ins jobs.Inserter
}

// Compile-time check: Scheduler is the outgoing port of the use cases.
var _ app.ExpiryScheduler = (*Scheduler)(nil)

// Bind sets the inserter of the job runtime.
func (s *Scheduler) Bind(ins jobs.Inserter) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ins = ins
}

// ScheduleQueuedExpiry inserts runs.queued_expiry of the run in the ambient unit of work, to run at
// or after at.
func (s *Scheduler) ScheduleQueuedExpiry(ctx context.Context, runID ids.ID, at time.Time) error {
	s.mu.RLock()
	ins := s.ins
	s.mu.RUnlock()
	if ins == nil {
		return errs.New(errs.CodeInternalError, "runs: the expiry scheduler has no job runtime")
	}
	return ins.InsertTx(ctx, QueuedExpiry{RunID: runID}, &jobs.InsertOptions{ScheduledAt: at})
}
