// SPDX-License-Identifier: Apache-2.0

package jobs_test

import (
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/platform/jobs"
)

// TestHandlerOptionsTimeout: zero keeps River's client default (0), a positive value is the job
// timeout of the kind, a negative value means no River timeout (-1).
func TestHandlerOptionsTimeout(t *testing.T) {
	t.Parallel()
	for in, want := range map[time.Duration]time.Duration{0: 0, 3 * time.Minute: 3 * time.Minute, -1: -1, -time.Hour: -1} {
		if got := jobs.WorkerTimeoutForTest(jobs.HandlerOptions{Timeout: in}); got != want {
			t.Errorf("Timeout %v gives %v, want %v", in, got, want)
		}
	}
}
