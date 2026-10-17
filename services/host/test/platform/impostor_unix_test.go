// SPDX-License-Identifier: Apache-2.0

//go:build platform && (linux || darwin)

package platform

import (
	"os/exec"
	"runtime"
	"syscall"
	"testing"
)

// impostorCmd listens on the endpoint of the helper as the session user (Python is on every
// hosted runner). On Linux and macOS the service manager owns the socket, so the bind fails or
// gets no client; either way the host service must send nothing.
func impostorCmd(user string) *exec.Cmd {
	sock := "/run/bothy-host/helper-" + user + ".sock"
	if runtime.GOOS == "darwin" {
		sock = "/Library/Application Support/bothy-host/run/helper-" + user + ".sock"
	}
	return exec.Command("/usr/bin/sudo", "-n", "-u", user, "--", "python3", "-c",
		"import socket,os,time\ns=socket.socket(socket.AF_UNIX)\ntry:\n os.unlink('"+sock+"')\nexcept Exception: pass\ntry:\n s.bind('"+sock+"'); s.listen(1)\nexcept Exception: pass\ntime.sleep(60)")
}

// crash ends the host service like a crash (SIGSEGV).
func crash(t *testing.T, pid int) {
	t.Helper()
	if pid == 0 {
		t.Fatal("host service not running")
	}
	if err := syscall.Kill(pid, syscall.SIGSEGV); err != nil {
		t.Fatal(err)
	}
}
