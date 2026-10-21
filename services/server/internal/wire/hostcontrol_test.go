// SPDX-License-Identifier: Apache-2.0

package wire_test

import (
	"slices"
	"testing"

	"github.com/OWNER/bothy/services/server/api/routes"
	authzapp "github.com/OWNER/bothy/services/server/internal/authz/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

// TestWire_RouteRolesAndRegistrations (M2-43, AZ-P1, AZ-P2): the route of the control connection
// is GET /api/v1/hosts/connect with host.connect and rl.host, authorized on the connection; the
// terminal module registers the five host ops of host_control with host.connect and the events
// host.updated, terminal_session.changed and host.presence_changed; the catalogue check of the
// start passes with the route. The mount in api and all only is held by
// TestServeRolesStartReadyAndStop.
func TestWire_RouteRolesAndRegistrations(t *testing.T) {
	t.Parallel()
	op := wire.HostConnectOperation()
	if !op.AuthorizedInBand || op.Pattern != "GET /api/v1/hosts/connect" || op.Action != "host.connect" || op.RateLimitClass != "rl.host" {
		t.Fatalf("operation %+v", op)
	}
	reg := wire.BuildRegistry(nil)
	var ops []authorize.ChannelOp
	for _, r := range reg.ChannelOps() {
		if r.Module == "terminal" {
			ops = append(ops, r.Op)
		}
	}
	want := []authorize.ChannelOp{authorize.ChannelOpHostControlHeartbeat, authorize.ChannelOpHostControlIdentify,
		authorize.ChannelOpHostControlReport, authorize.ChannelOpHostControlSessionEnded, authorize.ChannelOpHostControlSessionStarted}
	if !slices.Equal(ops, want) {
		t.Errorf("channel ops %v, want %v", ops, want)
	}
	events := map[authorize.EventType]string{}
	for _, e := range reg.Events() {
		events[e.Type] = e.Module
	}
	for _, e := range []authorize.EventType{authorize.EventHostUpdated, authorize.EventTerminalSessionChanged, authorize.EventHostPresenceChanged} {
		if events[e] != "terminal" {
			t.Errorf("%s registered by %q", e, events[e])
		}
	}
	bindings := append(wire.RouteBindings(routes.Enabled()), wire.GatewayRouteBinding(), wire.HostConnectRouteBinding())
	if err := authzapp.NewCatalog(nil).VerifyRegistrations(bindings, reg); err != nil {
		t.Fatalf("catalogue check: %v", err)
	}
}
