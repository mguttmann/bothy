// SPDX-License-Identifier: Apache-2.0

//go:build windows

package bothyapi

import (
	"os"
	"syscall"
	"unsafe"
)

// The kernel32 calls of the lock; the standard library does not export them.
var (
	kernel32     = syscall.NewLazyDLL("kernel32.dll")
	procLockEx   = kernel32.NewProc("LockFileEx")
	procUnlockEx = kernel32.NewProc("UnlockFileEx")
)

// lockfileExclusiveLock is LOCKFILE_EXCLUSIVE_LOCK of LockFileEx.
const lockfileExclusiveLock = 0x2

// lockFile takes the exclusive LockFileEx lock of the first byte range of f and blocks until it
// holds it.
func lockFile(f *os.File) error {
	var ol syscall.Overlapped
	r, _, err := procLockEx.Call(f.Fd(), lockfileExclusiveLock, 0, 1, 0, uintptr(unsafe.Pointer(&ol)))
	if r == 0 {
		return err
	}
	return nil
}

// unlockFile releases the lock of f.
func unlockFile(f *os.File) error {
	var ol syscall.Overlapped
	r, _, err := procUnlockEx.Call(f.Fd(), 0, 1, 0, uintptr(unsafe.Pointer(&ol)))
	if r == 0 {
		return err
	}
	return nil
}

// syncDir is a no-op: Windows has no directory fsync; the rename itself is atomic.
func syncDir(string) error { return nil }
