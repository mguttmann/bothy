// SPDX-License-Identifier: Apache-2.0

package computermcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"

	"github.com/coder/websocket"

	"github.com/OWNER/bothy/services/host/internal/frame"
	"github.com/OWNER/bothy/services/host/internal/noise"
)

// attachProtocolVersion is the version of the attach connection C1 speaks (computerAttach,
// x-protocol-versions).
const attachProtocolVersion = 1

// inboundBuffer is how many decoded frames the reader holds before it waits for the caller; a
// flow control size, not a limit of the protocol.
const inboundBuffer = 64

// Close codes of the attach connection (docs/api/asyncapi.yaml computerAttach, 27 5.3).
const (
	closeAuthFailed      = 4004
	closeTokenStale      = 4101
	closeHostUnavailable = 4102
	closeHandshakeFailed = 4104
)

// errStream is a fault of the stream: a frame against 27 6.3, a broken decryption, an image whose
// data does not sum to its bytes. The session ends locally (27 6.1).
var errStream = errors.New("computermcp: stream fault")

// errActionTooLong is an action whose frame would pass frame.MaxLen; nothing was sent.
var errActionTooLong = errors.New("computermcp: the action does not fit one frame")

// endedError is a session that ended; reason is the close reason of 8.3 or a local cause.
type endedError struct{ reason string }

func (e *endedError) Error() string { return "computermcp: session ended: " + e.reason }

// Local end reasons of a session, beside the close reasons the host sends in cu_state.
const (
	endStreamFault    = "stream_fault"
	endConnectionLost = "connection_lost"
)

// result is one answered action: the code, the image in memory and its geometry, the app, the
// cursor.
type result struct {
	code          string
	img           []byte
	format        string
	width, height int64
	scaleID       int64
	app           frame.App
	cursor        *frame.Point
}

// inbound is one decoded frame of the host, or the error that ended the reader.
type inbound struct {
	f   frame.Frame
	err error
}

// session is one attached computer session. Its methods are called by one goroutine at a time
// (the tool calls run in order); the reader goroutine only decrypts and decodes.
type session struct {
	id, hostID, desktop string
	ws                  *websocket.Conn
	tr                  *noise.Transport
	in                  chan inbound
	cancel              context.CancelFunc
	closeOnce           sync.Once

	seq       int64
	state     string // active or paused
	ended     *endedError
	lastScale int64
	platform  string // the platform the host named in message 2
}

// attachParams is what attach needs: the grant, the initiator built from the ephemeral key whose
// hash went into the open request, and the configuration.
type attachParams struct {
	grant     Grant
	hostID    string
	desktop   string
	hostKey   [32]byte
	ephemeral noise.KeyPair
	cfg       Config
}

// attach connects attach_path, sends identify with the token, runs the Noise handshake with the
// token as the payload of message 1 and reads the first cu_state (27 5.1 steps 8 and 9). Every
// step waits at most cfg.Timeout. A refusal of the server or the host is returned as an
// *attachRefusedError with the close code and reason.
func attach(ctx context.Context, p attachParams) (*session, error) {
	sidb, err := noise.ParseID(p.grant.SessionID)
	if err != nil {
		return nil, errors.New("computermcp: session_id is no UUID")
	}
	hostb, err := noise.ParseID(p.hostID)
	if err != nil {
		return nil, errors.New("computermcp: host_id is no UUID")
	}
	init, err := noise.NewInitiator(noise.InitiatorConfig{HostStatic: p.hostKey, SessionID: sidb, HostID: hostb, Ephemeral: p.ephemeral,
		Label: noise.ComputerPrologueLabel})
	if err != nil {
		return nil, err
	}
	step, cancel := context.WithTimeout(ctx, p.cfg.Timeout)
	defer cancel()
	ws, err := dialAttach(step, p)
	if err != nil {
		return nil, err
	}
	tr, platform, err := handshake(step, ws, init, p.grant.Token)
	if err != nil {
		_ = ws.CloseNow()
		return nil, err
	}
	s := &session{id: p.grant.SessionID, hostID: p.hostID, desktop: p.desktop, ws: ws, tr: tr, in: make(chan inbound, inboundBuffer),
		platform: platform}
	rctx, rcancel := context.WithCancel(context.WithoutCancel(ctx))
	s.cancel = rcancel
	go s.read(rctx)
	first, err := s.next(step)
	if err == nil && first.Type != frame.TypeCUState {
		err = errStream
	}
	if err == nil {
		err = s.applyState(first)
	}
	if err != nil {
		s.close()
		return nil, err
	}
	return s, nil
}

