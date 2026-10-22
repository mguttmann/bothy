// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"context"
	"errors"
)

// notifyState is the part of the server state the notify module fills.
type notifyState struct {
	// notify is the notify module (M3-18), built in the chat step over chat, sync and identity.
	notify *Notify
}

// moduleNotify is the name of the module in architecture/modules.yaml and of its start step, where it has one.
const moduleNotify = "notify"

// notifyWiring wires the notify module into the start step chat, after catch-up and snapshot.
func notifyWiring() moduleWiring {
	return moduleWiring{
		module:   moduleNotify,
		parts:    []part{{order: 1360, step: moduleChat, start: (*server).startNotify}},
		handlers: func(s *server, h *Handlers) { s.notify.fillHandlers(h) },
	}
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
	if err := a.NotifySubscribers().Bind(sy.Appender(), c.Service); err != nil {
		return err
	}
	s.mu.Lock()
	s.notify = n
	s.mu.Unlock()
	return nil
}
