// SPDX-License-Identifier: Apache-2.0

package jobs

import (
	"context"
	"errors"
	"time"

	"github.com/OWNER/bothy/services/server/internal/files/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/jobs"
)

// KindPurgeObjects is the kind of files.purge_objects (02 12.3, M3-08).
const KindPurgeObjects = "files.purge_objects"

// KindCleanupPending is the kind of files.cleanup_pending (02 12.3, M3-08).
const KindCleanupPending = "files.cleanup_pending"

// PurgeObjects are the arguments of files.purge_objects: only the file id (02 12.1 "Usage").
type PurgeObjects struct {
	// FileID is the deleted, failed or blocked file whose objects go.
	FileID ids.ID `json:"file_id"`
}

// Kind implements jobs.Args.
func (PurgeObjects) Kind() string { return KindPurgeObjects }

// CleanupPending are the arguments of the periodic files.cleanup_pending; it has none.
type CleanupPending struct{}

// Kind implements jobs.Args.
func (CleanupPending) Kind() string { return KindCleanupPending }

// PurgeRunner is the use case of files.purge_objects (app.Purge).
type PurgeRunner interface {
	Run(ctx context.Context, fileID ids.ID) error
}

// CleanupRunner is the use case of files.cleanup_pending (app.Cleanup).
type CleanupRunner interface {
	Run(ctx context.Context) error
}

// PurgeHandler runs files.purge_objects. The span job files.purge_objects comes from the handler
// wrapper of platform/jobs. An error makes River retry with its default attempts and backoff
// (02 12.4 rules 4 and 5); a cancelled context returns ctx.Err() (rule 2).
type PurgeHandler struct {
	// Purge is the use case; nil in a role that does not run jobs.
	Purge PurgeRunner
}

// errNoPurge reports a handler without its use case (a role that never runs jobs).
var errNoPurge = errors.New("files: files.purge_objects has no purge in this process")

// Work implements jobs.Handler.
func (h PurgeHandler) Work(ctx context.Context, job jobs.Job[PurgeObjects]) error {
	if h.Purge == nil {
		return errNoPurge
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return h.Purge.Run(ctx, job.Args.FileID)
}

// CleanupHandler runs files.cleanup_pending; the use case returns the first error of its steps
// after the last one, so River records it.
type CleanupHandler struct {
	// Cleanup is the use case; nil in a role that does not run jobs.
	Cleanup CleanupRunner
}

// errNoCleanup reports a handler without its use case (a role that never runs jobs).
var errNoCleanup = errors.New("files: files.cleanup_pending has no cleanup in this process")

// Work implements jobs.Handler.
func (h CleanupHandler) Work(ctx context.Context, _ jobs.Job[CleanupPending]) error {
	if h.Cleanup == nil {
		return errNoCleanup
	}
	return h.Cleanup.Run(ctx)
}

// RegisterCleanup adds files.purge_objects and files.cleanup_pending on the queue maintenance to
// reg (every role builds the same registry) and the periodic files.cleanup_pending every interval
// (file.gc_interval) and once at the start of the leader (02 12.3).
//
// Example:
//
//	err := jobs.RegisterCleanup(reg, purge, cleanup, values.Duration("file.gc_interval"))
func RegisterCleanup(reg *jobs.Registry, purge PurgeRunner, cleanup CleanupRunner, interval time.Duration) error {
	if interval <= 0 {
		return errors.New("files: file.gc_interval must be positive")
	}
	if err := jobs.Register[PurgeObjects](reg, PurgeHandler{Purge: purge}, jobs.HandlerOptions{Queue: jobs.QueueMaintenance}); err != nil {
		return err
	}
	if err := jobs.Register[CleanupPending](reg, CleanupHandler{Cleanup: cleanup}, jobs.HandlerOptions{Queue: jobs.QueueMaintenance}); err != nil {
		return err
	}
	return reg.AddPeriodic(jobs.PeriodicJob{Args: CleanupPending{}, Interval: interval, RunOnStart: true})
}

// Purges implements app.PurgeQueue over the inserter bound to q (Bind): files.purge_objects in the
// ambient unit of work, to run at once (02 8.5).
type Purges struct {
	// Q is the queue whose inserter the purge uses.
	Q *Queue
}

// Compile-time check: Purges is the outgoing port of the deletions.
var _ app.PurgeQueue = Purges{}

// Enqueue implements app.PurgeQueue.
func (p Purges) Enqueue(ctx context.Context, fileID ids.ID) error {
	if p.Q == nil {
		return errs.New(errs.CodeInternalError, "files: the purge queue has no job runtime")
	}
	p.Q.mu.RLock()
	ins := p.Q.ins
	p.Q.mu.RUnlock()
	if ins == nil {
		return errs.New(errs.CodeInternalError, "files: the purge queue has no job runtime")
	}
	return ins.InsertTx(ctx, PurgeObjects{FileID: fileID}, nil)
}
