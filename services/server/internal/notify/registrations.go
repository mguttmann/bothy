// SPDX-License-Identifier: Apache-2.0

package notify

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// PushRegistrations reads the push targets of people (03 push_registrations, D-W-108). It does
// not call authorize: the callers select the people (the consumer push and its jobs, M3-22 for
// the own registrations of a caller). M3-22 adds the writes on the same port. The token and
// preview_token_hash never leave the adapters of notify.
//
// Example:
//
//	regs, err := registrations.Active(ctx, []ids.ID{alex, sam})
//	for _, r := range regs[alex] { ... }
type PushRegistrations interface {
	// Active returns the registrations with disabled_at IS NULL of the given people, grouped by
	// person, each list in ascending id. People without one are absent. It reads in the ambient
	// transaction of ctx when there is one.
	//
	// Errors: those of the read.
	Active(ctx context.Context, userIDs []ids.ID) (map[ids.ID][]PushRegistration, error)
}

// PushRegistration is one active push registration without its token.
type PushRegistration struct {
	// ID is push_registrations.id; UserID the person; DeviceID the device of the registration.
	ID, UserID, DeviceID ids.ID
	// Provider is apns, fcm, unifiedpush or wns; Environment production or sandbox; AppID the
	// bundle or package of the app.
	Provider, Environment, AppID string
	// CreatedAt is created_at; LastSuccessAt is last_success_at, zero when NULL.
	CreatedAt, LastSuccessAt time.Time
	// FailureCount is failure_count.
	FailureCount int
}
