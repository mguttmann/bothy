// SPDX-License-Identifier: Apache-2.0

// Package httpapi holds the REST handlers of the identity module (adapters/httpapi, 02 "5.
// Schichten im Modul"): the operations listMyDevices, listUserDevices and getDevice (M1-33), the
// people, owner and self operations (People, M1-31) of the generated strict server, the cursor codec adapter and ExpectedHTU, the htu every DPoP check of
// a REST request expects (M1-34 and the login handlers use it).
//
// Example:
//
//	h := httpapi.NewDevices(devices) // embedded into wire.Handlers
//	htu := httpapi.ExpectedHTU(publicURL, r)
package httpapi

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Devices implements the device operations of rest.StrictServerInterface over identity.Devices.
// The use cases authorize; the handler only converts and adds the push registrations of the
// devices through identity.PushTargets (BindPushTargets, M3-22). Errors go to the problem writer
// of the chain unchanged.
type Devices struct {
	devicePush
	devices identity.Devices
}

// NewDevices returns the handlers over the device port.
func NewDevices(devices identity.Devices) *Devices {
	return &Devices{devices: devices}
}

// principal returns the actor of the request; outside the chain it is the anonymous principal,
// which authorize refuses.
func principal(ctx context.Context) actor.Actor {
	if a, ok := actor.FromContext(ctx); ok {
		return a
	}
	return actor.Anonymous()
}

// GetDevice implements rest.StrictServerInterface (GET /devices/{device_id}).
func (h *Devices) GetDevice(ctx context.Context, req rest.GetDeviceRequestObject) (rest.GetDeviceResponseObject, error) {
	d, err := h.devices.Get(ctx, principal(ctx), ids.ID(req.DeviceId))
	if err != nil {
		return nil, err
	}
	body, err := h.one(ctx, toREST(d))
	if err != nil {
		return nil, err
	}
	return rest.GetDevice200JSONResponse{Body: body}, nil
}

// ListMyDevices implements rest.StrictServerInterface (GET /me/devices).
func (h *Devices) ListMyDevices(ctx context.Context, req rest.ListMyDevicesRequestObject) (rest.ListMyDevicesResponseObject, error) {
	p, err := h.devices.ListMine(ctx, principal(ctx), pageOf(req.Params.Cursor, req.Params.Limit))
	if err != nil {
		return nil, err
	}
	body := toRESTPage(p)
	if err := h.fill(ctx, body.Items); err != nil {
		return nil, err
	}
	return rest.ListMyDevices200JSONResponse{Body: body}, nil
}

// ListUserDevices implements rest.StrictServerInterface (GET /users/{user_id}/devices).
func (h *Devices) ListUserDevices(ctx context.Context, req rest.ListUserDevicesRequestObject) (rest.ListUserDevicesResponseObject, error) {
	p, err := h.devices.ListForUser(ctx, principal(ctx), ids.ID(req.UserId), pageOf(req.Params.Cursor, req.Params.Limit))
	if err != nil {
		return nil, err
	}
	body := toRESTPage(p)
	if err := h.fill(ctx, body.Items); err != nil {
		return nil, err
	}
	return rest.ListUserDevices200JSONResponse{Body: body}, nil
}

func pageOf(cursor *rest.Cursor, limit *rest.Limit) identity.Page {
	var p identity.Page
	if cursor != nil {
		p.Cursor = *cursor
	}
	if limit != nil {
		p.Limit = *limit
	}
	return p
}

func toRESTPage(p identity.DevicePage) rest.DevicePage {
	out := rest.DevicePage{Items: make([]rest.Device, 0, len(p.Items))}
	for _, d := range p.Items {
		out.Items = append(out.Items, toREST(d))
	}
	if p.NextCursor != "" {
		out.NextCursor = &p.NextCursor
	}
	return out
}

// toREST converts a device; empty values become absent members, never null (ADR 0053), and
// times are UTC with millisecond precision.
func toREST(d identity.Device) rest.Device {
	return rest.Device{
		Id: rest.Uuid(d.ID), UserId: rest.Uuid(d.UserID), Name: d.Name,
		Platform: rest.DevicePlatform(d.Platform), State: rest.DeviceState(d.State), CreatedAt: ms(d.CreatedAt),
		AppVersion: optional(d.AppVersion), KeyThumbprint: optional(d.KeyThumbprint),
		StepUpKeyThumbprint: optional(d.StepUpKeyThumbprint),
		RevokeReason:        (*rest.DeviceRevokeReason)(optional(d.RevokeReason)),
		ApprovedAt:          optionalTime(d.ApprovedAt), RevokedAt: optionalTime(d.RevokedAt), LastSeenAt: optionalTime(d.LastSeenAt),
		VerificationCode: optional(d.VerificationCode),
	}
}

func optional(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func ms(t time.Time) time.Time { return t.UTC().Truncate(time.Millisecond) }

func optionalTime(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	v := ms(t)
	return &v
}
