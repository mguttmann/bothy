// SPDX-License-Identifier: Apache-2.0

package ws

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	gosync "sync"
	"time"

	"github.com/coder/websocket"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/OWNER/bothy/services/server/internal/gateway/app"
	"github.com/OWNER/bothy/services/server/internal/gateway/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/reqmeta"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
)

// Names of the span, the log messages and keys of the adapter (docs/observability/signals.md).
const (
	SpanName            = "gateway.connect"
	instrumentationName = "github.com/OWNER/bothy/services/server/internal/gateway"
	attrOutcome         = "bothy.outcome"
	attrCloseCode       = "bothy.close.code"
	attrPrincipalKind   = "bothy.principal.kind"
	logKeyErrorType     = "error.type"
	msgEncodeFailed     = "gateway frame not encodable"
	msgPanic            = "panic recovered"
	errorTypeEncode     = "encode"
	errorTypePanic      = "panic"
	actionConnect       = "gateway.connect"
)

// Config holds the configuration keys of the adapter.
type Config struct {
	// PublicURL is server.public_url: its origin is the only Origin the upgrade accepts.
	PublicURL string
	// MaxFrameBytes is sync.ws_max_frame_bytes.
	MaxFrameBytes int
	// MaxJSONDepth is http.max_json_depth, the nesting limit of SEC-IN-2 for frames as for bodies.
	MaxJSONDepth int
	// OutboundQueueMax is sync.outbound_queue_max.
	OutboundQueueMax int
	// WriteTimeout is gateway.write_timeout.
	WriteTimeout time.Duration
}

// Deps are the collaborators of the adapter.
type Deps struct {
	// Gateway holds the use cases.
	Gateway *app.Gateway
	// Snapshot converts the snapshot of ready.
	Snapshot SnapshotEncoder
	// Clock drives the deadlines of the connections; nil means clock.System().
	Clock clock.Clock
	// Log receives panics and encoding failures; nil means slog.Default().
	Log *slog.Logger
	// Tracer is the tracer provider of the span gateway.connect; nil means the global one.
	Tracer trace.TracerProvider
}

// Handler is the HTTP handler of GET /api/v1/gateway (04 section 6, 19 SEC-CORS-3): it checks
// the Origin, upgrades with github.com/coder/websocket (compression off, origin verification
// on) and runs one connection per request with a reader, a writer and a deadline goroutine until
// the connection ends. It is safe for concurrent use.
type Handler struct {
	cfg    Config
	d      Deps
	origin *url.URL
	// beforeWrite is a test hook passed to every connection's writer; nil in production.
	beforeWrite func()
}

// NewHandler returns the handler. Errors: a server.public_url that is no absolute http or https
// URL, a limit below 1 or a missing gateway or snapshot encoder.
func NewHandler(cfg Config, d Deps) (*Handler, error) {
	u, err := url.Parse(cfg.PublicURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, errors.New("gateway: server.public_url must be an absolute http or https URL")
	}
	if cfg.MaxFrameBytes < 1 || cfg.MaxJSONDepth < 1 || cfg.OutboundQueueMax < 1 || cfg.WriteTimeout <= 0 {
		return nil, errors.New("gateway: sync.ws_max_frame_bytes, http.max_json_depth, sync.outbound_queue_max and gateway.write_timeout must be positive")
	}
	if d.Gateway == nil || d.Snapshot == nil {
		return nil, errors.New("gateway: NewHandler needs the gateway and the snapshot encoder")
	}
	if d.Clock == nil {
		d.Clock = clock.System()
	}
	if d.Log == nil {
		d.Log = slog.Default()
	}
	if d.Tracer == nil {
		d.Tracer = otel.GetTracerProvider()
	}
	return &Handler{cfg: cfg, d: d, origin: u}, nil
}

// HTU returns the htu of the gateway for server.public_url: the HTTPS form of its origin plus
// /api/v1/gateway, also when the server itself speaks HTTP behind a TLS terminator (04 section 6.3
// step 3). Errors: a URL that does not parse or has no host.
func HTU(publicURL string) (string, error) {
	u, err := url.Parse(publicURL)
	if err != nil || u.Host == "" {
		return "", fmt.Errorf("gateway: server.public_url %q has no host", publicURL)
	}
	host := u.Host
	if u.Scheme == "http" && u.Port() == "80" {
		host = u.Hostname()
	}
	return "https://" + host + "/api/v1/gateway", nil
}

