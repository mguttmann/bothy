// SPDX-License-Identifier: Apache-2.0

package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/OWNER/bothy/services/server/internal/gateway/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/sync"
)

// Compile-time check: the hub is the EphemeralPublisher of the process (02 "Cross-cutting ports").
var _ eventlog.EphemeralPublisher = (*Hub)(nil)

// audienceSelf is the envelope member audience of a personal event.
const audienceSelf = "self"

// PublishEphemeral implements eventlog.EphemeralPublisher (brief M1-66, "Publisher and hub"): it
// checks ev against the register of the gateway (domain.LookupEphemeral), encodes the dispatch
// frame once, without seq and permission_version, and offers it to every candidate in phase live
// that the admission of durable events (sync.Admit with the type's receive action, delivery
// mode) admits. Candidates are the connections that focus the thread of a focus-bound type, the
// connections of the audience of a personal one, otherwise those of the scope rule of M1-65;
// typing.started skips every connection of its actor. A connection in catch-up gets nothing (no
// buffer, at most once); a full outbound queue drops the frame and never closes the connection
// (04 section 7.2 item 6). It never waits on a connection. Errors: an unregistered type, a schema
// version below 1, a payload that is no JSON object, contains a null or reaches
// sync.event_payload_max_bytes.
func (h *Hub) PublishEphemeral(ctx context.Context, ev eventlog.EphemeralEvent) error {
	if ev.Type == eventRunUpdated {
		// run.updated goes into the coalescer instead of the immediate path (M2-10 design note
		// 9); nothing changes for typing.started.
		if lb := h.liveBox(); lb != nil {
			lb.Changed(ctx, ev)
			return nil
		}
	}
	et, ok := domain.LookupEphemeral(ev.Type)
	if !ok || et.Direct {
		return fmt.Errorf("gateway: %q is no registered ephemeral type for every candidate", ev.Type)
	}
	if ev.SchemaVersion < 1 {
		return fmt.Errorf("gateway: %s needs a schema version of at least 1", ev.Type)
	}
	payload, err := h.ephemeralPayload(ev)
	if err != nil {
		return err
	}
	admit := h.admitEvent(ev, et)
	admit.Payload = payload
	entry, err := ephemeralEntry(ev, admit)
	if err != nil {
		return err
	}
	for _, m := range h.ephemeralCandidates(admit, et) {
		h.offerEphemeral(ctx, m, admit, entry)
	}
	return nil
}

// admitEvent returns the admission form of ev without its payload: type, time (now when ev has
// none), scope, resource, audience, actor and the receive action of et.
func (h *Hub) admitEvent(ev eventlog.EphemeralEvent, et domain.EphemeralType) sync.Event {
	occurred := ev.OccurredAt
	if occurred.IsZero() {
		occurred = h.d.Clock.Now()
	}
	admit := sync.Event{
		Type: ev.Type, SchemaVersion: ev.SchemaVersion, OccurredAt: occurred,
		SpaceID: ev.Scope.SpaceID, ChannelID: ev.Scope.ChannelID, ThreadID: ev.Scope.ThreadID,
		ResourceKind: ev.Resource.Kind, ResourceID: ev.Resource.ID, ResourceVersion: ev.Resource.Version,
		AudiencePrincipalID: ev.Audience, ReceiveAction: et.ReceiveAction,
	}
	if ev.Actor != nil {
		admit.ActorKind, admit.ActorPrincipalID = string(ev.Actor.Kind), ev.Actor.PrincipalID
	}
	return admit
}

// ephemeralEntry encodes the dispatch frame of admit once and returns its queue entry.
func ephemeralEntry(ev eventlog.EphemeralEvent, admit sync.Event) (domain.Entry, error) {
	frame, err := json.Marshal(outFrame{Op: opDispatch, D: ephemeralEnvelope(admit)})
	if err != nil {
		return domain.Entry{}, fmt.Errorf("gateway: %s cannot be encoded: %w", ev.Type, err)
	}
	return domain.Entry{Frame: frame, Class: domain.ClassDispatch, SpaceID: ev.Scope.SpaceID, ChannelID: ev.Scope.ChannelID,
		OccurredAt: admit.OccurredAt, Live: true, Ephemeral: true}, nil
}

// DirectOutcome is the result of OfferEphemeralTo.
type DirectOutcome int

// The outcomes of a direct offer.
const (
	// DirectSent: the frame is in the outbound queue of the connection.
	DirectSent DirectOutcome = iota
	// DirectSkipped: nothing was sent and nothing is wrong with the receiver: the connection is
	// not registered or not live (no frame before catchup.complete), its queue is full (the next
	// frame replaces this one), or the admission or the payload failed.
	DirectSkipped
	// DirectDenied: sync.Admit refused the receive action for the connection's actor (a
	// revocation the log tail applied).
	DirectDenied
)

