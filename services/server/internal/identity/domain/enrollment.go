// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
)

// Enrollment codes (docs/spec/08-identity.md section 3.2, brief M1-37 "Enrollment code format
// and storage"): 20 random bytes (160 bits, above the floor of 128) as 32 characters of Crockford
// Base32, shown as "en_" plus eight groups of four characters joined by "-", stored only as
// HMAC-SHA-256 with the token pepper.

// enrollmentAlphabet is Crockford Base32: digits and capitals without I, L, O and U.
const enrollmentAlphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

const (
	// enrollmentRandomBytes is the entropy of a code: 160 bits.
	enrollmentRandomBytes = 20
	// enrollmentCodeChars is the length of the canonical code: 160 bits / 5 bits per character.
	enrollmentCodeChars = 32
	// enrollmentPrefix starts the display form.
	enrollmentPrefix = "en_"
	// enrollmentGroup is the length of one group of the display form.
	enrollmentGroup = 4
	// enrollmentHashContext separates the HMAC input of codes from every other use of the pepper.
	enrollmentHashContext = "enrollment-code/v1"
)

// EnrollmentRandom is the random source of NewEnrollmentCode in production: crypto/rand.Reader.
// No package under internal/identity imports math/rand (ID-T27, depguard).
var EnrollmentRandom io.Reader = rand.Reader

// errShortRandom reports a random source that delivered fewer than 20 bytes.
var errShortRandom = errors.New("identity: the random source delivered too few bytes for an enrollment code")

// NewEnrollmentCode reads exactly 20 bytes from r and returns them as the canonical code: 32
// characters of Crockford Base32, upper case, without separators. Errors: the read error or
// errShortRandom; the caller treats both as internal.
//
// Example:
//
//	code, err := domain.NewEnrollmentCode(domain.EnrollmentRandom)
//	shown := domain.DisplayEnrollmentCode(code) // en_XXXX-XXXX-...
func NewEnrollmentCode(r io.Reader) (string, error) {
	var b [enrollmentRandomBytes]byte
	if _, err := io.ReadFull(r, b[:]); err != nil {
		if errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, io.EOF) {
			return "", errShortRandom
		}
		return "", fmt.Errorf("identity: enrollment code random source: %w", err)
	}
	// 160 bits are exactly 32 groups of 5 bits, most significant bit first.
	out := make([]byte, 0, enrollmentCodeChars)
	var acc uint64
	bits := 0
	for _, v := range b {
		acc = acc<<8 | uint64(v)
		bits += 8
		for bits >= 5 {
			bits -= 5
			out = append(out, enrollmentAlphabet[(acc>>uint(bits))&31])
		}
	}
	return string(out), nil
}

// DisplayEnrollmentCode returns the display form of a canonical code: "en_" plus groups of four
// characters joined by "-". It is the only form that leaves the server (once).
func DisplayEnrollmentCode(canonical string) string {
	var b strings.Builder
	b.WriteString(enrollmentPrefix)
	for i := 0; i < len(canonical); i += enrollmentGroup {
		if i > 0 {
			b.WriteByte('-')
		}
		b.WriteString(canonical[i:min(i+enrollmentGroup, len(canonical))])
	}
	return b.String()
}

// NormalizeEnrollmentCode turns user input into the canonical code: it trims, drops an optional
// "en_" prefix in any case, removes "-" and spaces, upper-cases and maps O to 0 and I and L to 1
// (Crockford). ok is false for anything that is not then 32 characters of the alphabet; such an
// input is no code and is never looked up.
func NormalizeEnrollmentCode(in string) (string, bool) {
	s := strings.TrimSpace(in)
	if len(s) >= len(enrollmentPrefix) && strings.EqualFold(s[:len(enrollmentPrefix)], enrollmentPrefix) {
		s = s[len(enrollmentPrefix):]
	}
	out := make([]byte, 0, enrollmentCodeChars)
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '-' || c == ' ':
			continue
		case c >= 'a' && c <= 'z':
			c -= 'a' - 'A'
		}
		switch c {
		case 'O':
			c = '0'
		case 'I', 'L':
			c = '1'
		}
		if strings.IndexByte(enrollmentAlphabet, c) < 0 || len(out) == enrollmentCodeChars {
			return "", false
		}
		out = append(out, c)
	}
	if len(out) != enrollmentCodeChars {
		return "", false
	}
	return string(out), true
}

