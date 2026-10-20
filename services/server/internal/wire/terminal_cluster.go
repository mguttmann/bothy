// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/platform/lifecycle"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	terminalpg "github.com/OWNER/bothy/services/server/internal/terminal/adapters/postgres"
	"github.com/OWNER/bothy/services/server/internal/terminal/adapters/wsrelay"
	terminalapp "github.com/OWNER/bothy/services/server/internal/terminal/app"
)

// The terminal across several api nodes (M2-47 Design note 2): cluster.mode chooses the signals
// of the terminal module and whether an api process holds the LISTEN ephemeral connection.

// ClusterPlan is what cluster.mode means for one role (ruling D-M2-47-2): Notify selects the
// NOTIFY signals of adapters/postgres instead of the local ones; Listener starts the component
// terminal_ephemeral with its stop step in lifecycle.PhaseEventTail.
type ClusterPlan struct {
	Multi    bool
	Notify   bool
	Listener bool
}

// TerminalClusterPlan returns the plan of mode for role: single sends no NOTIFY and runs no
// listener; multi uses the NOTIFY signals in every role, and the roles api and all, which hold
// connections, run one listener per process; the worker commits ends and units without a
// connection, so it needs no listener.
func TerminalClusterPlan(mode string, role Role) ClusterPlan {
	if mode != clusterModeMulti {
		return ClusterPlan{}
	}
	return ClusterPlan{Multi: true, Notify: true, Listener: role == RoleAPI || role == RoleAll}
}

// clusterMode returns cluster.mode of v.
func clusterMode(v *config.Values) string { return v.String(keyClusterMode) }

// terminalSignals returns the signals of the plan over pool: the NOTIFY implementation with multi,
// else LocalSignals, whose handler the relay binds once it exists. wrap, when set, wraps them (a
// test seam, Deps.TerminalSignals).
func terminalSignals(plan ClusterPlan, pool *pg.Pool, wrap func(terminalapp.Signals) terminalapp.Signals) (terminalapp.Signals, *terminalapp.LocalSignals) {
	var sig terminalapp.Signals
	var local *terminalapp.LocalSignals
	if plan.Notify {
		sig = terminalpg.NewNotifySignals(pool)
	} else {
		local = terminalapp.NewLocalSignals()
		sig = local
	}
	if wrap != nil {
		sig = wrap(sig)
	}
	return sig, local
}

// ClusterHubConfig binds the keys of the hub (M2-47).
type ClusterHubConfig struct {
	HeartbeatInterval     time.Duration `config:"terminal.host_heartbeat_interval"`
	HeartbeatMissLimit    int           `config:"terminal.host_heartbeat_miss_limit"`
	DuplicateProbeTimeout time.Duration `config:"terminal.host_duplicate_probe_timeout"`
	ReconnectBackoffCap   time.Duration `config:"database.listen_reconnect_backoff_cap"`
}

// newTerminalHub builds the hub of the signals over the module, the control connections and the
// relay of an api process; with one node it becomes the handler of local, with several the
// listener's.
func newTerminalHub(v *config.Values, plan ClusterPlan, tm *Terminal, hc *HostControl, ts *TerminalStreams, local *terminalapp.LocalSignals,
	clk clock.Clock, log *slog.Logger) (*wsrelay.Hub, ClusterHubConfig, error) {
	var cfg ClusterHubConfig
	if err := config.Bind(v, &cfg); err != nil {
		return nil, ClusterHubConfig{}, err
	}
	hub, err := wsrelay.NewHub(wsrelay.ClusterConfig{Multi: plan.Multi, HeartbeatInterval: cfg.HeartbeatInterval,
		HeartbeatMissLimit: cfg.HeartbeatMissLimit, DuplicateProbeTimeout: cfg.DuplicateProbeTimeout}, tm.Service, hc.Links, ts.Relay, clk, log)
	if err != nil {
		return nil, ClusterHubConfig{}, err
	}
	if local != nil {
		local.Bind(hub)
	}
	return hub, cfg, nil
}

// startTerminalEphemeral is the component terminal_ephemeral of api and all with cluster.mode
// multi (Design note 5): after the HTTP listeners it starts the LISTEN ephemeral connection of the
// process with the hub as handler and reconciler and registers its stop step in
// lifecycle.PhaseEventTail; an unexpected end of its goroutine ends the process. It is a soft
// dependency: no readiness check.
func (s *server) startTerminalEphemeral(ctx context.Context) error {
	s.mu.Lock()
	hub, pool, cfg := s.terminalHub, s.pool, s.hubCfg
	s.mu.Unlock()
	if hub == nil || pool == nil {
		return errors.New("wire: the terminal listener needs the hub of the relay and the pool")
	}
	l, err := terminalpg.NewEphemeralListener(pool, terminalpg.ListenerConfig{ReconnectBackoffCap: cfg.ReconnectBackoffCap,
		Gate: s.deps.TerminalListenerGate}, hub, hub, s.runner.Fail, s.logger())
	if err != nil {
		return err
	}
	if err := l.Start(ctx); err != nil {
		return err
	}
	s.runner.OnStop(lifecycle.StopStep{Name: l.Name(), Phase: lifecycle.PhaseEventTail, Run: l.Stop})
	return nil
}
