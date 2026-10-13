// SPDX-License-Identifier: Apache-2.0

// Package jobs is the adapter of the instance module to platform/jobs: the periodic job
// instance.key_rotation (docs/spec/02-architecture.md "Job catalogue", 19 section 10), which applies
// the due transitions of the server signing keys.
package jobs

import (
	"context"
	"errors"
	"time"

	"github.com/OWNER/bothy/services/server/internal/platform/jobs"
)

// KindKeyRotation is the kind of the job.
const KindKeyRotation = "instance.key_rotation"

// KeyRotation are the arguments of the job; it has none, every run reads the clock and the keys
// (02 "Rules for jobs").
type KeyRotation struct{}

// Kind returns KindKeyRotation.
func (KeyRotation) Kind() string { return KindKeyRotation }

// Rotation is the use case the job runs (app.Keys.RunRotation).
type Rotation interface {
	// RunRotation applies the due transitions of every purpose and returns how many it applied.
	RunRotation(ctx context.Context) (int, error)
}

// Handler works the job: one run of the rotation. A run applies only what is due at its time, so
// a retry after an error is safe.
type Handler struct {
	// Rotation is the use case.
	Rotation Rotation
}

// Work runs the use case once; its error makes River retry the job.
func (h Handler) Work(ctx context.Context, _ jobs.Job[KeyRotation]) error {
	_, err := h.Rotation.RunRotation(ctx)
	return err
}

// Register adds the handler on the queue maintenance and the periodic job every interval
// (signing_key.rotation_check_interval). Every role builds the same registry; only worker and
// all run River, so the job runs there. The error is the first rejected registration or an
// interval that is not positive.
func Register(reg *jobs.Registry, r Rotation, interval time.Duration) error {
	if interval <= 0 {
		return errors.New("instance: signing_key.rotation_check_interval must be positive")
	}
	if err := jobs.Register[KeyRotation](reg, Handler{Rotation: r}, jobs.HandlerOptions{Queue: jobs.QueueMaintenance}); err != nil {
		return err
	}
	return reg.AddPeriodic(jobs.PeriodicJob{Args: KeyRotation{}, Interval: interval})
}
