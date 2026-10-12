// SPDX-License-Identifier: Apache-2.0

// Package jobs is the adapter of the sync module to platform/jobs: the job
// sync.partition_maintenance (docs/spec/02-architecture.md "Job catalogue") with its arguments,
// handler and periodic registration.
package jobs

import (
	"context"
	"errors"
	"time"

	"github.com/OWNER/bothy/services/server/internal/platform/jobs"
	"github.com/OWNER/bothy/services/server/internal/sync/app"
)

// KindPartitionMaintenance is the kind of the job.
const KindPartitionMaintenance = "sync.partition_maintenance"

// PartitionMaintenance are the arguments of the job; it has none, every run reads the head.
type PartitionMaintenance struct{}

// Kind returns KindPartitionMaintenance.
func (PartitionMaintenance) Kind() string { return KindPartitionMaintenance }

// Maintainer is the use case the job runs (app.PartitionMaintenance).
type Maintainer interface {
	// Run creates partitions ahead and drops expired ones.
	Run(ctx context.Context) (app.MaintenanceResult, error)
}

// Handler works the job: one run of the partition maintenance. The run is idempotent, so a retry
// after an error (a lock timeout of DETACH, for example) is safe.
type Handler struct {
	// Maintenance is the use case.
	Maintenance Maintainer
}

// Work runs the use case once; its error makes River retry the job.
func (h Handler) Work(ctx context.Context, _ jobs.Job[PartitionMaintenance]) error {
	_, err := h.Maintenance.Run(ctx)
	return err
}

// Register adds the handler on queue maintenance and the periodic job every interval
// (sync.partition_maintenance_interval) with RunOnStart, so a worker runs it at its start too
// (02 "Job catalogue"). The error is the first rejected registration or an interval that is not
// positive.
func Register(reg *jobs.Registry, m Maintainer, interval time.Duration) error {
	if interval <= 0 {
		return errors.New("sync: sync.partition_maintenance_interval must be positive")
	}
	if err := jobs.Register[PartitionMaintenance](reg, Handler{Maintenance: m}, jobs.HandlerOptions{Queue: jobs.QueueMaintenance}); err != nil {
		return err
	}
	return reg.AddPeriodic(jobs.PeriodicJob{Args: PartitionMaintenance{}, Interval: interval, RunOnStart: true})
}