// EnrollmentCodeHash returns enrollment_codes.code_hash of a canonical code:
// HMAC-SHA-256(key = pepper, msg = "enrollment-code/v1" || 0x00 || canonical) (03 Grundsatz 9).
func EnrollmentCodeHash(pepper []byte, canonical string) []byte {
	m := hmac.New(sha256.New, pepper)
	m.Write([]byte(enrollmentHashContext))
	m.Write([]byte{0})
	m.Write([]byte(canonical))
	return m.Sum(nil)
}

// EnrollmentPurpose is enrollment_codes.purpose (08 section 3.2).
type EnrollmentPurpose string

// The three purposes of an enrollment code.
const (
	// PurposeFirstLogin is the first device of a person with a local identity and no device.
	PurposeFirstLogin EnrollmentPurpose = "first_login"
	// PurposeRecovery is a new device of a person with a local identity and a device.
	PurposeRecovery EnrollmentPurpose = "recovery"
	// PurposeIdentityLink links an external identity of a person without a local identity.
	PurposeIdentityLink EnrollmentPurpose = "identity_link"
)

// Valid reports whether p is one of the three purposes.
func (p EnrollmentPurpose) Valid() bool {
	return p == PurposeFirstLogin || p == PurposeRecovery || p == PurposeIdentityLink
}

// IssuePurpose returns the purpose of a code createEnrollmentCode issues: identity_link without a
// local identity, otherwise first_login without a device row and recovery with one.
func IssuePurpose(hasLocalIdentity, hasDevice bool) EnrollmentPurpose {
	switch {
	case !hasLocalIdentity:
		return PurposeIdentityLink
	case hasDevice:
		return PurposeRecovery
	}
	return PurposeFirstLogin
}

// EnrollmentConsumer is a path that redeems a code.
type EnrollmentConsumer string

// The three consumers of 08 section 3.2.
const (
	// ConsumerRedeem is redeemEnrollmentCode (device keys only, M1-37).
	ConsumerRedeem EnrollmentConsumer = "redeem"
	// ConsumerPasskey is the enrollment with a passkey (completeEnrollment, M1-40).
	ConsumerPasskey EnrollmentConsumer = "passkey"
	// ConsumerIdentityLink is the linking at an OIDC or header login (M1-41, M1-43).
	ConsumerIdentityLink EnrollmentConsumer = "identity_link"
)

// ConsumablePurposes returns the purposes consumer may redeem (08 section 3.2 table).
func ConsumablePurposes(consumer EnrollmentConsumer) []EnrollmentPurpose {
	switch consumer {
	case ConsumerRedeem, ConsumerPasskey:
		return []EnrollmentPurpose{PurposeFirstLogin, PurposeRecovery}
	case ConsumerIdentityLink:
		return []EnrollmentPurpose{PurposeFirstLogin, PurposeRecovery, PurposeIdentityLink}
	}
	return nil
}

// PurposeConsumableBy reports whether a code of purpose p may be redeemed by consumer.
func PurposeConsumableBy(p EnrollmentPurpose, consumer EnrollmentConsumer) bool {
	for _, a := range ConsumablePurposes(consumer) {
		if a == p {
			return true
		}
	}
	return false
}

// EnrollmentCodeState is the part of an enrollment_codes row that decides whether it is usable.
type EnrollmentCodeState struct {
	// ExpiresAt is expires_at; UsedAt is used_at (zero while unused).
	ExpiresAt, UsedAt time.Time
	// Attempts is the number of failed attempts.
	Attempts int
}

// Usable reports a code that is unused, not expired at now (now < expires_at) and has fewer than
// maxAttempts failed attempts (auth.enrollment_code_max_attempts).
func (s EnrollmentCodeState) Usable(now time.Time, maxAttempts int) bool {
	return s.UsedAt.IsZero() && now.Before(s.ExpiresAt) && s.Attempts < maxAttempts
}
