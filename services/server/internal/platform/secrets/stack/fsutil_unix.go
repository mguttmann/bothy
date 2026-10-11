// SPDX-License-Identifier: Apache-2.0

//go:build unix

package stack

import (
	"io/fs"
	"os"
	"syscall"
)

// readFlags opens files without blocking, so a pipe swapped in after the scan cannot stall init
// (opening a FIFO for reading blocks until a writer appears).
const readFlags = syscall.O_NONBLOCK

// ownerOf returns uid and gid of a scanned entry.
func ownerOf(info fs.FileInfo) (Owner, bool) {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return Owner{}, false
	}
	return Owner{UID: int(st.Uid), GID: int(st.Gid)}, true
}

// runningUser returns the effective uid and gid, which own _store and, when not root, every copy.
func runningUser() (Owner, error) {
	return Owner{UID: os.Geteuid(), GID: os.Getegid()}, nil
}

// syncDir makes the renames and removals in a directory durable before init reports success.
func syncDir(root *os.Root, dir string) error {
	f, err := root.Open(dir)
	if err != nil {
		return err
	}
	err = f.Sync()
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	return err
}
