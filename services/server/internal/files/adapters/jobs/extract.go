// SPDX-License-Identifier: Apache-2.0

package jobs

import (
	"context"
	"errors"
	stdsync "sync"
	"time"

	"github.com/OWNER/bothy/services/server/internal/files/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/jobs"
)

// ExtractText are the arguments of files.extract_text (M3-15, docs/spec/14-search.md 5.1): only
// the file id (02 12.1 "Usage"); the text never enters job arguments.
type ExtractText struct {
	// FileID is the file whose text the job extracts.
	FileID ids.ID `json:"file_id"`
}

// Kind implements jobs.Args.
func (ExtractText) Kind() string { return "files.extract_text" }

// ExtractRefresh are the arguments of files.extract_text_refresh: the file id after which the
// walk continues; the periodic run starts at the zero id.
type ExtractRefresh struct {
	// After is the last file id of the previous batch; the zero id (the start of the walk) is
	// left out of the JSON, because an id reads back only as a UUIDv7.
	After ids.ID `json:"after,omitzero"`
}

// Kind implements jobs.Args.
func (ExtractRefresh) Kind() string { return "files.extract_text_refresh" }

// TextRunner is the use case of files.extract_text (app.ExtractText).
type TextRunner interface {
	Run(ctx context.Context, fileID ids.ID) error
}

// RefreshRunner is the use case of files.extract_text_refresh (app.ExtractRefresh).
type RefreshRunner interface {
	Run(ctx context.Context, after ids.ID) error
}

// errNoExtraction reports a handler without its use case (a role that never runs jobs).
var errNoExtraction = errors.New("files: the text extraction has no use case in this process")

// ExtractTextHandler runs files.extract_text. The span job files.extract_text comes from the
// handler wrapper of platform/jobs; the use case logs only the file id and a code.
type ExtractTextHandler struct {
	// Text is the use case; nil in a role that does not run jobs.
	Text TextRunner
}

// Work implements jobs.Handler: the error of the run (a transient read or database failure)
// makes River retry.
func (h ExtractTextHandler) Work(ctx context.Context, job jobs.Job[ExtractText]) error {
	if h.Text == nil {
		return errNoExtraction
	}
	return h.Text.Run(ctx, job.Args.FileID)
}

// ExtractRefreshHandler runs files.extract_text_refresh.
type ExtractRefreshHandler struct {
	// Refresh is the use case; nil in a role that does not run jobs.
	Refresh RefreshRunner
}

// Work implements jobs.Handler.
func (h ExtractRefreshHandler) Work(ctx context.Context, job jobs.Job[ExtractRefresh]) error {
	if h.Refresh == nil {
		return errNoExtraction
	}
	return h.Refresh.Run(ctx, job.Args.After)
}

// RegisterExtract adds files.extract_text on the queue files without a River job timeout (the use
// case bounds reading and parsing with search.extract.timeout, and River's default could cut the
// result write; docs/decisions.d/M3-15.md), and files.extract_text_refresh on the queue
// maintenance, periodic every interval (search.extract.refresh_interval) and once at the start,
// because a new binary may bring new extractor versions. text and refresh may be nil where no job
// runs. Errors: an interval that is not positive, or the first rejected registration.
func RegisterExtract(reg *jobs.Registry, text TextRunner, refresh RefreshRunner, interval time.Duration) error {
	if interval <= 0 {
		return errors.New("files: search.extract.refresh_interval must be positive")
	}
	if err := jobs.Register[ExtractText](reg, ExtractTextHandler{Text: text}, jobs.HandlerOptions{Queue: jobs.QueueFiles, Timeout: -1}); err != nil {
		return err
	}
	if err := jobs.Register[ExtractRefresh](reg, ExtractRefreshHandler{Refresh: refresh}, jobs.HandlerOptions{Queue: jobs.QueueMaintenance}); err != nil {
		return err
	}
	return reg.AddPeriodic(jobs.PeriodicJob{Args: ExtractRefresh{}, Interval: interval, RunOnStart: true})
}

// ExtractQueue implements app.ExtractQueue over jobs.Inserter.InsertTx; internal/wire binds the
// inserter once the job runtime exists (Bind). It is safe for concurrent use.
type ExtractQueue struct {
	mu  stdsync.RWMutex
	ins jobs.Inserter
}

// Compile-time check: ExtractQueue is the outgoing port of the extraction.
var _ app.ExtractQueue = (*ExtractQueue)(nil)

// Bind sets the inserter of the job runtime.
func (q *ExtractQueue) Bind(ins jobs.Inserter) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.ins = ins
}

// inserter returns the bound inserter or an internal error.
func (q *ExtractQueue) inserter() (jobs.Inserter, error) {
	q.mu.RLock()
	defer q.mu.RUnlock()
	if q.ins == nil {
		return nil, errs.New(errs.CodeInternalError, "files: the extraction queue has no job runtime")
	}
	return q.ins, nil
}

// EnqueueExtract implements app.ExtractQueue: files.extract_text in the ambient unit of work.
func (q *ExtractQueue) EnqueueExtract(ctx context.Context, fileID ids.ID) error {
	ins, err := q.inserter()
	if err != nil {
		return err
	}
	return ins.InsertTx(ctx, ExtractText{FileID: fileID}, nil)
}

// EnqueueExtractRefresh implements app.ExtractQueue: the next run of the walk in the ambient unit
// of work.
func (q *ExtractQueue) EnqueueExtractRefresh(ctx context.Context, after ids.ID) error {
	ins, err := q.inserter()
	if err != nil {
		return err
	}
	return ins.InsertTx(ctx, ExtractRefresh{After: after}, nil)
}
