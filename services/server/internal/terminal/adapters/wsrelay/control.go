// SPDX-License-Identifier: Apache-2.0

package wsrelay

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	gosync "sync"
	"time"

	"github.com/coder/websocket"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/terminal/app"
	"github.com/OWNER/bothy/services/server/internal/terminal/domain"
)

// UseCases are the use cases of the control connection (app.Service, M2-43).
type UseCases interface {
	TerminalJWKS(ctx context.Context) ([]app.JWK, error)
	AuthFailed(ctx context.Context) error
	Authenticate(ctx context.Context, x app.Exchange, in app.Identify) (app.Login, domain.CloseCode, error)
	LoginWrite(ctx context.Context, l app.Login, in app.Identify) ([]domain.SessionClose, error)
	Ready(ctx context.Context, l app.Login) (app.Ready, domain.CloseCode, error)
	Heartbeat(ctx context.Context, l app.Login) (domain.CloseCode, error)
	ApplyReport(ctx context.Context, l app.Login, r domain.Report) (app.ReportResult, domain.CloseCode, error)
	SessionStarted(ctx context.Context, l app.Login, sessionID ids.ID) (app.SessionAnswer, domain.CloseCode, error)
	SessionEnded(ctx context.Context, l app.Login, sessionID ids.ID, end domain.HostSessionEnd) (app.SessionAnswer, domain.CloseCode, error)
	ConnectionLost(ctx context.Context, hostID ids.ID) error
	StaleReplaced(ctx context.Context, hostID ids.ID) error
	ResolveDuplicate(ctx context.Context, hostID ids.ID) error
}

// Compile-time check: the use cases of the module.
var _ UseCases = (*app.Service)(nil)

// Config holds the configuration keys of the control connection.
type Config struct {
	// HeartbeatInterval is terminal.host_heartbeat_interval (hello.heartbeat_interval_ms).
	HeartbeatInterval time.Duration
	// HeartbeatMissLimit is terminal.host_heartbeat_miss_limit.
	HeartbeatMissLimit int
	// AuthTimeout is terminal.host_auth_timeout, the time between hello and identify.
	AuthTimeout time.Duration
	// DuplicateProbeTimeout is terminal.host_duplicate_probe_timeout.
	DuplicateProbeTimeout time.Duration
	// MaxFrameBytes is sync.ws_max_frame_bytes.
	MaxFrameBytes int
	// MaxJSONDepth is http.max_json_depth.
	MaxJSONDepth int
	// OutboundQueueMax is sync.outbound_queue_max.
	OutboundQueueMax int
	// WriteTimeout is gateway.write_timeout.
	WriteTimeout time.Duration
}

// Deps are the collaborators of the handler.
type Deps struct {
	UseCases UseCases
	Links    *Links
	// Schemas checks the bodies of the host ops (httpserver.SchemaValidator over api/rest.Spec).
	Schemas SchemaChecker
	// Clock gives hello.server_time and heartbeat_ack.server_time; nil means clock.System().
	Clock clock.Clock
	// Log receives panics and internal errors without content; nil means slog.Default().
	Log *slog.Logger
}

// Handler is the HTTP handler of GET /api/v1/hosts/connect (asyncapi channel hostControl, 24
// section 3.5): it upgrades with github.com/coder/websocket (compression off, the library's
// origin check) and runs one control connection per request with a reader, a writer and the
// deadlines of the login and the heartbeat. It is safe for concurrent use.
type Handler struct {
	cfg Config
	d   Deps
}

