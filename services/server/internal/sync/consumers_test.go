// SPDX-License-Identifier: Apache-2.0

package sync_test

import (
	"slices"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/sync"
)

// TestConsumerNames: the names are exactly the set of the check of event_consumer_cursors in
// docs/spec/03-datenmodell.md 4.10, and Valid accepts nothing else.
func TestConsumerNames(t *testing.T) {
	t.Parallel()
	want := []sync.ConsumerName{sync.ConsumerPush, sync.ConsumerWebhooks}
	if got := sync.ConsumerNames(); !slices.Equal(got, want) {
		t.Fatalf("ConsumerNames = %v, want %v", got, want)
	}
	if string(sync.ConsumerPush) != "push" || string(sync.ConsumerWebhooks) != "webhooks" {
		t.Fatalf("names = %q, %q", sync.ConsumerPush, sync.ConsumerWebhooks)
	}
	for _, n := range want {
		if !n.Valid() {
			t.Errorf("%q is not valid", n)
		}
	}
	for _, n := range []sync.ConsumerName{"", "Push", "webhook", "push ", "agents", "webhooks\x00"} {
		if n.Valid() {
			t.Errorf("%q is valid", n)
		}
	}
	got := sync.ConsumerNames()
	got[0] = "changed"
	if sync.ConsumerNames()[0] != sync.ConsumerPush {
		t.Fatal("ConsumerNames returns a shared slice")
	}
}
