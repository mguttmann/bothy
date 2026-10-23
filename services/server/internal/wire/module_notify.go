// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"context"
	"errors"
	"slices"

	"github.com/OWNER/bothy/services/server/internal/authz"
	gatewayapp "github.com/OWNER/bothy/services/server/internal/gateway/app"
	notifyapp "github.com/OWNER/bothy/services/server/internal/notify/app"
	"github.com/OWNER/bothy/services/server/internal/platform/egress"
	"github.com/OWNER/bothy/services/server/internal/platform/jobs"
	syncpg "github.com/OWNER/bothy/services/server/internal/sync/adapters/postgres"
)

// notifyState is the part of the server state the notify module fills.
type notifyState struct {
	// notify is the notify module (M3-18), built in the chat step over chat, sync and identity.
	notify *Notify
	// push is the push path of notify (M3-19), built in the jobs step.
	push *Push
	// apnsProviders are the push providers of the APNs adapter (M3-20), built in the chat step
	// in worker and all when APNs is enabled.
	apnsProviders []notifyapp.PushProvider
	// fcm holds the FCM adapter (M3-21) when FCM is enabled, built in the chat step of worker
	// and all; nil otherwise.
	fcm []notifyapp.PushProvider
}

// moduleNotify is the name of the module in architecture/modules.yaml and of its start step, where it has one.
const moduleNotify = "notify"

// notifyWiring wires the notify module into the start step chat, after catch-up and snapshot,
// with the APNs adapter (M3-20) right after it, and its push path (M3-19) into the jobs step: the five jobs of notify on the queue delivery and
// the consumer push (startEventConsumers reads s.push).
func notifyWiring() moduleWiring {
	return moduleWiring{
		module: moduleNotify,
		parts: []part{
			{order: 1360, step: moduleChat, start: (*server).startNotify},
			{order: 1365, step: moduleChat, start: (*server).startAPNs},
			{order: 1366, step: moduleChat, when: (*server).worker, start: (*server).startFCM},
		},
		handlers: func(s *server, h *Handlers) { s.notify.fillHandlers(h) },
		jobs: func(s *server, m *JobModules) error {
			push, err := s.buildPush()
			if err != nil {
				return err
			}
			m.Push = push.Push
			s.mu.Lock()
			s.push = push
			s.mu.Unlock()
			return nil
		},
		bindJobs: func(s *server, ins jobs.Inserter) {
			s.mu.Lock()
			push := s.push
			s.mu.Unlock()
			if push != nil {
				push.BindJobs(ins)
			}
		},
	}
}

// buildPush builds the push path of notify (M3-19) in the jobs step over the modules of the
// earlier steps; the providers are Deps.PushProviders, the APNs adapter of startAPNs and the FCM
// adapter of startFCM.
func (s *server) buildPush() (*Push, error) {
	s.mu.Lock()
	pool, a, sy := s.pool, s.audit, s.sync
	n, c, m, p, id, az, rec := s.notify, s.chat, s.messaging, s.personal, s.identity, s.authorizer, s.instance
	providers := slices.Concat(s.deps.PushProviders, s.apnsProviders, s.fcm)
	s.mu.Unlock()
	if a == nil || sy == nil || n == nil || c == nil || m == nil || p == nil || id == nil {
		return nil, errors.New("wire: the push path needs the chat step and the identity step")
	}
	recipients, ok := az.(authz.Recipients)
	if !ok {
		return nil, errors.New("wire: the push path needs the Authorizer of authz (authz.Recipients)")
	}
	cfg, err := PushConfig(s.values)
	if err != nil {
		return nil, err
	}
	d := PushDeps{
		Notify: n, Authorizer: az, Recipients: recipients, Directory: id.Directory, Devices: id.Devices, Settings: p.Service.Settings(),
		Chat: c.Service, Messaging: m.Service, History: sy.History(), Positions: syncpg.NewPersonalRepository(pool),
		InstanceID: rec.InstanceID, Providers: providers,
	}
	if err := s.wirePresence(&d); err != nil {
		return nil, err
	}
	return NewPush(cfg, pool, a.UoW(), d, s.deps.Clock, s.logger())
}

