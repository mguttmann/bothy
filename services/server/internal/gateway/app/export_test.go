// SPDX-License-Identifier: Apache-2.0

package app

import (
	"github.com/OWNER/bothy/services/server/internal/gateway/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// NewTestConnection returns a ready connection on conn with the actor a, for the tests of the hub.
func NewTestConnection(conn Conn, a actor.Actor) (*Connection, *Session) {
	s := &Session{ID: ids.New(), PrincipalID: a.ID, PrincipalKind: a.Kind, DeviceID: a.DeviceID, AuthSessionID: a.SessionID, TokenID: a.TokenID}
	return &Connection{id: ids.New(), conn: conn, state: domain.StateReady, session: s, actor: a}, s
}

// PhaseOf returns the delivery phase of c in h; closing when c is not registered.
func (h *Hub) PhaseOf(c *Connection) domain.Phase {
	h.mu.Lock()
	m := h.members[c]
	h.mu.Unlock()
	if m == nil {
		return domain.PhaseClosing
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.phase
}
