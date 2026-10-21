// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/OWNER/bothy/services/server/internal/gateway/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/sync"
)

// accessChanged prepares the connections of the audience of an access.changed before the event
// itself is dispatched (04 section 8.3, 07 section 8.2 item 3), in this order: (a) each credential
// is revalidated and gateway.connect decided again in delivery mode, an unauthenticated answer
// closes with 4012 (08 "Lost device", "Disabling a person"); (b) the visible sets are read again
// for the indices; (c) the queued and buffered dispatches of the removed targets are dropped;
// (d) every focused thread is re-checked and the unreadable ones leave the focus (04 section 8.3
// item 2). The rights cache already saw the event (Observe), so every later check uses the new
// state.
func (h *Hub) accessChanged(ctx context.Context, ev sync.Event) {
	removed := domain.NewTargets(nil, nil)
	// A payload that cannot be read removes nothing here; the admission of later events still
	// applies the new rights. The removal covers only the events before the access.changed: the
	// catch-up pages can lie ahead of the tail and read the later events with newer rights, a
	// re-grant included (M1-65/15).
	if diff, err := authorize.ParseAccessChanged(ev.Payload); err == nil {
		removed = removedTargets(diff.Removed).Before(ev.Seq)
	}
	for _, m := range h.membersOf(ev.AudiencePrincipalID) {
		if !h.revalidate(ctx, m) {
			continue
		}
		h.reindex(ctx, m)
		h.purge(m, removed)
		h.recheckFocus(ctx, m, nil, nil)
	}
}

// removedTargets returns the spaces and channels of a list of access.changed; hosts carry no
// events of the gateway.
func removedTargets(list []authorize.AccessTarget) domain.Targets {
	var spaces, channels []ids.ID
	for _, t := range list {
		switch t.Kind {
		case authorize.AccessSpace:
			spaces = append(spaces, t.ID)
		case authorize.AccessChannel:
			channels = append(channels, t.ID)
		case authorize.AccessHost:
		}
	}
	return domain.NewTargets(spaces, channels)
}

// revalidate is step (a): identity reads the credential again (session, device, token) and authz
// decides gateway.connect with the new rights; an unauthenticated answer of either closes m with
// 4012, a forbidden gateway.connect closes like at identify (4016 or 4018, docs/DECISIONS.md
// M1-65/18). Another error keeps the connection (logged): the admission of every later event
// still fails closed. It reports whether m stays open.
func (h *Hub) revalidate(ctx context.Context, m *member) bool {
	_, a := m.c.current()
	err := h.d.Authenticator.Revalidate(ctx, a)
	if err == nil {
		var d authorize.Decision
		d, err = h.d.Authorizer.Authorize(authorize.Delivery(ctx), a, authorize.ActionGatewayConnect, authorize.Resource{Kind: authorize.ResourceInstance})
		if err == nil && d.Outcome == authorize.OutcomeUnauthenticated {
			err = d.Err()
		}
		if err == nil && !d.Allowed() {
			// A forbidden gateway.connect closes like at identify (authorizeConnect).
			h.closeMember(m, forbiddenClose(d))
			return false
		}
	}
	switch {
	case err == nil:
		return true
	case errs.KindOf(err) == errs.KindUnauthenticated:
		h.closeMember(m, domain.CloseRevoked)
		return false
	}
	h.d.Log.LogAttrs(ctx, slog.LevelWarn, msgRevalidateFailed, slog.String(logKeyErrorType, errorTypeRevalidate))
	return true
}

// forbiddenClose is the close code of a gateway.connect that is neither allowed nor
// unauthenticated, as authorizeConnect maps it: 4016 for a pending device or principal, else 4018.
func forbiddenClose(d authorize.Decision) domain.CloseCode {
	if d.Outcome == authorize.OutcomeForbidden && (d.Reason == "device_pending" || d.Reason == "principal_pending") {
		return domain.CloseDevicePending
	}
	return domain.CloseForbidden
}

// reindex is step (b): the visible sets of m are read again in delivery mode and replace its index
// entries. A failure makes m a candidate of every event until its next access.changed.
func (h *Hub) reindex(ctx context.Context, m *member) {
	h.mu.Lock()
	m.scopeGen++
	gen := m.scopeGen
	h.mu.Unlock()
	_, a := m.c.current()
	sc, err := h.d.Authorizer.Scope(authorize.Delivery(ctx), a, authorize.ActionChannelRead)
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.members[m.c] != m || m.scopeGen != gen {
		return
	}
	if err != nil {
		h.unindexLocked(m)
		m.pending = true
		h.pending[m] = struct{}{}
		h.d.Log.LogAttrs(ctx, slog.LevelWarn, msgScopeFailed, slog.String(logKeyErrorType, errorTypeScope))
		return
	}
	h.setIndexLocked(m, sc)
}

// purge is step (c): the dispatches of the removed targets leave the live buffer and the outbound
// queue of m; while m is not live the targets are remembered, so the catch-up runner and the flush
// drop them too (04 section 8.3 items 1 and 2).
func (h *Hub) purge(m *member, removed domain.Targets) {
	if removed.Empty() {
		return
	}
	m.mu.Lock()
	if m.phase == domain.PhaseCatchingUp {
		m.removed = append(m.removed, removed)
		kept := m.buffer[:0]
		for _, ev := range m.buffer {
			if !removed.Covers(ev.SpaceID, ev.ChannelID, ev.Seq) {
				kept = append(kept, ev)
			}
		}
		clear(m.buffer[len(kept):])
		m.buffer = kept
	}
	m.mu.Unlock()
	m.c.conn.Purge(removed)
	if m.consumer {
		h.consumers.Purge(m.c, removed)
	}
}

// sessionRevokedPayload is the part of the payload of session.revoked the hub reads.
type sessionRevokedPayload struct {
	SessionID ids.ID `json:"session_id"`
}

// sessionRevoked closes every connection of the audience whose credential carries the revoked
// session with 4012 (04 section 8.5); the event itself then reaches the other connections of the
// person whose credentials are still valid.
func (h *Hub) sessionRevoked(ev sync.Event) {
	var p sessionRevokedPayload
	if err := json.Unmarshal(ev.Payload, &p); err != nil || p.SessionID.IsZero() {
		return
	}
	for _, m := range h.membersOf(ev.AudiencePrincipalID) {
		if _, a := m.c.current(); a.SessionID == p.SessionID {
			h.closeMember(m, domain.CloseRevoked)
		}
	}
}
