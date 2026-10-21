// SPDX-License-Identifier: Apache-2.0

package control

import (
	"context"
	"errors"
	"log/slog"
	gosync "sync"
	"time"

	"github.com/coder/websocket"

	"github.com/OWNER/bothy/services/host/internal/startup"
)

// Log messages of a connection: codes only.
const (
	msgClosed       = "control connection closed"
	msgUpdate       = "update the host service or the server"
	msgServerError  = "server answered an op with an error"
	logCode         = "code"
	logOp           = "op"
	reasonNoVersion = "no common protocol version"
	logReason       = "reason"
)

// frameOrEnd is a frame of the server or the end of the reader.
type frameOrEnd struct {
	op   string
	body any
	// err ends the reader: the close of the server, a broken connection or a frame that does not
	// decode (ErrFrame).
	err error
}

// conn is one control connection.
type conn struct {
	c         *Client
	ws        *websocket.Conn
	peerLocal bool
	frames    chan frameOrEnd
	// done is closed when serve returned; the reader stops then.
	done chan struct{}
	once gosync.Once
	mu   gosync.Mutex
	// reported is set once a report was sent after the last ready; until then session.open is a
	// protocol violation (24 section 2.2 point 5) and no heartbeat goes out.
	reported bool
	// writeTimeout bounds every write: the heartbeat interval of hello.
	writeTimeout time.Duration
	// closeCode is the code this side closed with, 0 while open.
	closeCode int
	// reportMu orders the reports of this connection: each one sends the current report, so the
	// last one sent is the newest.
	reportMu gosync.Mutex
}

// read is the reader goroutine: every frame of the server, decoded, until the connection ends
// or serve returned (done), so the goroutine never outlives its connection.
func (k *conn) read(ctx context.Context) {
	defer close(k.frames)
	push := func(f frameOrEnd) bool {
		select {
		case k.frames <- f:
			return true
		case <-k.done:
			return false
		}
	}
	for {
		typ, b, err := k.ws.Read(ctx)
		switch {
		case err != nil:
			push(frameOrEnd{err: err})
			return
		case typ != websocket.MessageText:
			push(frameOrEnd{err: ErrFrame})
			return
		}
		op, body, err := decodeServerFrame(b)
		if err != nil {
			push(frameOrEnd{err: err})
			return
		}
		if !push(frameOrEnd{op: op, body: body}) {
			return
		}
	}
}

// write sends the frame {op, d} within the write timeout.
func (k *conn) write(ctx context.Context, op string, d any) error {
	b, err := encodeFrame(op, d)
	if err != nil {
		return err
	}
	timeout := k.writeTimeout
	if timeout <= 0 {
		timeout = k.c.cfg.BackoffCap
	}
	wctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.closeCode != 0 {
		return ErrClosed
	}
	return k.ws.Write(wctx, websocket.MessageText, b)
}

// close closes the connection with code once; the close frame follows every frame written
// before.
func (k *conn) close(code int) {
	k.once.Do(func() {
		k.mu.Lock()
		k.closeCode = code
		k.mu.Unlock()
		_ = k.ws.Close(websocket.StatusCode(code), "")
	})
}

// sendReport sends the current report with the server-host flag of this connection; without one
// it sends nothing.
func (k *conn) sendReport(ctx context.Context) error {
	k.reportMu.Lock()
	defer k.reportMu.Unlock()
	r, ok := k.c.reports.Get()
	if !ok {
		return nil
	}
	if err := k.write(ctx, opReport, withPeer(r, k.peerLocal)); err != nil {
		return err
	}
	k.mu.Lock()
	k.reported = true
	k.mu.Unlock()
	return nil
}

// hasReported reports a report sent after the last ready.
func (k *conn) hasReported() bool {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.reported
}

// Started implements Notifier.
func (k *conn) Started(ctx context.Context, sessionID string) error {
	return k.write(ctx, opSessionStarted, SessionStarted{SessionID: sessionID})
}

// Ended implements Notifier.
func (k *conn) Ended(ctx context.Context, e SessionEnded) error {
	return k.write(ctx, opSessionEnded, e)
}

// end turns the end of the reader into the reaction of the close policy.
func (k *conn) end(err error) outcome {
	code, reason := -1, ""
	var ce websocket.CloseError
	if errors.As(err, &ce) {
		code, reason = int(ce.Code), ce.Reason
	}
	if errors.Is(err, ErrFrame) {
		k.close(CodeDecodeError)
		code = CodeDecodeError
	}
	k.c.log.LogAttrs(context.Background(), slog.LevelInfo, msgClosed, slog.Int(logCode, code))
	return outcome{Reaction: Policy(code, reason, k.c.cfg.BackoffCap), code: code}
}

