// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"context"
	"errors"

	"github.com/OWNER/bothy/services/server/internal/authz"
)

// chatState is the part of the server state the chat module fills.
type chatState struct {
	// chat is the chat module (M1-52): spaces, the sidebar and their REST handlers.
	chat *Chat
}

// moduleChat is the name of the module in architecture/modules.yaml and of its start step, where it has one.
const moduleChat = "chat"

// chatWiring wires the chat module: the first part of the start step chat (6f).
func chatWiring() moduleWiring {
	return moduleWiring{
		module: moduleChat,
		parts:  []part{{order: 1300, step: moduleChat, start: (*server).startChat}},
		handlers: func(s *server, h *Handlers) {
			if s.chat != nil {
				s.chat.fillHandlers(h)
			}
		},
	}
}

// startChat is the first part of start step 6f (M1-52): the chat module over the pool, the unit
// of work and the recorder of the audit step, the appender of the sync step and the Authorizer.
// The step chat goes on with messaging (M1-56), runs, the personal data of sync, the readiness of
// agents, catch-up and snapshot, notify and stats (their module files).
func (s *server) startChat(context.Context) error {
	cfg, err := ChatConfig(s.values)
	if err != nil {
		return err
	}
	s.mu.Lock()
	pool, a, az, sy, id := s.pool, s.audit, s.authorizer, s.sync, s.identity
	s.mu.Unlock()
	rights, ok := az.(authz.RightsQuery)
	if a == nil || sy == nil || !ok || id == nil {
		return errors.New("wire: the chat step needs audit, sync, the Authorizer of authz and identity")
	}
	c, err := NewChat(cfg, pool, a.UoW(), az, rights, ChatWrites{Audit: a.Recorder(), Events: sy.Appender(), Rights: a.RightsHook(),
		Directory: id.Directory, Resolver: s.resolver()}, nil, s.deps.Clock)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.chat = c
	s.mu.Unlock()
	return nil
}
