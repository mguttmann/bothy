// SPDX-License-Identifier: Apache-2.0

package noise

import "errors"

// Errors of this package. Each error returned wraps exactly one of them; the text names the
// operation and lengths only, never payload, plaintext or key bytes (SEC-TERM-1, R-TM-10).
var (
	// ErrInvalidID reports a session or host ID that is not a canonical 36-character UUID.
	ErrInvalidID = errors.New("noise: invalid id")
	// ErrInvalidKey reports a key of the wrong length or a key pair whose public key does not
	// belong to its private key.
	ErrInvalidKey = errors.New("noise: invalid key")
	// ErrMessageTooLong reports a Noise message over 65535 bytes, or a payload or plaintext
	// that would make one (docs/spec/24-terminal.md 5.2).
	ErrMessageTooLong = errors.New("noise: message too long")
	// ErrHandshake reports a failed handshake step: a wrong prologue, a wrong static key, a
	// tampered or short message, or a Diffie-Hellman result of zero bytes.
	ErrHandshake = errors.New("noise: handshake failed")
	// ErrDecrypt reports a transport message that does not authenticate.
	ErrDecrypt = errors.New("noise: decryption failed")
	// ErrBroken reports a call after a failed handshake step or decryption.
	ErrBroken = errors.New("noise: state is broken after an earlier failure")
	// ErrNonceExhausted reports that a direction has used every nonce below the reserved
	// 2^64 - 1 (Noise revision 34, section 5.1); a new handshake is needed.
	ErrNonceExhausted = errors.New("noise: nonce exhausted")
	// ErrState reports a call out of the order of the handshake.
	ErrState = errors.New("noise: call out of order")
)
