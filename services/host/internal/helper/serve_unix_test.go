// SPDX-License-Identifier: Apache-2.0

//go:build linux || darwin

package helper

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/OWNER/bothy/services/host/internal/localchan"
	"github.com/OWNER/bothy/services/host/internal/platform/testkit"
)

// serveOnSocket runs Serve with o on a fresh socket until the test ends and returns a verified
// channel of the service to it.
func serveOnSocket(t *testing.T, o Options) *localchan.Conn {
	t.Helper()
	bound := testkit.Context(t)
	d, err := os.MkdirTemp("", "hs")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(d) })
	sock := filepath.Join(d, "h.sock")
	l, err := (&net.ListenConfig{}).Listen(bound, "unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	me := strconv.Itoa(os.Getuid())
	o.Listener, o.ServiceUID = l, me
	ctx, cancel := context.WithCancel(bound)
	done := make(chan error, 1)
	go func() { done <- Serve(ctx, o) }()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("Serve: %v", err)
			}
		case <-bound.Done():
			t.Error("Serve did not return")
		}
	})
	c, err := localchan.DialHelper(bound, sock, func() (localchan.Expect, error) {
		return localchan.Expect{UID: me, PID: os.Getpid()}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}

// TestServeSessionChannel (M2-45): with a SessionEnv every channel is a session channel, which
// takes a kind 0x01 frame (here of no session, so it is dropped) and keeps answering the checks;
// without it the channel serves the checks only and a frame closes it.
func TestServeSessionChannel(t *testing.T) {
	id := [localchan.SessionIDLen]byte{1}
	c := serveOnSocket(t, Options{Env: world(t), Session: &SessionEnv{Check: world(t)}})
	if err := c.SendFrame(id, []byte{0}); err != nil {
		t.Fatal(err)
	}
	var hello map[string]any
	if err := c.Call(testkit.Context(t), localchan.OpHello, nil, localchan.OpHello, &hello); err != nil ||
		hello["session_user"] != "bothy-s-alex" || hello["version"] != float64(localchan.ProtocolVersion) {
		t.Fatalf("hello on a session channel after a frame: %v %v", hello, err)
	}

	plain := serveOnSocket(t, Options{Env: world(t)})
	if err := plain.SendFrame(id, []byte{0}); err != nil {
		t.Fatal(err)
	}
	if err := plain.Call(testkit.Context(t), localchan.OpHello, nil, localchan.OpHello, &hello); err == nil || errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("a channel without sessions answered after a frame: %v", err)
	}
}
