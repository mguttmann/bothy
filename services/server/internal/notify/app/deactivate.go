// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/notify/domain"
)

// DeactivationStore is the part of RegistrationWriter the deactivation subscribers write.
type DeactivationStore interface {
	DisableOfSessions(ctx context.Context, sessionIDs []ids.ID, reason string, at time.Time) error
	DisableOfDevice(ctx context.Context, deviceID ids.ID, reason string, at time.Time) error
}

// WithRegistrations sets the store of the deactivation subscribers OnSessionEnded and
// OnDeviceRevoked (M3-22); internal/wire passes adapters/postgres. It returns s.
func (s *Subscribers) WithRegistrations(store DeactivationStore) *Subscribers {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.regs = store
	return s
}

// registrations returns the deactivation store; an internal error before WithRegistrations, so a
// revocation never commits with its registrations still active.
func (s *Subscribers) registrations() (DeactivationStore, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.regs == nil {
		return nil, errs.New(errs.CodeInternalError, "notify: the deactivation subscribers have no registration store")
	}
	return s.regs, nil
}

// OnSessionEnded runs in the transaction that ends sessions (auth.logout, session.revoke, refresh
// reuse, device.revoke, user.disable; M1-34, M1-45): it disables the active push registrations of
// the ended sessions with domain.DisableReasonForSession of the session reason (13 6.4 and 7.3,
// brief M3-22 design note 7 a, docs/DECISIONS.md D-M3-22-7). A row that is already disabled keeps
// its first reason. A veto later in the unit removes the write with everything else.
func (s *Subscribers) OnSessionEnded(ctx context.Context, ev identity.SessionEnded) error {
	store, err := s.registrations()
	if err != nil {
		return err
	}
	return store.DisableOfSessions(ctx, ev.SessionIDs, domain.DisableReasonForSession(string(ev.Reason)), s.now(ev.At))
}

// OnDeviceRevoked runs in the transaction that revokes a device (device.revoke, user.disable,
// pending expiry; M1-38, M1-45): it disables every active registration of the device with
// device_revoked, whatever its session (13 7.3 "all registrations device_revoked, immediately",
// brief M3-22 design note 7 b). No push is sent for the revocation (08 10 step 4).
func (s *Subscribers) OnDeviceRevoked(ctx context.Context, ev identity.DeviceRevoked) error {
	store, err := s.registrations()
	if err != nil {
		return err
	}
	return store.DisableOfDevice(ctx, ev.DeviceID, domain.DisabledDeviceRevoked, s.now(ev.At))
}

// now returns at, the time of the revocation the event carries, or the clock time when the event
// has none.
func (s *Subscribers) now(at time.Time) time.Time {
	if at.IsZero() {
		return s.clock.Now().UTC().Truncate(time.Microsecond)
	}
	return at
}
