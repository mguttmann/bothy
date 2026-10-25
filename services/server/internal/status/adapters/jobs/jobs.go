// SPDX-License-Identifier: Apache-2.0

// Package jobs is the adapter of the status module to platform/jobs: the job status.probe
// (docs/spec/02-architecture.md 12.3, docs/spec/16-operations.md 12) with its arguments, its
// handler and its periodic registration on the queue maintenance.
//
// Example:
//
//	err := jobs.Register(reg, probe, interval) // every status.check_interval and at the start
package jobs

import (
	"context"
	"errors"
	"time"

	"github.com/OWNER/bothy/services/server/internal/platform/jobs"
)

// KindProbe is the kind of the job.
const KindProbe = "status.probe"

// Probe are the arguments of the job; it has none, every round reads the clock (02 "Rules for
// jobs": arguments carry IDs only).
type Probe struct{}

// Kind returns KindProbe.
func (Probe) Kind() string { return KindProbe }

// Runner is the use case the job runs (app.Probe).
type Runner interface {
	// Run runs one round of the checks.
	Run(ctx context.Context) error
}

// Handler works the job: one round. A round is idempotent apart from its clock, so a retry after
// an error is safe.
type Handler struct {
	// Probe is the use case.
	Probe Runner
}

// Work runs one round; its error makes River retry the job.
func (h Handler) Work(ctx context.Context, _ jobs.Job[Probe]) error {
	if h.Probe == nil {
		return errors.New("status: status.probe runs without its use case")
	}
	return h.Probe.Run(ctx)
}

// Register adds the handler on queue maintenance and the periodic job every interval
// (status.check_interval) with RunOnStart, so the standard components exist as soon as a worker
// runs (16 12 "created during setup"); only the roles worker and all run it. r may be nil where no
// job runs (tests of the registration). The error is the first rejected registration.
func Register(reg *jobs.Registry, r Runner, interval time.Duration) error {
	if interval <= 0 {
		return errors.New("status: status.check_interval must be positive")
	}
	// A round bounds each check with status.check_interval; River sets no timeout of its own.
	if err := jobs.Register[Probe](reg, Handler{Probe: r}, jobs.HandlerOptions{Queue: jobs.QueueMaintenance, Timeout: -1}); err != nil {
		return err
	}
	return reg.AddPeriodic(jobs.PeriodicJob{Args: Probe{}, Interval: interval, RunOnStart: true})
}
