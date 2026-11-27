// SPDX-License-Identifier: Apache-2.0

package session

import (
	"context"
	"net/http"
	"sync/atomic"
	"time"
)

// Stream is an outbound data connection for another session kind than terminal: the computer
// sessions of docs/spec/27-computer-use.md use the same channel rules as hostData (hello and
// identify as text, then one Noise message per binary message, a text frame closes with 4002), so
// they reuse this connection instead of a copy. It is safe for concurrent use.
type Stream struct {
	dc     *dataConn
	queued atomic.Int64
}

// StreamIdentify is the identify of a data connection (schema HostIdentify with session_id).
type StreamIdentify struct {
	HostID, HostVersion, SessionID string
}

// DialStream opens the data connection url with hc, reads hello, and sends identify with the proof
// that sign returns for the challenge of hello (control.Proof). Errors: the dial, a hello without
// the values of the server, or a failed identify.
func DialStream(ctx context.Context, hc *http.Client, url string, id StreamIdentify, sign func(challenge string) string) (*Stream, error) {
	dc, err := dialData(ctx, hc, url, identifyData{HostID: id.HostID, HostVersion: id.HostVersion, SessionID: id.SessionID}, sign)
	if err != nil {
		return nil, err
	}
	return &Stream{dc: dc}, nil
}

// In returns the binary messages of the server in order; it is closed when the connection ends.
func (s *Stream) In() <-chan []byte { return s.dc.in }

// Done is closed when the connection ended.
func (s *Stream) Done() <-chan struct{} { return s.dc.done }

// Send queues one binary message and waits while the queue is full, until ctx ends or the
// connection ends; unlike a terminal stream, a computer stream carries image chunks in bursts and
// must not drop the connection for a full queue. It returns false when the message was not queued.
func (s *Stream) Send(ctx context.Context, m []byte) bool {
	select {
	case s.dc.out <- m:
		s.queued.Add(1)
		return true
	case <-s.dc.done:
		return false
	case <-ctx.Done():
		return false
	}
}

// Drain waits until the writer wrote every message queued with Send, the connection ended or ctx
// ended, so a last frame before Close reaches the server.
func (s *Stream) Drain(ctx context.Context) {
	for s.dc.written.Load() < s.queued.Load() {
		select {
		case <-s.dc.done:
			return
		case <-ctx.Done():
			return
		case <-time.After(drainPoll):
		}
	}
}

// drainPoll is the poll interval of Drain; a constant of the wait, not a product window.
const drainPoll = 5 * time.Millisecond

// Close closes the connection with code and reason once.
func (s *Stream) Close(code int, reason string) { s.dc.close(code, reason) }

// CloseCode returns the close code of the server (-1 for a lost connection) once Done is closed.
func (s *Stream) CloseCode() int { return s.dc.closeCode() }

// Keys returns the jwks of the hello of this connection (purpose terminal).
func (s *Stream) Keys() []map[string]any { return s.dc.hello.JWKS.Keys }

// Leeway returns auth.clock_leeway of the hello of this connection.
func (s *Stream) Leeway() time.Duration { return s.dc.leeway() }
