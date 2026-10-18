// SPDX-License-Identifier: Apache-2.0

package chat_test

import (
	"slices"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
)

// TestRegister_Events: chat declares the durable events of spaces and the sidebar (M1-52) and of
// channels and the main thread (M1-53).
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
	want := []string{"channel.archived", "channel.created", "channel.updated", "sidebar.updated", "space.archived", "space.created", "space.updated", "thread.created"}
	if !slices.Equal(got, want) {
		t.Fatalf("events %v, want %v", got, want)
	}
}
