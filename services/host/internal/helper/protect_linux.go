// SPDX-License-Identifier: Apache-2.0

package helper

import "golang.org/x/sys/unix"

// Protect sets PR_SET_DUMPABLE to 0, so no other process of the session user can attach to the
// helper with ptrace (24 2.2 PTY helper point 4).
// The argument (the account of the host service) is used on Windows only.
func Protect(string) error { return unix.Prctl(unix.PR_SET_DUMPABLE, 0, 0, 0, 0) }
