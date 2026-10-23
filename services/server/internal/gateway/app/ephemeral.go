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
	if !ok {
		return fmt.Errorf("gateway: %q is no registered ephemeral type", ev.Type)
	}
	if ev.SchemaVersion < 1 {
		return fmt.Errorf("gateway: %s needs a schema version of at least 1", ev.Type)
	}
	payload, err := h.ephemeralPayload(ev)
	if err != nil {
		return err
	}
	occurred := ev.OccurredAt
	if occurred.IsZero() {
		occurred = h.d.Clock.Now()
	}
	admit := sync.Event{
		Type: ev.Type, SchemaVersion: ev.SchemaVersion, OccurredAt: occurred,
		SpaceID: ev.Scope.SpaceID, ChannelID: ev.Scope.ChannelID, ThreadID: ev.Scope.ThreadID,
		ResourceKind: ev.Resource.Kind, ResourceID: ev.Resource.ID, ResourceVersion: ev.Resource.Version,
		AudiencePrincipalID: ev.Audience, ReceiveAction: et.ReceiveAction, Payload: payload,
	}
	if ev.Actor != nil {
		admit.ActorKind, admit.ActorPrincipalID = string(ev.Actor.Kind), ev.Actor.PrincipalID
	}
	frame, err := json.Marshal(outFrame{Op: opDispatch, D: ephemeralEnvelope(admit)})
	if err != nil {
		return fmt.Errorf("gateway: %s cannot be encoded: %w", ev.Type, err)
	}
	entry := domain.Entry{Frame: frame, Class: domain.ClassDispatch, SpaceID: ev.Scope.SpaceID, ChannelID: ev.Scope.ChannelID, OccurredAt: occurred,
		Live: true, Ephemeral: true}
	for _, m := range h.ephemeralCandidates(admit, et) {
		h.offerEphemeral(ctx, m, admit, entry)
	}
	return nil
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
// focus-bound type, otherwise the candidates of a durable event of the same scope or audience;
// without the connections of the actor when the type excludes them.
func (h *Hub) ephemeralCandidates(ev sync.Event, et domain.EphemeralType) []*member {
	var out []*member
	if et.FocusBound {
		h.mu.Lock()
		for m := range h.byThread[ev.ThreadID] {
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
