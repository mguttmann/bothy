// SPDX-License-Identifier: Apache-2.0

package pty

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

// runFileName is the name of the run file in Options.RunDir.
const runFileName = "pty-sessions.json"

// statZombie is SZOMB of the p_stat field of macOS (sys/proc.h).
const statZombie = 5

// exitPoll is the pause between two looks at the leader when kqueue is not available.
const exitPoll = 20 * time.Millisecond

// runEntry records one live session in the run file.
type runEntry struct {
	// PID is the session leader.
	PID int `json:"pid"`
	// StartSec and StartUsec are the leader's start time; together with PID they tell the same
	// process from a later one with a reused PID.
	StartSec  int64 `json:"start_sec"`
	StartUsec int64 `json:"start_usec"`
	// PGID and SID are the process group and POSIX session of the leader.
	PGID int `json:"pgid"`
	SID  int `json:"sid"`
}

// platform is the macOS state of a Supervisor: the run file and its entries.
type platform struct {
	runDir  string
	mu      sync.Mutex
	entries map[int]runEntry
}

// prepare ends the trees the run file of o.RunDir records and rewrites the file empty. A run
// file that cannot be read or parsed records nothing usable and is replaced.
func (pl *platform) prepare(o Options) error {
	pl.runDir = o.RunDir
	pl.entries = make(map[int]runEntry)
	if pl.runDir == "" {
		return nil
	}
	if raw, err := os.ReadFile(filepath.Join(pl.runDir, runFileName)); err == nil {
		var recorded []runEntry
		if json.Unmarshal(raw, &recorded) == nil {
			for _, e := range recorded {
				endRecorded(e)
			}
		}
	}
	pl.mu.Lock()
	defer pl.mu.Unlock()
	if err := pl.writeLocked(); err != nil {
		return failure(ErrRunFile, err)
	}
	return nil
}

// endRecorded ends the tree of e when its leader is still the recorded process. When no process
// has the leader's PID, the members of its session and process group are ended, because they
// keep both IDs taken, so no later process can have them. A leader PID that names another
// process now is left alone.
func endRecorded(e runEntry) {
	if e.PID <= 1 {
		return
	}
	leader := e.PID
	kp, err := unix.SysctlKinfoProc("kern.proc.pid", e.PID)
	switch {
	case err != nil || kp.Proc.P_pid != int32(e.PID): //nolint:gosec // PIDs fit in int32 on macOS.
		leader = -1
	case kp.Proc.P_starttime.Sec != e.StartSec || int64(kp.Proc.P_starttime.Usec) != e.StartUsec:
		return
	}
	_ = sweepGroup(snapshot, e.SID, e.PGID, leader)
}

// record adds e to the run file.
func (pl *platform) record(e runEntry) error {
	if pl.runDir == "" {
		return nil
	}
	pl.mu.Lock()
	defer pl.mu.Unlock()
	pl.entries[e.PID] = e
	if err := pl.writeLocked(); err != nil {
		delete(pl.entries, e.PID)
		return failure(ErrRunFile, err)
	}
	return nil
}

// drop removes the entry of pid from the run file.
func (pl *platform) drop(pid int) error {
	if pl.runDir == "" {
		return nil
	}
	pl.mu.Lock()
	defer pl.mu.Unlock()
	delete(pl.entries, pid)
	return pl.writeLocked()
}

// writeLocked writes the entries atomically with mode 0600: a temporary file in the same folder,
// synced, then renamed over the run file. The caller holds pl.mu.
func (pl *platform) writeLocked() error {
	list := make([]runEntry, 0, len(pl.entries))
	for _, e := range pl.entries {
		list = append(list, e)
	}
	slices.SortFunc(list, func(a, b runEntry) int { return a.PID - b.PID })
	raw, err := json.Marshal(list)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(pl.runDir, ".pty-sessions-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	if err := tmp.Chmod(0o600); err != nil {
		return errors.Join(err, tmp.Close(), os.Remove(name))
	}
	if _, err := tmp.Write(raw); err != nil {
		return errors.Join(err, tmp.Close(), os.Remove(name))
	}
	if err := tmp.Sync(); err != nil {
		return errors.Join(err, tmp.Close(), os.Remove(name))
	}
	if err := tmp.Close(); err != nil {
		return errors.Join(err, os.Remove(name))
	}
	if err := os.Rename(name, filepath.Join(pl.runDir, runFileName)); err != nil {
		return errors.Join(err, os.Remove(name))
	}
	return nil
}

