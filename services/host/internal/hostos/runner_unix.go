// SPDX-License-Identifier: Apache-2.0

//go:build linux || darwin

package hostos

import (
	"os/exec"
	"syscall"
)

// ownGroup starts the program in its own process group and ends the whole group on cancel, so a
// background job of a login shell ends with it.
func ownGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}
