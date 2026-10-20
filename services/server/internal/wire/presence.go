// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"context"
	"errors"
	"log/slog"

	agentsmetrics "github.com/OWNER/bothy/services/server/internal/agents/adapters/otelmetrics"
	agentspg "github.com/OWNER/bothy/services/server/internal/agents/adapters/postgres"
	agentsapp "github.com/OWNER/bothy/services/server/internal/agents/app"
	identitypg "github.com/OWNER/bothy/services/server/internal/identity/adapters/postgres"
	identityapp "github.com/OWNER/bothy/services/server/internal/identity/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/platform/lifecycle"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
)

// PresenceConfigFrom binds the keys of the presence tracker (M2-20): agent.presence_timeout,
// agent.heartbeat_persist_interval, billing.mismatch_alert_after and sync.heartbeat_interval. The
// error names the key whose binding failed, never a value.
func PresenceConfigFrom(v *config.Values) (agentsapp.PresenceConfig, error) {
	var c agentsapp.PresenceConfig
	if err := config.Bind(v, &c); err != nil {
		return agentsapp.PresenceConfig{}, err
	}
	return c, nil
}

// PresenceDeps are the collaborators of the presence tracker that the agents step has.
type PresenceDeps struct {
	UoW        txn.UnitOfWork
	Authorizer authorize.Authorizer
	Audit      auditing.Recorder
	// Clock nil means clock.System(); Timers nil means agentsapp.RealTimers; Log nil discards.
	Clock  clock.Clock
	Timers agentsapp.Timers
	Log    *slog.Logger
}

// BindPresence builds the presence tracker of the process over pool (M2-20, docs/DECISIONS.md
// D-M2-20-1) and binds it into the agent use cases and the REST handler of sendHeartbeat. The
// ephemeral publisher (the gateway hub) and the readiness port of chat are bound later
// (Tracker.BindPublisher in mountGateway, Tracker.BindReadiness in the chat step), before the
// public listener starts. Errors: a missing collaborator or a window the tracker refuses.
func (a *Agents) BindPresence(cfg agentsapp.PresenceConfig, pool *pg.Pool, d PresenceDeps) error {
	if a == nil || d.UoW == nil || d.Authorizer == nil || d.Audit == nil {
		return errors.New("wire: the presence tracker needs the agents module, the unit of work, the Authorizer and the recorder")
	}
	tr, err := agentsapp.NewTracker(cfg, agentsapp.PresenceDeps{
		Store: agentspg.NewRepository(pool), Principals: identityapp.NewDirectory(identitypg.NewRepository(pool)), UoW: d.UoW,
		Authorizer: d.Authorizer, Audit: d.Audit, Clock: d.Clock, Timers: d.Timers, Log: orDiscard(d.Log),
	})
	if err != nil {
		return err
	}
	a.Presence = tr
	a.Service.BindPresence(tr)
	a.Handlers.WithPresence(tr, cfg.HeartbeatInterval)
	return nil
}

// StartPresence is the start of the tracker in the roles api and all (D-M2-20-6): the start rule
// over the stored live agents, the flush loop, and the two presence gauges on the global meter
// provider of platform/otel (the registration lives as long as the process). Errors: the start's
// and the meter's.
func (a *Agents) StartPresence(ctx context.Context) error {
	if a == nil || a.Presence == nil {
		return errors.New("wire: no presence tracker bound")
	}
	if err := a.Presence.Start(ctx); err != nil {
		return err
	}
	_, err := agentsmetrics.RegisterPresence(nil, a.Presence.Samples)
	return err
}

// PresenceStopStep returns the stop step agent_presence in lifecycle.PhaseConnections (D-M2-20-5).
// Steps of one phase run in reverse registration order, so the server registers it before the
// stop step of the gateway connections: it runs after them, when no heartbeat can arrive. It
// stops the timers without publishing and writes every dirty entry within the rest of
// shutdown.total_timeout.
func (a *Agents) PresenceStopStep() lifecycle.StopStep {
	return lifecycle.StopStep{Name: agentsapp.StopStepName, Phase: lifecycle.PhaseConnections, Run: func(ctx context.Context) error {
		if a.Presence == nil {
			return nil
		}
		return a.Presence.Stop(ctx)
	}}
}
