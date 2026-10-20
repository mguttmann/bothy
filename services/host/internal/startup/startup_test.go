// SPDX-License-Identifier: Apache-2.0

package startup

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/host/internal/clock/clocktest"
	"github.com/OWNER/bothy/services/host/internal/config"
	"github.com/OWNER/bothy/services/host/internal/doctor"
	"github.com/OWNER/bothy/services/host/internal/hostos"
	"github.com/OWNER/bothy/services/host/internal/hostos/hostostest"
	"github.com/OWNER/bothy/services/host/internal/localchan"
	"github.com/OWNER/bothy/services/host/internal/pairing"
	"github.com/OWNER/bothy/services/host/internal/state"
)

const (
	svc   = "bothy-host"
	alex  = "bothy-s-alex"
	sam   = "bothy-s-sam"
	robin = "bothy-s-robin"
)

type fakeControl struct {
	mu      sync.Mutex
	ready   Ready
	err     error
	logins  int
	reports []Report
	closes  int
}

func (f *fakeControl) Login(context.Context) (Ready, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.logins++
	return f.ready, f.err
}

func (f *fakeControl) Report(_ context.Context, r Report) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reports = append(f.reports, r)
	return nil
}

func (f *fakeControl) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closes++
	return nil
}

func (f *fakeControl) count() (int, int, int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.logins, len(f.reports), f.closes
}

func (f *fakeControl) first() Report {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.reports[0]
}

func (f *fakeControl) last() Report {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.reports[len(f.reports)-1]
}

type world struct {
	t     *testing.T
	w     *hostostest.World
	cfg   *config.Config
	st    *state.Dir
	ctl   *fakeControl
	logs  *bytes.Buffer
	login map[string]bool // users with a subscription login file
}

func newWorld(t *testing.T) *world {
	t.Helper()
	w := hostostest.New("linux")
	w.AddUser(svc, "999", "/var/lib/bothy-host", false)
	for i, u := range []string{alex, sam, robin} {
		id := strconv.Itoa(1001 + i)
		w.AddUser(u, id, "/home/"+u, true)
		w.Dir("/home/"+u, id, 0o700)
		w.Dir("/home/"+u+"/.config/bothy-host/claude", id, 0o700)
	}
	w.Dir("/var/lib/bothy-host", "999", 0o700)
	w.File("/opt/bothy-host/bin/bothy-host", "0", 0o755, "")
	w.File("/etc/bothy-host/config.yaml", "0", 0o644, "")
	w.Dir("/usr/bin", "0", 0o755)
	w.Dir("/bin", "0", 0o755)
	cfg := config.Defaults(config.Platform{GOOS: "linux"})
	cfg.Path = "/etc/bothy-host/config.yaml"
	cfg.SessionUsers = []string{alex, sam}
	st := state.Open(filepath.Join(t.TempDir(), "state"), state.Plain{})
	if err := st.Ensure(); err != nil {
		t.Fatal(err)
	}
	return &world{t: t, w: w, cfg: &cfg, st: st, ctl: &fakeControl{ready: Ready{LLMCredentials: state.DeclOrganization}}, logs: &bytes.Buffer{}, login: map[string]bool{}}
}

func (x *world) pair() {
	if err := x.st.WritePairing(state.Pairing{ServerURL: "https://chat.example.com", HostID: "h"}); err != nil {
		x.t.Fatal(err)
	}
}

// Check implements doctor.Helpers: the helper-side checks in the view of the session user.
func (x *world) Check(ctx context.Context, user string, req doctor.Request) (doctor.HelperResult, error) {
	return doctor.RunHelper(ctx, x.helperEnv(user), req), nil
}

func (x *world) Scan(ctx context.Context, user string, req doctor.Request) (string, error) {
	return doctor.ScanEntries(ctx, x.helperEnv(user), req), nil
}

func (x *world) helperEnv(user string) doctor.HelperEnv {
	a, _ := x.w.Users().Lookup(user)
	v := x.w.As(user)
	return doctor.HelperEnv{GOOS: "linux", Self: a, Users: x.w.Users(), Security: x.w.Security(), Resolver: v, Files: v, Probe: v.Probe(),
		Runner: &hostostest.Runner{}, Clock: clocktest.New(time.Unix(0, 0)),
		LoginEnv: func(context.Context, time.Duration) (bool, string, error) { return x.login[user], "", nil }}
}

