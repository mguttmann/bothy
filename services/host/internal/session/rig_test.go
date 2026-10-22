// SPDX-License-Identifier: Apache-2.0

//go:build linux || darwin

package session

import (
	"bufio"
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
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
	"github.com/OWNER/bothy/services/host/internal/helper"
	"github.com/OWNER/bothy/services/host/internal/hostos"
	"github.com/OWNER/bothy/services/host/internal/noise"
	"github.com/OWNER/bothy/services/host/internal/profiles"
	"github.com/OWNER/bothy/services/host/internal/pty"
	"github.com/OWNER/bothy/services/host/internal/session/sessiontest"
	"github.com/OWNER/bothy/services/host/internal/startup"
	"github.com/OWNER/bothy/services/host/internal/state"
)

// helperMarker as the first argument makes the test binary the helper of the integration tests:
// this binary run again as a subprocess of the same user (Design note 17) with the socket, the
// temporary home, the file that counts the PTYs and a delay of every PTY start as the next
// arguments.
const helperMarker = "bothy-session-test-helper"

// envCanary is a value of the helper's environment that reaches the PTY through the allow list
// (LC_*) and must never reach a log, a report or a close reason.
const envCanary = "ENV-CANARY-7f3a"

func TestMain(m *testing.M) {
	if len(os.Args) == 6 && os.Args[1] == helperMarker {
		delay, _ := time.ParseDuration(os.Args[5])
		os.Exit(runHelper(os.Args[2], os.Args[3], os.Args[4], delay))
	}
	os.Exit(m.Run())
}

// countingStarter appends one line "<session id> <pid>" per started PTY to a file, so a test
// counts the PTYs and finds their leaders; delay slows every start (the window of a race).
type countingStarter struct {
	s     helper.Starter
	file  string
	delay time.Duration
}

