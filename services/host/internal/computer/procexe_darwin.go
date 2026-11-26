// SPDX-License-Identifier: Apache-2.0

package computer

import (
	"bytes"
	"context"
	"errors"
	"net"
	"os/exec"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"

	"github.com/OWNER/bothy/services/host/internal/confirm"
)

// codesignBin is the absolute path of codesign; never looked up in PATH.
const codesignBin = "/usr/bin/codesign"

// codesignTimeout bounds one signature check of the helper; a protocol constant of the check, not
// a product window.
const codesignTimeout = 20 * time.Second

// Constants of the kernel interface without cgo: the option LOCAL_PEERTOKEN of SOL_LOCAL (the
// audit token of the peer at connect time, sys/un.h) and proc_info with PROC_PIDUNIQIDENTIFIERINFO
// (the PID version of a process, sys/proc_info.h).
const (
	solLocal                 = 0
	sysProcInfo              = 336 // SYS_proc_info of sys/syscall.h; libSystem has no proc_pidinfo without cgo
	procInfoCallPidinfo      = 2
	procPidUniqIdentifierInf = 17
	tokenPID                 = 5
	tokenPIDVersion          = 7
)

// Errors of the peer binding.
var (
	errNoExe     = errors.New("computer: executable path of the peer unavailable")
	errNotPeer   = errors.New("computer: the process behind the connection changed")
	errNoBinding = errors.New("computer: no audit token of the peer")
)

// uniqIdentifierInfo is struct proc_uniqidentifierinfo of sys/proc_info.h.
type uniqIdentifierInfo struct {
	UUID      [16]byte
	UniqueID  uint64
	PUniqueID uint64
	IDVersion int32
	Reserve2  uint32
	Reserve3  uint64
	Reserve4  uint64
}

// darwinProc is the peer process as the audit token of the connection names it: the PID and its
// PID version, which changes with every exec, and a new process that reuses the PID has another.
type darwinProc struct {
	pid     int
	version int32
}

// DefaultBind reads the audit token of the peer of c (LOCAL_PEERTOKEN, taken by the kernel at
// connect time) without cgo and binds the peer to its PID and PID version. Errors: no Unix socket,
// no token, or a token of another PID than the peer credentials.
func DefaultBind(c net.Conn, p confirm.Peer, _ time.Time) (Process, error) {
	uc, ok := c.(*net.UnixConn)
	if !ok {
		return nil, errNoBinding
	}
	raw, err := uc.SyscallConn()
	if err != nil {
		return nil, err
	}
	var tok [8]uint32
	var gerr error
	if err := raw.Control(func(fd uintptr) {
		n := uint32(unsafe.Sizeof(tok))
		_, _, e := syscall.Syscall6(syscall.SYS_GETSOCKOPT, fd, solLocal, unix.LOCAL_PEERTOKEN, uintptr(unsafe.Pointer(&tok[0])), uintptr(unsafe.Pointer(&n)), 0) //nolint:gosec // G103: getsockopt writes the audit token into tok
		if e != 0 || n != uint32(unsafe.Sizeof(tok)) {
			gerr = errNoBinding
		}
	}); err != nil {
		return nil, err
	}
	if gerr != nil || int(tok[tokenPID]) != p.PID {
		return nil, errNoBinding
	}
	return darwinProc{pid: p.PID, version: int32(tok[tokenPIDVersion])}, nil //nolint:gosec // G115: the PID version is a signed int of the kernel
}

// pidVersion returns the current PID version of pid (proc_info PROC_PIDUNIQIDENTIFIERINFO).
func pidVersion(pid int) (int32, error) {
	var u uniqIdentifierInfo
	n, _, e := syscall.Syscall6(sysProcInfo, procInfoCallPidinfo, uintptr(pid), procPidUniqIdentifierInf, 0, uintptr(unsafe.Pointer(&u)), unsafe.Sizeof(u)) //nolint:gosec // G103,G115: proc_info writes the struct of its size into u
	if e != 0 || n != unsafe.Sizeof(u) {
		return 0, errNotPeer
	}
	return u.IDVersion, nil
}

// Current implements Process: the PID still carries the PID version of the token.
func (d darwinProc) Current() error {
	v, err := pidVersion(d.pid)
	if err != nil || v != d.version {
		return errNotPeer
	}
	return nil
}

// ExePath implements Process: the executable from kern.procargs2, read between two checks of the
// PID version, so it is the image of the captured process.
func (d darwinProc) ExePath() (string, error) {
	if err := d.Current(); err != nil {
		return "", err
	}
	exe, err := exePath(d.pid)
	if err != nil {
		return "", err
	}
	if err := d.Current(); err != nil {
		return "", err
	}
	return exe, nil
}

// Close implements Process: the token holds nothing.
func (darwinProc) Close() error { return nil }

// exePath returns the executable path of process pid from kern.procargs2: the buffer starts with
// argc (4 bytes), followed by the executable path ending in NUL.
func exePath(pid int) (string, error) {
	b, err := unix.SysctlRaw("kern.procargs2", pid)
	if err != nil || len(b) < 5 {
		return "", errNoExe
	}
	rest := b[4:]
	end := bytes.IndexByte(rest, 0)
	if end <= 0 {
		return "", errNoExe
	}
	return string(rest[:end]), nil
}

// DefaultVerifySignature runs codesign --verify --strict -R=<requirement> on path (the designated
// requirement with the team ID that the build carries, docs/spec/27-computer-use.md 9.4). The
// caller checks the process before and after, so the path is the image of the captured process.
func DefaultVerifySignature(ctx context.Context, path, requirement string) error {
	ctx, cancel := context.WithTimeout(ctx, codesignTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, codesignBin, "--verify", "--strict", "-R="+requirement, path) //nolint:gosec // fixed program; the arguments are the installed helper path and the requirement of the build
	cmd.Env = []string{}
	return cmd.Run()
}
