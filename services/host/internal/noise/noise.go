// SPDX-License-Identifier: Apache-2.0

package noise

import (
	"crypto/ecdh"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
)

// Protocol constants. They change only with a new protocol version, never through
// configuration (docs/spec/02-architecture.md "Code layout", D-N10).
const (
	// ProtocolName is the Noise protocol of the terminal stream (docs/spec/24-terminal.md 5.2).
	ProtocolName = "Noise_NK_25519_ChaChaPoly_SHA256"
	// PrologueLabel opens the prologue (docs/spec/24-terminal.md 5.2); 16 ASCII bytes.
	PrologueLabel = "bothy-terminal/1"
	// MaxMessageLen is the largest Noise message (Noise revision 34, section 3).
	MaxMessageLen = 65535
	// TagLen is the length of the ChaChaPoly authentication tag (Noise revision 34, section 12.3).
	TagLen = 16
	// MaxPlaintextLen is the largest plaintext of one transport message.
	MaxPlaintextLen = MaxMessageLen - TagLen
	// KeyLen is the length of an X25519 key (RFC 7748) and of the handshake hash.
	KeyLen = 32
	// IDLen is the length of a session or host ID in the prologue: the 16 bytes of a UUID.
	IDLen = 16
)

// uuidLen is the length of the canonical text form 8-4-4-4-12 of a UUID (RFC 9562).
const uuidLen = 36

// ParseID returns the 16 bytes of a session or host ID in the octet order of RFC 9562, the
// bytes as the string reads from left to right. Only the canonical 36-character form 8-4-4-4-12
// is accepted, with hex digits in either case; braces, a urn:uuid: prefix or other lengths fail
// with an error wrapping ErrInvalidID. The version is not checked: the prologue binds bytes, and
// the server validates its own IDs (docs/DECISIONS.md D-M2-38-7).
//
// Example: ParseID("0192f4a0-7b1c-7b22-9d4f-5a6b7c8d9e0f") gives 01 92 f4 a0 ... 9e 0f.
func ParseID(s string) ([IDLen]byte, error) {
	var id [IDLen]byte
	if len(s) != uuidLen {
		return id, fmt.Errorf("noise: id of %d characters, want %d: %w", len(s), uuidLen, ErrInvalidID)
	}
	hexDigits := make([]byte, 0, 2*IDLen)
	for i := 0; i < len(s); i++ {
		c := s[i]
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if c != '-' {
				return id, fmt.Errorf("noise: id without a hyphen at position %d: %w", i, ErrInvalidID)
			}
			continue
		}
		hexDigits = append(hexDigits, c)
	}
	if _, err := hex.Decode(id[:], hexDigits); err != nil {
		return id, fmt.Errorf("noise: id is not hexadecimal: %w", ErrInvalidID)
	}
	return id, nil
}

// Prologue returns PrologueLabel || sessionID || hostID, 48 bytes (docs/spec/24-terminal.md
// 5.2). Both sides must use the same prologue, else the handshake fails: it binds every stream
// to its protocol version, its session and its host.
func Prologue(sessionID, hostID [IDLen]byte) []byte {
	p := make([]byte, 0, len(PrologueLabel)+2*IDLen)
	p = append(p, PrologueLabel...)
	p = append(p, sessionID[:]...)
	return append(p, hostID[:]...)
}

// EphemeralKeyHash returns the SHA-256 of a 32-byte X25519 public key in base64url without
// padding: the client_ephemeral_key_hash of docs/api/openapi.yaml and the claim cek of the
// session token (docs/spec/24-terminal.md 5.3). Another length gives an error wrapping
// ErrInvalidKey. The host service compares it with the initiator ephemeral key of message 1
// (M2-45), so a relay cannot put the token into a handshake of its own.
func EphemeralKeyHash(public []byte) (string, error) {
	if len(public) != KeyLen {
		return "", fmt.Errorf("noise: ephemeral key of %d bytes, want %d: %w", len(public), KeyLen, ErrInvalidKey)
	}
	sum := sha256.Sum256(public)
	return base64.RawURLEncoding.EncodeToString(sum[:]), nil
}

// KeyPair is an X25519 key pair. Private is the 32-byte scalar as stored (the curve function
// clamps it), Public the matching public key.
type KeyPair struct {
	Private, Public [KeyLen]byte
}

// GenerateKeyPair reads 32 bytes from random and returns the key pair. Production passes
// crypto/rand.Reader; nil means crypto/rand.Reader as well. A short read gives an error.
func GenerateKeyPair(random io.Reader) (KeyPair, error) {
	if random == nil {
		random = rand.Reader
	}
	var private [KeyLen]byte
	if _, err := io.ReadFull(random, private[:]); err != nil {
		return KeyPair{}, fmt.Errorf("noise: read random key bytes: %w", err)
	}
	return KeyPairFromPrivate(private)
}

// KeyPairFromPrivate returns the key pair of a fixed private key, for recordings, test vectors
// and keys loaded from storage (M2-42).
func KeyPairFromPrivate(private [KeyLen]byte) (KeyPair, error) {
	key, err := ecdh.X25519().NewPrivateKey(private[:])
	if err != nil {
		return KeyPair{}, fmt.Errorf("noise: private key: %w", ErrInvalidKey)
	}
	kp := KeyPair{Private: private}
	copy(kp.Public[:], key.PublicKey().Bytes())
	return kp, nil
}

// check reports whether the public key belongs to the private key.
func (kp KeyPair) check() error {
	want, err := KeyPairFromPrivate(kp.Private)
	if err != nil {
		return err
	}
	if want.Public != kp.Public {
		return fmt.Errorf("noise: public key does not belong to the private key: %w", ErrInvalidKey)
	}
	return nil
}
