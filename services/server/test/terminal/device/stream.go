// SPDX-License-Identifier: Apache-2.0

package device

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"

	"github.com/coder/websocket"

	"github.com/OWNER/bothy/services/server/api/specschema"
)

// channelStream is the AsyncAPI channel of the device stream (docs/api/asyncapi.yaml).
const channelStream = "terminalStream"

// Prover is the part of a harness client the stream needs: the access token and a DPoP proof for
// a method, URL and nonce (harness.Client).
type Prover interface {
	Tokens() (access, refresh string)
	Proof(htm, htu, nonce string) string
}

// WireMessage is one binary message as it passed the relay, in the order of the stream (the relay
// view of TM-T06).
type WireMessage struct {
	// Sent is true for a message of the device, false for one it received.
	Sent bool
	// Bytes are the bytes of the message.
	Bytes []byte
}

// Hello is the terminal.hello of the server (asyncapi TerminalStreamHello).
type Hello struct {
	DpopNonce string `json:"dpop_nonce"`
}

// DialParams describe one device stream.
type DialParams struct {
	// Dialer reaches the instance origin.
	Dialer *Dialer
	// Origin is server.public_url (https://chat.example.com:8443).
	Origin string
	// StreamPath is stream_path of the grant (/api/v1/terminal/sessions/<id>/stream).
	StreamPath string
	// Prover gives the access token and the DPoP proofs of identify.
	Prover Prover
	// DeviceID is the device of the person.
	DeviceID string
	// Validate checks every text frame against the contract (G9); nil uses the default contract.
	Validate func(serverFrame bool, b []byte) error
}

// Stream is one device stream of a terminal session: terminal.hello, terminal.identify with
// DPoP and the nonce of hello (no Origin header), then the binary messages of Noise. It keeps
// every binary message it sent and received in memory and can cut its TCP connection without a
// close frame (TM-T16).
type Stream struct {
	// Hello is the hello of the server.
	Hello Hello
	// Violations are the frames of either side that broke the contract.
	Violations []string

	ws    *websocket.Conn
	tcp   net.Conn
	mu    sync.Mutex
	cond  *sync.Cond
	queue [][]byte
	wire  []WireMessage
	texts [][]byte
	cerr  error
	done  chan struct{}
}

// Dial opens the device stream of p: the WebSocket upgrade through the dialer, hello, identify.
// It returns the stream, which may already be closed by the server (CloseCode reports it).
//
// Errors: a refused upgrade (with its HTTP status) or a hello that is no text frame.
func Dial(ctx context.Context, p DialParams) (*Stream, error) {
	validate := p.Validate
	if validate == nil {
		contract, err := specschema.Default()
		if err != nil {
			return nil, err
		}
		validate = func(server bool, b []byte) error { return contract.ValidateChannelFrame(channelStream, server, b) }
	}
	s := &Stream{done: make(chan struct{})}
	s.cond = sync.NewCond(&s.mu)
	d := *p.Dialer
	d.OnConn = func(c net.Conn) { s.mu.Lock(); s.tcp = c; s.mu.Unlock() }
	tr := d.Transport()
	tr.DisableKeepAlives = true
	url := "wss://" + strings.TrimPrefix(p.Origin, "https://") + p.StreamPath
	ws, resp, err := websocket.Dial(ctx, url, &websocket.DialOptions{HTTPClient: &http.Client{Transport: tr}, CompressionMode: websocket.CompressionDisabled}) // nosemgrep: bothy-go-egress-only -- the Go test device of the suite; it dials only the TLS front of the test stack
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	if err != nil {
		status := 0
		if resp != nil {
			status = resp.StatusCode
		}
		return nil, fmt.Errorf("device: upgrade of the stream: status %d: %w", status, err)
	}
	ws.SetReadLimit(-1)
	s.ws = ws
	typ, b, err := ws.Read(ctx)
	if err != nil {
		return nil, fmt.Errorf("device: hello: %w", err)
	}
	if typ != websocket.MessageText {
		return nil, errors.New("device: hello is not a text frame")
	}
	if err := validate(true, b); err != nil {
		s.Violations = append(s.Violations, "server hello: "+err.Error())
	}
	var env struct {
		D Hello `json:"d"`
	}
	if err := json.Unmarshal(b, &env); err != nil {
		return nil, err
	}
	s.Hello = env.D
	go s.read(validate) //nolint:gosec // G118: the reader lives as long as the stream, not as the dial context
	access, _ := p.Prover.Tokens()
	htu := strings.TrimSuffix(p.Origin, "/") + p.StreamPath
	frame, err := json.Marshal(map[string]any{"op": "identify", "d": map[string]any{
		"protocol_version": 1, "device_id": p.DeviceID, "access_token": access, "dpop": p.Prover.Proof(http.MethodGet, htu, env.D.DpopNonce),
	}})
	if err != nil {
		return nil, err
	}
	if err := validate(false, frame); err != nil {
		s.Violations = append(s.Violations, "device identify: "+err.Error())
	}
	if err := ws.Write(ctx, websocket.MessageText, frame); err != nil {
		return s, nil //nolint:nilerr // the server may close right after hello; CloseCode reports why
	}
	return s, nil
}