// dialAttach opens the attach connection, reads hello and sends identify with the token.
func dialAttach(ctx context.Context, p attachParams) (*websocket.Conn, error) {
	u, err := attachURL(p.cfg.Server, p.grant.AttachPath)
	if err != nil {
		return nil, err
	}
	hc := p.cfg.HTTPClient
	if hc == nil {
		hc = newHTTPClient()
	}
	ws, resp, err := websocket.Dial(ctx, u, &websocket.DialOptions{HTTPClient: hc, CompressionMode: websocket.CompressionDisabled})
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	if err != nil {
		return nil, refusalOf(err)
	}
	ws.SetReadLimit(noise.MaxMessageLen)
	if err := readHello(ctx, ws); err != nil {
		_ = ws.CloseNow()
		return nil, err
	}
	id, _ := json.Marshal(map[string]any{"op": "identify", "d": map[string]any{"protocol_version": attachProtocolVersion, "token": p.grant.Token}})
	if err := ws.Write(ctx, websocket.MessageText, id); err != nil {
		_ = ws.CloseNow()
		return nil, refusalOf(err)
	}
	return ws, nil
}

// newHTTPClient returns the client of the attach connection: no proxy from the environment, the
// system's roots. C1 dials only attach_path on the origin of runner.server_url (D-CU-05-2).
func newHTTPClient() *http.Client {
	return &http.Client{Transport: &http.Transport{}} // nosemgrep: bothy-go-egress-only -- the host module has no egress package; C1 dials only attach_path on the origin of runner.server_url (D-CU-05-2)
}

// handshake sends message 1 with the token and reads message 2 (27 5.1 step 9); it returns the
// transport and the platform the host names.
func handshake(ctx context.Context, ws *websocket.Conn, init *noise.Initiator, token string) (*noise.Transport, string, error) {
	m1, err := init.WriteMessage1([]byte(token))
	if err != nil {
		return nil, "", err
	}
	if err := ws.Write(ctx, websocket.MessageBinary, m1); err != nil {
		return nil, "", refusalOf(err)
	}
	typ, m2, err := ws.Read(ctx)
	if err != nil {
		return nil, "", refusalOf(err)
	}
	if typ != websocket.MessageBinary {
		return nil, "", errStream
	}
	payload, tr, err := init.ReadMessage2(m2)
	if err != nil {
		return nil, "", errStream
	}
	var p2 struct {
		V        int    `json:"v"`
		Platform string `json:"platform"`
	}
	if json.Unmarshal(payload, &p2) != nil || p2.V != 1 {
		return nil, "", errStream
	}
	return tr, p2.Platform, nil
}

// readHello reads the hello of computerAttach and checks that it offers version 1.
func readHello(ctx context.Context, ws *websocket.Conn) error {
	typ, b, err := ws.Read(ctx)
	if err != nil {
		return refusalOf(err)
	}
	var h struct {
		Op string `json:"op"`
		D  struct {
			Versions []int `json:"protocol_versions"`
		} `json:"d"`
	}
	if typ != websocket.MessageText || json.Unmarshal(b, &h) != nil || h.Op != "hello" {
		return errStream
	}
	for _, v := range h.D.Versions {
		if v == attachProtocolVersion {
			return nil
		}
	}
	return errStream
}

// attachRefusedError is a close of the attach connection before the session started.
type attachRefusedError struct {
	code   int
	reason string
}

func (e *attachRefusedError) Error() string {
	return fmt.Sprintf("computermcp: attach closed with %d %s", e.code, e.reason)
}

// toolCode maps the close to a code of 27 7: the details not_enabled, helper_absent and
// helper_untrusted are not_enabled, busy is busy, the reasons of authorize map like a refusal of
// the runner, 4102 is host_offline, everything else stopped (D-CU-05-7).
func (e *attachRefusedError) toolCode() string {
	switch e.reason {
	case "not_enabled", "helper_absent", "helper_untrusted":
		return CodeNotEnabled
	case "busy":
		return CodeBusy
	case "computer_not_owner_run", "computer_tool_class", "invocation_token_invalid":
		return CodeNotOwnerRun
	}
	if e.code == closeHostUnavailable {
		return CodeHostOffline
	}
	return CodeStopped
}