// NewHandler returns the handler. Errors: a limit below 1 or a missing collaborator.
func NewHandler(cfg Config, d Deps) (*Handler, error) {
	if cfg.HeartbeatInterval <= 0 || cfg.HeartbeatMissLimit < 1 || cfg.AuthTimeout <= 0 || cfg.DuplicateProbeTimeout <= 0 ||
		cfg.MaxFrameBytes < 1 || cfg.MaxJSONDepth < 1 || cfg.OutboundQueueMax < 1 || cfg.WriteTimeout <= 0 {
		return nil, errors.New("wsrelay: the terminal.host_* keys, sync.ws_max_frame_bytes, http.max_json_depth, " +
			"sync.outbound_queue_max and gateway.write_timeout must be positive")
	}
	if d.UseCases == nil || d.Links == nil || d.Schemas == nil {
		return nil, errors.New("wsrelay: NewHandler needs the use cases, the registry and the schemas")
	}
	if d.Clock == nil {
		d.Clock = clock.System()
	}
	if d.Log == nil {
		d.Log = slog.Default()
	}
	return &Handler{cfg: cfg, d: d}, nil
}

// Log messages and keys of the adapter: codes only, never a proof, a challenge, a key or report
// content (24 section 10.4, SEC-LOG-2).
const (
	msgPanic      = "panic recovered"
	msgInternal   = "control connection failed"
	logErrorType  = "error.type"
	errorPanic    = "panic"
	errorInternal = "internal"
	reasonTooBig  = "frame_too_large"
	reasonBinary  = "binary_frame"
	codeNotFound  = "not_found"
	codeInvalid   = "validation_failed"
)

// ServeHTTP implements http.Handler: the upgrade, then the connection until it ends. The host
// service sends no Origin; a request with one passes only the library's same-host check.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ws, err := websocket.Accept(w, r, &websocket.AcceptOptions{CompressionMode: websocket.CompressionDisabled})
	if err != nil {
		return // Accept has answered the request
	}
	// The frame limit is enforced by the reader with 4002, not by the library's 1009.
	ws.SetReadLimit(-1)
	h.serve(context.WithoutCancel(r.Context()), ws)
}

// conn is the state of one connection while it runs.
type conn struct {
	h *Handler
	k *link
	// timer is the running deadline: the login, then the heartbeat.
	mu    gosync.Mutex
	timer *time.Timer
}

// serve runs one connection: hello, the login, ready and the ops until the connection ends.
func (h *Handler) serve(ctx context.Context, ws *websocket.Conn) {
	k := newLink(ws, h.cfg.OutboundQueueMax, h.cfg.WriteTimeout)
	c := &conn{h: h, k: k}
	go k.write(ctx)
	defer close(k.ended)
	defer c.stopTimer()
	defer func() {
		if recover() != nil {
			h.d.Log.LogAttrs(ctx, slog.LevelError, msgPanic, slog.String(logErrorType, errorPanic))
			k.close(domain.CodeUnknownError)
		}
		c.end(ctx)
	}()
	if !c.login(ctx) {
		return
	}
	c.ops(ctx)
}

// fail closes with 4000 after an internal error; the log names no content.
func (c *conn) fail(ctx context.Context) {
	c.h.d.Log.LogAttrs(ctx, slog.LevelError, msgInternal, slog.String(logErrorType, errorInternal))
	c.k.close(domain.CodeUnknownError)
}

// setTimer arms the deadline d that closes with code; the earlier deadline is stopped.
func (c *conn) setTimer(d time.Duration, fire func()) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.timer != nil {
		c.timer.Stop()
	}
	c.timer = time.AfterFunc(d, fire)
}

// stopTimer stops the running deadline and reports whether it had not fired.
func (c *conn) stopTimer() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.timer == nil {
		return true
	}
	return c.timer.Stop()
}

// heartbeatDeadline arms the heartbeat deadline from now: interval times (miss limit + 1).
func (c *conn) heartbeatDeadline() {
	now := time.Now()
	wait := domain.HeartbeatDeadline(now, c.h.cfg.HeartbeatInterval, c.h.cfg.HeartbeatMissLimit).Sub(now)
	c.setTimer(wait, func() { c.k.close(domain.CodeHeartbeatTimeout) })
}

