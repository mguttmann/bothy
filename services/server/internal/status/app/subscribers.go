// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/agents"
)

// Subscribers are the in-transaction subscribers of the status module (02 section 4.3): a
// disabled agent leaves the status page in the transaction that disables it (agents.AgentDisabled,
// brief M7-09); an enabled agent gets its component back with the next round of status.probe. They
// are safe for concurrent use.
type Subscribers struct {
	store PageStore
}

// NewSubscribers returns the subscribers over store.
func NewSubscribers(store PageStore) *Subscribers { return &Subscribers{store: store} }

// OnAgentDisabled removes the components of the agent. It appends no status.updated: only agent
// components changed (D-M7-09-5). Errors: the store's.
func (s *Subscribers) OnAgentDisabled(ctx context.Context, ev agents.AgentDisabled) error {
	_, err := s.store.DeleteAgentComponents(ctx, ev.AgentID)
	return err
}
