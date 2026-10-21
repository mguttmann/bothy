// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"slices"

	"github.com/OWNER/bothy/services/server/internal/gateway/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/sync"
)

// Focus implements Delivery: the entries of f replace the focus of c in the thread index, and f
// is stored in the session of c for resume. The focus handler has checked every entry.
func (h *Hub) Focus(c *Connection, f domain.Focus) {
	h.mu.Lock()
	m := h.members[c]
	if m != nil {
		h.setFocusLocked(m, f.Entries)
	}
	h.mu.Unlock()
	if m != nil {
		m.s.setFocus(f)
	}
}

// setFocusLocked replaces the focus of m with entries in the thread index; h.mu is held. The
// coalescer learns the threads that are new (they get the whole document once, M2-10 design note
// 10) and the ones that left (their live-box state goes).
func (h *Hub) setFocusLocked(m *member, entries []domain.FocusEntry) {
	old := map[ids.ID]bool{}
	for _, e := range m.focus {
		old[e.ThreadID] = true
		removeFrom(h.byThread, e.ThreadID, m)
	}
	m.focus = slices.Clone(entries)
	m.focusGen++
	var added []ids.ID
	for _, e := range m.focus {
		addTo(h.byThread, e.ThreadID, m)
		if !old[e.ThreadID] {
			added = append(added, e.ThreadID)
		}
		delete(old, e.ThreadID)
	}
	if h.livebox != nil && (len(added) > 0 || len(old) > 0) {
		removed := make([]ids.ID, 0, len(old))
		for t := range old {
			removed = append(removed, t)
		}
		h.livebox.focusChanged(m, added, removed)
	}
}

// focusing returns the connections that focus thread now.
func (h *Hub) focusing(thread ids.ID) []*member {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]*member, 0, len(h.byThread[thread]))
	for m := range h.byThread[thread] {
		out = append(out, m)
	}
	return out
}

// focusEntry returns the focus entry of thread of m; false when m no longer focuses it or left
// the hub.
func (h *Hub) focusEntry(m *member, thread ids.ID) (domain.FocusEntry, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.members[m.c] != m {
		return domain.FocusEntry{}, false
	}
	for _, e := range m.focus {
		if e.ThreadID == thread {
			return e, true
		}
	}
	return domain.FocusEntry{}, false
}

// restoreFocus restores the stored focus of the session s on the new connection c (resume): the
// entries enter the index of c, then each thread is re-checked like on pruning; the readable ones
// stay, and the session keeps them with its idle flag (04 section 6.3 step 7).
func (h *Hub) restoreFocus(ctx context.Context, c *Connection, s *Session) {
	f := s.focusState()
	if len(f.Entries) == 0 {
		return
	}
	h.mu.Lock()
	m := h.members[c]
	if m != nil && len(m.focus) == 0 {
		h.setFocusLocked(m, f.Entries)
	}
	h.mu.Unlock()
	if m == nil {
		return
	}
	h.recheckFocus(ctx, m, nil, nil)
	if !h.registered(m) {
		return
	}
	h.mu.Lock()
	kept := slices.Clone(m.focus)
	h.mu.Unlock()
	s.setFocus(domain.Focus{Entries: kept, Idle: f.Idle})
}

// registered reports whether m is still the member of its connection.
func (h *Hub) registered(m *member) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.members[m.c] == m
}

// recheckFocus re-checks the focused threads of m that affected selects (all when nil) with
// gateway.focus in delivery mode and drops the unreadable ones; a kept entry takes the channel and
// space of moved, the thread.moved that triggered the check. A failed check drops the thread too
// (fail closed: the client focuses it again). A focus op that replaced the set meanwhile wins.
func (h *Hub) recheckFocus(ctx context.Context, m *member, affected func(domain.FocusEntry) bool, moved *sync.Event) {
	h.mu.Lock()
	entries, gen := slices.Clone(m.focus), m.focusGen
	h.mu.Unlock()
	if len(entries) == 0 {
		return
	}
	_, a := m.c.current()
	kept := make([]domain.FocusEntry, 0, len(entries))
	for _, e := range entries {
		if affected != nil && !affected(e) {
			kept = append(kept, e)
			continue
		}
		if !h.mayFocus(ctx, a, e) {
			continue
		}
		if moved != nil {
			e.ChannelID, e.SpaceID = moved.ChannelID, moved.SpaceID
		}
		kept = append(kept, e)
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.members[m.c] == m && m.focusGen == gen {
		h.setFocusLocked(m, kept)
	}
}

// mayFocus decides gateway.focus on the thread of e for a in delivery mode (no denial audit).
func (h *Hub) mayFocus(ctx context.Context, a actor.Actor, e domain.FocusEntry) bool {
	d, err := h.d.Authorizer.Authorize(authorize.Delivery(ctx), a, authorize.ActionGatewayFocus,
		authorize.Resource{Kind: authorize.ResourceThread, ID: e.ThreadID})
	return err == nil && d.Allowed()
}

// structureChanged re-checks, before ev is dispatched, the connections that focus a thread ev
// affects: the moved thread of thread.moved, every thread of the channel of channel.archived and
// of the space of space.archived (brief M1-66, "Focus pruning").
func (h *Hub) structureChanged(ctx context.Context, ev sync.Event) {
	affected, moved := affectedBy(ev)
	h.mu.Lock()
	var ms []*member
	for _, m := range h.members {
		if slices.ContainsFunc(m.focus, affected) {
			ms = append(ms, m)
		}
	}
	h.mu.Unlock()
	for _, m := range ms {
		h.recheckFocus(ctx, m, affected, moved)
	}
}

// affectedBy returns the selector of the focus entries ev affects and, for thread.moved, the
// event whose scope is the thread's new place.
func affectedBy(ev sync.Event) (func(domain.FocusEntry) bool, *sync.Event) {
	switch ev.Type {
	case eventThreadMoved:
		thread := ev.ThreadID
		if thread.IsZero() {
			thread = ev.ResourceID
		}
		return func(e domain.FocusEntry) bool { return e.ThreadID == thread }, &ev
	case eventChannelArchived:
		channel := ev.ChannelID
		if channel.IsZero() {
			channel = ev.ResourceID
		}
		return func(e domain.FocusEntry) bool { return e.ChannelID == channel }, nil
	}
	space := ev.SpaceID
	if space.IsZero() {
		space = ev.ResourceID
	}
	return func(e domain.FocusEntry) bool { return e.SpaceID == space }, nil
}
