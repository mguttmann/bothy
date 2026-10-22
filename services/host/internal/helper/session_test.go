// SPDX-License-Identifier: Apache-2.0

package helper

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/host/internal/config"
	"github.com/OWNER/bothy/services/host/internal/doctor"
	"github.com/OWNER/bothy/services/host/internal/frame"
	"github.com/OWNER/bothy/services/host/internal/hostos/hostostest"
	"github.com/OWNER/bothy/services/host/internal/localchan"
	"github.com/OWNER/bothy/services/host/internal/noise"
	"github.com/OWNER/bothy/services/host/internal/platform/testkit"
	"github.com/OWNER/bothy/services/host/internal/profiles"
	"github.com/OWNER/bothy/services/host/internal/pty"
	"github.com/OWNER/bothy/services/host/internal/state"
)

const (
	sessOne  = "0192f4a0-7b1c-7b22-9d4f-00000000c001"
	sessTwo  = "0192f4a0-7b1c-7b22-9d4f-00000000c002"
	selfHome = "/home/bothy-s-alex"
	tool     = "/opt/tool/bin/run"
	workdir  = "/home/bothy-s-alex/project"
	codeKill = 137
)

// fakeProc is a PTY whose output, input, size and exit the test drives.
type fakeProc struct {
	out      chan []byte
	written  chan []byte
	resized  chan [2]uint16
	exited   chan struct{}
	killed   chan struct{}
	exitOnce sync.Once
	killOnce sync.Once
	code     int
}

func newFakeProc() *fakeProc {
	return &fakeProc{out: make(chan []byte, 8), written: make(chan []byte, 8), resized: make(chan [2]uint16, 8),
		exited: make(chan struct{}), killed: make(chan struct{})}
}

func (p *fakeProc) Read(b []byte) (int, error) {
	select {
	case d := <-p.out:
		return copy(b, d), nil
	case <-p.killed:
		return 0, io.EOF
	}
}

func (p *fakeProc) Write(b []byte) (int, error) {
	p.written <- slices.Clone(b)
	return len(b), nil
}

func (p *fakeProc) Resize(cols, rows uint16) error {
	p.resized <- [2]uint16{cols, rows}
	return nil
}

// exit ends the leader with code.
func (p *fakeProc) exit(code int) {
	p.exitOnce.Do(func() {
		p.code = code
		close(p.exited)
	})
}

func (p *fakeProc) Wait() int {
	<-p.exited
	return p.code
}

func (p *fakeProc) Kill() error {
	p.killOnce.Do(func() { close(p.killed) })
	p.exit(codeKill)
	return nil
}

func (p *fakeProc) PID() int { return 42 }

// fakeStarter hands out fakeProcs and records the starts.
type fakeStarter struct {
	mu    sync.Mutex
	specs []pty.StartSpec
	procs chan *fakeProc
	err   error
}

func (s *fakeStarter) Start(spec pty.StartSpec) (pty.Process, error) {
	s.mu.Lock()
	s.specs = append(s.specs, spec)
	s.mu.Unlock()
	if s.err != nil {
		return nil, s.err
	}
	p := newFakeProc()
	s.procs <- p
	return p, nil
}

func (s *fakeStarter) last() pty.StartSpec {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.specs[len(s.specs)-1]
}

// chanRig is one session channel served by ServeSessions over an in-memory pipe.
type chanRig struct {
	t       *testing.T
	ctx     context.Context
	svc     net.Conn
	starter *fakeStarter
	st      *State
	packets chan localchan.Packet
	done    chan struct{}
}

