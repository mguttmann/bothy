// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"context"
	"errors"
	"log/slog"

	gatewayapp "github.com/OWNER/bothy/services/server/internal/gateway/app"
	instancepg "github.com/OWNER/bothy/services/server/internal/instance/adapters/postgres"
	instanceapp "github.com/OWNER/bothy/services/server/internal/instance/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/ratelimit"
	"github.com/OWNER/bothy/services/server/internal/runs"
)

// gatewayState is the part of the server state the gateway module fills.
type gatewayState struct {
	// gateway is the WebSocket gateway of api and all (M1-64) with the hub the log tail feeds
	// (M1-65).
	gateway *Gateway
	// cluster is the gateway across several api nodes (M7-17): the publisher decorator, the
	// senders of the device and consumer signals and the gateway's route of the LISTEN ephemeral
	// connection; nil when the plan neither sends nor receives (role all with single).
	cluster *gatewayapp.Cluster
	// node is the process instance (gateway_presence.node_id, the node of every signal).
	node ids.ID
}

// moduleGateway is the name of the module in architecture/modules.yaml and of its start step, where it has one.
const moduleGateway = "gateway"

// gatewayWiring wires the WebSocket gateway: its route binding and its mount after the REST
// routes (api and all).
func gatewayWiring() moduleWiring {
	return moduleWiring{
		module: moduleGateway,
		parts: []part{
			// M7-17 (and M2-47 for the terminal): the one LISTEN ephemeral connection of the
			// process after the HTTP listeners (soft dependency).
			{order: 2650, step: pg.EphemeralComponent, when: func(s *server) bool { return s.gatewayPlan().Listener }, start: (*server).startEphemeral},
		},
		routeBindings: func() []authorize.RouteBinding { return []authorize.RouteBinding{GatewayRouteBinding()} },
		mounts: []moduleMount{{order: 100, mount: func(s *server, _ context.Context, mux Mux, chain *httpserver.Chain, limiter *ratelimit.Limiter,
			log *slog.Logger) error {
			return s.mountGateway(mux, chain, limiter, log)
		}}},
	}
}

// mountGateway builds the WebSocket gateway (M1-64) over identity's Authenticator and DPoP
// nonces, the Authorizer, the limiter of the chain (classes rl.gateway_connect, rl.read and
// rl.typing), catch-up and snapshot of sync, the instance row and the threads of chat (M1-66),
// mounts GET /api/v1/gateway on mux and starts the sweeper of the gateway sessions with its stop
// step.
func (s *server) mountGateway(mux Mux, chain *httpserver.Chain, limiter *ratelimit.Limiter, log *slog.Logger) error {
	cfg, err := GatewayConfigFrom(s.values)
	if err != nil {
		return err
	}
	s.mu.Lock()
	id, az, reads, pool, a, ch, ag, rn, st := s.identity, s.authorizer, s.syncReads, s.pool, s.audit, s.chat, s.agents, s.runs, s.stats
	s.mu.Unlock()
	var invocations runs.Delivery
	var answers gatewayapp.AutocompleteAnswerer
	var liveBoxRuns LiveBoxRuns
	if rn != nil {
		invocations, answers, liveBoxRuns = rn.Service, rn.Service, rn.Service
		if s.maint.ReadOnly {
			liveBoxRuns = maintenancePatcher{LiveBoxRuns: rn.Service, running: rn.Service}
		}
	}
	if id == nil || id.Authenticator == nil || az == nil || reads == nil || a == nil || ch == nil || ag == nil {
		return errors.New("wire: the gateway needs identity, the Authorizer, catch-up and snapshot, the unit of work, chat and agents")
	}
	cluster, err := s.newGatewayCluster(cfg, ch.Service)
	if err != nil {
		return err
	}
	gw, err := NewGateway(cfg, GatewayDeps{
		Authenticator: id.Authenticator, Authorizer: az, Nonces: id.DPoP, Limiter: limiter, Reads: reads,
		Instance: instanceapp.NewState(instancepg.NewRepository(pool), a.UoW()), Threads: ch.Service, Consumers: ag.Consumers,
		Presence: ag.Presence, Invocations: invocations, Autocomplete: answers, Runs: liveBoxRuns, Clock: s.deps.Clock, Log: log,
		Cluster: cluster, ClusterSignals: s.gatewayPlan().Signals, Measure: s.measureRecorder(), Live: st.liveStream(),
	})
	if err != nil {
		return err
	}
	if err := s.startPresenceWriter(gw); err != nil {
		return err
	}
	if err := bindAutocomplete(s.values, rn, gw); err != nil {
		return err
	}
	if ag.Presence != nil {
		// agent.presence_changed goes through the hub of this process (M1-66, D-M2-20-1).
		ag.Presence.BindPublisher(gw.Publisher)
	}
	if gw.Engine != nil {
		// The REST consumer operations read through the engine (agents.ConsumerFeed, M2-07).
		ag.Consumers.SetFeed(gw.Engine)
	}
	if rn != nil {
		// run.updated goes through the hub of this process (M2-10 design note 8).
		rn.Service.BindPublisher(gw.Publisher)
	}
	gw.Mount(mux, chain)
	s.runner.OnStop(gw.StartSweeper())
	s.runner.OnStop(gw.StartConsumerSaver())
	s.runner.OnStop(gw.StartLiveBox())
	s.runner.OnStop(gw.StopConnections())
	s.mu.Lock()
	s.gateway, s.cluster = gw, cluster
	s.mu.Unlock()
	return nil
}
