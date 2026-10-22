// SPDX-License-Identifier: Apache-2.0

package smoke

import (
	"context"
	"encoding/base32"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/api/rest"
)

// seedFake is the server side of the seed: it redeems the code and plays setMyLocalCredential.
type seedFake struct {
	secret  []byte
	calls   []rest.LocalCredentialSet
	token   string
	enabled bool
}

func (f *seedFake) SetCredentials(_ Key, token string) { f.token = token }

func (f *seedFake) RedeemEnrollmentCode(_ context.Context, body rest.EnrollRedeemRequest) (rest.TokenResponse, error) {
	return rest.TokenResponse{TokenType: "DPoP", AccessToken: "at", RefreshToken: "rt",
		Device: rest.Device{Id: body.Device.DeviceId, State: rest.DeviceStateActive}}, nil
}

func (f *seedFake) SetMyLocalCredential(_ context.Context, body rest.LocalCredentialSet) (rest.LocalCredentialResult, error) {
	f.calls = append(f.calls, body)
	if f.token != "at" {
		return rest.LocalCredentialResult{}, &StatusError{Operation: "setMyLocalCredential", Status: 401}
	}
	if body.TotpEnroll != nil && *body.TotpEnroll {
		uri := "otpauth://totp/Team%20Chat:alex?secret=" + base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(f.secret)
		return rest.LocalCredentialResult{PasswordSet: true, TotpSecretUri: &uri}, nil
	}
	if body.TotpCode != nil && *body.TotpCode == TOTPCode(f.secret, TOTPStep(time.Now())) {
		f.enabled = true
	}
	return rest.LocalCredentialResult{PasswordSet: true, TotpEnabled: f.enabled}, nil
}

// TestSeed: the seed redeems the enrollment code, sets a password with TOTP, confirms it with a
// code of the current step and returns the login of the console test (brief M6-03 Design note 7).
func TestSeed(t *testing.T) {
	t.Parallel()
	f := &seedFake{secret: []byte("0123456789abcdefghij")}
	login, err := Seed(t.Context(), f, []byte("en_0000-1111-2222-3333-4444-5555-6666-7777\n"))
	if err != nil {
		t.Fatal(err)
	}
	if !f.enabled || len(f.calls) != 2 || f.calls[0].Password == nil || login.Password != *f.calls[0].Password {
		t.Fatalf("calls %+v, login %+v", f.calls, login)
	}
	if login.LoginName != smokeHandle || len(login.Password) < 20 || login.TOTPSecret == "" || login.LastStep == 0 {
		t.Fatalf("login %+v", login)
	}
	b, err := json.Marshal(login) // #nosec G117 -- a fake login of the test
	if err != nil || !strings.Contains(string(b), `"totp_secret"`) {
		t.Fatalf("json %s %v", b, err)
	}
	if _, err := Seed(t.Context(), f, []byte("not a code")); err == nil {
		t.Fatal("a bad setup output was accepted")
	}
}

// TestTOTPCode holds RFC 6238 appendix B (SHA-1, 8 digits there; the last 6 here).
func TestTOTPCode(t *testing.T) {
	t.Parallel()
	// The key of appendix B: the ASCII digits 1 to 9 and 0, twice.
	secret := []byte(strings.Repeat("1234567890", 2))
	for step, want := range map[int64]string{59 / 30: "287082", 1111111109 / 30: "081804", 1234567890 / 30: "005924"} {
		if got := TOTPCode(secret, step); got != want {
			t.Errorf("step %d: %s, want %s", step, got, want)
		}
	}
}
