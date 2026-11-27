// SPDX-License-Identifier: Apache-2.0

package computer

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/host/internal/confirm"
	"github.com/OWNER/bothy/services/host/internal/control"
	"github.com/OWNER/bothy/services/host/internal/frame"
	"github.com/OWNER/bothy/services/host/internal/hostos"
	"github.com/OWNER/bothy/services/host/internal/noise"
	"github.com/OWNER/bothy/services/host/internal/platform/testkit"
	"github.com/OWNER/bothy/services/host/internal/session/sessiontest"
)

// wait bounds every wait of the rig: the patience of the host tests (quality.test.patience).
const wait = testkit.Patience

// pipeListener hands net.Pipe ends to Helper.Serve.
type pipeListener struct {
	ch     chan net.Conn
	closed chan struct{}
	once   sync.Once
}

func newPipeListener() *pipeListener {
	return &pipeListener{ch: make(chan net.Conn), closed: make(chan struct{})}
}

func (l *pipeListener) Accept() (net.Conn, error) {
	select {
	case c := <-l.ch:
		return c, nil
	case <-l.closed:
		return nil, net.ErrClosed
	}
}

func (l *pipeListener) Close() error {
	l.once.Do(func() { close(l.closed) })
	return nil
}

func (l *pipeListener) Addr() net.Addr { return &net.UnixAddr{Name: "pipe", Net: "unix"} }

// fakeHelper is the computer helper of the tests: it answers capture, do and position like the
// helper of 27 9.4 and records every op it got.
type fakeHelper struct {
	t    *testing.T
	c    net.Conn
	wmu  sync.Mutex
	mu   sync.Mutex
	ops  []any
	scr  Screen
	img  []byte
	imgW int64
	imgH int64
	// doCode is the code of every do; empty means ok.
	doCode string
	// capCode and posCode are the codes of every capture and position; empty means ok. noCursor
	// leaves the cursor out of the done of a position.
	capCode, posCode string
	noCursor         bool
	cursor           XY
	gotOp            chan string
}

// connect attaches a new fake helper with hello h to the rig's helper channel.
func (r *rig) connect(h Hello) *fakeHelper {
	a, b := net.Pipe()
	go func() {
		select {
		case r.l.ch <- a:
		case <-time.After(wait):
		}
	}()
	return startFakeHelper(r.t, b, h)
}

// startFakeHelper sends hello h on c and answers the requests of the host service.
func startFakeHelper(t *testing.T, c net.Conn, h Hello) *fakeHelper {
	f := &fakeHelper{t: t, c: c, scr: Screen{X: 0, Y: 0, Width: 1512, Height: 982, Scale: 2}, imgW: 1280, imgH: 831,
		img: bytes.Repeat([]byte{0xab, 0xcd, 0xef}, 30000), cursor: XY{X: 756, Y: 472.5}, gotOp: make(chan string, 256)}
	h.Op = OpHello
	f.send(h)
	go f.loop()
	return f
}

func (f *fakeHelper) send(m any) {
	b, err := EncodeMsg(m, testFrameMax)
	if err != nil {
		f.t.Error(err)
		return
	}
	f.wmu.Lock()
	defer f.wmu.Unlock()
	_, _ = f.c.Write(b)
}

func (f *fakeHelper) sendImage(img []byte) {
	b, _ := EncodeFrame(KindImage, img, testFrameMax)
	f.wmu.Lock()
	defer f.wmu.Unlock()
	_, _ = f.c.Write(b)
}

