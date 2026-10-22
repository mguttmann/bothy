// SPDX-License-Identifier: Apache-2.0

package session

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/host/internal/clock"
	"github.com/OWNER/bothy/services/host/internal/config"
	"github.com/OWNER/bothy/services/host/internal/control"
	"github.com/OWNER/bothy/services/host/internal/doctor"
	"github.com/OWNER/bothy/services/host/internal/frame"
	"github.com/OWNER/bothy/services/host/internal/localchan"
	"github.com/OWNER/bothy/services/host/internal/noise"
	"github.com/OWNER/bothy/services/host/internal/platform/testkit"
	"github.com/OWNER/bothy/services/host/internal/profiles"
	"github.com/OWNER/bothy/services/host/internal/session/sessiontest"
)

// fakeHelper is the helper of a session user in this process: every dial gets an in-memory
// session channel whose far end answers pty.start with answer and records what the service sends.
type fakeHelper struct {
	t       *testing.T
	mu      sync.Mutex
	conn    *localchan.Conn
	dialErr error
	dials   int
	answer  func(req localchan.PTYStart) (op string, body any)

	starts  chan localchan.PTYStart
	stops   chan string
	resizes chan localchan.PTYResize
	frames  chan []byte
}

func newFakeHelper(t *testing.T) *fakeHelper {
	return &fakeHelper{t: t, starts: make(chan localchan.PTYStart, 64), stops: make(chan string, 64),
		resizes: make(chan localchan.PTYResize, 64), frames: make(chan []byte, 64),
		answer: func(req localchan.PTYStart) (string, any) {
			return localchan.OpPTYStarted, localchan.PTYStarted{SessionID: req.SessionID}
		}}
}

func (f *fakeHelper) dial(context.Context, string) (*localchan.Conn, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.dials++
	if f.dialErr != nil {
		return nil, f.dialErr
	}
	a, b := net.Pipe()
	f.t.Cleanup(func() { _ = a.Close(); _ = b.Close() })
	f.conn = localchan.NewConn(b, localchan.Peer{})
	go f.serve(f.conn)
	return localchan.NewConn(a, localchan.Peer{}), nil
}

func (f *fakeHelper) dialCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.dials
}

func (f *fakeHelper) helperEnd() *localchan.Conn {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.conn
}

func (f *fakeHelper) serve(h *localchan.Conn) {
	for {
		p, err := h.ReadPacket()
		if err != nil {
			return
		}
		if p.IsFrame {
			f.frames <- p.Frame
			continue
		}
		switch p.Msg.Op {
		case localchan.OpPTYStart:
			var req localchan.PTYStart
			_ = json.Unmarshal(p.Msg.D, &req)
			f.starts <- req
			f.mu.Lock()
			answer := f.answer
			f.mu.Unlock()
			if op, body := answer(req); op != "" {
				_ = h.Send(op, p.Msg.ID, body)
			}
		case localchan.OpPTYStop:
			var req localchan.PTYStop
			_ = json.Unmarshal(p.Msg.D, &req)
			f.stops <- req.SessionID
		case localchan.OpPTYResize:
			var req localchan.PTYResize
			_ = json.Unmarshal(p.Msg.D, &req)
			f.resizes <- req
		}
	}
}

// output sends the frame fr of session id as PTY output.
func (f *fakeHelper) output(t *testing.T, id string, fr frame.Frame) {
	t.Helper()
	if err := f.sendOutput(id, fr); err != nil {
		t.Fatal(err)
	}
}

// sendOutput sends the frame fr of session id; an exit frame, which the service ignores as
// output, follows, so fr was handled when sendOutput returns (the link reads one packet after the
// other).
func (f *fakeHelper) sendOutput(id string, fr frame.Frame) error {
	b, err := frame.Encode(fr)
	if err != nil {
		return err
	}
	return f.sendRaw(id, b)
}

// sendRaw is sendOutput with the frame bytes b.
func (f *fakeHelper) sendRaw(id string, b []byte) error {
	sid, err := noise.ParseID(id)
	if err != nil {
		return err
	}
	h := f.helperEnd()
	if err := h.SendFrame(sid, b); err != nil {
		return err
	}
	marker, err := frame.Encode(frame.Exit(0))
	if err != nil {
		return err
	}
	return h.SendFrame(sid, marker)
}

// exited reports the end of the process of session id.
func (f *fakeHelper) exited(t *testing.T, id string, code int) {
	t.Helper()
	if err := f.helperEnd().Send(localchan.OpPTYExited, "x", localchan.PTYExited{SessionID: id, ExitCode: code}); err != nil {
		t.Fatal(err)
	}
}

// notes is the Notifier of the in-process rig; down makes every session.ended fail.
type notes struct {
	down    atomic.Bool
	started chan string
	ended   chan control.SessionEnded
	mu      sync.Mutex
	count   int
}

func newNotes() *notes {
	return &notes{started: make(chan string, 64), ended: make(chan control.SessionEnded, 64)}
}

