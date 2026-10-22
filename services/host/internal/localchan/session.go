// SPDX-License-Identifier: Apache-2.0

package localchan

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// Session ops of kind 0x00 between the service user and a PTY helper (M2-45, docs/spec/24-terminal.md
// 2.2): the service sends pty.start, pty.resize and pty.stop; the helper answers pty.start with
// pty.started or pty.failed and reports pty.exited once the process of a session ended. They run on
// a session channel that stays open; when it breaks, the helper ends every PTY started on it
// (D-W-90).
const (
	OpPTYStart   = "pty.start"
	OpPTYResize  = "pty.resize"
	OpPTYStop    = "pty.stop"
	OpPTYStarted = "pty.started"
	OpPTYExited  = "pty.exited"
	OpPTYFailed  = "pty.failed"
)

// SessionIDLen is the length of the session ID at the start of a kind 0x01 body: the 16 bytes of
// noise.ParseID.
const SessionIDLen = 16

// MaxFrameLen is the longest frame a kind 0x01 body carries after the session ID.
const MaxFrameLen = MaxLen - 1 - SessionIDLen

// MaxData is the largest payload of a data frame in a kind 0x01 body: the helper reads PTY output in
// chunks of at most this size.
const MaxData = MaxFrameLen - 1

// PTYStart is the body of pty.start. Program and Args are set only for a profile of kind command
// (the program HD11 checked, from the configuration); WorkdirIndex and Workdir only for kind
// claude (the entry wdi names in the list of the session user after HD7, which the helper resolves
// again and compares with the identity it reported); APIKey only for claude with organization,
// read by the service from claude_api_key for this one start. The client of the server never
// supplies any of them.
type PTYStart struct {
	SessionID    string   `json:"session_id"`
	Profile      string   `json:"profile"`
	Kind         string   `json:"kind"`
	Declaration  string   `json:"declaration"`
	Program      string   `json:"program,omitempty"`
	Args         []string `json:"args,omitempty"`
	WorkdirIndex *int     `json:"workdir_index,omitempty"`
	Workdir      string   `json:"workdir,omitempty"`
	Cols         uint16   `json:"cols"`
	Rows         uint16   `json:"rows"`
	APIKey       string   `json:"api_key,omitempty"`
}

// PTYResize is the body of pty.resize.
type PTYResize struct {
	SessionID string `json:"session_id"`
	Cols      uint16 `json:"cols"`
	Rows      uint16 `json:"rows"`
}

// PTYStop is the body of pty.stop: end the PTY with its whole tree at once.
type PTYStop struct {
	SessionID string `json:"session_id"`
}

// PTYStarted is the body of pty.started.
type PTYStarted struct {
	SessionID string `json:"session_id"`
}

// PTYExited is the body of pty.exited: the exit code of the process (Unix: the status, or 128 plus
// the signal number; Windows: the exit code).
type PTYExited struct {
	SessionID string `json:"session_id"`
	ExitCode  int    `json:"exit_code"`
}

// PTYFailed is the body of pty.failed: the PTY did not start; CloseDetail is a code of 24 10.3
// (pty_failed or shared_location).
type PTYFailed struct {
	SessionID   string `json:"session_id"`
	CloseDetail string `json:"close_detail"`
}

// Packet is one message of a session channel: a control message, or (IsFrame) the frame of one
// session.
type Packet struct {
	Msg       Message
	IsFrame   bool
	SessionID [SessionIDLen]byte
	// Frame holds the bytes of one frame (M2-38 encoding); the receiver decodes it.
	Frame []byte
}

// ErrFrameTooLarge is a frame that does not fit a kind 0x01 body.
var ErrFrameTooLarge = errors.New("localchan: frame too large for the channel")

// EncodeFrame returns the wire form of a kind 0x01 message: sessionID followed by the frame bytes.
func EncodeFrame(sessionID [SessionIDLen]byte, f []byte) ([]byte, error) {
	if len(f) == 0 || len(f) > MaxFrameLen {
		return nil, ErrFrameTooLarge
	}
	n := 1 + SessionIDLen + len(f)
	out := make([]byte, lenSize, lenSize+n)
	binary.BigEndian.PutUint32(out, uint32(n)) //nolint:gosec // bounded by MaxFrameLen above
	out = append(out, KindFrame)
	out = append(out, sessionID[:]...)
	return append(out, f...), nil
}

// ReadPacket reads one message of either kind from r. Errors: ErrTooLarge, ErrUnknownKind,
// ErrMalformed or the error of r.
func ReadPacket(r io.Reader) (Packet, error) {
	var hdr [lenSize]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return Packet{}, err
	}
	n := binary.BigEndian.Uint32(hdr[:])
	if n == 0 || n > MaxLen {
		return Packet{}, ErrTooLarge
	}
	buf := make([]byte, n)
	if _, err := io.ReadFull(r, buf); err != nil {
		return Packet{}, err
	}
	if buf[0] == KindFrame {
		if len(buf) < 1+SessionIDLen+1 {
			return Packet{}, ErrMalformed
		}
		var p Packet
		p.IsFrame = true
		copy(p.SessionID[:], buf[1:1+SessionIDLen])
		p.Frame = buf[1+SessionIDLen:]
		return p, nil
	}
	m, err := decodeBody(buf)
	if err != nil {
		return Packet{}, err
	}
	return Packet{Msg: m}, nil
}

// ReadPacket reads the next message of either kind; only the reader goroutine of a session channel
// calls it.
func (c *Conn) ReadPacket() (Packet, error) { return ReadPacket(c.rwc) }

// Send writes the control message op with body d; safe for concurrent use with SendFrame.
func (c *Conn) Send(op, id string, d any) error {
	body, err := json.Marshal(d)
	if err != nil {
		return fmt.Errorf("localchan: %w", err)
	}
	b, err := Encode(Message{Op: op, ID: id, D: body})
	if err != nil {
		return err
	}
	c.wmu.Lock()
	defer c.wmu.Unlock()
	_, err = c.rwc.Write(b)
	return err
}

// SendFrame writes the frame bytes f of the session sessionID; safe for concurrent use with Send.
func (c *Conn) SendFrame(sessionID [SessionIDLen]byte, f []byte) error {
	b, err := EncodeFrame(sessionID, f)
	if err != nil {
		return err
	}
	c.wmu.Lock()
	defer c.wmu.Unlock()
	_, err = c.rwc.Write(b)
	return err
}