// read queues the binary messages and keeps the text frames until the stream ends.
func (s *Stream) read(validate func(bool, []byte) error) {
	defer close(s.done)
	for {
		typ, b, err := s.ws.Read(context.Background())
		s.mu.Lock()
		if err != nil {
			s.cerr = err
			s.cond.Broadcast()
			s.mu.Unlock()
			return
		}
		switch typ {
		case websocket.MessageBinary:
			s.queue = append(s.queue, b)
			s.wire = append(s.wire, WireMessage{Bytes: b})
		case websocket.MessageText:
			s.texts = append(s.texts, b)
			if verr := validate(true, b); verr != nil {
				s.Violations = append(s.Violations, "server frame: "+verr.Error())
			}
		}
		s.cond.Broadcast()
		s.mu.Unlock()
	}
}

// Send sends m as one binary message and records it.
func (s *Stream) Send(ctx context.Context, m []byte) error {
	s.mu.Lock()
	s.wire = append(s.wire, WireMessage{Sent: true, Bytes: append([]byte(nil), m...)})
	s.mu.Unlock()
	return s.ws.Write(ctx, websocket.MessageBinary, m)
}

// Recv returns the next binary message; after the end of the stream its close error.
func (s *Stream) Recv(ctx context.Context) ([]byte, error) {
	stop := context.AfterFunc(ctx, func() { s.mu.Lock(); s.cond.Broadcast(); s.mu.Unlock() })
	defer stop()
	s.mu.Lock()
	defer s.mu.Unlock()
	for len(s.queue) == 0 {
		if s.cerr != nil {
			return nil, s.cerr
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		s.cond.Wait()
	}
	m := s.queue[0]
	s.queue = s.queue[1:]
	return m, nil
}

// CloseCode waits for the end of the stream within ctx and returns the close code and reason of
// the server; 0 means the stream did not end with a close frame (a cut or ctx ended).
func (s *Stream) CloseCode(ctx context.Context) (int, string) {
	select {
	case <-s.done:
	case <-ctx.Done():
		return 0, ""
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var ce websocket.CloseError
	if errors.As(s.cerr, &ce) {
		return int(ce.Code), ce.Reason
	}
	return 0, ""
}

// Ended reports whether the stream has ended.
func (s *Stream) Ended() bool {
	select {
	case <-s.done:
		return true
	default:
		return false
	}
}

// Cut closes the TCP connection of the stream without a close frame (a network loss of the
// device, TM-T16).
func (s *Stream) Cut() error {
	s.mu.Lock()
	c := s.tcp
	s.mu.Unlock()
	if c == nil {
		return errors.New("device: no TCP connection to cut")
	}
	return c.Close()
}

// Close closes the stream with 1000 (the device leaves).
func (s *Stream) Close() error { return s.ws.Close(websocket.StatusNormalClosure, "") }

// Wire returns a copy of every binary message the stream sent and received.
func (s *Stream) Wire() []WireMessage {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]WireMessage, len(s.wire))
	for i, m := range s.wire {
		out[i] = WireMessage{Sent: m.Sent, Bytes: append([]byte(nil), m.Bytes...)}
	}
	return out
}
