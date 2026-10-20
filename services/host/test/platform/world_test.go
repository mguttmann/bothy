// SPDX-License-Identifier: Apache-2.0

//go:build platform

package platform

// The world of the session tests of M2-45 (brief M2-45 "Tests", block "Platform", Design note 17):
// the fake server of internal/session/sessiontest plays the server, pairs the installed host
// service through bothy-host pair, signs the session tokens and records report, identify,
// session.started and session.ended; the Go test device opens the data connections. Robin is
// the owner; Alex, Sam and Robin have the session users bothy-s-alex, bothy-s-sam and
// bothy-s-robin.

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/host/internal/frame"
	"github.com/OWNER/bothy/services/host/internal/noise"
	"github.com/OWNER/bothy/services/host/internal/session/sessiontest"
)

// The pairing code the fake server accepts and the persons (sub) behind the session users.
const (
	pairingCode = "EXAMPLE-CODE"
	subAlex     = "0192f4a0-0000-7000-8000-0000000000a1"
	subSam      = "0192f4a0-0000-7000-8000-0000000000a2"
	subRobin    = "0192f4a0-0000-7000-8000-0000000000a3"
)

// subOf maps a session user to the person the fake server issues its tokens for.
var subOf = map[string]string{userA: subAlex, userB: subSam, userC: subRobin}

// Close codes and reasons of 24 5.3 and 10.3 as the suite asserts them.
const (
	codeTokenStale    = 4101
	codeHandshakeFail = 4104
	codeClosed        = 4100
	reasonOpenFailed  = "open_failed"
	reasonPolicy      = "host_policy_violation"
	reasonClient      = "client_closed"
	reasonExited      = "process_exited"
)

// Waits of the suite: bounds of polling loops, never sleeps of that length. They are generous
// because a hosted runner is slow; each loop returns as soon as its condition holds.
const (
	waitLong    = 120 * time.Second
	waitShort   = 30 * time.Second
	openWindow  = 60 * time.Second
	pollEvery   = 100 * time.Millisecond
	tokenTTL    = 2 * time.Minute
	checkPeriod = "3s" // host_service.credential_check_interval of the tests with periodic runs
)

// apiKeyCanary is the content of claude_api_key; only its SHA-256 may reach the start log.
const apiKeyCanary = "EXAMPLE-API-KEY-CANARY-3b1d"

// world is one installed and paired host service with the fake server.
type world struct {
	t   *testing.T
	ctx context.Context
	srv *sessiontest.HostServer
	key [noise.KeyLen]byte
}

