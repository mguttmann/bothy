// SPDX-License-Identifier: Apache-2.0

package wsrelay

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/coder/websocket"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
	"github.com/OWNER/bothy/services/server/internal/terminal/app"
	"github.com/OWNER/bothy/services/server/internal/terminal/domain"
)

// DataSessions checks the session of a data connection (app.Service.DataSession, M2-44).
type DataSessions interface {
	DataSession(ctx context.Context, l app.Login, id ids.ID) (bool, error)
}

// DataHandler is the HTTP handler of GET /api/v1/hosts/sessions/{session_id}/stream (asyncapi
// channel hostData, Design note 9 of M2-44): the host service opens it after session.open. The
// login is the one function of the control connection (hello, identify with session_id within
// terminal.host_auth_timeout, the proof and authorize host.connect with its close codes); then a
// session that is unknown, of another host or closed gives 4100, and the relay takes the
// connection as the host side of the session or refuses it (1000 without an expectation on this
// node or while a paired host side exists). After identify only binary messages pass. It is safe
// for concurrent use.
type DataHandler struct {
	h        *Handler
	relay    *Relay
	sessions DataSessions
}

// NewDataHandler returns the handler of the data connections over the control handler h, the
// relay and the session check. Errors: a missing collaborator.
func NewDataHandler(h *Handler, relay *Relay, sessions DataSessions) (*DataHandler, error) {
	if h == nil || relay == nil || sessions == nil {
		return nil, errors.New("wsrelay: NewDataHandler needs the control handler, the relay and the session check")
	}
	return &DataHandler{h: h, relay: relay, sessions: sessions}, nil
}

// sessionPathValue is the path parameter of both stream routes.
const sessionPathValue = "session_id"

// ServeHTTP implements http.Handler. The host service sends no Origin; a request with one passes
// only the library's same-host check, like the control connection.
func (d *DataHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	id, err := ids.Parse(r.PathValue(sessionPathValue))
	if err != nil || id.IsZero() {
		httpserver.WriteProblem(w, r, errs.New(errs.CodeNotFound, ""))
		return
	}
	ws, err := websocket.Accept(w, r, &websocket.AcceptOptions{CompressionMode: websocket.CompressionDisabled})
	if err != nil {
		return // Accept has answered the request
	}
	// The Noise limit is enforced by readFrame with 4002, not by the library's 1009.
	ws.SetReadLimit(-1)
	d.serve(context.WithoutCancel(r.Context()), ws, id)
}

// serve runs one data connection: the login, the session check, then the relay until it ends.
func (d *DataHandler) serve(ctx context.Context, ws *websocket.Conn, id ids.ID) {
	k := newLink(ws, d.h.cfg.OutboundQueueMax, d.h.cfg.WriteTimeout)
	c := &conn{h: d.h, k: k}
	go k.write(ctx)
	defer close(k.ended)
	defer c.stopTimer()
	defer func() {
		if p := recover(); p != nil {
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
	open, err := d.sessions.DataSession(ctx, l, id)
	switch {
	case err != nil:
		c.fail(ctx)
		return
	case !open:
		k.close(domain.CloseCode(domain.StreamTerminalClosed))
		return
	}
	// From here on the relay owns the connection: the text writer of the login stops without a
	// close frame (hello was written before identify could arrive).
	k.quiet()
	sd, code := d.relay.attachHost(id, l.HostID, ws)
	if sd == nil {
		_ = ws.Close(websocket.StatusCode(code), domain.StreamCode(code).Name())
		return
	}
	runSide(ctx, d.relay, sd)
}

// runSide runs the reader of sd until the connection ends, with the writer of the relay beside it
// (Design note 7): each binary message goes to the relay unchanged; a text frame or a message
// above the Noise limit is a protocol error; the end is reported with the peer's close code.
func runSide(ctx context.Context, r *Relay, sd *side) {
	go r.write(ctx, sd)
	for {
		msg, err := readFrame(ctx, sd.ws)
		switch {
		case errors.Is(err, errProtocol):
			r.protocolError(sd)
			r.closed(sd, 0, "")
			return
		case err != nil:
			code, reason := 0, ""
			var ce websocket.CloseError
			if errors.As(err, &ce) {
				code, reason = int(ce.Code), ce.Reason
			}
			r.closed(sd, code, reason)
			return
		}
		r.forward(sd, msg)
	}
}
