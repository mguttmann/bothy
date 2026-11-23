// SPDX-License-Identifier: Apache-2.0

package jobs

import (
	"context"
	"log/slog"
	"time"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
	"go.opentelemetry.io/otel/trace/noop"

	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// StopClientForTest stops the River client of rt behind its back, as a fatal River error would.
func StopClientForTest(ctx context.Context, rt *Runtime) error {
	return rt.client.StopAndCancel(ctx)
}

// MigrateLockIDForTest is the advisory lock that serialises Migrate.
const MigrateLockIDForTest = migrateLockID

// timeoutArgs is an argument type for WorkerTimeoutForTest.
type timeoutArgs struct{}

func (timeoutArgs) Kind() string { return "jobs.timeout" }

// WorkerTimeoutForTest returns River's job timeout of a worker registered with o.
func WorkerTimeoutForTest(o HandlerOptions) time.Duration {
	return newWorker[timeoutArgs](nil, o, "jobs.timeout", workerDeps{}).Timeout(nil)
}

// timeoutHandler records whether it ran and whether its context was exempt from maintenance mode.
type timeoutHandler struct{ ran, exempt *bool }

func (h timeoutHandler) Work(ctx context.Context, _ Job[timeoutArgs]) error {
	*h.ran, *h.exempt = true, txn.WritesInMaintenance(ctx)
	return nil
}

// WorkInMaintenanceForTest runs one attempt of the kind jobs.timeout in maintenance mode m (nil:
// outside the mode) and reports whether the handler ran, whether its context carried the
// maintenance exemption, and the result.
func WorkInMaintenanceForTest(m *Maintenance) (ran, exempt bool, err error) {
	w := newWorker[timeoutArgs](timeoutHandler{ran: &ran, exempt: &exempt}, HandlerOptions{}, "jobs.timeout",
		workerDeps{tracer: noop.NewTracerProvider().Tracer("test"), log: slog.Default(), maintenance: m})
	err = w.Work(context.Background(), &river.Job[timeoutArgs]{JobRow: &rivertype.JobRow{}})
	return ran, exempt, err
}

// ValidateForTest is Config.validate.
func ValidateForTest(c Config) error { return c.validate() }

// OutcomeOfForTest is outcomeOf.
func OutcomeOfForTest(ctx context.Context, err error) string { return outcomeOf(ctx, err) }

// failingHandler returns err, or panics when err is nil.
type failingHandler struct{ err error }

func (h failingHandler) Work(context.Context, Job[timeoutArgs]) error {
	if h.err == nil {
		panic("boom")
	}
	return h.err
}

// WorkFailingForTest runs one attempt of the kind jobs.timeout whose handler returns err (panics
// for nil), as attempt of maxAttempts, logging to log.
func WorkFailingForTest(log *slog.Logger, err error, attempt, maxAttempts int) error {
	w := newWorker[timeoutArgs](failingHandler{err: err}, HandlerOptions{}, "jobs.timeout",
		workerDeps{tracer: noop.NewTracerProvider().Tracer("test"), log: log})
	return w.Work(context.Background(), &river.Job[timeoutArgs]{JobRow: &rivertype.JobRow{Attempt: attempt, MaxAttempts: maxAttempts}})
}

// RiverLoggerForTest is the logger River gets from a runtime with log.
func RiverLoggerForTest(log *slog.Logger) *slog.Logger { return newRiverLogger(log) }
