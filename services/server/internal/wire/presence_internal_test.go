// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
	"time"

	gatewayapp "github.com/OWNER/bothy/services/server/internal/gateway/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/sync"
)

// nopAuth is an Authenticator that accepts nothing; the hub of this test never authenticates.
type nopAuth struct{ actor.Authenticator }

// nopAuthz is an Authorizer the hub of this test never asks.
type nopAuthz struct{ authorize.Authorizer }

// nopCatchUp is the catch-up port of a hub that never catches up in this test.
type nopCatchUp struct{ sync.CatchUp }

// TestWirePresence_ByRole: role all wires the gateway's PresenceView, the live routes and the
// gateway's publisher; role worker wires presence.None (nil), the fail-closed routes of M3-19 and
// no publisher and logs the Info line once with bothy.server.role; role api wires nothing (M3-23
// design note 1, D-M3-23-1).
func TestWirePresence_ByRole(t *testing.T) {
	t.Parallel()
	hub, err := gatewayapp.NewHub(gatewayapp.HubConfig{QueueMax: 1, PageSize: 1, CursorEvery: 1, RetryBase: time.Millisecond, RetryCap: time.Millisecond,
		PayloadMax: 1024}, gatewayapp.HubDeps{Authenticator: nopAuth{}, Authorizer: nopAuthz{}, CatchUp: nopCatchUp{}})
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	newServer := func(role Role) *server {
		return &server{role: role, gatewayState: gatewayState{gateway: &Gateway{Hub: hub}}, log: newSwitchLogger(slog.New(slog.NewJSONHandler(&buf, nil)))}
	}
	var all PushDeps
	newServer(RoleAll).wirePresence(&all)
	if _, ok := all.Presence.(*gatewayapp.PresenceView); !ok || !all.LiveRoutes || all.Publisher != hub {
		t.Fatalf("role all: %+v", all)
	}
	var worker PushDeps
	newServer(RoleWorker).wirePresence(&worker)
	if worker.Presence != nil || worker.LiveRoutes || worker.Publisher != nil || worker.Routes != nil {
		t.Fatalf("role worker: %+v", worker)
	}
	logged := buf.String()
	if strings.Count(logged, msgPresenceUnavailable) != 1 || !strings.Contains(logged, `"bothy.server.role":"worker"`) {
		t.Fatalf("log %q", logged)
	}
	var api PushDeps
	newServer(RoleAPI).wirePresence(&api)
	if api.Presence != nil || api.LiveRoutes || api.Publisher != nil {
		t.Fatalf("role api: %+v", api)
	}
}
