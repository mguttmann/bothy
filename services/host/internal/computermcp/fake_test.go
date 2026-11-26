// SPDX-License-Identifier: Apache-2.0

package computermcp

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/OWNER/bothy/services/host/internal/frame"
	"github.com/OWNER/bothy/services/host/internal/noise"
	"github.com/OWNER/bothy/services/host/internal/platform/testkit"
)

// Test values (white label: example persons and example.com only).
const (
	tHostA    = "0192f4a0-7b1c-7b22-9d4f-5a6b7c8d9e01"
	tHostB    = "0192f4a0-7b1c-7b22-9d4f-5a6b7c8d9e02"
	tNonce    = "nonce-of-the-run-of-alex"
	tDesktopA = "Desk of Alex"
	tWait     = testkit.Patience
)

// fakeHost is the server attach endpoint and the host service in one: it serves hello and
// identify like computerAttach, then runs the responder side of 27 5.3 with a test key and
// answers every cu_action through script.
type fakeHost struct {
	t      *testing.T
	srv    *httptest.Server
	static noise.KeyPair

	mu       sync.Mutex
	sessions map[string]*fakeSession // by session ID
	attaches int
	actions  []frame.Action // every action received, in order
	frames   [][]byte       // every plaintext frame received
	closed   bool           // a close frame arrived
	// script answers one action; nil answers ok with an image of imgSize bytes.
	script func(a frame.Action) []frame.Frame
	// openState is the first cu_state.
	openState frame.State
	// features are the features message 2 names; nil leaves the member out like an older host.
	features []string
	// refuseM1, when its code is set, closes the connection after message 1 like a host check
	// of 27 5.3 that failed.
	refuseM1 closing
	// closeAfter, when set, closes the connection after the frames of the next action;
	// garbageAfter sends a message that does not decrypt instead.
	closeAfter   closing
	garbageAfter bool
}

// closing is a close code and reason.
type closing struct {
	code   websocket.StatusCode
	reason string
}

// fakeSession is one session the fake runner opened at the fake host.
type fakeSession struct {
	id, host, token, cek string
}

func newFakeHost(t *testing.T) *fakeHost {
	t.Helper()
	kp, err := noise.GenerateKeyPair(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	h := &fakeHost{t: t, static: kp, sessions: map[string]*fakeSession{}, openState: frame.State{State: frame.StateActive, Reason: ""},
		features: []string{"quick"}}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/computer/sessions/{id}/attach", h.serveAttach)
	h.srv = httptest.NewServer(mux)
	t.Cleanup(h.srv.Close)
	return h
}

// URL returns the origin of the fake server.
func (h *fakeHost) URL() *url.URL {
	u, _ := url.Parse(h.srv.URL)
	return u
}

// expect registers a session the fake runner opened.
func (h *fakeHost) expect(s *fakeSession) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.sessions[s.id] = s
}

func (h *fakeHost) received() []frame.Action {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]frame.Action(nil), h.actions...)
}

func (h *fakeHost) attachCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.attaches
}

func (h *fakeHost) gotClose() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.closed
}

