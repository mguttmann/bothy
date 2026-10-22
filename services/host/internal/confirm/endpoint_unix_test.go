// SPDX-License-Identifier: Apache-2.0

//go:build linux || darwin

package confirm

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

// shortDir returns a folder short enough for a socket path (macOS allows little more than a
// hundred bytes, and t.TempDir can be longer).
func shortDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "cf")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}

func TestListenUnixSocket(t *testing.T) {
	path := filepath.Join(shortDir(t), "confirm.sock")
	// A stale socket of an earlier run is replaced.
	stale, err := (&net.ListenConfig{}).Listen(context.Background(), "unix", path)
	if err != nil {
		t.Fatal(err)
	}
	stale.(*net.UnixListener).SetUnlinkOnClose(false)
	_ = stale.Close()

	l, err := Listen(path)
	if err != nil {
		t.Fatalf("Listen over a stale socket: %v", err)
	}
	t.Cleanup(func() { _ = l.Close() })
	fi, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode()&os.ModeSocket == 0 || fi.Mode().Perm() != 0o666 {
		t.Fatalf("mode %v, want a socket with 0666", fi.Mode())
	}

	accepted := make(chan net.Conn, 1)
	go func() {
		c, err := l.Accept()
		if err == nil {
			accepted <- c
		}
	}()
	c, err := dialEndpoint(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })
	sc := waitFor(t, accepted, "the accept")
	t.Cleanup(func() { _ = sc.Close() })
	p, err := PeerOf(sc)
	if err != nil {
		t.Fatal(err)
	}
	if p.UID != strconv.Itoa(os.Getuid()) || p.PID != os.Getpid() {
		t.Fatalf("PeerOf = %+v, want uid %d pid %d", p, os.Getuid(), os.Getpid())
	}
	if _, err := PeerOf(nil); err == nil {
		t.Fatal("PeerOf(nil) without an error")
	}
}

func TestListenKeepsOtherFiles(t *testing.T) {
	path := filepath.Join(shortDir(t), "confirm.sock")
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if l, err := Listen(path); err == nil {
		_ = l.Close()
		t.Fatal("Listen replaced a regular file")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("the regular file is gone: %v", err)
	}
}

func TestListenNeedsFolder(t *testing.T) {
	path := filepath.Join(shortDir(t), "missing", "confirm.sock")
	if l, err := Listen(path); err == nil {
		_ = l.Close()
		t.Fatal("Listen created the folder")
	}
}
