// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/host/internal/state"
)

// TestRunKeepsItsLogOnMacOSAndWindows: launchd drops the standard error of a job and a Windows
// service has none, so on these systems "run" writes its log lines also to bothy-host.log in the
// state directory, readable only by the owner of that directory. Before, the lines of a refused
// start were lost there.
func TestRunKeepsItsLogOnMacOSAndWindows(t *testing.T) {
	for _, goos := range []string{"darwin", "windows"} {
		f := newFake(t)
		sys.goos = goos
		code, stderr := runEnded(t, "--config", f.cfg)
		if code != exitRefused || !strings.Contains(stderr, "start refused") {
			t.Fatalf("%s: run: %d %q", goos, code, stderr)
		}
		p := filepath.Join(f.state, state.LogFile)
		b, err := os.ReadFile(p) //nolint:gosec // the log file of the fake in the test folder
		if err != nil {
			t.Fatalf("%s: %v", goos, err)
		}
		if len(b) == 0 || !strings.Contains(string(b), "start refused") || !strings.HasPrefix(stderr, string(b)) {
			t.Errorf("%s: the log file differs from standard error:\nfile: %q\nstderr: %q", goos, b, stderr)
		}
		if fi, err := os.Stat(p); err != nil || fi.Mode().Perm() != 0o600 {
			t.Errorf("%s: mode of the log file: %v %v", goos, fi, err)
		}
	}
}

// TestRunLogsToTheJournalOnLinux: systemd keeps standard error in the journal, so Linux gets no
// log file.
func TestRunLogsToTheJournalOnLinux(t *testing.T) {
	f := newFake(t)
	code, stderr := runEnded(t, "--config", f.cfg)
	if code != exitRefused || !strings.Contains(stderr, "start refused") {
		t.Fatalf("run: %d %q", code, stderr)
	}
	if _, err := os.Stat(filepath.Join(f.state, state.LogFile)); !os.IsNotExist(err) {
		t.Errorf("a log file on Linux: %v", err)
	}
}

// TestRunWithoutALogFile: when the log file cannot be opened (here the state directory is
// missing), "run" says so once and goes on with standard error alone.
func TestRunWithoutALogFile(t *testing.T) {
	f := newFake(t)
	sys.goos = "darwin"
	if err := os.Remove(f.state); err != nil {
		t.Fatal(err)
	}
	code, stderr := runEnded(t, "--config", f.cfg)
	if code != exitRefused || strings.Count(stderr, "log file unavailable") != 1 || !strings.Contains(stderr, "start refused") {
		t.Fatalf("run: %d %q", code, stderr)
	}
}

// TestLogFileStartsAnewAtItsBound: a line that would grow the file past its bound moves the
// file to its predecessor name and starts a new one; no line is split, the newest lines are
// kept in order, and both files stay within the bound.
func TestLogFileStartsAnewAtItsBound(t *testing.T) {
	dir := t.TempDir()
	path, old := filepath.Join(dir, state.LogFile), filepath.Join(dir, state.LogFileOld)
	const limit = 100
	l, err := openLogFile(path, old, limit)
	if err != nil {
		t.Fatal(err)
	}
	var all string
	for i := range 25 {
		line := fmt.Sprintf("line %02d of the log\n", i)
		all += line
		if n, err := l.Write([]byte(line)); err != nil || n != len(line) {
			t.Fatalf("write %d: %d %v", i, n, err)
		}
	}
	read := func(p string) string {
		b, err := os.ReadFile(p) //nolint:gosec // a file of the test folder
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	cur, prev := read(path), read(old)
	if len(cur) == 0 || len(cur) > limit || len(prev) == 0 || len(prev) > limit {
		t.Errorf("sizes %d and %d with the bound %d", len(cur), len(prev), limit)
	}
	if !strings.HasSuffix(all, prev+cur) || !strings.HasSuffix(cur, "line 24 of the log\n") {
		t.Errorf("the files do not hold the newest lines in order:\n%q\n%q", prev, cur)
	}
	// A new start of the service appends to the file.
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := l.Write([]byte("late\n")); !errors.Is(err, os.ErrClosed) {
		t.Errorf("write after close: %v", err)
	}
	again, err := openLogFile(path, old, limit)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = again.Close() }()
	if _, err := again.Write([]byte("next\n")); err != nil {
		t.Fatal(err)
	}
	if got := read(path); got != cur+"next\n" {
		t.Errorf("after the reopen: %q", got)
	}
	// A line longer than the bound is written whole into a file of its own.
	long := strings.Repeat("x", 2*limit) + "\n"
	if _, err := again.Write([]byte(long)); err != nil {
		t.Fatal(err)
	}
	if got := read(path); got != long {
		t.Errorf("a long line: %d bytes in the file", len(got))
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("no standard error") }

// TestBothWritesPastAFailure: a Windows service has no standard error; the line still reaches
// the other writer, and only a failure of both is an error.
func TestBothWritesPastAFailure(t *testing.T) {
	var b strings.Builder
	for _, w := range []both{{a: failingWriter{}, b: &b}, {a: &b, b: failingWriter{}}} {
		if n, err := w.Write([]byte("line\n")); err != nil || n != 5 {
			t.Errorf("write: %d %v", n, err)
		}
	}
	if b.String() != "line\nline\n" {
		t.Errorf("got %q", b.String())
	}
	if _, err := (both{a: failingWriter{}, b: failingWriter{}}).Write([]byte("x")); err == nil {
		t.Error("both writers failed without an error")
	}
}
