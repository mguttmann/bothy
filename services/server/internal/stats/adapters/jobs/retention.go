// SPDX-License-Identifier: Apache-2.0

// Package jobs is the adapter of the stats module to platform/jobs: the job stats.retention
// (docs/spec/02-architecture.md 12.3, docs/spec/25-visualization.md 3.5) with its arguments, its
// handler and its periodic registration.
//
// Example:
//
//	err := jobs.Register(reg, retention) // hourly and at the start of a worker
package jobs

import (
	"context"
	"errors"
	"time"

	"github.com/OWNER/bothy/services/server/internal/platform/jobs"
	"github.com/OWNER/bothy/services/server/internal/stats/app"
)

// KindRetention is the kind of the job.
const KindRetention = "stats.retention"

// RetentionInterval is the period of stats.retention: once an hour, and once at the start of a
// worker (RunOnStart). A day has 24 runs, so tomorrow's partition stands ready many runs before
// midnight and an expired day is gone within an hour of its end; a flush that still meets a day
// without a partition creates it itself (docs/DECISIONS.md D-M6-13-6). It is a fixed schedule,
// not an operating limit, so it has no configuration key.
const RetentionInterval = time.Hour

// Retention are the arguments of the job; it has none, every run reads the clock (02 "Rules for
// jobs": arguments carry IDs only).
type Retention struct{}

// Kind returns KindRetention.
func (Retention) Kind() string { return KindRetention }

// Runner is the use case the job runs (app.Retention).
type Runner interface {
	// Run creates today's and tomorrow's partition and drops the expired ones.
	Run(ctx context.Context) (app.RetentionResult, error)
}

// Handler works the job: one run of the retention. The run is idempotent, so a retry after an
// error (a lock timeout of DETACH, for example) is safe.
type Handler struct {
	// Retention is the use case.
	Retention Runner
}

// Work runs the use case once; its error makes River retry the job.
func (h Handler) Work(ctx context.Context, _ jobs.Job[Retention]) error {
	if h.Retention == nil {
		return errors.New("stats: stats.retention runs without its use case")
	}
	_, err := h.Retention.Run(ctx)
	return err
}

// Register adds the handler on queue maintenance and the periodic job every RetentionInterval
// with RunOnStart; only the roles worker and all run it (02 12.3). r may be nil where no job runs
// (the role api and tests of the registration). The error is the first rejected registration.
func Register(reg *jobs.Registry, r Runner) error {
	if err := jobs.Register[Retention](reg, Handler{Retention: r}, jobs.HandlerOptions{Queue: jobs.QueueMaintenance}); err != nil {
		return err
	}
	return reg.AddPeriodic(jobs.PeriodicJob{Args: Retention{}, Interval: RetentionInterval, RunOnStart: true})
}
