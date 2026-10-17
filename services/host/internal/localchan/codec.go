// SPDX-License-Identifier: Apache-2.0

// Package localchan is the local channel between the service user and each PTY helper, and the
// admin endpoint of the running service (docs/spec/24-terminal.md 2.2, docs/DECISIONS.md
// D-M2-42-11). Endpoints are Unix sockets or named pipes with PIPE_REJECT_REMOTE_CLIENTS, never a
// TCP or UDP listener.
//
// Peer check: with socket activation the operating system reports the service manager as the peer
// of a connecting client, so on Linux and macOS the helper creates a socketpair, passes one end with
// SCM_RIGHTS over the accepted connection, and every further message runs over that pair; the
// service reads the peer of its end (SO_PEERCRED, LOCAL_PEERCRED and LOCAL_PEERPID) and compares
// user ID and process ID with the session user and the main PID the service manager reports. The
// helper accepts only the service user as client. Before both checks pass, no message flows: the
// helper speaks only after the first message of the service, and the service sends it only after
// its check.
//
// Codec: a 4-byte big-endian length, one kind byte and the body. Kind 0x00 is a JSON control
// message {"op", "id", "d"}; kind 0x01 is reserved for the session frames of M2-45 and refused
// here; any other kind closes the channel. The length counts kind and body and is at most
// frame.MaxLen plus the kind byte (a protocol constant).
package localchan

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/OWNER/bothy/services/host/internal/frame"
)

// Message kinds.
const (
	KindControl byte = 0x00
	KindFrame   byte = 0x01
)

// lenSize is the size of the length prefix.
const lenSize = 4

// MaxLen is the largest value of the length prefix: the kind byte and a body of at most
// frame.MaxLen bytes.
const MaxLen = frame.MaxLen + 1

// Ops of this ticket.
const (
	OpHello       = "hello"
	OpCheckRun    = "check.run"
	OpCheckResult = "check.result"
	OpScanRun     = "scan.run"
	OpScanResult  = "scan.result"
	OpDoctorRun   = "doctor.run"
	OpDoctorRes   = "doctor.result"
	OpPairRun     = "pair.run"
	OpPairRes     = "pair.result"
	OpError       = "error"
)

// ProtocolVersion is the version the helper announces in hello.
const ProtocolVersion = 1

// Message is a control message.
type Message struct {
	Op string          `json:"op"`
	ID string          `json:"id"`
	D  json.RawMessage `json:"d"`
}

// Codec errors; each closes the channel.
var (
	ErrTooLarge    = errors.New("localchan: message too large")
	ErrUnknownKind = errors.New("localchan: unknown message kind")
	ErrFrameKind   = errors.New("localchan: session frames are not accepted on this channel yet")
	ErrMalformed   = errors.New("localchan: malformed message")
)

// Encode returns the wire form of m.
func Encode(m Message) ([]byte, error) {
	if m.D == nil {
		m.D = json.RawMessage("{}")
	}
	body, err := json.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("localchan: %w", err)
	}
	if len(body)+1 > MaxLen {
		return nil, ErrTooLarge
	}
	out := make([]byte, lenSize+1, lenSize+1+len(body))
	binary.BigEndian.PutUint32(out, uint32(len(body)+1)) //nolint:gosec // bounded by MaxLen above
	out[lenSize] = KindControl
	return append(out, body...), nil
}

// WriteMessage writes m to w.
func WriteMessage(w io.Writer, m Message) error {
	b, err := Encode(m)
	if err != nil {
		return err
	}
	_, err = w.Write(b)
	return err
}

// ReadMessage reads one message from r.
func ReadMessage(r io.Reader) (Message, error) {
	var hdr [lenSize]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return Message{}, err
	}
	n := binary.BigEndian.Uint32(hdr[:])
	if n == 0 || n > MaxLen {
		return Message{}, ErrTooLarge
	}
	buf := make([]byte, n)
	if _, err := io.ReadFull(r, buf); err != nil {
		return Message{}, err
	}
	return decodeBody(buf)
}

// DecodeMessage decodes exactly one message from b; trailing bytes are malformed.
func DecodeMessage(b []byte) (Message, error) {
	rd := bytes.NewReader(b)
	m, err := ReadMessage(rd)
	if err != nil {
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			return Message{}, ErrMalformed
		}
		return Message{}, err
	}
	if rd.Len() != 0 {
		return Message{}, ErrMalformed
	}
	return m, nil
}

func decodeBody(buf []byte) (Message, error) {
	switch buf[0] {
	case KindControl:
	case KindFrame:
		return Message{}, ErrFrameKind
	default:
		return Message{}, ErrUnknownKind
	}
	var m Message
	dec := json.NewDecoder(bytes.NewReader(buf[1:]))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&m); err != nil || m.Op == "" || dec.More() {
		return Message{}, ErrMalformed
	}
	return m, nil
}