// read returns the next frame. It returns ok false when the connection ended (the reader is done)
// and closes with 4002 a binary frame, a frame over sync.ws_max_frame_bytes and a frame the codec
// refuses.
func (c *conn) read(ctx context.Context) (frame, bool) {
	// After a close the reader stops: the close handshake of coder/websocket reads until the
	// peer's close frame itself, and a frame left half read must not be parsed as the next one.
	if c.k.isClosing() {
		return frame{}, false
	}
	typ, rd, err := c.k.ws.Reader(ctx)
	if err != nil || c.k.isClosing() {
		return frame{}, false
	}
	if typ != websocket.MessageText {
		c.k.closeReason(domain.CodeDecodeError, reasonBinary)
		return frame{}, false
	}
	b, err := io.ReadAll(io.LimitReader(rd, int64(c.h.cfg.MaxFrameBytes)+1))
	if err != nil {
		return frame{}, false
	}
	if len(b) > c.h.cfg.MaxFrameBytes {
		c.k.closeReason(domain.CodeDecodeError, reasonTooBig)
		return frame{}, false
	}
	f, err := decodeFrame(b, c.h.cfg.MaxJSONDepth, c.h.d.Schemas)
	if err != nil {
		c.k.close(domain.CodeDecodeError)
		return frame{}, false
	}
	return f, !c.k.isClosing()
}

// login runs hello, identify, the duplicate check, the login unit of work and ready (Design notes
// 3 and 5 of M2-43). It reports whether the connection is logged in.
func (c *conn) login(ctx context.Context) bool {
	l, in, ok := c.identify(ctx, false)
	if !ok {
		return false
	}
	return c.register(ctx, l, in)
}

// identify is the one login function of the control connection and the data connection (M2-43
// Design note 3, M2-44 Design note 9): hello with the challenge and the key set, identify within
// terminal.host_auth_timeout, the proof and authorize host.connect (Authenticate, whose denials
// authz records). data says which members identify must carry: live_sessions on the control
// connection, session_id on a data connection. It returns the login and the decoded identify, or
// false after it closed the connection with its code.
func (c *conn) identify(ctx context.Context, data bool) (app.Login, app.Identify, bool) {
	h, k := c.h, c.k
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		c.fail(ctx)
		return app.Login{}, app.Identify{}, false
	}
	challenge := base64.RawURLEncoding.EncodeToString(raw)
	helloAt := stamp(h.d.Clock.Now())
	keys, err := h.d.UseCases.TerminalJWKS(ctx)
	if err != nil {
		c.fail(ctx)
		return app.Login{}, app.Identify{}, false
	}
	k.lastJWKS = keySet(keys)
	k.send(helloFrame(challenge, helloAt, h.cfg.HeartbeatInterval, h.cfg.HeartbeatMissLimit, keys))
	c.setTimer(h.cfg.AuthTimeout, func() {
		if err := h.d.UseCases.AuthFailed(ctx); err != nil {
			h.d.Log.LogAttrs(ctx, slog.LevelError, msgInternal, slog.String(logErrorType, errorInternal))
		}
		k.close(domain.CodeHostAuthFailed)
	})
	f, ok := c.read(ctx)
	if !ok || !c.stopTimer() {
		return app.Login{}, app.Identify{}, false
	}
	if f.op != opIdentify {
		if err := h.d.UseCases.AuthFailed(ctx); err != nil {
			c.fail(ctx)
			return app.Login{}, app.Identify{}, false
		}
		k.close(domain.CodeHostAuthFailed)
		return app.Login{}, app.Identify{}, false
	}
	in := identifyOf(f.d.(*rest.HostIdentify))
	in.Data = data
	l, code, err := h.d.UseCases.Authenticate(ctx, app.Exchange{Challenge: challenge, HelloAt: helloAt}, in)
	switch {
	case err != nil:
		c.fail(ctx)
		return app.Login{}, app.Identify{}, false
	case code != 0:
		k.close(code)
		return app.Login{}, app.Identify{}, false
	}
	return l, in, true
}