func (c countingStarter) Start(spec pty.StartSpec) (pty.Process, error) {
	time.Sleep(c.delay)
	p, err := c.s.Start(spec)
	if err == nil && c.file != "" {
		f, ferr := os.OpenFile(c.file, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
		if ferr == nil {
			_, _ = fmt.Fprintln(f, spec.SessionID, p.PID())
			_ = f.Close()
		}
	}
	return p, err
}

// runHelper is the helper subprocess: the production helper.Serve with the session user being the
// current user, /bin/sh as its login shell and a temporary home.
func runHelper(sock, home, starts string, delay time.Duration) int {
	me, err := user.Current()
	if err != nil {
		return 1
	}
	self := hostos.Account{Name: me.Username, ID: me.Uid, Home: home, Shell: "/bin/sh"}
	r := hostos.ExecRunner{}
	users, sec, probe := hostos.Platform(r)
	sup, err := pty.New(pty.Options{ProcessGroupOnly: true})
	if err != nil {
		return 1
	}
	defer func() { _ = sup.Close() }()
	l, err := (&net.ListenConfig{}).Listen(context.Background(), "unix", sock)
	if err != nil {
		return 1
	}
	login := helper.LoginEnv(r, self)
	env := doctor.HelperEnv{GOOS: currentGOOS(), Self: self, Users: users, Security: sec, Resolver: hostos.OSResolver{}, Files: hostos.OSLister{},
		Probe: probe, Runner: r, Clock: clock.Real(), LoginEnv: login, MkdirPrivate: helper.MkdirPrivate}
	penv := profiles.Env{GOOS: env.GOOS, Self: self, Environ: []string{"PATH=/usr/bin:/bin", "LANG=C", "LC_CANARY=" + envCanary}, Resolver: hostos.OSResolver{}, Security: sec,
		Shells: []string{"/bin/sh"}, LoginEnv: login}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// The helper ends when the test process (its parent) ends: its standard input closes.
	go func() {
		_, _ = bufio.NewReader(os.Stdin).ReadString('\n')
		cancel()
	}()
	err = helper.Serve(ctx, helper.Options{Listener: l, ServiceUID: me.Uid, Env: env,
		Session: &helper.SessionEnv{Check: env, Profiles: penv, PTY: countingStarter{s: sup, file: starts, delay: delay}}})
	if err != nil {
		return 1
	}
	return 0
}

// recorder is the Notifier of the tests: it records session.started and session.ended.
type recorder struct {
	mu      sync.Mutex
	started []string
	ended   []control.SessionEnded
	down    atomic.Bool
}

func newRecorder() *recorder { return &recorder{} }

func (r *recorder) Started(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.started = append(r.started, id)
	return nil
}

func (r *recorder) Ended(_ context.Context, e control.SessionEnded) error {
	if r.down.Load() {
		return errors.New("control connection down")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ended = append(r.ended, e)
	return nil
}

// wait polls cond (which takes the lock itself) until it holds or the timeout passes.
func (r *recorder) wait(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timeout waiting for %s (ended %d)", what, r.endedCount())
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func (r *recorder) endedOf(id string) (control.SessionEnded, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, e := range r.ended {
		if e.SessionID == id {
			return e, true
		}
	}
	return control.SessionEnded{}, false
}

func (r *recorder) waitEnded(t *testing.T, id string) control.SessionEnded {
	t.Helper()
	var out control.SessionEnded
	r.wait(t, "session.ended", func() bool {
		e, ok := r.endedOf(id)
		out = e
		return ok
	})
	return out
}

func (r *recorder) startedCount(id string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for _, s := range r.started {
		if s == id {
			n++
		}
	}
	return n
}

func (r *recorder) endedCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.ended)
}

// fakeRunner is the doctor runner of the gate: it returns res and counts the runs.
type fakeRunner struct {
	mu        sync.Mutex
	res       *doctor.Result
	light     int
	lightUser map[string]int
}

func (f *fakeRunner) RunLight(context.Context) *doctor.Result {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.light++
	return f.res
}

func (f *fakeRunner) RunLightUser(_ context.Context, u string) *doctor.Result {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lightUser[u]++
	return f.res
}

func (f *fakeRunner) Last() *doctor.Result {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.res
}

func (f *fakeRunner) set(res *doctor.Result) {
	f.mu.Lock()
	f.res = res
	f.mu.Unlock()
}

// fakeConfirm answers the local confirmation.
type fakeConfirm struct {
	mu    sync.Mutex
	allow bool
	delay time.Duration
	asked []string
}

func (f *fakeConfirm) Ask(ctx context.Context, kind, profile, osu string, timeout time.Duration) bool {
	f.mu.Lock()
	f.asked = append(f.asked, kind)
	allow, delay := f.allow, f.delay
	f.mu.Unlock()
	if delay > timeout {
		return false
	}
	select {
	case <-time.After(delay):
	case <-ctx.Done():
		return false
	}
	return allow
}

// syncBuffer collects the log lines.
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

// rig is the in-process service with the fake server and the helper subprocess.
type rig struct {
	t       *testing.T
	ctx     context.Context
	srv     *sessiontest.Server
	signer  *sessiontest.Signer
	values  sessiontest.Values
	key     noise.KeyPair
	auth    ed25519.PrivateKey
	cfg     *config.Config
	store   *memStore
	runner  *fakeRunner
	gate    *Gate
	mgr     *Manager
	note    *recorder
	logs    *syncBuffer
	osu     string
	sub     string
	starts  string
	home    string
	confirm *fakeConfirm
	peerBad atomic.Bool
	clk     clock.Clock
	sock    string
	// startDelay slows every PTY start of the helper.
	startDelay time.Duration
	helper     *exec.Cmd
	// rest keeps, per data connection, the output that followed the last match of readUntil, so
	// output that arrives in one frame with the match is not lost to the next readUntil.
	rest map[*sessiontest.Conn]string
}

type rigOption func(*rig)

func withConfirm(f *fakeConfirm) rigOption { return func(r *rig) { r.confirm = f } }
func withStartDelay(d time.Duration) rigOption {
	return func(r *rig) { r.startDelay = d }
}
func withClock(c clock.Clock) rigOption { return func(r *rig) { r.clk = c } }
func withValues(v sessiontest.Values) rigOption {
	return func(r *rig) { r.values = v }
}

// shellResult is a passing doctor result that offers shell and a command profile.
func shellResult(osu string) *doctor.Result {
	res := &doctor.Result{Declaration: state.DeclOrganization, Workdirs: map[string][]doctor.Workdir{osu: {}},
		Profiles: []doctor.Profile{{Name: "shell", DisplayName: "Shell", Kind: config.KindShell}, {Name: "echo", DisplayName: "Echo", Kind: config.KindCommand}}}
	for _, c := range doctor.Checks {
		res.Checks = append(res.Checks, doctor.CheckResult{Check: c})
	}
	return res
}

func newRig(t *testing.T, opts ...rigOption) *rig {
	t.Helper()
	me, err := user.Current()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	t.Cleanup(cancel)
	// The home of the helper lies under /tmp, not under the temporary folder of the test user: the
	// checks of HD11 walk the chain files (.claude, .git) of every folder above a place, and a
	// folder above a temporary folder inside a real home could hold a large real .claude.
	dir, err := os.MkdirTemp("/tmp", "bst")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	home := filepath.Join(dir, "home")
	if err := os.Mkdir(home, 0o700); err != nil {
		t.Fatal(err)
	}
	r := &rig{t: t, ctx: ctx, signer: sessiontest.NewSigner("terminal-key-1"), values: sessiontest.DefaultValues, osu: me.Username, sub: alexSub,
		starts: filepath.Join(dir, "starts"), home: home, sock: filepath.Join(dir, "h.sock"), clk: clock.Real(), note: newRecorder(), logs: &syncBuffer{},
		store: &memStore{m: map[string]string{}}}
	for _, o := range opts {
		o(r)
	}
	r.srv = sessiontest.NewServer(t, func() map[string]any { return sessiontest.Hello([]map[string]any{r.signer.JWK()}, r.values) })
	r.key, _ = noise.GenerateKeyPair(nil)
	_, r.auth, _ = ed25519.GenerateKey(rand.Reader)
	cfg := config.Defaults(config.Platform{GOOS: currentGOOS()})
	cfg.SessionUsers = []string{r.osu}
	cfg.Profiles = append(cfg.Profiles, config.Profile{Name: "echo", DisplayName: "Echo", Kind: config.KindCommand, Program: "/bin/echo", Args: []string{"command-ran"}})
	cfg.ScrollbackBytes = 4096
	cfg.RequireLocalConfirm = r.confirm != nil
	r.cfg = &cfg
	r.runner = &fakeRunner{res: shellResult(r.osu), lightUser: map[string]int{}}
	r.gate = NewGate(r.runner, r.clk, cfg.CredentialCheckInterval, nil, nil)
	r.startHelper()
	pid := r.helper.Process.Pid
	ch := startup.ChannelHelpers{GOOS: currentGOOS(), Users: platformUsers(), Services: pidService(func() int {
		if r.peerBad.Load() {
			return pid + 1
		}
		return pid
	}), Endpoint: func(string, string) string { return r.sock }}
	deps := Deps{HostID: hostID, NoiseKey: r.key, AuthKey: r.auth, HostVersion: "1.2.3",
		HTTP: func() *http.Client {
			return &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: r.srv.Pool, MinVersion: tls.VersionTLS12}}}
		},
		DataURL: func(p string) (string, error) { return "wss" + strings.TrimPrefix(r.srv.URL, "https") + p, nil },
		Config:  r.cfg, Bindings: r.store, APIKey: func() ([]byte, error) { return []byte("EXAMPLE-API-KEY-CANARY"), nil },
		Gate: r.gate, Result: r.runner.Last, Offered: profiles.Offered, ServerHost: func() bool { return false },
		Dial: ch.Open, Clock: r.clk, Log: slog.New(slog.NewJSONHandler(r.logs, nil))}
	if r.confirm != nil {
		deps.Confirm = r.confirm.Ask
	}
	r.mgr = New(deps)
	t.Cleanup(r.mgr.Shutdown)
	return r
}

