// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"context"
	"errors"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/platform/jobs"
	syncpg "github.com/OWNER/bothy/services/server/internal/sync/adapters/postgres"
)

// notifyState is the part of the server state the notify module fills.
type notifyState struct {
	// notify is the notify module (M3-18), built in the chat step over chat, sync and identity.
	notify *Notify
	// push is the push path of notify (M3-19), built in the jobs step.
	push *Push
}

// moduleNotify is the name of the module in architecture/modules.yaml and of its start step, where it has one.
const moduleNotify = "notify"

// notifyWiring wires the notify module into the start step chat, after catch-up and snapshot,
// and its push path (M3-19) into the jobs step: the five jobs of notify on the queue delivery and
// the consumer push (startEventConsumers reads s.push).
func notifyWiring() moduleWiring {
	return moduleWiring{
		module:   moduleNotify,
		parts:    []part{{order: 1360, step: moduleChat, start: (*server).startNotify}},
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
// earlier steps; the providers come from Deps.PushProviders (none in production until M3-20 and
// M3-21).
func (s *server) buildPush() (*Push, error) {
	s.mu.Lock()
	pool, a, sy := s.pool, s.audit, s.sync
	n, c, m, p, id, az, rec := s.notify, s.chat, s.messaging, s.personal, s.identity, s.authorizer, s.instance
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
	return NewPush(cfg, pool, a.UoW(), PushDeps{
		Notify: n, Authorizer: az, Recipients: recipients, Directory: id.Directory, Devices: id.Devices, Settings: p.Service.Settings(),
		Chat: c.Service, Messaging: m.Service, History: sy.History(), Positions: syncpg.NewPersonalRepository(pool),
		InstanceID: rec.InstanceID, Providers: s.deps.PushProviders,
	}, s.deps.Clock, s.logger())
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
