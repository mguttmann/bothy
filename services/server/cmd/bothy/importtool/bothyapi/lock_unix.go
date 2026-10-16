// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package bothyapi

import (
	"os"
	"path/filepath"
	"syscall"
)

// lockFile takes the exclusive flock of f and blocks until it holds it. flock locks belong to
// the open file description, so two opens in one process exclude each other as two processes do.
func lockFile(f *os.File) error {
	for {
		err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX) // #nosec G115 -- a file descriptor fits into int
		if err != syscall.EINTR {
			return err
		}
	}
}

// unlockFile releases the flock of f.
func unlockFile(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_UN) // #nosec G115 -- a file descriptor fits into int
}

// syncDir makes a rename or removal in dir durable.
func syncDir(dir string) error {
	d, err := os.Open(filepath.Clean(dir))
	if err != nil {
		return err
	}
	serr := d.Sync()
	cerr := d.Close()
	if serr != nil {
		return serr
	}
	return cerr
}