func (h *fakeHost) serveAttach(w http.ResponseWriter, r *http.Request) {
	ws, err := websocket.Accept(w, r, &websocket.AcceptOptions{CompressionMode: websocket.CompressionDisabled})
	if err != nil {
		return
	}
	ws.SetReadLimit(noise.MaxMessageLen + 1)
	ctx := r.Context()
	h.mu.Lock()
	h.attaches++
	s := h.sessions[r.PathValue("id")]
	static, openState, refuse, features := h.static, h.openState, h.refuseM1, h.features
	h.mu.Unlock()
	hello, _ := json.Marshal(map[string]any{"op": "hello", "d": map[string]any{"protocol_versions": []int{1}, "server_time": "2026-11-26T09:30:00.000Z"}})
	if ws.Write(ctx, websocket.MessageText, hello) != nil {
		return
	}
	typ, b, err := ws.Read(ctx)
	var id struct {
		Op string `json:"op"`
		D  struct {
			ProtocolVersion int    `json:"protocol_version"`
			Token           string `json:"token"`
		} `json:"d"`
	}
	if err != nil || typ != websocket.MessageText || json.Unmarshal(b, &id) != nil || id.Op != "identify" || s == nil || id.D.Token != s.token || id.D.ProtocolVersion != 1 {
		_ = ws.Close(4004, "token_invalid")
		return
	}
	sid, _ := noise.ParseID(s.id)
	hid, _ := noise.ParseID(s.host)
	resp, _ := noise.NewResponder(noise.ResponderConfig{Static: static, SessionID: sid, HostID: hid, Label: noise.ComputerPrologueLabel})
	_, m1, err := ws.Read(ctx)
	if err != nil {
		return
	}
	payload, eph, err := resp.ReadMessage1(m1)
	cek, _ := noise.EphemeralKeyHash(eph[:])
	if err != nil || string(payload) != s.token || cek != s.cek {
		_ = ws.Close(4104, "token_invalid")
		return
	}
	if refuse.code != 0 {
		_ = ws.Close(refuse.code, refuse.reason)
		return
	}
	p2 := map[string]any{"v": 1, "host_version": "1.0.0", "platform": "macos"}
	if features != nil {
		p2["features"] = features
	}
	p2b, _ := json.Marshal(p2)
	m2, tr, err := resp.WriteMessage2(p2b)
	if err != nil || ws.Write(ctx, websocket.MessageBinary, m2) != nil {
		return
	}
	send := func(f frame.Frame) bool {
		b, err := frame.Encode(f)
		if err != nil {
			h.t.Errorf("fake host encode: %v", err)
			return false
		}
		m, _ := tr.Encrypt(b)
		return ws.Write(ctx, websocket.MessageBinary, m) == nil
	}
	st, _ := frame.NewState(openState)
	if !send(st) {
		return
	}
	for {
		_, msg, err := ws.Read(ctx)
		if err != nil {
			return
		}
		pt, err := tr.Decrypt(msg)
		if err != nil {
			_ = ws.Close(4002, "stream_fault")
			return
		}
		h.mu.Lock()
		h.frames = append(h.frames, bytes.Clone(pt))
		h.mu.Unlock()
		f, err := frame.Decode(pt, frame.KindComputer, frame.FromDevice)
		if err != nil {
			_ = ws.Close(4002, "stream_fault")
			return
		}
		if f.Type == frame.TypeClose {
			h.mu.Lock()
			h.closed = true
			h.mu.Unlock()
			st, _ := frame.NewState(frame.State{State: frame.StateStopped, Reason: "done"})
			send(st)
			_ = ws.Close(websocket.StatusNormalClosure, "")
			return
		}
		a, err := f.ActionBody()
		if err != nil {
			return
		}
		h.mu.Lock()
		h.actions = append(h.actions, a)
		script := h.script
		h.mu.Unlock()
		out := okImage(a, 3000, frame.FormatJPEG)
		if a.Quick && a.Action != "screenshot" && a.Action != "zoom" {
			// The host answers a quick action without an image (27 6.4).
			out = codeOnly(a, "ok")
		}
		if script != nil {
			out = script(a)
		}
		for _, f := range out {
			if !send(f) {
				return
			}
		}
		h.mu.Lock()
		closeAfter, garbage := h.closeAfter, h.garbageAfter
		h.mu.Unlock()
		if garbage {
			_ = ws.Write(ctx, websocket.MessageBinary, testImage(40, 9))
		}
		if closeAfter.code != 0 {
			_ = ws.Close(closeAfter.code, closeAfter.reason)
			return
		}
	}
}

// testImage returns n bytes with a known pattern.
func testImage(n int, seed byte) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = seed + byte(i%251)
	}
	return b
}

