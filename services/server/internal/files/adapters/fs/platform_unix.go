// SPDX-License-Identifier: Apache-2.0

//go:build unix

package fs

import (
	"errors"
	iofs "io/fs"
	"syscall"
)

// Open flags of the Unix kernel. O_NOFOLLOW protects only the creation of temporary files
// (O_CREATE|O_EXCL never follows a link, also in os.Root); on every other open os.Root resolves a
// link inside the root itself, flag or not, so the driver relies on its Lstat checks (inspect)
// and on the identity check after a read open (openRegular). O_NONBLOCK keeps an open of a FIFO
// that replaced an object from blocking; on a regular file it has no effect.
const (
	oNoFollow = syscall.O_NOFOLLOW
	oNonBlock = syscall.O_NONBLOCK
)

// fileOwner returns the user ID that owns the file of fi.
func fileOwner(fi iofs.FileInfo) (int, bool) {
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, false
	}
	return int(st.Uid), true
}

// syncUnsupported reports the one error of a directory sync that counts as success: the
// platform does not support syncing a directory at all.
func syncUnsupported(err error) bool {
	return errors.Is(err, syscall.ENOTSUP) || errors.Is(err, syscall.EOPNOTSUPP) || errors.Is(err, errors.ErrUnsupported)
}

// notDir reports that a component of a path is not a directory, so the path names nothing.
func notDir(err error) bool { return errors.Is(err, syscall.ENOTDIR) }

// notEmpty reports the error of removing a directory that still has entries (Linux ENOTEMPTY,
// some systems EEXIST).
func notEmpty(err error) bool {
	return errors.Is(err, syscall.ENOTEMPTY) || errors.Is(err, syscall.EEXIST)
}

// renameRaced reports the EINVAL of macOS renameat when a concurrent rmdir removes the target
// directory during the call.
func renameRaced(err error) bool { return errors.Is(err, syscall.EINVAL) }
