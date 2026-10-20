// SPDX-License-Identifier: Apache-2.0

package wsrelay

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/netip"
	"net/url"
	"slices"
	"strings"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/reqmeta"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
	"github.com/OWNER/bothy/services/server/internal/terminal/app"
	"github.com/OWNER/bothy/services/server/internal/terminal/domain"
)

// StreamUseCases are the decisions of the device stream (app.Service, M2-44).
type StreamUseCases interface {
	StreamAccess(ctx context.Context, a actor.Actor, id ids.ID) (domain.StreamCode, string, error)
	StreamAuthFailed(ctx context.Context, id ids.ID) error
}

// Nonces gives the current DPoP nonce of the server (identity.DPoP.CurrentNonce).
type Nonces interface {
	CurrentNonce() string
}

// StreamConfig holds the configuration of the device stream.
type StreamConfig struct {
	// PublicURL is server.public_url: its origin is the only Origin the upgrade accepts, and the
	// htu of the proof is the HTTPS form of its host plus the stream path.
	PublicURL string
	// OpenTimeout is terminal.open_timeout: identify must arrive within it.
	OpenTimeout time.Duration
	// MaxFrameBytes is sync.ws_max_frame_bytes, the largest text frame before identify.
	MaxFrameBytes int
	// MaxJSONDepth is http.max_json_depth.
	MaxJSONDepth int
}

// StreamDeps are the collaborators of the device stream.
type StreamDeps struct {
	UseCases      StreamUseCases
	Authenticator actor.Authenticator
	Nonces        Nonces
	Relay         *Relay
	// Schemas checks the body of identify (TerminalStreamIdentify).
	Schemas SchemaChecker
	// Timers drive the deadline of identify (terminal.open_timeout); nil means app.RealTimers.
	Timers app.Timers
	// Clock gives hello.server_time; nil means clock.System().
	Clock clock.Clock
	// Log receives panics without content; nil means slog.Default().
	Log *slog.Logger
	// Limiter applies rl.terminal at identify and rl.public_auth to failed credentials (M2-46);
	// nil means NoLimiter.
	Limiter Limiter
}

// StreamHandler is the HTTP handler of GET /api/v1/terminal/sessions/{session_id}/stream (asyncapi
// channel terminalStream, Design note 8 of M2-44): the Origin check before the upgrade
// (SEC-CORS-3), hello with the DPoP nonce, exactly one identify within terminal.open_timeout, the
// access token with its proof of the device, authorize terminal.stream, then the relay. It is
// safe for concurrent use.
type StreamHandler struct {
	cfg    StreamConfig
	d      StreamDeps
	origin *url.URL
}

// NewStreamHandler returns the handler. Errors: a server.public_url that is no absolute http or
// https URL, a limit below 1 or a missing collaborator.
func NewStreamHandler(cfg StreamConfig, d StreamDeps) (*StreamHandler, error) {
	u, err := url.Parse(cfg.PublicURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, errors.New("wsrelay: server.public_url must be an absolute http or https URL")
	}
	if cfg.OpenTimeout <= 0 || cfg.MaxFrameBytes < 1 || cfg.MaxJSONDepth < 1 {
		return nil, errors.New("wsrelay: terminal.open_timeout, sync.ws_max_frame_bytes and http.max_json_depth must be positive")
	}
	if d.UseCases == nil || d.Authenticator == nil || d.Nonces == nil || d.Relay == nil || d.Schemas == nil {
		return nil, errors.New("wsrelay: NewStreamHandler needs the use cases, the authenticator, the nonces, the relay and the schemas")
	}
	if d.Timers == nil {
		d.Timers = app.RealTimers{}
	}
	if d.Clock == nil {
		d.Clock = clock.System()
	}
	if d.Log == nil {
		d.Log = slog.Default()
	}
	if d.Limiter == nil {
		d.Limiter = NoLimiter{}
	}
	return &StreamHandler{cfg: cfg, d: d, origin: u}, nil
}

// StreamSchemaNames returns the schemas the device stream checks, for the validator of
// internal/wire.
func StreamSchemaNames() []string { return []string{"TerminalStreamIdentify"} }

// sameOrigin reports whether the Origin value o names the scheme, host and port of server.public_url.
func (h *StreamHandler) sameOrigin(o string) bool {
	u, err := url.Parse(o)
	if err != nil || u.Host == "" || u.Path != "" || u.RawQuery != "" || u.User != nil {
		return false
	}
	return strings.EqualFold(u.Scheme, h.origin.Scheme) && strings.EqualFold(u.Hostname(), h.origin.Hostname()) &&
		port(u) == port(h.origin)
}

// port returns the port of u, the default port of its scheme when none is given.
func port(u *url.URL) string {
	if p := u.Port(); p != "" {
		return p
	}
	if strings.EqualFold(u.Scheme, "https") {
		return "443"
	}
	return "80"
}

