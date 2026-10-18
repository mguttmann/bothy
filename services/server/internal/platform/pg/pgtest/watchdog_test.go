// SPDX-License-Identifier: Apache-2.0

package pgtest

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
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
var watchdogStubs = map[string]string{
	"docker-entrypoint.sh": "#!/bin/sh\necho start >>\"$STUB_DIR/started\"\nexec perl -e '$SIG{INT} = sub { open(my $f, \">\", \"$ENV{STUB_DIR}/stopped\"); exit 0 }; sleep 60'\n",
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
	deadline := time.Now().Add(10 * time.Second)
	for w.starts() < 1 {
		if time.Now().After(deadline) {
			t.Fatal("the stub server did not start")
		}
		time.Sleep(50 * time.Millisecond)
	}
	if err := w.cmd.Process.Signal(syscall.SIGUSR1); err != nil {
		t.Fatal(err)
	}
	deadline = time.Now().Add(15 * time.Second)
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
