// SPDX-License-Identifier: Apache-2.0

//go:build linux || darwin

package pty

import (
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/creack/pty"
	"golang.org/x/sys/unix"
)

const (
	// sweepLimit bounds how long one tree end keeps killing before it reports ErrKill.
	sweepLimit = 10 * time.Second
	// sweepPause lets the kernel deliver SIGKILL before the next look at the tree.
	sweepPause = 10 * time.Millisecond
)

// sessionTree is the platform part of one session's process tree.
type sessionTree interface {
	// started runs right after the leader was started (also when the start failed, with pid 0),
	// before anything else happens to the session.
	started(pid int) error
	// sweep kills every process of the tree, repeating until none is left. It runs only while the
	// leader is not reaped, so the leader's PID still names it.
	sweep(leader int) error
	// release runs once after the leader was reaped and the tree is gone.
	release() error
}

// unixProcess is a session on Linux or macOS.
type unixProcess struct {
	sup    *Supervisor
	tree   sessionTree
	master *os.File
	cmd    *exec.Cmd
	pid    int

	treeMu    sync.Mutex
	treeEnded bool
	treeErr   error

	// done is closed once the leader is reaped and the tree released; code and releaseErr are
	// set before.
	done       chan struct{}
	code       int
	releaseErr error

	killOnce sync.Once
	killErr  error
}

// start opens the PTY, prepares the tree and starts the leader in its own session with the PTY
// as controlling terminal.
func (s *Supervisor) start(spec *StartSpec) (Process, error) {
	master, tty, err := pty.Open()
	if err != nil {
		return nil, failure(ErrStart, err)
	}
	if err := setSize(master, spec.Cols, spec.Rows); err != nil {
		_ = tty.Close()
		_ = master.Close()
		return nil, failure(ErrStart, err)
	}
	cmd := &exec.Cmd{
		Path:   spec.Path,
		Args:   append([]string{spec.argv0()}, spec.Args...),
		Env:    spec.env(),
		Dir:    spec.Dir,
		Stdin:  tty,
		Stdout: tty,
		Stderr: tty,
		// Ctty is the descriptor of the terminal in the child: its standard input.
		SysProcAttr: &syscall.SysProcAttr{Setsid: true, Setctty: true, Ctty: 0},
	}
	tree, err := s.newTree(spec.SessionID, cmd.SysProcAttr)
	if err != nil {
		_ = tty.Close()
		_ = master.Close()
		return nil, err
	}
	startErr := cmd.Start()
	_ = tty.Close()
	if startErr != nil {
		_ = tree.started(0)
		_ = tree.release()
		_ = master.Close()
		return nil, failure(ErrStart, startErr)
	}
	p := &unixProcess{sup: s, tree: tree, master: master, cmd: cmd, pid: cmd.Process.Pid, done: make(chan struct{})}
	recordErr := tree.started(p.pid)
	go p.reap()
	if recordErr != nil {
		// A session the helper could not record could outlive a crash of the helper; it does
		// not run.
		_ = p.Kill()
		return nil, recordErr
	}
	return p, nil
}

// reap waits for the end of the leader, ends the rest of its tree while the leader's PID is still
// taken, then reaps the leader and releases the tree.
func (p *unixProcess) reap() {
	waitExited(p.pid)
	_ = p.endTree()
	_ = p.cmd.Wait()
	p.code = exitCode(p.cmd.ProcessState)
	p.releaseErr = p.tree.release()
	close(p.done)
}

// endTree sweeps the tree once; later calls return the result of the first.
func (p *unixProcess) endTree() error {
	p.treeMu.Lock()
	defer p.treeMu.Unlock()
	if !p.treeEnded {
		p.treeErr = p.tree.sweep(p.pid)
		p.treeEnded = true
	}
	return p.treeErr
}

// exitCode maps the state of the reaped leader to the exit code of Process.Wait.
func exitCode(st *os.ProcessState) int {
	if st == nil {
		return -1
	}
	if ws, ok := st.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
		return 128 + int(ws.Signal())
	}
	return st.ExitCode()
}

// Read reads PTY output.
func (p *unixProcess) Read(b []byte) (int, error) { return p.master.Read(b) }

// Write writes input to the PTY.
func (p *unixProcess) Write(b []byte) (int, error) { return p.master.Write(b) }

// PID returns the process ID of the session leader.
func (p *unixProcess) PID() int { return p.pid }

// Wait returns the exit code of the leader once it is reaped.
func (p *unixProcess) Wait() int {
	<-p.done
	return p.code
}

// Resize changes the size of the terminal.
func (p *unixProcess) Resize(cols, rows uint16) error {
	if cols == 0 || rows == 0 {
		return ErrResize
	}
	if err := setSize(p.master, cols, rows); err != nil {
		return failure(ErrResize, err)
	}
	return nil
}

// Kill ends the tree, waits until the leader is reaped and closes the PTY.
func (p *unixProcess) Kill() error {
	p.killOnce.Do(func() {
		treeErr := p.endTree()
		<-p.done
		_ = p.master.Close()
		p.sup.forget(p)
		if treeErr != nil || p.releaseErr != nil {
			p.killErr = ErrKill
		}
	})
	return p.killErr
}