// darwinTree is the tree of one macOS session: its POSIX session, whose ID is the leader's PID
// because the leader starts with setsid.
type darwinTree struct {
	plat   *platform
	leader int
}

// newTree prepares the tree of a new session; the leader's own session needs nothing more than
// the Setsid the caller set.
func (s *Supervisor) newTree(string, *syscall.SysProcAttr) (sessionTree, error) {
	return &darwinTree{plat: &s.plat}, nil
}

// started records the leader with its start time in the run file.
func (t *darwinTree) started(pid int) error {
	if pid <= 0 {
		return nil
	}
	t.leader = pid
	e := runEntry{PID: pid, PGID: pid, SID: pid}
	if kp, err := unix.SysctlKinfoProc("kern.proc.pid", pid); err == nil {
		e.StartSec = kp.Proc.P_starttime.Sec
		e.StartUsec = int64(kp.Proc.P_starttime.Usec)
	}
	return t.plat.record(e)
}

// sweep kills every process of the session.
func (t *darwinTree) sweep(leader int) error {
	return sweepGroup(snapshot, leader, leader, leader)
}

// release removes the session from the run file.
func (t *darwinTree) release() error {
	if t.leader == 0 {
		return nil
	}
	return t.plat.drop(t.leader)
}

// snapshot reads the process table. The POSIX session is read with getsid only for processes of
// the calling user, the only ones a tree end may kill.
func snapshot() ([]procEntry, error) {
	kps, err := unix.SysctlKinfoProcSlice("kern.proc.all")
	if err != nil {
		return nil, err
	}
	uid := os.Geteuid()
	out := make([]procEntry, 0, len(kps))
	for i := range kps {
		kp := &kps[i]
		e := procEntry{
			pid:    int(kp.Proc.P_pid),
			ppid:   int(kp.Eproc.Ppid),
			pgid:   int(kp.Eproc.Pgid),
			sid:    -1,
			uid:    int(kp.Eproc.Ucred.Uid),
			zombie: kp.Proc.P_stat == statZombie,
		}
		if e.uid == uid {
			if sid, err := unix.Getsid(e.pid); err == nil {
				e.sid = sid
			}
		}
		out = append(out, e)
	}
	return out, nil
}

// waitExited blocks until pid has exited without reaping it. Without kqueue it polls the
// process table.
func waitExited(pid int) {
	if waitKqueue(pid) {
		return
	}
	for !exited(pid) {
		time.Sleep(exitPoll)
	}
}

// waitKqueue waits for the NOTE_EXIT of pid and reports whether it saw the exit. It looks at the
// process table after registering, because a leader that already exited may give no event.
func waitKqueue(pid int) bool {
	kq, err := unix.Kqueue()
	if err != nil {
		return false
	}
	defer func() { _ = unix.Close(kq) }()
	change := unix.Kevent_t{Ident: uint64(pid), Filter: unix.EVFILT_PROC, Flags: unix.EV_ADD | unix.EV_ONESHOT, Fflags: unix.NOTE_EXIT} //nolint:gosec // a PID is positive.
	if _, err := unix.Kevent(kq, []unix.Kevent_t{change}, nil, &unix.Timespec{}); err != nil {
		return false
	}
	if exited(pid) {
		return true
	}
	events := make([]unix.Kevent_t, 1)
	for {
		n, err := unix.Kevent(kq, nil, events, nil)
		if errors.Is(err, unix.EINTR) {
			continue
		}
		return err == nil && n == 1 && events[0].Fflags&unix.NOTE_EXIT != 0
	}
}

// exited reports whether pid is a zombie or gone.
func exited(pid int) bool {
	kp, err := unix.SysctlKinfoProc("kern.proc.pid", pid)
	return err != nil || kp.Proc.P_pid != int32(pid) || kp.Proc.P_stat == statZombie //nolint:gosec // PIDs fit in int32 on macOS.
}
