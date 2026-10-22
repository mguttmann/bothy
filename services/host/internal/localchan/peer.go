// SPDX-License-Identifier: Apache-2.0

package localchan

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"sync"
	"sync/atomic"
)

// Peer is the process at the other end of a channel as the operating system reports it.
type Peer struct {
	// UID is the uid (Linux, macOS) or the user SID of the process token (Windows).
	UID string
	PID int
}

// Expect is what the service expects at the other end of the channel of a session user.
type Expect struct {
	UID string
	PID int
}

// ErrPeerMismatch is the failure of a peer check; nothing was sent on the channel.
var ErrPeerMismatch = errors.New("localchan: peer check failed")

// VerifyPeer compares got with want; any difference is ErrPeerMismatch. The error names neither
// IDs nor paths.
func VerifyPeer(got Peer, want Expect) error {
	if got.UID != want.UID || got.PID != want.PID || got.PID <= 0 {
		return ErrPeerMismatch
	}
	return nil
}

// Conn is a verified channel: one request at a time.
type Conn struct {
	rwc io.ReadWriteCloser
	mu  sync.Mutex
	// wmu serialises Send and SendFrame of a session channel, which several goroutines write.
	wmu  sync.Mutex
	next atomic.Uint64
	// Peer is the verified peer.
	Peer Peer
}

// NewConn wraps a verified stream.
func NewConn(rwc io.ReadWriteCloser, peer Peer) *Conn { return &Conn{rwc: rwc, Peer: peer} }

// Close closes the channel.
func (c *Conn) Close() error { return c.rwc.Close() }

// Read reads the next message.
func (c *Conn) Read() (Message, error) { return ReadMessage(c.rwc) }

// Write sends m.
func (c *Conn) Write(m Message) error { return WriteMessage(c.rwc, m) }

// RemoteError is an error answer of the other end; its text is a code.
type RemoteError struct{ Code string }

func (e *RemoteError) Error() string { return "localchan: remote error " + e.Code }

// Call sends op with d and decodes the answer with op want into out. It closes the channel when
// ctx ends, so a hanging peer cannot block the caller.
func (c *Conn) Call(ctx context.Context, op string, d any, want string, out any) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	body, err := json.Marshal(d)
	if err != nil {
		return fmt.Errorf("localchan: %w", err)
	}
	id := strconv.FormatUint(c.next.Add(1), 10)
	stop := context.AfterFunc(ctx, func() { _ = c.rwc.Close() })
	defer stop()
	if err := c.Write(Message{Op: op, ID: id, D: body}); err != nil {
		return ctxErr(ctx, err)
	}
	m, err := c.Read()
	if err != nil {
		return ctxErr(ctx, err)
	}
	if m.Op == OpError {
		var e struct {
			Code string `json:"code"`
		}
		_ = json.Unmarshal(m.D, &e)
		return &RemoteError{Code: e.Code}
	}
	if m.Op != want || m.ID != id {
		_ = c.rwc.Close()
		return ErrMalformed
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(m.D, out); err != nil {
		return ErrMalformed
	}
	return nil
}

func ctxErr(ctx context.Context, err error) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return err
}

// Handler answers one request: it returns the op and body of the answer; an empty op closes the
// channel.
type Handler func(ctx context.Context, m Message) (op string, d any)

// ServeConn answers requests on c until the peer closes it or a message is malformed (then the
// channel closes).
func ServeConn(ctx context.Context, c *Conn, h Handler) error {
	defer func() { _ = c.Close() }()
	for {
		m, err := c.Read()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		op, d := h(ctx, m)
		if op == "" {
			return ErrMalformed // the handler refused the message: close the channel
		}
		body, err := json.Marshal(d)
		if err != nil {
			return fmt.Errorf("localchan: %w", err)
		}
		if err := c.Write(Message{Op: op, ID: m.ID, D: body}); err != nil {
			return err
		}
	}
}

// ErrorAnswer is the body of an op error.
func ErrorAnswer(code string) (string, any) {
	return OpError, map[string]string{"code": code}
}