func (n *notes) Started(_ context.Context, id string) error {
	n.started <- id
	return nil
}

func (n *notes) Ended(_ context.Context, e control.SessionEnded) error {
	if n.down.Load() {
		return errNotifierDown
	}
	n.mu.Lock()
	n.count++
	n.mu.Unlock()
	n.ended <- e
	return nil
}

func (n *notes) endedCount() int {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.count
}

var errNotifierDown = errors.New("control connection down")

// recv returns the next value of ch, or fails the test when ctx ends first.
func recv[T any](t *testing.T, ctx context.Context, what string, ch <-chan T) T {
	t.Helper()
	select {
	case v := <-ch:
		return v
	case <-ctx.Done():
		t.Fatalf("timeout waiting for %s", what)
		var zero T
		return zero
	}
}

// until polls cond until it holds; the wait is bounded by testkit.Deadline.
func until(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := testkit.Deadline()
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timeout waiting for %s", what)
		}
		runtime.Gosched()
		time.Sleep(time.Millisecond)
	}
}

// pipeRig is the in-process service with the fake server and a fake helper on an in-memory
// session channel; it runs on every platform.
type pipeRig struct {
	t       *testing.T
	ctx     context.Context
	srv     *sessiontest.Server
	signer  *sessiontest.Signer
	key     noise.KeyPair
	cfg     *config.Config
	runner  *pipeRunner
	gate    *Gate
	mgr     *Manager
	note    *notes
	helper  *fakeHelper
	clk     clock.Clock
	confirm Confirmer
	apiKey  func() ([]byte, error)
	res     *doctor.Result
}

// pipeRunner is the doctor runner of the in-process rig; onUser runs before every Light run of a
// session user and may change the result.
type pipeRunner struct {
	mu     sync.Mutex
	res    *doctor.Result
	onUser func(*pipeRunner)
}

func (p *pipeRunner) RunLight(context.Context) *doctor.Result { return p.Last() }

func (p *pipeRunner) RunLightUser(context.Context, string) *doctor.Result {
	p.mu.Lock()
	f := p.onUser
	p.mu.Unlock()
	if f != nil {
		f(p)
	}
	return p.Last()
}

func (p *pipeRunner) Last() *doctor.Result {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.res
}

func (p *pipeRunner) set(res *doctor.Result) {
	p.mu.Lock()
	p.res = res
	p.mu.Unlock()
}

// pipeResult is a passing result that offers shell, the command profile echo and claude with one
// working directory of alex.
func pipeResult() *doctor.Result {
	res := passing()
	res.ClaudeBin = "/opt/claude/bin/claude"
	res.Workdirs = map[string][]doctor.Workdir{alexOSU: {{DisplayName: "Project", Path: "/srv/project"}}}
	res.Profiles = append(res.Profiles, doctor.Profile{Name: "echo", DisplayName: "Echo", Kind: config.KindCommand},
		doctor.Profile{Name: "claude", DisplayName: "Claude", Kind: config.KindClaude})
	return res
}

type pipeOption func(*pipeRig)

func pipeClock(c clock.Clock) pipeOption       { return func(r *pipeRig) { r.clk = c } }
func pipeConfirm(c Confirmer) pipeOption       { return func(r *pipeRig) { r.confirm = c } }
func pipeDoctor(res *doctor.Result) pipeOption { return func(r *pipeRig) { r.res = res } }

func newPipeRig(t *testing.T, opts ...pipeOption) *pipeRig {
	t.Helper()
	r := &pipeRig{t: t, ctx: testkit.Context(t), signer: sessiontest.NewSigner("terminal-key-1"), note: newNotes(), helper: newFakeHelper(t),
		clk: clock.Real(), res: pipeResult(), apiKey: func() ([]byte, error) { return []byte("EXAMPLE-API-KEY"), nil }}
	for _, o := range opts {
		o(r)
	}
	r.srv = sessiontest.NewServer(t, func() map[string]any {
		return sessiontest.Hello([]map[string]any{r.signer.JWK()}, sessiontest.DefaultValues)
	})
	var err error
	if r.key, err = noise.GenerateKeyPair(nil); err != nil {
		t.Fatal(err)
	}
	_, auth, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Defaults(config.Platform{GOOS: runtime.GOOS})
	cfg.SessionUsers = []string{alexOSU}
	cfg.Profiles = append(cfg.Profiles, config.Profile{Name: "echo", DisplayName: "Echo", Kind: config.KindCommand, Program: "/bin/echo", Args: []string{"hello"}})
	cfg.ScrollbackBytes = 4096
	r.cfg = &cfg
	r.runner = &pipeRunner{res: r.res}
	r.gate = NewGate(r.runner, r.clk, cfg.CredentialCheckInterval, nil, nil)
	r.mgr = New(Deps{HostID: hostID, NoiseKey: r.key, AuthKey: auth, HostVersion: "1.2.3",
		HTTP: func() *http.Client {
			return &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: r.srv.Pool, MinVersion: tls.VersionTLS12}}}
		},
		DataURL: func(p string) (string, error) { return "wss" + strings.TrimPrefix(r.srv.URL, "https") + p, nil },
		Config:  r.cfg, Bindings: &memStore{m: map[string]string{}}, APIKey: func() ([]byte, error) { return r.apiKey() },
		Gate: r.gate, Result: r.runner.Last, Offered: profiles.Offered, ServerHost: func() bool { return false },
		Dial: r.helper.dial, Confirm: r.confirm, Clock: r.clk, Log: slog.New(slog.DiscardHandler)})
	t.Cleanup(r.mgr.Shutdown)
	return r
}

