// SPDX-License-Identifier: Apache-2.0

package main

import (
	"io"
	"os"
	"sync"

	"github.com/OWNER/bothy/services/host/internal/hostos"
	"github.com/OWNER/bothy/services/host/internal/state"
)

// The log file of the host service (docs/decisions.d/AUD-host-01.md). launchd drops the standard
// error of a job that names no StandardErrorPath, and a Windows service has no standard error,
// so on macOS and Windows "run" writes every log line also to state.LogFile in the state
// directory, which only the service user and administrators read. systemd keeps the lines in
// the journal, so Linux gets no file.

// logFileMax bounds the log file: a line that would grow it past the bound first moves the file
// to state.LogFileOld (replacing that one) and starts a new file, so both together stay below
// twice the bound plus one line. A bound of the diagnostic file, not a product window.
const logFileMax = 4 << 20

// logFile is an append-only file that starts anew at a size bound and keeps one predecessor.
type logFile struct {
	path, old string
	limit     int64

	mu   sync.Mutex
	f    *os.File
	size int64
}

// openLogFile opens path for appending (mode 0600, created when missing); old is the path its
// content moves to when a write would grow it past limit bytes.
func openLogFile(path, old string, limit int64) (*logFile, error) {
	l := &logFile{path: path, old: old, limit: limit}
	if err := l.open(os.O_APPEND); err != nil {
		return nil, err
	}
	return l, nil
}

func (l *logFile) open(flag int) error {
	f, err := os.OpenFile(l.path, os.O_CREATE|os.O_WRONLY|flag, 0o600)
	if err != nil {
		return err
	}
	fi, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return err
	}
	l.f, l.size = f, fi.Size()
	return nil
}

// Write appends p, one whole log line, and never splits it over two files. Errors: os.ErrClosed
// after Close or after a new file could not be opened.
func (l *logFile) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.f == nil {
		return 0, os.ErrClosed
	}
	if l.size > 0 && l.size+int64(len(p)) > l.limit {
		_ = l.f.Close()
		l.f = nil
		// When the rename fails the new file starts empty all the same, so the bound holds.
		_ = os.Rename(l.path, l.old)
		if err := l.open(os.O_TRUNC); err != nil {
			return 0, err
		}
	}
	n, err := l.f.Write(p)
	l.size += int64(n)
	return n, err
}

// Close closes the file; later writes fail.
func (l *logFile) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.f == nil {
		return nil
	}
	err := l.f.Close()
	l.f = nil
	return err
}

// both writes every line to a and to b. A failure of one of them (a Windows service has no
// standard error) does not keep the line from the other.
type both struct{ a, b io.Writer }

func (w both) Write(p []byte) (int, error) {
	_, aerr := w.a.Write(p)
	_, berr := w.b.Write(p)
	if aerr != nil && berr != nil {
		return 0, aerr
	}
	return len(p), nil
}

// serviceLog returns the writer of the log of "run" and its close: stderr, and on macOS and
// Windows also the log file in the state directory st. Errors: the file cannot be opened (the
// state directory is missing or not writable); the writer is then stderr alone.
func (a *app) serviceLog(stderr io.Writer, st *state.Dir) (io.Writer, func(), error) {
	if a.goos != hostos.Darwin && a.goos != hostos.Windows {
		return stderr, func() {}, nil
	}
	f, err := openLogFile(st.File(state.LogFile), st.File(state.LogFileOld), logFileMax)
	if err != nil {
		return stderr, func() {}, err
	}
	return both{a: stderr, b: f}, func() { _ = f.Close() }, nil
}
