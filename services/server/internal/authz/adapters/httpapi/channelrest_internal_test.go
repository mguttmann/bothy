// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"testing"

	"github.com/OWNER/bothy/services/server/internal/authz"
)

// TestChannelREST_AllowAutonomousRuns (audit srv-chat-3): the channel of view-as carries
// allow_autonomous_runs, false included.
func TestChannelREST_AllowAutonomousRuns(t *testing.T) {
	t.Parallel()
	for _, allow := range []bool{true, false} {
		got := channelREST(authz.ChannelDetail{Kind: authz.ChannelText, AllowAutonomousRuns: allow})
		if got.AllowAutonomousRuns == nil || *got.AllowAutonomousRuns != allow {
			t.Fatalf("allow %v: %+v", allow, got.AllowAutonomousRuns)
		}
	}
}