// serve runs the login and the ops of one connection (Design note 12 b to e of M2-43).
func (k *conn) serve(ctx context.Context) outcome {
	c := k.c
	f, ok := <-k.frames
	if !ok || f.err != nil {
		return k.end(f.err)
	}
	hello, isHello := f.body.(*Hello)
	if !isHello {
		k.close(CodeDecodeError)
		return outcome{Reaction: Reaction{Kind: Backoff}, code: CodeDecodeError}
	}
	version, ok := ChooseVersion(ProtocolVersions, hello.ProtocolVersions)
	if !ok {
		c.log.LogAttrs(ctx, slog.LevelError, msgUpdate, slog.String(logReason, reasonNoVersion))
		k.close(CodeNormal)
		return outcome{Reaction: Reaction{Kind: Stop}, code: CodeHostVersionUnsupported}
	}
	interval := time.Duration(hello.HeartbeatIntervalMs) * time.Millisecond
	if interval <= 0 || hello.HeartbeatMissLimit < 1 {
		k.close(CodeDecodeError)
		return outcome{Reaction: Reaction{Kind: Backoff}, code: CodeDecodeError}
	}
	k.writeTimeout = interval
	c.keys.Set(hello.JWKS)
	live := c.sessions.Live()
	if live == nil {
		live = []string{}
	}
	if err := k.write(ctx, opIdentify, Identify{ProtocolVersion: version, HostID: c.cfg.HostID,
		Proof: Proof(c.cfg.AuthKey, c.cfg.HostID, hello.Challenge, c.cfg.Clock.Now()), HostVersion: c.cfg.HostVersion, LiveSessions: live}); err != nil {
		return outcome{Reaction: Reaction{Kind: Backoff}}
	}
	silence := interval * time.Duration(hello.HeartbeatMissLimit+1)
	return k.ops(ctx, interval, silence)
}

// ops handles the frames after identify until the connection ends.
func (k *conn) ops(ctx context.Context, interval, silence time.Duration) outcome {
	c := k.c
	quiet := c.cfg.Clock.NewTimer(silence)
	defer func() { quiet.Stop() }()
	beat := c.cfg.Clock.NewTimer(interval)
	defer func() { beat.Stop() }()
	ready := false
	for {
		select {
		case <-ctx.Done():
			k.close(CodeGoingAway)
			return outcome{Reaction: Reaction{Kind: Backoff}, code: CodeGoingAway}
		case <-quiet.C():
			// No frame of the server for heartbeat_interval_ms times (heartbeat_miss_limit + 1).
			k.close(CodeHeartbeatTimeout)
			return outcome{Reaction: Reaction{Kind: Backoff}, code: CodeHeartbeatTimeout}
		case <-beat.C():
			beat = c.cfg.Clock.NewTimer(interval)
			if ready && k.hasReported() {
				_ = k.write(ctx, opHeartbeat, Heartbeat{ActiveSessions: c.sessions.Active()})
			}
		case f, ok := <-k.frames:
			if !ok || f.err != nil {
				return k.end(f.err)
			}
			quiet.Stop()
			quiet = c.cfg.Clock.NewTimer(silence)
			if out, done := k.handle(ctx, f, &ready); done {
				return out
			}
		}
	}
}

// handle reacts to one frame after identify; done ends the connection with out.
func (k *conn) handle(ctx context.Context, f frameOrEnd, ready *bool) (outcome, bool) {
	c := k.c
	switch b := f.body.(type) {
	case *ReadyFrame:
		*ready = true
		k.mu.Lock()
		k.reported = false
		k.mu.Unlock()
		c.backoff.Reset()
		// The connection is the live one before the current report goes out, so a Report from
		// now on reaches it; then the current report first, before any other frame is handled
		// (24 section 2.2 point 5).
		c.attach(k)
		if err := k.sendReport(ctx); err != nil {
			return outcome{Reaction: Reaction{Kind: Backoff}}, true
		}
		c.deliver(startup.Ready{LLMCredentials: b.LLMCredentials}, k)
	case *HeartbeatAck:
		if b.JWKS != nil {
			c.keys.Set(*b.JWKS)
		}
	case *SessionOpen:
		if !*ready || !k.hasReported() {
			// session.open before the own report breaks the order of 24 section 2.2 point 5.
			k.close(CodeDecodeError)
			return outcome{Reaction: Reaction{Kind: Backoff}, code: CodeDecodeError}, true
		}
		c.sessions.Open(ctx, *b, k)
	case *SessionClose:
		c.sessions.Close(ctx, *b)
	case *ErrorFrame:
		c.log.LogAttrs(ctx, slog.LevelWarn, msgServerError, slog.String(logOp, b.Op), slog.String(logCode, b.Code))
	default:
		// A second hello.
		k.close(CodeDecodeError)
		return outcome{Reaction: Reaction{Kind: Backoff}, code: CodeDecodeError}, true
	}
	return outcome{}, false
}
