// SPDX-License-Identifier: Apache-2.0

//go:build platform && windows

package platform

import (
	"os/exec"
	"strconv"
	"testing"
)

// impostorCmd creates the named pipe of the helper first as the session user.
func impostorCmd(user string) *exec.Cmd {
	return exec.Command("powershell", "-NoProfile", "-Command",
		"$p = New-Object System.IO.Pipes.NamedPipeServerStream('bothy-host-helper-"+user+"'); Start-Sleep 60")
}

// crash ends the host service with TerminateProcess, without SERVICE_STOPPED.
func crash(t *testing.T, pid int) {
	t.Helper()
	if pid == 0 {
		t.Fatal("host service not running")
	}
	sh(t, "taskkill", "/F", "/PID", strconv.Itoa(pid))
}

// stageImpostor exists only on Linux and macOS, where the service manager owns the endpoint of the
// helper (impostor_unix_test.go); startImpostor calls it only there.
func stageImpostor(t *testing.T, _ string) func() {
	t.Helper()
	t.Fatal("stageImpostor runs only on Linux and macOS")
	return func() {}
}
