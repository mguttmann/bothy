// SPDX-License-Identifier: Apache-2.0

//go:build linux || darwin

package localchan

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"testing"
	"time"
)

// shortDir returns a temporary folder with a path short enough for a Unix socket.
func shortDir(t *testing.T) string {
	t.Helper()
	d, err := os.MkdirTemp("", "lc")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(d) })
	return d
}

var me = strconv.Itoa(os.Getuid())

func echo(_ context.Context, m Message) (string, any) {
	if m.Op == "boom" {
		return ErrorAnswer("boom_code")
	}
	return m.Op + ".ok", map[string]string{"seen": m.Op}
}

func startHelper(t *testing.T, allow string, h Handler) string {
	t.Helper()
	sock := filepath.Join(shortDir(t), "h.sock")
	l, err := (&net.ListenConfig{}).Listen(context.Background(), "unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { _ = ServeHelper(ctx, l, allow, h); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	return sock
}

func TestPeerCheckPasses(t *testing.T) {
	sock := startHelper(t, me, echo)
	c, err := DialHelper(context.Background(), sock, func() (Expect, error) { return Expect{UID: me, PID: os.Getpid()}, nil })
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close() }()
	var out map[string]string
	if err := c.Call(context.Background(), "ping", nil, "ping.ok", &out); err != nil || out["seen"] != "ping" {
		t.Fatalf("%v %v", out, err)
	}
	var re *RemoteError
	if err := c.Call(context.Background(), "boom", nil, "boom.ok", nil); !errors.As(err, &re) || re.Code != "boom_code" {
		t.Errorf("remote error %v", err)
	}
	if err := c.Call(context.Background(), "ping", nil, "other", nil); !errors.Is(err, ErrMalformed) {
		t.Errorf("wrong answer op: %v", err)
	}
}

func TestPeerCheckWrongUser(t *testing.T) {
	sock := startHelper(t, me, echo)
	if _, err := DialHelper(context.Background(), sock, func() (Expect, error) { return Expect{UID: "99999", PID: os.Getpid()}, nil }); !errors.Is(err, ErrPeerMismatch) {
		t.Errorf("%v", err)
	}
}

func TestPeerCheckExpectationUnknown(t *testing.T) {
	sock := startHelper(t, me, echo)
	if _, err := DialHelper(context.Background(), sock, func() (Expect, error) { return Expect{}, errors.New("no main PID") }); !errors.Is(err, ErrPeerMismatch) {
		t.Errorf("%v", err)
	}
}

func TestPeerCheckWrongPID(t *testing.T) {
	sock := startHelper(t, me, echo)
	if _, err := DialHelper(context.Background(), sock, func() (Expect, error) { return Expect{UID: me, PID: os.Getpid() + 1}, nil }); !errors.Is(err, ErrPeerMismatch) {
		t.Errorf("%v", err)
	}
}

// TestNoMessageBeforePeerCheck: a replaced helper that hands over its socketpair gets no byte when
// the peer check fails.
func TestNoMessageBeforePeerCheck(t *testing.T) {
	sock := filepath.Join(shortDir(t), "fake.sock")
	l, err := (&net.ListenConfig{}).Listen(context.Background(), "unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = l.Close() }()
	var got atomic.Int64
	done := make(chan struct{})
	go func() {
		defer close(done)
		c, err := l.Accept()
		if err != nil {
			return
		}
		pair, err := handoff(c, me)
		if err != nil {
			t.Error(err)
			return
		}
		_ = pair.SetReadDeadline(time.Now().Add(2 * time.Second))
		b := make([]byte, 64)
		n, _ := pair.Read(b)
		got.Add(int64(n))
		_ = pair.Close()
	}()
	if _, err := DialHelper(context.Background(), sock, func() (Expect, error) { return Expect{UID: me, PID: os.Getpid() + 7}, nil }); !errors.Is(err, ErrPeerMismatch) {
		t.Fatalf("%v", err)
	}
	<-done
	if got.Load() != 0 {
		t.Errorf("the fake helper received %d bytes", got.Load())
	}
}

func TestHelperRejectsOtherClient(t *testing.T) {
	sock := startHelper(t, "99999", echo)
	if _, err := DialHelper(context.Background(), sock, func() (Expect, error) { return Expect{UID: me, PID: os.Getpid()}, nil }); err == nil {
		t.Error("a client of another user got a channel")
	}
	// A listener without a handoff (a process that took over the endpoint) gives no channel.
	plain := filepath.Join(shortDir(t), "p.sock")
	l, err := (&net.ListenConfig{}).Listen(context.Background(), "unix", plain)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = l.Close() }()
	go func() {
		c, err := l.Accept()
		if err == nil {
			_, _ = c.Write([]byte{0})
			_ = c.Close()
		}
	}()
	if _, err := DialHelper(context.Background(), plain, func() (Expect, error) { return Expect{UID: me, PID: os.Getpid()}, nil }); err == nil {
		t.Error("handoff without a descriptor accepted")
	}
	if _, err := DialHelper(context.Background(), filepath.Join(shortDir(t), "none.sock"), func() (Expect, error) { return Expect{}, nil }); err == nil {
		t.Error("missing endpoint")
	}
}

func TestAdminEndpoint(t *testing.T) {
	path := filepath.Join(shortDir(t), "admin.sock")
	l, err := ListenAdmin(path)
	if err != nil {
		t.Fatal(err)
	}
	fi, _ := os.Stat(path)
	if fi.Mode().Perm() != 0o600 {
		t.Errorf("mode %v", fi.Mode().Perm())
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { _ = ServeAdmin(ctx, l, me, echo); close(done) }()
	defer func() { cancel(); <-done }()
	c, err := DialAdmin(context.Background(), path, me)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Call(context.Background(), OpDoctorRun, nil, OpDoctorRun+".ok", nil); err != nil {
		t.Error(err)
	}
	_ = c.Close()
	if _, err := DialAdmin(context.Background(), path, "99999"); !errors.Is(err, ErrPeerMismatch) {
		t.Errorf("foreign server accepted: %v", err)
	}
	// A server for another uid answers nobody.
	path2 := filepath.Join(shortDir(t), "admin2.sock")
	l2, err := ListenAdmin(path2)
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = ServeAdmin(ctx, l2, "99999", echo) }()
	// The server closes the connection at once; the client fails either at the peer check of the
	// closed socket or at the call.
	if c2, err := DialAdmin(context.Background(), path2, me); err == nil {
		cctx, ccancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer ccancel()
		if err := c2.Call(cctx, OpDoctorRun, nil, OpDoctorRun+".ok", nil); err == nil {
			t.Error("client of another uid answered")
		}
	}
	if _, err := ListenAdmin(filepath.Join(shortDir(t), "no", "dir.sock")); err == nil {
		t.Error("listen in a missing folder")
	}
}

func TestActivatedListenerAbsent(t *testing.T) {
	if _, err := ActivatedListener("linux", "1", "1"); err == nil {
		t.Error("foreign LISTEN_PID accepted")
	}
	if _, err := receivedFD(nil); err == nil {
		t.Error("empty control message")
	}
}

func TestCallHonoursContext(t *testing.T) {
	sock := startHelper(t, me, func(ctx context.Context, m Message) (string, any) {
		time.Sleep(time.Second)
		return "late", nil
	})
	c, err := DialHelper(context.Background(), sock, func() (Expect, error) { return Expect{UID: me, PID: os.Getpid()}, nil })
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if err := c.Call(ctx, "x", nil, "late", nil); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("%v", err)
	}
}
