// SPDX-License-Identifier: Apache-2.0

package runs_test

import (
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/runs"
)

// TestRegister_Events: runs declares run.state_changed (M2-05).
func TestRegister_Events(t *testing.T) {
	t.Parallel()
	r := authorize.NewRegistry()
	runs.Register(r)
	ev := r.Events()
	if len(ev) != 1 || ev[0].Module != "runs" || ev[0].Type != authorize.EventRunStateChanged {
		t.Fatalf("events %+v", ev)
	}
}