// register runs the duplicate check under the per-host lock, the login unit of work, the
// registration and ready.
func (c *conn) register(ctx context.Context, l app.Login, in app.Identify) bool {
	h, k := c.h, c.k
	unlock := h.d.Links.lockHost(l.HostID)
	if !c.duplicate(ctx, l.HostID) {
		unlock()
		return false
	}
	closes, err := h.d.UseCases.LoginWrite(ctx, l, in)
	if err != nil {
		unlock()
		if errs.KindOf(err) == errs.KindNotFound {
			k.close(domain.CodeHostUnpaired)
			return false
		}
		c.fail(ctx)
		return false
	}
	k.login = l
	if !h.d.Links.put(k) {
		unlock()
		k.close(domain.CodeServerRestart)
		return false
	}
	unlock()
	ready, code, err := h.d.UseCases.Ready(ctx, l)
	switch {
	case err != nil:
		c.fail(ctx)
		return false
	case code != 0:
		k.markHandled()
		k.close(code)
		return false
	}
	k.send(readyFrame(ready))
	for _, sc := range closes {
		k.send(sessionCloseFrame(sc))
	}
	c.heartbeatDeadline()
	return true
}

// duplicate handles a live link of the same host (Design note 5 of M2-43, 24 section 3.4): a ping
// within terminal.host_duplicate_probe_timeout. No answer closes the older with 4009, detaches its
// sessions and audits stale_connection_replaced; the new login goes on. An answer disables the host
// (or puts an unconfirmed one back into pairing) and closes both with 4112. It reports whether the
// login goes on. The caller holds the per-host lock.
func (c *conn) duplicate(ctx context.Context, hostID ids.ID) bool {
	h, k := c.h, c.k
	old := h.d.Links.get(hostID)
	if old == nil {
		return true
	}
	pctx, cancel := context.WithTimeout(ctx, h.cfg.DuplicateProbeTimeout)
	err := old.ws.Ping(pctx)
	cancel()
	old.markHandled()
	if h.d.Links.remove(old) && old.isReported() {
		h.d.Links.publish(hostID, false)
	}
	if err != nil {
		old.close(domain.CodeHeartbeatTimeout)
		if r := h.d.Links.relayOf(); r != nil {
			r.ControlLost(hostID, domain.CodeHeartbeatTimeout)
		}
		if err := h.d.UseCases.ConnectionLost(ctx, hostID); err != nil {
			c.fail(ctx)
			return false
		}
		if err := h.d.UseCases.StaleReplaced(ctx, hostID); err != nil {
			c.fail(ctx)
			return false
		}
		return true
	}
	if err := h.d.UseCases.ResolveDuplicate(ctx, hostID); err != nil {
		old.close(domain.CodeUnknownError)
		c.fail(ctx)
		return false
	}
	old.close(domain.CodeHostDuplicate)
	k.close(domain.CodeHostDuplicate)
	return false
}

// ops handles the frames after ready in order until the connection ends (Design notes 4 a, 8, 9
// and 10 of M2-43).
func (c *conn) ops(ctx context.Context) {
	for {
		f, ok := c.read(ctx)
		if !ok {
			return
		}
		switch f.op {
		case opHeartbeat:
			c.heartbeat(ctx)
		case opReport:
			c.report(ctx, reportOf(f.d.(*rest.HostReport)))
		case opSessionStarted:
			id := ids.ID(f.d.(*rest.HostSessionStarted).SessionId)
			a, code, err := c.h.d.UseCases.SessionStarted(ctx, c.k.login, id)
			c.answer(ctx, opSessionStarted, a, code, err)
			if r := c.h.d.Links.relayOf(); r != nil && err == nil && code == 0 && !a.NotFound && a.Close == nil {
				// The deadline of an opening is met by session.started (Design note 7 of M2-44).
				r.Started(id)
			}
		case opSessionEnded:
			d := f.d.(*rest.HostSessionEnded)
			a, code, err := c.h.d.UseCases.SessionEnded(ctx, c.k.login, ids.ID(d.SessionId), endOf(d))
			c.answer(ctx, opSessionEnded, a, code, err)
		default:
			// A second identify (Design note 2).
			c.k.close(domain.CodeDecodeError)
		}
	}
}

