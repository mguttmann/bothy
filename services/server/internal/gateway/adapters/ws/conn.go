// SPDX-License-Identifier: Apache-2.0

package ws

import (
	"context"
	"errors"
	"log/slog"
	gosync "sync"
	"time"

	"github.com/coder/websocket"

	"github.com/OWNER/bothy/services/server/internal/gateway/app"
	"github.com/OWNER/bothy/services/server/internal/gateway/domain"
)

// Errors of Send; the gateway ignores them, the connection is ending.
var (
	errClosing  = errors.New("gateway: connection closing")
	errOverflow = errors.New("gateway: outbound queue full")
)

// wsConn implements app.Conn on one WebSocket: Send encodes a frame of the gateway and queues it,
// Offer queues an entry the hub encoded, both in the bounded queue of the domain
// (sync.outbound_queue_max frames) that the writer goroutine drains in order; Close starts the
// close handshake in its own goroutine, so no caller waits on the network. Frames still queued at
// the close are dropped.
type wsConn struct {
	ws       *websocket.Conn
	snapshot SnapshotEncoder
	log      *slog.Logger
	done     chan struct{}
	closing  gosync.Once
	closed   gosync.WaitGroup
	// ready wakes the writer after a push (capacity 1).
	ready chan struct{}

	mu    gosync.Mutex
	queue *domain.Queue
	// space is closed and replaced whenever the queue loses entries, so WaitSpace wakes up.
	space chan struct{}
	// app is the gateway's connection, set right after Open; overflow closes through it, so the
	// close code reaches the gateway.
	app *app.Connection
	// beforeWrite is a test hook of the writer; nil in production.
	beforeWrite func()
}

// newConn returns the connection over ws with a queue of queueMax frames.
func newConn(ws *websocket.Conn, queueMax int, snapshot SnapshotEncoder, log *slog.Logger) *wsConn {
	return &wsConn{ws: ws, snapshot: snapshot, log: log, queue: domain.NewQueue(queueMax), done: make(chan struct{}),
		ready: make(chan struct{}, 1), space: make(chan struct{})}
}

// bind sets the gateway's connection of w.
func (w *wsConn) bind(c *app.Connection) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.app = c
}

// closeApp closes through the gateway's connection when there is one, else directly.
func (w *wsConn) closeApp(code domain.CloseCode) {
	w.mu.Lock()
	c := w.app
	w.mu.Unlock()
	if c != nil {
		c.Close(code, "")
		return
	}
	w.Close(code, code.Name())
}

// isDone reports whether the connection is closing.
func (w *wsConn) isDone() bool {
	select {
	case <-w.done:
		return true
	default:
		return false
	}
}

// push queues e and wakes the writer; false when the queue is full or the connection closing.
func (w *wsConn) push(e domain.Entry) bool {
	if w.isDone() {
		return false
	}
	w.mu.Lock()
	ok := w.queue.Push(e)
	w.mu.Unlock()
	if ok {
		select {
		case w.ready <- struct{}{}:
		default:
		}
	}
	return ok
}

// freed wakes every WaitSpace; w.mu is held.
func (w *wsConn) freed() {
	close(w.space)
	w.space = make(chan struct{})
}

// Send implements app.Conn: the frame is encoded now and queued; a full queue closes the
// connection with 4014 (04 section 7.2 item 5). A frame that cannot be encoded closes with 4000.
func (w *wsConn) Send(f any) error {
	if w.isDone() {
		return errClosing
	}
	b, err := encodeFrame(f, w.snapshot)
	if err != nil {
		w.log.LogAttrs(context.Background(), slog.LevelError, msgEncodeFailed, slog.String(logKeyErrorType, errorTypeEncode))
		w.closeApp(domain.CloseUnknownError)
		return err
	}
	if !w.push(domain.Entry{Frame: b, Class: domain.ClassControl}) {
		if w.isDone() {
			return errClosing
		}
		w.closeApp(domain.CloseSlowConsumer)
		return errOverflow
	}
	return nil
}

// Offer implements app.Conn.
func (w *wsConn) Offer(e domain.Entry) bool { return w.push(e) }

// WaitSpace implements app.Conn.
func (w *wsConn) WaitSpace(ctx context.Context) error {
	for {
		w.mu.Lock()
		roomy, ch := w.queue.Roomy(), w.space
		w.mu.Unlock()
		if roomy {
			return nil
		}
		select {
		case <-w.done:
			return errClosing
		case <-ctx.Done():
			return ctx.Err()
		case <-ch:
		}
	}
}

// Purge implements app.Conn.
func (w *wsConn) Purge(t domain.Targets) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.queue.Purge(t) > 0 {
		w.freed()
	}
}

// pop returns the oldest queued frame.
func (w *wsConn) pop() ([]byte, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	e, ok := w.queue.Pop()
	if ok {
		w.freed()
	}
	return e.Frame, ok
}

// Close implements app.Conn: the first call stops the writer and starts the close handshake with
// code and reason in its own goroutine; later calls do nothing.
func (w *wsConn) Close(code domain.CloseCode, reason string) {
	w.closing.Do(func() {
		close(w.done)
		w.dropQueue()
		w.closed.Add(1)
		go func() {
			defer w.closed.Done()
			_ = w.ws.Close(websocket.StatusCode(code), reason)
		}()
	})
}

// abort ends the connection without a close frame (the client is gone or a write failed).
func (w *wsConn) abort() {
	w.closing.Do(func() {
		close(w.done)
		w.dropQueue()
		_ = w.ws.CloseNow()
	})
}

// dropQueue drops every queued frame of a closing connection.
func (w *wsConn) dropQueue() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.queue.Clear()
	w.freed()
}

// write is the writer goroutine: it writes the queued frames in order, each with the deadline
// gateway.write_timeout, until the connection closes or a write fails.
func (w *wsConn) write(timeout time.Duration) {
	for {
		b, ok := w.pop()
		if !ok {
			select {
			case <-w.done:
				return
			case <-w.ready:
			}
			continue
		}
		if !w.writeOne(b, timeout) {
			return
		}
	}
}

// writeOne writes one frame and reports whether the writer goes on. A missed deadline ends the
// connection with 4014; coder/websocket tears the connection down when a write's context ends, so
// the client sees the end without the close frame then (docs/DECISIONS.md M1-64/8).
func (w *wsConn) writeOne(b []byte, timeout time.Duration) bool {
	if w.beforeWrite != nil {
		w.beforeWrite()
	}
	select {
	case <-w.done:
		return false
	default:
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := w.ws.Write(ctx, websocket.MessageText, b); err != nil {
		if ctx.Err() != nil {
			w.closeApp(domain.CloseSlowConsumer)
		}
		w.abort()
		return false
	}
	return true
}