func sessionEnv(t *testing.T, starter *fakeStarter) SessionEnv {
	t.Helper()
	w := hostostest.New("linux")
	w.AddUser("bothy-s-alex", "1001", selfHome, true)
	w.Dir(selfHome, "1001", 0o700)
	w.Dir(selfHome+"/.config", "1001", 0o700)
	w.Dir(selfHome+"/.config/bothy-host", "1001", 0o700)
	w.Dir(selfHome+"/.config/bothy-host/claude", "1001", 0o700)
	w.Dir(workdir, "1001", 0o700)
	w.Dir("/opt", "0", 0o755)
	w.Dir("/opt/tool", "0", 0o755)
	w.Dir("/opt/tool/bin", "0", 0o755)
	w.File(tool, "0", 0o755, "")
	w.File("/usr/bin/claude", "0", 0o755, "")
	w.Dir("/bin", "0", 0o755)
	w.File("/bin/sh", "0", 0o755, "")
	w.Dir(selfHome+"/.claude", "1001", 0o700)
	self, _ := w.Users().Lookup("bothy-s-alex")
	v := w.As("bothy-s-alex")
	check := doctor.HelperEnv{GOOS: "linux", Self: self, Users: w.Users(), Security: w.Security(), Resolver: v, Files: v, Probe: v.Probe(),
		Runner: &hostostest.Runner{}}
	prof := profiles.Env{GOOS: "linux", Self: self, Resolver: v, Security: w.Security(), Shells: []string{"/bin/sh"},
		LoginEnv: func(context.Context, time.Duration) (bool, string, error) { return false, "", nil }}
	return SessionEnv{Check: check, Profiles: prof, PTY: starter}
}

func newChanRig(t *testing.T) *chanRig {
	t.Helper()
	starter := &fakeStarter{procs: make(chan *fakeProc, 4)}
	svc, hlp := net.Pipe()
	r := &chanRig{t: t, ctx: testkit.Context(t), svc: svc, starter: starter, st: &State{},
		packets: make(chan localchan.Packet, 64), done: make(chan struct{})}
	go func() {
		defer close(r.done)
		ServeSessions(r.ctx, localchan.NewConn(hlp, localchan.Peer{}), sessionEnv(t, starter), r.st)
	}()
	go func() {
		defer close(r.packets)
		for {
			p, err := localchan.ReadPacket(svc)
			if err != nil {
				return
			}
			r.packets <- p
		}
	}()
	t.Cleanup(func() { _ = svc.Close(); <-r.done })
	return r
}

func (r *chanRig) send(op string, d any) {
	r.t.Helper()
	b, err := json.Marshal(d)
	if err != nil {
		r.t.Fatal(err)
	}
	if err := localchan.WriteMessage(r.svc, localchan.Message{Op: op, ID: "1", D: b}); err != nil {
		r.t.Fatalf("send %s: %v", op, err)
	}
}

func (r *chanRig) sendFrame(id string, f frame.Frame) {
	r.t.Helper()
	sid, _ := noise.ParseID(id)
	fb, err := frame.Encode(f)
	if err != nil {
		r.t.Fatal(err)
	}
	b, err := localchan.EncodeFrame(sid, fb)
	if err != nil {
		r.t.Fatal(err)
	}
	if _, err := r.svc.Write(b); err != nil {
		r.t.Fatalf("send frame: %v", err)
	}
}

// next returns the next packet of the helper.
func (r *chanRig) next() localchan.Packet {
	r.t.Helper()
	select {
	case p, ok := <-r.packets:
		if !ok {
			r.t.Fatal("the helper closed the channel")
		}
		return p
	case <-time.After(testkit.Patience):
		r.t.Fatal("no packet from the helper")
	}
	return localchan.Packet{}
}

// expect returns the body of the next control message, which must be op.
func (r *chanRig) expect(op string, into any) {
	r.t.Helper()
	p := r.next()
	if p.IsFrame || p.Msg.Op != op {
		r.t.Fatalf("got %+v, want op %s", p.Msg, op)
	}
	if err := json.Unmarshal(p.Msg.D, into); err != nil {
		r.t.Fatal(err)
	}
}

func (r *chanRig) proc() *fakeProc {
	r.t.Helper()
	select {
	case p := <-r.starter.procs:
		return p
	case <-time.After(testkit.Patience):
		r.t.Fatal("no PTY started")
	}
	return nil
}