// pidService is the service manager of the tests: it reports the PID of the helper subprocess as
// the main PID of the helper service, so the peer check runs unchanged.
type pidService func() int

func (p pidService) MainPID(context.Context, string) (int, error) { return p(), nil }

func platformUsers() hostos.Users {
	users, _, _ := hostos.Platform(hostos.ExecRunner{})
	return users
}

func (r *rig) startHelper() {
	r.t.Helper()
	exe, err := os.Executable()
	if err != nil {
		r.t.Fatal(err)
	}
	cmd := exec.CommandContext(context.WithoutCancel(r.ctx), exe, helperMarker, r.sock, r.home, r.starts, r.startDelay.String()) //nolint:gosec // the test binary itself
	cmd.Env = []string{"PATH=/usr/bin:/bin"}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		r.t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		r.t.Fatal(err)
	}
	r.helper = cmd
	r.t.Cleanup(func() {
		_ = stdin.Close()
		done := make(chan struct{})
		go func() { _ = cmd.Wait(); close(done) }()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			_ = cmd.Process.Kill()
			<-done
		}
	})
	for i := 0; i < 200; i++ {
		if _, err := os.Stat(r.sock); err == nil {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	r.t.Fatal("helper socket missing")
}

// startCount returns the number of PTYs the helper started.
func (r *rig) checkRun(req doctor.Request) {
	r.t.Helper()
	pid := r.helper.Process.Pid
	ch := startup.ChannelHelpers{GOOS: currentGOOS(), Users: platformUsers(), Services: pidService(func() int { return pid }),
		Endpoint: func(string, string) string { return r.sock }}
	if _, err := ch.Check(r.ctx, r.osu, req); err != nil {
		r.t.Fatalf("check.run: %v", err)
	}
}

func (r *rig) startCount() int { return len(r.startedPIDs()) }

// startedPIDs returns the leader PIDs of the PTYs the helper started.
func (r *rig) startedPIDs() []int {
	b, _ := os.ReadFile(r.starts)
	var out []int
	for _, l := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		if f := strings.Fields(l); len(f) == 2 {
			out = append(out, atoi(f[1]))
		}
	}
	return out
}

