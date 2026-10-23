// SPDX-License-Identifier: Apache-2.0

package app

import (
	"time"

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

// FocusOf returns the focused thread ids of c in the hub index; nil when c is not registered.
func (h *Hub) FocusOf(c *Connection) []ids.ID {
	h.mu.Lock()
	defer h.mu.Unlock()
	m := h.members[c]
	if m == nil {
		return nil
	}
	out := make([]ids.ID, 0, len(m.focus))
	for _, e := range m.focus {
		out = append(out, e.ThreadID)
	}
	return out
}

// ForceFocus puts thread into the focus index of c without any check, so a test can prove that
// admission still refuses a reader without permission.
func (h *Hub) ForceFocus(c *Connection, thread ids.ID) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if m := h.members[c]; m != nil {
		h.setFocusLocked(m, append(m.focus, domain.FocusEntry{ThreadID: thread}))
	}
}

// FocusState returns the stored focus of the session.
func (s *Session) FocusState() domain.Focus { return s.focusState() }

// Sweep runs the REST window sweeper of e at now, for the tests of the engine.
func (e *ConsumerEngine) Sweep(now time.Time) { e.sweep(now) }

// LiveBoxState returns the number of runs, sent versions, snapshot marks and connections with
// pending focus snapshots the coalescer holds, so a test can prove that its state goes.
func (c *Coalescer) LiveBoxState() (runs, sent, marks, focused int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.runs), len(c.sent), len(c.marks), len(c.focused)
}
