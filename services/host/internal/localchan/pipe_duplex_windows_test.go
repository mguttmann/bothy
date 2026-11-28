// SPDX-License-Identifier: Apache-2.0

package localchan

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"testing"
	"time"

	"golang.org/x/sys/windows"

	"github.com/OWNER/bothy/services/host/internal/platform/testkit"
)

// TestPipeWritesWhileAReadIsPending: each end of a helper pipe writes while a read of its own is
// still waiting, as the session channel of a helper does (it reads the next message while a
// goroutine answers hello). On a synchronous handle Windows queues the write behind the pending
// read, so the answer never left and the host service saw helper_unreachable after its bound
// (V1-13/9, run 46122005337).
func TestPipeWritesWhileAReadIsPending(t *testing.T) {
	u, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	path := fmt.Sprintf(`\\.\pipe\bothy-localchan-duplex-test-%d`, os.Getpid())
	l, err := ListenPipe(path, HelperPipeSDDL(u.User.Sid.String()))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = l.Close() }()
	accepted := make(chan net.Conn, 1)
	go func() {
		c, err := l.Accept()
		if err != nil {
			t.Error(err)
		}
		accepted <- c
	}()
	client, err := dialPipe(testkit.Context(t), path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = client.Close() }()
	server := <-accepted
	if server == nil {
		t.FailNow()
	}
	defer func() { _ = server.Close() }()
	for _, end := range []struct {
		name          string
		writer, other net.Conn
	}{{"server", server, client}, {"client", client, server}} {
		pending := make(chan []byte, 1)
		go func() {
			b := make([]byte, 1)
			if _, err := io.ReadFull(end.writer, b); err != nil {
				b = nil
			}
			pending <- b
		}()
		time.Sleep(200 * time.Millisecond) // the read of the writing end is waiting
		wrote := make(chan error, 1)
		go func() { _, err := end.writer.Write([]byte("w")); wrote <- err }()
		select {
		case err := <-wrote:
			if err != nil {
				t.Fatalf("%s write: %v", end.name, err)
			}
		case <-time.After(testkit.Patience):
			t.Fatalf("%s: the write waited behind its own pending read", end.name)
		}
		got := make([]byte, 1)
		if _, err := io.ReadFull(end.other, got); err != nil || string(got) != "w" {
			t.Fatalf("%s: the other end read %q, %v", end.name, got, err)
		}
		// Release the pending read of the writing end.
		if _, err := end.other.Write([]byte("r")); err != nil {
			t.Fatal(err)
		}
		if b := <-pending; string(b) != "r" {
			t.Fatalf("%s: pending read got %q", end.name, b)
		}
	}
}

// TestPipeCloseEndsAccept: Close of a pipe listener ends an Accept that waits for a client
// (D-M2-42-29: a synchronous ConnectNamedPipe never returned).
func TestPipeCloseEndsAccept(t *testing.T) {
	u, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	path := fmt.Sprintf(`\\.\pipe\bothy-localchan-close-test-%d`, os.Getpid())
	l, err := ListenPipe(path, HelperPipeSDDL(u.User.Sid.String()))
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { _, err := l.Accept(); done <- err }()
	time.Sleep(200 * time.Millisecond) // Accept waits for a client
	_ = l.Close()
	select {
	case err := <-done:
		if !errors.Is(err, net.ErrClosed) {
			t.Fatalf("Accept after Close: %v, want net.ErrClosed", err)
		}
	case <-time.After(testkit.Patience):
		t.Fatal("Close did not end the waiting Accept")
	}
}