// okImage answers a with code ok and an image of n bytes in format.
func okImage(a frame.Action, n int, format string) []frame.Frame {
	return imageFrames(frame.Result{Seq: a.Seq, Code: "ok", Image: true, Width: 1280, Height: 800, ScaleID: 4,
		App: frame.App{ID: "com.example.editor", Name: "Editor"}}, testImage(n, byte(a.Seq)), format) //nolint:gosec // G115: small test sequence numbers
}

// imageFrames returns cu_result, cu_image_begin, the chunks and cu_image_end.
func imageFrames(r frame.Result, img []byte, format string) []frame.Frame {
	rf, _ := frame.NewResult(r)
	bf, _ := frame.NewImageBegin(frame.ImageBegin{Seq: r.Seq, Format: format, Width: r.Width, Height: r.Height, ScaleID: r.ScaleID, Bytes: int64(len(img))})
	out := []frame.Frame{rf, bf}
	for len(img) > 0 {
		n := min(len(img), frame.MaxImageChunk)
		out = append(out, frame.ImageData(img[:n]))
		img = img[n:]
	}
	return append(out, frame.ImageEnd())
}

// codeOnly answers a with code and no image.
func codeOnly(a frame.Action, code string) []frame.Frame {
	rf, _ := frame.NewResult(frame.Result{Seq: a.Seq, Code: code, App: frame.App{ID: "com.example.editor", Name: "Editor"}})
	return []frame.Frame{rf}
}

// fakeRunner serves the request folder of 27 10.2: it accepts requests with the nonce, opens
// sessions at the fake host and writes answers; refuse, when set, answers every request.
type fakeRunner struct {
	t    *testing.T
	dir  string
	host *fakeHost

	mu       sync.Mutex
	requests []map[string]any
	raw      [][]byte
	keys     map[string]noise.KeyPair // host key per host ID; default the fake host's key
	gens     map[string]int64
	refuse   map[string]any // the answer of every request when set
	silent   bool           // never answer
}

func newFakeRunner(ctx context.Context, t *testing.T, host *fakeHost) *fakeRunner {
	t.Helper()
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o700); err != nil { //nolint:gosec // G302: the request folder of 27 10.2 is 0700
		t.Fatal(err)
	}
	r := &fakeRunner{t: t, dir: dir, host: host, keys: map[string]noise.KeyPair{}, gens: map[string]int64{}}
	go r.loop(ctx)
	return r
}

func (r *fakeRunner) set(f func(r *fakeRunner)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	f(r)
}

func (r *fakeRunner) seen() []map[string]any {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]map[string]any(nil), r.requests...)
}

func (r *fakeRunner) loop(ctx context.Context) {
	tick := time.NewTicker(2 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
		entries, _ := os.ReadDir(r.dir)
		for _, e := range entries {
			name := e.Name()
			if !strings.HasSuffix(name, ".request.json") {
				continue
			}
			id := strings.TrimSuffix(name, ".request.json")
			answer := filepath.Join(r.dir, id+".answer.json")
			if _, err := os.Stat(answer); err == nil {
				continue
			}
			b, err := os.ReadFile(filepath.Join(r.dir, name)) //nolint:gosec // G304: a request file of the test folder
			if err != nil {
				continue
			}
			r.mu.Lock()
			silent := r.silent
			r.mu.Unlock()
			if silent {
				continue
			}
			ans := r.answer(b)
			out, _ := json.Marshal(ans)
			tmp := answer + ".tmp"
			if err := os.WriteFile(tmp, out, 0o600); err != nil {
				r.t.Errorf("fake runner write: %v", err)
				continue
			}
			_ = os.Rename(tmp, answer)
		}
	}
}

