// SPDX-License-Identifier: Apache-2.0

package gateway_test

import (
	"slices"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/gateway"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
)

// TestRegister_Ops: the gateway declares exactly the ops of gateway_ops.enabled (M1-64).
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
	want := []authorize.Op{authorize.OpHeartbeat, authorize.OpIdentify, authorize.OpReauth, authorize.OpResume}
	if !slices.Equal(got, want) {
		t.Errorf("ops %v, want %v", got, want)
	}
}
