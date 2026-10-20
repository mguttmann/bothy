// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"

	"github.com/OWNER/bothy/services/host/internal/frame"
	"github.com/OWNER/bothy/services/host/internal/noise"
)

// Error kinds of the protocol itself; errors of internal/noise and internal/frame carry the Go
// name of their sentinel instead.
const (
	kindBadJSON    = "bad_json"
	kindUnknownOp  = "unknown_op"
	kindOutOfOrder = "out_of_order"
	kindBadField   = "bad_field"
	// kindInternal covers an error that wraps no known sentinel, for example a failed read of
	// crypto/rand; the device treats it like any other failure of the sidecar.
	kindInternal = "internal"
)

// sentinels maps every sentinel error of internal/noise and internal/frame to its Go name, the
// error kind the device sees. The names are unique across both packages.
var sentinels = []struct {
	name string
	err  error
}{
	{"ErrInvalidID", noise.ErrInvalidID},
	{"ErrInvalidKey", noise.ErrInvalidKey},
	{"ErrMessageTooLong", noise.ErrMessageTooLong},
	{"ErrHandshake", noise.ErrHandshake},
	{"ErrDecrypt", noise.ErrDecrypt},
	{"ErrBroken", noise.ErrBroken},
	{"ErrNonceExhausted", noise.ErrNonceExhausted},
	{"ErrState", noise.ErrState},
	{"ErrEmpty", frame.ErrEmpty},
	{"ErrUnknownType", frame.ErrUnknownType},
	{"ErrWrongDirection", frame.ErrWrongDirection},
	{"ErrMalformed", frame.ErrMalformed},
	{"ErrTooLong", frame.ErrTooLong},
}

// request is one line on stdin. Byte fields are pointers so that a missing field differs from
// an empty one (an empty payload is valid base64url).
type request struct {
	Op         string  `json:"op"`
	HostStatic *string `json:"host_static"`
	SessionID  *string `json:"session_id"`
	HostID     *string `json:"host_id"`
	Payload    *string `json:"payload"`
	Message    *string `json:"message"`
	Frame      *string `json:"frame"`
}

// response is one line on stdout. Byte fields are pointers so that an empty value is written
// as "" instead of being left out.
type response struct {
	OK              bool       `json:"ok"`
	EphemeralPublic *string    `json:"ephemeral_public,omitempty"`
	Cek             *string    `json:"cek,omitempty"`
	Message         *string    `json:"message,omitempty"`
	Payload         *string    `json:"payload,omitempty"`
	Frame           *string    `json:"frame,omitempty"`
	Error           *errorBody `json:"error,omitempty"`
}

// errorBody names the kind of a failure only; the text of a Go error is never written, so no
// answer carries more than the kind.
type errorBody struct {
	Kind string `json:"kind"`
}

// protocolError is a failure with a fixed kind of the protocol.
type protocolError string

func (e protocolError) Error() string { return string(e) }

// session is the state of one device session: one handshake, then one transport.
type session struct {
	// pending is the ephemeral key pair of keypair, which start then uses.
	pending   *noise.KeyPair
	initiator *noise.Initiator
	transport *noise.Transport
}

// fatalMessage is the only text serve writes to stderr; it carries no data of the session.
const fatalMessage = "noisedevice: input or output failed\n"

// serve answers one JSON line on out for each line read from in until EOF of in, then returns
// exit code 0. A failed read (other than EOF) or write gives exit code 1 and one fixed line on
// errw; nothing else is ever written to errw, so stderr carries no key, token or frame bytes.
func serve(in io.Reader, out, errw io.Writer) int {
	r := bufio.NewReader(in)
	w := bufio.NewWriter(out)
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	var s session
	for {
		line, err := r.ReadBytes('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			_, _ = io.WriteString(errw, fatalMessage)
			return 1
		}
		if len(line) == 0 && err != nil {
			return 0 // EOF after the last line
		}
		if encErr := encoder.Encode(s.handle(bytes.TrimSuffix(line, []byte("\n")))); encErr != nil || w.Flush() != nil {
			_, _ = io.WriteString(errw, fatalMessage)
			return 1
		}
		if err != nil {
			return 0 // a last line without a newline, then EOF
		}
	}
}

// handle runs one request line and returns its answer.
func (s *session) handle(line []byte) response {
	var req request
	dec := json.NewDecoder(bytes.NewReader(line))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil || dec.More() {
		return failure(protocolError(kindBadJSON))
	}
	var (
		resp response
		err  error
	)
	switch req.Op {
	case "keypair":
		resp, err = s.keypair()
	case "start":
		resp, err = s.start(req)
	case "message1":
		resp, err = s.message1(req)
	case "message2":
		resp, err = s.message2(req)
	case "seal":
		resp, err = s.seal(req)
	case "open":
		resp, err = s.open(req)
	default:
		err = protocolError(kindUnknownOp)
	}
	if err != nil {
		return failure(err)
	}
	resp.OK = true
	return resp
}

// failure returns the error answer for err.
func failure(err error) response {
	return response{Error: &errorBody{Kind: kindOf(err)}}
}

