// SPDX-License-Identifier: Apache-2.0

//go:build android || darwin || dragonfly || freebsd || illumos || ios || linux || netbsd || openbsd

package stack

import (
	"errors"
	"os"
	"syscall"
)

// lockTarget takes an exclusive flock(2) on the volume root and returns the function that
// releases it. It does not wait: a held lock means another run, reported as errTargetLocked. The
// lock belongs to the open directory, not to the process, so it keeps two runs in one process
// apart too, and the kernel drops it when the process ends.
func lockTarget(root *os.Root) (release func(), err error) {
	f, err := root.Open(".")
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = f.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, errTargetLocked
		}
		return nil, err
	}
	return func() { _ = f.Close() }, nil
}