func (f *fakeHelper) loop() {
	for {
		kind, body, err := ReadFrame(f.c, testFrameMax)
		if err != nil || kind != KindJSON {
			return
		}
		op, msg, err := DecodeMsg(body)
		if err != nil {
			f.t.Errorf("helper got a malformed message: %v", err)
			return
		}
		f.mu.Lock()
		f.ops = append(f.ops, msg)
		scr, img, w, h, code, cur := f.scr, f.img, f.imgW, f.imgH, f.doCode, f.cursor
		capCode, posCode, noCursor := f.capCode, f.posCode, f.noCursor
		f.mu.Unlock()
		app := AppID{ID: "com.example.editor", Name: "Editor", Signer: "EXAMPLE123"}
		switch m := msg.(type) {
		case *Capture:
			if capCode != "" {
				f.send(Done{Op: OpDone, Seq: m.Seq, Code: capCode, App: app, Screen: scr})
				break
			}
			f.send(Done{Op: OpDone, Seq: m.Seq, Code: CodeOK, App: app, Screen: scr, Image: &ImageInfo{Format: m.Format, Width: w, Height: h, Bytes: int64(len(img))}})
			f.sendImage(img)
		case *Do:
			if code == "" {
				code = CodeOK
			}
			f.send(Done{Op: OpDone, Seq: m.Seq, Code: code, App: app, Screen: scr})
		case *Position:
			switch {
			case posCode != "":
				f.send(Done{Op: OpDone, Seq: m.Seq, Code: posCode, App: app, Screen: scr})
			case noCursor:
				f.send(Done{Op: OpDone, Seq: m.Seq, Code: CodeOK, App: app, Screen: scr})
			default:
				f.send(Done{Op: OpDone, Seq: m.Seq, Code: CodeOK, App: app, Screen: scr, Cursor: &cur})
			}
		}
		select {
		case f.gotOp <- op:
		default:
		}
	}
}

// opsOf returns the recorded ops with name op.
func (f *fakeHelper) opsOf(op string) []any {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []any
	for _, m := range f.ops {
		b, _ := json.Marshal(m)
		if strings.Contains(string(b), `"op":"`+op+`"`) {
			out = append(out, m)
		}
	}
	return out
}

// waitOp waits until the helper got op.
func (f *fakeHelper) waitOp(op string) {
	f.t.Helper()
	deadline := testkit.Deadline()
	for time.Now().Before(deadline) {
		if len(f.opsOf(op)) > 0 {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	f.t.Fatalf("the helper never got %s", op)
}

// fakeStream is the data connection: in carries the messages of bothy-computer-mcp, out those of
// the host.
type fakeStream struct {
	in     chan []byte
	out    chan []byte
	done   chan struct{}
	once   sync.Once
	mu     sync.Mutex
	code   int
	reason string
	keys   []map[string]any
}

func newStream(keys []map[string]any) *fakeStream {
	return &fakeStream{in: make(chan []byte, 16), out: make(chan []byte, 1024), done: make(chan struct{}), keys: keys, code: -1}
}

func (s *fakeStream) In() <-chan []byte     { return s.in }
func (s *fakeStream) Done() <-chan struct{} { return s.done }
func (s *fakeStream) Send(ctx context.Context, m []byte) bool {
	select {
	case s.out <- m:
		return true
	case <-s.done:
		return false
	case <-ctx.Done():
		return false
	}
}
func (s *fakeStream) Drain(context.Context) {}
func (s *fakeStream) Close(code int, reason string) {
	s.once.Do(func() {
		s.mu.Lock()
		s.code, s.reason = code, reason
		s.mu.Unlock()
		close(s.done)
	})
}
func (s *fakeStream) CloseCode() int         { return s.code }
func (s *fakeStream) Keys() []map[string]any { return s.keys }
func (s *fakeStream) Leeway() time.Duration  { return 30 * time.Second }
func (s *fakeStream) closed() (int, string, bool) {
	select {
	case <-s.done:
		s.mu.Lock()
		defer s.mu.Unlock()
		return s.code, s.reason, true
	default:
		return 0, "", false
	}
}

// dropByServer ends the connection from the server side (a lost connection).
func (s *fakeStream) dropByServer() { close(s.in) }

// notifier records the reports.
type notifier struct {
	mu      sync.Mutex
	started []control.ComputerStarted
	states  []control.ComputerState
	ended   []control.ComputerEnded
	fail    bool
}

func (n *notifier) ComputerStarted(_ context.Context, s control.ComputerStarted) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.started = append(n.started, s)
	return nil
}

func (n *notifier) ComputerState(_ context.Context, s control.ComputerState) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.states = append(n.states, s)
	return nil
}

