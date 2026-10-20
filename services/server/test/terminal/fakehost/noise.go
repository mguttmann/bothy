// SPDX-License-Identifier: Apache-2.0

package fakehost

import (
	"bytes"
	"context"
	"crypto/ecdh"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	gosync "sync"

	"github.com/coder/websocket"
	"github.com/flynn/noise"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/jose"
	"github.com/OWNER/bothy/services/server/internal/terminal/adapters/hosttest"
)

// The Noise protocol of the terminal stream (24 section 5.2): the host's internal/noise cannot be
// imported from the server module, so the fakes speak it with github.com/flynn/noise at the
// version services/host/go.mod pins (M2-46 Design note 13).
const (
	// prologueLabel opens the prologue; 16 ASCII bytes.
	prologueLabel = "bothy-terminal/1"
	// noiseKeyLen is the length of an X25519 key.
	noiseKeyLen = 32
)

// The frame types of 24 section 5.4: the first byte of the plaintext of every transport message.
const (
	// FrameStart (device to host) carries columns and rows; once, as the first transport message.
	FrameStart byte = 0x01
	// FrameData (both directions) carries input or PTY output.
	FrameData byte = 0x02
	// FrameResize (device to host) carries columns and rows.
	FrameResize byte = 0x03
	// FrameClose (device to host) ends the session with client_closed.
	FrameClose byte = 0x04
	// FrameExit (host to device) carries the exit code.
	FrameExit byte = 0x05
)

// errNoiseKey is a NoiseAccept script on a host without UseNoise.
var errNoiseKey = errors.New("fakehost: NoiseAccept without a Noise key (UseNoise)")

// suite is the cipher suite of Noise_NK_25519_ChaChaPoly_SHA256.
var suite = noise.NewCipherSuite(noise.DH25519, noise.CipherChaChaPoly, noise.HashSHA256)

// NoiseKey is the key material of a host that speaks Noise: the Ed25519 login key of the control
// and data connections and the static X25519 key pair of the handshake, whose public half goes
// into pairHost as noise_public_key.
type NoiseKey struct {
	// Login is the login key of the host (hosttest proofs).
	Login hosttest.Key
	// Public and Private are the static X25519 key pair.
	Public, Private [noiseKeyLen]byte
}

// NewNoiseKey returns a fresh login key and static X25519 key pair of a host. NewKey stays for the
// tests that never run a handshake.
func NewNoiseKey() NoiseKey {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		panic(err)
	}
	k := NoiseKey{Login: hosttest.NewKey(priv)}
	x, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		panic(err)
	}
	copy(k.Private[:], x.Bytes())
	copy(k.Public[:], x.PublicKey().Bytes())
	return k
}

// Ephemeral is the X25519 key pair of one handshake of the device. The device reports its hash as
// client_ephemeral_key_hash before the session is opened (15 section 23.2), so it exists before
// the handshake.
type Ephemeral struct {
	Public, Private [noiseKeyLen]byte
}

// NewEphemeral returns a fresh ephemeral key pair.
func NewEphemeral() Ephemeral {
	x, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		panic(err)
	}
	var e Ephemeral
	copy(e.Private[:], x.Bytes())
	copy(e.Public[:], x.PublicKey().Bytes())
	return e
}

// Hash returns the SHA-256 of the public key in base64url without padding: the
// client_ephemeral_key_hash of openTerminalSession and the claim cek of the session token.
func (e Ephemeral) Hash() string { return ephemeralHash(e.Public[:]) }

