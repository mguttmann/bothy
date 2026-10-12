// SPDX-License-Identifier: Apache-2.0

//go:build unix

package secrets

import (
	"fmt"
	"io/fs"
	"os"
	"syscall"
)

// openFlags opens without blocking: should a FIFO replace the checked file between the check and
// the open, the open returns at once and the second check refuses it, instead of waiting for a
// writer. Regular files ignore O_NONBLOCK.
const openFlags = os.O_RDONLY | syscall.O_NONBLOCK

// platformProblem is empty: unix has file ownership, so the driver can enforce SEC-SEC-1.
const platformProblem = ""

// ownerProblem returns why the owner of fi is not uid, or "" when it is.
func ownerProblem(fi fs.FileInfo, uid int) string {
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return "file ownership cannot be checked"
	}
	if int64(st.Uid) != int64(uid) {
		return fmt.Sprintf("owner uid %d, want uid %d", st.Uid, uid)
	}
	return ""
}
