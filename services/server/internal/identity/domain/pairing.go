// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
)

// Device pairing (docs/spec/08-identity.md section 5.3, brief M1-38 "Pairing"): the new device
// shows a code of auth.pairing_code_chars Crockford Base32 characters, displayed in two groups
// joined by "-", stored only as HMAC-SHA-256 with the token pepper.

// pairingHashContext separates the HMAC input of pairing codes from every other use of the pepper.
const pairingHashContext = "pairing-code/v1"

// errShortPairingRandom reports a random source that delivered fewer bytes than characters.
var errShortPairingRandom = errors.New("identity: the random source delivered too few bytes for a pairing code")

// NewPairingCode reads one byte per character from r and returns the canonical code of chars
// characters of Crockford Base32 (upper case, no separator). Each byte maps to a character by its
// low five bits, which is unbiased because 256 is a multiple of 32. Errors: the read error or a
// short source; the caller treats both as internal.
//
// Example:
//
//	code, err := domain.NewPairingCode(domain.EnrollmentRandom, 8)
//	shown := domain.DisplayPairingCode(code) // 7KQ2-M9XD
func NewPairingCode(r io.Reader, chars int) (string, error) {
	b := make([]byte, chars)
	if _, err := io.ReadFull(r, b); err != nil {
		if errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, io.EOF) {
			return "", errShortPairingRandom
		}
		return "", fmt.Errorf("identity: pairing code random source: %w", err)
	}
	for i, v := range b {
		b[i] = enrollmentAlphabet[v&31]
	}
	return string(b), nil
}

// DisplayPairingCode returns the display form of a canonical code: two groups joined by "-", the
// first one the longer for an odd length. It is shown only on the new device.
func DisplayPairingCode(canonical string) string {
	half := (len(canonical) + 1) / 2
	return canonical[:half] + "-" + canonical[half:]
}

// NormalizePairingCode turns user input into the canonical code like NormalizeEnrollmentCode, but
// without a prefix: it removes "-" and spaces, upper-cases and maps O to 0 and I and L to 1. ok is
// false for anything that is not then exactly chars characters of the alphabet; such an input is
// no code and is never looked up.
func NormalizePairingCode(in string, chars int) (string, bool) {
	out := make([]byte, 0, chars)
	for i := 0; i < len(in); i++ {
		c := in[i]
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
		if strings.IndexByte(enrollmentAlphabet, c) < 0 || len(out) == chars {
			return "", false
		}
		out = append(out, c)
	}
	if len(out) != chars {
		return "", false
	}
	return string(out), true
}

// PairingCodeHash returns device_pairings.pairing_code_hash of a canonical code:
// HMAC-SHA-256(key = pepper, msg = "pairing-code/v1" || 0x00 || canonical) (03 Grundsatz 9).
func PairingCodeHash(pepper []byte, canonical string) []byte {
	m := hmac.New(sha256.New, pepper)
	m.Write([]byte(pairingHashContext))
	m.Write([]byte{0})
	m.Write([]byte(canonical))
	return m.Sum(nil)
}

// PairingStatus is device_pairings.status.
type PairingStatus string

// The states of a pairing (03 device_pairings).
const (
	// PairingRequested waits for the approval of an active device.
	PairingRequested PairingStatus = "requested"
	// PairingApproved created the device; the new device may collect its tokens.
	PairingApproved PairingStatus = "approved"
	// PairingCompleted handed out the tokens.
	PairingCompleted PairingStatus = "completed"
	// PairingRejected ended by a proof with another key.
	PairingRejected PairingStatus = "rejected"
	// PairingExpired ended unused after expires_at.
	PairingExpired PairingStatus = "expired"
)

// pairingTransitions are the allowed transitions: requested -> approved -> completed,
// requested -> rejected | expired, approved -> expired.
var pairingTransitions = map[PairingStatus][]PairingStatus{
	PairingRequested: {PairingApproved, PairingRejected, PairingExpired},
	PairingApproved:  {PairingCompleted, PairingExpired},
}

// To checks the transition from s to next. Errors: invalid_state_transition for every transition
// the state machine does not list.
func (s PairingStatus) To(next PairingStatus) error {
	for _, n := range pairingTransitions[s] {
		if n == next {
			return nil
		}
	}
	return errs.New(errs.CodeInvalidStateTransition, "")
}

// Open reports a pairing that can still change: requested or approved.
func (s PairingStatus) Open() bool { return s == PairingRequested || s == PairingApproved }

// DeviceRegistrationV1 is device_pairings.device_registration, schema device_registration.v1:
// the registration of the new device with its public keys, never a secret.
type DeviceRegistrationV1 struct {
	// V is the schema version, always 1.
	V int `json:"v"`
	// DeviceID is the UUIDv7 the client generated.
	DeviceID string `json:"device_id"`
	// Name, Platform and AppVersion are shown in the device list.
	Name       string `json:"name"`
	Platform   string `json:"platform"`
	AppVersion string `json:"app_version,omitempty"`
	// DPoP is the key of the DPoP proofs; StepUp the key with user verification, nil for none.
	DPoP   RegisteredKey  `json:"dpop"`
	StepUp *RegisteredKey `json:"step_up,omitempty"`
	// Attestation is the platform attestation of the DPoP key, nil for none.
	Attestation json.RawMessage `json:"attestation,omitempty"`
}

// RegisteredKey is one public key of a DeviceRegistrationV1.
type RegisteredKey struct {
	// JWK is the public JWK without private members.
	JWK json.RawMessage `json:"jwk"`
	// Alg is ES256 or EdDSA.
	Alg string `json:"alg"`
	// Attestation is the platform attestation of the key, nil for none.
	Attestation json.RawMessage `json:"attestation,omitempty"`
}