// ephemeralHash is the hash of 24 section 5.3 over an X25519 public key.
func ephemeralHash(public []byte) string {
	sum := sha256.Sum256(public)
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// Prologue returns "bothy-terminal/1" || session ID || host ID (24 section 5.2): the 16 bytes of
// each ID in the octet order of RFC 9562.
func Prologue(session, host ids.ID) []byte {
	p := make([]byte, 0, len(prologueLabel)+2*len(session))
	p = append(p, prologueLabel...)
	p = append(p, session[:]...)
	return append(p, host[:]...)
}

// sizeFrame encodes a frame of typ with columns and rows (big-endian, 2 bytes each).
func sizeFrame(typ byte, cols, rows int) []byte {
	b := []byte{typ, 0, 0, 0, 0}
	binary.BigEndian.PutUint16(b[1:3], uint16(cols)) //nolint:gosec // G115: test sizes of 1 to 65535
	binary.BigEndian.PutUint16(b[3:5], uint16(rows)) //nolint:gosec // G115: test sizes of 1 to 65535
	return b
}

// StartFrame returns the frame start with cols and rows (24 section 5.4).
func StartFrame(cols, rows int) []byte { return sizeFrame(FrameStart, cols, rows) }

// ResizeFrame returns the frame resize with cols and rows.
func ResizeFrame(cols, rows int) []byte { return sizeFrame(FrameResize, cols, rows) }

// DataFrame returns the frame data with payload (1 to 65518 bytes of input or output).
func DataFrame(payload []byte) []byte { return append([]byte{FrameData}, payload...) }

// CloseFrame returns the frame close (no content).
func CloseFrame() []byte { return []byte{FrameClose} }

// NoiseParams are the inputs of the device side of the handshake.
type NoiseParams struct {
	// HostStatic is the static Noise public key of the host (noise_public_key of the grant).
	HostStatic [noiseKeyLen]byte
	// HostID goes into the prologue with the session ID of the device stream.
	HostID ids.ID
	// Ephemeral is the key pair whose hash the device reported when it opened the session.
	Ephemeral Ephemeral
	// Token is the session token, the payload of message 1.
	Token string
}

// NoiseStream is the device side of the encrypted stream after the handshake. Send and RecvFrame
// may run in two goroutines; neither is safe for concurrent calls of itself.
type NoiseStream struct {
	dev        *Device
	send, recv *noise.CipherState
	// HostPayload is the decrypted payload of message 2.
	HostPayload []byte
}

// Handshake runs the initiator of Noise_NK_25519_ChaChaPoly_SHA256 on the device stream: message 1
// (-> e, es) carries the session token, message 2 (<- e, ee) is read and decrypted. Errors: the
// stream, or a message 2 that does not authenticate (wrong host key or prologue).
func (dv *Device) Handshake(ctx context.Context, p NoiseParams) (*NoiseStream, error) {
	hs, err := noise.NewHandshakeState(noise.Config{
		CipherSuite: suite,
		// flynn/noise draws the ephemeral key from Random; this reader yields exactly the reported one.
		Random:     bytes.NewReader(bytes.Clone(p.Ephemeral.Private[:])),
		Pattern:    noise.HandshakeNK,
		Initiator:  true,
		Prologue:   Prologue(dv.Session, p.HostID),
		PeerStatic: bytes.Clone(p.HostStatic[:]),
	})
	if err != nil {
		return nil, err
	}
	m1, _, _, err := hs.WriteMessage(nil, []byte(p.Token))
	if err != nil {
		return nil, err
	}
	if err := dv.Send(ctx, m1); err != nil {
		return nil, err
	}
	m2, err := dv.Recv(ctx)
	if err != nil {
		return nil, fmt.Errorf("fakehost: message 2: %w", err)
	}
	payload, send, recv, err := hs.ReadMessage(nil, m2)
	if err != nil || send == nil || recv == nil {
		return nil, fmt.Errorf("fakehost: message 2 does not authenticate: %w", err)
	}
	return &NoiseStream{dev: dv, send: send, recv: recv, HostPayload: payload}, nil
}

// Send encrypts frame (one frame of 24 section 5.4) and sends it as one binary message.
func (s *NoiseStream) Send(ctx context.Context, frame []byte) error {
	m, err := s.send.Encrypt(nil, nil, frame)
	if err != nil {
		return err
	}
	return s.dev.Send(ctx, m)
}

// RecvFrame receives and decrypts the next transport message and returns its frame type and
// payload. Errors: the stream, an empty frame, or a message that does not authenticate.
func (s *NoiseStream) RecvFrame(ctx context.Context) (byte, []byte, error) {
	m, err := s.dev.Recv(ctx)
	if err != nil {
		return 0, nil, err
	}
	f, err := s.recv.Decrypt(nil, nil, m)
	if err != nil {
		return 0, nil, err
	}
	if len(f) == 0 {
		return 0, nil, errors.New("fakehost: empty frame")
	}
	return f[0], f[1:], nil
}

// UseNoise gives the host its static Noise key pair for the script NoiseAccept.
func (h *Host) UseNoise(k NoiseKey) {
	h.mu.Lock()
	h.noise = &k
	h.mu.Unlock()
}

// Plain returns the frames the host decrypted from the device of session, in order (the positive
// control of TM-T06: the content arrives in clear behind the encryption).
func (h *Host) Plain(session ids.ID) [][]byte {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([][]byte, 0, len(h.plain[session]))
	for _, f := range h.plain[session] {
		out = append(out, bytes.Clone(f))
	}
	return out
}

// hostPayload is the payload of message 2 (24 section 5.2).
func hostPayload() []byte {
	b, _ := json.Marshal(map[string]any{"v": 1, "host_version": "1.0.0", "profile": "shell"})
	return b
}

// noiseAccept runs NoiseAccept: the responder of the handshake reads message 1, whose payload is
// the session token, verifies it like Accept and checks its claim cek against the initiator's
// ephemeral key (24 section 5.3), answers message 2, sends session.started on the frame start and
// then answers every data frame with Script.Output, or with the same data when Output is nil.
// The frame close makes it send session.ended with client_closed. A failed step closes the data
// connection with 4104.
func (h *Host) noiseAccept(dc *DataConn, s Script) {
	h.mu.Lock()
	nk := h.noise
	h.mu.Unlock()
	if nk == nil {
		h.t.Error(errNoiseKey)
		_ = dc.ws.Close(4104, "handshake_failed")
		return
	}
	hs, err := noise.NewHandshakeState(noise.Config{
		CipherSuite:   suite,
		Random:        rand.Reader,
		Pattern:       noise.HandshakeNK,
		Prologue:      Prologue(dc.Session, h.ID),
		StaticKeypair: noise.DHKey{Private: bytes.Clone(nk.Private[:]), Public: bytes.Clone(nk.Public[:])},
	})
	if err != nil {
		_ = dc.ws.Close(4104, "handshake_failed")
		return
	}
	m1, err := dc.Recv(h.ctx)
	if err != nil {
		return
	}
	tok, _, _, err := hs.ReadMessage(nil, m1)
	if err != nil {
		_ = dc.ws.Close(4104, "handshake_failed")
		return
	}
	h.mu.Lock()
	h.tokens = append(h.tokens, string(tok))
	h.mu.Unlock()
	if err := h.VerifyToken(string(tok), dc.Session); err != nil || tokenCEK(string(tok)) != ephemeralHash(hs.PeerEphemeral()) {
		_ = dc.ws.Close(4104, "token_invalid")
		return
	}
	m2, recv, send, err := hs.WriteMessage(nil, hostPayload())
	if err != nil || recv == nil || send == nil {
		_ = dc.ws.Close(4104, "handshake_failed")
		return
	}
	if err := dc.Send(h.ctx, m2); err != nil {
		return
	}
	h.noiseFrames(dc, s, recv, send)
}

// noiseFrames is the transport part of NoiseAccept.
func (h *Host) noiseFrames(dc *DataConn, s Script, recv, send *noise.CipherState) {
	for first := true; ; first = false {
		m, err := dc.Recv(h.ctx)
		if err != nil {
			return
		}
		f, err := recv.Decrypt(nil, nil, m)
		if err != nil || len(f) == 0 || first != (f[0] == FrameStart) {
			_ = dc.ws.Close(4104, "handshake_failed")
			return
		}
		h.mu.Lock()
		h.plain[dc.Session] = append(h.plain[dc.Session], bytes.Clone(f))
		h.mu.Unlock()
		switch f[0] {
		case FrameStart:
			_ = h.Send("session.started", map[string]any{"session_id": dc.Session.String()})
		case FrameData:
			out := f
			if s.Output != nil {
				out = DataFrame(s.Output)
			}
			c, err := send.Encrypt(nil, nil, out)
			if err != nil || dc.Send(h.ctx, c) != nil {
				return
			}
		case FrameClose:
			_ = h.Send("session.ended", map[string]any{"session_id": dc.Session.String(), "close_reason": "client_closed"})
			return
		}
	}
}

// tokenCEK returns the claim cek of a compact session token, or "" when it has none.
func tokenCEK(tok string) string {
	j, err := jose.ParseCompactJWS(tok, []string{jose.AlgEdDSA}, tokenTyp)
	if err != nil {
		return ""
	}
	var c struct {
		Cek string `json:"cek"`
	}
	if json.Unmarshal(j.Payload, &c) != nil {
		return ""
	}
	return c.Cek
}

// WireMessage is one binary WebSocket message a fake sent or received: what passed the relay.
type WireMessage struct {
	// Sent is true for a message the fake sent, false for one it received.
	Sent bool
	// Bytes are the message as on the wire.
	Bytes []byte
}

// cloneWire returns a deep copy of msgs.
func cloneWire(msgs []WireMessage) []WireMessage {
	out := make([]WireMessage, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, WireMessage{Sent: m.Sent, Bytes: bytes.Clone(m.Bytes)})
	}
	return out
}

// Wire returns every binary message the data connection sent and received so far, in order.
func (dc *DataConn) Wire() []WireMessage {
	dc.mu.Lock()
	defer dc.mu.Unlock()
	return cloneWire(dc.wire)
}

// Wire returns every binary message the device stream sent and received so far, in order.
func (dv *Device) Wire() []WireMessage {
	dv.mu.Lock()
	defer dv.mu.Unlock()
	return cloneWire(dv.wire)
}

// sendBinary writes m as one binary message on ws and records it in wire under mu.
func sendBinary(ctx context.Context, ws *websocket.Conn, mu *gosync.Mutex, wire *[]WireMessage, m []byte) error {
	mu.Lock()
	*wire = append(*wire, WireMessage{Sent: true, Bytes: bytes.Clone(m)})
	mu.Unlock()
	return ws.Write(ctx, websocket.MessageBinary, m)
}