// newWorld installs the host service with users and extra configuration lines (each indented by
// two spaces under host_service), pairs it with the fake server, which declares decl, and waits
// until the host service is logged in and reported.
func newWorld(t *testing.T, decl, extra string, users ...string) *world {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	w := &world{t: t, ctx: ctx, srv: sessiontest.NewHostServer(t, sessiontest.HostOptions{Code: pairingCode, Declaration: decl})}
	installed(t, extra, users...)
	ca := filepath.Join(t.TempDir(), "server-ca.pem")
	if err := os.WriteFile(ca, w.srv.CAPEM(), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, code := host("pair", "--server", w.srv.URL, "--code", pairingCode, "--ca-file", ca); code != 0 {
		t.Fatalf("pair: exit %d, %d bytes of output: %s", code, len(out), firstLine(out))
	}
	key, ok := w.srv.NoisePublic()
	if !ok {
		t.Fatal("pair did not reach the fake server")
	}
	w.key = key
	w.waitReady()
	return w
}

// firstLine returns the first line of out (a command error names a code there, never a secret).
func firstLine(out string) string {
	l, _, _ := strings.Cut(strings.TrimSpace(out), "\n")
	return l
}

// waitReady waits until the host service is logged in and sent its report after ready.
func (w *world) waitReady() {
	w.t.Helper()
	ctx, cancel := context.WithTimeout(w.ctx, waitLong)
	defer cancel()
	if err := w.srv.WaitReady(ctx); err != nil {
		w.t.Fatalf("the host service did not log in and report: %v", err)
	}
}

// restart runs bothy-host restart and waits for the next login and report.
func (w *world) restart() {
	w.t.Helper()
	n := w.srv.Logins()
	if out, code := host("restart"); code != 0 {
		w.t.Fatalf("restart: exit %d: %s", code, firstLine(out))
	}
	w.waitLogin(n + 1)
}

// waitLogin waits for the n-th login since the start and its report.
func (w *world) waitLogin(n int) {
	w.t.Helper()
	ctx, cancel := context.WithTimeout(w.ctx, waitLong)
	defer cancel()
	if err := w.srv.WaitLogins(ctx, n); err != nil {
		w.t.Fatalf("no login %d of the host service: %v", n, err)
	}
	w.waitReady()
}

// report waits for a report from index since on that satisfies cond.
func (w *world) report(since int, what string, cond func(sessiontest.Report) bool) sessiontest.Report {
	w.t.Helper()
	ctx, cancel := context.WithTimeout(w.ctx, waitLong)
	defer cancel()
	r, err := w.srv.WaitReport(ctx, since, cond)
	if err != nil {
		w.t.Fatalf("no report with %s: %v", what, err)
	}
	return r
}

// lastReport returns the newest report.
func (w *world) lastReport() sessiontest.Report {
	w.t.Helper()
	rs := w.srv.Reports()
	if len(rs) == 0 {
		w.t.Fatal("no report yet")
	}
	return rs[len(rs)-1]
}

// setDeclaration sends a further ready with decl and waits for the report that follows it.
func (w *world) setDeclaration(decl string) {
	w.t.Helper()
	if err := w.srv.SetDeclaration(w.ctx, decl); err != nil {
		w.t.Fatal(err)
	}
	w.waitReady()
}

// opening describes one session.open with its token.
type opening struct {
	osu, prf string
	// wdi is the working directory index; -1 omits the claim.
	wdi int
	// mut changes the claims before signing (a manipulated server).
	mut func(map[string]any)
}

func shellOf(osu string) opening { return opening{osu: osu, prf: "shell", wdi: -1} }

func claudeOf(osu string, wdi int) opening { return opening{osu: osu, prf: "claude", wdi: wdi} }

// tsess is one session of the test device.
type tsess struct {
	w    *world
	id   string
	o    opening
	dev  *sessiontest.Device
	dc   *sessiontest.Conn
	seen bytes.Buffer
	mark int
	exit *int32
}

// claims returns the claims of a token for o: issued now by the fake server for the person of
// o.osu. own is true: the fake server runs on this machine, so the host service detects a server
// host (24 2.3) and step 13 needs own (only the owner opens there); the fake server bypasses
// that rule of the server, the separation of the persons rests on sub and osu.
func (w *world) claims(id string, o opening, att string, dev *sessiontest.Device) map[string]any {
	c := sessiontest.Claims(w.srv.HostID, id, subOf[o.osu], o.osu, o.prf, dev.Cek(), att, time.Now(), tokenTTL)
	c["own"] = true
	if o.wdi >= 0 {
		c["wdi"] = o.wdi
	}
	if o.mut != nil {
		o.mut(c)
	}
	return c
}

// handshake sends session.open for a new session, opens its data connection and sends message 1.
// It returns the session after message 2, or nil with the close code and reason of a refusal.
func (w *world) handshake(o opening) (*tsess, int, string) {
	w.t.Helper()
	ctx, cancel := context.WithTimeout(w.ctx, waitLong)
	defer cancel()
	s := &tsess{w: w, id: sessiontest.NewID(), o: o}
	if err := w.srv.Open(ctx, s.id, o.prf, openWindow); err != nil {
		w.t.Fatalf("session.open: %v", err)
	}
	s.dc = w.srv.Data(ctx, s.id, 1)
	dev, err := sessiontest.NewDevice(w.key, s.id, w.srv.HostID)
	if err != nil {
		w.t.Fatal(err)
	}
	s.dev = dev
	m1, err := dev.Message1(w.srv.Signer.Sign(w.claims(s.id, o, "open", dev)))
	if err != nil {
		w.t.Fatal(err)
	}
	if err := s.dc.SendBinary(ctx, m1); err != nil {
		w.t.Fatal(err)
	}
	m2, err := s.dc.NextBinary(ctx)
	if err != nil {
		select {
		case <-s.dc.Done():
		case <-ctx.Done():
			w.t.Fatal("no message 2 and no close of the data connection")
		}
		code, reason, _ := s.dc.Closed()
		return &tsess{w: w, id: s.id, o: o}, code, reason
	}
	if err := dev.ReadMessage2(m2); err != nil {
		w.t.Fatalf("message 2: %v", err)
	}
	return s, 0, ""
}

// refused opens o and expects the refusal of the handshake with code and detail, without a PTY:
// no session.started and no session.ended (the host service reports nothing for a refusal).
func (w *world) refused(o opening, code int, detail string) {
	w.t.Helper()
	s, got, reason := w.handshake(o)
	if s.dev != nil {
		w.t.Fatalf("%s for %s passed the handshake, want %d %s", o.prf, o.osu, code, detail)
	}
	if got != code || reason != detail {
		w.t.Errorf("refusal %d %q, want %d %q", got, reason, code, detail)
	}
	time.Sleep(time.Second) // a late report would arrive within this
	if w.srv.StartedCount(s.id) != 0 || w.srv.EndedCount(s.id) != 0 {
		w.t.Errorf("a refused handshake reported session.started or session.ended")
	}
}

// start sends start and waits for session.started or session.ended; it returns the end when the
// host service ended the opening.
func (s *tsess) start() (sessiontest.Ended, bool) {
	s.w.t.Helper()
	s.send(frame.Start(120, 40))
	deadline := time.Now().Add(waitLong)
	for time.Now().Before(deadline) {
		if e, ok := s.w.srv.EndedOf(s.id); ok {
			return e, true
		}
		if s.w.srv.StartedCount(s.id) > 0 {
			return sessiontest.Ended{}, false
		}
		time.Sleep(pollEvery)
	}
	s.w.t.Fatal("neither session.started nor session.ended after start")
	return sessiontest.Ended{}, false
}

// open opens o and expects a running PTY.
func (w *world) open(o opening) *tsess {
	w.t.Helper()
	s, code, reason := w.handshake(o)
	if s.dev == nil {
		w.t.Fatalf("%s for %s refused with %d %s", o.prf, o.osu, code, reason)
	}
	if e, ended := s.start(); ended {
		w.t.Fatalf("%s for %s ended at the opening: %s %s", o.prf, o.osu, e.CloseReason, e.CloseDetail)
	}
	return s
}

// openFails opens o, expects the end of the opening after start without a PTY and returns it; the
// fake server then closes the data connection with 4100 like the end path of the server.
func (w *world) openFails(o opening) sessiontest.Ended {
	w.t.Helper()
	s, code, reason := w.handshake(o)
	if s.dev == nil {
		w.t.Fatalf("%s for %s refused with %d %s, want an end after start", o.prf, o.osu, code, reason)
	}
	e, ended := s.start()
	if !ended {
		w.t.Fatalf("%s for %s started a PTY", o.prf, o.osu)
	}
	if w.srv.StartedCount(s.id) != 0 {
		w.t.Error("session.started for an opening that ended")
	}
	s.serverEnd()
	return e
}

// send seals and sends f.
func (s *tsess) send(f frame.Frame) {
	s.w.t.Helper()
	m, err := s.dev.Seal(f)
	if err != nil {
		s.w.t.Fatal(err)
	}
	if err := s.dc.SendBinary(s.w.ctx, m); err != nil {
		s.w.t.Fatalf("send on the data connection: %v", err)
	}
}

// typeLine types line and Enter into the PTY.
func (s *tsess) typeLine(line string) { s.send(frame.Data([]byte(line + enter))) }

// digest is the failure text for output: its length and a hash prefix, never the content (24 10.1
// holds for the failure messages of the host tests too).
func digest(b []byte) string {
	sum := sha256.Sum256(b)
	return "len " + strconv.Itoa(len(b)) + " sha256 " + hex.EncodeToString(sum[:4])
}

// expect reads the output of the PTY until re matches what arrived since the last match and
// returns the submatches; the exit frame is kept.
func (s *tsess) expect(re *regexp.Regexp) []string {
	s.w.t.Helper()
	ctx, cancel := context.WithTimeout(s.w.ctx, waitShort)
	defer cancel()
	for {
		if m := re.FindSubmatchIndex(s.seen.Bytes()[s.mark:]); m != nil {
			out := make([]string, len(m)/2)
			for i := range out {
				if m[2*i] >= 0 {
					out[i] = string(s.seen.Bytes()[s.mark+m[2*i] : s.mark+m[2*i+1]])
				}
			}
			s.mark += m[1]
			return out
		}
		if !s.readOne(ctx) {
			s.w.t.Fatalf("output never matched (%s)", digest(s.seen.Bytes()[s.mark:]))
		}
	}
}

// readOne reads one frame of the host into seen; false when the connection ended or ctx ended.
func (s *tsess) readOne(ctx context.Context) bool {
	m, err := s.dc.NextBinary(ctx)
	if err != nil {
		return false
	}
	f, err := s.dev.Open(m)
	if err != nil {
		s.w.t.Fatalf("frame of the host: %v", err)
	}
	switch f.Type {
	case frame.TypeData:
		s.seen.Write(f.Data)
	case frame.TypeExit:
		c := f.ExitCode
		s.exit = &c
	}
	return true
}

// waitExit reads until the exit frame and returns its code.
func (s *tsess) waitExit() int32 {
	s.w.t.Helper()
	ctx, cancel := context.WithTimeout(s.w.ctx, waitShort)
	defer cancel()
	for s.exit == nil {
		if !s.readOne(ctx) {
			s.w.t.Fatal("no exit frame")
		}
	}
	return *s.exit
}

// ended waits for the session.ended of the session.
func (s *tsess) ended() sessiontest.Ended {
	s.w.t.Helper()
	ctx, cancel := context.WithTimeout(s.w.ctx, waitLong)
	defer cancel()
	e, err := s.w.srv.WaitEnded(ctx, s.id)
	if err != nil {
		s.w.t.Fatalf("no session.ended: %v", err)
	}
	return e
}

// serverEnd closes the data connection with 4100, as the server's end path does after
// session.ended (M2-44 Design note 11).
func (s *tsess) serverEnd() {
	if s.dc != nil {
		s.dc.Close(codeClosed, "")
	}
}

// closeByClient sends close and expects session.ended with client_closed.
func (s *tsess) closeByClient() {
	s.w.t.Helper()
	s.send(frame.Close())
	if e := s.ended(); e.CloseReason != reasonClient {
		s.w.t.Errorf("end %s %s, want %s", e.CloseReason, e.CloseDetail, reasonClient)
	}
	s.serverEnd()
}

// expectEnd waits for session.ended with reason and detail, then ends the data connection as the
// server does.
func (s *tsess) expectEnd(reason, detail string) {
	s.w.t.Helper()
	e := s.ended()
	if e.CloseReason != reason || e.CloseDetail != detail {
		s.w.t.Errorf("session of %s ended %s %s, want %s %s", s.o.osu, e.CloseReason, e.CloseDetail, reason, detail)
	}
	s.serverEnd()
}

// identity runs the identity command in the PTY and returns the user it names (Unix: the numeric
// user ID, Windows: the account of whoami).
func (s *tsess) identity() string {
	s.w.t.Helper()
	s.typeLine(identityCmd)
	return s.expect(identityRe)[1]
}

// background starts the background processes of the tree test in the PTY and returns their IDs.
func (s *tsess) background() []int {
	s.w.t.Helper()
	var pids []int
	for i, cmd := range backgroundCmds {
		s.typeLine(cmd)
		n := atoiOr(s.expect(backgroundRes[i])[1])
		if n <= 0 {
			s.w.t.Fatal("no process ID of a background process")
		}
		pids = append(pids, n)
	}
	return pids
}

func atoiOr(v string) int {
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil {
		return -1
	}
	return n
}

// waitGone waits until none of pids exists.
func waitGone(t *testing.T, what string, pids []int) {
	t.Helper()
	deadline := time.Now().Add(waitShort)
	for time.Now().Before(deadline) {
		if !slices.ContainsFunc(pids, pidAlive) {
			return
		}
		time.Sleep(pollEvery)
	}
	t.Errorf("%s: a process of the session tree still runs", what)
}

// Fake programs: the fake claude of M2-42 built once with the start log in a place every session
// user can write, under a folder of a trusted owner (the build of the Makefile logs into the
// checkout, which a session user cannot reach on a hosted runner).
var (
	fakeOnce sync.Once
	fakeErr  error
)

// fakeDir is the folder of the fake programs and the start log.
func fakeDir() string { return programDir("fake") }

// fakeClaude returns the path of the fake claude, built on first use.
func fakeClaude(t *testing.T) string {
	t.Helper()
	fakeOnce.Do(func() { fakeErr = buildFake() })
	if fakeErr != nil {
		t.Fatalf("build the fake claude: %v", fakeErr)
	}
	return filepath.Join(fakeDir(), exeName("claude"))
}

// startLogPath is the start log of the fake programs (claude-starts.log, M2-42).
func startLogPath() string { return filepath.Join(fakeDir(), "claude-starts.log") }

func buildFake() error {
	if err := os.MkdirAll(fakeDir(), 0o755); err != nil {
		return err
	}
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		return err
	}
	cmd := exec.Command("go", "build", "-ldflags", "-X 'main.startLog="+startLogPath()+"'", //nolint:gosec // the build of the suite
		"-o", filepath.Join(fakeDir(), exeName("claude")), "./testdata/fakeclaude")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		return errors.New(firstLine(string(out)))
	}
	return openForAll(startLogPath())
}

