// SPDX-License-Identifier: Apache-2.0

package app

import (
	"cmp"
	"context"
	"slices"

	"github.com/OWNER/bothy/services/server/internal/gateway/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/presence"
)

// PresenceView implements the kernel port presence.View over the hub of this process (13 4.4,
// 13 12 point 4; brief M3-23 design note 2; docs/DECISIONS.md D-M3-23-2): for each requested
// person it returns one entry per device with a counted connection, with a copy of the focused
// threads and the idle flag of the last focus op. The answer is an internal fact of the hint
// decision of notify; it is never sent to anyone and never logged (OQ-06). internal/wire wires
// it only in role all, where the hub and the consumer push share the process (D-M3-23-1).
type PresenceView struct{ h *Hub }

// Compile-time check.
var _ presence.View = (*PresenceView)(nil)

// NewPresenceView returns the presence view over h.
//
// Example:
//
//	view := app.NewPresenceView(hub)
//	conns, err := view.Connections(ctx, []ids.ID{alexID})
func NewPresenceView(h *Hub) *PresenceView { return &PresenceView{h: h} }

// Connections implements presence.View: see Hub.Presence. It reads no database and never waits
// on a connection. Errors: only the error of a cancelled ctx.
func (v *PresenceView) Connections(ctx context.Context, userIDs []ids.ID) (map[ids.ID][]presence.Connection, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return v.h.Presence(userIDs), nil
}

// presenceCandidate is the copy of one member of a requested person, taken under the hub's lock.
type presenceCandidate struct {
	m     *member
	seq   uint64
	focus []domain.FocusEntry
}

// Presence returns, for each person of userIDs with at least one counted connection, one
// presence.Connection per device; people without one are absent from the map. A connection
// counts exactly when its principal is a human (kind user) with a device, it is in phase live
// (after catchup.complete, 04 7.2 item 4, because notification.created is never delivered before
// it) and it is not closing. Focus is a copy of the focused threads the hub keeps (after the
// checks and the pruning of M1-66), Idle the flag of the last focus op; a connection that never
// sent focus has an empty focus and Idle false, so a connected desktop counts as active (13 4.4).
// A device holds one connection (04 6.1); while the hub briefly holds two during a replacement,
// the newer registration wins. The index fields are copied under the hub's lock, the phase, the
// close state and the idle flag each under their own short lock afterwards, so the read never
// nests locks and never waits on a connection queue. Every returned slice is new.
func (h *Hub) Presence(userIDs []ids.ID) map[ids.ID][]presence.Connection {
	byUser := h.presenceCandidates(userIDs)
	out := make(map[ids.ID][]presence.Connection, len(byUser))
	for u, cands := range byUser {
		if conns := devicePresence(cands); len(conns) > 0 {
			out[u] = conns
		}
	}
	return out
}

// presenceCandidates copies, under the hub's lock, the registration order and the focus of every
// connection of each person of userIDs.
func (h *Hub) presenceCandidates(userIDs []ids.ID) map[ids.ID][]presenceCandidate {
	byUser := make(map[ids.ID][]presenceCandidate, len(userIDs))
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, u := range userIDs {
		if _, done := byUser[u]; done {
			continue
		}
		for m := range h.byPrincipal[u] {
			byUser[u] = append(byUser[u], presenceCandidate{m: m, seq: m.seq, focus: slices.Clone(m.focus)})
		}
	}
	return byUser
}

// devicePresence returns one entry per device of the counted connections of cands, the newest
// registration of a device first.
func devicePresence(cands []presenceCandidate) []presence.Connection {
	slices.SortFunc(cands, func(a, b presenceCandidate) int { return cmp.Compare(b.seq, a.seq) })
	var conns []presence.Connection
	for _, c := range cands {
		pc, ok := counted(c)
		if !ok || slices.ContainsFunc(conns, func(x presence.Connection) bool { return x.DeviceID == pc.DeviceID }) {
			continue
		}
		conns = append(conns, pc)
	}
	return conns
}

// counted returns the presence entry of the candidate c and whether its connection counts.
func counted(c presenceCandidate) (presence.Connection, bool) {
	if c.m.consumer || c.m.c.Closing() {
		return presence.Connection{}, false
	}
	_, a := c.m.c.current()
	if a.Kind != actor.KindUser || a.DeviceID.IsZero() {
		return presence.Connection{}, false
	}
	c.m.mu.Lock()
	live := c.m.phase == domain.PhaseLive
	c.m.mu.Unlock()
	if !live {
		return presence.Connection{}, false
	}
	focus := make([]ids.ID, 0, len(c.focus))
	for _, e := range c.focus {
		focus = append(focus, e.ThreadID)
	}
	return presence.Connection{DeviceID: a.DeviceID, Focus: focus, Idle: c.m.s.focusState().Idle}, true
}
