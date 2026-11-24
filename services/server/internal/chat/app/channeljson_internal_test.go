// SPDX-License-Identifier: Apache-2.0

package app

import (
	"encoding/json"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/chat/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// TestChannelBody_AllowAutonomousRuns (audit srv-chat-3): the channel of the events and of the
// 412 current carries allow_autonomous_runs like getChannel, also when it is false.
func TestChannelBody_AllowAutonomousRuns(t *testing.T) {
	t.Parallel()
	for _, allow := range []bool{true, false} {
		ch := domain.NewChannel(ids.New(), ids.New(), domain.ChannelForum, "bugs", "", "a0", ids.New(), 20)
		ch.AllowAutonomousRuns = allow
		b, err := json.Marshal(channelBody(ChannelRow{Channel: ch}))
		if err != nil {
			t.Fatal(err)
		}
		var m map[string]any
		if err := json.Unmarshal(b, &m); err != nil {
			t.Fatal(err)
		}
		if got, ok := m["allow_autonomous_runs"]; !ok || got != allow {
			t.Fatalf("allow %v: payload %s", allow, b)
		}
	}
}
