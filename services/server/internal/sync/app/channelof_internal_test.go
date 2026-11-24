// SPDX-License-Identifier: Apache-2.0

package app

import (
	"testing"

	"github.com/OWNER/bothy/services/server/internal/chat"
)

// TestChannelOf_AllowAutonomousRuns (audit srv-chat-3): the snapshot channel keeps
// allow_autonomous_runs of chat.
func TestChannelOf_AllowAutonomousRuns(t *testing.T) {
	t.Parallel()
	if got := channelOf(chat.ChannelView{Channel: chat.Channel{AllowAutonomousRuns: true}}); !got.AllowAutonomousRuns {
		t.Fatalf("snapshot channel %+v", got)
	}
}
