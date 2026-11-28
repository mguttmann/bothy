// SPDX-License-Identifier: Apache-2.0

//go:build linux || darwin

package platform

import "regexp"

// The commands the suite types into a PTY on Linux and macOS (bash and zsh, the login shells
// install gives the session users). Each regular expression needs digits or a word where the
// typed command line has a quote or a dollar sign, so the echo of the input never matches. They
// live in this file without the platform tag so make check runs them through both shells
// (typed_unix_test.go); sizeCmd needs a terminal and stays in world_unix_test.go.
const (
	enter       = "\n"
	identityCmd = `echo "UID=$(id -u)"`
	parentCmd   = `echo "PP=$PPID"`
	shellPIDCmd = `echo "SH=$$"`
)

var (
	identityRe = regexp.MustCompile(`UID=(\d+)`)
	parentRe   = regexp.MustCompile(`PP=(\d+)`)
	shellPIDRe = regexp.MustCompile(`SH=(\d+)`)
	// backgroundCmds start a nohup child and a child that left the session with setsid (perl is on
	// every hosted runner); each prints its process ID. "$!" ends the line unquoted: an
	// interactive zsh removes a "!" followed by a double quote (history expansion).
	backgroundCmds = []string{
		`nohup sleep 3000 >/dev/null 2>&1 & echo BG=$!`,
		`perl -e 'use POSIX; if (fork) { exit 0 } POSIX::setsid(); print "SID=$$\n"; exec "sleep", "3001"'; sleep 1`,
	}
	backgroundRes = []*regexp.Regexp{regexp.MustCompile(`BG=(\d+)`), regexp.MustCompile(`SID=(\d+)`)}
)