// closed waits until ServeSessions returned.
func (r *chanRig) closed() {
	r.t.Helper()
	select {
	case <-r.done:
	case <-time.After(testkit.Patience):
		r.t.Fatal("the helper kept the channel open")
	}
}

func receive[T any](t *testing.T, ch <-chan T, what string) T {
	t.Helper()
	select {
	case v := <-ch:
		return v
	case <-time.After(testkit.Patience):
		t.Fatalf("no %s", what)
	}
	var zero T
	return zero
}

func startCommand(r *chanRig, id string) *fakeProc {
	r.t.Helper()
	r.send(localchan.OpPTYStart, localchan.PTYStart{SessionID: id, Profile: "tool", Kind: config.KindCommand, Program: tool,
		Args: []string{"-v"}, Cols: 80, Rows: 24})
	var started localchan.PTYStarted
	r.expect(localchan.OpPTYStarted, &started)
	if started.SessionID != id {
		r.t.Fatalf("pty.started for %q", started.SessionID)
	}
	return r.proc()
}

// TestSessionLifecycle: pty.start starts the command with the checked path and the size of the
// request, input frames reach the PTY, output goes out in data frames of the session, pty.resize
// and pty.stop act on the PTY, and the end is reported once with the exit code.
func TestSessionLifecycle(t *testing.T) {
	r := newChanRig(t)
	p := startCommand(r, sessOne)
	spec := r.starter.last()
	if spec.Path != tool || spec.Argv0 != tool || !slices.Equal(spec.Args, []string{"-v"}) || spec.Dir != selfHome ||
		spec.Cols != 80 || spec.Rows != 24 || spec.SessionID != sessOne {
		t.Fatalf("start spec %+v", spec)
	}
	// The helper clears the start environment once the PTY runs, so no value of it stays in memory.
	if len(spec.Env) == 0 || slices.ContainsFunc(spec.Env, func(v string) bool { return v != "" }) {
		t.Errorf("start environment not cleared after the start: %d entries", len(spec.Env))
	}

	r.sendFrame(sessOne, frame.Data([]byte("ls\n")))
	if got := receive(t, p.written, "input"); string(got) != "ls\n" {
		t.Errorf("input %q", got)
	}
	// A frame of an unknown session and a frame that is no data frame are dropped.
	r.sendFrame(sessTwo, frame.Data([]byte("lost")))
	r.sendFrame(sessOne, frame.Resize(1, 1))
	r.sendFrame(sessOne, frame.Data([]byte("pwd\n")))
	if got := receive(t, p.written, "input"); string(got) != "pwd\n" {
		t.Errorf("input after dropped frames %q", got)
	}

	p.out <- []byte("hello")
	out := r.next()
	sid, _ := noise.ParseID(sessOne)
	if !out.IsFrame || out.SessionID != sid {
		t.Fatalf("output packet %+v", out)
	}
	if f, err := frame.Decode(out.Frame, frame.FromHost); err != nil || f.Type != frame.TypeData || string(f.Data) != "hello" {
		t.Fatalf("output frame %+v (%v)", f, err)
	}

	r.send(localchan.OpPTYResize, localchan.PTYResize{SessionID: sessOne, Cols: 120, Rows: 40})
	if got := receive(t, p.resized, "resize"); got != [2]uint16{120, 40} {
		t.Errorf("resize %v", got)
	}
	// Resize and stop of an unknown or malformed session change nothing.
	r.send(localchan.OpPTYResize, localchan.PTYResize{SessionID: sessTwo, Cols: 1, Rows: 1})
	r.send(localchan.OpPTYStop, localchan.PTYStop{SessionID: "not-a-session"})

	r.send(localchan.OpPTYStop, localchan.PTYStop{SessionID: sessOne})
	var exited localchan.PTYExited
	r.expect(localchan.OpPTYExited, &exited)
	if exited.SessionID != sessOne || exited.ExitCode != codeKill {
		t.Errorf("pty.exited %+v", exited)
	}
	select {
	case <-p.killed:
	default:
		t.Error("pty.stop did not kill the tree")
	}
}