// htu returns the htu of the stream of the session id: the HTTPS form of the host of
// server.public_url plus the path, also behind a TLS terminator.
func (h *StreamHandler) htu(id ids.ID) string {
	host := h.origin.Host
	if h.origin.Scheme == "http" && h.origin.Port() == "80" {
		host = h.origin.Hostname()
	}
	return "https://" + host + "/api/v1/terminal/sessions/" + id.String() + "/stream"
}

// ServeHTTP implements http.Handler: a present Origin other than the origin of server.public_url
// (several Origin headers included) answers 403 forbidden before the upgrade (SEC-CORS-3); the
// library's origin verification stays on; compression is off.
func (h *StreamHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	origins := r.Header.Values("Origin")
	if len(origins) > 1 || len(origins) == 1 && !h.sameOrigin(origins[0]) {
		httpserver.WriteProblem(w, r, errs.New(errs.CodeForbidden, ""))
		return
	}
	id, err := ids.Parse(r.PathValue(sessionPathValue))
	if err != nil || id.IsZero() {
		httpserver.WriteProblem(w, r, errs.New(errs.CodeNotFound, ""))
		return
	}
	ws, err := websocket.Accept(w, r, &websocket.AcceptOptions{OriginPatterns: []string{h.origin.Host}, CompressionMode: websocket.CompressionDisabled})
	if err != nil {
		return // Accept has answered the request
	}
	ws.SetReadLimit(-1)
	peer, _ := netip.ParseAddrPort(r.RemoteAddr)
	ctx := reqmeta.WithTransport(authorize.WithoutRequestScope(context.WithoutCancel(r.Context())),
		reqmeta.Transport{Peer: peer.Addr().Unmap(), Header: r.Header.Clone(), Action: string(authorize.ActionTerminalStream)})
	h.serve(ctx, ws, id)
}

// The ops of the device stream (asyncapi terminalStream).
const (
	opTerminalHello    = "hello"
	opTerminalIdentify = "identify"
)

// closedAlready marks an identify whose connection already ended or was closed by its deadline.
const closedAlready = -1

// serve runs one device stream: hello, identify, the decision, then the relay.
func (h *StreamHandler) serve(ctx context.Context, ws *websocket.Conn, id ids.ID) {
	defer func() {
		if p := recover(); p != nil {
			h.d.Log.LogAttrs(ctx, slog.LevelError, msgPanic, slog.String(logErrorType, errorPanic))
			_ = ws.Close(websocket.StatusCode(domain.CodeUnknownError), "")
		}
	}()
	hello := encode(opTerminalHello, rest.TerminalStreamHello{
		ProtocolVersions: domain.SupportedProtocolVersions(), DpopNonce: h.d.Nonces.CurrentNonce(), ServerTime: stamp(h.d.Clock.Now()),
	})
	if err := ws.Write(ctx, websocket.MessageText, hello); err != nil {
		_ = ws.CloseNow()
		return
	}
	a, code, reason := h.identify(ctx, ws, id)
	if code == closedAlready {
		return
	}
	if code != 0 {
		_ = ws.Close(websocket.StatusCode(code), reason)
		return
	}
	sd, c := h.d.Relay.attachDevice(id, a.DeviceID, ws)
	if sd == nil {
		_ = ws.Close(websocket.StatusCode(c), domain.StreamCode(c).Name())
		return
	}
	runSide(ctx, h.d.Relay, sd)
}

// identify reads identify within terminal.open_timeout and decides it (the table of Design note
// 8). It returns the principal, or the close code with its reason. Presented credentials reserve
// one token of rl.public_auth of the client address first and keep it when the check fails (4004
// or 4012); with the bucket empty the stream closes with 4008 before any check (M2-46 Design note
// 10 c).
func (h *StreamHandler) identify(ctx context.Context, ws *websocket.Conn, id ids.ID) (actor.Actor, int, string) {
	in, code, reason := h.readIdentify(ctx, ws, id)
	if code != 0 {
		return actor.Actor{}, code, reason
	}
	if in.AccessToken == nil || *in.AccessToken == "" || in.Dpop == nil {
		// The console path (cookie session) is M6-18's; any other identify without a token fails.
		return h.authFailed(ctx, id)
	}
	ok, retry, settle := h.d.Limiter.ReserveCredentials(clientIP(ctx))
	if !ok {
		return actor.Actor{}, int(domain.CodeRateLimited), retryText(retry)
	}
	a, code, reason := h.checkCredentials(ctx, in, id)
	settle(code == int(domain.StreamAuthFailed) || code == int(domain.StreamRevoked))
	return a, code, reason
}