// heartbeat answers heartbeat with heartbeat_ack, with the key set of purpose terminal when it
// changed since the last set sent on the link, and arms the deadline again.
func (c *conn) heartbeat(ctx context.Context) {
	code, err := c.h.d.UseCases.Heartbeat(ctx, c.k.login)
	switch {
	case err != nil:
		c.fail(ctx)
		return
	case code != 0:
		c.k.close(code)
		return
	}
	keys, err := c.h.d.UseCases.TerminalJWKS(ctx)
	if err != nil {
		c.fail(ctx)
		return
	}
	var send []app.JWK
	if set := keySet(keys); set != c.k.lastJWKS {
		c.k.lastJWKS, send = set, keys
	}
	c.heartbeatDeadline()
	c.k.send(heartbeatAckFrame(c.h.d.Clock.Now(), send))
}

// report applies a report; the first applied report after ready makes the host connected.
func (c *conn) report(ctx context.Context, r domain.Report) {
	_, code, err := c.h.d.UseCases.ApplyReport(ctx, c.k.login, r)
	switch {
	case isValidation(err):
		c.k.send(errorFrame(opReport, codeInvalid))
		return
	case err != nil:
		c.fail(ctx)
		return
	case code != 0:
		c.k.close(code)
		return
	}
	if c.k.markReported() && c.h.d.Links.current(c.k) {
		c.h.d.Links.publish(c.k.login.HostID, true)
	}
}

// answer sends what a session op asks for: error for an invalid end or a foreign session, and
// session.close.
func (c *conn) answer(ctx context.Context, op string, a app.SessionAnswer, code domain.CloseCode, err error) {
	switch {
	case isValidation(err):
		c.k.send(errorFrame(op, codeInvalid))
		return
	case err != nil:
		c.fail(ctx)
		return
	case code != 0:
		c.k.close(code)
		return
	}
	if a.NotFound {
		c.k.send(errorFrame(op, codeNotFound))
	}
	if a.Close != nil {
		c.k.send(sessionCloseFrame(*a.Close))
	}
}

// end runs when the reader returned (Design note 11 of M2-43): the link leaves the registry; when
// it ended without a state change of the host and no newer link of the host is registered, its
// active sessions become detached; the presence turns false.
func (c *conn) end(ctx context.Context) {
	k := c.k
	if k.login.HostID.IsZero() {
		k.close(domain.CodeNormal)
		return
	}
	removed := c.h.d.Links.remove(k)
	code, handled := k.result()
	k.close(domain.CodeNormal)
	if removed && k.isReported() {
		c.h.d.Links.publish(k.login.HostID, false)
	}
	if r := c.h.d.Links.relayOf(); r != nil && removed {
		// The relay closes the streams of the host's sessions (Design note 10 of M2-44); it does
		// nothing for 4110, 4111 and 4112, whose state change ends the sessions.
		r.ControlLost(k.login.HostID, code)
	}
	if !removed || handled || code.ChangesHostState() {
		return
	}
	if err := c.h.d.UseCases.ConnectionLost(ctx, k.login.HostID); err != nil {
		c.h.d.Log.LogAttrs(ctx, slog.LevelError, msgInternal, slog.String(logErrorType, errorInternal))
	}
}

// isValidation reports a validation_failed of a use case: the op is answered with error.
func isValidation(err error) bool {
	e, ok := errs.As(err)
	return ok && e.Code == errs.CodeValidationFailed
}

// keySet is the comparison form of a key set.
func keySet(keys []app.JWK) string {
	b, _ := json.Marshal(keys) // strings only: never fails
	return string(b)
}
