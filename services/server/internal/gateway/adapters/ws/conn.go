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

// wsConn implements app.Conn on one WebSocket: Send encodes the frame and queues it for the
// writer goroutine, bounded by sync.outbound_queue_max; Close starts the close handshake in its
// own goroutine, so no caller waits on the network. Frames still queued at the close are dropped.
type wsConn struct {
	ws       *websocket.Conn
	snapshot SnapshotEncoder
	log      *slog.Logger
	queue    chan []byte
	done     chan struct{}
	closing  gosync.Once
	closed   gosync.WaitGroup
	// app is the gateway's connection, set right after Open; overflow closes through it, so the
	// close code reaches the gateway.
	mu  gosync.Mutex
	app *app.Connection
	// beforeWrite is a test hook of the writer; nil in production.
	beforeWrite func()
}

// newConn returns the connection over ws with a queue of queueMax frames.
func newConn(ws *websocket.Conn, queueMax int, snapshot SnapshotEncoder, log *slog.Logger) *wsConn {
	return &wsConn{ws: ws, snapshot: snapshot, log: log, queue: make(chan []byte, queueMax), done: make(chan struct{})}
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

// Send implements app.Conn: the frame is encoded now and queued; a full queue closes the
// connection with 4014 (04 section 7.2 item 5). A frame that cannot be encoded closes with 4000.
func (w *wsConn) Send(f any) error {
	select {
	case <-w.done:
		return errClosing
	default:
	}
	b, err := encodeFrame(f, w.snapshot)
	if err != nil {
		w.log.LogAttrs(context.Background(), slog.LevelError, msgEncodeFailed, slog.String(logKeyErrorType, errorTypeEncode))
		w.closeApp(domain.CloseUnknownError)
		return err
	}
	select {
	case w.queue <- b:
		return nil
	default:
		w.closeApp(domain.CloseSlowConsumer)
		return errOverflow
	}
}

// Close implements app.Conn: the first call stops the writer and starts the close handshake with
// code and reason in its own goroutine; later calls do nothing.
func (w *wsConn) Close(code domain.CloseCode, reason string) {
	w.closing.Do(func() {
		close(w.done)
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
		_ = w.ws.CloseNow()
	})
}

// write is the writer goroutine: it writes the queued frames in order, each with the deadline
// gateway.write_timeout, until the connection closes or a write fails.
func (w *wsConn) write(timeout time.Duration) {
	for {
		select {
		case <-w.done:
			return
		case b := <-w.queue:
			if !w.writeOne(b, timeout) {
				return
			}
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