// OfferEphemeralTo offers the direct ephemeral event ev (domain.EphemeralType.Direct, stats.live)
// to the one connection c (M6-15 design note 5, 04 2.1 and 7.2 item 6): c must be live, then
// sync.Admit with the type's receive action in delivery mode decides for the actor of c, and only
// after that payload builds the payload, which is checked like every ephemeral payload (a JSON
// object without null below sync.event_payload_max_bytes) and queued without waiting; a full
// queue drops it and never closes the connection. Errors: a type that is not direct, a schema
// version below 1, and the errors of Admit and payload (with DirectSkipped).
func (h *Hub) OfferEphemeralTo(ctx context.Context, c *Connection, ev eventlog.EphemeralEvent, payload func() (any, error)) (DirectOutcome, error) {
	et, ok := domain.LookupEphemeral(ev.Type)
	if !ok || !et.Direct || ev.SchemaVersion < 1 {
		return DirectSkipped, fmt.Errorf("gateway: %q is no direct ephemeral type with a schema version", ev.Type)
	}
	h.mu.Lock()
	m := h.members[c]
	h.mu.Unlock()
	if m == nil || !h.live(m) {
		return DirectSkipped, nil
	}
	admit := h.admitEvent(ev, et)
	_, a := c.current()
	ok, err := h.d.CatchUp.Admit(authorize.Delivery(ctx), a, admit)
	switch {
	case err != nil:
		return DirectSkipped, err
	case !ok:
		return DirectDenied, nil
	}
	if ev.Payload, err = payload(); err != nil {
		return DirectSkipped, err
	}
	if admit.Payload, err = h.ephemeralPayload(ev); err != nil {
		return DirectSkipped, err
	}
	entry, err := ephemeralEntry(ev, admit)
	if err != nil {
		return DirectSkipped, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.phase == domain.PhaseLive && m.c.conn.Offer(entry) {
		return DirectSent, nil
	}
	return DirectSkipped, nil
}

// ephemeralPayload encodes the payload of ev and checks it: a JSON object without null (ADR
// 0053) and smaller than sync.event_payload_max_bytes.
func (h *Hub) ephemeralPayload(ev eventlog.EphemeralEvent) (json.RawMessage, error) {
	b, err := json.Marshal(ev.Payload)
	switch {
	case err != nil:
		return nil, fmt.Errorf("gateway: the payload of %s cannot be encoded: %w", ev.Type, err)
	case len(b) == 0 || b[0] != '{':
		return nil, fmt.Errorf("gateway: the payload of %s is no JSON object", ev.Type)
	case len(b) >= h.cfg.PayloadMax:
		return nil, fmt.Errorf("gateway: the payload of %s reaches sync.event_payload_max_bytes", ev.Type)
	case hasNull(b):
		return nil, fmt.Errorf("gateway: the payload of %s contains null", ev.Type)
	}
	return b, nil
}

// hasNull reports whether the valid JSON value b contains a null literal outside strings.
func hasNull(b []byte) bool {
	dec := json.NewDecoder(bytes.NewReader(b))
	for {
		tok, err := dec.Token()
		if err != nil {
			return false
		}
		if tok == nil {
			return true
		}
	}
}

// ephemeralEnvelope returns the envelope of an ephemeral event: the durable form without seq and
// permission_version (04 "Two kinds"), with a fresh UUIDv7 and occurred_at in UTC with
// milliseconds.
func ephemeralEnvelope(ev sync.Event) sync.Envelope {
	env := sync.Envelope{
		ID: ids.New(), Type: ev.Type, SchemaVersion: ev.SchemaVersion, OccurredAt: ev.OccurredAt.UTC().Format(timestampMillisUTC),
		Scope:    sync.EnvelopeScope{SpaceID: ev.SpaceID, ChannelID: ev.ChannelID, ThreadID: ev.ThreadID},
		Resource: sync.EnvelopeResource{Kind: ev.ResourceKind, ID: ev.ResourceID, Version: ev.ResourceVersion},
		Payload:  ev.Payload,
	}
	if ev.ActorKind != "" {
		env.Actor = &sync.EnvelopeActor{ID: ev.ActorPrincipalID, Kind: ev.ActorKind}
	}
	if !ev.AudiencePrincipalID.IsZero() {
		env.Audience = audienceSelf
	}
	return env
}

// ephemeralCandidates returns the connections that may receive ev: the thread index for a
// focus-bound type (for a foreground-only type without the connections that hold the thread only
// in background_thread_ids), otherwise the candidates of a durable event of the same scope or
// audience; without the connections of the actor when the type excludes them.
func (h *Hub) ephemeralCandidates(ev sync.Event, et domain.EphemeralType) []*member {
	var out []*member
	if et.FocusBound {
		h.mu.Lock()
		for m := range h.byThread[ev.ThreadID] {
			if et.ForegroundOnly && !focusesInForeground(m, ev.ThreadID) {
				continue
			}
			out = append(out, m)
		}
		h.mu.Unlock()
	} else {
		out = h.candidates(ev)
	}
	if !et.ExcludeActor || ev.ActorPrincipalID.IsZero() {
		return out
	}
	kept := out[:0]
	for _, m := range out {
		if _, a := m.c.current(); a.ID != ev.ActorPrincipalID {
			kept = append(kept, m)
		}
	}
	return kept
}

// offerEphemeral offers entry to m when m is live and admitted; otherwise, and when its queue is
// full, the frame is dropped. A failed admission drops it too: the durable path closes or
// revokes the connection on its next event.
func (h *Hub) offerEphemeral(ctx context.Context, m *member, ev sync.Event, entry domain.Entry) {
	if !h.live(m) {
		return
	}
	_, a := m.c.current()
	ok, err := h.d.CatchUp.Admit(authorize.Delivery(ctx), a, ev)
	if err != nil || !ok {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.phase == domain.PhaseLive {
		_ = m.c.conn.Offer(entry)
	}
}

// live reports whether m is in phase live; an agent consumer connection only once its runner
// queued catchup.complete or resync_required (INT-W41/1).
func (h *Hub) live(m *member) bool {
	if m.consumer {
		h.mu.Lock()
		e := h.consumers
		h.mu.Unlock()
		if e == nil || !e.Streaming(m.c) {
			return false
		}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.phase == domain.PhaseLive
}

// focusesInForeground reports whether m holds thread in thread_ids, not only in
// background_thread_ids (04 6.2, R-SY-27); h.mu is held.
func focusesInForeground(m *member, thread ids.ID) bool {
	for _, e := range m.focus {
		if e.ThreadID == thread {
			return !e.Background
		}
	}
	return false
}
