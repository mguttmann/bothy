// SPDX-License-Identifier: Apache-2.0

package pty

import (
	"bufio"
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

const (
	// cgroupMount is where the cgroup v2 hierarchy is mounted.
	cgroupMount = "/sys/fs/cgroup"
	// helperLeaf is the leaf cgroup the helper moves itself into.
	helperLeaf = "helper"
	// sessionPrefix starts the name of every session cgroup.
	sessionPrefix = "s-"
	// exitPoll is the pause between two looks at the leader when waitid is not available.
	exitPoll = 20 * time.Millisecond
)

// platform is the Linux state of a Supervisor: the delegated cgroup, or none in the test mode
// Options.ProcessGroupOnly.
type platform struct {
	root      string
	groupOnly bool
}

// ownCgroup returns the cgroup v2 directory of the calling process from /proc/self/cgroup.
func ownCgroup() (string, error) {
	raw, err := os.ReadFile("/proc/self/cgroup")
	if err != nil {
		return "", err
	}
	sc := bufio.NewScanner(bytes.NewReader(raw))
	for sc.Scan() {
		if rel, ok := strings.CutPrefix(sc.Text(), "0::"); ok {
			return filepath.Join(cgroupMount, filepath.Clean("/"+rel)), nil
		}
	}
	return "", ErrNoDelegation
}

// prepare finds the delegated cgroup, checks that it is a cgroup v2 directory and moves the
// calling process into its leaf "helper", which also proves that the process may create and fill
// cgroups there.
func (pl *platform) prepare(o Options) error {
	if o.ProcessGroupOnly {
		pl.groupOnly = true
		return nil
	}
	root := o.CgroupRoot
	if root == "" {
		own, err := ownCgroup()
		if err != nil {
			return failure(ErrNoDelegation, err)
		}
		root = own
		if filepath.Base(own) == helperLeaf {
			root = filepath.Dir(own)
		}
	}
	var fs unix.Statfs_t
	if err := unix.Statfs(root, &fs); err != nil {
		return failure(ErrNoDelegation, err)
	}
	if fs.Type != unix.CGROUP2_SUPER_MAGIC {
		return ErrNoDelegation
	}
	leaf := filepath.Join(root, helperLeaf)
	if err := os.Mkdir(leaf, 0o750); err != nil && !errors.Is(err, os.ErrExist) {
		return failure(ErrNoDelegation, err)
	}
	if err := os.WriteFile(filepath.Join(leaf, "cgroup.procs"), []byte(strconv.Itoa(os.Getpid())), 0); err != nil {
		return failure(ErrNoDelegation, err)
	}
	pl.root = root
	return nil
}

// newTree creates the cgroup s-<session id> and sets attr so the child is cloned straight into
// it (CLONE_INTO_CGROUP); a kernel that refuses that makes the start fail, because Go offers no
// way to place a child before it runs user code otherwise.
func (s *Supervisor) newTree(id string, attr *syscall.SysProcAttr) (sessionTree, error) {
	if s.plat.groupOnly {
		return groupTree{}, nil
	}
	dir := filepath.Join(s.plat.root, sessionPrefix+id)
	if err := os.Mkdir(dir, 0o750); err != nil {
		return nil, failure(ErrCgroup, err)
	}
	fd, err := unix.Open(dir, unix.O_DIRECTORY|unix.O_RDONLY|unix.O_CLOEXEC, 0)
	if err != nil {
		_ = unix.Rmdir(dir)
		return nil, failure(ErrCgroup, err)
	}
	attr.UseCgroupFD = true
	attr.CgroupFD = fd
	return &cgroupTree{dir: dir, fd: fd}, nil
}

// groupTree is the tree of a session without a cgroup: its POSIX session, process group and
// descendants in /proc, like macOS without the run file.
type groupTree struct{}

func (groupTree) started(int) error { return nil }

func (groupTree) sweep(leader int) error { return sweepGroup(snapshot, leader, leader, leader) }

func (groupTree) release() error { return nil }

// cgroupTree is the tree of a session in its own cgroup.
type cgroupTree struct {
	dir string
	fd  int
}

// started closes the descriptor of the cgroup the child was cloned into.
func (t *cgroupTree) started(int) error {
	if t.fd >= 0 {
		_ = unix.Close(t.fd)
		t.fd = -1
	}
	return nil
}

// sweep writes cgroup.kill, or SIGKILLs every PID of cgroup.procs where the kernel has no
// cgroup.kill, until no living process is left in the cgroup.
func (t *cgroupTree) sweep(int) error {
	killFile := os.WriteFile(filepath.Join(t.dir, "cgroup.kill"), []byte("1"), 0)
	deadline := time.Now().Add(sweepLimit)
	for {
		pids, err := t.living()
		if err != nil {
			return failure(ErrKill, err)
		}
		if len(pids) == 0 {
			return nil
		}
		if killFile != nil {
			for _, pid := range pids {
				_ = unix.Kill(pid, unix.SIGKILL)
			}
		}
		if time.Now().After(deadline) {
			return ErrKill
		}
		time.Sleep(sweepPause)
	}
}

// living returns the PIDs of cgroup.procs that are not zombies.
func (t *cgroupTree) living() ([]int, error) {
	raw, err := os.ReadFile(filepath.Join(t.dir, "cgroup.procs"))
	if err != nil {
		return nil, err
	}
	var pids []int
	for _, f := range strings.Fields(string(raw)) {
		pid, err := strconv.Atoi(f)
		if err != nil {
			continue
		}
		if e, err := readProc(pid); err == nil && !e.zombie {
			pids = append(pids, pid)
		}
	}
	return pids, nil
}

// release removes the cgroup once the kernel lets go of it.
func (t *cgroupTree) release() error {
	_ = t.started(0)
	deadline := time.Now().Add(sweepLimit)
	for {
		err := unix.Rmdir(t.dir)
		if err == nil || errors.Is(err, unix.ENOENT) {
			return nil
		}
		if !errors.Is(err, unix.EBUSY) || time.Now().After(deadline) {
			return failure(ErrKill, err)
		}
		time.Sleep(sweepPause)
	}
}

// readProc reads one process from /proc/<pid>/stat and the owner of /proc/<pid>.
func readProc(pid int) (procEntry, error) {
	dir := "/proc/" + strconv.Itoa(pid)
	raw, err := os.ReadFile(dir + "/stat") //nolint:gosec // a /proc path built from a PID
	if err != nil {
		return procEntry{}, err
	}
	// The command name in parentheses may hold spaces and parentheses; the fields after the
	// last ')' are state, ppid, pgrp and session.
	end := bytes.LastIndexByte(raw, ')')
	if end < 0 {
		return procEntry{}, syscall.EINVAL
	}
	fields := strings.Fields(string(raw[end+1:]))
	if len(fields) < 4 {
		return procEntry{}, syscall.EINVAL
	}
	var st unix.Stat_t
	if err := unix.Stat(dir, &st); err != nil {
		return procEntry{}, err
	}
	e := procEntry{pid: pid, uid: int(st.Uid), zombie: fields[0] == "Z" || fields[0] == "X"}
	if e.ppid, err = strconv.Atoi(fields[1]); err != nil {
		return procEntry{}, err
	}
	if e.pgid, err = strconv.Atoi(fields[2]); err != nil {
		return procEntry{}, err
	}
	if e.sid, err = strconv.Atoi(fields[3]); err != nil {
		return procEntry{}, err
	}
	return e, nil
}

// snapshot reads every process of /proc; a process that ends while it is read is left out.
func snapshot() ([]procEntry, error) {
	ents, err := os.ReadDir("/proc")
	if err != nil {
		return nil, err
	}
	out := make([]procEntry, 0, len(ents))
	for _, d := range ents {
		pid, err := strconv.Atoi(d.Name())
		if err != nil {
			continue
		}
		if e, err := readProc(pid); err == nil {
			out = append(out, e)
		}
	}
	return out, nil
}

// waitExited blocks until pid has exited without reaping it (waitid with WNOWAIT), so the PID
// stays taken until the tree is ended; without waitid it polls /proc.
func waitExited(pid int) {
	for {
		var info unix.Siginfo
		err := unix.Waitid(unix.P_PID, pid, &info, unix.WEXITED|unix.WNOWAIT, nil)
		if err == nil {
			return
		}
		if !errors.Is(err, unix.EINTR) {
			break
		}
	}
	for {
		e, err := readProc(pid)
		if err != nil || e.zombie {
			return
		}
		time.Sleep(exitPoll)
	}
}
