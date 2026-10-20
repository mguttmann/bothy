// SPDX-License-Identifier: Apache-2.0

package device

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"sync"
)

// The frame types of 24 5.4 (protocol constants, written literally from the chapter).
const (
	FrameStart       byte = 0x01
	FrameData        byte = 0x02
	FrameResize      byte = 0x03
	FrameClose       byte = 0x04
	FrameExit        byte = 0x05
	FrameReplayBegin byte = 0x06
	FrameReplayEnd   byte = 0x07
)

// StartFrame encodes start with the size of the terminal.
func StartFrame(cols, rows uint16) []byte { return sizeFrame(FrameStart, cols, rows) }

// ResizeFrame encodes resize with the size of the terminal.
func ResizeFrame(cols, rows uint16) []byte { return sizeFrame(FrameResize, cols, rows) }

// DataFrame encodes data with the input bytes.
func DataFrame(b []byte) []byte { return append([]byte{FrameData}, b...) }

// CloseFrame encodes close.
func CloseFrame() []byte { return []byte{FrameClose} }

// sizeFrame encodes a frame with columns and rows (two big-endian uint16).
func sizeFrame(t byte, cols, rows uint16) []byte {
	out := []byte{t, 0, 0, 0, 0}
	binary.BigEndian.PutUint16(out[1:], cols)
	binary.BigEndian.PutUint16(out[3:], rows)
	return out
}

// ErrNoMessage2 reports a handshake whose stream ended before message 2.
var ErrNoMessage2 = errors.New("device: the stream ended before message 2")

// Conn is one opening or attachment of a session as the device sees it: one sidecar (one
// ephemeral key) and one device stream. The output of the host is collected in memory.
type Conn struct {
	// Sidecar runs Noise for this connection; Stream is the device stream.
	Sidecar *Sidecar
	Stream  *Stream
	// CEK is the client_ephemeral_key_hash of the ephemeral key of this connection.
	CEK string
	// Message1 is the Noise message 1 this connection sent (the relay view of TM-T03).
	Message1 []byte

	mu          sync.Mutex
	cond        *sync.Cond
	output      []byte
	replayBegin int
	replayEnd   int
	exit        *int32
	readErr     error
	reading     bool
}

// NewConn starts a sidecar of bin and draws the ephemeral key, so CEK is known before
// openTerminalSession or attachTerminalSession.
//
// Errors: the sidecar could not start or draw the key.
func NewConn(bin string) (*Conn, error) {
	sc, err := StartSidecar(bin)
	if err != nil {
		return nil, err
	}
	_, cek, err := sc.Keypair()
	if err != nil {
		_ = sc.Close()
		return nil, err
	}
	c := &Conn{Sidecar: sc, CEK: cek}
	c.cond = sync.NewCond(&c.mu)
	return c, nil
}

// Handshake runs start of the sidecar with the host's static key and the IDs, sends message 1
// with payload (the session token) and reads message 2. The message 1 is kept in Message1.
//
// Errors: a SidecarError, or ErrNoMessage2 when the stream ended first (CloseCode tells why).
func (c *Conn) Handshake(ctx context.Context, hostStatic []byte, sessionID, hostID string, payload []byte) error {
	if _, _, err := c.Sidecar.Start(hostStatic, sessionID, hostID); err != nil {
		return err
	}
	m1, err := c.Sidecar.Message1(payload)
	if err != nil {
		return err
	}
	c.Message1 = m1
	if err := c.Stream.Send(ctx, m1); err != nil {
		return ErrNoMessage2
	}
	m2, err := c.Stream.Recv(ctx)
	if err != nil {
		return ErrNoMessage2
	}
	if _, err := c.Sidecar.Message2(m2); err != nil {
		return err
	}
	c.startReader()
	return nil
}

// SendFrame seals frame and sends it.
func (c *Conn) SendFrame(ctx context.Context, frame []byte) error {
	m, err := c.Sidecar.Seal(frame)
	if err != nil {
		return err
	}
	return c.Stream.Send(ctx, m)
}

// SendRaw sends m unchanged on the stream (a recorded message of another connection, TM-T03).
func (c *Conn) SendRaw(ctx context.Context, m []byte) error { return c.Stream.Send(ctx, m) }

// startReader decrypts every later message of the host and collects the frames.
func (c *Conn) startReader() {
	c.mu.Lock()
	if c.reading {
		c.mu.Unlock()
		return
	}
	c.reading = true
	c.mu.Unlock()
	go func() {
		for {
			m, err := c.Stream.Recv(context.Background())
			if err != nil {
				c.mu.Lock()
				c.readErr = err
				c.cond.Broadcast()
				c.mu.Unlock()
				return
			}
			f, err := c.Sidecar.Open(m)
			c.mu.Lock()
			switch {
			case err != nil:
				c.readErr = err
			case len(f) == 0:
			case f[0] == FrameData:
				c.output = append(c.output, f[1:]...)
			case f[0] == FrameReplayBegin:
				c.replayBegin++
			case f[0] == FrameReplayEnd:
				c.replayEnd++
			case f[0] == FrameExit && len(f) == 5:
				code := int32(binary.BigEndian.Uint32(f[1:])) //nolint:gosec // G115: two's complement is the wire format
				c.exit = &code
			}
			c.cond.Broadcast()
			c.mu.Unlock()
			if err != nil {
				return
			}
		}
	}()
}

// WaitOutput waits until the collected output contains want or ctx ends.
//
// Errors: ctx ended, or the stream ended first; the error never carries output.
func (c *Conn) WaitOutput(ctx context.Context, want []byte) error {
	stop := context.AfterFunc(ctx, func() { c.mu.Lock(); c.cond.Broadcast(); c.mu.Unlock() })
	defer stop()
	c.mu.Lock()
	defer c.mu.Unlock()
	for !bytes.Contains(c.output, want) {
		if c.readErr != nil {
			return fmt.Errorf("device: the stream ended before the expected output (%d bytes seen)", len(c.output))
		}
		if ctx.Err() != nil {
			return fmt.Errorf("device: expected output not seen (%d bytes seen): %w", len(c.output), ctx.Err())
		}
		c.cond.Wait()
	}
	return nil
}

// Output returns a copy of the collected output (only for canary scans in memory, never printed).
func (c *Conn) Output() []byte {
	c.mu.Lock()
	defer c.mu.Unlock()
	return bytes.Clone(c.output)
}

// Replay returns how many replay_begin and replay_end frames the host sent.
func (c *Conn) Replay() (begin, end int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.replayBegin, c.replayEnd
}

// Exit returns the exit code of an exit frame, if one came.
func (c *Conn) Exit() (int32, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.exit == nil {
		return 0, false
	}
	return *c.exit, true
}

// Close ends the stream and the sidecar.
func (c *Conn) Close() {
	if c.Stream != nil {
		_ = c.Stream.Close()
	}
	_ = c.Sidecar.Close()
}
