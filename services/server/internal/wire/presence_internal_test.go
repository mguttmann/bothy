// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"log/slog"
	"testing"
	"time"

	gatewayapp "github.com/OWNER/bothy/services/server/internal/gateway/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/sync"
)

// nopAuth is an Authenticator that accepts nothing; the hub of this test never authenticates.
type nopAuth struct{ actor.Authenticator }

// nopAuthz is an Authorizer the hub of this test never asks.
type nopAuthz struct{ authorize.Authorizer }

// nopCatchUp is the catch-up port of a hub that never catches up in this test.
type nopCatchUp struct{ sync.CatchUp }

// TestWirePresence_ByRole: role all with cluster.mode single wires the gateway's PresenceView over
// the hub, the live routes and the gateway's publisher (M3-23 design note 1, D-M3-23-1); role api
// wires nothing; role worker and role all with multi read gateway_presence (M7-17 Design note 3),
// so without the pool their wiring fails instead of falling back to the fail-closed routes. The
// table-backed wiring with a database runs in test/gateway/multinode.
func TestWirePresence_ByRole(t *testing.T) {
	t.Parallel()
	hub, err := gatewayapp.NewHub(gatewayapp.HubConfig{QueueMax: 1, PageSize: 1, CursorEvery: 1, RetryBase: time.Millisecond, RetryCap: time.Millisecond,
		PayloadMax: 1024}, gatewayapp.HubDeps{Authenticator: nopAuth{}, Authorizer: nopAuthz{}, CatchUp: nopCatchUp{}})
	if err != nil {
		t.Fatal(err)
	}
	single, multi := loadValues(t), loadValues(t, "BOTHY_CLUSTER__MODE=multi")
	newServer := func(role Role, v *config.Values) *server {
		return &server{role: role, values: v, gatewayState: gatewayState{gateway: &Gateway{Hub: hub}},
			log: newSwitchLogger(slog.New(slog.DiscardHandler))}
	}
	var all PushDeps
	if err := newServer(RoleAll, single).wirePresence(&all); err != nil {
		t.Fatal(err)
	}
	if _, ok := all.Presence.(*gatewayapp.PresenceView); !ok || !all.LiveRoutes || all.Publisher != hub {
		t.Fatalf("role all: %+v", all)
	}
	var api PushDeps
	if err := newServer(RoleAPI, multi).wirePresence(&api); err != nil || api.Presence != nil || api.LiveRoutes || api.Publisher != nil {
		t.Fatalf("role api: %+v %v", api, err)
	}
	for _, c := range []struct {
		role Role
		v    *config.Values
	}{{RoleWorker, single}, {RoleWorker, multi}, {RoleAll, multi}} {
		var d PushDeps
		if err := newServer(c.role, c.v).wirePresence(&d); err == nil {
			t.Errorf("role %s %s without the pool: wired %+v", c.role, clusterMode(c.v), d)
		}
	}
}