func (x *world) deps() Deps {
	return Deps{Config: x.cfg, State: x.st, Control: x.ctl, Log: slog.New(slog.NewTextHandler(x.logs, nil)), AdminUID: strconv.Itoa(os.Getuid()),
		Doctor: doctor.Env{GOOS: "linux", Config: x.cfg, ServiceUser: svc, StateDir: "/var/lib/bothy-host", KeyFile: "/var/lib/bothy-host/keys",
			Users: x.w.Users(), Security: x.w.Security(), Resolver: x.w.As(svc), Files: x.w.As(svc), Probe: x.w.As(svc).Probe(), Helpers: x,
			HelperBinary: "/opt/bothy-host/bin/bothy-host", SystemSearchDirs: []string{"/usr/bin", "/bin"}, PeerCheckProven: true}}
}

func hd(r Report, check string) []doctor.ReportEntry {
	var out []doctor.ReportEntry
	for _, e := range r.Doctor {
		if e.Check == check && (check != doctor.HD6 || e.Code != doctor.CodeAuthStatusUnchecked) {
			out = append(out, e)
		}
	}
	return out
}

func TestRefusedStartReportsThenExits(t *testing.T) {
	x := newWorld(t)
	x.pair()
	x.w.File("/home/"+sam+"/.claude/.credentials.json", "1002", 0o600, "EXAMPLE")
	s, code := Start(context.Background(), x.deps())
	logins, reports, closes := x.ctl.count()
	if s != nil || code != ExitStartRefused || logins != 1 || reports != 1 || closes != 1 {
		t.Fatalf("code %d logins %d reports %d closes %d", code, logins, reports, closes)
	}
	r := x.ctl.last()
	if len(r.Doctor) < 11 {
		t.Errorf("report has %d entries", len(r.Doctor))
	}
	if e := hd(r, doctor.HD6); len(e) != 1 || e[0].SessionUser != sam || e[0].OK || e[0].Code != doctor.CodeSubscriptionLogin {
		t.Errorf("HD6 %+v", e)
	}
	if !strings.Contains(x.logs.String(), "session_user="+sam) || strings.Contains(x.logs.String(), "credentials") {
		t.Errorf("log:\n%s", x.logs)
	}
}

func TestRefusedStartUnpairedLocalOnly(t *testing.T) {
	x := newWorld(t)
	x.cfg.SessionUsers = nil
	_, code := Start(context.Background(), x.deps())
	logins, reports, _ := x.ctl.count()
	if code != ExitStartRefused || logins != 0 || reports != 0 || !strings.Contains(x.logs.String(), "code=session_users_empty") {
		t.Errorf("code %d logins %d reports %d\n%s", code, logins, reports, x.logs)
	}
}