// checkCredentials verifies the access token and its proof of identify, applies rl.terminal to
// the human principal (after the credential check, before authorize; 05 section 3) and authorizes
// terminal.stream.
func (h *StreamHandler) checkCredentials(ctx context.Context, in rest.TerminalStreamIdentify, id ids.ID) (actor.Actor, int, string) {
	ip, _ := reqmeta.ClientIP(ctx)
	a, err := h.d.Authenticator.Authenticate(ctx, actor.Credentials{Method: http.MethodGet, HTU: h.htu(id),
		Authorization: "DPoP " + *in.AccessToken, DPoP: *in.Dpop, DPoPProofs: []string{*in.Dpop}, ClientIP: ip})
	switch {
	case err != nil && isSessionRevoked(err):
		_ = h.d.UseCases.StreamAuthFailed(ctx, id)
		return actor.Actor{}, int(domain.StreamRevoked), domain.StreamRevoked.Name()
	case err != nil:
		return h.authFailed(ctx, id)
	case a.Kind == actor.KindUser && a.DeviceID != ids.ID(in.DeviceId):
		return h.authFailed(ctx, id)
	}
	if a.Kind == actor.KindUser {
		if ok, retry := h.d.Limiter.AllowTerminal(a.ID); !ok {
			return actor.Actor{}, int(domain.CodeRateLimited), retryText(retry)
		}
	}
	// Agents, apps and personal tokens reach authorize too, whose gate answers them (TM-T17).
	sc, reason, err := h.d.UseCases.StreamAccess(actor.WithActor(ctx, a), a, id)
	switch {
	case err != nil:
		return actor.Actor{}, int(domain.CodeUnknownError), ""
	case sc != 0:
		return actor.Actor{}, int(sc), nameOr(sc, reason)
	case a.Kind != actor.KindUser:
		return actor.Actor{}, int(domain.StreamAuthFailed), domain.StreamAuthFailed.Name()
	}
	return a, 0, ""
}

// readIdentify reads identify within terminal.open_timeout (Timers) and decodes it. It returns
// the frame, or the close code: closedAlready when the connection ended or the deadline closed it
// with 4004, 4002 for a frame that is no valid identify of protocol version 1.
func (h *StreamHandler) readIdentify(ctx context.Context, ws *websocket.Conn, id ids.ID) (rest.TerminalStreamIdentify, int, string) {
	// The deadline closes the connection itself; cancelling the read would tear it down without
	// a close frame.
	var timedOut atomic.Bool
	stop := h.d.Timers.AfterFunc(h.cfg.OpenTimeout, func() {
		timedOut.Store(true)
		_, code, reason := h.authFailed(ctx, id)
		go func() { _ = ws.Close(websocket.StatusCode(code), reason) }()
	})
	typ, rd, err := ws.Reader(ctx)
	if err != nil || timedOut.Load() {
		stop()
		return rest.TerminalStreamIdentify{}, closedAlready, ""
	}
	b, err := io.ReadAll(io.LimitReader(rd, int64(h.cfg.MaxFrameBytes)+1))
	if !stop() {
		return rest.TerminalStreamIdentify{}, closedAlready, ""
	}
	if err != nil || typ != websocket.MessageText || len(b) > h.cfg.MaxFrameBytes {
		return rest.TerminalStreamIdentify{}, int(domain.StreamDecodeError), domain.StreamDecodeError.Name()
	}
	in, ok := h.decodeIdentify(b)
	if !ok || !slices.Contains(domain.SupportedProtocolVersions(), in.ProtocolVersion) {
		return rest.TerminalStreamIdentify{}, int(domain.StreamDecodeError), domain.StreamDecodeError.Name()
	}
	return in, 0, ""
}

// isSessionRevoked reports the answer of the authenticator for a revoked session (4012).
func isSessionRevoked(err error) bool {
	e, ok := errs.As(err)
	return ok && e.Code == errs.CodeSessionRevoked
}

// nameOr returns reason, or the name of code without one.
func nameOr(code domain.StreamCode, reason string) string {
	if reason != "" {
		return reason
	}
	return code.Name()
}

// authFailed records the denial of an identify that did not authenticate and returns 4004.
func (h *StreamHandler) authFailed(ctx context.Context, id ids.ID) (actor.Actor, int, string) {
	if err := h.d.UseCases.StreamAuthFailed(ctx, id); err != nil {
		h.d.Log.LogAttrs(ctx, slog.LevelError, msgInternal, slog.String(logErrorType, errorInternal))
	}
	return actor.Actor{}, int(domain.StreamAuthFailed), domain.StreamAuthFailed.Name()
}

// decodeIdentify decodes identify strictly: SEC-IN-2, exactly the members op and d, op identify,
// d against TerminalStreamIdentify without unknown members.
func (h *StreamHandler) decodeIdentify(b []byte) (rest.TerminalStreamIdentify, bool) {
	if httpserver.CheckJSON(b, h.cfg.MaxJSONDepth) != nil {
		return rest.TerminalStreamIdentify{}, false
	}
	var env envelope
	if err := strict(b, &env); err != nil || env.Op == nil || *env.Op != opTerminalIdentify || len(env.D) == 0 {
		return rest.TerminalStreamIdentify{}, false
	}
	if h.d.Schemas.Validate("TerminalStreamIdentify", env.D) != nil {
		return rest.TerminalStreamIdentify{}, false
	}
	var d rest.TerminalStreamIdentify
	dec := json.NewDecoder(bytes.NewReader(env.D))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&d); err != nil {
		return rest.TerminalStreamIdentify{}, false
	}
	return d, true
}
