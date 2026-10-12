// SPDX-License-Identifier: Apache-2.0

package noise

import (
	"bytes"
	"crypto/subtle"
	"fmt"
	"io"

	"github.com/flynn/noise"
)

// handshakeOverhead is the part of a handshake message of the pattern NK that is not payload:
// the ephemeral public key and the tag of the encrypted payload (Noise revision 34, 7.5).
const handshakeOverhead = KeyLen + TagLen

// safeDH is X25519 that fails on a Diffie-Hellman result of 32 zero bytes, which a low-order
// public key produces. golang.org/x/crypto/curve25519 rejects it today; the wrapper keeps the
// rule independent of the library version and equal to the Dart side (docs/DECISIONS.md
// D-M2-38-9).
type safeDH struct{ noise.DHFunc }

func (d safeDH) DH(private, public []byte) ([]byte, error) {
	out, err := d.DHFunc.DH(private, public)
	if err != nil {
		return nil, err
	}
	if subtle.ConstantTimeCompare(out, make([]byte, len(out))) == 1 {
		return nil, fmt.Errorf("noise: zero Diffie-Hellman result")
	}
	return out, nil
}

var cipherSuite = noise.NewCipherSuite(safeDH{noise.DH25519}, noise.CipherChaChaPoly, noise.HashSHA256)

// InitiatorConfig configures the device side of the handshake.
type InitiatorConfig struct {
	// HostStatic is the host's static Noise public key from the pairing (hosts.noise_public_key).
	HostStatic [KeyLen]byte
	// SessionID and HostID go into the prologue.
	SessionID, HostID [IDLen]byte
	// Ephemeral is required: the device computes client_ephemeral_key_hash from it before the
	// session is opened (docs/spec/15-client-architecture.md 23.2, step 2).
	Ephemeral KeyPair
}

// Initiator is the device side of the handshake: WriteMessage1, then ReadMessage2. It is not
// safe for concurrent use.
type Initiator struct {
	hs     *noise.HandshakeState
	step   int
	broken bool
	hash   []byte
}

// NewInitiator starts the handshake of the device. The error wraps ErrInvalidKey if the
// ephemeral public key does not belong to its private key.
func NewInitiator(cfg InitiatorConfig) (*Initiator, error) {
	return newInitiator(cfg.HostStatic, Prologue(cfg.SessionID, cfg.HostID), cfg.Ephemeral)
}

// newInitiator takes a raw prologue; the test vectors of TM-T01 use their own prologues.
func newInitiator(hostStatic [KeyLen]byte, prologue []byte, ephemeral KeyPair) (*Initiator, error) {
	if err := ephemeral.check(); err != nil {
		return nil, err
	}
	hs, err := noise.NewHandshakeState(noise.Config{
		CipherSuite: cipherSuite,
		// The library draws the ephemeral key of a message from Random; this reader yields
		// exactly the configured private key.
		Random:     bytes.NewReader(bytes.Clone(ephemeral.Private[:])),
		Pattern:    noise.HandshakeNK,
		Initiator:  true,
		Prologue:   prologue,
		PeerStatic: bytes.Clone(hostStatic[:]),
	})
	if err != nil {
		return nil, fmt.Errorf("noise: start initiator: %w", ErrHandshake)
	}
	return &Initiator{hs: hs}, nil
}

// WriteMessage1 returns message 1 (-> e, es) with the payload, the session token, encrypted to
// the host. A payload that would make the message longer than MaxMessageLen gives
// ErrMessageTooLong and changes no state; a failed step (for example a low-order host key) gives
// ErrHandshake and breaks the initiator.
func (i *Initiator) WriteMessage1(payload []byte) ([]byte, error) {
	switch {
	case i.broken:
		return nil, fmt.Errorf("noise: write message 1: %w", ErrBroken)
	case i.hs == nil || i.step != 0:
		return nil, fmt.Errorf("noise: write message 1: %w", ErrState)
	case handshakeOverhead+len(payload) > MaxMessageLen:
		return nil, fmt.Errorf("noise: write message 1: payload of %d bytes, limit %d: %w", len(payload), MaxMessageLen-handshakeOverhead, ErrMessageTooLong)
	}
	msg, _, _, err := i.hs.WriteMessage(nil, payload)
	if err != nil {
		i.broken = true
		return nil, fmt.Errorf("noise: write message 1: %w", ErrHandshake)
	}
	i.step = 1
	return msg, nil
}

// ReadMessage2 processes message 2 (<- e, ee) and returns the host's payload and the transport
// of the device. Errors wrap ErrMessageTooLong or ErrHandshake (wrong prologue, wrong host key,
// tampered or short message) and break the initiator.
func (i *Initiator) ReadMessage2(msg []byte) (payload []byte, t *Transport, err error) {
	switch {
	case i.broken:
		return nil, nil, fmt.Errorf("noise: read message 2: %w", ErrBroken)
	case i.hs == nil || i.step != 1:
		return nil, nil, fmt.Errorf("noise: read message 2: %w", ErrState)
	case len(msg) > MaxMessageLen:
		i.broken = true
		return nil, nil, fmt.Errorf("noise: read message 2 of %d bytes, limit %d: %w", len(msg), MaxMessageLen, ErrMessageTooLong)
	case len(msg) < handshakeOverhead:
		i.broken = true
		return nil, nil, fmt.Errorf("noise: read message 2 of %d bytes, minimum %d: %w", len(msg), handshakeOverhead, ErrHandshake)
	}
	payload, send, recv, err := i.hs.ReadMessage(nil, msg)
	if err != nil || send == nil || recv == nil {
		i.broken = true
		return nil, nil, fmt.Errorf("noise: read message 2 of %d bytes: %w", len(msg), ErrHandshake)
	}
	i.step = 2
	i.hash = bytes.Clone(i.hs.ChannelBinding())
	return payload, &Transport{send: send, recv: recv}, nil
}

