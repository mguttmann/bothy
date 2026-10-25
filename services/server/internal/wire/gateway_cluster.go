// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"context"
	"errors"
	"time"

	gatewaymetrics "github.com/OWNER/bothy/services/server/internal/gateway/adapters/otelmetrics"
	gatewaypg "github.com/OWNER/bothy/services/server/internal/gateway/adapters/postgres"
	gatewayapp "github.com/OWNER/bothy/services/server/internal/gateway/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/platform/lifecycle"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
)

// The gateway across several api nodes (M7-17 Design note 3, docs/decisions.d/M7-17.md
// D-M7-17-7): what cluster.mode and the role mean for the channel ephemeral and the shared
// presence state gateway_presence.

// GatewayClusterPlan is what cluster.mode means for the gateway in one role:
//
//	| role, mode     | Signals | Listener | PresenceWrite | PresenceTable |
//	| all, single    | no      | no       | no            | no (hub)      |
//	| all, multi     | yes     | yes      | yes           | yes           |
//	| api, any       | multi   | yes      | yes           | (no push)     |
//	| worker, any    | hints   | no       | no            | yes           |
//
// A worker also sends the run signal of the stall marking of runs.stall_check (M2-51,
// runsPublisher in module_runs.go).
//
// Signals: the gateway sends its signals (typing, run, agent presence, device, consumer, hint)
// after the local publish. Listener: the process holds the one LISTEN ephemeral connection, also
// in role api with single, because a separate worker sends notification.created through it.
// PresenceWrite: the process writes gateway_presence. PresenceTable: the push path reads
// gateway_presence instead of the hub, and a worker publishes hints as signals.
type GatewayClusterPlan struct {
	Multi, Signals, Listener, PresenceWrite, PresenceTable bool
}

// GatewayPlan returns the plan of mode for role.
func GatewayPlan(mode string, role Role) GatewayClusterPlan {
	multi := mode == clusterModeMulti
	switch role {
	case RoleAPI:
		return GatewayClusterPlan{Multi: multi, Signals: multi, Listener: true, PresenceWrite: true}
	case RoleWorker:
		return GatewayClusterPlan{Multi: multi, PresenceTable: true}
	case RoleAll:
		return GatewayClusterPlan{Multi: multi, Signals: multi, Listener: multi, PresenceWrite: multi, PresenceTable: multi}
	}
	return GatewayClusterPlan{}
}

// gatewayPlan returns the plan of this process.
func (s *server) gatewayPlan() GatewayClusterPlan { return GatewayPlan(clusterMode(s.values), s.role) }

// PresenceConfig binds the keys of gateway_presence (M7-17).
type PresenceConfig struct {
	FlushInterval time.Duration `config:"cluster.presence_flush_interval"`
	TTL           time.Duration `config:"cluster.presence_ttl"`
}

// PresenceConfigOf binds PresenceConfig from v; the error names the key, never a value.
func PresenceConfigOf(v *config.Values) (PresenceConfig, error) {
	var c PresenceConfig
	if err := config.Bind(v, &c); err != nil {
		return PresenceConfig{}, err
	}
	return c, nil
}

// nodeID returns the process instance of this server (gateway_presence.node_id and the node of
// every signal), a UUIDv7 created once per start.
func (s *server) nodeID() ids.ID {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.node.IsZero() {
		s.node = ids.New()
	}
	return s.node
}

// newGatewayCluster builds the cluster of an api or all process when its plan sends or receives
// signals; nil otherwise. threads is the chat port of the places of rebuilt events.
func (s *server) newGatewayCluster(cfg GatewayConfig, threads gatewayapp.Threads) (*gatewayapp.Cluster, error) {
	plan := s.gatewayPlan()
	if !plan.Signals && !plan.Listener {
		return nil, nil
	}
	return gatewayapp.NewCluster(gatewayapp.ClusterConfig{Node: s.nodeID(), TypingTTL: cfg.TypingTTL}, gatewayapp.ClusterDeps{
		Sender: gatewaypg.NewSignalSender(s.pool), Threads: threads, Clock: s.deps.Clock, Log: s.logger(),
	})
}

