// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/binary"
	"strconv"
	"strings"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// verificationContext separates the HMAC input of verification codes from every other use of the
// token pepper.
const verificationContext = "device-verification/v1"

// MaxVerificationDigits is the longest verification code: 10^19 still fits into a uint64.
const MaxVerificationDigits = 19

// DeriveVerificationCode returns the decimal code a pending device shows (docs/spec/08-identity.md
// section 5.2, R-ID-15): HMAC-SHA-256(key = pepper, msg = "device-verification/v1" || 0x00 ||
// deviceID (16 bytes) || 0x00 || dpopJKT), the first 8 bytes as big-endian uint64 modulo
// 10^digits, left-padded with zeros to digits. The code is derived, never stored: it depends on
// the device and its DPoP key, so a second login of an attacker with another key shows another
// code. digits is clamped to 1..MaxVerificationDigits (the configuration bounds it).
//
// Example:
//
//	code := domain.DeriveVerificationCode(pepper, dev.ID, dev.KeyThumbprint, 6) // "051265"
func DeriveVerificationCode(pepper []byte, deviceID ids.ID, dpopJKT string, digits int) string {
	digits = min(max(digits, 1), MaxVerificationDigits)
	m := hmac.New(sha256.New, pepper)
	m.Write([]byte(verificationContext))
	m.Write([]byte{0})
	m.Write(deviceID[:])
	m.Write([]byte{0})
	m.Write([]byte(dpopJKT))
	n := binary.BigEndian.Uint64(m.Sum(nil)[:8])
	mod := uint64(1)
	for range digits {
		mod *= 10
	}
	s := strconv.FormatUint(n%mod, 10)
	return strings.Repeat("0", digits-len(s)) + s
}

// VerificationCodeEqual compares the expected code with the typed one in constant time; spaces in
// either are ignored (people type "051 265"), any other difference, a different length included,
// is false.
func VerificationCodeEqual(expected, typed string) bool {
	e, t := strings.ReplaceAll(expected, " ", ""), strings.ReplaceAll(typed, " ", "")
	return e != "" && subtle.ConstantTimeCompare([]byte(e), []byte(t)) == 1
}
