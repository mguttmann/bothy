// SPDX-License-Identifier: Apache-2.0

package jobs

import (
	"context"
	"time"
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
