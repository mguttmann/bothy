// SPDX-License-Identifier: Apache-2.0

// Package jobs is the job adapter of the files module (docs/spec/02-architecture.md 12.3 "Job catalogue"):
// the arguments and the handler of files.scan on the queue files, and the ScanQueue of the use
// cases over jobs.Inserter.InsertTx.
//
// Example:
//
//	q := &jobs.Queue{}
//	err := jobs.Register(reg, pass) // every role; worker and all run it
//	q.Bind(rt.Inserter())
package jobs

import (
	"context"
	"errors"
	stdsync "sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/OWNER/bothy/services/server/internal/files/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/jobs"
	"github.com/OWNER/bothy/services/server/internal/platform/otel/attr"
)

// instrumentationName is the scope of the span file.scan.
const instrumentationName = "github.com/OWNER/bothy/services/server/internal/files/adapters/jobs"

// SpanName is the CLIENT span of one scan pass (signals.md 5.2).
const SpanName = "file.scan"

// Scan are the arguments of files.scan: only the file id and the retry round (02 "Einsatz").
type Scan struct {
	// FileID is the file to scan.
	FileID ids.ID `json:"file_id"`
	// Round counts the retries of the pass, 0 for the job of the completion.
	Round int64 `json:"round"`
}

// Kind implements jobs.Args.
func (Scan) Kind() string { return "files.scan" }

// Runner is the use case the handler runs (app.ScanPass).
type Runner interface {
	Run(ctx context.Context, fileID ids.ID, round int64) (app.PassResult, error)
}

// Handler runs files.scan inside the span file.scan (kind CLIENT) with bothy.file.id and
// bothy.outcome. The pass schedules its own retries and returns nil for them, so River's attempt
// limit never ends the retries; an error is a failed database unit (River retries the job).
type Handler struct {
	// Pass is the use case; nil in a role that does not run jobs.
	Pass Runner
	// Tracer is the tracer provider; nil means the global one.
	Tracer trace.TracerProvider
}

// errNoPass reports a handler without its use case (a role that never runs jobs).
var errNoPass = errors.New("files: files.scan has no scan pass in this process")

// Work implements jobs.Handler.
func (h Handler) Work(ctx context.Context, job jobs.Job[Scan]) error {
	if h.Pass == nil {
		return errNoPass
	}
	tp := h.Tracer
	if tp == nil {
		tp = otel.GetTracerProvider()
	}
	ctx, span := tp.Tracer(instrumentationName).Start(ctx, SpanName, trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(attribute.String(attr.FileID, job.Args.FileID.String())))
	defer span.End()
	res, err := h.Pass.Run(ctx, job.Args.FileID, job.Args.Round)
	if err != nil {
		span.SetStatus(codes.Error, "")
		return err
	}
	span.SetAttributes(attribute.String(attr.Outcome, res.Outcome))
	return nil
}

// Register adds files.scan on the queue files to reg (every role builds the same registry), with
// no River job timeout: the pass bounds itself with file.scan.timeout, and River's default could
// be shorter and cut the unit of work that follows (docs/DECISIONS.md D-M3-04-2).
func Register(reg *jobs.Registry, pass Runner) error {
	return jobs.Register[Scan](reg, Handler{Pass: pass}, jobs.HandlerOptions{Queue: jobs.QueueFiles, Timeout: -1})
}

// Queue implements app.ScanQueue over jobs.Inserter.InsertTx. The use cases are built before the
// job runtime of the role, so internal/wire binds the inserter once the runtime exists (Bind); an
// insert before that is an internal error. It is safe for concurrent use.
type Queue struct {
	mu  stdsync.RWMutex
	ins jobs.Inserter
}

// Compile-time check: Queue is the outgoing port of the use cases.
var _ app.ScanQueue = (*Queue)(nil)

// Bind sets the inserter of the job runtime.
func (q *Queue) Bind(ins jobs.Inserter) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.ins = ins
}

// EnqueueScan implements app.ScanQueue: files.scan with round in the ambient unit of work, to run
// at or after at (zero: now).
func (q *Queue) EnqueueScan(ctx context.Context, fileID ids.ID, round int64, at time.Time) error {
	q.mu.RLock()
	ins := q.ins
	q.mu.RUnlock()
	if ins == nil {
		return errs.New(errs.CodeInternalError, "files: the scan queue has no job runtime")
	}
	return ins.InsertTx(ctx, Scan{FileID: fileID, Round: round}, &jobs.InsertOptions{ScheduledAt: at})
}
