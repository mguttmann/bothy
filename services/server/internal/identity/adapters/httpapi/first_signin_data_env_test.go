// SPDX-License-Identifier: Apache-2.0

package httpapi_test

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/domain"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
)

// firstPassword is the first call of setMyLocalCredential: a password and a new TOTP secret.
const firstPassword = `{"password":"` + testPassword + `","totp_enroll":true}`

// passwordStepUpWith confirms a step-up of the app session c with the password and the TOTP code
// of secret at the next step of the clock.
func (w *sworld) passwordStepUpWith(c *stepClient, secret []byte) *httpservertest.Response {
	w.t.Helper()
	id, _ := c.challenge()
	w.clk.Advance(domain.TOTPPeriod) // a TOTP code is used once
	body, _ := json.Marshal(map[string]any{"challenge_id": id, "password": testPassword, "totp_code": w.code(secret)})
	return c.call(http.MethodPost, stepUpPasswordPath, string(body))
}

// TestFirstSignInData_NoSelfIssuedStepUp (05 section 2.4, 07 section 4.2 step 8): a session that
// can confirm a step-up needs one for passkey.register and local_credential.set, also for the
// first sign-in data of a kind, so whoever holds only a session never issues the factor that
// confirms its own step-up. Only a session with nothing to confirm with sets up the first sign-in
// data without one, and never from a cli device.
func TestFirstSignInData_NoSelfIssuedStepUp(t *testing.T) {
	t.Parallel()
	w := newConsoleWorld(t)
	keyless := func(a localAccount) *stepClient {
		return w.loginDevice(a.user, identity.DeviceStateActive, identity.PlatformWindows, false)
	}
	refused := func(t *testing.T, r *httpservertest.Response) {
		t.Helper()
		w.want(r, http.StatusUnauthorized, "step_up_required")
	}
	// run runs one case with the world reporting to its subtest, so every case reports alone.
	run := func(name string, f func(t *testing.T)) {
		t.Run(name, func(t *testing.T) {
			parent := w.t
			w.t = t
			defer func() { w.t = parent }()
			f(t)
		})
	}
	run("DeviceWithStepUpKey", func(t *testing.T) {
		a := w.localPerson("ada")
		// The session holds no step-up; its device has a step_up key, so the key is asked for.
		refused(t, a.phone.setLocal(firstPassword))
		refused(t, a.phone.call(http.MethodPost, registerBeginPath, ""))
		if n := w.credRows(a.user) + w.passkeys(a.user, false); n != 0 {
			t.Fatalf("%d rows of sign-in data without a step-up", n)
		}
		a.phone.stepUp()
		w.want(a.phone.register(authenticator(), "Laptop"), http.StatusCreated, "")
		w.establish(a.phone)
	})
	run("NothingToConfirmWith", func(t *testing.T) {
		// A device without a step_up key, a person without a passkey and without a password:
		// the first set-up needs only the session, as before.
		win := keyless(w.localPerson("bea"))
		w.establish(win)
	})
	run("FirstPasskeyNeedsThePassword", func(t *testing.T) {
		win := keyless(w.localPerson("cleo"))
		secret := w.establish(win)
		// The session confirms with password and TOTP now, so the first passkey needs that.
		refused(t, win.call(http.MethodPost, registerBeginPath, ""))
		w.want(w.passwordStepUpWith(win, secret), http.StatusOK, "")
		w.want(win.register(authenticator(), "Key"), http.StatusCreated, "")
	})
	run("FirstPasswordNeedsThePasskey", func(t *testing.T) {
		d := w.localPerson("dana")
		win := keyless(d)
		pk := authenticator()
		w.want(win.register(pk, "Key"), http.StatusCreated, "")
		refused(t, win.setLocal(firstPassword))
		if n := w.credRows(d.user); n != 0 {
			t.Fatalf("%d local credentials without a step-up", n)
		}
		r, _ := win.passkeyStepUp(pk)
		w.want(r, http.StatusOK, "")
		w.establish(win)
	})
	run("ConsoleSession", func(t *testing.T) {
		e := w.localPerson("eli")
		secret := w.establish(keyless(e))
		c := w.newConsole()
		code := func() string {
			w.clk.Advance(domain.TOTPPeriod)
			return w.code(secret)
		}
		b, _ := json.Marshal(map[string]any{"client": "console", "login_name": "eli", "password": testPassword, "totp_code": code()})
		w.want(c.do(http.MethodPost, passwordLoginPath, string(b)), http.StatusOK, "")
		b, _ = json.Marshal(map[string]any{"challenge_id": c.challenge()["challenge_id"], "password": testPassword, "totp_code": code()})
		w.want(c.do(http.MethodPost, stepUpPasswordPath, string(b)), http.StatusOK, "")
		// The step-up of the login ran out; the browser left open adds no passkey.
		w.clk.Advance(sessCfg.StepUpMaxAge + time.Second)
		refused(t, c.do(http.MethodPost, registerBeginPath, ""))
		if n := w.passkeys(e.user, false); n != 0 {
			t.Fatalf("%d passkeys from a console session without a step-up", n)
		}
	})
	run("CLIDevice", func(t *testing.T) {
		f := w.localPerson("finn")
		cli := w.loginDevice(f.user, identity.DeviceStateActive, identity.PlatformCLI, false)
		refused(t, cli.setLocal(firstPassword))
		refused(t, cli.call(http.MethodPost, registerBeginPath, ""))
		if n := w.credRows(f.user) + w.passkeys(f.user, false); n != 0 {
			t.Fatalf("%d rows of sign-in data from a cli device", n)
		}
	})
}
