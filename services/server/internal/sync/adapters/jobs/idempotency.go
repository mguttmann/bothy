// SPDX-License-Identifier: Apache-2.0

package jobs

import (
	"context"
	"errors"
	"time"

	"github.com/OWNER/bothy/services/server/internal/platform/jobs"
	"github.com/OWNER/bothy/services/server/internal/sync/app"
)

// KindIdempotencyCleanup is the kind of the job that deletes expired idempotency keys.
const KindIdempotencyCleanup = "sync.idempotency_cleanup"

// IdempotencyCleanup are the arguments of the job; it has none, every run reads the clock (02
// "Rules for jobs": arguments carry IDs only).
type IdempotencyCleanup struct{}

// Kind returns KindIdempotencyCleanup.
func (IdempotencyCleanup) Kind() string { return KindIdempotencyCleanup }

// Cleaner is the use case the job runs (app.IdempotencyCleanup).
type Cleaner interface {
	// Run deletes the expired keys in batches.
	Run(ctx context.Context) (app.CleanupResult, error)
}

// CleanupHandler works the job: one run of the cleanup. A repeated run is harmless, so a retry
// after an error is safe.
type CleanupHandler struct {
	// Cleanup is the use case.
	Cleanup Cleaner
}

// Work runs the use case once; its error (ctx.Err() on cancellation) makes River retry the job.
func (h CleanupHandler) Work(ctx context.Context, _ jobs.Job[IdempotencyCleanup]) error {
	_, err := h.Cleanup.Run(ctx)
	return err
}

// RegisterIdempotencyCleanup adds the handler on queue maintenance and the periodic job every
// interval (sync.idempotency_cleanup_interval, 02 "Job catalogue"); only the roles worker and all
// run it. The error is the first rejected registration or an interval that is not positive.
func RegisterIdempotencyCleanup(reg *jobs.Registry, c Cleaner, interval time.Duration) error {
	if interval <= 0 {
		return errors.New("sync: sync.idempotency_cleanup_interval must be positive")
	}
	if err := jobs.Register[IdempotencyCleanup](reg, CleanupHandler{Cleanup: c}, jobs.HandlerOptions{Queue: jobs.QueueMaintenance}); err != nil {
		return err
	}
	return reg.AddPeriodic(jobs.PeriodicJob{Args: IdempotencyCleanup{}, Interval: interval})
}
