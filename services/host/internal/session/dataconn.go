// SPDX-License-Identifier: Apache-2.0

package session

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"

	"github.com/OWNER/bothy/services/host/internal/control"
	"github.com/OWNER/bothy/services/host/internal/noise"
)

// sendQueue bounds the encrypted messages waiting for the writer of one data connection; a device
// that reads slower than its session writes loses the connection (detached), so a slow device never
// blocks the helper or another session. A protocol constant of the queue, not a product window: the
// relay applies terminal.relay_buffer_max_bytes.
const sendQueue = 256

// dataConn is one outbound data connection of a session (asyncapi channel hostData): hello and
// identify as text, then one Noise message per binary message.
type dataConn struct {
	ws    *websocket.Conn
	hello *control.Hello
	in    chan []byte
	out   chan []byte
	done  chan struct{}
	once  sync.Once
	// code and reason are the close of the server, or -1 for a lost connection; set before done
	// closes.
	mu     sync.Mutex
	code   int
	reason string
	closed bool
}

// identifyData is identify on a data connection (schema HostIdentify with session_id).
type identifyData struct {
	ProtocolVersion int    `json:"protocol_version"`
	HostID          string `json:"host_id"`
	Proof           string `json:"proof"`
	HostVersion     string `json:"host_version"`
	SessionID       string `json:"session_id"`
}

// errHello is a data connection whose hello does not decode or lacks the values of M2-45.
var errHello = errors.New("session: hello of the data connection unusable")

// dialData opens the data connection url with hc, reads hello and sends identify with the proof of
// the host login (control.Proof) and the session ID.
func dialData(ctx context.Context, hc *http.Client, url string, d identifyData, sign func(challenge string) string) (*dataConn, error) {
	ws, resp, err := websocket.Dial(ctx, url, &websocket.DialOptions{HTTPClient: hc, CompressionMode: websocket.CompressionDisabled})
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	if err != nil {
		return nil, err
	}
	ws.SetReadLimit(-1)
	typ, b, err := ws.Read(ctx)
	if err != nil || typ != websocket.MessageText {
		_ = ws.CloseNow()
		return nil, errHello
	}
	h, err := control.DecodeHello(b)
	if err != nil || h.ClockLeewayMs < 0 || h.IdleTimeoutMs <= 0 || h.DetachGraceMs <= 0 {
		_ = ws.Close(websocket.StatusCode(control.CodeDecodeError), "")
		return nil, errHello
	}
	d.ProtocolVersion = 1
	d.Proof = sign(h.Challenge)
	frame, _ := json.Marshal(struct {
		Op string       `json:"op"`
		D  identifyData `json:"d"`
	}{"identify", d})
	if err := ws.Write(ctx, websocket.MessageText, frame); err != nil {
		_ = ws.CloseNow()
		return nil, err
	}
	dc := &dataConn{ws: ws, hello: h, in: make(chan []byte, 16), out: make(chan []byte, sendQueue), done: make(chan struct{}), code: -1}
	go dc.read()
	go dc.write()
	return dc, nil
}

// leeway, idle and detach are the server values of the hello of this connection.
func (dc *dataConn) leeway() time.Duration {
	return time.Duration(dc.hello.ClockLeewayMs) * time.Millisecond
}
func (dc *dataConn) idle() time.Duration { return time.Duration(dc.hello.IdleTimeoutMs) * time.Millisecond }
func (dc *dataConn) detach() time.Duration {
	return time.Duration(dc.hello.DetachGraceMs) * time.Millisecond
}

// read queues the binary messages until the connection ends; a text frame ends it like the relay.
func (dc *dataConn) read() {
	defer close(dc.in)
	for {
		typ, b, err := dc.ws.Read(context.Background())
		if err != nil {
			code, reason := -1, ""
			var ce websocket.CloseError
			if errors.As(err, &ce) {
				code, reason = int(ce.Code), ce.Reason
			}
			dc.finish(code, reason)
			return
		}
		if typ != websocket.MessageBinary {
			dc.close(control.CodeDecodeError, "")
			dc.finish(-1, "")
			return
		}
		select {
		case dc.in <- b:
		case <-dc.done:
			return
		}
	}
}

// write sends the queued messages in order.
func (dc *dataConn) write() {
	for {
		select {
		case m := <-dc.out:
			if err := dc.ws.Write(context.Background(), websocket.MessageBinary, m); err != nil {
				dc.close(CodeNormal, "")
				return
			}
		case <-dc.done:
			return
		}
	}
}

// finish records the end of the connection once.
func (dc *dataConn) finish(code int, reason string) {
	dc.once.Do(func() {
		dc.mu.Lock()
		dc.code, dc.reason = code, reason
		dc.mu.Unlock()
		close(dc.done)
	})
}

// send queues one message; a full queue closes the connection (the device is too slow).
func (dc *dataConn) send(m []byte) {
	select {
	case dc.out <- m:
	case <-dc.done:
	default:
		dc.close(CodeNormal, "")
	}
}

// next returns the next binary message, or false when the connection ended or the timer fired.
func (dc *dataConn) next(deadline <-chan time.Time) ([]byte, bool) {
	select {
	case m, ok := <-dc.in:
		return m, ok
	case <-deadline:
		return nil, false
	}
}

// close closes the connection with code and reason once (this side ends it); the queued messages
// before it are not waited for.
func (dc *dataConn) close(code int, reason string) {
	dc.mu.Lock()
	if dc.closed {
		dc.mu.Unlock()
		return
	}
	dc.closed = true
	dc.mu.Unlock()
	go func() {
		_ = dc.ws.Close(websocket.StatusCode(code), reason)
		dc.finish(code, reason)
	}()
}

// closeCode returns the close code of the server (-1 for a lost connection) once done is closed.
func (dc *dataConn) closeCode() int {
	dc.mu.Lock()
	defer dc.mu.Unlock()
	return dc.code
}

// encryptFrame encrypts frame bytes with t.
func encryptFrame(t *noise.Transport, f []byte) ([]byte, error) { return t.Encrypt(f) }