// TestSessionExitReportsCode: a leader that exits on its own ends the tree and reports its code.
func TestSessionExitReportsCode(t *testing.T) {
	r := newChanRig(t)
	p := startCommand(r, sessOne)
	p.exit(3)
	var exited localchan.PTYExited
	r.expect(localchan.OpPTYExited, &exited)
	if exited.ExitCode != 3 {
		t.Errorf("exit code %d", exited.ExitCode)
	}
	select {
	case <-p.killed:
	default:
		t.Error("the tree was not ended after the leader exited")
	}
	// The ended session takes no more input.
	r.sendFrame(sessOne, frame.Data([]byte("late")))
	r.send(localchan.OpPTYStop, localchan.PTYStop{SessionID: sessOne})
	select {
	case got := <-p.written:
		t.Errorf("input %q reached an ended session", got)
	default:
	}
}

// TestSessionChannelBreakEndsAll: when the channel breaks, every PTY of it ends with its tree.
func TestSessionChannelBreakEndsAll(t *testing.T) {
	r := newChanRig(t)
	a := startCommand(r, sessOne)
	b := startCommand(r, sessTwo)
	_ = r.svc.Close()
	r.closed()
	for _, p := range []*fakeProc{a, b} {
		select {
		case <-p.killed:
		case <-time.After(testkit.Patience):
			t.Fatal("a PTY outlived its channel")
		}
	}
}

// TestSessionStartFailures: every start the helper refuses answers pty.failed with its detail and
// starts nothing.
func TestSessionStartFailures(t *testing.T) {
	cases := []struct {
		name   string
		req    localchan.PTYStart
		check  *doctor.Request
		err    error
		detail string
	}{
		{"unknown kind", localchan.PTYStart{Kind: "other"}, nil, nil, profiles.DetailPTYFailed},
		{"relative program", localchan.PTYStart{Kind: config.KindCommand, Program: "run"}, nil, nil, profiles.DetailPTYFailed},
		{"start fails", localchan.PTYStart{Kind: config.KindCommand, Program: tool}, nil, errors.New("no pty"), profiles.DetailPTYFailed},
		{"claude without a check", localchan.PTYStart{Kind: config.KindClaude, Declaration: state.DeclOrganization, Workdir: workdir, APIKey: "k"},
			nil, nil, profiles.DetailSharedLocation},
		{"claude in a folder outside the check", localchan.PTYStart{Kind: config.KindClaude, Declaration: state.DeclOrganization, Workdir: selfHome, APIKey: "k"},
			&doctor.Request{Workdirs: []string{workdir}, ClaudeBin: "/usr/bin/claude"}, nil, profiles.DetailSharedLocation},
		{"claude without a program", localchan.PTYStart{Kind: config.KindClaude, Declaration: state.DeclOrganization, Workdir: workdir, APIKey: "k"},
			&doctor.Request{Workdirs: []string{workdir}}, nil, profiles.DetailPTYFailed},
		{"claude without a key", localchan.PTYStart{Kind: config.KindClaude, Declaration: state.DeclOrganization, Workdir: workdir},
			&doctor.Request{Workdirs: []string{workdir}, ClaudeBin: "/usr/bin/claude"}, nil, profiles.DetailPTYFailed},
		{"claude with no declaration", localchan.PTYStart{Kind: config.KindClaude, Declaration: state.DeclNone, Workdir: workdir},
			&doctor.Request{Workdirs: []string{workdir}, ClaudeBin: "/usr/bin/claude"}, nil, profiles.DetailPTYFailed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := newChanRig(t)
			r.starter.err = tc.err
			if tc.check != nil {
				r.st.remember(*tc.check, doctor.HelperResult{WorkdirIDs: []string{workdir}})
			}
			tc.req.SessionID = sessOne
			r.send(localchan.OpPTYStart, tc.req)
			var failed localchan.PTYFailed
			r.expect(localchan.OpPTYFailed, &failed)
			if failed.SessionID != sessOne || failed.CloseDetail != tc.detail {
				t.Errorf("pty.failed %+v, want %s", failed, tc.detail)
			}
		})
	}
}

