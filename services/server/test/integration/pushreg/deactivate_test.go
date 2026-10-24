// SPDX-License-Identifier: Apache-2.0

package pushreg_test

import (
	"net/http"
	"testing"

	"github.com/OWNER/bothy/services/server/test/harness"
)

// alexDevices returns a world in which Alex has his phone (the harness device, apns) and a tablet
// (a second session on a new Android device, fcm), both registered, and a message he may read.
func alexDevices(t *testing.T) (w *world, tablet *harness.Client, phoneReg, tabletReg registered, msg string) {
	t.Helper()
	w = newWorld(t, nil)
	tablet, err := w.s.UserSession(t.Context(), w.alex.PrincipalID)
	if err != nil {
		t.Fatal(err)
	}
	w.android(tablet)
	phoneReg = w.register(w.alex, "apns", apnsToken(), apnsTopic)
	tabletReg = w.register(tablet, "fcm", fcmToken(), fcmApp)
	msg = w.post(w.s.Owner, w.allgemeinMain, "hello")
	wantStatus(t, "phone before", w.fetch(w.alex, phoneReg.PreviewToken, msg), http.StatusOK, "")
	wantStatus(t, "tablet before", w.fetch(tablet, tabletReg.PreviewToken, msg), http.StatusOK, "")
	return w, tablet, phoneReg, tabletReg, msg
}

// wantDisabled fails the test unless registration id is disabled with reason.
func wantDisabled(t *testing.T, w *world, id, reason string) {
	t.Helper()
	r, ok := w.row(id)
	if !ok || !r.Disabled || r.DisabledReason != reason {
		t.Errorf("registration exists %v disabled %v reason %q, want %q", ok, r.Disabled, r.DisabledReason, reason)
	}
}

// wantActive fails the test unless registration id is active.
func wantActive(t *testing.T, w *world, id string) {
	t.Helper()
	if r, ok := w.row(id); !ok || r.Disabled {
		t.Errorf("registration exists %v disabled %v (%q), want active", ok, r.Disabled, r.DisabledReason)
	}
}

// TestLogout_PreviewTokenWorthless: PLAN row, 13 14 row "auth.logout", design note 7 a.
func TestLogout_PreviewTokenWorthless(t *testing.T) {
	w, tablet, phoneReg, tabletReg, msg := alexDevices(t)
	harness.MustDo(t, w.alex, harness.Request{Method: http.MethodPost, Path: "/auth/logout"}, http.StatusNoContent)
	wantDisabled(t, w, phoneReg.ID, "logout")
	wantActive(t, w, tabletReg.ID)
	wantStatus(t, "phone after logout", w.fetch(w.alex, phoneReg.PreviewToken, msg), http.StatusUnauthorized, "unauthenticated")
	wantStatus(t, "tablet after logout", w.fetch(tablet, tabletReg.PreviewToken, msg), http.StatusOK, "")
}

// TestDeviceRevoke_PreviewTokenWorthless: PLAN row, ID-T26 step "push registrations", 13 7.3,
// design note 7 b.
func TestDeviceRevoke_PreviewTokenWorthless(t *testing.T) {
	w, tablet, phoneReg, tabletReg, msg := alexDevices(t)
	harness.MustDo(t, tablet, harness.Request{Method: http.MethodPost, Path: "/devices/" + w.alex.DeviceID + "/revoke",
		Body: map[string]any{"reason": "lost"}}, http.StatusOK)
	wantDisabled(t, w, phoneReg.ID, "device_revoked")
	wantActive(t, w, tabletReg.ID)
	wantStatus(t, "phone after revoke", w.fetch(w.alex, phoneReg.PreviewToken, msg), http.StatusUnauthorized, "unauthenticated")
	wantStatus(t, "tablet after revoke", w.fetch(tablet, tabletReg.PreviewToken, msg), http.StatusOK, "")
}

// TestUserDisable_PreviewTokensWorthless: PLAN row, ID-T22 step 5, 08 11, design note 7 c.
func TestUserDisable_PreviewTokensWorthless(t *testing.T) {
	w, tablet, phoneReg, tabletReg, msg := alexDevices(t)
	harness.MustDo(t, w.s.Owner, harness.Request{Method: http.MethodPost, Path: "/users/" + w.alex.PrincipalID + "/disable",
		Body: map[string]any{"reason": "left"}}, http.StatusOK)
	wantDisabled(t, w, phoneReg.ID, "device_revoked")
	wantDisabled(t, w, tabletReg.ID, "device_revoked")
	wantStatus(t, "phone after disable", w.fetch(w.alex, phoneReg.PreviewToken, msg), http.StatusUnauthorized, "unauthenticated")
	wantStatus(t, "tablet after disable", w.fetch(tablet, tabletReg.PreviewToken, msg), http.StatusUnauthorized, "unauthenticated")
}

// TestSessionEnd_ReasonMapping: a session ended by the owner (revokeSession, reason admin) gives
// logout (design note 7 a); the mapping of every reason is TestDisableReasonForSession.
func TestSessionEnd_ReasonMapping(t *testing.T) {
	w, _, phoneReg, tabletReg, _ := alexDevices(t)
	harness.MustDo(t, w.s.Owner, harness.Request{Method: http.MethodPost, Path: "/sessions/" + w.alex.SessionID + "/revoke"}, http.StatusNoContent)
	wantDisabled(t, w, phoneReg.ID, "logout")
	wantActive(t, w, tabletReg.ID)
}
