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
	"slices"
	gosync "sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
	"github.com/OWNER/bothy/services/server/internal/terminal/app"
	"github.com/OWNER/bothy/services/server/internal/terminal/domain"
)

// The relay of computer sessions (CU-04, docs/spec/27-computer-use.md 5.1 steps 7 and 8, 6.1,
// 8.3): the attach connection of bothy-computer-mcp (asyncapi computerAttach) and the computer data
// connection of the host service (hostComputerData) pair in memory on the node the proxy routes the
// session to, like the terminal streams (24 9). The relay forwards every binary message unchanged
// and reads none: it counts only lengths and times (no frame content enters a log, a metric, the
// audit or an event; sentinel test CU-T18). There is no reattach: when a side closes, the session
// ends (connection_lost); a text frame or a message above the Noise limit ends it with error and
// stream_fault.

// ComputerUseCases are the decisions of the computer relay (app.Service).
type ComputerUseCases interface {
	AttachComputer(ctx context.Context, sessionID ids.ID, token string) (app.AttachOutcome, error)
	ComputerDataSession(ctx context.Context, l app.Login, id ids.ID) (bool, error)
	EndComputerSession(ctx context.Context, id ids.ID, end domain.ComputerEnd) error
}

// Compile-time check: the use cases of the module.
var _ ComputerUseCases = (*app.Service)(nil)

// ComputerMetrics records the relay counts of computer sessions: lengths only (the instruments
// bothy.terminal.computer.relay.size and .messages of docs/observability/signals.md 4.3a); never
// content.
type ComputerMetrics interface {
	// ComputerRelayed counts one forwarded message of n bytes in direction ("to_host" or
	// "to_agent").
	ComputerRelayed(ctx context.Context, direction string, n int)
}

// NoComputerMetrics records nothing.
type NoComputerMetrics struct{}

// ComputerRelayed implements ComputerMetrics.
func (NoComputerMetrics) ComputerRelayed(context.Context, string, int) {}

// The directions of ComputerMetrics.
const (
	directionToHost  = "to_host"
	directionToAgent = "to_agent"
)

// ComputerRelayConfig holds the configuration of the computer relay.
type ComputerRelayConfig struct {
	// OpenTimeout is computer.open_timeout: the deadline of the pairing of a slot.
	OpenTimeout time.Duration
	// WriteTimeout is gateway.write_timeout, the bound of one forwarded write.
	WriteTimeout time.Duration
}

// ComputerRelay is the in-memory relay of the computer sessions of one node. It is safe for
// concurrent use.
type ComputerRelay struct {
	cfg     ComputerRelayConfig
	uc      ComputerUseCases
	links   *Links
	clk     clock.Clock
	log     *slog.Logger
	metrics ComputerMetrics
	mu      gosync.Mutex
	slots   map[ids.ID]*cslot
	stopped atomic.Bool
}

// cslot is the slot of one computer session on this node.
type cslot struct {
	id, host     ids.ID
	agent, hst   *cside
	paired, done bool
	timer        *time.Timer
	// pending are the messages a side sent before its counterpart arrived (the first Noise
	// message of bothy-computer-mcp), at most computerPendingMax, written at the pairing.
	pending []pendingMsg
	// fmu orders the writes of the held messages before every later write.
	fmu gosync.Mutex
}

// pendingMsg is one message held until the pairing, with the side it came from.
type pendingMsg struct {
	from string
	msg  []byte
}

// computerPendingMax bounds the messages held before the pairing: the handshake needs one from
// bothy-computer-mcp; a side that sends more before its counterpart arrives is a stream fault.
const computerPendingMax = 2

// cside is one connection of a slot; writes are serialized by mu.
type cside struct {
	ws   *websocket.Conn
	mu   gosync.Mutex
	once gosync.Once
}

// close closes the side once with code and reason.
func (c *cside) close(code int, reason string) {
	c.once.Do(func() { go func() { _ = c.ws.Close(websocket.StatusCode(code), reason) }() })
}