// wirePresence wires the presence of the push path by role and cluster.mode (13 12 point 4,
// RK-37; M3-23 design note 1, M7-17 Design note 3; docs/decisions.d/M3-23.md D-M3-23-1 and
// docs/decisions.d/M7-17.md D-M7-17-7):
//
//   - role all with single: the gateway's PresenceView over the hub, the live routes and the hub
//     as publisher of notification.created, because the hub and the consumer push share the
//     process and no other node exists;
//   - role all with multi: the table-backed view of gateway_presence (every node of the
//     installation), the live routes and the cluster publisher of the gateway, which delivers on
//     this node and sends the hint to the others;
//   - role worker: the table-backed view, the live routes and a publisher that sends the hint to
//     the api nodes over NOTIFY ephemeral (the worker holds no connection);
//   - role api runs no consumer and no job and wires nothing.
func (s *server) wirePresence(d *PushDeps) error {
	plan := s.gatewayPlan()
	switch s.role {
	case RoleAll:
		s.mu.Lock()
		gw := s.gateway
		s.mu.Unlock()
		if gw == nil || gw.Hub == nil {
			return nil
		}
		pub := gw.Publisher
		if pub == nil {
			pub = gw.Hub
		}
		d.Presence, d.LiveRoutes, d.Publisher = gatewayapp.NewPresenceView(gw.Hub), true, pub
		if plan.PresenceTable {
			view, _, err := s.workerPresence()
			if err != nil {
				return err
			}
			d.Presence = view
		}
	case RoleWorker:
		view, cluster, err := s.workerPresence()
		if err != nil {
			return err
		}
		d.Presence, d.LiveRoutes, d.Publisher = view, true, cluster.Publisher(nil)
	case RoleAPI:
	}
	return nil
}

// startAPNs builds the APNs adapter (M3-20) over the secret provider and the egress
// configuration of the process: in worker and all with APNs enabled, otherwise none
// (APNsProviders). An unusable APNs key ends the start here.
func (s *server) startAPNs(ctx context.Context) error {
	eg, err := EgressConfig(s.values)
	if err != nil {
		return err
	}
	providers, err := APNsProviders(ctx, s.values, s.role, s.apnsDeps(eg))
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.apnsProviders = providers
	s.mu.Unlock()
	return nil
}

// apnsDeps returns the collaborators of the APNs adapter of the process for the egress
// configuration eg: the secret provider, the egress dependencies of the process and the test
// overrides Deps.APNsEndpoints and Deps.APNsRootCAs, which production leaves nil.
func (s *server) apnsDeps(eg egress.Config) APNsDeps {
	ed := s.egressDeps()
	ed.RootCAs = s.deps.APNsRootCAs
	return APNsDeps{Secrets: s.sp, Egress: eg, EgressDeps: ed, Endpoints: s.deps.APNsEndpoints, Clock: s.deps.Clock, Logger: s.logger()}
}

// startFCM builds the FCM adapter (M3-21) in the chat step of worker and all when
// push.fcm.project_id is set; a service account that cannot be used stops the start. The role
// api runs no notify.push_send and reads no service account.
func (s *server) startFCM(ctx context.Context) error {
	cfg, err := FCMConfig(s.values)
	if err != nil {
		return err
	}
	eg, err := EgressConfig(s.values)
	if err != nil {
		return err
	}
	providers, err := FCMProviders(ctx, s.role, cfg, FCMDeps{Egress: eg, EgressDeps: s.egressDeps(), Secrets: s.sp,
		Clock: s.deps.Clock, Logger: s.logger()})
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.fcm = providers
	s.mu.Unlock()
	return nil
}

// startNotify builds the notify module (M3-18) in the chat step over the chat service, the
// settings of the personal data of sync and the directory of identity, and binds the
// in-transaction subscribers of notify of the unit of work to the appender and the chat paths.
func (s *server) startNotify(context.Context) error {
	cfg, err := NotifyConfig(s.values)
	if err != nil {
		return err
	}
	s.mu.Lock()
	pool, a, az, sy, c, p, id := s.pool, s.audit, s.authorizer, s.sync, s.chat, s.personal, s.identity
	s.mu.Unlock()
	if a == nil || sy == nil || c == nil || p == nil || id == nil {
		return errors.New("wire: notify needs audit, sync, chat, the personal data of sync and identity")
	}
	n, err := NewNotify(cfg, pool, a.UoW(), az, sy.Appender(), NotifyDeps{Chat: c.Service, Settings: p.Service.Settings(), Users: id.Directory},
		s.deps.Clock, s.logger())
	if err != nil {
		return err
	}
	if err := a.NotifySubscribers().WithClock(s.deps.Clock).Bind(sy.Appender(), c.Service); err != nil {
		return err
	}
	s.mu.Lock()
	s.notify = n
	s.mu.Unlock()
	return nil
}
