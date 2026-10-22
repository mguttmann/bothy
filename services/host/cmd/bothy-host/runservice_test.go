// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"net"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/host/internal/platform/testkit"
	"github.com/OWNER/bothy/services/host/internal/state"
)

// idleListener is an admin endpoint that accepts nothing until it is closed.
type idleListener struct {
	once sync.Once
	done chan struct{}
}

func newIdleListener() *idleListener { return &idleListener{done: make(chan struct{})} }

func (l *idleListener) Accept() (net.Conn, error) {
	<-l.done
	return nil, net.ErrClosed
}

func (l *idleListener) Close() error {
	l.once.Do(func() { close(l.done) })
	return nil
}

func (*idleListener) Addr() net.Addr { return &net.UnixAddr{Name: "admin", Net: "unix"} }

// lockedBuffer is a stderr that goroutines of the service may still write while the test reads
// it (the control loop of a refused start keeps logging until the process exits).
type lockedBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (l *lockedBuffer) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

func (l *lockedBuffer) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.String()
}

// runEnded runs "run" with args in the fake system with a context that has already ended, so the
// service stops right after its start; it returns the exit code and stderr.
func runEnded(t *testing.T, args ...string) (int, string) {
	t.Helper()
	admin := newIdleListener()
	sys.listenAdmin = func(string) (net.Listener, error) { return admin, nil }
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var stdout bytes.Buffer
	stderr := &lockedBuffer{}
	code := run(ctx, append([]string{"run"}, args...), &stdout, stderr)
	return code, stderr.String()
}

// pairedFake is the fake system with the keys and a pairing record for serverURL.
func pairedFake(t *testing.T, serverURL string) *fakeSys {
	t.Helper()
	f := newFake(t)
	st := state.Open(f.state, state.Plain{})
	writeKeys(t, st)
	writePairing(t, st, serverURL)
	return f
}

// --doctor-once runs the doctor once and logs in to no server, also on a paired host.
func TestRunDoctorOnce(t *testing.T) {
	url, _ := localServer(t)
	f := pairedFake(t, url)
	code, stderr := runEnded(t, "--doctor-once", "--config", f.cfg)
	if code != 0 || strings.Contains(stderr, "login failed") {
		t.Fatalf("doctor-once: %d %q", code, stderr)
	}
}

// A paired host builds its session manager and the control client and dials the server of its
// pairing record. The fake system runs no PTY helper, so HD10 then refuses the start (78).
func TestRunPairedHost(t *testing.T) {
	url, dialed := localServer(t)
	f := pairedFake(t, url)
	code, stderr := runEnded(t, "--config", f.cfg)
	if code != exitRefused || strings.Contains(stderr, "terminal sessions unavailable") || strings.Contains(stderr, "control connection unavailable") ||
		!strings.Contains(stderr, "helper_unreachable") {
		t.Fatalf("paired host: %d %q", code, stderr)
	}
	select {
	case <-dialed:
	case <-time.After(testkit.Patience):
		t.Fatal("the control client did not dial the server of the pairing record")
	}
}

// A pairing record whose server URL the control client refuses gives no control connection: the
// host goes on with the stored declaration.
func TestRunControlUnavailable(t *testing.T) {
	f := pairedFake(t, "http://chat.example.com")
	code, stderr := runEnded(t, "--config", f.cfg)
	if code != exitRefused || !strings.Contains(stderr, "control connection unavailable") || !strings.Contains(stderr, "login failed, the stored declaration decides") {
		t.Fatalf("control unavailable: %d %q", code, stderr)
	}
}

// When the session manager cannot be built (the required confirmation endpoint is missing), the
// host logs it and goes on; every session.open is then refused.
func TestRunSessionsUnavailable(t *testing.T) {
	url, _ := localServer(t)
	f := pairedFake(t, url)
	if confirmEndpointAvailable(sys.goos) {
		t.Skip("this machine has the confirmation endpoint folder")
	}
	b, err := os.ReadFile(f.cfg)
	if err != nil {
		t.Fatal(err)
	}
	cfg := strings.Replace(string(b), "host_service:\n", "host_service:\n  require_local_confirm: true\n", 1)
	if err := os.WriteFile(f.cfg, []byte(cfg), 0o600); err != nil { //nolint:gosec // the configuration file of the fake in the test folder
		t.Fatal(err)
	}
	code, stderr := runEnded(t, "--config", f.cfg)
	if code != exitRefused || !strings.Contains(stderr, "terminal sessions unavailable") {
		t.Fatalf("sessions unavailable: %d %q", code, stderr)
	}
}
