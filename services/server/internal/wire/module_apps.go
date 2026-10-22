// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"context"
	"errors"

	instancepg "github.com/OWNER/bothy/services/server/internal/instance/adapters/postgres"
	instanceapp "github.com/OWNER/bothy/services/server/internal/instance/app"
	"github.com/OWNER/bothy/services/server/internal/platform/egress"
	"github.com/OWNER/bothy/services/server/internal/platform/jobs"
)

// appsState is the part of the server state the apps module fills.
type appsState struct {
	// apps is the module apps (M2-24): the app use cases and their REST handlers.
	apps *Apps
}

// moduleApps is the name of the module in architecture/modules.yaml and of its start step, where it has one.
const moduleApps = "apps"

// appsWiring wires the module apps: its start step (6h), the job apps.webhook_deliver (M2-26)
// and the inserter of its scheduler.
func appsWiring() moduleWiring {
	return moduleWiring{
		module:   moduleApps,
		parts:    []part{{order: 1600, step: moduleApps, start: (*server).startApps}},
		handlers: func(s *server, h *Handlers) { s.apps.fillHandlers(h) },
		jobs: func(s *server, m *JobModules) error {
			s.mu.Lock()
			ap := s.apps
			s.mu.Unlock()
			if ap == nil {
				return errors.New("wire: the jobs step needs the apps step")
			}
			m.Deliveries = ap.Webhooks
			return nil
		},
		bindJobs: func(s *server, ins jobs.Inserter) {
			s.mu.Lock()
			ap := s.apps
			s.mu.Unlock()
			ap.BindJobs(ins)
		},
	}
}

// startApps is start step 6h (M2-24, M2-26): the module apps over the pool, the unit of work,
// rights hook and recorder of the audit step, the Authorizer and identity, with the outgoing
// webhooks over the egress configuration, the master key of the v1 secrets (resolved once, so a
// missing or malformed key ends the start), the instance row and the log reader of sync. The
// signing keys and the job runtime start later and are bound then (BindKeys, BindJobs).
func (s *server) startApps(ctx context.Context) error {
	cfg, err := AppsConfig(s.values)
	if err != nil {
		return err
	}
	wcfg, masterRef, err := WebhooksConfig(s.values)
	if err != nil {
		return err
	}
	eg, err := EgressConfig(s.values)
	if err != nil {
		return err
	}
	sender, err := NewWebhookSender(eg, wcfg, egress.Deps{Logger: s.logger()})
	if err != nil {
		return err
	}
	master := NewHMACMasterKey(s.sp, masterRef)
	if err := master.Check(ctx); err != nil {
		return err
	}
	icfg, err := IncomingConfig(s.values)
	if err != nil {
		return err
	}
	s.mu.Lock()
	pool, a, az, admin, id, sy, ch, msg := s.pool, s.audit, s.authorizer, s.authzAdmin, s.identity, s.sync, s.chat, s.messaging
	s.mu.Unlock()
	if a == nil || az == nil || admin == nil || id == nil || sy == nil || ch == nil || msg == nil {
		return errors.New("wire: the apps step needs the audit step, sync, the Authorizer, the authz Admin, identity, chat and messaging")
	}
	state := instanceapp.NewState(instancepg.NewRepository(pool), a.UoW())
	ap, err := NewApps(cfg, pool, AppsDeps{UoW: a.UoW(), Rights: a.RightsHook(), Authorizer: az, Audit: a.Recorder(), Identity: id,
		Grants: admin.Admin, Clock: s.deps.Clock, Webhooks: WebhookDeps{
			Config: wcfg, State: state, Log: sy.LogReader(), Master: master, Sender: sender, Logger: s.logger(),
		}, Incoming: icfg, Chat: ch, Messages: msg})
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.apps = ap
	s.mu.Unlock()
	return nil
}
