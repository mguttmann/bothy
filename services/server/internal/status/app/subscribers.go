// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"sync"

	"github.com/OWNER/bothy/services/server/internal/agents"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
)

// errSubscribersUnbound reports a subscriber that runs before the start bound the appender.
var errSubscribersUnbound = errors.New("status: the in-transaction subscribers are not bound to the appender")

// Subscribers are the in-transaction subscribers of the status module (02 section 4.3): a
// disabled agent leaves the status page in the transaction that disables it (agents.AgentDisabled,
// brief M7-09); an enabled agent gets its component back with the next round of status.probe. They
// write nothing until Bind gives them the appender of sync. They are safe for concurrent use.
type Subscribers struct {
	store PageStore

	mu     sync.Mutex
	events eventlog.Appender
}

// NewSubscribers returns the unbound subscribers over store.
func NewSubscribers(store PageStore) *Subscribers { return &Subscribers{store: store} }

// Bind gives the subscribers the appender of status.updated.
func (s *Subscribers) Bind(events eventlog.Appender) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = events
}

// OnAgentDisabled removes the components of the agent. It appends no status.updated: only agent
// components changed (D-M7-09-5). Errors: the store's, the appender's, or
// errSubscribersUnbound when a component was removed before Bind.
func (s *Subscribers) OnAgentDisabled(ctx context.Context, ev agents.AgentDisabled) error {
	if s.store == nil {
		return errSubscribersUnbound
	}
	gone, err := s.store.DeleteAgentComponents(ctx, ev.AgentID)
	if err != nil || len(gone) == 0 {
		return err
	}
	s.mu.Lock()
	events := s.events
	s.mu.Unlock()
	if events == nil {
		return errSubscribersUnbound
	}
	comps, err := s.store.Components(ctx)
	if err != nil {
		return err
	}
	return appendUpdated(ctx, events, overall(comps), nil, nil)
}