// refusalOf turns a close of the peer into an *attachRefusedError; other errors stay.
func refusalOf(err error) error {
	var ce websocket.CloseError
	if errors.As(err, &ce) {
		return &attachRefusedError{code: int(ce.Code), reason: ce.Reason}
	}
	return err
}

// read decrypts and decodes the messages of the host until the connection ends; the first error
// is the last item.
func (s *session) read(ctx context.Context) {
	defer close(s.in)
	for {
		item := s.readOne(ctx)
		select {
		case s.in <- item:
		case <-ctx.Done():
			return
		}
		if item.err != nil {
			return
		}
	}
}

// readOne reads, decrypts and decodes one message of the host.
func (s *session) readOne(ctx context.Context) inbound {
	typ, msg, err := s.ws.Read(ctx)
	if err != nil {
		return inbound{err: err}
	}
	if typ != websocket.MessageBinary {
		return inbound{err: errStream}
	}
	pt, err := s.tr.Decrypt(msg)
	if err != nil {
		return inbound{err: errStream}
	}
	f, err := frame.Decode(pt, frame.KindComputer, frame.FromHost)
	if err != nil {
		return inbound{err: errStream}
	}
	return inbound{f: f}
}

// next returns the next frame of the host, waiting at most ctx.
func (s *session) next(ctx context.Context) (frame.Frame, error) {
	select {
	case got, ok := <-s.in:
		return s.take(got, ok)
	case <-ctx.Done():
		return frame.Frame{}, ctx.Err()
	}
}

// take turns one item of the reader into a frame. The end of the reader ends the session: a
// stream fault as stream_fault, a close with a reason as that reason, any other end as
// connection_lost.
func (s *session) take(got inbound, ok bool) (frame.Frame, error) {
	switch {
	case !ok:
		return frame.Frame{}, s.end(endConnectionLost)
	case errors.Is(got.err, errStream):
		return frame.Frame{}, s.end(endStreamFault)
	case got.err != nil:
		var ce websocket.CloseError
		if errors.As(got.err, &ce) && ce.Reason != "" {
			return frame.Frame{}, s.end(ce.Reason)
		}
		return frame.Frame{}, s.end(endConnectionLost)
	}
	return got.f, nil
}

// applyState applies a cu_state frame: active or paused updates the state, stopped ends the
// session with its reason.
func (s *session) applyState(f frame.Frame) error {
	st, err := f.StateBody()
	if err != nil {
		return s.end(endStreamFault)
	}
	switch st.State {
	case frame.StateActive, frame.StatePaused:
		s.state = st.State
		return nil
	case frame.StateStopped:
		return s.end(st.Reason)
	}
	return s.end(endStreamFault)
}

// end ends the session locally once and returns the *endedError; later calls return the first.
func (s *session) end(reason string) error {
	if s.ended == nil {
		if reason == "" {
			reason = endConnectionLost
		}
		s.ended = &endedError{reason: reason}
		s.close()
	}
	return s.ended
}

// close stops the reader and closes the connection.
func (s *session) close() {
	s.closeOnce.Do(func() {
		if s.cancel != nil {
			s.cancel()
		}
		_ = s.ws.CloseNow()
	})
}

// drainStates applies the cu_state frames that arrived while no action ran; any other frame is a
// stream fault.
func (s *session) drainStates() error {
	for s.ended == nil {
		select {
		case got, ok := <-s.in:
			f, err := s.take(got, ok)
			if err != nil {
				return err
			}
			if f.Type != frame.TypeCUState {
				return s.end(endStreamFault)
			}
			if err := s.applyState(f); err != nil {
				return err
			}
		default:
			return nil
		}
	}
	return s.ended
}

// do sends one action and returns its result with the image reassembled (27 6.3): cu_result,
// then for an image cu_image_begin, the data frames and cu_image_end, whose data must sum to
// bytes; only cu_state stopped may stand between them.
func (s *session) do(ctx context.Context, action string, args json.RawMessage) (result, error) {
	if err := s.drainStates(); err != nil {
		return result{}, err
	}
	// The frame is built and its length checked before the seq is taken, so an action that does
	// not fit one frame leaves the session and the order of seq untouched (27 6.3).
	f, err := frame.NewAction(frame.Action{Seq: s.seq + 1, Action: action, Args: args})
	if err == nil {
		_, err = frame.Encode(f)
	}
	if err != nil {
		return result{}, errActionTooLong
	}
	s.seq++
	if err := s.send(ctx, f); err != nil {
		return result{}, err
	}
	r, err := s.awaitResult(ctx)
	if err != nil {
		return result{}, err
	}
	out := result{code: r.Code, width: r.Width, height: r.Height, scaleID: r.ScaleID, app: r.App, cursor: r.Cursor}
	if r.Image {
		if out.img, out.format, err = s.image(ctx, r); err != nil {
			return result{}, err
		}
	}
	if out.scaleID > 0 && (out.img != nil || out.cursor != nil) {
		s.lastScale = out.scaleID
	}
	return out, nil
}