// HandshakeHash returns the handshake hash h after a completed handshake (a channel binding
// equal on both sides), else nil.
func (i *Initiator) HandshakeHash() []byte { return bytes.Clone(i.hash) }

// ResponderConfig configures the host side of the handshake.
type ResponderConfig struct {
	// Static is the host's static Noise key pair.
	Static KeyPair
	// SessionID and HostID go into the prologue.
	SessionID, HostID [IDLen]byte
	// Ephemeral fixes the ephemeral key of message 2 for recordings and test vectors; nil
	// generates it from Random.
	Ephemeral *KeyPair
	// Random is the source of the ephemeral key when Ephemeral is nil; nil means
	// crypto/rand.Reader.
	Random io.Reader
}

// Responder is the host side of the handshake: ReadMessage1, then WriteMessage2. It is not safe
// for concurrent use.
type Responder struct {
	hs     *noise.HandshakeState
	step   int
	broken bool
	hash   []byte
}

// NewResponder starts the handshake of the host service. The error wraps ErrInvalidKey if a
// public key does not belong to its private key, or reports a failed read of Random.
func NewResponder(cfg ResponderConfig) (*Responder, error) {
	return newResponder(cfg.Static, Prologue(cfg.SessionID, cfg.HostID), cfg.Ephemeral, cfg.Random)
}

// newResponder takes a raw prologue; the test vectors of TM-T01 use their own prologues.
func newResponder(static KeyPair, prologue []byte, ephemeral *KeyPair, random io.Reader) (*Responder, error) {
	if err := static.check(); err != nil {
		return nil, err
	}
	var eph KeyPair
	if ephemeral != nil {
		if err := ephemeral.check(); err != nil {
			return nil, err
		}
		eph = *ephemeral
	} else {
		var err error
		if eph, err = GenerateKeyPair(random); err != nil {
			return nil, err
		}
	}
	hs, err := noise.NewHandshakeState(noise.Config{
		CipherSuite:   cipherSuite,
		Random:        bytes.NewReader(bytes.Clone(eph.Private[:])),
		Pattern:       noise.HandshakeNK,
		Initiator:     false,
		Prologue:      prologue,
		StaticKeypair: noise.DHKey{Private: bytes.Clone(static.Private[:]), Public: bytes.Clone(static.Public[:])},
	})
	if err != nil {
		return nil, fmt.Errorf("noise: start responder: %w", ErrHandshake)
	}
	return &Responder{hs: hs}, nil
}

// ReadMessage1 processes message 1 and returns its payload (the session token) and the
// initiator's ephemeral public key, whose EphemeralKeyHash M2-45 compares with the claim cek.
// Errors wrap ErrMessageTooLong or ErrHandshake (wrong prologue, wrong static key, tampered or
// short message, low-order key) and break the responder.
func (r *Responder) ReadMessage1(msg []byte) (payload []byte, initiatorEphemeral [KeyLen]byte, err error) {
	switch {
	case r.broken:
		return nil, initiatorEphemeral, fmt.Errorf("noise: read message 1: %w", ErrBroken)
	case r.hs == nil || r.step != 0:
		return nil, initiatorEphemeral, fmt.Errorf("noise: read message 1: %w", ErrState)
	case len(msg) > MaxMessageLen:
		r.broken = true
		return nil, initiatorEphemeral, fmt.Errorf("noise: read message 1 of %d bytes, limit %d: %w", len(msg), MaxMessageLen, ErrMessageTooLong)
	case len(msg) < handshakeOverhead:
		r.broken = true
		return nil, initiatorEphemeral, fmt.Errorf("noise: read message 1 of %d bytes, minimum %d: %w", len(msg), handshakeOverhead, ErrHandshake)
	}
	payload, _, _, err = r.hs.ReadMessage(nil, msg)
	if err != nil {
		r.broken = true
		return nil, initiatorEphemeral, fmt.Errorf("noise: read message 1 of %d bytes: %w", len(msg), ErrHandshake)
	}
	r.step = 1
	copy(initiatorEphemeral[:], r.hs.PeerEphemeral())
	return payload, initiatorEphemeral, nil
}

// WriteMessage2 returns message 2 (<- e, ee) with the host's payload and the transport of the
// host. A payload that would make the message longer than MaxMessageLen gives
// ErrMessageTooLong and changes no state; a failed step gives ErrHandshake and breaks the
// responder.
func (r *Responder) WriteMessage2(payload []byte) (msg []byte, t *Transport, err error) {
	switch {
	case r.broken:
		return nil, nil, fmt.Errorf("noise: write message 2: %w", ErrBroken)
	case r.hs == nil || r.step != 1:
		return nil, nil, fmt.Errorf("noise: write message 2: %w", ErrState)
	case handshakeOverhead+len(payload) > MaxMessageLen:
		return nil, nil, fmt.Errorf("noise: write message 2: payload of %d bytes, limit %d: %w", len(payload), MaxMessageLen-handshakeOverhead, ErrMessageTooLong)
	}
	msg, recv, send, err := r.hs.WriteMessage(nil, payload)
	if err != nil || send == nil || recv == nil {
		r.broken = true
		return nil, nil, fmt.Errorf("noise: write message 2: %w", ErrHandshake)
	}
	r.step = 2
	r.hash = bytes.Clone(r.hs.ChannelBinding())
	return msg, &Transport{send: send, recv: recv}, nil
}

// HandshakeHash returns the handshake hash h after a completed handshake, else nil.
func (r *Responder) HandshakeHash() []byte { return bytes.Clone(r.hash) }
