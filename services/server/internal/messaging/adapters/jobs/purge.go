// SPDX-License-Identifier: Apache-2.0

// Package jobs registers the River jobs of messaging (FEAT-delete-spaces-chats): the periodic
// messaging.purge_deleted on the queue maintenance (02 section 12.3).
package jobs

import (
	"context"
	"errors"
	"time"

	"github.com/OWNER/bothy/services/server/internal/platform/jobs"
)

// KindPurgeDeleted is the job kind of the purge of deleted spaces, channels and chats.
const KindPurgeDeleted = "messaging.purge_deleted"

// PurgeDeleted are the arguments of messaging.purge_deleted: none, every run reads the due items.
type PurgeDeleted struct{}

// Kind returns KindPurgeDeleted.
func (PurgeDeleted) Kind() string { return KindPurgeDeleted }

// PurgeRunner is the use case of the job (messaging/app.Purge).
type PurgeRunner interface {
	Run(ctx context.Context) (int, error)
}

// PurgeHandler runs the use case.
type PurgeHandler struct {
	// Purge is the use case; nil in a role that does not run jobs.
	Purge PurgeRunner
}

var errNoPurge = errors.New("messaging: messaging.purge_deleted has no purge in this process")

// Work runs one purge round.
func (h PurgeHandler) Work(ctx context.Context, _ jobs.Job[PurgeDeleted]) error {
	if h.Purge == nil {
		return errNoPurge
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	_, err := h.Purge.Run(ctx)
	return err
}

// Register registers messaging.purge_deleted on the queue maintenance, periodic every interval
// (chat.delete_purge_interval) and once at the start of the worker. Errors: an interval that is
// not positive, a rejected registration.
func Register(reg *jobs.Registry, purge PurgeRunner, interval time.Duration) error {
	if interval <= 0 {
		return errors.New("messaging: chat.delete_purge_interval must be positive")
	}
	if err := jobs.Register[PurgeDeleted](reg, PurgeHandler{Purge: purge}, jobs.HandlerOptions{Queue: jobs.QueueMaintenance}); err != nil {
		return err
	}
	return reg.AddPeriodic(jobs.PeriodicJob{Args: PurgeDeleted{}, Interval: interval, RunOnStart: true})
}
