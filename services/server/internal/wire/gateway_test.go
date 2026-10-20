// SPDX-License-Identifier: Apache-2.0

package wire_test

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/api/routes"
	authzapp "github.com/OWNER/bothy/services/server/internal/authz/app"
	"github.com/OWNER/bothy/services/server/internal/instance"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/platform/ratelimit"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

// TestGatewayRoute_AuthorizedInBandOnlyGateway (M1-64): the gateway route is registered with
// gateway.connect and rl.gateway_connect and passes the catalogue check of the start (AZ-P1);
// it is the only route marked AuthorizedInBand.
func TestGatewayRoute_AuthorizedInBandOnlyGateway(t *testing.T) {
	t.Parallel()
	op := wire.GatewayOperation()
	if !op.AuthorizedInBand || op.Pattern != "GET /api/v1/gateway" || op.Action != "gateway.connect" || op.RateLimitClass != "rl.gateway_connect" {
		t.Fatalf("gateway operation %+v", op)
	}
	for _, r := range routes.Enabled() {
		if wire.HTTPOperation(r).AuthorizedInBand {
			t.Errorf("%s is marked AuthorizedInBand", r.ID)
		}
	}
	bindings := append(wire.RouteBindings(routes.Enabled()), wire.GatewayRouteBinding())
	if err := authzapp.NewCatalog(nil).VerifyRegistrations(bindings, wire.BuildRegistry(nil)); err != nil {
		t.Fatalf("catalogue check: %v", err)
	}
}

// fixedInstance is an instance row with a fixed epoch.
type fixedInstance struct{}

func (fixedInstance) Load(context.Context) (instance.Record, error) { return instance.Record{}, nil }

// staticNonces is a nonce source for the wiring test.
type staticNonces struct{}

func (staticNonces) CurrentNonce() string { return "n" }

// TestGatewayOpsEnableList: the gateway registers exactly the ops of gateway_ops.enabled with the
// action of each (routes.GatewayOps).
func TestGatewayOpsEnableList(t *testing.T) {
	t.Parallel()
	values := loadEnv(t, nil)
	cfg, err := wire.GatewayConfigFrom(values)
	if err != nil {
		t.Fatal(err)
	}
	rl, err := wire.RateLimitConfig(values)
	if err != nil {
		t.Fatal(err)
	}
	l, err := ratelimit.New(rl, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	gw, err := wire.NewGateway(cfg, wire.GatewayDeps{Authenticator: actor.AnonymousOnly{}, Authorizer: allowAll{}, Nonces: staticNonces{},
		Limiter: l, Reads: &wire.SyncReads{}, Instance: fixedInstance{}})
	if err != nil {
		t.Fatal(err)
	}
	var want []string
	for _, op := range routes.GatewayOps() {
		want = append(want, op.Op)
		if op.Action != string(authorize.ActionGatewayConnect) {
			t.Errorf("%s has action %s", op.Op, op.Action)
		}
	}
	if got := gw.App.Dispatcher().Ops(); !slices.Equal(got, want) || !slices.Equal(want, []string{"heartbeat", "identify", "reauth", "resume"}) {
		t.Errorf("registered %v, enabled %v", got, want)
	}
	if cfg.HeartbeatInterval != 30*time.Second || cfg.ResumeWindow != 5*time.Minute {
		t.Errorf("defaults %+v", cfg)
	}
}
