// SPDX-License-Identifier: Apache-2.0

// Package jobs registers the periodic job of identity with the job runtime of
// internal/platform/jobs (M1-35): identity.auth_cleanup on the queue maintenance every
// auth.cleanup_interval (docs/spec/02-architecture.md section 12.3). The arguments are an empty
// struct (02 section 12.1: only ids, and this job needs none); the handler runs the use case
// app.AuthCleanup and returns its error, so River retries a failed run. Requests never depend on
// the job, because every use case checks expiry itself.
//
// Example (internal/wire):
//
//	if err := jobs.Register(reg, cleanup, cfg.CleanupInterval); err != nil {
//		return nil, err
//	}
package jobs

import (
	"context"
	"errors"
	"time"

	"github.com/OWNER/bothy/services/server/internal/platform/jobs"
)

// KindAuthCleanup is the kind of the job that deletes expired challenges (02 section 12.3).
const KindAuthCleanup = "identity.auth_cleanup"

// AuthCleanup are the arguments of identity.auth_cleanup: none, every run reads the clock.
type AuthCleanup struct{}

// Kind implements jobs.Args.
func (AuthCleanup) Kind() string { return KindAuthCleanup }

// Cleaner is the use case the job runs; app.AuthCleanup implements it.
type Cleaner interface {
	// Run deletes the expired rows and returns how many.
	Run(ctx context.Context) (int64, error)
}

// Handler runs identity.auth_cleanup. A repeated run is harmless, so a retry is safe.
type Handler struct {
	// Cleanup is the use case.
	Cleanup Cleaner
}

// Work implements jobs.Handler; the error of the run is returned so River retries.
func (h Handler) Work(ctx context.Context, _ jobs.Job[AuthCleanup]) error {
	_, err := h.Cleanup.Run(ctx)
	return err
}

// Register registers the job on the queue maintenance and adds it as a periodic job with interval
// (auth.cleanup_interval); only the roles worker and all run it. c may be nil where no job runs
// (tests of the registration); a worker always passes it. Errors: an interval that is not
// positive, or the first rejected registration.
func Register(reg *jobs.Registry, c Cleaner, interval time.Duration) error {
	if interval <= 0 {
		return errors.New("identity: auth.cleanup_interval must be positive")
	}
	if err := jobs.Register[AuthCleanup](reg, Handler{Cleanup: c}, jobs.HandlerOptions{Queue: jobs.QueueMaintenance}); err != nil {
		return err
	}
	return reg.AddPeriodic(jobs.PeriodicJob{Args: AuthCleanup{}, Interval: interval})
}
