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

// ThumbnailArgs are the arguments of files.thumbnail (M3-06): only the file id (02 12.1 "Usage").
type ThumbnailArgs struct {
	// FileID is the file whose original, thumbnails or container values the job processes.
	FileID ids.ID `json:"file_id"`
	// Bulk is 1 for the job of an import file, inserted with the lowest priority (M8-11).
	Bulk int64 `json:"bulk,omitempty"`
}

// Kind implements jobs.Args.
func (ThumbnailArgs) Kind() string { return "files.thumbnail" }

// ImageRunner is the use case the handler runs (app.ImageProcessor).
type ImageRunner interface {
	ProcessImage(ctx context.Context, fileID ids.ID) error
}

// ThumbnailHandler runs files.thumbnail. The span job files.thumbnail comes from the handler
// wrapper of platform/jobs; the use case logs only the file id and an error class. An error makes
// River retry with its default attempts and backoff.
type ThumbnailHandler struct {
	// Images is the use case; nil in a role that does not run jobs.
	Images ImageRunner
}

// errNoImages reports a handler without its use case (a role that never runs jobs).
var errNoImages = errors.New("files: files.thumbnail has no image processor in this process")

// Work implements jobs.Handler.
func (h ThumbnailHandler) Work(ctx context.Context, job jobs.Job[ThumbnailArgs]) error {
	if h.Images == nil {
		return errNoImages
	}
	return h.Images.ProcessImage(ctx, job.Args.FileID)
}

// RegisterThumbnail adds files.thumbnail on the queue files to reg (every role builds the same
// registry) with the River job timeout file.image.processing_timeout (12 7.2, 02 12.4 "Rules for
// jobs" item 5) and River's default attempts and backoff.
func RegisterThumbnail(reg *jobs.Registry, images ImageRunner, timeout time.Duration) error {
	return jobs.Register[ThumbnailArgs](reg, ThumbnailHandler{Images: images}, jobs.HandlerOptions{Queue: jobs.QueueFiles, Timeout: timeout})
}

// EnqueueThumbnail implements app.ScanQueue: files.thumbnail for fileID in the ambient unit of
// work (the final unit of the scan, brief M3-06 design note 1), with the lowest priority in a
// context marked with app.WithBulkJobs.
func (q *Queue) EnqueueThumbnail(ctx context.Context, fileID ids.ID) error {
	q.mu.RLock()
	ins := q.ins
	q.mu.RUnlock()
	if ins == nil {
		return errs.New(errs.CodeInternalError, "files: the scan queue has no job runtime")
	}
	bulk := app.BulkJobs(ctx)
	return ins.InsertTx(ctx, ThumbnailArgs{FileID: fileID, Bulk: bulkMark(bulk)}, &jobs.InsertOptions{Priority: priorityOf(bulk)})
}