// startPresenceWriter starts the writer of gateway_presence over the hub of gw when the plan
// writes it (role api, role all with multi) and registers its stop step in
// lifecycle.PhaseEventTail, after the connections closed, so the clean shutdown deletes the rows
// of the node.
func (s *server) startPresenceWriter(gw *Gateway) error {
	if !s.gatewayPlan().PresenceWrite || gw.Hub == nil {
		return nil
	}
	cfg, err := PresenceConfigOf(s.values)
	if err != nil {
		return err
	}
	metrics, err := gatewayPresenceMetrics()
	if err != nil {
		return err
	}
	w, err := gatewayapp.NewPresenceWriter(gatewayapp.PresenceWriterConfig{Node: s.nodeID(), FlushInterval: cfg.FlushInterval, TTL: cfg.TTL},
		gw.Hub, gatewaypg.NewPresenceStore(s.pool, s.audit.UoW()), s.deps.Clock, s.logger(), metrics)
	if err != nil {
		return err
	}
	w.Start()
	s.runner.OnStop(lifecycle.StopStep{Name: "gateway_presence", Phase: lifecycle.PhaseEventTail, Run: w.Stop})
	return nil
}

// startEphemeral is the component ephemeral_listener of a process whose plan listens (M7-17
// Design note 1): after the HTTP listeners it starts the one LISTEN ephemeral connection of the
// process with the routes of the gateway and, with cluster.mode multi, of the terminal (M2-47),
// and registers its stop step in lifecycle.PhaseEventTail; an unexpected end of its goroutine ends
// the process. It is a soft dependency: no readiness check.
func (s *server) startEphemeral(ctx context.Context) error {
	s.mu.Lock()
	pool, hub, cluster, hubCfg := s.pool, s.terminalHub, s.cluster, s.hubCfg
	s.mu.Unlock()
	var routes []pg.EphemeralRoute
	if cluster != nil {
		routes = append(routes, cluster)
	}
	if s.plan.Listener {
		if hub == nil {
			return errors.New("wire: the terminal route needs the hub of the relay")
		}
		r, err := terminalRoute(hub, s.logger())
		if err != nil {
			return err
		}
		routes = append(routes, r)
	}
	if pool == nil || len(routes) == 0 {
		return errors.New("wire: the ephemeral listener needs the pool and a route")
	}
	backoff := hubCfg.ReconnectBackoffCap
	if backoff <= 0 {
		backoff = s.values.Duration("database.listen_reconnect_backoff_cap")
	}
	l, err := pg.NewEphemeralListener(pool, pg.EphemeralConfig{ReconnectBackoffCap: backoff, Gate: s.deps.TerminalListenerGate},
		routes, s.runner.Fail, s.logger())
	if err != nil {
		return err
	}
	if err := l.Start(ctx); err != nil {
		return err
	}
	s.runner.OnStop(lifecycle.StopStep{Name: l.Name(), Phase: lifecycle.PhaseEventTail, Run: l.Stop})
	return nil
}

// gatewayPresenceMetrics returns the counter of failed presence writes on the global meter
// provider of platform/otel.
func gatewayPresenceMetrics() (*gatewaymetrics.PresenceMetrics, error) {
	return gatewaymetrics.NewPresence(nil)
}

// workerPresence returns, for a process whose push path reads gateway_presence (role worker, and
// role all with multi), the table-backed presence view and, for role worker, the publisher that
// sends notification.created to the api nodes as a signal (M7-17 Design note 3, D-M7-17-7).
// Role all with multi passes the cluster publisher of its gateway instead, which delivers locally
// first.
func (s *server) workerPresence() (*gatewaypg.PresenceView, *gatewayapp.Cluster, error) {
	cfg, err := PresenceConfigOf(s.values)
	if err != nil {
		return nil, nil, err
	}
	view, err := gatewaypg.NewPresenceView(s.pool, cfg.TTL, s.deps.Clock)
	if err != nil {
		return nil, nil, err
	}
	if s.role != RoleWorker {
		return view, nil, nil
	}
	gwCfg, err := GatewayConfigFrom(s.values)
	if err != nil {
		return nil, nil, err
	}
	cluster, err := gatewayapp.NewCluster(gatewayapp.ClusterConfig{Node: s.nodeID(), TypingTTL: gwCfg.TypingTTL},
		gatewayapp.ClusterDeps{Sender: gatewaypg.NewSignalSender(s.pool), Clock: s.deps.Clock, Log: s.logger()})
	return view, cluster, err
}
