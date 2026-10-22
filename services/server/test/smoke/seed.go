// SPDX-License-Identifier: Apache-2.0

package smoke

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1" // #nosec G505 -- TOTP of RFC 6238 is HMAC-SHA-1, the algorithm of the server (08 2.5)
	"encoding/base32"
	"encoding/binary"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/OWNER/bothy/services/server/api/rest"
)

// totpPeriod is the step of a TOTP code (RFC 6238, 08 2.5).
const totpPeriod = 30 * time.Second

// SeedREST is what the seed of the console test needs from the server.
type SeedREST interface {
	// SetCredentials sets the DPoP key of every later proof and the access token.
	SetCredentials(k Key, accessToken string)
	// RedeemEnrollmentCode is redeemEnrollmentCode (POST /auth/enroll/redeem), 201.
	RedeemEnrollmentCode(ctx context.Context, body rest.EnrollRedeemRequest) (rest.TokenResponse, error)
	// SetMyLocalCredential is setMyLocalCredential (PUT /me/local-credential), 200.
	SetMyLocalCredential(ctx context.Context, body rest.LocalCredentialSet) (rest.LocalCredentialResult, error)
}

// ConsoleLogin is the local account the console test logs in with: the handle of the owner, its
// password and its TOTP secret (base32 without padding), and the TOTP step the seed already used,
// so the test never reuses it (08 2.5). The seed writes it as JSON (mode 0600) and never prints it.
type ConsoleLogin struct {
	LoginName  string `json:"login_name"`
	Password   string `json:"password"`
	TOTPSecret string `json:"totp_secret"`
	LastStep   int64  `json:"last_step"`
}

// Seed prepares the console test (brief M6-03 Design note 7) after `bothy setup bootstrap-owner`:
// it redeems the enrollment code of setupStdout as a device of the owner, sets a random password,
// enrolls TOTP and confirms it with the code of the current step. The owner then has a local
// account with password and TOTP for the console form of loginWithPassword. Errors name the step,
// never a value.
func Seed(ctx context.Context, api SeedREST, setupStdout []byte) (ConsoleLogin, error) {
	code, err := ReadEnrollmentCode(setupStdout)
	if err != nil {
		return ConsoleLogin{}, err
	}
	dpop, err := NewKey()
	if err != nil {
		return ConsoleLogin{}, err
	}
	stepUp, err := NewKey()
	if err != nil {
		return ConsoleLogin{}, err
	}
	api.SetCredentials(dpop, "")
	tr, err := api.RedeemEnrollmentCode(ctx, rest.EnrollRedeemRequest{
		EnrollmentCode: code,
		Device: rest.DeviceRegistration{
			DeviceId: newUUIDv7(), Name: "console e2e seed", Platform: rest.DevicePlatformMacos,
			Alg: rest.DeviceRegistrationAlgES256, PublicJwk: dpop.PublicJWK(),
			StepUpKey: &rest.DeviceKeyRegistration{Alg: rest.DeviceKeyRegistrationAlgES256, PublicJwk: stepUp.PublicJWK()},
		},
	})
	if err != nil {
		return ConsoleLogin{}, err
	}
	api.SetCredentials(dpop, tr.AccessToken)
	password, err := randomPassword()
	if err != nil {
		return ConsoleLogin{}, err
	}
	enroll := true
	set, err := api.SetMyLocalCredential(ctx, rest.LocalCredentialSet{Password: &password, TotpEnroll: &enroll})
	if err != nil {
		return ConsoleLogin{}, err
	}
	secret, err := totpSecret(set.TotpSecretUri)
	if err != nil {
		return ConsoleLogin{}, err
	}
	step := TOTPStep(time.Now())
	totp := TOTPCode(secret, step)
	done, err := api.SetMyLocalCredential(ctx, rest.LocalCredentialSet{TotpCode: &totp})
	if err != nil {
		return ConsoleLogin{}, err
	}
	if !done.TotpEnabled {
		return ConsoleLogin{}, errors.New("smoke: the TOTP enrollment was not confirmed")
	}
	return ConsoleLogin{LoginName: smokeHandle, Password: password, TOTPSecret: base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(secret), LastStep: step}, nil
}

// totpSecret returns the secret of the otpauth URI of setMyLocalCredential.
func totpSecret(uri *string) ([]byte, error) {
	if uri == nil {
		return nil, errors.New("smoke: setMyLocalCredential returned no totp_secret_uri")
	}
	u, err := url.Parse(*uri)
	if err != nil {
		return nil, errors.New("smoke: the totp_secret_uri is no URI")
	}
	s := strings.ToUpper(strings.TrimRight(u.Query().Get("secret"), "="))
	secret, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(s)
	if err != nil || len(secret) == 0 {
		return nil, errors.New("smoke: the totp_secret_uri has no base32 secret")
	}
	return secret, nil
}

// randomPassword returns 32 random lowercase hex characters with a suffix of the other classes,
// so it passes any composition rule of the password policy (08 2.5).
func randomPassword() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("smoke: random password: %w", err)
	}
	return fmt.Sprintf("%x-Aa1!", b), nil
}

// TOTPStep returns the TOTP step of t (RFC 6238, period 30 s, T0 = 0).
func TOTPStep(t time.Time) int64 { return t.Unix() / int64(totpPeriod/time.Second) }

// TOTPCode returns the six-digit HOTP of secret for step (RFC 4226 5.3 with HMAC-SHA-1).
func TOTPCode(secret []byte, step int64) string {
	var msg [8]byte
	binary.BigEndian.PutUint64(msg[:], uint64(step)) // #nosec G115 -- a step of the current time is positive
	m := hmac.New(sha1.New, secret)
	_, _ = m.Write(msg[:])
	sum := m.Sum(nil)
	off := sum[len(sum)-1] & 0x0f
	v := binary.BigEndian.Uint32(sum[off:off+4]) & 0x7fffffff
	return fmt.Sprintf("%06d", v%1000000)
}

// SetMyLocalCredential implements SeedREST; a PUT stores no response, so it carries no
// Idempotency-Key.
func (c *HTTPClient) SetMyLocalCredential(ctx context.Context, body rest.LocalCredentialSet) (rest.LocalCredentialResult, error) {
	var out rest.LocalCredentialResult
	return out, c.do(ctx, call{op: "setMyLocalCredential", method: "PUT", path: "/me/local-credential", body: body, want: 200, out: &out})
}
