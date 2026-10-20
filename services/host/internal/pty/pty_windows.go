// SPDX-License-Identifier: Apache-2.0

package pty

import (
	"math"
	"os"
	"slices"
	"strings"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

// winProcess is a session on Windows: a pseudo console and a Job Object holding the tree.
type winProcess struct {
	sup     *Supervisor
	job     windows.Handle
	process windows.Handle
	pid     int
	in      *os.File
	out     *os.File

	// mu guards console against a Resize after ClosePseudoConsole.
	mu      sync.Mutex
	console windows.Handle
	closed  bool

	// done is closed once the leader exited and the rest of its tree was terminated; code is set
	// before.
	done chan struct{}
	code int

	killOnce sync.Once
	killErr  error
}

// coord converts a terminal size to the console's coordinate type, which ends at the largest
// int16.
func coord(cols, rows uint16) windows.Coord {
	clamp := func(v uint16) int16 {
		if v > math.MaxInt16 {
			return math.MaxInt16
		}
		return int16(v)
	}
	return windows.Coord{X: clamp(cols), Y: clamp(rows)}
}

// envBlock builds the Unicode environment block of env, sorted by name without regard to case
// as Windows expects; an empty environment is an empty block, never the helper's own.
func envBlock(env []string) (*uint16, error) {
	sorted := slices.Clone(env)
	slices.SortStableFunc(sorted, func(a, b string) int {
		ka, _, _ := strings.Cut(a, "=")
		kb, _, _ := strings.Cut(b, "=")
		return strings.Compare(strings.ToUpper(ka), strings.ToUpper(kb))
	})
	var block []uint16
	for _, kv := range sorted {
		u, err := windows.UTF16FromString(kv)
		if err != nil {
			return nil, err
		}
		block = append(block, u...)
	}
	if len(block) == 0 {
		block = append(block, 0)
	}
	block = append(block, 0)
	return &block[0], nil
}

// start creates the pseudo console and the Job Object, creates the leader suspended with the
// pseudo console attribute, assigns it to the job and only then lets it run, so no process of
// the session ever runs outside the job.
func (s *Supervisor) start(spec *StartSpec) (Process, error) {
	var inR, inW, outR, outW windows.Handle
	if err := windows.CreatePipe(&inR, &inW, nil, 0); err != nil {
		return nil, failure(ErrStart, err)
	}
	if err := windows.CreatePipe(&outR, &outW, nil, 0); err != nil {
		_ = windows.CloseHandle(inR)
		_ = windows.CloseHandle(inW)
		return nil, failure(ErrStart, err)
	}
	var console windows.Handle
	err := windows.CreatePseudoConsole(coord(spec.Cols, spec.Rows), inR, outW, 0, &console)
	// The pseudo console holds its own copies of its ends of the pipes.
	_ = windows.CloseHandle(inR)
	_ = windows.CloseHandle(outW)
	if err != nil {
		_ = windows.CloseHandle(inW)
		_ = windows.CloseHandle(outR)
		return nil, failure(ErrStart, err)
	}
	in := os.NewFile(uintptr(inW), "pty-input")
	out := os.NewFile(uintptr(outR), "pty-output")
	fail := func(cause error) (Process, error) {
		windows.ClosePseudoConsole(console)
		_ = in.Close()
		_ = out.Close()
		return nil, failure(ErrStart, cause)
	}
	job, err := newJob()
	if err != nil {
		return fail(err)
	}
	pi, err := createSuspended(spec, console)
	if err != nil {
		_ = windows.CloseHandle(job)
		return fail(err)
	}
	if err := windows.AssignProcessToJobObject(job, pi.Process); err != nil {
		_ = windows.TerminateProcess(pi.Process, killedExitCode)
		_ = windows.CloseHandle(pi.Thread)
		_ = windows.CloseHandle(pi.Process)
		_ = windows.CloseHandle(job)
		return fail(err)
	}
	if _, err := windows.ResumeThread(pi.Thread); err != nil {
		_ = windows.TerminateJobObject(job, killedExitCode)
		_ = windows.CloseHandle(pi.Thread)
		_ = windows.CloseHandle(pi.Process)
		_ = windows.CloseHandle(job)
		return fail(err)
	}
	_ = windows.CloseHandle(pi.Thread)
	p := &winProcess{
		sup: s, job: job, process: pi.Process, pid: int(pi.ProcessId),
		in: in, out: out, console: console, done: make(chan struct{}),
	}
	go p.reap()
	return p, nil
}

// createSuspended creates the leader suspended, attached to console, with exactly the
// environment of spec.
func createSuspended(spec *StartSpec, console windows.Handle) (*windows.ProcessInformation, error) {
	attrs, err := windows.NewProcThreadAttributeList(1)
	if err != nil {
		return nil, err
	}
	defer attrs.Delete()
	// The attribute value is the console handle itself, not a pointer to it.
	value := *(*unsafe.Pointer)(unsafe.Pointer(&console)) //nolint:gosec // the handle's bits as the pointer-typed value the API takes
	if err := attrs.Update(windows.PROC_THREAD_ATTRIBUTE_PSEUDOCONSOLE, value, unsafe.Sizeof(console)); err != nil {
		return nil, err
	}
	si := windows.StartupInfoEx{ProcThreadAttributeList: attrs.List()}
	si.Cb = uint32(unsafe.Sizeof(si))
	app, err := windows.UTF16PtrFromString(spec.Path)
	if err != nil {
		return nil, err
	}
	line, err := windows.UTF16PtrFromString(windows.ComposeCommandLine(append([]string{spec.argv0()}, spec.Args...)))
	if err != nil {
		return nil, err
	}
	dir, err := windows.UTF16PtrFromString(spec.Dir)
	if err != nil {
		return nil, err
	}
	env, err := envBlock(spec.env())
	if err != nil {
		return nil, err
	}
	var pi windows.ProcessInformation
	flags := uint32(windows.CREATE_SUSPENDED | windows.EXTENDED_STARTUPINFO_PRESENT | windows.CREATE_UNICODE_ENVIRONMENT)
	if err := windows.CreateProcess(app, line, nil, nil, false, flags, env, dir, &si.StartupInfo, &pi); err != nil {
		return nil, err
	}
	return &pi, nil
}

// reap waits for the end of the leader, terminates the rest of its job and keeps the exit code.
func (p *winProcess) reap() {
	_, _ = windows.WaitForSingleObject(p.process, windows.INFINITE)
	_ = windows.TerminateJobObject(p.job, killedExitCode)
	var code uint32
	if err := windows.GetExitCodeProcess(p.process, &code); err == nil {
		p.code = int(code)
	} else {
		p.code = -1
	}
	close(p.done)
}

// Read reads output of the pseudo console.
func (p *winProcess) Read(b []byte) (int, error) { return p.out.Read(b) }

// Write writes input to the pseudo console.
func (p *winProcess) Write(b []byte) (int, error) { return p.in.Write(b) }

// PID returns the process ID of the session leader.
func (p *winProcess) PID() int { return p.pid }

// Wait returns the exit code of the leader once it exited.
func (p *winProcess) Wait() int {
	<-p.done
	return p.code
}

// Resize changes the size of the pseudo console.
func (p *winProcess) Resize(cols, rows uint16) error {
	if cols == 0 || rows == 0 {
		return ErrResize
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return ErrResize
	}
	if err := windows.ResizePseudoConsole(p.console, coord(cols, rows)); err != nil {
		return failure(ErrResize, err)
	}
	return nil
}

// Kill terminates the job, waits for the leader, closes the pseudo console, which ends the
// output, and releases every handle.
func (p *winProcess) Kill() error {
	p.killOnce.Do(func() {
		termErr := windows.TerminateJobObject(p.job, killedExitCode)
		<-p.done
		p.mu.Lock()
		p.closed = true
		windows.ClosePseudoConsole(p.console)
		p.mu.Unlock()
		_ = p.in.Close()
		_ = p.out.Close()
		_ = windows.CloseHandle(p.process)
		_ = windows.CloseHandle(p.job)
		p.sup.forget(p)
		if termErr != nil {
			p.killErr = failure(ErrKill, termErr)
		}
	})
	return p.killErr
}
