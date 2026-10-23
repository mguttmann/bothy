// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// PushTargetSource is the part of RegistrationWriter that PushTargets reads.
type PushTargetSource interface {
	OfDevices(ctx context.Context, deviceIDs []ids.ID) (map[ids.ID][]identity.PushTargetView, error)
}

// PushTargets implements the source port identity.PushTargets that identity declares, so the
// schema Device lists the push registrations of a device without identity importing notify (02
// section 4.3, brief M3-22 design note 11). It never authorizes: identity calls it only for
// devices it authorized. It never returns a token or a hash.
type PushTargets struct {
	store PushTargetSource
}

// Compile-time check: PushTargets is identity's source port.
var _ identity.PushTargets = PushTargets{}

// NewPushTargets returns the source over store.
//
// Example:
//
//	people.BindPushTargets(app.NewPushTargets(postgres.NewRegistrations(pool)))
func NewPushTargets(store PushTargetSource) PushTargets { return PushTargets{store: store} }

// ForDevices implements identity.PushTargets with one read (ListRegistrationsOfDevices).
func (p PushTargets) ForDevices(ctx context.Context, deviceIDs []ids.ID) (map[ids.ID][]identity.PushTargetView, error) {
	return p.store.OfDevices(ctx, deviceIDs)
}
