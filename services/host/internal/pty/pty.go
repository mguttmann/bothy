// SPDX-License-Identifier: Apache-2.0

package pty

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"syscall"
)

// StartSpec describes one PTY start; every value comes from the helper, never from a client.
type StartSpec struct {
	// SessionID is the canonical session UUID in lower case; it names the cgroup s-<id> on Linux.
	SessionID string
	// Path is the absolute path of the program.
	Path string
	// Argv0 is argv[0]; "" means Path. A login shell passes "-" plus the base name of the shell.
	Argv0 string
	// Args are the arguments after argv[0].
	Args []string
	// Env is the complete environment of the program; nothing is inherited from the helper.
	Env []string
	// Dir is the absolute working directory.
	Dir string
	// Cols and Rows are the initial size of the terminal; both must be above zero.
	Cols, Rows uint16
}

// Process is one running session: the PTY and its whole process tree.
type Process interface {
	// Read reads PTY output. It returns an error (io.EOF or another) once the PTY is closed or no
	// process of the tree holds the terminal any more.
	Read(p []byte) (int, error)
	// Write writes input to the PTY.
	Write(p []byte) (int, error)
	// Resize changes the size of the terminal; it fails with ErrResize after Kill or for a zero
	// size.
	Resize(cols, rows uint16) error
	// Wait blocks until the session leader exited and returns its exit code: on Unix the exit
	// status, or 128 plus the signal number after a signal; on Windows the exit code of the
	// process. It may be called more than once and returns the same value each time.
	Wait() int
	// Kill ends the whole tree at once without grace and releases the PTY. It is idempotent and
	// also valid after Wait; it returns ErrKill when a process of the tree could not be ended.
	Kill() error
	// PID returns the process ID of the session leader.
	PID() int
}

// Options configure a Supervisor.
type Options struct {
	// CgroupRoot (Linux) is the delegated cgroup of the helper as a directory under
	// /sys/fs/cgroup; "" means the own cgroup from /proc/self/cgroup, or its parent when the own
	// cgroup is already the leaf "helper" of an earlier New.
	CgroupRoot string
	// ProcessGroupOnly (Linux, tests only; cmd/bothy-host never sets it) uses no cgroup, only the
	// process group and session of each PTY like macOS without the run file. It exists because
	// test runners usually have no delegated cgroup; TestSessionCgroup never uses it.
	ProcessGroupOnly bool
	// RunDir (macOS) is the folder of the run file; production passes
	// <home>/.config/bothy-host/run, created with mode 0700 by the caller. "" disables the run
	// file.
	RunDir string
}

// The errors of this package. Their text names an operation and a code and never a path, a
// command line or an environment value; an error may wrap one of them together with the
// operating system's error number.
var (
	// ErrNoDelegation means New found no cgroup v2 directory it may create cgroups in (Linux).
	ErrNoDelegation = errors.New("pty new: no_delegation")
	// ErrRunFile means the run file could not be written (macOS).
	ErrRunFile = errors.New("pty new: run_file_failed")
	// ErrSpec means a StartSpec value is invalid.
	ErrSpec = errors.New("pty start: invalid_spec")
	// ErrClosed means Start ran after Close.
	ErrClosed = errors.New("pty start: supervisor_closed")
	// ErrStart means the PTY or the program could not be started.
	ErrStart = errors.New("pty start: start_failed")
	// ErrCgroup means the cgroup of the session could not be created (Linux); the PTY does not
	// start.
	ErrCgroup = errors.New("pty start: cgroup_failed")
	// ErrResize means the terminal size could not be changed.
	ErrResize = errors.New("pty resize: resize_failed")
	// ErrKill means a process of the tree could not be ended or its cgroup not removed.
	ErrKill = errors.New("pty kill: kill_failed")
)

// failure returns sentinel, joined with the error number of cause when there is one, so the
// text gains the reason of the operating system but never the path or command line that the
// cause may carry.
func failure(sentinel, cause error) error {
	var errno syscall.Errno
	if errors.As(cause, &errno) {
		return fmt.Errorf("%w: %w", sentinel, errno)
	}
	return sentinel
}