// kindOf returns the protocol kind of err or the Go name of the sentinel it wraps.
func kindOf(err error) string {
	var pe protocolError
	if errors.As(err, &pe) {
		return string(pe)
	}
	for _, s := range sentinels {
		if errors.Is(err, s.err) {
			return s.name
		}
	}
	return kindInternal
}

// start draws a fresh ephemeral key pair from crypto/rand and builds the initiator; the answer
// carries the ephemeral public key and its client_ephemeral_key_hash for openTerminalSession.
func (s *session) start(req request) (response, error) {
	if s.initiator != nil {
		return response{}, protocolError(kindOutOfOrder)
	}
	hostStatic, err := field(req.HostStatic)
	if err != nil {
		return response{}, err
	}
	if req.SessionID == nil || req.HostID == nil {
		return response{}, protocolError(kindBadField)
	}
	if len(hostStatic) != noise.KeyLen {
		return response{}, noise.ErrInvalidKey
	}
	sessionID, err := noise.ParseID(*req.SessionID)
	if err != nil {
		return response{}, err
	}
	hostID, err := noise.ParseID(*req.HostID)
	if err != nil {
		return response{}, err
	}
	var ephemeral noise.KeyPair
	if s.pending != nil {
		ephemeral = *s.pending
	} else if ephemeral, err = noise.GenerateKeyPair(nil); err != nil {
		return response{}, err
	}
	cek, err := noise.EphemeralKeyHash(ephemeral.Public[:])
	if err != nil {
		return response{}, err
	}
	init, err := noise.NewInitiator(noise.InitiatorConfig{
		HostStatic: [noise.KeyLen]byte(hostStatic), SessionID: sessionID, HostID: hostID, Ephemeral: ephemeral,
	})
	if err != nil {
		return response{}, err
	}
	s.initiator = init
	return response{EphemeralPublic: encoded(ephemeral.Public[:]), Cek: &cek}, nil
}

// keypair draws the ephemeral key pair before start: openTerminalSession needs its
// client_ephemeral_key_hash before the device knows the session ID of the prologue. Once only,
// and only before start.
func (s *session) keypair() (response, error) {
	if s.pending != nil || s.initiator != nil {
		return response{}, protocolError(kindOutOfOrder)
	}
	kp, err := noise.GenerateKeyPair(nil)
	if err != nil {
		return response{}, err
	}
	cek, err := noise.EphemeralKeyHash(kp.Public[:])
	if err != nil {
		return response{}, err
	}
	s.pending = &kp
	return response{EphemeralPublic: encoded(kp.Public[:]), Cek: &cek}, nil
}

// message1 writes message 1 with the payload, the compact session token.
func (s *session) message1(req request) (response, error) {
	if s.initiator == nil {
		return response{}, protocolError(kindOutOfOrder)
	}
	payload, err := field(req.Payload)
	if err != nil {
		return response{}, err
	}
	msg, err := s.initiator.WriteMessage1(payload)
	if err != nil {
		return response{}, err
	}
	return response{Message: encoded(msg)}, nil
}

// message2 reads message 2 of the host and keeps the transport. Without start, or after a
// completed handshake, the answer is out_of_order; message 2 before message 1 is refused by the
// initiator itself (ErrState).
func (s *session) message2(req request) (response, error) {
	if s.initiator == nil || s.transport != nil {
		return response{}, protocolError(kindOutOfOrder)
	}
	msg, err := field(req.Message)
	if err != nil {
		return response{}, err
	}
	payload, t, err := s.initiator.ReadMessage2(msg)
	if err != nil {
		return response{}, err
	}
	s.transport = t
	return response{Payload: encoded(payload)}, nil
}

// seal encrypts a frame the device may send; any other frame fails before it uses a nonce.
func (s *session) seal(req request) (response, error) {
	if s.transport == nil {
		return response{}, protocolError(kindOutOfOrder)
	}
	fb, err := field(req.Frame)
	if err != nil {
		return response{}, err
	}
	if _, err := frame.Decode(fb, frame.FromDevice); err != nil {
		return response{}, err
	}
	msg, err := s.transport.Encrypt(fb)
	if err != nil {
		return response{}, err
	}
	return response{Message: encoded(msg)}, nil
}

// open decrypts a transport message of the host and returns its frame, which must be one the
// host may send.
func (s *session) open(req request) (response, error) {
	if s.transport == nil {
		return response{}, protocolError(kindOutOfOrder)
	}
	msg, err := field(req.Message)
	if err != nil {
		return response{}, err
	}
	plain, err := s.transport.Decrypt(msg)
	if err != nil {
		return response{}, err
	}
	if _, err := frame.Decode(plain, frame.FromHost); err != nil {
		return response{}, err
	}
	return response{Frame: encoded(plain)}, nil
}

// field decodes a byte field: base64url without padding, present (an empty string is empty
// bytes).
func field(v *string) ([]byte, error) {
	if v == nil {
		return nil, protocolError(kindBadField)
	}
	b, err := base64.RawURLEncoding.Strict().DecodeString(*v)
	if err != nil {
		return nil, protocolError(kindBadField)
	}
	return b, nil
}

// encoded returns b as base64url without padding.
func encoded(b []byte) *string {
	s := base64.RawURLEncoding.EncodeToString(b)
	return &s
}
