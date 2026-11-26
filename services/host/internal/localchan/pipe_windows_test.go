// SPDX-License-Identifier: Apache-2.0

package localchan

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

// testPipe returns a fresh pipe name and the SDDL that lets the account of the test use it.
func testPipe(t *testing.T) (string, string, string) {
	t.Helper()
	u, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	sid := u.User.Sid.String()
	return fmt.Sprintf(`\\.\pipe\bothy-localchan-test-%d-%d`, os.Getpid(), time.Now().UnixNano()), HelperPipeSDDL(sid), sid
}

// TestPipeRoundTripChecksTheClientAfterItsFirstMessage: the server checks the token of the client
// only after its first bytes arrived (Windows refuses the impersonation before), and the request
// that carried them reaches the handler whole. Before, every connection was closed unanswered.
func TestPipeRoundTripChecksTheClientAfterItsFirstMessage(t *testing.T) {
	path, sddl, sid := testPipe(t)
	l, err := ListenPipe(path, sddl)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- ServeHelper(ctx, l, sid, func(_ context.Context, m Message) (string, any) {
			return OpHello, map[string]any{"version": ProtocolVersion, "session_user": m.Op}
		})
	}()
	// Two connections one after the other: the second needs a new instance of the pipe, which
	// the server creates after the first client connected.
	for i := range 2 {
		c, err := DialHelper(ctx, path, func() (Expect, error) { return Expect{UID: sid, PID: os.Getpid()}, nil })
		if err != nil {
			t.Fatalf("dial %d: %v", i, err)
		}
		var hello struct {
			Version     int    `json:"version"`
			SessionUser string `json:"session_user"`
		}
		if err := c.Call(ctx, OpHello, struct{}{}, OpHello, &hello); err != nil || hello.Version != ProtocolVersion || hello.SessionUser != OpHello {
			t.Fatalf("call %d: %+v %v", i, hello, err)
		}
		_ = c.Close()
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("serve: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Close of the listener did not end a pending Accept")
	}
}

// TestPipeCloseEndsAPendingAccept: Close ends an Accept that waits for a client, so a helper
// service stops (D-M2-42-29).
func TestPipeCloseEndsAPendingAccept(t *testing.T) {
	path, sddl, _ := testPipe(t)
	l, err := ListenPipe(path, sddl)
	if err != nil {
		t.Fatal(err)
	}
	got := make(chan error, 1)
	go func() {
		_, err := l.Accept()
		got <- err
	}()
	time.Sleep(200 * time.Millisecond)
	_ = l.Close()
	select {
	case err := <-got:
		if !errors.Is(err, net.ErrClosed) {
			t.Errorf("Accept after Close: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Accept did not end after Close")
	}
}

// TestDialHelperWaitsForAMissingPipe: a helper that creates its pipe a moment after the host
// service dials is reached within the context.
func TestDialHelperWaitsForAMissingPipe(t *testing.T) {
	path, sddl, sid := testPipe(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	go func() {
		time.Sleep(500 * time.Millisecond)
		l, err := ListenPipe(path, sddl)
		if err != nil {
			return
		}
		_ = ServeHelper(ctx, l, sid, func(context.Context, Message) (string, any) { return OpHello, struct{}{} })
	}()
	c, err := DialHelper(ctx, path, func() (Expect, error) { return Expect{UID: sid, PID: os.Getpid()}, nil })
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	_ = c.Close()
}
