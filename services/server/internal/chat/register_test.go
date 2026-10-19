// SPDX-License-Identifier: Apache-2.0

package chat_test

import (
	"slices"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
)

// TestRegister_Events: chat declares the durable events of spaces and the sidebar (M1-52), of
// channels and the main thread (M1-53), of threads (M1-55), of moving threads (M1-62) and of the channel behaviour (M2-02).
func TestRegister_Events(t *testing.T) {
	t.Parallel()
	r := authorize.NewRegistry()
	chat.Register(r)
	var got []string
	for _, e := range r.Events() {
		if e.Module != "chat" {
			t.Fatalf("module %q", e.Module)
		}
		got = append(got, string(e.Type))
	}
	slices.Sort(got)
	want := []string{"agent.config_changed", "channel.archived", "channel.created", "channel.settings_changed", "channel.updated", "sidebar.updated", "space.archived", "space.created", "space.updated", "thread.closed", "thread.created",
		"thread.moved", "thread.moved_out", "thread.reopened", "thread.updated"}
	if !slices.Equal(got, want) {
		t.Fatalf("events %v, want %v", got, want)
	}
}