func (r *fakeRunner) answer(b []byte) map[string]any {
	refused := map[string]any{"v": 1, "ok": false, "status": 0, "code": "request_refused", "reason": nil}
	var req map[string]any
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	if dec.Decode(&req) != nil {
		return refused
	}
	r.mu.Lock()
	r.requests = append(r.requests, req)
	r.raw = append(r.raw, b)
	refuse := r.refuse
	r.mu.Unlock()
	if refuse != nil {
		return refuse
	}
	if fmt.Sprint(req["v"]) != "1" || req["nonce"] != tNonce {
		return refused
	}
	switch req["op"] {
	case "list":
		if len(req) != 3 {
			return refused
		}
		return map[string]any{"v": 1, "ok": true, "result": map[string]any{"items": []any{
			map[string]any{"host_id": tHostA, "name": tDesktopA, "platform": "macos", "online": true, "locked": false, "busy": false},
			map[string]any{"host_id": tHostB, "name": "Laptop of Sam", "platform": "windows", "online": false, "locked": false, "busy": false},
		}}}
	case "open":
		host, _ := req["host_id"].(string)
		cek, _ := req["client_ephemeral_key_hash"].(string)
		_, hasPin := req["pin_changed"].(bool)
		if len(req) != 6 || host == "" || cek == "" || !hasPin {
			return refused
		}
		r.mu.Lock()
		kp, ok := r.keys[host]
		if !ok {
			r.host.mu.Lock()
			kp = r.host.static
			r.host.mu.Unlock()
		}
		gen := r.gens[host]
		if gen == 0 {
			gen = 1
		}
		r.mu.Unlock()
		sid := testUUID(r.t)
		tok := "tok." + base64.RawURLEncoding.EncodeToString(testImage(24, 7))
		r.host.expect(&fakeSession{id: sid, host: host, token: tok, cek: cek})
		return map[string]any{"v": 1, "ok": true, "result": map[string]any{
			"session_id": sid, "token": tok, "attach_path": "/api/v1/computer/sessions/" + sid + "/attach",
			"noise_public_key": base64.RawURLEncoding.EncodeToString(kp.Public[:]), "key_fingerprint": "7K2Q-M9XD-0000",
			"key_generation": gen, "expires_at": "2026-11-26T09:31:30.000Z",
		}}
	}
	return refused
}

func testUUID(t *testing.T) string {
	t.Helper()
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		t.Fatal(err)
	}
	b[6] = b[6]&0x0f | 0x70
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// rig is one MCP server under test with its fakes and an MCP client on pipes.
type rig struct {
	t      *testing.T
	ctx    context.Context
	host   *fakeHost
	runner *fakeRunner
	pins   string
	logBuf *syncBuffer
	in     *io.PipeWriter
	out    *bufio.Reader
	done   chan error
	nextID int
}

// syncBuffer is a bytes.Buffer safe for the logger and the test.
type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

func newRig(t *testing.T) *rig {
	t.Helper()
	ctx := testkit.Context(t)
	host := newFakeHost(t)
	runner := newFakeRunner(ctx, t, host)
	pinDir := t.TempDir()
	r := &rig{t: t, ctx: ctx, host: host, runner: runner, pins: filepath.Join(pinDir, "computer", "pins.json"), logBuf: &syncBuffer{}}
	r.start()
	return r
}

// config returns the configuration of the server under test.
func (r *rig) config() Config {
	return Config{Dir: r.runner.dir, Nonce: tNonce, Timeout: tWait, PinsPath: r.pins, Server: r.host.URL(), Version: "1.0.0",
		Log: slog.New(slog.NewJSONHandler(r.logBuf, nil))}
}

// start runs a new server process on fresh pipes (a new turn).
func (r *rig) start() {
	r.t.Helper()
	srv, err := New(r.config())
	if err != nil {
		r.t.Fatal(err)
	}
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	r.in, r.out, r.done = inW, bufio.NewReader(outR), make(chan error, 1)
	go func() {
		err := srv.Serve(r.ctx, inR, outW)
		_ = outW.Close()
		r.done <- err
	}()
	r.t.Cleanup(func() { _ = inW.Close() })
}

// stop closes stdin and waits for Serve.
func (r *rig) stop() {
	r.t.Helper()
	_ = r.in.Close()
	select {
	case err := <-r.done:
		if err != nil {
			r.t.Fatalf("Serve: %v", err)
		}
	case <-time.After(testkit.Patience):
		r.t.Fatal("Serve did not end after EOF")
	}
}