func (n *notifier) ComputerEnded(_ context.Context, e control.ComputerEnded) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.fail {
		return errors.New("control connection down")
	}
	n.ended = append(n.ended, e)
	return nil
}

func (n *notifier) endedOf(id string) []control.ComputerEnded {
	n.mu.Lock()
	defer n.mu.Unlock()
	var out []control.ComputerEnded
	for _, e := range n.ended {
		if e.SessionID == id {
			out = append(out, e)
		}
	}
	return out
}

// waitEnded waits for the computer.ended of id.
func (n *notifier) waitEnded(t *testing.T, id string) control.ComputerEnded {
	t.Helper()
	deadline := testkit.Deadline()
	for time.Now().Before(deadline) {
		if e := n.endedOf(id); len(e) > 0 {
			return e[0]
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("no computer.ended for %s", id)
	return control.ComputerEnded{}
}

// rig is the manager with a fake helper channel, fake data connections and a token signer.
type rig struct {
	t       *testing.T
	m       *Manager
	h       *Helper
	l       *pipeListener
	signer  *sessiontest.Signer
	static  noise.KeyPair
	n       *notifier
	peerUID string
	mu      sync.Mutex
	streams map[string]*fakeStream
	logs    *bytes.Buffer
	logMu   sync.Mutex
}

// syncBuf is a goroutine-safe log sink.
type syncBuf struct {
	mu *sync.Mutex
	b  *bytes.Buffer
}

func (s syncBuf) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func newRig(t *testing.T) *rig {
	t.Helper()
	static, err := noise.GenerateKeyPair(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	r := &rig{t: t, l: newPipeListener(), signer: sessiontest.NewSigner("terminal-key-1"), static: static, n: &notifier{},
		peerUID: uidAlex, streams: map[string]*fakeStream{}, logs: &bytes.Buffer{}}
	log := slog.New(slog.NewTextHandler(syncBuf{mu: &r.logMu, b: r.logs}, nil))
	r.h = NewHelper(HelperOptions{Listener: r.l, MaxBytes: testFrameMax, Log: log,
		PeerOf: func(net.Conn) (confirm.Peer, error) { return confirm.Peer{UID: r.peer(), PID: 4242}, nil },
		Rule:   PeerRule{GOOS: hostos.Linux, TestPeerUID: uidAlex, Console: confirm.Rule{ServiceUID: uidService}},
		OnEvent: func(e Event) {
			if r.m != nil {
				r.m.HelperEvent(e)
			}
		}})
	r.m = NewManager(Config{HostID: tHost, NoiseKey: static, HostVersion: "1.0.0", Platform: "macos", Helper: r.h, Log: log,
		Dial: func(_ context.Context, path, sid string) (Stream, error) {
			if path != control.ComputerDataPathPrefix+sid+"/stream" {
				return nil, errors.New("wrong path")
			}
			return r.stream(sid), nil
		}})
	ctx, cancel := context.WithCancel(context.Background())
	go func() { _ = r.h.Serve(ctx) }()
	t.Cleanup(func() {
		r.m.Shutdown()
		cancel()
	})
	return r
}

func (r *rig) peer() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.peerUID
}

func (r *rig) stream(sid string) *fakeStream {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.streams[sid]
	if !ok {
		s = newStream([]map[string]any{r.signer.JWK()})
		r.streams[sid] = s
	}
	return s
}

// hello is the hello of an enabled helper with generation 3.
func hello() Hello {
	return Hello{Enabled: true, Generation: 3, OSAccount: "alex", Permissions: Permissions{Capture: "granted", Input: "granted"},
		HelperVersion: "1.0.0", Signature: "development"}
}

// waitState waits until the helper mirror satisfies cond.
func (r *rig) waitState(cond func(HelperState) bool) {
	r.t.Helper()
	deadline := testkit.Deadline()
	for time.Now().Before(deadline) {
		if cond(r.h.State()) {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	r.t.Fatalf("helper state never reached: %+v", r.h.State())
}

// openOp returns a computer.open of session sid.
func openOp(sid string) control.ComputerOpen {
	return control.ComputerOpen{SessionID: sid, RunID: tRun, AgentID: tAgent, AgentName: "Builder", UserID: tOwner,
		DataPath: control.ComputerDataPathPrefix + sid + "/stream", ExpiresAt: time.Now().Add(20 * time.Second).Format(time.RFC3339Nano),
		Settings: control.ComputerSettings{ImageMaxEdge: 1280, JPEGQuality: 80, SettleMs: 0, AutoScreenshot: true, WaitMaxMs: 60000,
			KeyRepeatMax: 50, ScrollAmountMax: 50, AuditAppsMax: 32, StatusIntervalMs: 2000}}
}

// device is bothy-computer-mcp of the tests: the Noise initiator and the frames.
type device struct {
	t   *testing.T
	s   *fakeStream
	tr  *noise.Transport
	seq int64
}

// claimsFor returns valid attach token claims of session sid for the ephemeral key hash cek.
func claimsFor(sid, cek string) map[string]any {
	now := time.Now()
	return map[string]any{
		"iss": "0192f3a4-5b6c-7d8e-9f01-0000000000e1", "aud": tHost, "sub": tOwner, "sid": sid, "jti": sessiontest.NewID(),
		"iat": now.Unix(), "exp": now.Add(30 * time.Second).Unix(), "run": tRun, "agt": tAgent, "cek": cek, "own": true, "gen": 3,
	}
}

// handshake sends message 1 with the token claims edit makes and reads message 2; ok false when
// the host closed the connection instead.
func (r *rig) handshake(sid string, edit func(map[string]any)) (*device, bool) {
	r.t.Helper()
	s := r.stream(sid)
	eph, err := noise.GenerateKeyPair(rand.Reader)
	if err != nil {
		r.t.Fatal(err)
	}
	sidb, _ := noise.ParseID(sid)
	hostb, _ := noise.ParseID(tHost)
	init, err := noise.NewInitiator(noise.InitiatorConfig{HostStatic: r.static.Public, SessionID: sidb, HostID: hostb, Ephemeral: eph,
		Label: noise.ComputerPrologueLabel})
	if err != nil {
		r.t.Fatal(err)
	}
	cek, _ := noise.EphemeralKeyHash(eph.Public[:])
	claims := claimsFor(sid, cek)
	if edit != nil {
		edit(claims)
	}
	tok := r.signer.SignRaw(map[string]any{"alg": "EdDSA", "kid": r.signer.Kid, "typ": TokenType}, claims)
	m1, err := init.WriteMessage1([]byte(tok))
	if err != nil {
		r.t.Fatal(err)
	}
	s.in <- m1
	select {
	case m2 := <-s.out:
		p, tr, err := init.ReadMessage2(m2)
		if err != nil {
			r.t.Fatal(err)
		}
		var p2 map[string]any
		if json.Unmarshal(p, &p2) != nil || p2["v"] != float64(1) || p2["platform"] != "macos" || p2["host_version"] != "1.0.0" ||
			!slices.Contains(asSlice(p2["features"]), any("quick")) {
			r.t.Errorf("message 2 payload %s", p)
		}
		return &device{t: r.t, s: s, tr: tr}, true
	case <-s.done:
		return nil, false
	case <-time.After(wait):
		r.t.Fatal("no message 2")
		return nil, false
	}
}

// next returns the next frame of the host.
func (d *device) next() frame.Frame {
	d.t.Helper()
	select {
	case m := <-d.s.out:
		pt, err := d.tr.Decrypt(m)
		if err != nil {
			d.t.Fatal(err)
		}
		f, err := frame.Decode(pt, frame.KindComputer, frame.FromHost)
		if err != nil {
			d.t.Fatal(err)
		}
		return f
	case <-time.After(wait):
		d.t.Fatal("no frame of the host")
		return frame.Frame{}
	}
}

// state reads the next frame as cu_state.
func (d *device) state() frame.State {
	d.t.Helper()
	f := d.next()
	s, err := f.StateBody()
	if err != nil {
		d.t.Fatalf("frame 0x%02x is no cu_state", byte(f.Type))
	}
	return s
}

// sendFrame encrypts and sends f.
func (d *device) sendFrame(f frame.Frame) {
	d.t.Helper()
	b, err := frame.Encode(f)
	if err != nil {
		d.t.Fatal(err)
	}
	m, err := d.tr.Encrypt(b)
	if err != nil {
		d.t.Fatal(err)
	}
	d.s.in <- m
}

// action sends one action and returns its result and image.
func (d *device) action(name string, args string) (frame.Result, []byte, frame.ImageBegin) {
	d.t.Helper()
	return d.send(name, args, false)
}

// quickAction sends one action with quick, as a batch item before the last one (27 6.4).
func (d *device) quickAction(name string, args string) (frame.Result, []byte, frame.ImageBegin) {
	d.t.Helper()
	return d.send(name, args, true)
}

// send sends one action and returns its result and image.
func (d *device) send(name string, args string, quick bool) (frame.Result, []byte, frame.ImageBegin) {
	d.t.Helper()
	d.seq++
	f, err := frame.NewAction(frame.Action{Seq: d.seq, Action: name, Args: json.RawMessage(args), Quick: quick})
	if err != nil {
		d.t.Fatal(err)
	}
	d.sendFrame(f)
	rf := d.next()
	res, err := rf.ResultBody()
	if err != nil {
		d.t.Fatalf("frame 0x%02x is no cu_result", byte(rf.Type))
	}
	if !res.Image {
		return res, nil, frame.ImageBegin{}
	}
	bf := d.next()
	begin, err := bf.ImageBeginBody()
	if err != nil {
		d.t.Fatal("no cu_image_begin after cu_result")
	}
	var img []byte
	for {
		f := d.next()
		if f.Type == frame.TypeCUImageEnd {
			break
		}
		if f.Type != frame.TypeCUImageData {
			d.t.Fatalf("frame 0x%02x between image begin and end", byte(f.Type))
		}
		img = append(img, f.Data...)
	}
	if int64(len(img)) != begin.Bytes {
		d.t.Errorf("image of %d bytes, begin said %d", len(img), begin.Bytes)
	}
	return res, img, begin
}

// open runs computer.open and the handshake of a session with a connected helper; it returns the
// device after the first cu_state.
func (r *rig) open(sid string) (*device, frame.State) {
	r.t.Helper()
	r.m.Open(context.Background(), openOp(sid), r.n)
	d, ok := r.handshake(sid, nil)
	if !ok {
		r.t.Fatalf("handshake of %s refused: %v", sid, r.n.endedOf(sid))
	}
	return d, d.state()
}

// logText returns the log so far.
func (r *rig) logText() string {
	r.logMu.Lock()
	defer r.logMu.Unlock()
	return r.logs.String()
}

// pressedAfter reports whether the helper got a do after its last release (a key or button that
// may still be pressed).
func (f *fakeHelper) pressedAfterRelease() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	last := -1
	for i, m := range f.ops {
		if _, ok := m.(*Release); ok {
			last = i
		}
	}
	return slices.ContainsFunc(f.ops[last+1:], func(m any) bool { _, ok := m.(*Do); return ok })
}

var _ io.Closer = (*pipeListener)(nil)

// asSlice returns v as a JSON array, nil when it is none.
func asSlice(v any) []any {
	a, _ := v.([]any)
	return a
}