// sameOrigin reports whether the Origin value o names the scheme, host and port of server.public_url.
func (h *Handler) sameOrigin(o string) bool {
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

// ServeHTTP implements http.Handler. A present Origin other than the origin of server.public_url
// (several Origin headers included) answers 403 forbidden before the upgrade (SEC-CORS-3); nothing
// else is checked there. After the upgrade the connection runs until it ends.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if origins := r.Header.Values("Origin"); len(origins) > 1 || len(origins) == 1 && !h.sameOrigin(origins[0]) {
		httpserver.WriteProblem(w, r, errs.New(errs.CodeForbidden, ""))
		return
	}
	ws, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns:  []string{h.origin.Host},
		CompressionMode: websocket.CompressionDisabled,
	})
	if err != nil {
		return // Accept has answered the request
	}
	// The frame limit is enforced by the reader below with its own close code (4002), not by
	// the library's 1009.
	ws.SetReadLimit(-1)
	// The TCP peer, the header fields of the upgrade request and the action reach identity as
	// on every REST request (the authenticate stage does it there): the session binding of
	// trusted header logins checks them on every identify, resume and reauth (M1-43).
	// The connection outlives the upgrade request: it drops the request memo of the chain, so
	// identify, heartbeats, ops, catch-up pages and live checks read the current rights version
	// (AZ-P9), each op and page with its own memo.
	peer, _ := netip.ParseAddrPort(r.RemoteAddr)
	base := reqmeta.WithTransport(authorize.WithoutRequestScope(context.WithoutCancel(r.Context())),
		reqmeta.Transport{Peer: peer.Addr().Unmap(), Header: r.Header.Clone(), Action: actionConnect})
	ctx, span := h.d.Tracer.Tracer(instrumentationName).Start(base, SpanName, trace.WithSpanKind(trace.SpanKindServer))
	h.run(ctx, ws, span)
}

// run serves one connection and ends the span with its outcome.
func (h *Handler) run(ctx context.Context, ws *websocket.Conn, span trace.Span) {
	conn := newConn(ws, h.cfg.OutboundQueueMax, h.d.Snapshot, h.d.Log)
	conn.beforeWrite = h.beforeWrite
	var wg gosync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer h.recover(ctx, conn)
		conn.write(h.cfg.WriteTimeout)
	}()
	c, err := h.d.Gateway.Open(ctx, conn)
	if err != nil {
		conn.Close(domain.CloseUnknownError, domain.CloseUnknownError.Name())
		conn.closed.Wait()
		wg.Wait()
		h.end(span, domain.CloseUnknownError, false, "")
		return
	}
	conn.bind(c)
	poke := make(chan struct{}, 1)
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer h.recover(ctx, conn)
		h.deadlines(ctx, c, conn, poke)
	}()
	h.read(ctx, c, conn, poke)
	conn.abort()
	h.d.Gateway.Closed(c)
	conn.closed.Wait()
	wg.Wait()
	code, ready, kind := c.Result()
	h.end(span, code, ready, string(kind))
}

// end sets the attributes of the span gateway.connect and ends it.
func (h *Handler) end(span trace.Span, code domain.CloseCode, ready bool, kind string) {
	if kind == "" {
		kind = "anonymous"
	}
	span.SetAttributes(attribute.String(attrOutcome, domain.Outcome(code, ready)), attribute.Int(attrCloseCode, int(code)),
		attribute.String(attrPrincipalKind, kind))
	span.End()
}

// recover turns a panic of a connection goroutine into the close 4000 (02 "Panics"); the log line
// names neither frame contents nor the stack.
func (h *Handler) recover(ctx context.Context, conn *wsConn) {
	if p := recover(); p != nil {
		h.d.Log.LogAttrs(ctx, slog.LevelError, msgPanic, slog.String(logKeyErrorType, errorTypePanic))
		conn.closeApp(domain.CloseUnknownError)
	}
}

// read is the reader loop: one frame after the other, in order. A binary frame, a frame above
// sync.ws_max_frame_bytes and a frame that is not one JSON object {op, d} close with 4002;
// every other frame goes to the gateway. It returns when the connection ends.
func (h *Handler) read(ctx context.Context, c *app.Connection, conn *wsConn, poke chan<- struct{}) {
	defer h.recover(ctx, conn)
	for !c.Closing() {
		typ, rd, err := conn.ws.Reader(ctx)
		if err != nil {
			return
		}
		if typ != websocket.MessageText {
			c.Close(domain.CloseDecodeError, reasonBinaryFrame)
			continue
		}
		b, err := io.ReadAll(io.LimitReader(rd, int64(h.cfg.MaxFrameBytes)+1))
		if err != nil {
			return
		}
		if len(b) > h.cfg.MaxFrameBytes {
			c.Close(domain.CloseDecodeError, reasonFrameTooLarge)
			continue
		}
		op, d, err := decodeEnvelope(b, h.cfg.MaxJSONDepth)
		if err != nil {
			c.Close(domain.CloseDecodeError, "")
			continue
		}
		h.d.Gateway.Handle(ctx, c, op, d)
		select {
		case poke <- struct{}{}:
		default:
		}
	}
	// After the close the reader stops: the close handshake of coder/websocket reads until the
	// peer's close frame itself, and a frame left half read (binary or too large) must not be
	// parsed as the next one.
}

// deadlines is the deadline goroutine: it ticks the gateway at the instant the connection asks
// for and after every op, until the connection is closed.
func (h *Handler) deadlines(ctx context.Context, c *app.Connection, conn *wsConn, poke <-chan struct{}) {
	timer := time.NewTimer(time.Hour)
	defer timer.Stop()
	for {
		next := h.d.Gateway.Tick(ctx, c, h.d.Clock.Now())
		if next.IsZero() {
			return
		}
		timer.Reset(max(next.Sub(h.d.Clock.Now()), time.Millisecond))
		select {
		case <-conn.done:
			return
		case <-poke:
		case <-timer.C:
		}
	}
}