// NewComputerRelay returns the relay. Errors: a deadline that is not positive or a missing
// collaborator.
func NewComputerRelay(cfg ComputerRelayConfig, uc ComputerUseCases, links *Links, clk clock.Clock, log *slog.Logger, m ComputerMetrics) (*ComputerRelay, error) {
	if cfg.OpenTimeout <= 0 || cfg.WriteTimeout <= 0 {
		return nil, errors.New("wsrelay: computer.open_timeout and gateway.write_timeout must be positive")
	}
	if uc == nil || links == nil {
		return nil, errors.New("wsrelay: NewComputerRelay needs the use cases and the links")
	}
	if clk == nil {
		clk = clock.System()
	}
	if log == nil {
		log = slog.Default()
	}
	if m == nil {
		m = NoComputerMetrics{}
	}
	return &ComputerRelay{cfg: cfg, uc: uc, links: links, clk: clk, log: log, metrics: m, slots: map[ids.ID]*cslot{}}, nil
}

// Compile-time check: the relay is the port of the use cases.
var _ app.ComputerRelay = (*ComputerRelay)(nil)

// ExpectComputer implements app.ComputerRelay: the slot of the session with its deadline; a slot
// that exists keeps its deadline.
func (r *ComputerRelay) ExpectComputer(e app.ComputerExpectation) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.slotLocked(e.SessionID, e.HostID, e.Deadline)
}

// slotLocked returns the slot of id, created with its deadline when there is none. r.mu is held.
func (r *ComputerRelay) slotLocked(id, host ids.ID, deadline time.Time) *cslot {
	if s := r.slots[id]; s != nil {
		return s
	}
	s := &cslot{id: id, host: host}
	r.slots[id] = s
	wait := deadline.Sub(r.clk.Now())
	if wait < 0 {
		wait = 0
	}
	s.timer = time.AfterFunc(wait, func() { r.deadline(id) })
	return s
}

// deadline ends a slot that did not pair in time: both lone sides close with 4102 and the session
// ends open_failed with open_timeout (27 5.1 step 9). A slot without any side is only dropped:
// with several nodes every node expects the session, and only the node the proxy routes it to
// pairs it, so the others must not end it; an opening that paired nowhere ends by terminal.sweep
// (D-CU-04-12). The end itself changes only a session still in opening (EndComputerSession).
func (r *ComputerRelay) deadline(id ids.ID) {
	r.mu.Lock()
	s := r.slots[id]
	if s == nil || s.paired || s.done {
		r.mu.Unlock()
		return
	}
	s.done = true
	delete(r.slots, id)
	sides := []*cside{s.agent, s.hst}
	r.mu.Unlock()
	if s.agent == nil && s.hst == nil {
		return
	}
	for _, sd := range sides {
		if sd != nil {
			sd.close(int(domain.StreamHostUnavailable), domain.StreamHostUnavailable.Name())
		}
	}
	r.end(id, domain.ComputerEnd{Reason: domain.ComputerOpenFailed, Detail: domain.ComputerOpenTimeout})
}

// end ends the session through the use cases; a failure is logged without content.
func (r *ComputerRelay) end(id ids.ID, end domain.ComputerEnd) {
	if err := r.uc.EndComputerSession(context.Background(), id, end); err != nil {
		r.log.LogAttrs(context.Background(), slog.LevelError, msgInternal, slog.String(logErrorType, errorInternal))
	}
}

// The sides of a slot.
const (
	sideAgent = "agent"
	sideHost  = "host"
)

