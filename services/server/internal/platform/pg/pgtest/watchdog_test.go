// SPDX-License-Identifier: Apache-2.0

package pgtest

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
)

// Stubs for watchdogScript outside the container. The entry point stands for PostgreSQL: it
// records each start, records a fast shutdown request (SIGINT) and ends; perl, because a background job of a
// non-interactive shell starts with SIGINT ignored and a shell cannot trap it again. psql
// answers with the file "answer" and fails while it is empty; timeout runs its command.
//
// Perl runs a signal handler only between two operations, so a SIGINT that arrives after the
// last check and before a blocking sleep starts is noticed only when the sleep ends: a busy
// host that preempts the stub right after it wrote "ready" loses the SIGINT the watchdog sends
// then. The stub therefore blocks SIGINT before it reports ready and waits in sigsuspend, which
// unblocks and waits in one step. alarm bounds the life of a stub whose watchdog died. With
// STUB_PAUSE it stops itself between "ready" and the wait (TestWatchdogStubKeepsAnINTAfterReady).
var watchdogStubs = map[string]string{
	"docker-entrypoint.sh": "#!/bin/sh\necho start >>\"$STUB_DIR/started\"\nexec perl -MPOSIX -e '$SIG{INT} = sub { open(my $f, \">\", \"$ENV{STUB_DIR}/stopped\"); exit 0 }; sigprocmask(SIG_BLOCK, POSIX::SigSet->new(SIGINT)); open(my $r, \">>\", \"$ENV{STUB_DIR}/ready\"); print $r \"ready\\n\"; close $r; POSIX::raise(SIGSTOP) if $ENV{STUB_PAUSE}; alarm 60; sigsuspend(POSIX::SigSet->new()) while 1'\n",
	"psql":                 "#!/bin/sh\n[ -s \"$STUB_DIR/answer\" ] || exit 1\ncat \"$STUB_DIR/answer\"\n",
	"timeout":              "#!/bin/sh\nshift\nexec \"$@\"\n",
}

// watchdogRun is watchdogScript running against the stubs.
type watchdogRun struct {
	t    *testing.T
	dir  string
	cmd  *exec.Cmd
	done chan struct{}
}

// writeWatchdogStubs writes the stubs into a new directory and returns it. Call it before
// t.Parallel: while another goroutine forks, a just-written executable can fail to start with
// ETXTBSY (Go issue 22315), and no parallel test forks while the serial tests run.
func writeWatchdogStubs(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range watchdogStubs {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o700); err != nil { // #nosec G306 -- stub executables
			t.Fatal(err)
		}
	}
	return dir
}

// startWatchdog starts watchdogScript with the stubs in bin, two misses, a grace of three
// seconds and a stop wait of five seconds, with psql answering first.
func startWatchdog(t *testing.T, bin, first string) *watchdogRun {
	t.Helper()
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash not found")
	}
	if _, err := exec.LookPath("perl"); err != nil {
		t.Skip("perl not found")
	}
	dir := t.TempDir()
	w := &watchdogRun{t: t, dir: dir, done: make(chan struct{})}
	w.answer(first)
	env := []string{"PATH=" + bin + ":/usr/bin:/bin", "STUB_DIR=" + dir}
	limits := watchdogEnv(2, 3*time.Second, 5*time.Second)
	keys := make([]string, 0, len(limits))
	for k := range limits {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		env = append(env, k+"="+limits[k])
	}
	// Not the test context: the cleanup below stops the script with SIGTERM, whose trap then
	// stops the stub with SIGINT.
	w.cmd = exec.CommandContext(context.Background(), bash, "-c", watchdogScript, "pgtest-watchdog", "postgres") // #nosec G204 -- the script constant
	w.cmd.Env = env
	if err := w.cmd.Start(); err != nil {
		t.Fatal(err)
	}
	go func() { _ = w.cmd.Wait(); close(w.done) }()
	t.Cleanup(func() {
		select {
		case <-w.done:
		default:
			_ = w.cmd.Process.Signal(syscall.SIGTERM)
			select {
			case <-w.done:
			case <-time.After(testkit.Patience):
				_ = w.cmd.Process.Kill()
				<-w.done
			}
		}
	})
	return w
}

// answer sets what psql prints; the empty text makes psql fail.
func (w *watchdogRun) answer(s string) {
	w.t.Helper()
	if err := os.WriteFile(filepath.Join(w.dir, "answer"), []byte(s), 0o600); err != nil {
		w.t.Fatal(err)
	}
}

// ended reports whether the script ended within limit.
func (w *watchdogRun) ended(limit time.Duration) bool {
	select {
	case <-w.done:
		return true
	case <-time.After(limit):
		return false
	}
}

// stopped reports whether the stub server got the fast shutdown request.
func (w *watchdogRun) stopped() bool {
	_, err := os.Stat(filepath.Join(w.dir, "stopped"))
	return err == nil
}