func TestDeclarationFromReady(t *testing.T) {
	// HD6 finding with an unknown declaration (never received): the host starts.
	x := newWorld(t)
	x.login[sam] = true
	x.ctl.err = ErrUnreachable
	x.pair()
	if s, code := Start(context.Background(), x.deps()); s == nil || code != 0 {
		t.Fatalf("unknown declaration refused: %d", code)
	}
	// ready with organization refuses after one report.
	x = newWorld(t)
	x.login[sam] = true
	x.pair()
	if _, code := Start(context.Background(), x.deps()); code != ExitStartRefused {
		t.Errorf("organization: %d", code)
	}
	if _, reports, _ := x.ctl.count(); reports != 1 {
		t.Errorf("reports %d", reports)
	}
	if d, _ := x.st.ReadDeclaration(); d != state.DeclOrganization {
		t.Errorf("stored %q", d)
	}
	// personal passes and is stored; a second ready re-evaluates.
	x = newWorld(t)
	x.login[sam] = true
	x.pair()
	x.ctl.ready = Ready{LLMCredentials: state.DeclPersonal}
	s, code := Start(context.Background(), x.deps())
	if s == nil || code != 0 {
		t.Fatalf("personal refused: %d", code)
	}
	if d, _ := x.st.ReadDeclaration(); d != state.DeclPersonal {
		t.Errorf("stored %q", d)
	}
	res := s.OnReady(context.Background(), Ready{LLMCredentials: state.DeclOrganization})
	if e := hd(x.ctl.last(), doctor.HD6); !res.Check(doctor.HD6).Failed() || len(e) != 1 || e[0].OK || e[0].SessionUser != sam {
		t.Errorf("second ready: %+v", hd(x.ctl.last(), doctor.HD6))
	}
	if h, _ := s.Readiness(sam); h {
		t.Error("host ready with HD6 at organization")
	}
	// stored organization, HD6 finding, ready with personal: no refusal, one report with HD6 ok.
	x = newWorld(t)
	x.login[sam] = true
	x.pair()
	if err := x.st.WriteDeclaration(state.DeclOrganization); err != nil {
		t.Fatal(err)
	}
	x.ctl.ready = Ready{LLMCredentials: state.DeclPersonal}
	s, code = Start(context.Background(), x.deps())
	if s == nil || code != 0 {
		t.Fatalf("refused: %d", code)
	}
	if _, reports, _ := x.ctl.count(); reports < 1 {
		t.Error("no report")
	}
	if e := hd(x.ctl.first(), doctor.HD6); len(e) != 0 {
		t.Errorf("HD6 %+v", e)
	}
	if d, _ := x.st.ReadDeclaration(); d != state.DeclPersonal {
		t.Errorf("stored %q", d)
	}
	// stored organization, HD6 finding, Login fails: the codes are logged and the start refused.
	x = newWorld(t)
	x.login[sam] = true
	x.pair()
	_ = x.st.WriteDeclaration(state.DeclOrganization)
	x.ctl.err = errors.New("tls")
	if _, code := Start(context.Background(), x.deps()); code != ExitStartRefused || !strings.Contains(x.logs.String(), "code=subscription_login_detected") {
		t.Errorf("login failed: %d\n%s", code, x.logs)
	}
	if _, reports, _ := x.ctl.count(); reports != 0 {
		t.Error("report without login")
	}
}

func adminPair(t *testing.T) (net.Listener, string) {
	t.Helper()
	d, err := os.MkdirTemp("", "st")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(d) })
	p := filepath.Join(d, "admin.sock")
	l, err := localchan.ListenAdmin(p)
	if err != nil {
		t.Fatal(err)
	}
	return l, p
}

func askDoctor(t *testing.T, path string) DoctorAnswer {
	t.Helper()
	c, err := localchan.DialAdmin(context.Background(), path, strconv.Itoa(os.Getuid()))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close() }()
	a, err := AskDoctor(context.Background(), c)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func TestDoctorCommandInRunningService(t *testing.T) {
	x := newWorld(t)
	x.pair()
	l, path := adminPair(t)
	d := x.deps()
	d.Admin = l
	s, code := Start(context.Background(), d)
	if s == nil {
		t.Fatalf("refused %d", code)
	}
	done := make(chan int)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { done <- s.Serve(ctx) }()
	// The entry scans of the start read the fake file system in the background; they finish
	// before the test changes it.
	s.Runner().WaitScans()
	// An HD11 finding blocks only its session user; the service stays.
	x.w.Node("/home/" + alex).Sec.Mode = 0o755
	a := askDoctor(t, path)
	if a.Exit != 1 || !strings.Contains(a.Output, "HD11 FAIL shared_location session_user="+alex) {
		t.Errorf("answer %+v", a)
	}
	if h, u := s.Readiness(alex); !h || u {
		t.Error("A not blocked or host blocked")
	}
	if _, u := s.Readiness(sam); !u {
		t.Error("B blocked")
	}
	select {
	case c := <-done:
		t.Fatalf("service ended with %d", c)
	case <-time.After(50 * time.Millisecond):
	}
	// An HD9 finding ends the service with 78 after the report.
	x.w.AddUser("bothy-s-old", "1600", "/home/bothy-s-old", true)
	x.w.Dir("/home/bothy-s-old", "1600", 0o755)
	_ = x.st.WriteBindings(map[string]string{"bothy-s-old": "p"})
	_, before, _ := x.ctl.count()
	a = askDoctor(t, path)
	if a.Exit != 1 {
		t.Errorf("exit %d", a.Exit)
	}
	if c := <-done; c != ExitStartRefused {
		t.Errorf("service ended with %d", c)
	}
	if _, after, _ := x.ctl.count(); after <= before {
		t.Error("no report before the end")
	}
	// The named HD9 entry is not there because bindings are read at start only; the unnamed one is.
	if e := hd(x.ctl.last(), doctor.HD9); len(e) != 1 || e[0].OK {
		t.Errorf("HD9 %+v", e)
	}
}