// resetStartLog empties the start log for one test.
func resetStartLog(t *testing.T) {
	t.Helper()
	fakeClaude(t)
	if err := os.Truncate(startLogPath(), 0); err != nil {
		t.Fatal(err)
	}
}

// startLog returns the lines of the start log.
func startLog(t *testing.T) []string {
	t.Helper()
	b, err := os.ReadFile(startLogPath())
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimSpace(string(b)), "\n")
}

// interactiveStarts returns the entries the fake claude wrote in its interactive mode.
func interactiveStarts(t *testing.T) []map[string]string {
	t.Helper()
	var out []map[string]string
	for _, l := range startLog(t) {
		var m map[string]string
		if strings.HasPrefix(l, "{") && json.Unmarshal([]byte(l), &m) == nil && m["mode"] == "interactive" {
			out = append(out, m)
		}
	}
	return out
}

// waitInteractive waits for n interactive entries in the start log.
func waitInteractive(t *testing.T, n int) []map[string]string {
	t.Helper()
	deadline := time.Now().Add(waitShort)
	for time.Now().Before(deadline) {
		if e := interactiveStarts(t); len(e) >= n {
			return e
		}
		time.Sleep(pollEvery)
	}
	t.Fatalf("the fake claude logged fewer than %d interactive starts", n)
	return nil
}

