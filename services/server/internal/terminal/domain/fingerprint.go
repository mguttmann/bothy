// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"crypto/sha256"
	"errors"
	"strings"
)

// FingerprintLabel is the domain separation label of the fingerprint (24 section 4.3).
const FingerprintLabel = "bothy-host-fp/1"

// Lengths of the fingerprint (24 section 4.3): 52 Crockford characters for the 32 bytes of the
// hash, shown in groups of four.
const (
	// FingerprintChars is the length of the compact fingerprint without separators.
	FingerprintChars = 52
	// fingerprintGroup is the size of one display group.
	fingerprintGroup = 4
	// hostKeyBytes is the length of both public keys.
	hostKeyBytes = 32
)

// crockfordAlphabet is the Crockford base32 alphabet without I, L, O and U.
const crockfordAlphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

// errKeyLength reports a key that is not exactly 32 bytes; it never names the key.
var errKeyLength = errors.New("terminal: a host key must be exactly 32 bytes")

// ComputeFingerprint returns the fingerprint of a host in display form (24 section 4.3):
// Crockford base32 of SHA-256(FingerprintLabel || noisePublicKey || authPublicKey) as 13 groups of
// four characters joined by "-". Both keys are the raw public keys of exactly 32 bytes (X25519
// and the decoded x of the Ed25519 JWK). The host service computes the same function (M2-38);
// both check services/host/testdata/noise/fingerprint.json. Errors: a key of another length.
//
// Example:
//
//	fp, err := domain.ComputeFingerprint(noise, authRaw) // "F0CC-6QS6-...-HK90"
func ComputeFingerprint(noisePublicKey, authPublicKey []byte) (string, error) {
	if len(noisePublicKey) != hostKeyBytes || len(authPublicKey) != hostKeyBytes {
		return "", errKeyLength
	}
	h := sha256.New()
	h.Write([]byte(FingerprintLabel))
	h.Write(noisePublicKey)
	h.Write(authPublicKey)
	compact := crockford(h.Sum(nil))
	var b strings.Builder
	for i := 0; i < len(compact); i += fingerprintGroup {
		if i > 0 {
			b.WriteByte('-')
		}
		b.WriteString(compact[i:min(i+fingerprintGroup, len(compact))])
	}
	return b.String(), nil
}

// CompactFingerprint returns the display form without its separators, the form the owner types
// a prefix of (HostConfirm.fingerprint_prefix).
func CompactFingerprint(display string) string { return strings.ReplaceAll(display, "-", "") }

// crockford encodes b in Crockford base32: 5-bit groups from the most significant bit, the last
// group padded with zero bits on the right, upper case, no padding character, no check symbol.
func crockford(b []byte) string {
	var out strings.Builder
	var acc uint
	bits := 0
	for _, c := range b {
		acc = acc<<8 | uint(c)
		bits += 8
		for bits >= 5 {
			bits -= 5
			out.WriteByte(crockfordAlphabet[(acc>>uint(bits))&0x1f])
		}
	}
	if bits > 0 {
		out.WriteByte(crockfordAlphabet[(acc<<uint(5-bits))&0x1f])
	}
	return out.String()
}
