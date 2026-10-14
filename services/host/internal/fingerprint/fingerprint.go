// SPDX-License-Identifier: Apache-2.0

package fingerprint

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"
)

// Label is the domain separation prefix of the fingerprint hash (docs/spec/24-terminal.md 4.3).
// It changes only with a new protocol version (docs/spec/02-architecture.md "Code layout",
// D-N10).
const Label = "bothy-host-fp/1"

// keyLen is the length of both raw public keys: X25519 and Ed25519 keys are 32 bytes
// (RFC 7748, RFC 8032).
const keyLen = 32

// groupLen is the number of characters per group of the display form (docs/spec/24-terminal.md
// 4.3, the form of key_fingerprint in docs/api/openapi.yaml).
const groupLen = 4

// ErrInvalidKey reports a public key that is not exactly 32 bytes long.
var ErrInvalidKey = errors.New("fingerprint: invalid key length")

// Compute returns the display form of the fingerprint of a host: 52 Crockford base32
// characters in 13 groups of four joined by "-". noisePublicKey is the raw X25519 key
// (hosts.noise_public_key), authPublicKey the raw Ed25519 key (the decoded x of
// hosts.auth_public_jwk). Both must be 32 bytes, else the error wraps ErrInvalidKey. The order
// of the keys matters.
//
// Example: the noise key 0x00..0x1f and the auth key 0x20..0x3f give
// "F0CC-6QS6-RF5D-3BSH-CAZQ-PXMR-J8RA-AHK5-1HF0-9S02-1GW6-BW3J-HK90".
func Compute(noisePublicKey, authPublicKey []byte) (string, error) {
	if len(noisePublicKey) != keyLen {
		return "", fmt.Errorf("fingerprint: noise key of %d bytes, want %d: %w", len(noisePublicKey), keyLen, ErrInvalidKey)
	}
	if len(authPublicKey) != keyLen {
		return "", fmt.Errorf("fingerprint: auth key of %d bytes, want %d: %w", len(authPublicKey), keyLen, ErrInvalidKey)
	}
	h := sha256.New()
	h.Write([]byte(Label)) // hash.Hash.Write never returns an error
	h.Write(noisePublicKey)
	h.Write(authPublicKey)
	encoded := Encode(h.Sum(nil))
	var b strings.Builder
	for i := 0; i < len(encoded); i += groupLen {
		if i > 0 {
			b.WriteByte('-')
		}
		b.WriteString(encoded[i:min(i+groupLen, len(encoded))])
	}
	return b.String(), nil
}

// Compact removes the group separators of a display form, for comparing a fingerprint that a
// person typed or pasted with or without them.
func Compact(display string) string {
	return strings.ReplaceAll(display, "-", "")
}