// ran reports whether the program at path appended its name to the start log.
func ran(t *testing.T, path string) bool {
	t.Helper()
	return slices.ContainsFunc(startLog(t), func(l string) bool { return samePath(strings.TrimSpace(l), path) })
}

func samePath(a, b string) bool {
	if runtime.GOOS == "windows" {
		return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
	}
	return filepath.Clean(a) == filepath.Clean(b)
}

// canary copies the fake claude to dir/name: a program whose run the start log shows.
func canary(t *testing.T, dir, name string) string {
	t.Helper()
	p := filepath.Join(dir, exeName(name))
	copyFile(t, fakeClaude(t), p, 0o755)
	return p
}

// writeAPIKey places claude_api_key in the state directory as the operator does (16 4.4 point 5).
func writeAPIKey(t *testing.T) {
	t.Helper()
	p := filepath.Join(stateDir(), "claude_api_key")
	if err := os.WriteFile(p, []byte(apiKeyCanary), 0o600); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" {
		chownTo(t, p, "bothy-host")
	}
}

// apiKeyHash is the SHA-256 the fake claude logs for the key canary.
func apiKeyHash() string {
	sum := sha256.Sum256([]byte(apiKeyCanary))
	return hex.EncodeToString(sum[:])
}

// workdirsYAML returns host_service.claude_workdirs for the lists of each session user.
func workdirsYAML(lists map[string][][2]string) string {
	var b strings.Builder
	b.WriteString("  claude_workdirs:\n")
	keys := make([]string, 0, len(lists))
	for k := range lists {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for _, u := range keys {
		items := make([]string, 0, len(lists[u]))
		for _, d := range lists[u] {
			items = append(items, "{display_name: "+d[0]+", path: '"+d[1]+"'}")
		}
		b.WriteString("    " + u + ": [" + strings.Join(items, ", ") + "]\n")
	}
	return b.String()
}

// claudeYAML is the configuration of the claude profile with the fake claude as
// host_service.claude_bin.
func claudeYAML(t *testing.T) string {
	return "  profiles: [shell, claude]\n  claude_bin: '" + fakeClaude(t) + "'\n"
}

// bindingOf returns the person bound to sessionUser in bindings.json.
func bindingOf(t *testing.T, sessionUser string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(stateDir(), "bindings.json"))
	if err != nil {
		t.Fatalf("bindings.json: %v", err)
	}
	var m map[string]string
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("bindings.json: %v", err)
	}
	return m[sessionUser]
}

// noCanaryIn fails t when canary appears in a file under dir or in a body sent to the fake server.
func noCanaryIn(t *testing.T, w *world, dir, canary string) {
	t.Helper()
	_ = filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if b, err := os.ReadFile(p); err == nil && bytes.Contains(b, []byte(canary)) {
			t.Errorf("the canary is in the state directory (file %s)", d.Name())
		}
		return nil
	})
	for _, b := range w.srv.Received() {
		if bytes.Contains(b, []byte(canary)) {
			t.Error("the canary reached the fake server")
		}
	}
}