func TestSessionUserAddReportedAfterRestart(t *testing.T) {
	x := newWorld(t)
	x.pair()
	x.cfg.SessionUsers = []string{alex, sam, robin}
	if s, code := Start(context.Background(), x.deps()); s == nil {
		t.Fatalf("refused %d", code)
	}
	var names []string
	for _, u := range x.ctl.last().SessionUsers {
		names = append(names, u.OSUser)
	}
	if strings.Join(names, ",") != alex+","+sam+","+robin {
		t.Errorf("reported %v", names)
	}
}

func TestNoControlIsUnreachable(t *testing.T) {
	c := NoControl{}
	if _, err := c.Login(context.Background()); !errors.Is(err, ErrUnreachable) {
		t.Error(err)
	}
	if c.Report(context.Background(), Report{}) == nil || c.Close() != nil {
		t.Error("report or close")
	}
	x := newWorld(t)
	x.pair()
	d := x.deps()
	d.Control = NoControl{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if code := Run(ctx, d); code != 0 {
		t.Errorf("run with NoControl: %d", code)
	}
}

func TestConfigReadOnlyAtStart(t *testing.T) {
	x := newWorld(t)
	x.pair()
	l, path := adminPair(t)
	d := x.deps()
	d.Admin = l
	cfgFile := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(cfgFile, []byte("host_service: {session_users: [bothy-s-alex, bothy-s-sam]}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	s, _ := Start(context.Background(), d)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = s.Serve(ctx) }()
	if err := config.AddSessionUser(cfgFile, robin); err != nil {
		t.Fatal(err)
	}
	askDoctor(t, path)
	for _, u := range x.ctl.last().SessionUsers {
		if u.OSUser == robin {
			t.Error("a change of the file took effect before a restart")
		}
	}
}

func TestDoctorOnce(t *testing.T) {
	x := newWorld(t)
	l, path := adminPair(t)
	d := x.deps()
	d.Admin = l
	done := make(chan int)
	go func() { done <- DoctorOnce(context.Background(), d) }()
	var a DoctorAnswer
	for i := 0; ; i++ {
		c, err := localchan.DialAdmin(context.Background(), path, strconv.Itoa(os.Getuid()))
		if err == nil {
			a, err = AskDoctor(context.Background(), c)
			_ = c.Close()
			if err == nil {
				break
			}
		}
		if i > 200 {
			t.Fatal("no answer")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if a.Exit != 0 || len(a.Doctor) != 11 {
		t.Errorf("%+v", a)
	}
	if c := <-done; c != 0 {
		t.Errorf("exit %d", c)
	}
	if logins, _, _ := x.ctl.count(); logins != 0 {
		t.Error("doctor-once logged in")
	}
}

// TestNoNetworkListener: no package of the host module opens a TCP or UDP listener; the admin
// endpoint and every channel are Unix sockets or named pipes.
func TestNoNetworkListener(t *testing.T) {
	root := filepath.Join("..", "..")
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") || strings.Contains(p, "testdata") {
			return err
		}
		b, err := os.ReadFile(p) //nolint:gosec // the file of this test
		if err != nil {
			return err
		}
		for _, bad := range []string{`"tcp`, `"udp`, "ListenTCP", "ListenUDP", "ListenPacket", "ListenIP", "http.ListenAndServe", "httptest"} {
			if strings.Contains(string(b), bad) {
				t.Errorf("%s contains %s", p, bad)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	l, _ := adminPair(t)
	if l.Addr().Network() != "unix" {
		t.Errorf("admin endpoint on %s", l.Addr().Network())
	}
	_ = l.Close()
}

type fakeServices map[string]int

func (f fakeServices) MainPID(_ context.Context, name string) (int, error) {
	if pid, ok := f[name]; ok {
		return pid, nil
	}
	return 0, hostos.ErrNotRunning
}

func TestChannelHelpers(t *testing.T) {
	x := newWorld(t)
	dir, err := os.MkdirTemp("", "ch")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(dir) }()
	me := strconv.Itoa(os.Getuid())
	w := hostostest.New("linux")
	w.AddUser(alex, me, "/home/"+alex, true)
	sock := filepath.Join(dir, "a.sock")
	l, err := (&net.ListenConfig{}).Listen(context.Background(), "unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	env := x.helperEnv(alex)
	go func() {
		_ = localchan.ServeHelper(ctx, l, me, func(c context.Context, m localchan.Message) (string, any) {
			switch m.Op {
			case localchan.OpHello:
				return localchan.OpHello, map[string]any{"version": 1, "session_user": alex}
			case localchan.OpCheckRun:
				return localchan.OpCheckResult, doctor.HelperResult{PlaceCode: "x"}
			}
			return localchan.OpScanResult, map[string]string{"code": doctor.ScanEntries(c, env, doctor.Request{EntryScanTimeout: time.Hour})}
		})
	}()
	h := ChannelHelpers{GOOS: "linux", Users: w.Users(), Services: fakeServices{hostos.HelperServiceName("linux", alex): os.Getpid()},
		Endpoint: func(string, string) string { return sock }}
	res, err := h.Check(context.Background(), alex, doctor.Request{})
	if err != nil || res.PlaceCode != "x" {
		t.Fatalf("%+v %v", res, err)
	}
	if code, err := h.Scan(context.Background(), alex, doctor.Request{}); err != nil || code != "" {
		t.Errorf("scan %q %v", code, err)
	}
	h.Services = fakeServices{hostos.HelperServiceName("linux", alex): os.Getpid() + 1}
	if _, err := h.Check(context.Background(), alex, doctor.Request{}); !errors.Is(err, doctor.ErrPeerMismatch) {
		t.Errorf("wrong PID: %v", err)
	}
	if _, err := h.Scan(context.Background(), alex, doctor.Request{}); !errors.Is(err, doctor.ErrPeerMismatch) {
		t.Errorf("scan wrong PID: %v", err)
	}
	h.Services = fakeServices{}
	if _, err := h.Check(context.Background(), alex, doctor.Request{}); !errors.Is(err, doctor.ErrPeerMismatch) {
		t.Errorf("helper not running: %v", err)
	}
	h.Services = fakeServices{hostos.HelperServiceName("linux", sam): os.Getpid()}
	w.AddUser(sam, me, "/home/"+sam, true)
	if _, err := h.Check(context.Background(), sam, doctor.Request{}); err == nil {
		t.Error("hello of another session user accepted")
	}
	if _, err := (ChannelHelpers{GOOS: "linux", Users: w.Users(), Services: fakeServices{}, Endpoint: func(string, string) string { return filepath.Join(dir, "none") }}).Check(context.Background(), alex, doctor.Request{}); err == nil || errors.Is(err, doctor.ErrPeerMismatch) {
		t.Errorf("unreachable: %v", err)
	}
}

func TestPairThroughAdmin(t *testing.T) {
	x := newWorld(t)
	l, path := adminPair(t)
	d := x.deps()
	d.Admin = l
	var got pairing.Options
	d.Pair = func(_ context.Context, o pairing.Options) (pairing.Result, error) {
		got = o
		if o.Code == "bad" {
			return pairing.Result{}, pairing.ErrFingerprintMismatch
		}
		return pairing.Result{Fingerprint: "F0CC", HostID: "h"}, nil
	}
	s, _ := Start(context.Background(), d)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = s.Serve(ctx) }()
	c, err := localchan.DialAdmin(context.Background(), path, strconv.Itoa(os.Getuid()))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close() }()
	a, err := AskPair(context.Background(), c, PairRequest{ServerURL: "https://chat.example.com", Code: "c"})
	if err != nil || a.Fingerprint != "F0CC" || got.State != x.st || len(got.Lists.SessionUsers) != 2 {
		t.Errorf("%+v %v %+v", a, err, got.Lists)
	}
	a, err = AskPair(context.Background(), c, PairRequest{Code: "bad"})
	if err != nil || a.Error != "pair: fingerprint mismatch" {
		t.Errorf("%+v %v", a, err)
	}
	var re *localchan.RemoteError
	if err := c.Call(context.Background(), "nope", nil, "x", nil); !errors.As(err, &re) {
		t.Errorf("unknown op %v", err)
	}
	if err := c.Call(context.Background(), localchan.OpPairRun, "not an object", localchan.OpPairRes, nil); !errors.As(err, &re) {
		t.Errorf("malformed %v", err)
	}
}
