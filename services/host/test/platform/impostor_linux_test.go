// SPDX-License-Identifier: Apache-2.0

//go:build platform && linux

package platform

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// stagedImpostor is the fake helper of TM-T26 on Linux: it listens on the socket path it gets,
// answers every connection as a helper does (one byte with one end of a fresh socketpair in
// SCM_RIGHTS, 24 2.2 "Peer check of the local channel"), and writes the flag file when anything
// arrives on that end, which would be a frame sent to the fake helper.
const stagedImpostor = `import os, socket, sys
path, flag = sys.argv[1], sys.argv[2]
s = socket.socket(socket.AF_UNIX)
s.bind(path)
os.chmod(path, 0o666)
s.listen(4)
keep = []
while True:
    c, _ = s.accept()
    a, b = socket.socketpair()
    socket.send_fds(c, [b"\0"], [b.fileno()])
    b.close()
    keep.append((c, a))
    a.settimeout(15)
    try:
        if a.recv(1):
            open(flag, "w").write("frame\n")
    except OSError:
        pass
`

// stageLinuxImpostor puts a process of user on the endpoint of its helper (TM-T26) after the
// caller killed the helper. On Linux systemd owns the endpoint (bothy-host install writes a socket
// unit), so a process of the session user can neither bind the path nor keep systemd from
// starting the real helper on the next connection; the first step proves that. The suite then
// stands in for an attacker who got there anyway: as root it stops the socket unit and moves a
// socket the session user bound in a folder of its own onto the endpoint, so the next connection
// of the host service reaches a peer with the right user ID and the wrong process ID, which is
// what the peer check must refuse. The returned cleanup ends the impostor, fails the test when a
// frame reached it, and gives the endpoint back to systemd.
func stageLinuxImpostor(t *testing.T, user string) func() {
	t.Helper()
	endpoint := "/run/bothy-host/helper-" + user + ".sock"
	socketUnit := "bothy-host-helper-" + user + ".socket"
	// The unprivileged attempt: the bind must fail, the endpoint stays the one systemd created.
	attempt := impostorCmd(user)
	if err := attempt.Start(); err != nil {
		t.Fatal(err)
	}
	time.Sleep(2 * time.Second)
	_ = attempt.Process.Signal(syscall.SIGTERM)
	_ = attempt.Wait()
	if fi, err := os.Stat(endpoint); err != nil || fi.Sys().(*syscall.Stat_t).Uid != 0 {
		t.Fatalf("a process of %s took the endpoint of its helper: %v", user, err)
	}
	// The staged attacker: a socket of the session user in a folder of its own on the file
	// system of the endpoint (a rename keeps the bound socket, a copy would not).
	dir := "/run/termsuite-impostor-" + user
	sh(t, "/usr/bin/install", "-d", "-o", user, "-m", "0700", dir)
	own, flag := filepath.Join(dir, "helper.sock"), filepath.Join(dir, "frame")
	cmd := exec.CommandContext(context.WithoutCancel(context.Background()), "/usr/bin/sudo", "-n", "-u", user, "--", "python3", "-c", stagedImpostor, own, flag) //nolint:gosec // the impostor of the suite
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 100; i++ {
		if _, err := os.Stat(own); err == nil {
			break
		}
		time.Sleep(pollEvery)
	}
	sh(t, "/usr/bin/systemctl", "stop", socketUnit, "bothy-host-helper-"+user+".service")
	if err := os.Rename(own, endpoint); err != nil {
		t.Fatal(err)
	}
	return func() {
		_ = cmd.Process.Signal(syscall.SIGTERM)
		_ = cmd.Wait()
		if _, err := os.Stat(flag); err == nil {
			t.Errorf("the fake helper of %s got a frame", user)
		}
		_ = os.Remove(endpoint)
		_ = os.RemoveAll(dir)
		_, _ = try("/usr/bin/systemctl", "start", socketUnit)
	}
}
