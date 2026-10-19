// SPDX-License-Identifier: Apache-2.0

package jobs

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/riverqueue/river"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/OWNER/bothy/services/server/internal/platform/otel/attr"
)

// The values of bothy.outcome on the span "job <bothy.job.kind>" and on bothy.job.duration
// (docs/observability/signals.md "Spans", 4.4). The catalogue also lists snoozed; it stays unused
// until the River-free interface offers a snooze (README.md "Spans").
const (
	outcomeCompleted = "completed"
	outcomeFailed    = "failed"
	outcomeCancelled = "cancelled"
)

// Log messages of the handler wrapper; constant text (signals.md "Logs").
const msgPanic = "job panicked"

// errTypePanic is the error.type of a recovered panic.
const errTypePanic = "panic"

// workerDeps are what every wrapped handler needs from the runtime.
type workerDeps struct {
	tracer  trace.Tracer
	log     *slog.Logger
	metrics *jobMetrics
}

// worker adapts a Handler to River's Worker. River never reaches the handler directly: the
// wrapper adds the span, turns a panic into an error, refuses a swallowed cancellation and
// applies the handler's retry rule.
type worker[A Args] struct {
	river.WorkerDefaults[A]
	h    Handler[A]
	o    HandlerOptions
	kind string
	d    workerDeps
}

// newWorker wraps h for River.
func newWorker[A Args](h Handler[A], o HandlerOptions, kind string, d workerDeps) *worker[A] {
	return &worker[A]{h: h, o: o, kind: kind, d: d}
}

// Work runs one attempt inside the CONSUMER span "job <kind>" with bothy.job.kind and
// bothy.outcome, and records its duration with the same two attributes in bothy.job.duration. A
// nil result while ctx has ended becomes ctx.Err(): a handler that swallowed
// the cancellation would otherwise mark an unfinished job as done, and a rerun is safe because
// jobs are idempotent (docs/DECISIONS.md M1-14).
func (w *worker[A]) Work(ctx context.Context, j *river.Job[A]) (err error) {
	start := time.Now()
	ctx, span := w.d.tracer.Start(ctx, "job "+w.kind,
		trace.WithSpanKind(trace.SpanKindConsumer),
		trace.WithAttributes(attribute.String(attr.JobKind, w.kind)))
	defer func() {
		outcome := outcomeOf(ctx, err)
		if w.d.metrics != nil {
			w.d.metrics.observed(ctx, w.kind, outcome, time.Since(start))
		}
		span.SetAttributes(attribute.String(attr.Outcome, outcome))
		if outcome == outcomeFailed {
			span.SetStatus(codes.Error, "")
		}
		span.End()
	}()
	err = w.run(ctx, j)
	if err == nil && ctx.Err() != nil {
		err = ctx.Err()
	}
	return err
}

// run calls the handler and turns a panic into an error. The log line names the kind and the
// error class, never the arguments or the panic value, which may carry content.
func (w *worker[A]) run(ctx context.Context, j *river.Job[A]) (err error) {
	defer func() {
		if p := recover(); p != nil {
			w.d.log.ErrorContext(ctx, msgPanic, slog.String(attr.JobKind, w.kind), slog.String(attr.ErrorType, errTypePanic))
			err = fmt.Errorf("jobs: job %s panicked", w.kind)
		}
	}()
	return w.h.Work(ctx, Job[A]{ID: j.ID, Attempt: j.Attempt, Args: j.Args})
}

// NextRetry returns the time of the next attempt from HandlerOptions.NextRetry, or the zero time
// for River's default backoff.
func (w *worker[A]) NextRetry(j *river.Job[A]) time.Time {
	if w.o.NextRetry == nil {
		return time.Time{}
	}
	return time.Now().Add(w.o.NextRetry(j.Attempt))
}

// Timeout returns River's job timeout of the kind from HandlerOptions.Timeout: 0 keeps River's
// client default, a negative value becomes -1, River's "never time out".
func (w *worker[A]) Timeout(*river.Job[A]) time.Duration {
	if w.o.Timeout < 0 {
		return -1
	}
	return w.o.Timeout
}

// outcomeOf maps the result of an attempt to bothy.outcome.
func outcomeOf(ctx context.Context, err error) string {
	switch {
	case err == nil:
		return outcomeCompleted
	case ctx.Err() != nil && (errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)):
		return outcomeCancelled
	default:
		return outcomeFailed
	}
}
