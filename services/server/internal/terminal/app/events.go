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
// noise_public_key; key_changed stays absent (only re-pairing of M2-41 sets it).
func (s *Service) emitUpdated(ctx context.Context, v terminal.HostView) error {
	payload, err := json.Marshal(struct {
		Host hostJSON `json:"host"`
	}{eventHostBody(v)})
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