// awaitResult reads frames until the cu_result of the last action; cu_state frames before it
// are applied, any other frame is a stream fault.
func (s *session) awaitResult(ctx context.Context) (frame.Result, error) {
	for {
		f, err := s.next(ctx)
		if err != nil {
			return frame.Result{}, err
		}
		if f.Type == frame.TypeCUState {
			if err := s.applyState(f); err != nil {
				return frame.Result{}, err
			}
			continue
		}
		r, err := f.ResultBody()
		if f.Type != frame.TypeCUResult || err != nil || r.Seq != s.seq {
			return frame.Result{}, s.end(endStreamFault)
		}
		return r, nil
	}
}

// image reads cu_image_begin, the data and cu_image_end of the result r.
func (s *session) image(ctx context.Context, r frame.Result) ([]byte, string, error) {
	f, err := s.next(ctx)
	if err != nil {
		return nil, "", err
	}
	if f.Type == frame.TypeCUState {
		return nil, "", s.stateInImage(f)
	}
	b, err := f.ImageBeginBody()
	if f.Type != frame.TypeCUImageBegin || err != nil || b.Seq != r.Seq || b.Width != r.Width || b.Height != r.Height || b.ScaleID != r.ScaleID ||
		b.Bytes < 1 || (b.Format != frame.FormatJPEG && b.Format != frame.FormatPNG) {
		return nil, "", s.end(endStreamFault)
	}
	img, err := s.imageData(ctx, b.Bytes)
	if err != nil {
		return nil, "", err
	}
	return img, b.Format, nil
}

// imageData reads the data frames up to cu_image_end; their sum must be exactly want bytes.
func (s *session) imageData(ctx context.Context, want int64) ([]byte, error) {
	var img []byte
	for {
		f, err := s.next(ctx)
		if err == nil {
			err = s.imageFrame(f, &img, want)
		}
		if errors.Is(err, errImageDone) {
			return img, nil
		}
		if err != nil {
			clear(img)
			return nil, err
		}
	}
}

// errImageDone marks cu_image_end with the right number of bytes.
var errImageDone = errors.New("computermcp: image complete")

// imageFrame applies one frame inside an image to img.
func (s *session) imageFrame(f frame.Frame, img *[]byte, want int64) error {
	switch {
	case f.Type == frame.TypeCUImageData && int64(len(*img)+len(f.Data)) <= want:
		*img = append(*img, f.Data...)
		return nil
	case f.Type == frame.TypeCUImageEnd && int64(len(*img)) == want:
		return errImageDone
	case f.Type == frame.TypeCUState:
		return s.stateInImage(f)
	}
	return s.end(endStreamFault)
}

// stateInImage handles a cu_state inside an image: only stopped may stand there.
func (s *session) stateInImage(f frame.Frame) error {
	if st, err := f.StateBody(); err == nil && st.State == frame.StateStopped {
		return s.end(st.Reason)
	}
	return s.end(endStreamFault)
}

// send encrypts and writes one frame; a failure ends the session.
func (s *session) send(ctx context.Context, f frame.Frame) error {
	b, err := frame.Encode(f)
	if err != nil {
		return err
	}
	msg, err := s.tr.Encrypt(b)
	if err != nil {
		return s.end(endStreamFault)
	}
	if err := s.ws.Write(ctx, websocket.MessageBinary, msg); err != nil {
		return s.end(endConnectionLost)
	}
	return nil
}

// done sends close (27 6.3, the tool done) and waits at most the context for cu_state stopped or
// the end of the connection; then the session is closed.
func (s *session) done(ctx context.Context) {
	if s.ended != nil {
		return
	}
	if s.send(ctx, frame.Close()) == nil {
		for s.ended == nil {
			f, err := s.next(ctx)
			if err != nil {
				break
			}
			if f.Type == frame.TypeCUState {
				_ = s.applyState(f)
			}
		}
	}
	_ = s.end("done")
}
