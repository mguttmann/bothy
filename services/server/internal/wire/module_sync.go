// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"context"
	"errors"

	gatewayapp "github.com/OWNER/bothy/services/server/internal/gateway/app"
	identitypg "github.com/OWNER/bothy/services/server/internal/identity/adapters/postgres"
	identityapp "github.com/OWNER/bothy/services/server/internal/identity/app"
	instancepg "github.com/OWNER/bothy/services/server/internal/instance/adapters/postgres"
	instanceapp "github.com/OWNER/bothy/services/server/internal/instance/app"
	"github.com/OWNER/bothy/services/server/internal/sync"
	"github.com/OWNER/bothy/services/server/internal/sync/adapters/otelmetrics"
)

// syncState is the part of the server state the sync module fills.
type syncState struct {
	// syncCfg and sync are the event log (M1-16): its appender for the modules, the partition
	// maintenance of the job and the log tail of api and all.
	syncCfg SyncConfig
	sync    *Sync
	// personal is the personal data of sync (M1-61), built in the chat step over chat and messaging.
	personal *Personal
	// syncReads is catch-up and snapshot of sync (M1-63), built in the chat step after personal.
	syncReads *SyncReads
}

// moduleSync is the name of the module in architecture/modules.yaml and of its start step, where it has one.
const moduleSync = "sync"

// syncWiring wires the sync module: the event log (6b), the personal data and catch-up and
// snapshot in the step chat, the log tail (api, all), the worker consumers (worker, all) and the
// jobs sync.partition_maintenance and sync.idempotency_cleanup.
func syncWiring() moduleWiring {
	return moduleWiring{
		module: moduleSync,
		parts: []part{
			{order: 800, step: moduleSync, start: (*server).startSync},
			{order: 1330, step: moduleChat, start: (*server).startPersonal},
			{order: 1350, step: moduleChat, start: (*server).startSyncReads},
			{order: 2100, step: "log_tail", when: (*server).api, start: whileFinalized((*server).startLogTail)},
			// 02 "Start" step 8: the worker consumers start after River (M2-01).
			{order: 2400, step: "event_consumers", when: (*server).worker, start: whileFinalized((*server).startEventConsumers)},
		},
		handlers: func(s *server, h *Handlers) {
			s.personal.fillHandlers(h)
			s.syncReads.fillHandlers(h)
		},
		jobs: func(s *server, m *JobModules) error {
			s.mu.Lock()
			sy := s.sync
			s.mu.Unlock()
			m.Partitions, m.PartitionInterval = sy.Maintenance(), s.syncCfg.MaintenanceInterval
			m.Cleanup, m.CleanupInterval = sy.IdempotencyCleanup(), s.syncCfg.Cleanup.Interval
			return nil
		},
	}
}

// startSync is start step 6b: it builds the event log over the pool and the unit of work of the
// audit step, whose registry holds the sync hook (M1-16), and registers the lag gauges of the
// worker consumers in every role, so a stopped or missing worker still shows a growing lag
// (M2-01, D-M2-01-5).
func (s *server) startSync(context.Context) error {
	s.mu.Lock()
	pool, a := s.pool, s.audit
	s.mu.Unlock()
	if a == nil {
		return errors.New("wire: the sync step needs the unit of work of the audit step")
	}
	sy, err := NewSync(pool, a.UoW(), s.syncCfg, nil, s.logger())
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.sync = sy
	s.mu.Unlock()
	gauges := s.deps.ConsumerGauges
	if gauges == nil {
		gauges = func(c sync.Consumers) error {
			_, err := otelmetrics.RegisterConsumerGauges(nil, c, s.deps.Clock)
			return err
		}
	}
	return gauges(sy.Consumers())
}

// startEventConsumers starts the worker consumers of worker and all after River
// (startConsumers) with the production handlers or those of Deps.ConsumerHandlers.
func (s *server) startEventConsumers(ctx context.Context) error {
	s.mu.Lock()
	sy, ap := s.sync, s.apps
	s.mu.Unlock()
	handlers := consumerHandlers(ap)
	if s.deps.ConsumerHandlers != nil {
		handlers = s.deps.ConsumerHandlers
	}
	return startConsumers(ctx, s.runner, sy, handlers)
}

// startLogTail starts the log tail of api and all (02 "Start and graceful shutdown" step 8, startTail).
func (s *server) startLogTail(ctx context.Context) error {
	s.mu.Lock()
	pool, sy, az, gw, ts := s.pool, s.sync, s.authorizer, s.gateway, s.terminalStreams
	s.mu.Unlock()
	var hub *gatewayapp.Hub
	if gw != nil {
		hub = gw.Hub
	}
	var observers []LogObserver
	if ts != nil {
		observers = append(observers, ts.Observer())
	}
	versions := instanceapp.NewVersions(instancepg.NewRepository(pool))
	tail := sy.NewTail(DeliverySink(az, hub, versions, observers...))
	if err := startTail(ctx, s.runner, s.reg, tail); err != nil {
		return err
	}
	if hub != nil {
		// The hub position starts at the head the tail started at (agent consumers, M2-07).
		hub.Seed(tail.Head())
	}
	return nil
}

// startPersonal builds the personal data of sync (M1-61) in the chat step over chat and messaging.
func (s *server) startPersonal(context.Context) error {
	pcfg, err := PersonalConfig(s.values)
	if err != nil {
		return err
	}
	s.mu.Lock()
	pool, a, az, sy, c, m := s.pool, s.audit, s.authorizer, s.sync, s.chat, s.messaging
	s.mu.Unlock()
	if a == nil || sy == nil || c == nil || m == nil {
		return errors.New("wire: the personal data need audit, sync, chat and messaging")
	}
	p, err := NewPersonal(pcfg, pool, a.UoW(), az, sy.Appender(), c.Service, m.Service, s.deps.Clock)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.personal = p
	s.mu.Unlock()
	return nil
}

// startSyncReads builds catch-up and snapshot of sync (M1-63) in the chat step over the chat,
// messaging and runs modules and the personal data, the agents step, the principal directory of
// identity and the instance settings (feature flags).
func (s *server) startSyncReads(context.Context) error {
	cfg, err := CatchUpConfig(s.values)
	if err != nil {
		return err
	}
	s.mu.Lock()
	pool, a, az, c, m, p, rn, ag, st := s.pool, s.audit, s.authorizer, s.chat, s.messaging, s.personal, s.runs, s.agents, s.settings
	s.mu.Unlock()
	if ag == nil || st == nil {
		return errors.New("wire: catch-up and snapshot need the agents and settings steps")
	}
	if a == nil || c == nil || m == nil || p == nil || rn == nil {
		return errors.New("wire: catch-up and snapshot need audit, chat, messaging, the personal data and runs")
	}
	r, err := NewSyncReads(cfg, SyncReadDeps{Pool: pool, UoW: a.UoW(), Authorizer: az, Chat: c.Service, Messaging: m.Service, Personal: p.Service,
		Runs:   rn.Service,
		Agents: ag.Service, Directory: identityapp.NewDirectory(identitypg.NewRepository(pool)), Flags: st.Admin})
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.syncReads = r
	s.mu.Unlock()
	return nil
}