// setSize sets the window size of the terminal behind f. It runs on the raw descriptor, because
// File.Fd would switch a pollable descriptor to blocking mode and keep Close from ending a
// pending Read.
func setSize(f *os.File, cols, rows uint16) error {
	raw, err := f.SyscallConn()
	if err != nil {
		return err
	}
	var ioErr error
	if err := raw.Control(func(fd uintptr) {
		ioErr = unix.IoctlSetWinsize(int(fd), unix.TIOCSWINSZ, &unix.Winsize{Row: rows, Col: cols})
	}); err != nil {
		return err
	}
	return ioErr
}

// procEntry is one process of a process table snapshot.
type procEntry struct {
	pid, ppid, pgid, sid, uid int
	zombie                    bool
}

// members returns the processes of procs that are not zombies and belong to a tree: owned by
// uid, and either the leader, in the POSIX session sid or the process group pgid, or a
// descendant of such a process. A zombie is never killed but still links its children.
func members(procs []procEntry, uid, sid, pgid, leader int) []int {
	in := make(map[int]bool)
	for _, e := range procs {
		if e.uid == uid && (e.pid == leader || e.sid == sid || e.pgid == pgid) {
			in[e.pid] = true
		}
	}
	for addDescendants(procs, uid, in) {
	}
	var out []int
	for _, e := range procs {
		if in[e.pid] && !e.zombie {
			out = append(out, e.pid)
		}
	}
	return out
}

// addDescendants adds to in every process of uid whose parent is in it and reports whether it
// added one.
func addDescendants(procs []procEntry, uid int, in map[int]bool) bool {
	grown := false
	for _, e := range procs {
		if e.uid == uid && !in[e.pid] && in[e.ppid] {
			in[e.pid] = true
			grown = true
		}
	}
	return grown
}

// sweepGroup ends the tree of the session sid and process group pgid; leader is the session
// leader, or -1 when it is gone. It first stops every member with SIGSTOP, taking snapshots until
// one finds no member it has not stopped, because a stopped process neither forks nor exits, so
// no parent chain breaks while the tree is collected. Then it kills with SIGKILL, taking
// snapshots until one finds no member left. It never signals the calling process, its process
// group or a group or session at or below PID 1, so a damaged record cannot reach beyond the
// tree.
func sweepGroup(snapshot func() ([]procEntry, error), sid, pgid, leader int) error {
	if sid <= 1 || pgid <= 1 {
		return ErrKill
	}
	t := treeSignals{
		snapshot: snapshot, sid: sid, pgid: pgid, leader: leader,
		// The group signal reaches members a snapshot may miss. It is sent only while the
		// leader holds the group ID, so it cannot reach a later group with a reused ID, and
		// never to the helper's own group.
		group:    leader > 1 && pgid != unix.Getpgrp(),
		deadline: time.Now().Add(sweepLimit),
	}
	if err := t.freeze(); err != nil {
		return err
	}
	return t.kill()
}

// treeSignals is one tree end of sweepGroup.
type treeSignals struct {
	snapshot          func() ([]procEntry, error)
	sid, pgid, leader int
	group             bool
	deadline          time.Time
}

// targets returns the members of a fresh snapshot, then signals the group when allowed. The
// snapshot comes first: a group SIGKILL before it would end the leader and hand its other
// children to init before the snapshot could see their parent chain.
func (t *treeSignals) targets(sig unix.Signal) ([]int, error) {
	procs, err := t.snapshot()
	if err != nil {
		if t.group {
			_ = unix.Kill(-t.pgid, sig)
		}
		return nil, failure(ErrKill, err)
	}
	found := killable(members(procs, os.Geteuid(), t.sid, t.pgid, t.leader))
	if t.group {
		_ = unix.Kill(-t.pgid, sig)
	}
	return found, nil
}

// freeze stops every member until a snapshot finds none it has not stopped yet; at the deadline
// it leaves the rest to kill.
func (t *treeSignals) freeze() error {
	stopped := make(map[int]bool)
	for time.Now().Before(t.deadline) {
		found, err := t.targets(unix.SIGSTOP)
		if err != nil {
			return err
		}
		fresh := false
		for _, pid := range found {
			if !stopped[pid] {
				_ = unix.Kill(pid, unix.SIGSTOP)
				stopped[pid] = true
				fresh = true
			}
		}
		if !fresh {
			return nil
		}
		// SIGSTOP arrives asynchronously; a child forked before it shows up in the next
		// snapshot.
		time.Sleep(sweepPause)
	}
	return nil
}

// kill sends SIGKILL to every member until a snapshot finds none.
func (t *treeSignals) kill() error {
	for {
		found, err := t.targets(unix.SIGKILL)
		if err != nil {
			return err
		}
		if len(found) == 0 {
			return nil
		}
		for _, pid := range found {
			// A process that ended in between gives ESRCH; one that cannot be killed stays in
			// the next snapshot until the deadline reports it.
			_ = unix.Kill(pid, unix.SIGKILL)
		}
		if time.Now().After(t.deadline) {
			return ErrKill
		}
		time.Sleep(sweepPause)
	}
}

// killable drops the calling process and PIDs at or below 1 from pids.
func killable(pids []int) []int {
	self := os.Getpid()
	out := pids[:0]
	for _, pid := range pids {
		if pid != self && pid > 1 {
			out = append(out, pid)
		}
	}
	return out
}
