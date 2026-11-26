// SPDX-License-Identifier: Apache-2.0

//go:build darwin

package computer

import (
	"context"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/host/internal/confirm"
)

// childMarker as the first argument makes the test binary act as a peer that connects to the
// socket of the second argument, then execs another image.
const childMarker = "cu03-exec-child"

func init() {
	if len(os.Args) != 3 || os.Args[1] != childMarker {
		return
	}
	var d net.Dialer
	c, err := d.DialContext(context.Background(), "unix", os.Args[2])

	if err != nil {
		os.Exit(3)
	}
	f, err := c.(*net.UnixConn).File() // a duplicate without close-on-exec survives the exec
	if err != nil {
		os.Exit(3)
	}
	_, _, _ = syscall.Syscall(syscall.SYS_FCNTL, f.Fd(), syscall.F_SETFD, 0)
	_, _ = c.Write([]byte{1})
	_, _ = io.ReadFull(os.Stdin, make([]byte, 1))
	_ = syscall.Exec("/bin/sleep", []string{"sleep", "10"}, nil)
	os.Exit(4)
}

// TestDefaultBindExecAfterConnect: the real audit token binding on macOS. A peer that connects and
// then execs another image keeps its PID and start time, but the PID version of its audit token
// no longer matches: Current and ExePath fail (D-CU-03-12).
func TestDefaultBindExecAfterConnect(t *testing.T) {
	dir, err := os.MkdirTemp("", "cu")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	sock := filepath.Join(dir, "s.sock")
	l, err := (&net.ListenConfig{}).Listen(context.Background(), "unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = l.Close() }()
	self, _ := os.Executable()
	cmd := exec.CommandContext(context.Background(), self, childMarker, sock) //nolint:gosec // the test binary itself
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
	c, err := l.Accept()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close() }()
	if _, err := io.ReadFull(c, make([]byte, 1)); err != nil {
		t.Fatal(err)
	}
	peer, err := confirm.PeerOf(c)
	if err != nil || peer.PID != cmd.Process.Pid {
		t.Fatalf("peer %+v %v", peer, err)
	}
	proc, err := DefaultBind(c, peer, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := proc.Current(); err != nil {
		t.Fatalf("before the exec: %v", err)
	}
	if exe, err := proc.ExePath(); err != nil || exe != self {
		t.Fatalf("exe %q %v, want %q", exe, err, self)
	}
	_, _ = stdin.Write([]byte{1}) // the child execs /bin/sleep now, the socket stays open
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) && proc.Current() == nil {
		time.Sleep(10 * time.Millisecond)
	}
	if proc.Current() == nil {
		t.Fatal("the binding still accepts the process after an exec")
	}
	if _, err := proc.ExePath(); err == nil {
		t.Error("ExePath answers for the exec'd image")
	}
}
