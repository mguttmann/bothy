// SPDX-License-Identifier: Apache-2.0

// Package noise is the end-to-end layer of the terminal stream: the handshake
// Noise_NK_25519_ChaChaPoly_SHA256 of the Noise Protocol Framework, revision 34, over
// github.com/flynn/noise, with the prologue "bothy-terminal/1" || session ID || host ID and the
// transport states that follow (docs/spec/24-terminal.md 5.2, ADR 0068, R-TM-02).
//
// The device is the initiator, the host service the responder. Message 1 carries the session
// token, message 2 the host's payload; both sides then hold a Transport. The package checks the
// limits of the Noise specification itself, so that its behaviour does not depend on the
// library version: at most 65535 bytes per Noise message, the reserved nonce 2^64 - 1, and a
// Diffie-Hellman result of 32 zero bytes fails. After any failed handshake step or decryption the
// object refuses every further call (ErrBroken), because a failed authentication means
// manipulation (docs/spec/24-terminal.md 5.3, close code 4104).
//
// The package does no I/O, writes no logs and never puts payloads, plaintext, keys or tokens
// into an error text (SEC-TERM-1, R-TM-10). The session token itself is checked by M2-45.
package noise
