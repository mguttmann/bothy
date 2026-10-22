// SPDX-License-Identifier: Apache-2.0

package pty

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/host/internal/platform/testkit"
)

// helperMarker as the first argument turns the test binary into a test helper process instead of
// a test run: the tests need programs that call setsid, hold a supervisor or start detached
// children, and the test binary is the only program every runner has.
const helperMarker = "bothy-pty-test-helper"

// waitLimit bounds every wait of these tests, so a broken tree end fails instead of hanging.
const waitLimit = testkit.Patience

func TestMain(m *testing.M) {
	if len(os.Args) > 2 && os.Args[1] == helperMarker {
		os.Exit(runTestHelper(os.Args[2], os.Args[3:]))
	}
	os.Exit(m.Run())
}

// testBinary returns the absolute path of the running test binary for re-executions.
func testBinary(t *testing.T) string {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("test binary: %v", err)
	}
	return exe
}

// testSessionID returns a canonical session ID that differs per call within one test run.
func testSessionID(n int) string {
	return fmt.Sprintf("5b0d1f3e-7c2a-4e8b-9a61-%012x", n)
}

// collector reads a Process until it fails and keeps everything it read.
type collector struct {
	mu   sync.Mutex
	buf  bytes.Buffer
	done chan struct{}
	err  error
}

// collect starts reading p in the background.
func collect(p io.Reader) *collector {
	c := &collector{done: make(chan struct{})}
	go func() {
		defer close(c.done)
		chunk := make([]byte, 4096)
		for {
			n, err := p.Read(chunk)
			c.mu.Lock()
			c.buf.Write(chunk[:n])
			if err != nil {
				c.err = err
			}
			c.mu.Unlock()
			if err != nil {
				return
			}
		}
	}()
	return c
}

// text returns everything read so far.
func (c *collector) text() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.buf.String()
}

// waitMatch waits until the output matches re and returns the submatches of the first match.
func (c *collector) waitMatch(t *testing.T, re *regexp.Regexp) []string {
	t.Helper()
	deadline := testkit.Deadline()
	for time.Now().Before(deadline) {
		if m := re.FindStringSubmatch(c.text()); m != nil {
			return m
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("output never matched %q; output so far: %q", re.String(), c.text())
	return nil
}

// waitEnd waits until reading failed, which happens once the PTY is closed.
func (c *collector) waitEnd(t *testing.T) {
	t.Helper()
	select {
	case <-c.done:
	case <-time.After(waitLimit):
		t.Fatal("reading the PTY never ended")
	}
}

// waitCode returns the exit code of p or fails the test when the leader does not exit in time.
func waitCode(t *testing.T, p Process) int {
	t.Helper()
	got := make(chan int, 1)
	go func() { got <- p.Wait() }()
	select {
	case code := <-got:
		return code
	case <-time.After(waitLimit):
		t.Fatal("the session leader never exited")
		return 0
	}
}

func TestStartRejectsSpec(t *testing.T) {
	s, err := New(testOptions(t))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	good := shellSpec(t, testSessionID(1), "exit 0")
	cases := map[string]func(*StartSpec){
		"session id not canonical": func(sp *StartSpec) { sp.SessionID = "../escape" },
		"session id upper case":    func(sp *StartSpec) { sp.SessionID = strings.ToUpper(sp.SessionID) },
		"relative program":         func(sp *StartSpec) { sp.Path = "sh" },
		"zero columns":             func(sp *StartSpec) { sp.Cols = 0 },
		"zero rows":                func(sp *StartSpec) { sp.Rows = 0 },
		"relative directory":       func(sp *StartSpec) { sp.Dir = "work" },
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			sp := good
			change(&sp)
			p, err := s.Start(sp)
			if !errors.Is(err, ErrSpec) {
				if p != nil {
					_ = p.Kill()
				}
				t.Fatalf("Start = %v, want ErrSpec", err)
			}
		})
	}
}

func TestStartAfterClose(t *testing.T) {
	s, err := New(testOptions(t))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if _, err := s.Start(shellSpec(t, testSessionID(2), "exit 0")); !errors.Is(err, ErrClosed) {
		t.Fatalf("Start after Close = %v, want ErrClosed", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

func TestStartFailureNamesNoPath(t *testing.T) {
	s, err := New(testOptions(t))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	sp := shellSpec(t, testSessionID(3), "exit 0")
	missing := t.TempDir() + string(os.PathSeparator) + "secret-program-name"
	sp.Path = missing
	_, err = s.Start(sp)
	if !errors.Is(err, ErrStart) {
		t.Fatalf("Start of a missing program = %v, want ErrStart", err)
	}
	if strings.Contains(err.Error(), "secret-program-name") || strings.Contains(err.Error(), t.TempDir()) {
		t.Fatalf("error text names the path: %q", err.Error())
	}
}

func TestErrorTextsNameOperationAndCode(t *testing.T) {
	for _, e := range []error{ErrNoDelegation, ErrStart, ErrSpec, ErrClosed, ErrCgroup, ErrResize, ErrKill, ErrRunFile} {
		if !regexp.MustCompile(`^pty [a-z]+: [a-z_]+$`).MatchString(e.Error()) {
			t.Errorf("error text %q does not have the form %q", e.Error(), "pty <operation>: <code>")
		}
	}
}