// TestSessionClaudeStarts: claude starts in the checked working directory, with organization the
// key goes only into the environment of that start, and with personal the login shell runs it.
func TestSessionClaudeStarts(t *testing.T) {
	r := newChanRig(t)
	r.st.remember(doctor.Request{Workdirs: []string{workdir}, ClaudeBin: "/usr/bin/claude", SearchList: []string{"/usr/bin"},
		AuthStatusTimeout: testkit.Patience},
		doctor.HelperResult{WorkdirIDs: []string{workdir}})
	r.send(localchan.OpPTYStart, localchan.PTYStart{SessionID: sessOne, Kind: config.KindClaude, Declaration: state.DeclOrganization,
		Workdir: workdir, APIKey: "secret-key", Cols: 80, Rows: 24})
	var started localchan.PTYStarted
	r.expect(localchan.OpPTYStarted, &started)
	r.proc()
	spec := r.starter.last()
	if spec.Path != "/usr/bin/claude" || spec.Dir != workdir {
		t.Fatalf("claude start %+v", spec)
	}

	r.send(localchan.OpPTYStart, localchan.PTYStart{SessionID: sessTwo, Kind: config.KindClaude, Declaration: state.DeclPersonal,
		Workdir: workdir, Cols: 80, Rows: 24})
	r.expect(localchan.OpPTYStarted, &started)
	r.proc()
	if spec := r.starter.last(); spec.Dir != workdir || spec.Path != "/bin/sh" || spec.Argv0 != "-sh" {
		t.Fatalf("personal claude start %+v", spec)
	}
}

// TestSessionShellStarts: the profile shell starts the login shell of the session user in its home.
func TestSessionShellStarts(t *testing.T) {
	r := newChanRig(t)
	r.send(localchan.OpPTYStart, localchan.PTYStart{SessionID: sessOne, Kind: config.KindShell, Cols: 80, Rows: 24})
	var started localchan.PTYStarted
	r.expect(localchan.OpPTYStarted, &started)
	r.proc()
	if spec := r.starter.last(); spec.Path != "/bin/sh" || spec.Argv0 != "-sh" || spec.Dir != selfHome {
		t.Fatalf("shell start %+v", spec)
	}
}

// TestSessionChannelRefusesViolations: an unknown op, a malformed session op, a pty.start with a
// malformed session ID and a check.run for another session user close the channel.
func TestSessionChannelRefusesViolations(t *testing.T) {
	cases := []struct {
		name string
		op   string
		d    any
	}{
		{"unknown op", "pty.unknown", map[string]string{}},
		{"malformed start", localchan.OpPTYStart, []int{1}},
		{"malformed resize", localchan.OpPTYResize, []int{1}},
		{"malformed stop", localchan.OpPTYStop, []int{1}},
		{"malformed session id", localchan.OpPTYStart, localchan.PTYStart{SessionID: "x", Kind: config.KindShell}},
		{"check of another user", localchan.OpCheckRun, doctor.Request{SessionUser: "bothy-s-sam"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := newChanRig(t)
			r.send(tc.op, tc.d)
			r.closed()
		})
	}
}

// TestSessionChannelServesChecks: hello and check.run of M2-42 are answered on a session channel,
// and check.run is remembered for the starts of claude.
func TestSessionChannelServesChecks(t *testing.T) {
	r := newChanRig(t)
	r.send(localchan.OpHello, map[string]int{"v": localchan.ProtocolVersion})
	var hello map[string]any
	r.expect(localchan.OpHello, &hello)

	r.send(localchan.OpCheckRun, doctor.Request{Mode: doctor.ModeLight, SessionUser: "bothy-s-alex", Workdirs: []string{workdir}})
	var res doctor.HelperResult
	r.expect(localchan.OpCheckResult, &res)
	last, _, ok := r.st.last()
	if !ok || !slices.Equal(last.Workdirs, []string{workdir}) {
		t.Errorf("remembered check %+v (%v)", last, ok)
	}
}
