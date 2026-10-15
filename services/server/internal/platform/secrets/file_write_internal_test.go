// SPDX-License-Identifier: Apache-2.0

//go:build unix

package secrets

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// lockedDir returns a directory holding the file existing that its owner may read but not write,
// so creating, linking and removing names in it fails with EACCES. It is skipped for root, who
// may write anyway.
func lockedDir(t *testing.T) (dir, existing string) {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("root ignores the directory mode")
	}
	dir = t.TempDir()
	existing = filepath.Join(dir, "access_token-k1.pem")
	if err := os.WriteFile(existing, []byte("EXAMPLE"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o500); err != nil { // #nosec G302 -- a test directory without write permission.
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) }) // #nosec G302 -- restores the test directory for its removal.
	return dir, existing
}

// TestFileDriverWriteFailures pins that a failed system call of Put and Delete is ErrWriteFailed
// with the cause of the call as detail, and that a reference of another scheme is refused before
// any I/O.
func TestFileDriverWriteFailures(t *testing.T) {
	ctx := context.Background()
	dir, existing := lockedDir(t)
	d := newFileDriver(os.Geteuid())

	_, err := d.Put(ctx, Ref{Scheme: schemeFile, Path: filepath.Join(dir, "access_token-k2.pem")}, NewValue([]byte("EXAMPLE")))
	var re *RefError
	if !errors.Is(err, ErrWriteFailed) || ErrorCode(err) != "write_failed" || !errors.As(err, &re) || re.Detail != syscall.EACCES.Error() {
		t.Errorf("Put into a read-only directory = %v, want ErrWriteFailed with the EACCES detail", err)
	}
	err = d.Delete(ctx, Ref{Scheme: schemeFile, Path: existing})
	if !errors.Is(err, ErrWriteFailed) || ErrorCode(err) != "write_failed" {
		t.Errorf("Delete in a read-only directory = %v, want ErrWriteFailed", err)
	}
	if _, err := os.Stat(existing); err != nil {
		t.Errorf("the file is gone after a failed Delete: %v", err)
	}

	other := Ref{Scheme: "env", Path: "BOTHY_EXAMPLE"}
	if _, err := d.Put(ctx, other, NewValue([]byte("EXAMPLE"))); !errors.Is(err, ErrUnsupportedScheme) {
		t.Errorf("Put of an env reference = %v, want ErrUnsupportedScheme", err)
	}
	if err := d.Delete(ctx, other); !errors.Is(err, ErrUnsupportedScheme) {
		t.Errorf("Delete of an env reference = %v, want ErrUnsupportedScheme", err)
	}
}

// TestWriteErrorDetail pins the detail of writeError: the cause of a path or link error, and a
// fixed text for any other error, so no path or value reaches the message.
func TestWriteErrorDetail(t *testing.T) {
	cases := map[string]struct {
		err  error
		want string
	}{
		"path":  {&fs.PathError{Op: "open", Path: "/example/secret", Err: syscall.EACCES}, syscall.EACCES.Error()},
		"link":  {&os.LinkError{Op: "link", Old: "/example/a", New: "/example/b", Err: syscall.EXDEV}, syscall.EXDEV.Error()},
		"other": {errors.New("EXAMPLE /example/secret"), "write error"},
	}
	for name, c := range cases {
		re := writeError("file:/example/secret", c.err)
		if !errors.Is(re, ErrWriteFailed) || re.Detail != c.want {
			t.Errorf("%s: writeError = %+v, want ErrWriteFailed with detail %q", name, re, c.want)
		}
	}
}

// TestErrorCodeOfWriteErrors pins the three codes that only Put and Delete return.
func TestErrorCodeOfWriteErrors(t *testing.T) {
	for err, want := range map[error]string{
		&RefError{Err: ErrReadOnly}:    "read_only",
		&RefError{Err: ErrExists}:      "exists",
		&RefError{Err: ErrWriteFailed}: "write_failed",
	} {
		if got := ErrorCode(err); got != want {
			t.Errorf("ErrorCode(%v) = %q, want %q", err, got, want)
		}
	}
}
