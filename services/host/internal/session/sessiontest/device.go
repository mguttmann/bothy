// SPDX-License-Identifier: Apache-2.0

package sessiontest

import (
	"errors"

	"github.com/OWNER/bothy/services/host/internal/frame"
	"github.com/OWNER/bothy/services/host/internal/noise"
)

// Device is the Go test device: the Noise initiator and the frames (M2-38).
type Device struct {
	Ephemeral noise.KeyPair
	init      *noise.Initiator
	t         *noise.Transport
	// Payload2 is the payload of message 2.
	Payload2 []byte
}

// NewDevice returns a device for the session sessionID on host hostID with the host's static key.
func NewDevice(hostStatic [noise.KeyLen]byte, sessionID, hostID string) (*Device, error) {
	sid, err := noise.ParseID(sessionID)
	if err != nil {
		return nil, err
	}
	hid, err := noise.ParseID(hostID)
	if err != nil {
		return nil, err
	}
	e, err := noise.GenerateKeyPair(nil)
	if err != nil {
		return nil, err
	}
	i, err := noise.NewInitiator(noise.InitiatorConfig{HostStatic: hostStatic, SessionID: sid, HostID: hid, Ephemeral: e})
	if err != nil {
		return nil, err
	}
	return &Device{Ephemeral: e, init: i}, nil
}

// Cek returns the client_ephemeral_key_hash of the device (the claim cek).
func (d *Device) Cek() string {
	h, _ := noise.EphemeralKeyHash(d.Ephemeral.Public[:])
	return h
}

// Message1 returns message 1 with the token as payload.
func (d *Device) Message1(token string) ([]byte, error) { return d.init.WriteMessage1([]byte(token)) }

// ReadMessage2 completes the handshake.
func (d *Device) ReadMessage2(msg []byte) error {
	p, t, err := d.init.ReadMessage2(msg)
	if err != nil {
		return err
	}
	d.Payload2, d.t = p, t
	return nil
}

// Seal encrypts frame f.
func (d *Device) Seal(f frame.Frame) ([]byte, error) {
	b, err := frame.Encode(f)
	if err != nil {
		return nil, err
	}
	return d.SealRaw(b)
}

// SealRaw encrypts raw frame bytes (a negative test may send anything).
func (d *Device) SealRaw(b []byte) ([]byte, error) {
	if d.t == nil {
		return nil, errors.New("sessiontest: no transport before message 2")
	}
	return d.t.Encrypt(b)
}

// Open decrypts and decodes one message of the host.
func (d *Device) Open(msg []byte) (frame.Frame, error) {
	if d.t == nil {
		return frame.Frame{}, errors.New("sessiontest: no transport before message 2")
	}
	pt, err := d.t.Decrypt(msg)
	if err != nil {
		return frame.Frame{}, err
	}
	return frame.Decode(pt, frame.FromHost)
}
