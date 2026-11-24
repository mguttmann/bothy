// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"testing"

	"github.com/OWNER/bothy/services/server/internal/sync"
)

// TestChannelViewREST_AllowAutonomousRuns (audit srv-chat-3): the snapshot's ChannelView carries
// allow_autonomous_runs, false included.
func TestChannelViewREST_AllowAutonomousRuns(t *testing.T) {
	t.Parallel()
	for _, allow := range []bool{true, false} {
		got := channelViewREST(sync.SnapshotChannel{Kind: "forum", AllowAutonomousRuns: allow})
		if got.Channel.AllowAutonomousRuns == nil || *got.Channel.AllowAutonomousRuns != allow {
			t.Fatalf("allow %v: %+v", allow, got.Channel.AllowAutonomousRuns)
		}
	}
}