// sessionOpen sends session.open for id.
func (r *rig) sessionOpen(id, prf string, window time.Duration) {
	r.mgr.Open(r.ctx, control.SessionOpen{SessionID: id, DataPath: control.DataPathPrefix + id + "/stream", Profile: prf,
		ExpiresAt: r.clk.Now().Add(window).UTC().Format(time.RFC3339Nano)}, r.note)
}

// claims returns valid claims for id and the device dev.
func (r *rig) claims(id, prf, att string, dev *sessiontest.Device) map[string]any {
	return sessiontest.Claims(hostID, id, r.sub, r.osu, prf, dev.Cek(), att, r.clk.Now(), 30*time.Second)
}

// handshake sends message 1 with tok on dc and reads message 2.
func (r *rig) handshake(dc *sessiontest.Conn, dev *sessiontest.Device, tok string) error {
	m1, err := dev.Message1(tok)
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

func (r *rig) device(id string) *sessiontest.Device {
	d, err := sessiontest.NewDevice(r.key.Public, id, hostID)
	if err != nil {
		r.t.Fatal(err)
	}
	return d
}

// open runs a full opening of id with prf: session.open, message 1 and 2, start.
func (r *rig) open(id, prf string) (*sessiontest.Device, *sessiontest.Conn) {
	r.t.Helper()
	r.sessionOpen(id, prf, 20*time.Second)
	dc := r.srv.Data(r.ctx, id, r.srv.DataCount(id)+1)
	dev := r.device(id)
	if err := r.handshake(dc, dev, r.signer.Sign(r.claims(id, prf, AttOpen, dev))); err != nil {
		r.t.Fatalf("handshake: %v", err)
	}
	r.send(dc, dev, frame.Start(120, 40))
	return dev, dc
}

// openWdi runs a full opening of the profile claude with the working directory index wdi.
func (r *rig) openWdi(id, prf string, wdi int) (*sessiontest.Device, *sessiontest.Conn) {
	r.t.Helper()
	r.sessionOpen(id, prf, 20*time.Second)
	dc := r.srv.Data(r.ctx, id, r.srv.DataCount(id)+1)
	dev := r.device(id)
	c := r.claims(id, prf, AttOpen, dev)
	c["wdi"] = wdi
	if err := r.handshake(dc, dev, r.signer.Sign(c)); err != nil {
		r.t.Fatalf("handshake: %v", err)
	}
	r.send(dc, dev, frame.Start(120, 40))
	return dev, dc
}

func (r *rig) send(dc *sessiontest.Conn, dev *sessiontest.Device, f frame.Frame) {
	r.t.Helper()
	m, err := dev.Seal(f)
	if err != nil {
		r.t.Fatal(err)
	}
	if err := dc.SendBinary(r.ctx, m); err != nil {
		r.t.Fatal(err)
	}
}

// readUntil reads frames of the host until the data seen contains want; it returns the frames.
func (r *rig) readUntil(dc *sessiontest.Conn, dev *sessiontest.Device, want string) []frame.Frame {
	r.t.Helper()
	if r.rest == nil {
		r.rest = map[*sessiontest.Conn]string{}
	}
	var seen strings.Builder
	seen.WriteString(r.rest[dc])
	var out []frame.Frame
	for !strings.Contains(seen.String(), want) {
		m, err := dc.NextBinary(r.ctx)
		if err != nil {
			r.t.Fatalf("waiting for output of %d bytes: %v (seen %d bytes)", len(want), err, seen.Len())
		}
		f, err := dev.Open(m)
		if err != nil {
			r.t.Fatalf("frame: %v", err)
		}
		out = append(out, f)
		seen.Write(f.Data)
	}
	// The output after the match stays for the next readUntil on dc: a slow machine delivers the
	// answers of two commands in one frame.
	text := seen.String()
	r.rest[dc] = text[strings.Index(text, want)+len(want):]
	return out
}

// closedWith waits for the end of dc and returns its close code and reason.
func (r *rig) closedWith(dc *sessiontest.Conn) (int, string) {
	r.t.Helper()
	select {
	case <-dc.Done():
	case <-r.ctx.Done():
		r.t.Fatal("data connection still open")
	}
	code, reason, _ := dc.Closed()
	return code, reason
}

// stillOpen reports that dc is open after d.
func stillOpen(dc *sessiontest.Conn, d time.Duration) bool {
	select {
	case <-dc.Done():
		return false
	case <-time.After(d):
		return true
	}
}

// pidAlive reports whether a process with pid exists.
func pidAlive(pid int) bool {
	p, err := os.FindProcess(pid)
	return err == nil && p.Signal(sigZero) == nil
}

func atoi(s string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(s))
	return n
}

// waitFor polls cond for up to a bounded time.
func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	for i := 0; i < 400; i++ {
		if cond() {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for %s", what)
}

// controlClose is session.close of id with the reason of a revocation.
func controlClose(id string) control.SessionClose {
	return control.SessionClose{SessionID: id, CloseReason: "access_revoked"}
}
