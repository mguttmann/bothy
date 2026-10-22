// SPDX-License-Identifier: Apache-2.0

// Package pty starts the terminal sessions of a PTY helper and ends each of them with its whole
// process tree (docs/spec/24-terminal.md 6.5, R-TM-15).
//
// A Supervisor starts every session in a new pseudo terminal: on Linux and macOS through
// github.com/creack/pty with its own POSIX session and controlling terminal, on Windows through a
// pseudo console (ConPTY). Every end, whatever its reason, kills the whole tree at once without a
// grace period, because every end is either the person's close or an enforcement that wants it
// immediately:
//
//   - Linux: each session runs in its own cgroup s-<session id> below the delegated cgroup of the
//     helper unit; the child is cloned straight into it (CLONE_INTO_CGROUP), so no user code runs
//     outside it. The end writes cgroup.kill, falls back to SIGKILL of every PID in cgroup.procs,
//     and removes the cgroup. The helper itself moves into the leaf "helper" first, because
//     cgroup v2 keeps processes out of inner nodes.
//   - macOS: the end kills every process of the helper's user that belongs to the session by its
//     POSIX session, its process group or a parent chain in a process table snapshot, repeating
//     snapshots until one finds none. A run file in the helper's run folder records the live
//     sessions, because launchd does not end process groups created with setsid when the helper
//     exits; the next start ends the recorded trees whose leader is still the same process.
//     Limit: a process that left the session with setsid after its parent ended is not found.
//   - Windows: each session runs in its own Job Object with JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE and
//     without JOB_OBJECT_LIMIT_BREAKAWAY_OK; the end calls TerminateJobObject and
//     ClosePseudoConsole, and the tree also ends when the helper dies, since the job handle
//     belongs to it.
//
// The end of the session leader is an end of the session too: the rest of its tree is killed
// before the leader is reaped, so no background process outlives a session and a recorded PID is
// never reused while the tree is ended.
//
// Every value of a StartSpec comes from the helper, never from a client. Errors are sentinel
// values whose text names an operation and a code; they never carry a path, a command line or an
// environment value. The package imports no other package of this module and opens no network
// listener.
package pty