// attach takes ws as the side kind of the session id; it returns nil and the close code when the
// side is taken or the relay stopped. A slot is created on demand with the deadline from now, so
// a side that arrives before the signal of the opening still pairs.
func (r *ComputerRelay) attach(kind string, id, host ids.ID, ws *websocket.Conn) (*cside, *cslot, int) {
	if r.stopped.Load() {
		return nil, nil, int(domain.CodeServerRestart)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	s := r.slotLocked(id, host, r.clk.Now().Add(r.cfg.OpenTimeout))
	sd := &cside{ws: ws}
	switch {
	case s.done:
		return nil, nil, int(domain.StreamTerminalClosed)
	case kind == sideAgent && s.agent == nil:
		s.agent = sd
	case kind == sideHost && s.hst == nil:
		s.hst = sd
	default:
		return nil, nil, int(domain.StreamNormal)
	}
	if s.agent != nil && s.hst != nil && !s.paired {
		s.paired = true
		s.timer.Stop()
	}
	return sd, s, 0
}

// run reads the side sd of slot s until it ends: each binary message goes to the other side
// unchanged once both are paired; before the pairing at most computerPendingMax messages are held
// and written at the pairing, any further one is a stream fault. The end of the side ends the
// session.
func (r *ComputerRelay) run(ctx context.Context, kind string, s *cslot, sd *cside) {
	r.flush(ctx, s)
	for {
		msg, err := readFrame(ctx, sd.ws)
		switch {
		case errors.Is(err, errProtocol):
			r.closeSlot(s, int(domain.CodeDecodeError), domain.ComputerEnd{Reason: domain.ComputerError, Detail: domain.ComputerStreamFault})
			return
		case err != nil:
			r.sideClosed(s, kind, err)
			return
		}
		if !r.forward(ctx, s, kind, msg) {
			r.closeSlot(s, int(domain.CodeDecodeError), domain.ComputerEnd{Reason: domain.ComputerError, Detail: domain.ComputerStreamFault})
			return
		}
	}
}

// forward writes msg to the other side of s, or holds it before the pairing; it reports false for a
// message above the hold or of an ended slot. Only the length is counted.
func (r *ComputerRelay) forward(ctx context.Context, s *cslot, from string, msg []byte) bool {
	r.mu.Lock()
	if s.done {
		r.mu.Unlock()
		return false
	}
	if !s.paired || len(s.pending) > 0 {
		ok := len(s.pending) < computerPendingMax
		if ok {
			s.pending = append(s.pending, pendingMsg{from: from, msg: msg})
		}
		r.mu.Unlock()
		if ok {
			r.flush(ctx, s)
		}
		return ok
	}
	r.mu.Unlock()
	s.fmu.Lock()
	r.write(ctx, s, from, msg)
	s.fmu.Unlock()
	return true
}

// flush writes the held messages of a paired slot in order, once; later writes wait for it.
func (r *ComputerRelay) flush(ctx context.Context, s *cslot) {
	s.fmu.Lock()
	defer s.fmu.Unlock()
	r.mu.Lock()
	if !s.paired || s.done || len(s.pending) == 0 {
		r.mu.Unlock()
		return
	}
	held := s.pending
	s.pending = nil
	r.mu.Unlock()
	for _, p := range held {
		r.write(ctx, s, p.from, p.msg)
	}
}

// write writes msg from the side from to the other side of the paired slot s and counts its
// length.
func (r *ComputerRelay) write(ctx context.Context, s *cslot, from string, msg []byte) {
	r.mu.Lock()
	to, dir := s.hst, directionToHost
	if from == sideHost {
		to, dir = s.agent, directionToAgent
	}
	r.mu.Unlock()
	if to == nil {
		return
	}
	to.mu.Lock()
	wctx, cancel := context.WithTimeout(ctx, r.cfg.WriteTimeout)
	err := to.ws.Write(wctx, websocket.MessageBinary, msg)
	cancel()
	to.mu.Unlock()
	if err == nil {
		r.metrics.ComputerRelayed(ctx, dir, len(msg))
	}
}

// sideClosed ends the session of a side that closed (27 8.3 connection_lost); a host that refused
// the handshake with 4101 or 4104 and a detail of 27 5.3 as the reason ends it open_failed with
// that detail, and the agent gets the same code and reason.
func (r *ComputerRelay) sideClosed(s *cslot, kind string, err error) {
	end := domain.ComputerEnd{Reason: domain.ComputerConnectionLost}
	code := int(domain.StreamTerminalClosed)
	var ce websocket.CloseError
	if kind == sideHost && errors.As(err, &ce) && (int(ce.Code) == int(domain.StreamTokenInvalid) || int(ce.Code) == int(domain.StreamHandshakeFailed)) &&
		domain.ValidateComputerEnd(domain.ComputerOpenFailed, domain.ComputerCloseDetail(ce.Reason)) == nil && ce.Reason != "" {
		end = domain.ComputerEnd{Reason: domain.ComputerOpenFailed, Detail: domain.ComputerCloseDetail(ce.Reason)}
		code = int(ce.Code)
	}
	r.closeSlot(s, code, end)
}

// closeSlot ends the slot once: both sides close with code and the session ends with end.
func (r *ComputerRelay) closeSlot(s *cslot, code int, end domain.ComputerEnd) {
	r.mu.Lock()
	if s.done {
		r.mu.Unlock()
		return
	}
	s.done = true
	s.timer.Stop()
	if r.slots[s.id] == s {
		delete(r.slots, s.id)
	}
	sides := []*cside{s.agent, s.hst}
	r.mu.Unlock()
	reason := string(end.Reason)
	if end.Detail != "" {
		reason = string(end.Detail)
	}
	for _, sd := range sides {
		if sd != nil {
			sd.close(code, reason)
		}
	}
	r.end(s.id, end)
}

// computerSessionEnd is the part of the payload ComputerSession of computer_session.changed the
// observer reads.
type computerSessionEnd struct {
	HostID      string `json:"host_id"`
	State       string `json:"state"`
	CloseReason string `json:"close_reason"`
}

// ObserveLogEvent is the consumer of the log tail for computer sessions (27 8.4 item 1): a
// computer_session.changed in state closed closes both sides of the session on this node with 4100
// and the close reason, drops the slot and sends computer.close to the host service (a no-op
// without its link on this node). Every other event is ignored; it never blocks on the network.
func (r *ComputerRelay) ObserveLogEvent(ctx context.Context, typ string, resourceID ids.ID, payload []byte) {
	if typ != eventComputerChg {
		return
	}
	var p computerSessionEnd
	if err := json.Unmarshal(payload, &p); err != nil || p.State != domain.ComputerClosed {
		return
	}
	host, err := ids.Parse(p.HostID)
	if err != nil {
		return
	}
	r.mu.Lock()
	s := r.slots[resourceID]
	var sides []*cside
	if s != nil {
		s.done = true
		s.timer.Stop()
		delete(r.slots, resourceID)
		sides = []*cside{s.agent, s.hst}
	}
	r.mu.Unlock()
	for _, sd := range sides {
		if sd != nil {
			sd.close(int(domain.StreamTerminalClosed), p.CloseReason)
		}
	}
	_ = r.links.CloseComputer(ctx, host, resourceID, domain.ComputerCloseReason(p.CloseReason))
}

// Shutdown closes every side with 4013; the sessions end with connection_lost through the readers.
func (r *ComputerRelay) Shutdown(context.Context) error {
	r.stopped.Store(true)
	r.mu.Lock()
	var sides []*cside
	for _, s := range r.slots {
		sides = append(sides, s.agent, s.hst)
	}
	r.mu.Unlock()
	for _, sd := range sides {
		if sd != nil {
			sd.close(int(domain.CodeServerRestart), "")
		}
	}
	return nil
}

// eventComputerChg is the event type the observer reads.
const eventComputerChg = "computer_session.changed"

// The ops of the attach connection (asyncapi computerAttach).
const (
	opComputerHello    = "hello"
	opComputerIdentify = "identify"
)

// AttachConfig holds the configuration of the attach connection.
type AttachConfig struct {
	// OpenTimeout is computer.open_timeout: identify must arrive within it.
	OpenTimeout time.Duration
	// MaxFrameBytes is sync.ws_max_frame_bytes, the largest text frame before identify.
	MaxFrameBytes int
	// MaxJSONDepth is http.max_json_depth.
	MaxJSONDepth int
}

// AttachHandler is the HTTP handler of GET /api/v1/computer/sessions/{session_id}/attach (asyncapi
// computerAttach, 27 5.4): hello, exactly one identify with the attach token within
// computer.open_timeout, the decision of computer.attach, then the relay. It is safe for
// concurrent use.
type AttachHandler struct {
	cfg     AttachConfig
	uc      ComputerUseCases
	relay   *ComputerRelay
	schemas SchemaChecker
	clk     clock.Clock
	log     *slog.Logger
}

// AttachSchemaNames returns the schemas the attach connection checks, for the validator of
// internal/wire.
func AttachSchemaNames() []string { return []string{"ComputerAttachIdentify"} }

// NewAttachHandler returns the handler. Errors: a limit below 1 or a missing collaborator.
func NewAttachHandler(cfg AttachConfig, uc ComputerUseCases, relay *ComputerRelay, schemas SchemaChecker, clk clock.Clock, log *slog.Logger) (*AttachHandler, error) {
	if cfg.OpenTimeout <= 0 || cfg.MaxFrameBytes < 1 || cfg.MaxJSONDepth < 1 {
		return nil, errors.New("wsrelay: computer.open_timeout, sync.ws_max_frame_bytes and http.max_json_depth must be positive")
	}
	if uc == nil || relay == nil || schemas == nil {
		return nil, errors.New("wsrelay: NewAttachHandler needs the use cases, the relay and the schemas")
	}
	if clk == nil {
		clk = clock.System()
	}
	if log == nil {
		log = slog.Default()
	}
	return &AttachHandler{cfg: cfg, uc: uc, relay: relay, schemas: schemas, clk: clk, log: log}, nil
}

// ServeHTTP implements http.Handler. bothy-computer-mcp sends no Origin; a request with one passes
// only the library's same-host check.
func (h *AttachHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	id, err := ids.Parse(r.PathValue(sessionPathValue))
	if err != nil || id.IsZero() {
		httpserver.WriteProblem(w, r, errs.New(errs.CodeNotFound, ""))
		return
	}
	ws, err := websocket.Accept(w, r, &websocket.AcceptOptions{CompressionMode: websocket.CompressionDisabled})
	if err != nil {
		return // Accept has answered the request
	}
	ws.SetReadLimit(-1)
	h.serve(context.WithoutCancel(r.Context()), ws, id)
}

// serve runs one attach connection.
func (h *AttachHandler) serve(ctx context.Context, ws *websocket.Conn, id ids.ID) {
	defer func() {
		if recover() != nil {
			h.log.LogAttrs(ctx, slog.LevelError, msgPanic, slog.String(logErrorType, errorPanic))
			_ = ws.Close(websocket.StatusCode(domain.CodeUnknownError), "")
		}
	}()
	hello := encode(opComputerHello, rest.ComputerAttachHello{ProtocolVersions: domain.SupportedProtocolVersions(), ServerTime: stamp(h.clk.Now())})
	if err := ws.Write(ctx, websocket.MessageText, hello); err != nil {
		_ = ws.CloseNow()
		return
	}
	token, code := h.readIdentify(ctx, ws)
	if code != 0 {
		_ = ws.Close(websocket.StatusCode(code), domain.StreamCode(code).Name())
		return
	}
	out, err := h.uc.AttachComputer(ctx, id, token)
	switch {
	case err != nil:
		h.log.LogAttrs(ctx, slog.LevelError, msgInternal, slog.String(logErrorType, errorInternal))
		_ = ws.Close(websocket.StatusCode(domain.CodeUnknownError), "")
		return
	case out.Code != 0:
		_ = ws.Close(websocket.StatusCode(out.Code), out.Reason)
		return
	}
	sd, s, c := h.relay.attach(sideAgent, id, ids.ID{}, ws)
	if sd == nil {
		_ = ws.Close(websocket.StatusCode(c), domain.StreamCode(c).Name())
		return
	}
	h.relay.run(ctx, sideAgent, s, sd)
}

// readIdentify reads identify within computer.open_timeout and decodes it strictly (SEC-IN-2,
// exactly op and d, d against ComputerAttachIdentify without unknown members, a supported
// protocol version). It returns the token, or the close code: 4004 after the deadline, 4002 for a
// frame that is no valid identify.
func (h *AttachHandler) readIdentify(ctx context.Context, ws *websocket.Conn) (string, int) {
	rctx, cancel := context.WithTimeout(ctx, h.cfg.OpenTimeout)
	defer cancel()
	typ, rd, err := ws.Reader(rctx)
	if err != nil {
		return "", int(domain.StreamAuthFailed)
	}
	b, err := io.ReadAll(io.LimitReader(rd, int64(h.cfg.MaxFrameBytes)+1))
	if err != nil || typ != websocket.MessageText || len(b) > h.cfg.MaxFrameBytes || httpserver.CheckJSON(b, h.cfg.MaxJSONDepth) != nil {
		return "", int(domain.StreamDecodeError)
	}
	var env envelope
	if err := strict(b, &env); err != nil || env.Op == nil || *env.Op != opComputerIdentify || len(env.D) == 0 ||
		h.schemas.Validate("ComputerAttachIdentify", env.D) != nil {
		return "", int(domain.StreamDecodeError)
	}
	var d rest.ComputerAttachIdentify
	dec := json.NewDecoder(bytes.NewReader(env.D))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&d); err != nil || !slices.Contains(domain.SupportedProtocolVersions(), d.ProtocolVersion) || d.Token == "" {
		return "", int(domain.StreamDecodeError)
	}
	return d.Token, 0
}

