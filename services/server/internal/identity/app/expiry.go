// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
)

// expiryBatch is the number of rows one read of an expiry takes; a run reads again until a read
// returns fewer, and each row ends in its own unit of work.
const expiryBatch = 200

// ExpirePendingDevices revokes every pending device older than auth.device_pending_ttl with
// pending_expired, each in its own unit of work audited as the system action device.expire
// without actor (job identity.auth_cleanup, 08 section 5.2). It returns how many it revoked.
// Authentication treats such a device as revoked already, so correctness never waits for this
// run; a repeated run is harmless. Errors: the first failing read or unit of work.
func (s *Service) ExpirePendingDevices(ctx context.Context) (int64, error) {
	ctx = system(ctx)
	var n int64
	for {
		due, err := s.d.Devices.PendingDevicesCreatedBefore(ctx, s.d.Clock.Now().Add(-s.cfg.DevicePendingTTL), expiryBatch)
		if err != nil {
			return n, err
		}
		for _, d := range due {
			if err := s.expirePendingDevice(ctx, d.ID, d.UserID); err != nil {
				return n, err
			}
			n++
		}
		if len(due) < expiryBatch {
			return n, nil
		}
	}
}

// ExpirePairings expires every requested or approved pairing past expires_at, each in its own
// unit of work; the device of an approved pairing that was never collected is revoked with
// pairing_expired and audited as device.expire (08 section 5.3). It returns how many pairings it
// read as due. The pairing endpoints check expires_at themselves, so correctness never waits for
// this run. Errors: the first failing read or unit of work.
func (s *Service) ExpirePairings(ctx context.Context) (int64, error) {
	ctx = system(ctx)
	var n int64
	for {
		due, err := s.d.Pairings.DuePairings(ctx, s.d.Clock.Now(), expiryBatch)
		if err != nil {
			return n, err
		}
		for _, rec := range due {
			if err := s.expirePairing(ctx, rec); err != nil {
				return n, err
			}
			n++
		}
		if len(due) < expiryBatch {
			return n, nil
		}
	}
}
