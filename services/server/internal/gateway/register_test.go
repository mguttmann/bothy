// SPDX-License-Identifier: Apache-2.0

package gateway_test

import (
	"slices"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/gateway"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
)

// TestRegister_Ops: the gateway declares exactly the ops of gateway_ops.enabled (M1-64, M1-66)
// and the ephemeral type it emits itself, typing.started.
func TestRegister_Ops(t *testing.T) {
	t.Parallel()
	r := authorize.NewRegistry()
	gateway.Register(r)
	var got []authorize.Op
	for _, o := range r.Ops() {
		if o.Module != "gateway" {
			t.Errorf("module %q", o.Module)
		}
		got = append(got, o.Op)
	}
	want := []authorize.Op{authorize.OpFocus, authorize.OpHeartbeat, authorize.OpIdentify, authorize.OpReauth, authorize.OpResume,
		authorize.OpTypingStart, authorize.OpTypingStop}
	if !slices.Equal(got, want) {
		t.Errorf("ops %v, want %v", got, want)
	}
	ev := r.Events()
	if len(ev) != 1 || ev[0].Module != "gateway" || ev[0].Type != authorize.EventTypingStarted {
		t.Errorf("events %+v, want typing.started of gateway", ev)
	}
}