// ComputerDataHandler is the HTTP handler of GET /api/v1/hosts/computer-sessions/{session_id}/stream
// (asyncapi hostComputerData): the login of the host service like a data connection of a terminal
// session, a computer session of this host that is not closed, then the relay.
type ComputerDataHandler struct {
	h     *Handler
	relay *ComputerRelay
	uc    ComputerUseCases
}

// NewComputerDataHandler returns the handler over the control handler h. Errors: a missing
// collaborator.
func NewComputerDataHandler(h *Handler, relay *ComputerRelay, uc ComputerUseCases) (*ComputerDataHandler, error) {
	if h == nil || relay == nil || uc == nil {
		return nil, errors.New("wsrelay: NewComputerDataHandler needs the control handler, the relay and the use cases")
	}
	return &ComputerDataHandler{h: h, relay: relay, uc: uc}, nil
}

// ServeHTTP implements http.Handler.
func (d *ComputerDataHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	id, err := ids.Parse(r.PathValue(sessionPathValue))
	if err != nil || id.IsZero() {
		httpserver.WriteProblem(w, r, errs.New(errs.CodeNotFound, ""))
		return
	}
	ws, err := websocket.Accept(w, r, &websocket.AcceptOptions{CompressionMode: websocket.CompressionDisabled})
	if err != nil {
		return
	}
	ws.SetReadLimit(-1)
	d.serve(context.WithoutCancel(r.Context()), ws, id)
}

