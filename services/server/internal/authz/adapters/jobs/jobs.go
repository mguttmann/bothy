// SPDX-License-Identifier: Apache-2.0

// Package jobs registers the periodic jobs of authz with the job runtime of internal/platform/jobs
// (M1-30): authz.expiry_sweep and authz.expiry_reminder on the queue maintenance, both with the
// period authz.expiry_sweep_interval (07 sections 8.3 and 9.4, 02 section 12). The arguments are
// empty structs (02 section 12.1: only ids, and these jobs need none); the handlers call the use
// case of app.ExpiryService and return its error, so River retries a failed run.
//
// Example (internal/wire):
//
//	if err := jobs.Register(reg, expiry, cfg.Interval); err != nil {
//		return nil, err
//	}
package jobs

import (
	"context"
	"errors"
	"time"

	"github.com/OWNER/bothy/services/server/internal/authz/app"
	"github.com/OWNER/bothy/services/server/internal/platform/jobs"
)

// The job kinds of authz (02 section 12.3).
const (
	// KindExpirySweep reports expired grants and memberships.
	KindExpirySweep = "authz.expiry_sweep"
	// KindExpiryReminder writes access.expiring before an expiry.
	KindExpiryReminder = "authz.expiry_reminder"
)

// ExpirySweep are the arguments of authz.expiry_sweep: none.
type ExpirySweep struct{}

// Kind implements jobs.Args.
func (ExpirySweep) Kind() string { return KindExpirySweep }

// ExpiryReminder are the arguments of authz.expiry_reminder: none.
type ExpiryReminder struct{}

// Kind implements jobs.Args.
func (ExpiryReminder) Kind() string { return KindExpiryReminder }

// Expiry is the use case both jobs run; app.ExpiryService implements it.
type Expiry interface {
	// Sweep reports due expiries (app.ExpiryService.Sweep).
	Sweep(ctx context.Context) (app.ExpiryResult, error)
	// Remind writes due reminders (app.ExpiryService.Remind).
	Remind(ctx context.Context) (app.ExpiryResult, error)
}

// SweepHandler runs authz.expiry_sweep.
type SweepHandler struct {
	// Expiry is the use case.
	Expiry Expiry
}

// Work implements jobs.Handler; the error of the run is returned so River retries.
func (h SweepHandler) Work(ctx context.Context, _ jobs.Job[ExpirySweep]) error {
	_, err := h.Expiry.Sweep(ctx)
	return err
}

// ReminderHandler runs authz.expiry_reminder.
type ReminderHandler struct {
	// Expiry is the use case.
	Expiry Expiry
}

// Work implements jobs.Handler; the error of the run is returned so River retries.
func (h ReminderHandler) Work(ctx context.Context, _ jobs.Job[ExpiryReminder]) error {
	_, err := h.Expiry.Remind(ctx)
	return err
}

// Register registers both jobs on the queue maintenance and adds them as periodic jobs with
// interval (authz.expiry_sweep_interval). e may be nil where no job runs (tests of the
// registration); a worker always passes it. Errors: an interval that is not positive, or the
// first rejected registration.
func Register(reg *jobs.Registry, e Expiry, interval time.Duration) error {
	if interval <= 0 {
		return errors.New("authz: authz.expiry_sweep_interval must be positive")
	}
	opts := jobs.HandlerOptions{Queue: jobs.QueueMaintenance}
	if err := jobs.Register[ExpirySweep](reg, SweepHandler{Expiry: e}, opts); err != nil {
		return err
	}
	if err := jobs.Register[ExpiryReminder](reg, ReminderHandler{Expiry: e}, opts); err != nil {
		return err
	}
	if err := reg.AddPeriodic(jobs.PeriodicJob{Args: ExpirySweep{}, Interval: interval}); err != nil {
		return err
	}
	return reg.AddPeriodic(jobs.PeriodicJob{Args: ExpiryReminder{}, Interval: interval})
}
