// SPDX-License-Identifier: Apache-2.0

package app

import "context"

// ScheduleTextExtract exposes the trigger of the text extraction to the tests of app_test.
func ScheduleTextExtract(t *ExtractTrigger, ctx context.Context, f FileRecord) error {
	return t.schedule(ctx, f)
}