// serve runs one computer data connection: the login, the session check, then the relay.
func (d *ComputerDataHandler) serve(ctx context.Context, ws *websocket.Conn, id ids.ID) {
	k := newLink(ws, d.h.cfg.OutboundQueueMax, d.h.cfg.WriteTimeout)
	c := &conn{h: d.h, k: k}
	go k.write(ctx)
	defer close(k.ended)
	defer c.stopTimer()
	defer func() {
		if recover() != nil {
			d.h.d.Log.LogAttrs(ctx, slog.LevelError, msgPanic, slog.String(logErrorType, errorPanic))
			k.close(domain.CodeUnknownError)
		}
	}()
	l, in, ok := c.identify(ctx, true)
	if !ok {
		return
	}
	if in.SessionID != id {
		k.close(domain.CodeDecodeError)
		return
	}
	open, err := d.uc.ComputerDataSession(ctx, l, id)
	switch {
	case err != nil:
		c.fail(ctx)
		return
	case !open:
		k.close(domain.CloseCode(domain.StreamTerminalClosed))
		return
	}
	k.quiet()
	sd, s, code := d.relay.attach(sideHost, id, l.HostID, ws)
	if sd == nil {
		_ = ws.Close(websocket.StatusCode(code), domain.StreamCode(code).Name())
		return
	}
	d.relay.run(ctx, sideHost, s, sd)
}
