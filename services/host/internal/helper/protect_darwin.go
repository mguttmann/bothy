// SPDX-License-Identifier: Apache-2.0

package helper

// Protect is a no-op on macOS: the hardened runtime without get-task-allow, signed in by install
// and checked by HD10, keeps task_for_pid of other processes of the user out.
// The argument (the account of the host service) is used on Windows only.
func Protect(string) error { return nil }
