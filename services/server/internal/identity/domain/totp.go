// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"crypto/hmac"
	"crypto/sha1" //nolint:gosec // G505: HMAC-SHA-1 is the TOTP algorithm of RFC 6238 every authenticator app supports, not a hash of stored data.
	"crypto/subtle"
	"encoding/base32"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"time"
)

// TOTP of local accounts (docs/spec/08-identity.md section 2.5, brief M1-39): RFC 6238 with
// HMAC-SHA-1, six digits, a step of 30 seconds from T0 = 0 and at most one step of tolerance. A
// code is accepted only for a step after the last accepted one, so every code works once (RFC 6238
// section 5.2). Tests hold these constants (ID-T27).
const (
	// TOTPPeriod is the time step X of RFC 6238.
	TOTPPeriod = 30 * time.Second
	// TOTPTolerance is how many steps before and after the current one are accepted.
	TOTPTolerance = 1
	// TOTPDigits is the length of a code.
	TOTPDigits = 6
	// TOTPSecretBytes is the length of a new secret: 160 bits, as RFC 4226 section 4 recommends.
	TOTPSecretBytes = 20
)

// errShortTOTPRandom reports a random source that delivered fewer than TOTPSecretBytes bytes.
var errShortTOTPRandom = errors.New("identity: the random source delivered too few bytes for a TOTP secret")

// NewTOTPSecret reads TOTPSecretBytes bytes from r (crypto/rand in production). Errors: the read
// error or errShortTOTPRandom; the caller treats both as internal.
func NewTOTPSecret(r io.Reader) ([]byte, error) {
	b := make([]byte, TOTPSecretBytes)
	if _, err := io.ReadFull(r, b); err != nil {
		if errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, io.EOF) {
			return nil, errShortTOTPRandom
		}
		return nil, fmt.Errorf("identity: TOTP secret random source: %w", err)
	}
	return b, nil
}

// TOTPStep returns the RFC 6238 time step of t: floor(unix seconds / 30), T0 = 0.
func TOTPStep(t time.Time) int64 {
	sec := t.Unix()
	period := int64(TOTPPeriod / time.Second)
	step := sec / period
	if sec < 0 && sec%period != 0 {
		step--
	}
	return step
}

// TOTPCode returns the HOTP value (RFC 4226 section 5.3) of secret for the counter step with
// digits decimal digits, zero-padded on the left. digits outside 1 to 9 is a programming error
// and panics.
//
// Example:
//
//	code := domain.TOTPCode(secret, domain.TOTPStep(now), domain.TOTPDigits)
func TOTPCode(secret []byte, step int64, digits int) string {
	if digits < 1 || digits > 9 {
		panic("identity: TOTP digits outside 1 to 9")
	}
	var msg [8]byte
	binary.BigEndian.PutUint64(msg[:], uint64(step)) //nolint:gosec // G115: the counter is the two's-complement bit pattern RFC 4226 hashes.
	mac := hmac.New(sha1.New, secret)
	mac.Write(msg[:])
	sum := mac.Sum(nil)
	off := sum[len(sum)-1] & 0x0f
	bin := binary.BigEndian.Uint32(sum[off:off+4]) & 0x7fffffff
	mod := uint32(1)
	for range digits {
		mod *= 10
	}
	return fmt.Sprintf("%0*d", digits, bin%mod)
}

// VerifyTOTP checks the six-digit code against secret at now: it is accepted for one of the steps
// current-1, current and current+1 that is greater than *lastStep (nil when no code was accepted
// yet). It returns the matched step. Each candidate is compared in constant time, and every
// candidate is computed, so the time does not reveal which step matched.
func VerifyTOTP(secret []byte, code string, now time.Time, lastStep *int64) (step int64, ok bool) {
	if !wellFormedCode(code) {
		return 0, false
	}
	cur := TOTPStep(now)
	for s := cur - TOTPTolerance; s <= cur+TOTPTolerance; s++ {
		match := subtle.ConstantTimeCompare([]byte(TOTPCode(secret, s, TOTPDigits)), []byte(code)) == 1
		if match && !ok && (lastStep == nil || s > *lastStep) {
			step, ok = s, true
		}
	}
	return step, ok
}

// wellFormedCode reports exactly TOTPDigits ASCII digits.
func wellFormedCode(code string) bool {
	if len(code) != TOTPDigits {
		return false
	}
	for i := range len(code) {
		if code[i] < '0' || code[i] > '9' {
			return false
		}
	}
	return true
}

// totpBase32 is the secret encoding of the otpauth URI: RFC 4648 Base32 without padding.
var totpBase32 = base32.StdEncoding.WithPadding(base32.NoPadding)

// TOTPURI returns the otpauth URI of an enrollment (brief M1-39 "TOTP"): label issuer:loginName,
// the secret in Base32 without padding, issuer, algorithm SHA1, six digits and 30 seconds; every
// part is URL-escaped. The URI is a secret: it is returned once and never logged.
//
// Example:
//
//	domain.TOTPURI("chat.example.com", "alex", secret)
//	// otpauth://totp/chat.example.com:alex?secret=...&issuer=chat.example.com&algorithm=SHA1&digits=6&period=30
func TOTPURI(issuer, loginName string, secret []byte) string {
	return "otpauth://totp/" + url.PathEscape(issuer) + ":" + url.PathEscape(loginName) +
		"?secret=" + url.QueryEscape(totpBase32.EncodeToString(secret)) +
		"&issuer=" + url.QueryEscape(issuer) +
		"&algorithm=SHA1&digits=" + strconv.Itoa(TOTPDigits) +
		"&period=" + strconv.Itoa(int(TOTPPeriod/time.Second))
}
