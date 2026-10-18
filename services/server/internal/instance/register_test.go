// SPDX-License-Identifier: Apache-2.0

package instance_test

import (
	"testing"

	"github.com/OWNER/bothy/services/server/internal/instance"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
)

// TestRegister_Events: instance declares the durable event its settings, branding and feature
// flags append: instance.updated (M1-67 found it emitted but not registered).
func TestRegister_Events(t *testing.T) {
	t.Parallel()
	r := authorize.NewRegistry()
	instance.Register(r)
	got := r.Events()
	if len(got) != 1 || got[0].Module != "instance" || got[0].Type != authorize.EventInstanceUpdated {
		t.Fatalf("events %+v", got)
	}
}
