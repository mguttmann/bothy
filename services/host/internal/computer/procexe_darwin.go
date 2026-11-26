// SPDX-License-Identifier: Apache-2.0

package computer

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"time"

	"golang.org/x/sys/unix"
)

// codesignBin is the absolute path of codesign; never looked up in PATH.
const codesignBin = "/usr/bin/codesign"

// codesignTimeout bounds one signature check of the helper; a protocol constant of the check, not
// a product window.
const codesignTimeout = 20 * time.Second

// errNoExe is a process whose executable path cannot be read.
var errNoExe = errors.New("computer: executable path of the peer unavailable")

// DefaultExePath returns the executable path of process pid from kern.procargs2 without cgo: the
// buffer starts with argc (4 bytes), followed by the executable path ending in NUL.
func DefaultExePath(pid int) (string, error) {
	b, err := unix.SysctlRaw("kern.procargs2", pid)
	if err != nil || len(b) < 5 {
		return "", errNoExe
	}
	rest := b[4:]
	end := bytes.IndexByte(rest, 0)
	if end <= 0 {
		return "", errNoExe
	}
	return string(rest[:end]), nil
}

// DefaultVerifySignature runs codesign --verify --strict -R=<requirement> on path (the designated
// requirement with the team ID that the build carries, docs/spec/27-computer-use.md 9.4).
func DefaultVerifySignature(ctx context.Context, path, requirement string) error {
	ctx, cancel := context.WithTimeout(ctx, codesignTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, codesignBin, "--verify", "--strict", "-R="+requirement, path) //nolint:gosec // fixed program; the arguments are the installed helper path and the requirement of the build
	cmd.Env = []string{}
	return cmd.Run()
}
