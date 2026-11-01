// SPDX-License-Identifier: Apache-2.0

//go:build linux || darwin

package startup

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/host/internal/doctor"
	"github.com/OWNER/bothy/services/host/internal/hostos"
	"github.com/OWNER/bothy/services/host/internal/hostos/hostostest"
	"github.com/OWNER/bothy/services/host/internal/localchan"
	"github.com/OWNER/bothy/services/host/internal/platform/testkit"
)

// childArg marks the test binary started as the stand-in PTY helper of TestChildHelperProcess;
// the session user follows it.
const childArg = "child-helper"

// childUser returns the session user of the child process, "" in a normal test run.
func childUser() string {
	for i, a := range os.Args {
		if a == childArg && i+1 < len(os.Args) {
			return os.Args[i+1]
		}
	}
	return ""
}

// TestChildHelperProcess is not a test: it is the child process of TestChildHelpers. It serves the
// inherited listener (fd 3) like "bothy-host pty-helper --owner-account" and exits when its
// standard input closes.
func TestChildHelperProcess(t *testing.T) {
	user := childUser()
	if user == "" {
		t.Skip("child process of TestChildHelpers")
	}
	l, err := localchan.InheritedListener()
	if err != nil {
		os.Exit(3)
	}
	ctx, cancel := context.WithCancel(context.Background())
	go func() { _, _ = io.Copy(io.Discard, os.Stdin); cancel() }()
	_ = localchan.ServeHelper(ctx, l, strconv.Itoa(os.Getuid()), func(_ context.Context, m localchan.Message) (string, any) {
		if m.Op == localchan.OpHello {
			return localchan.OpHello, map[string]any{"version": localchan.ProtocolVersion, "session_user": user}
		}
		return localchan.OpCheckResult, doctor.HelperResult{PlaceCode: "child-" + strconv.Itoa(os.Getpid())}
	})
	os.Exit(0)
}

// TestChildHelpers: in the owner account mode (ADR 0073, D-OA-4) the PTY helper is a child of the
// service; the peer check compares the account's uid and the PID of exactly that child, the child
// is started again after it ended, and it ends when the service closes its standard input.
func TestChildHelpers(t *testing.T) {
	dir, err := os.MkdirTemp("", "oa")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(dir) }()
	me := strconv.Itoa(os.Getuid())
	w := hostostest.New("linux")
	w.AddUser(alex, me, "/home/"+alex, true)
	kids := &ChildHelpers{GOOS: "linux", Dir: filepath.Join(dir, "run"), Command: func(ctx context.Context, user string) *exec.Cmd {
		return exec.CommandContext(ctx, os.Args[0], "-test.run=^TestChildHelperProcess$", "--", childArg, user) //nolint:gosec // the test binary
	}}
	defer kids.Close()
	h := ChannelHelpers{GOOS: "linux", Users: w.Users(), Services: kids, Endpoint: kids.Endpoint}
	ctx := testkit.Context(t)
	res, err := h.Check(ctx, alex, doctor.Request{})
	if err != nil {
		t.Fatal(err)
	}
	pid, err := kids.MainPID(ctx, hostos.HelperServiceName("linux", alex))
	if err != nil || res.PlaceCode != "child-"+strconv.Itoa(pid) {
		t.Fatalf("answer %q from pid %d: %v", res.PlaceCode, pid, err)
	}
	if fi, err := os.Stat(kids.Dir); err != nil || fi.Mode().Perm() != 0o700 {
		t.Fatalf("socket folder %v %v", fi, err)
	}
	// Another session user is never served by this child.
	if _, err := kids.MainPID(ctx, hostos.HelperServiceName("linux", "bothy-s-sam")); !errors.Is(err, hostos.ErrNotRunning) {
		t.Errorf("unknown helper: %v", err)
	}
	// A peer with another PID than the child fails the peer check.
	wrong := ChannelHelpers{GOOS: "linux", Users: w.Users(), Services: fakeServices{hostos.HelperServiceName("linux", alex): pid + 1}, Endpoint: kids.Endpoint}
	if _, err := wrong.Check(ctx, alex, doctor.Request{}); !errors.Is(err, doctor.ErrPeerMismatch) {
		t.Errorf("wrong PID: %v", err)
	}
	// The child is started again after it ended.
	p, _ := os.FindProcess(pid)
	_ = p.Kill()
	deadline := testkit.Deadline()
	for {
		res, err = h.Check(ctx, alex, doctor.Request{})
		if err == nil && res.PlaceCode != "child-"+strconv.Itoa(pid) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("no new child: %q %v", res.PlaceCode, err)
		}
		time.Sleep(50 * time.Millisecond)
	}
	// Close ends the child through its standard input.
	pid2, _ := kids.MainPID(ctx, hostos.HelperServiceName("linux", alex))
	kids.Close()
	if !waitGone(pid2) {
		t.Error("the child outlived Close")
	}
}

// waitGone waits until no process pid exists any more.
func waitGone(pid int) bool {
	deadline := testkit.Deadline()
	for time.Now().Before(deadline) {
		p, err := os.FindProcess(pid)
		if err != nil || p.Signal(sigZero) != nil {
			return true
		}
		time.Sleep(50 * time.Millisecond)
	}
	return false
}

var sigZero = syscall.Signal(0)
