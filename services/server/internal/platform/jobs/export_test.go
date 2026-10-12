// SPDX-License-Identifier: Apache-2.0

package jobs

import "context"

// StopClientForTest stops the River client of rt behind its back, as a fatal River error would.
func StopClientForTest(ctx context.Context, rt *Runtime) error {
	return rt.client.StopAndCancel(ctx)
}

// MigrateLockIDForTest is the advisory lock that serialises Migrate.
const MigrateLockIDForTest = migrateLockID
