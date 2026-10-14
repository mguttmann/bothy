// SPDX-License-Identifier: Apache-2.0

package noise

import (
	"fmt"
	"math"
	"sync/atomic"

	"github.com/flynn/noise"
)

// lastNonce is reserved by Noise revision 34, section 5.1; no message may use it.
const lastNonce = math.MaxUint64

// Transport holds the two cipher states after the handshake: one for the messages this side
// sends and one for the messages it receives (the first cipher of Split runs from initiator to
// responder). One goroutine may call Encrypt while another calls Decrypt; neither method is safe
// for concurrent calls of itself, because each message takes the next nonce of its direction.
type Transport struct {
	send, recv   *noise.CipherState
	sendN, recvN uint64
	broken       atomic.Bool
}

// Encrypt returns the Noise transport message for plaintext (one frame of
// docs/spec/24-terminal.md 5.4). More than MaxPlaintextLen bytes give ErrMessageTooLong without
// using a nonce; a direction that has used every nonce gives ErrNonceExhausted; after a failure
// of the transport every call gives ErrBroken. Messages must be sent in the order of the calls.
func (t *Transport) Encrypt(plaintext []byte) ([]byte, error) {
	switch {
	case t.broken.Load():
		return nil, fmt.Errorf("noise: encrypt: %w", ErrBroken)
	case len(plaintext) > MaxPlaintextLen:
		return nil, fmt.Errorf("noise: encrypt %d bytes, limit %d: %w", len(plaintext), MaxPlaintextLen, ErrMessageTooLong)
	case t.sendN >= lastNonce:
		return nil, fmt.Errorf("noise: encrypt: %w", ErrNonceExhausted)
	}
	msg, err := t.send.Encrypt(nil, nil, plaintext)
	if err != nil {
		t.broken.Store(true)
		return nil, fmt.Errorf("noise: encrypt: %w", ErrBroken)
	}
	t.sendN++
	return msg, nil
}

// Decrypt authenticates and decrypts one transport message of the peer. A message over
// MaxMessageLen gives ErrMessageTooLong, a shorter one than the tag or one that does not
// authenticate gives ErrDecrypt; each of these breaks the transport, so every later call gives
// ErrBroken (docs/DECISIONS.md D-M2-38-8). A direction that has used every nonce gives
// ErrNonceExhausted.
func (t *Transport) Decrypt(msg []byte) ([]byte, error) {
	switch {
	case t.broken.Load():
		return nil, fmt.Errorf("noise: decrypt: %w", ErrBroken)
	case t.recvN >= lastNonce:
		return nil, fmt.Errorf("noise: decrypt: %w", ErrNonceExhausted)
	case len(msg) > MaxMessageLen:
		t.broken.Store(true)
		return nil, fmt.Errorf("noise: decrypt %d bytes, limit %d: %w", len(msg), MaxMessageLen, ErrMessageTooLong)
	case len(msg) < TagLen:
		t.broken.Store(true)
		return nil, fmt.Errorf("noise: decrypt %d bytes, minimum %d: %w", len(msg), TagLen, ErrDecrypt)
	}
	plaintext, err := t.recv.Decrypt(nil, nil, msg)
	if err != nil {
		t.broken.Store(true)
		return nil, fmt.Errorf("noise: decrypt %d bytes: %w", len(msg), ErrDecrypt)
	}
	t.recvN++
	return plaintext, nil
}

// setNonces sets the next nonce of both directions; a test hook for the nonce limit.
func (t *Transport) setNonces(send, recv uint64) {
	t.sendN, t.recvN = send, recv
	t.send.SetNonce(send)
	t.recv.SetNonce(recv)
}