// sessionOpen sends session.open for sessID with an opening window.
func (r *pipeRig) sessionOpen(window time.Duration) {
	r.mgr.Open(r.ctx, control.SessionOpen{SessionID: sessID, DataPath: control.DataPathPrefix + sessID + "/stream", Profile: "shell",
		ExpiresAt: r.clk.Now().Add(window).UTC().Format(time.RFC3339Nano)}, r.note)
}

// device returns a test device for sessID.
func (r *pipeRig) device() *sessiontest.Device {
	r.t.Helper()
	dev, err := sessiontest.NewDevice(r.key.Public, sessID, hostID)
	if err != nil {
		r.t.Fatal(err)
	}
	return dev
}

// handshakeOn runs message 1 and 2 on dc with a token of prf and att.
func (r *pipeRig) handshakeOn(dc *sessiontest.Conn, dev *sessiontest.Device, prf, att string) error {
	id, _ := dc.Identify["session_id"].(string)
	c := sessiontest.Claims(hostID, id, alexSub, alexOSU, prf, dev.Cek(), att, r.clk.Now(), 30*time.Second)
	m1, err := dev.Message1(r.signer.Sign(c))
	if err != nil {
		return err
	}
	if err := dc.SendBinary(r.ctx, m1); err != nil {
		return err
	}
	m2, err := dc.NextBinary(r.ctx)
	if err != nil {
		return err
	}
	return dev.ReadMessage2(m2)
}

// connect sends session.open for sessID and runs message 1 and 2 with a token of prf and att.
func (r *pipeRig) connect(prf, att string) (*sessiontest.Device, *sessiontest.Conn) {
	r.t.Helper()
	n := r.srv.DataCount(sessID) + 1
	r.sessionOpen(20 * time.Second)
	dc := r.srv.Data(r.ctx, sessID, n)
	dev := r.device()
	if err := r.handshakeOn(dc, dev, prf, att); err != nil {
		r.t.Fatalf("handshake: %v", err)
	}
	return dev, dc
}

func (r *pipeRig) send(dc *sessiontest.Conn, dev *sessiontest.Device, f frame.Frame) {
	r.t.Helper()
	m, err := dev.Seal(f)
	if err != nil {
		r.t.Fatal(err)
	}
	if err := dc.SendBinary(r.ctx, m); err != nil {
		r.t.Fatal(err)
	}
}

// open runs a full opening of the shell for sessID up to session.started.
func (r *pipeRig) open() (*sessiontest.Device, *sessiontest.Conn) {
	r.t.Helper()
	dev, dc := r.connect("shell", AttOpen)
	r.send(dc, dev, frame.Start(120, 40))
	recv(r.t, r.ctx, "pty.start", r.helper.starts)
	if got := recv(r.t, r.ctx, "session.started", r.note.started); got != sessID {
		r.t.Fatalf("session.started of %q", got)
	}
	return dev, dc
}

// next returns the next frame of the host on dc.
func (r *pipeRig) next(dc *sessiontest.Conn, dev *sessiontest.Device) frame.Frame {
	r.t.Helper()
	m, err := dc.NextBinary(r.ctx)
	if err != nil {
		r.t.Fatalf("next frame: %v", err)
	}
	f, err := dev.Open(m)
	if err != nil {
		r.t.Fatalf("frame: %v", err)
	}
	return f
}

// closedWith waits for the end of dc and returns the close code of the host service.
func (r *pipeRig) closedWith(dc *sessiontest.Conn) int {
	r.t.Helper()
	select {
	case <-dc.Done():
	case <-r.ctx.Done():
		r.t.Fatal("data connection still open")
	}
	code, _, _ := dc.Closed()
	return code
}

// detachedNow reports whether id runs without a data connection.
func (r *pipeRig) detachedNow(id string) bool {
	s, ok := r.mgr.liveOf(id)
	if !ok {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.dc == nil
}

// detach ends the data connection of the running session sessID from the server side.
func (r *pipeRig) detach(dc *sessiontest.Conn) {
	r.t.Helper()
	dc.Close(CodeNormal, "")
	until(r.t, "detached", func() bool { return r.detachedNow(sessID) })
}