func TestWatchdogScript(t *testing.T) {
	if *childMode != "" {
		return
	}
	bin := writeWatchdogStubs(t)
	t.Parallel()
	cases := []struct {
		name  string
		first string // the answer of psql at the start
		then  string // the answer after two seconds
		ends  bool
	}{
		{name: "a held lease keeps the server", first: "t 1\n", then: "t 1\n", ends: false},
		{name: "a lost lease stops the server", first: "t 1\n", then: "t 0\n", ends: true},
		{name: "a server unreadable after the lease stops", first: "t 1\n", then: "", ends: true},
		{name: "a lease never taken stops the server", first: "f 0\n", then: "f 0\n", ends: true},
		{name: "a server never readable stops", first: "", then: "", ends: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			w := startWatchdog(t, bin, tc.first)
			if w.ended(2 * time.Second) {
				t.Fatal("the watchdog ended within two seconds")
			}
			w.answer(tc.then)
			// Two misses or the grace of three seconds, plus slack for a busy machine.
			if got := w.ended(15 * time.Second); got != tc.ends {
				t.Fatalf("watchdog ended = %v, want %v", got, tc.ends)
			}
			if !tc.ends {
				// docker stop: the trap must stop the server as well.
				if err := w.cmd.Process.Signal(syscall.SIGTERM); err != nil {
					t.Fatal(err)
				}
				if !w.ended(15 * time.Second) {
					t.Fatal("the watchdog did not end on SIGTERM")
				}
			}
			if !w.stopped() {
				t.Error("the server got no fast shutdown request")
			}
		})
	}
}

// starts returns how often the stub server started.
// readies returns how often the stub server had its SIGINT handler in place.
func (w *watchdogRun) readies() int {
	b, err := os.ReadFile(filepath.Join(w.dir, "ready"))
	if err != nil {
		return 0
	}
	return strings.Count(string(b), "ready\n")
}

func (w *watchdogRun) starts() int {
	b, err := os.ReadFile(filepath.Join(w.dir, "started"))
	if err != nil {
		return 0
	}
	return strings.Count(string(b), "start\n")
}

// TestWatchdogScriptRestartsOnUSR1: SIGUSR1 (DB.Restart) stops the server with a fast shutdown
// and starts it again, and the script, the container's main process, keeps running.
func TestWatchdogScriptRestartsOnUSR1(t *testing.T) {
	if *childMode != "" {
		return
	}
	bin := writeWatchdogStubs(t)
	t.Parallel()
	w := startWatchdog(t, bin, "t 1\n")
	deadline := testkit.Deadline()
	// Wait until the stub has its SIGINT handler: "started" is written before it is in place,
	// and an INT before that ends the stub without the fast shutdown the test checks.
	for w.readies() < 1 {
		if time.Now().After(deadline) {
			t.Fatal("the stub server did not start")
		}
		time.Sleep(50 * time.Millisecond)
	}
	if err := w.cmd.Process.Signal(syscall.SIGUSR1); err != nil {
		t.Fatal(err)
	}
	deadline = testkit.Deadline()
	for w.starts() < 2 {
		if time.Now().After(deadline) {
			t.Fatalf("the server started %d times after SIGUSR1, want 2", w.starts())
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !w.stopped() {
		t.Error("the server got no fast shutdown request before the new start")
	}
	if w.ended(3 * time.Second) {
		t.Fatal("the watchdog ended after the restart")
	}
}

// TestWatchdogStubKeepsAnINTAfterReady: a SIGINT that reaches the stub server after it reported
// ready and before it waits for the signal still ends it with the fast shutdown request. With
// STUB_PAUSE the stub stops itself (SIGSTOP) in exactly that gap, so the test sends SIGINT there
// deterministically, as a busy host does by chance when it preempts the stub between writing
// "ready" and starting to wait while the watchdog sends SIGINT.
func TestWatchdogStubKeepsAnINTAfterReady(t *testing.T) {
	if *childMode != "" {
		return
	}
	bin := writeWatchdogStubs(t)
	t.Parallel()
	if _, err := exec.LookPath("perl"); err != nil {
		t.Skip("perl not found")
	}
	dir := t.TempDir()
	cmd := exec.CommandContext(context.Background(), filepath.Join(bin, "docker-entrypoint.sh")) // #nosec G204 -- the stub
	cmd.Env = []string{"PATH=" + bin + ":/usr/bin:/bin", "STUB_DIR=" + dir, "STUB_PAUSE=1"}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() { _ = cmd.Wait(); close(done) }()
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		<-done
	})
	testkit.Until(t, "the stub server stopping itself after ready", func() bool { return processStopped(t, cmd.Process.Pid) })
	if err := cmd.Process.Signal(syscall.SIGINT); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Process.Signal(syscall.SIGCONT); err != nil {
		t.Fatal(err)
	}
	// The stub ends by itself after 60 seconds; a SIGINT it only notices then is lost for the
	// watchdog, which waits its stop wait (5 seconds here, 30 in the container) and starts the
	// next server. Half the stub's lifetime separates the two outcomes with room for a busy host.
	select {
	case <-done:
	case <-time.After(30 * time.Second): // nosemgrep: bothy-go-test-wall-clock-wait -- a latency bound under test: a lost SIGINT only ends the stub after its 60-second alarm
		t.Fatal("the stub server did not act on SIGINT")
	}
	if _, err := os.Stat(filepath.Join(dir, "stopped")); err != nil {
		t.Fatal("the stub server ended without the fast shutdown request")
	}
}

// processStopped reports whether the process pid is stopped (state T in ps).
func processStopped(t *testing.T, pid int) bool {
	t.Helper()
	out, err := exec.CommandContext(context.Background(), "ps", "-o", "stat=", "-p", strconv.Itoa(pid)).Output() // #nosec G204 -- a pid
	if err != nil {
		return false
	}
	return bytes.HasPrefix(bytes.TrimSpace(out), []byte("T"))
}
