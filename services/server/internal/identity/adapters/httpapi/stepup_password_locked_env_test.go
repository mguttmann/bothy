// SPDX-License-Identifier: Apache-2.0

package httpapi_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/identity"
)

// TestStepUpWithPassword_LockedAccountDoesNotCount (ID-T31; 08 6.2 and 13, R-ID-10,
// SEC-STEPUP-LOCKOUT): while the password path of the account is locked, a step-up with the
// right password and code from a Windows app session is refused with 401 step_up_required and
// reason password_locked that counts nothing: no failed challenge, the session is not revoked
// after auth.step_up_fail_limit attempts, the refusal is audited with cause locked, and the
// step-up succeeds once the lockout is over.
func TestStepUpWithPassword_LockedAccountDoesNotCount(t *testing.T) {
	t.Parallel()
	w := newConsoleWorld(t)
	win := w.loginDevice(w.alex, identity.DeviceStateActive, identity.PlatformWindows, false)
	attempts := sessCfg.StepUpFailLimit + 1
	// Each attempt advances the clock by one TOTP step; the lock ends ten seconds after the last.
	w.sql(`UPDATE local_credentials SET locked_until = $2 WHERE identity_id IN (SELECT id FROM user_identities WHERE user_id = $1)`,
		w.alex.String(), w.clk.Now().Add(time.Duration(attempts)*30*time.Second+10*time.Second))
	for range attempts {
		w.wantReason(w.passwordStepUpApp(win, testPassword), http.StatusUnauthorized, "step_up_required", "password_locked")
	}
	if n := w.failedCount(win.session); n != 0 {
		t.Fatalf("failed challenges while locked = %d, want 0", n)
	}
	var locked int
	for _, a := range w.audits("auth.step_up") {
		if a.Decision == "deny" && a.ResourceID == win.session.String() && a.Details["cause"] == "locked" {
			locked++
		}
	}
	// Equal denials of one session are aggregated into one row (SEC-AUD-6).
	if locked == 0 {
		t.Fatal("no audited refusal with cause locked")
	}
	w.want(w.passwordStepUpApp(win, testPassword), http.StatusOK, "")
	if got := w.stepMethod(win.session); got != identity.StepUpMethodPasswordTOTP {
		t.Fatalf("step_up_method %q", got)
	}
}
