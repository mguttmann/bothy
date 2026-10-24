// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"context"
	"sync/atomic"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// devicePush fills the member push_registrations of the devices a handler returns through the
// source port identity.PushTargets (M3-22 design note 11, docs/DECISIONS.md D-M3-22-1). internal/wire
// binds the port once notify is built; until then, and in tests that bind none, the member stays
// absent. It is safe for concurrent use.
type devicePush struct {
	p atomic.Pointer[identity.PushTargets]
}

// BindPushTargets sets the source of the push registrations of devices; a second call replaces
// the first.
func (d *devicePush) BindPushTargets(p identity.PushTargets) { d.p.Store(&p) }

// fill reads the registrations of devs in one call and sets their member push_registrations:
// absent for a device without one (ADR 0053), disabled_at and disabled_reason only for a disabled
// registration. The caller already authorized every device, so no authorize runs here.
func (d *devicePush) fill(ctx context.Context, devs []rest.Device) error {
	src := d.p.Load()
	if src == nil || *src == nil || len(devs) == 0 {
		return nil
	}
	idList := make([]ids.ID, 0, len(devs))
	for _, dv := range devs {
		idList = append(idList, ids.ID(dv.Id))
	}
	regs, err := (*src).ForDevices(ctx, idList)
	if err != nil {
		return err
	}
	for i := range devs {
		dv := &devs[i]
		list := regs[ids.ID(dv.Id)]
		if len(list) == 0 {
			continue
		}
		out := make([]rest.PushRegistration, 0, len(list))
		for _, r := range list {
			out = append(out, rest.PushRegistration{
				Id: rest.Uuid(r.ID), DeviceId: rest.Uuid(r.DeviceID), Provider: rest.PushProvider(r.Provider),
				Environment: rest.PushEnvironment(r.Environment), AppId: r.AppID, CreatedAt: ms(r.CreatedAt),
				DisabledAt: optionalTime(r.DisabledAt), DisabledReason: optional(r.DisabledReason),
			})
		}
		dv.PushRegistrations = &out
	}
	return nil
}

// one fills a single device.
func (d *devicePush) one(ctx context.Context, dv rest.Device) (rest.Device, error) {
	devs := []rest.Device{dv}
	if err := d.fill(ctx, devs); err != nil {
		return rest.Device{}, err
	}
	return devs[0], nil
}
