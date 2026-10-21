// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"encoding/json"

	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/domain/device"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// devicePendingPayload is DevicePendingPayload of docs/api/openapi.yaml: only the IDs and the
// platform, never the name, a key or the app version, because the event reaches the other devices
// of the person as a security hint (R-AZ-31, ADR 0031).
type devicePendingPayload struct {
	DeviceID string `json:"device_id"`
	Platform string `json:"platform"`
}

// eventActorRef is the actor of a durable event: a kind and its principal, both empty for none.
type eventActorRef struct {
	kind eventlog.ActorKind
	id   ids.ID
}

// insertDevice is the one way the use cases insert a device row: it stores d through store and,
// exactly when d is pending, appends one device.pending through events in the same unit of work
// (05 section 6, R-AZ-31), so a veto later in the unit removes the device and the event together
// and every pending device has exactly one event. Every insert goes through it, so no path can
// register a pending device without the source of its security hint. Errors: those of store and
// events, and internal for a pending device without events (fail closed, nothing is inserted).
func insertDevice(ctx context.Context, store DeviceStore, events eventlog.Appender, who eventActorRef, d device.Device) (identity.Device, error) {
	pending := d.State == device.StatePending
	if pending && events == nil {
		return identity.Device{}, errs.New(errs.CodeInternalError, "identity: a pending device needs the event appender (device.pending)")
	}
	out, err := store.InsertDevice(ctx, d)
	if err != nil || !pending {
		return out, err
	}
	payload, err := json.Marshal(devicePendingPayload{DeviceID: out.ID.String(), Platform: string(out.Platform)})
	if err != nil {
		return identity.Device{}, errs.Wrap(errs.CodeInternalError, err, "identity: device.pending payload")
	}
	err = events.Append(ctx, eventlog.Event{
		Type: string(authorize.EventDevicePending), SchemaVersion: 1, ActorKind: who.kind, ActorPrincipalID: who.id,
		ResourceKind: string(authorize.ResourceDevice), ResourceID: out.ID, AudiencePrincipalID: out.UserID,
		ReceiveAction: receiveSelf, Payload: payload,
	})
	if err != nil {
		return identity.Device{}, err
	}
	return out, nil
}
