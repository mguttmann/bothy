// SPDX-License-Identifier: Apache-2.0

package identity

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// PushTargets is the source port through which identity shows the push registrations of devices
// in the schema Device (member push_registrations; 02 section 4.3, M3-22 design note 11). identity
// declares it and the module notify implements it, so identity never imports notify; internal/wire
// binds it (the pattern of AvatarFiles). It never authorizes: identity calls it only for devices
// it already authorized, and it never returns a token or a hash.
//
// Example:
//
//	regs, err := targets.ForDevices(ctx, []ids.ID{phone, tablet})
//	for _, r := range regs[phone] { ... }
type PushTargets interface {
	// ForDevices returns every registration of the given devices, active and disabled, grouped by
	// device, each list ascending by id, in one read. Devices without one are absent; no devices
	// give an empty map. Errors: those of the read, as kernel/errs errors.
	ForDevices(ctx context.Context, deviceIDs []ids.ID) (map[ids.ID][]PushTargetView, error)
}

// PushTargetView is one push registration as the schema PushRegistration shows it: never the
// device token, never the preview token or its hash. Zero values are absent members.
type PushTargetView struct {
	// ID is push_registrations.id; DeviceID its device.
	ID, DeviceID ids.ID
	// Provider is apns, fcm, unifiedpush or wns; Environment production or sandbox; AppID the
	// bundle or package of the app.
	Provider, Environment, AppID string
	// CreatedAt is created_at; DisabledAt is disabled_at, zero while the registration is active.
	CreatedAt, DisabledAt time.Time
	// DisabledReason is disabled_reason, "" while the registration is active.
	DisabledReason string
}
