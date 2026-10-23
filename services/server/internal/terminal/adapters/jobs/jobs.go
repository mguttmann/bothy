// SPDX-License-Identifier: Apache-2.0

// Package jobs registers the periodic job of the terminal module with the job runtime of
// internal/platform/jobs (M2-46): terminal.sweep on the queue maintenance with the period
// terminal.sweep_interval, run once at the start of the worker as well (02 section 12.3, 24
// section 6.3). The arguments are an empty struct (02 section 12.1: only ids, and the sweep needs
// none); the handler calls app.Service.Sweep and returns its error, so River retries a failed run.
//
// Example (internal/wire):
//
//	if err := jobs.Register(reg, terminal.Service, cfg.SweepInterval); err != nil {
//		return nil, err
//	}
package jobs

import (
	"context"
	"errors"
	"time"

	"github.com/OWNER/bothy/services/server/internal/platform/jobs"
	"github.com/OWNER/bothy/services/server/internal/terminal/app"
)

// KindSweep is the job kind of the sweep (02 section 12.3).
const KindSweep = "terminal.sweep"

// Sweep are the arguments of terminal.sweep: none.
type Sweep struct{}

// Kind implements jobs.Args.
func (Sweep) Kind() string { return KindSweep }

// Sweeper is the use case of the job; app.Service implements it.
type Sweeper interface {
	// Sweep ends the sessions whose limits passed in the database (app.Service.Sweep).
	Sweep(ctx context.Context) (app.SweepResult, error)
}

// SweepHandler runs terminal.sweep.
type SweepHandler struct {
	// Sweeper is the use case.
	Sweeper Sweeper
}

// Work implements jobs.Handler; the error of the run is returned so River retries. A handler
// without its use case fails (a registration test builds one without it, no worker does).
func (h SweepHandler) Work(ctx context.Context, _ jobs.Job[Sweep]) error {
	if h.Sweeper == nil {
		return errors.New("terminal: terminal.sweep without its use case")
	}
	_, err := h.Sweeper.Sweep(ctx)
	return err
}

// Register registers terminal.sweep on the queue maintenance and adds it as a periodic job with
// interval (terminal.sweep_interval) that also runs right after the worker became leader. s may be
// nil where no job runs (tests of the registration); a worker always passes it. Errors: an
// interval that is not positive, or the first rejected registration.
func Register(reg *jobs.Registry, s Sweeper, interval time.Duration) error {
	if interval <= 0 {
		return errors.New("terminal: terminal.sweep_interval must be positive")
	}
	if err := jobs.Register[Sweep](reg, SweepHandler{Sweeper: s}, jobs.HandlerOptions{Queue: jobs.QueueMaintenance}); err != nil {
		return err
	}
	return reg.AddPeriodic(jobs.PeriodicJob{Args: Sweep{}, Interval: interval, RunOnStart: true})
}
