// SPDX-License-Identifier: Apache-2.0

package httpapi_test

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
)

// passwordStepUpApp sends stepUpWithPassword from the app session of c (DPoP, no cookie, no
// X-Console-Request) for a new challenge with password and the current TOTP code of Alex.
func (w *cworld) passwordStepUpApp(c *stepClient, password string) *httpservertest.Response {
	w.t.Helper()
	id, _ := c.challenge()
	w.clk.Advance(30 * time.Second) // a TOTP code is used once
	body, _ := json.Marshal(map[string]any{"challenge_id": id, "password": password, "totp_code": w.code(w.secret)})
	return c.call(http.MethodPost, stepUpPasswordPath, string(body))
}

// TestStepUpWithPassword_AppWithoutStepUpKey (FIX-stepup-macos, 08 6.2): the Windows app has no
// step_up key (D-M4-11-2), so its session confirms with password and TOTP: a wrong password is
// 401 step_up_required and counts, the right one is 200 without a cookie, the session holds
// step_up_method password_totp, auth.step_up records the confirmation with that method, and a
// step-up action of the owner is allowed afterwards.
func TestStepUpWithPassword_AppWithoutStepUpKey(t *testing.T) {
	t.Parallel()
	w := newConsoleWorld(t)
	win := w.loginDevice(w.alex, identity.DeviceStateActive, identity.PlatformWindows, false)
	if d := w.decide(win.actor(), authorize.ActionIdentityProviderManage, instance); d.Outcome != authorize.OutcomeStepUpRequired {
		t.Fatalf("before the step-up: %+v", d)
	}
	w.want(w.passwordStepUpApp(win, "wrong-password-123"), http.StatusUnauthorized, "step_up_required")
	if n := w.failedCount(win.session); n != 1 {
		t.Fatalf("failed challenges %d, want 1", n)
	}
	r := w.passwordStepUpApp(win, testPassword)
	m := w.want(r, http.StatusOK, "")
	if m["step_up_at"] == nil || m["valid_until"] == nil {
		t.Fatalf("body %v", m)
	}
	if got := r.Header.Values("Set-Cookie"); len(got) != 0 {
		t.Fatalf("Set-Cookie %v, want none for an app session", got)
	}
	if got := w.stepMethod(win.session); got != identity.StepUpMethodPasswordTOTP {
		t.Fatalf("step_up_method %q", got)
	}
	var confirmed int
	for _, a := range w.audits("auth.step_up") {
		if a.Decision == "allow" && a.ResourceID == win.session.String() && a.Details["step"] == "confirm" {
			if a.Details["method"] != identity.StepUpMethodPasswordTOTP {
				t.Fatalf("audit %+v", a)
			}
			confirmed++
		}
	}
	if confirmed != 1 {
		t.Fatalf("confirmations in the audit: %d, want 1", confirmed)
	}
	if d := w.decide(win.actor(), authorize.ActionIdentityProviderManage, instance); d.Outcome != authorize.OutcomeAllow {
		t.Fatalf("after the step-up: %+v", d)
	}
}

// TestStepUpWithPassword_RefusedForKeyAndCLIDevices (FIX-stepup-macos): a device with an active
// step_up key keeps the device key (403 step_up_not_possible, cause step_up_key_present, no
// downgrade), and a cli device gets 403 step_up_not_possible (08 6.4); neither counts a failure.
func TestStepUpWithPassword_RefusedForKeyAndCLIDevices(t *testing.T) {
	t.Parallel()
	w := newConsoleWorld(t)
	w.wantReason(w.passwordStepUpApp(w.phone, testPassword), http.StatusForbidden, "forbidden", "step_up_not_possible")
	got := w.notPossibleAudits()
	if len(got) != 1 || got[0].Details["cause"] != "step_up_key_present" || got[0].ResourceID != w.phone.session.String() {
		t.Fatalf("audit %+v", got)
	}
	if n := w.failedCount(w.phone.session); n != 0 {
		t.Fatalf("failed challenges %d, want 0", n)
	}
	cli := w.loginDevice(w.alex, identity.DeviceStateActive, identity.PlatformCLI, false)
	body, _ := json.Marshal(map[string]any{"challenge_id": ids.New().String(), "password": testPassword, "totp_code": w.code(w.secret)})
	w.wantReason(cli.call(http.MethodPost, stepUpPasswordPath, string(body)), http.StatusForbidden, "forbidden", "step_up_not_possible")
	if got := w.stepMethod(cli.session); got != "" {
		t.Fatalf("step_up_method of the cli session %q", got)
	}
}