// sessionIDPattern is the canonical lower-case UUID form; it keeps the cgroup name a plain name.
var sessionIDPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// validate checks spec before anything is created for it.
func (spec *StartSpec) validate() error {
	if !sessionIDPattern.MatchString(spec.SessionID) ||
		!filepath.IsAbs(spec.Path) || !filepath.IsAbs(spec.Dir) ||
		spec.Cols == 0 || spec.Rows == 0 {
		return ErrSpec
	}
	// A NUL would cut a value short in the operating system's view of it.
	values := append([]string{spec.Path, spec.Argv0, spec.Dir}, spec.Args...)
	for _, v := range append(values, spec.Env...) {
		if strings.ContainsRune(v, 0) {
			return ErrSpec
		}
	}
	for _, kv := range spec.Env {
		if k, _, ok := strings.Cut(kv, "="); !ok || k == "" {
			return ErrSpec
		}
	}
	return nil
}

// argv0 returns argv[0] of spec.
func (spec *StartSpec) argv0() string {
	if spec.Argv0 != "" {
		return spec.Argv0
	}
	return spec.Path
}

// env returns a copy of the environment that is never nil, because a nil environment would make
// the operating system calls inherit the helper's own.
func (spec *StartSpec) env() []string {
	return append(make([]string, 0, len(spec.Env)), spec.Env...)
}

// Supervisor starts the sessions of one helper and ends their trees.
type Supervisor struct {
	plat platform

	// startMu is held for reading by every Start and for writing by KillAll, so no session that
	// is starting while the channel breaks survives the break.
	startMu sync.RWMutex

	mu     sync.Mutex
	live   map[Process]struct{}
	closed bool
}

// New prepares the platform and returns a Supervisor.
//
// Linux: unless o.ProcessGroupOnly, it moves the calling process into the leaf <root>/helper
// (cgroup v2 keeps processes out of inner nodes) and fails with ErrNoDelegation when root is no
// cgroup v2 directory or the process may not create cgroups in it. macOS: with o.RunDir it ends
// every tree recorded in the run file whose leader is still the same process, then rewrites the
// file empty; it fails with ErrRunFile when the file cannot be written. Windows: nothing.
func New(o Options) (*Supervisor, error) {
	s := &Supervisor{live: make(map[Process]struct{})}
	if err := s.plat.prepare(o); err != nil {
		return nil, err
	}
	return s, nil
}

// Start starts spec in a new PTY and returns the running session. It fails with ErrSpec for an
// invalid spec, ErrClosed after Close, ErrCgroup when the cgroup of the session cannot be created
// (Linux) and ErrStart when the PTY or the program cannot be started; nothing runs then.
func (s *Supervisor) Start(spec StartSpec) (Process, error) {
	if err := spec.validate(); err != nil {
		return nil, err
	}
	s.startMu.RLock()
	defer s.startMu.RUnlock()
	s.mu.Lock()
	closed := s.closed
	s.mu.Unlock()
	if closed {
		return nil, ErrClosed
	}
	p, err := s.start(&spec)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	s.live[p] = struct{}{}
	s.mu.Unlock()
	return p, nil
}

// forget removes an ended session from the live set.
func (s *Supervisor) forget(p Process) {
	s.mu.Lock()
	delete(s.live, p)
	s.mu.Unlock()
}

// KillAll ends every live tree at once and waits until they are gone; the supervisor stays
// usable. The helper calls it when its channel to the host service breaks (D-W-90).
func (s *Supervisor) KillAll() {
	s.startMu.Lock()
	defer s.startMu.Unlock()
	s.mu.Lock()
	all := make([]Process, 0, len(s.live))
	for p := range s.live {
		all = append(all, p)
	}
	s.mu.Unlock()
	var wg sync.WaitGroup
	for _, p := range all {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = p.Kill()
		}()
	}
	wg.Wait()
}

// Close ends every live tree and releases the supervisor; a later Start fails with ErrClosed.
// It is idempotent and returns nil.
func (s *Supervisor) Close() error {
	s.mu.Lock()
	s.closed = true
	s.mu.Unlock()
	s.KillAll()
	return nil
}
