// SPDX-License-Identifier: Apache-2.0

//go:build linux || darwin

package confirm

import (
	"context"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/OWNER/bothy/services/host/internal/platform/testkit"
)

// The agent with the platform dial reaches the service over the Unix socket of Listen, and the
// service checks the peer with PeerOf: the whole local confirmation over a real socket.
func TestConfirmOverUnixSocket(t *testing.T) {
	path := filepath.Join(shortDir(t), "confirm.sock")
	l, err := Listen(path)
	if err != nil {
		t.Fatal(err)
	}
	self := strconv.Itoa(os.Getuid())
	rule := Rule{ServiceUID: uidService, Excluded: func() []string { return nil },
		ConsoleOwner: func(context.Context) (string, error) { return self, nil }}
	s := startServer(t, ServerOptions{Listener: l, Rule: rule, PeerOf: PeerOf})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	d := &fixedDialog{allow: true}
	go func() {
		done <- RunAgent(ctx, AgentOptions{Endpoint: path, Dialog: d.dialog, Log: slog.New(slog.DiscardHandler)})
	}()
	t.Cleanup(func() {
		cancel()
		if err := waitFor(t, done, "RunAgent to return"); err != nil {
			t.Errorf("RunAgent = %v after the end of ctx", err)
		}
	})
	waitAgentsOf(t, s, 1)
	if !s.Ask(context.Background(), reqOpen, testkit.Patience) {
		t.Fatal("Ask = false after an allow over the socket")
	}
	if got := d.requests(); len(got) != 1 || got[0] != reqOpen {
		t.Fatalf("dialog saw %+v, want %+v", got, reqOpen)
	}
}

func waitAgentsOf(t *testing.T, s *Server, want int) {
	t.Helper()
	h := &harness{t: t, s: s}
	h.waitAgents(want)
}

func TestDialEndpointWithoutService(t *testing.T) {
	if c, err := dialEndpoint(testkit.Context(t), filepath.Join(shortDir(t), "none.sock")); err == nil {
		_ = c.Close()
		t.Fatal("dialEndpoint reached a socket that does not exist")
	}
}

// A stale socket that cannot be removed makes Listen fail and stays as it is.
func TestListenStaleSocketNotRemovable(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root removes files in a read-only folder")
	}
	dir := shortDir(t)
	path := filepath.Join(dir, "confirm.sock")
	stale, err := (&net.ListenConfig{}).Listen(context.Background(), "unix", path)
	if err != nil {
		t.Fatal(err)
	}
	stale.(*net.UnixListener).SetUnlinkOnClose(false)
	_ = stale.Close()
	if err := os.Chmod(dir, 0o500); err != nil { //nolint:gosec // a read-only test folder, so the stale socket cannot be removed
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) }) //nolint:gosec // the test folder writable again for its removal
	if l, err := Listen(path); err == nil {
		_ = l.Close()
		t.Fatal("Listen succeeded although the stale socket could not be removed")
	}
	if fi, err := os.Lstat(path); err != nil || fi.Mode()&os.ModeSocket == 0 {
		t.Fatalf("the stale socket changed: %v %v", fi, err)
	}
}
