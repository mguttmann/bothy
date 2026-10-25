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
