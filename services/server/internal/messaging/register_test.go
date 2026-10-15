// SPDX-License-Identifier: Apache-2.0

package messaging_test

import (
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/messaging"
)

// TestRegister_Events: messaging declares message.created (M1-56), message.updated and
// message.deleted (M1-58).
func TestRegister_Events(t *testing.T) {
	t.Parallel()
	r := authorize.NewRegistry()
	messaging.Register(r)
	ev := r.Events()
	want := []authorize.EventType{authorize.EventMessageCreated, authorize.EventMessageDeleted, authorize.EventMessageUpdated}
	if len(ev) != len(want) {
		t.Fatalf("events %+v", ev)
	}
	for i, e := range ev {
		if e.Module != "messaging" || e.Type != want[i] {
			t.Fatalf("events %+v", ev)
		}
	}
}
