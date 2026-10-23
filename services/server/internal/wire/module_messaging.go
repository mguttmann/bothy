// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"context"
	"errors"

	"github.com/OWNER/bothy/services/server/internal/runs/adapters/messagingsource"
)

// messagingState is the part of the server state the messaging module fills.
type messagingState struct {
	// messaging is the messaging module (M1-56), built in the chat step over the threads of chat.
	messaging *Messaging
	// cardComponents is the source of the components of cards (M2-15): runs implements the source
	// ports of messaging and binds its use cases to it once the runs module exists (startRuns).
	cardComponents *messagingsource.Source
}

// moduleMessaging is the name of the module in architecture/modules.yaml and of its start step, where it has one.
const moduleMessaging = "messaging"

// messagingWiring wires the messaging module into the start step chat, after chat.
func messagingWiring() moduleWiring {
	return moduleWiring{
		module:   moduleMessaging,
		parts:    []part{{order: 1310, step: moduleChat, start: (*server).startMessaging}},
		handlers: func(s *server, h *Handlers) { s.messaging.fillHandlers(h) },
	}
}

// startMessaging builds the messaging module (M1-56) in the chat step over the threads of chat,
// the appender and redactor of sync, the recorder of audit and the directory of identity, with
// the component source of cards that the runs part binds (M2-15).
func (s *server) startMessaging(context.Context) error {
	mcfg, err := MessagingConfig(s.values)
	if err != nil {
		return err
	}
	s.mu.Lock()
	pool, a, az, sy, id, c := s.pool, s.audit, s.authorizer, s.sync, s.identity, s.chat
	s.mu.Unlock()
	if a == nil || sy == nil || id == nil || c == nil {
		return errors.New("wire: messaging needs audit, sync, identity and chat")
	}
	components := messagingsource.New()
	m, err := NewMessaging(mcfg, pool, a.UoW(), az, MessagingWrites{Events: sy.Appender(), Redactor: sy.Redactor(), Audit: a.Recorder(),
		Credentials: lazyCredentials(s.authenticator), Components: components}, c.Service, id.Directory, s.deps.Clock, s.logger())
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.messaging, s.cardComponents = m, components
	s.mu.Unlock()
	return nil
}
