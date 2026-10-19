// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"encoding/json"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/terminal"
)

// emitUpdated appends host.updated for v (05 section 6, 24 section 10.2): durable, resource host
// with its new version, empty scope (the hub treats it as instance level), no audience, receive
// action host.read, the actor of the request, payload HostUpdatedPayload with the host without
// noise_public_key; key_changed stays absent.
func (s *Service) emitUpdated(ctx context.Context, v terminal.HostView) error {
	return s.emitHostUpdated(ctx, v, false)
}

// emitHostUpdated appends host.updated for v like emitUpdated; keyChanged sets key_changed = true,
// exactly when the stored keys were dropped (M2-41: re-pairing and the fallback of TM-T19).
func (s *Service) emitHostUpdated(ctx context.Context, v terminal.HostView, keyChanged bool) error {
	payload, err := json.Marshal(struct {
		Host       hostJSON `json:"host"`
		KeyChanged bool     `json:"key_changed,omitempty"`
	}{eventHostBody(v), keyChanged})
	if err != nil {
		return errs.Wrap(errs.CodeInternalError, err, "terminal: host.updated payload")
	}
	kind, id := eventActor(ctx)
	return s.d.Events.Append(ctx, eventlog.Event{
		Type: eventHostUpdated, SchemaVersion: 1, ActorKind: kind, ActorPrincipalID: id,
		ResourceKind: resourceHost, ResourceID: v.ID, ResourceVersion: v.Version, ReceiveAction: receiveHostRead, Payload: payload,
	})
}

// eventActor returns the actor of the request in ctx; only people reach host.manage.
func eventActor(ctx context.Context) (eventlog.ActorKind, ids.ID) {
	if a, ok := actor.FromContext(ctx); ok && a.Kind == actor.KindUser && !a.ID.IsZero() {
		return eventlog.ActorUser, a.ID
	}
	return "", ids.ID{}
}
