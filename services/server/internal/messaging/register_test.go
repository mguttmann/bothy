// SPDX-License-Identifier: Apache-2.0

package messaging_test

import (
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/messaging"
)

// TestRegister_Events: messaging declares message.created (M1-56).
func TestRegister_Events(t *testing.T) {
	t.Parallel()
	r := authorize.NewRegistry()
	messaging.Register(r)
	ev := r.Events()
	if len(ev) != 1 || ev[0].Module != "messaging" || ev[0].Type != authorize.EventMessageCreated {
		t.Fatalf("events %+v", ev)
	}
}
