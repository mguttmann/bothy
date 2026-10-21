// SPDX-License-Identifier: Apache-2.0

package jobs

import (
	"context"
	"errors"
	stdsync "sync"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/jobs"
	"github.com/OWNER/bothy/services/server/internal/runs/app"
)

// The kinds of the lifecycle jobs of M2-09 (02 12.3 "Job catalogue").
const (
	// KindCancelTimeout ends a run whose cancel request the adapter did not confirm within
	// agent.cancel_grace.
	KindCancelTimeout = "runs.cancel_timeout"
	// KindStallCheck fails the runs whose adapter restarted or vanished (interrupted, stale).
	KindStallCheck = "runs.stall_check"
)

// CancelTimeout are the arguments of runs.cancel_timeout: only the run (02 12.1: IDs only).
type CancelTimeout struct {
	RunID ids.ID `json:"run_id"`
}

// Kind returns KindCancelTimeout.
func (CancelTimeout) Kind() string { return KindCancelTimeout }

// StallCheck are the arguments of runs.stall_check: none, the job reads the active runs itself.
type StallCheck struct{}

// Kind returns KindStallCheck.
func (StallCheck) Kind() string { return KindStallCheck }

// Lifecycle is the part of the use cases the two jobs run (app.Service).
type Lifecycle interface {
	// CancelTimeout ends the run cancelled with cancel_timeout when it is still active; an ended
	// run is left as it is.
	CancelTimeout(ctx context.Context, runID ids.ID) error
	// StallCheck fails the interrupted and stale runs.
	StallCheck(ctx context.Context) error
}

// CancelTimeoutHandler works runs.cancel_timeout once; the use case locks the run and is
// idempotent, so a retry is safe.
type CancelTimeoutHandler struct {
	Lifecycle Lifecycle
}

// Work runs the use case for the run of the job.
func (h CancelTimeoutHandler) Work(ctx context.Context, j jobs.Job[CancelTimeout]) error {
	return h.Lifecycle.CancelTimeout(ctx, j.Args.RunID)
}

// StallCheckHandler works runs.stall_check once.
type StallCheckHandler struct {
	Lifecycle Lifecycle
}

// Work runs the stall check.
func (h StallCheckHandler) Work(ctx context.Context, _ jobs.Job[StallCheck]) error {
	return h.Lifecycle.StallCheck(ctx)
}

// RegisterLifecycle adds both handlers on the queue maintenance and runs.stall_check as a
// periodic job every interval (agent.presence_timeout, the window of interrupted; ruling
// D-M2-09-19) that also runs once at the start of a worker. Every role builds the same registry;
// only worker and all work jobs. l may be nil only where no job runs. Errors: a non-positive
// interval or a rejected registration.
func RegisterLifecycle(reg *jobs.Registry, l Lifecycle, interval time.Duration) error {
	if interval <= 0 {
		return errors.New("runs: agent.presence_timeout must be positive")
	}
	opts := jobs.HandlerOptions{Queue: jobs.QueueMaintenance}
	if err := jobs.Register[CancelTimeout](reg, CancelTimeoutHandler{Lifecycle: l}, opts); err != nil {
		return err
	}
	if err := jobs.Register[StallCheck](reg, StallCheckHandler{Lifecycle: l}, opts); err != nil {
		return err
	}
	return reg.AddPeriodic(jobs.PeriodicJob{Args: StallCheck{}, Interval: interval, RunOnStart: true})
}

// CancelScheduler implements app.CancelTimeoutScheduler over jobs.Inserter.InsertTx. Like
// Scheduler, internal/wire binds the inserter once the job runtime exists; a schedule before that
// is an internal error. It is safe for concurrent use.
type CancelScheduler struct {
	mu  stdsync.RWMutex
	ins jobs.Inserter
}

// Compile-time check: CancelScheduler is the outgoing port of the use cases.
var _ app.CancelTimeoutScheduler = (*CancelScheduler)(nil)

// Bind sets the inserter of the job runtime.
func (s *CancelScheduler) Bind(ins jobs.Inserter) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ins = ins
}

// Schedule inserts runs.cancel_timeout of the run in the ambient unit of work, to run at or after
// at (now + agent.cancel_grace).
func (s *CancelScheduler) Schedule(ctx context.Context, runID ids.ID, at time.Time) error {
	s.mu.RLock()
	ins := s.ins
	s.mu.RUnlock()
	if ins == nil {
		return errs.New(errs.CodeInternalError, "runs: the cancel timeout scheduler has no job runtime")
	}
	return ins.InsertTx(ctx, CancelTimeout{RunID: runID}, &jobs.InsertOptions{ScheduledAt: at})
}
