// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"testing"

	"github.com/OWNER/bothy/services/server/internal/identity/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// TestSessionRevokeReasonFor: the person of the session ends it with logout, anybody else with
// admin (03 auth_sessions.revoke_reason, brief M1-45 ruling 2).
func TestSessionRevokeReasonFor(t *testing.T) {
	t.Parallel()
	alex, sam := ids.New(), ids.New()
	for _, c := range []struct {
		actor, user ids.ID
		want        domain.SessionRevokeReason
	}{
		{alex, alex, domain.SessionRevokeLogout},
		{sam, alex, domain.SessionRevokeAdmin},
		{ids.ID{}, alex, domain.SessionRevokeAdmin},
	} {
		if got := domain.SessionRevokeReasonFor(c.actor, c.user); got != c.want {
			t.Errorf("SessionRevokeReasonFor(%v, %v) = %q, want %q", c.actor, c.user, got, c.want)
		}
	}
}

// TestDeviceRevokeReasonFor: the person of the device revokes it with revoked_by_self, anybody
// else with revoked_by_owner (03 devices.revoke_reason).
func TestDeviceRevokeReasonFor(t *testing.T) {
	t.Parallel()
	alex, owner := ids.New(), ids.New()
	if got := domain.DeviceRevokeReasonFor(alex, alex); got != domain.DeviceRevokedBySelf {
		t.Errorf("own device: %q", got)
	}
	if got := domain.DeviceRevokeReasonFor(owner, alex); got != domain.DeviceRevokedByOwner {
		t.Errorf("foreign device: %q", got)
	}
}

// TestDisableTransition: active and pending become disabled, disabled stays (no change), deleted
// is invalid_state_transition (08 section 11, 06 section 2 "Ohne Wirkung").
func TestDisableTransition(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		from   domain.Status
		change bool
		code   errs.Code
	}{
		{domain.StatusActive, true, ""},
		{domain.StatusPending, true, ""},
		{domain.StatusDisabled, false, ""},
		{domain.StatusDeleted, false, errs.CodeInvalidStateTransition},
		{domain.Status("unknown"), false, errs.CodeInvalidStateTransition},
	} {
		change, err := domain.DisableTransition(c.from)
		if change != c.change {
			t.Errorf("DisableTransition(%q) change = %v, want %v", c.from, change, c.change)
		}
		if c.code == "" {
			if err != nil {
				t.Errorf("DisableTransition(%q) = %v", c.from, err)
			}
			continue
		}
		if e, ok := errs.As(err); !ok || e.Code != c.code {
			t.Errorf("DisableTransition(%q) error = %v, want %s", c.from, err, c.code)
		}
	}
}

// TestDeviceRevokeTransition: pending and active become revoked, revoked stays (08 section 10).
func TestDeviceRevokeTransition(t *testing.T) {
	t.Parallel()
	for state, want := range map[string]bool{"pending": true, "active": true, "revoked": false} {
		if got := domain.DeviceRevokeTransition(state); got != want {
			t.Errorf("DeviceRevokeTransition(%q) = %v, want %v", state, got, want)
		}
	}
}

// TestValidReasons: the reason codes the use cases store are exactly the values the CHECK
// constraints of 03 allow for them.
func TestValidReasons(t *testing.T) {
	t.Parallel()
	for _, r := range []domain.SessionRevokeReason{domain.SessionRevokeLogout, domain.SessionRevokeAdmin,
		domain.SessionRevokeDeviceRevoked, domain.SessionRevokeUserDisabled, domain.SessionRevokeInactive} {
		if !r.Valid() {
			t.Errorf("%q not valid", r)
		}
	}
	for _, r := range []domain.DeviceRevokeReason{domain.DeviceRevokedBySelf, domain.DeviceRevokedByOwner,
		domain.DeviceRevokeUserDisabled, domain.DeviceRevokeInactive} {
		if !r.Valid() {
			t.Errorf("%q not valid", r)
		}
	}
	if domain.SessionRevokeReason("lost").Valid() || domain.DeviceRevokeReason("lost").Valid() {
		t.Error("an unknown reason is valid")
	}
}