// rpcResponse is one JSON-RPC answer.
type rpcResponse struct {
	ID     json.RawMessage `json:"id"`
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Code    int             `json:"code"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data"`
	} `json:"error"`
}

// send writes one raw line.
func (r *rig) send(line string) {
	r.t.Helper()
	if _, err := io.WriteString(r.in, line+"\n"); err != nil {
		r.t.Fatal(err)
	}
}

// read reads one answer line.
func (r *rig) read() rpcResponse {
	r.t.Helper()
	type res struct {
		line []byte
		err  error
	}
	ch := make(chan res, 1)
	go func() {
		l, err := r.out.ReadBytes('\n')
		ch <- res{l, err}
	}()
	select {
	case got := <-ch:
		if got.err != nil {
			r.t.Fatalf("read answer: %v", got.err)
		}
		var resp rpcResponse
		if err := json.Unmarshal(got.line, &resp); err != nil {
			r.t.Fatalf("answer is no JSON: %v", err)
		}
		return resp
	case <-time.After(testkit.Patience):
		r.t.Fatal("no answer")
	}
	return rpcResponse{}
}

// rpc sends a request and returns its answer.
func (r *rig) rpc(method string, params any) rpcResponse {
	r.t.Helper()
	r.nextID++
	b, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": r.nextID, "method": method, "params": params})
	r.send(string(b))
	resp := r.read()
	if string(resp.ID) != fmt.Sprint(r.nextID) {
		r.t.Fatalf("answer id %s, want %d", resp.ID, r.nextID)
	}
	return resp
}

// initialize runs the handshake of the legacy era.
func (r *rig) initialize() {
	r.t.Helper()
	resp := r.rpc("initialize", map[string]any{"protocolVersion": "2025-11-25", "capabilities": map[string]any{},
		"clientInfo": map[string]any{"name": "test-client", "version": "1"}})
	if resp.Error != nil {
		r.t.Fatalf("initialize: %+v", resp.Error)
	}
	r.send(`{"jsonrpc":"2.0","method":"notifications/initialized"}`)
}

// toolResult is the result of tools/call.
type toolResult struct {
	Content []struct {
		Type     string `json:"type"`
		Text     string `json:"text"`
		Data     string `json:"data"`
		MimeType string `json:"mimeType"`
	} `json:"content"`
	IsError bool `json:"isError"`
}

// call calls a tool and returns its result.
func (r *rig) call(name string, args any) toolResult {
	r.t.Helper()
	resp := r.rpc("tools/call", map[string]any{"name": name, "arguments": args})
	if resp.Error != nil {
		r.t.Fatalf("tools/call %s: %+v", name, resp.Error)
	}
	var tr toolResult
	if err := json.Unmarshal(resp.Result, &tr); err != nil {
		r.t.Fatal(err)
	}
	return tr
}

// text returns the text of the first block.
func (tr toolResult) text() string {
	if len(tr.Content) == 0 || tr.Content[0].Type != "text" {
		return ""
	}
	return tr.Content[0].Text
}

// image returns the decoded image block, or nil.
func (tr toolResult) image(t *testing.T) ([]byte, string) {
	t.Helper()
	for _, c := range tr.Content {
		if c.Type == "image" {
			b, err := base64.StdEncoding.DecodeString(c.Data)
			if err != nil {
				t.Fatalf("image data: %v", err)
			}
			return b, c.MimeType
		}
	}
	return nil, ""
}

// open opens a session on desktop A after initialize. The first screenshot of open_session
// (seq 1) is dropped from the actions and frames the fake host recorded, so a test counts its
// own actions; their seq starts at 2.
func (r *rig) open() toolResult {
	r.t.Helper()
	res := r.call("open_session", map[string]any{"desktop": tDesktopA})
	if res.IsError {
		r.t.Fatalf("open_session: %s", res.text())
	}
	r.host.mu.Lock()
	r.host.actions, r.host.frames = nil, nil
	r.host.mu.Unlock()
	return res
}
