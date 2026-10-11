// SPDX-License-Identifier: Apache-2.0

//go:build unix

package secrets_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/platform/secrets"
)

// modeBits are the bits the file driver checks: permissions plus setuid, setgid and sticky.
const modeBits = os.ModePerm | os.ModeSetuid | os.ModeSetgid | os.ModeSticky

// writeSecret writes content to dir/name with exactly mode and returns its file reference.
func writeSecret(t *testing.T, dir, name string, content []byte, mode os.FileMode) secrets.Ref {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, content, 0o600); err != nil {
		t.Fatal(err)
	}
	// Chmod after the write: the umask must not change the mode under test.
	if err := os.Chmod(p, mode); err != nil {
		t.Fatal(err)
	}
	return secrets.Ref{Scheme: "file", Path: p}
}

// replaceSecret swaps the file of ref for one with new content and mode 0600 the way a rotation
// does it: a new file renamed over the old one.
func replaceSecret(t *testing.T, ref secrets.Ref, content string) {
	t.Helper()
	tmp := ref.Path + ".new"
	if err := os.WriteFile(tmp, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(tmp, ref.Path); err != nil {
		t.Fatal(err)
	}
}

// assertInsecure checks that d refuses ref with ErrInsecureFile, a message that names the
// reference and contains want, and no trace of the canary.
func assertInsecure(t *testing.T, d secrets.Driver, ref secrets.Ref, want, canary string) {
	t.Helper()
	v, ver, err := d.Get(context.Background(), ref)
	checkInsecure(t, ref, v, ver, err, want, canary)
}

// checkInsecure is assertInsecure for a result the test obtained itself.
func checkInsecure(t *testing.T, ref secrets.Ref, v secrets.Value, ver secrets.Version, err error, want, canary string) {
	t.Helper()
	if !errors.Is(err, secrets.ErrInsecureFile) {
		t.Fatalf("Get error = %v, want ErrInsecureFile", err)
	}
	if code := secrets.ErrorCode(err); code != "insecure_file" {
		t.Fatalf("ErrorCode = %q, want insecure_file", code)
	}
	msg := err.Error()
	if !strings.Contains(msg, ref.String()) || !strings.Contains(msg, want) {
		t.Fatalf("message %q lacks the reference %q or %q", msg, ref.String(), want)
	}
	assertNoSecret(t, "error", msg, canary)
	if v.Len() != 0 || ver != "" {
		t.Fatalf("Get returned a value of length %d and version %q with the error", v.Len(), ver)
	}
}

func TestFileDriverRejectsInsecureModes(t *testing.T) {
	t.Parallel()
	canary := newCanary(t)
	for _, tc := range []struct {
		mode os.FileMode
		want string
	}{
		{0o644, "mode 0644, want 0400 or 0600"},
		{0o640, "mode 0640"},
		{0o444, "mode 0444"},
		{0o700, "mode 0700"},
		{0o601, "mode 0601"},
		{0o604, "mode 0604"},
		{0o200, "mode 0200"},
		{os.ModeSetuid | 0o600, "mode 04600"},
		{os.ModeSetgid | 0o400, "mode 02400"},
		{os.ModeSticky | 0o600, "mode 01600"},
	} {
		t.Run(tc.want, func(t *testing.T) {
			t.Parallel()
			ref := writeSecret(t, t.TempDir(), "secret", []byte(canary), 0o600)
			// Some systems refuse special bits on regular files for non-root users (macOS:
			// sticky); then there is no such file to refuse.
			if err := os.Chmod(ref.Path, tc.mode); err != nil {
				t.Skipf("chmod %v refused: %v", tc.mode, err)
			}
			fi, err := os.Stat(ref.Path)
			if err != nil {
				t.Fatal(err)
			}
			if fi.Mode()&modeBits != tc.mode {
				t.Skipf("the file system did not keep mode %v (got %v); nothing to check", tc.mode, fi.Mode()&modeBits)
			}
			assertInsecure(t, secrets.NewFileDriver(), ref, tc.want, canary)
		})
	}

	t.Run("directory", func(t *testing.T) {
		t.Parallel()
		dir := filepath.Join(t.TempDir(), "secret")
		if err := os.Mkdir(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		assertInsecure(t, secrets.NewFileDriver(), secrets.Ref{Scheme: "file", Path: dir}, "not a regular file", canary)
	})

	t.Run("fifo", func(t *testing.T) {
		t.Parallel()
		p := filepath.Join(t.TempDir(), "secret")
		if err := syscall.Mkfifo(p, 0o600); err != nil {
			t.Fatal(err)
		}
		// Opening a FIFO for reading blocks until a writer appears; the driver must refuse it
		// before it opens anything.
		type result struct {
			v   secrets.Value
			ver secrets.Version
			err error
		}
		ref := secrets.Ref{Scheme: "file", Path: p}
		done := make(chan result, 1)
		go func() {
			v, ver, err := secrets.NewFileDriver().Get(context.Background(), ref)
			done <- result{v, ver, err}
		}()
		select {
		case r := <-done:
			checkInsecure(t, ref, r.v, r.ver, r.err, "not a regular file", canary)
		case <-time.After(10 * time.Second):
			t.Fatal("Get blocked on a FIFO")
		}
	})

	t.Run("foreign owner simulated", func(t *testing.T) {
		t.Parallel()
		ref := writeSecret(t, t.TempDir(), "secret", []byte(canary), 0o600)
		uid := os.Geteuid()
		want := fmt.Sprintf("owner uid %d, want uid %d", uid, uid+1)
		assertInsecure(t, secrets.NewFileDriverForUID(uid+1), ref, want, canary)
	})

	t.Run("foreign owner real", func(t *testing.T) {
		t.Parallel()
		if os.Geteuid() != 0 {
			t.Skip("needs root: only root can give a file to another uid; the simulated subtest covers the rule")
		}
		ref := writeSecret(t, t.TempDir(), "secret", []byte(canary), 0o600)
		if err := os.Chown(ref.Path, 65532, 65532); err != nil {
			t.Fatal(err)
		}
		assertInsecure(t, secrets.NewFileDriver(), ref, "owner uid 65532, want uid 0", canary)
	})
}

func TestFileDriverAccepts0400And0600(t *testing.T) {
	t.Parallel()
	for _, mode := range []os.FileMode{0o400, 0o600} {
		t.Run(mode.String(), func(t *testing.T) {
			t.Parallel()
			ref := writeSecret(t, t.TempDir(), "secret", []byte("EXAMPLE\n"), mode)
			d := secrets.NewFileDriver()
			if d.Scheme() != "file" {
				t.Fatalf("Scheme = %q, want file", d.Scheme())
			}
			v, ver, err := d.Get(context.Background(), ref)
			if err != nil {
				t.Fatalf("Get: %v", err)
			}
			if got := string(v.Bytes()); got != "EXAMPLE" {
				t.Fatalf("value = %q, want EXAMPLE", got)
			}
			// The version is the SHA-256 prefix of the value after the newline rule, the same
			// as for "EXAMPLE" from the environment.
			if ver != "f2e8378c30d317ab" {
				t.Fatalf("version = %q, want f2e8378c30d317ab", ver)
			}
		})
	}
}

func TestFileDriverStripsOneTrailingNewline(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		content, want string
	}{
		{"EXAMPLE-v\n", "EXAMPLE-v"},
		{"EXAMPLE-v\r\n", "EXAMPLE-v"},
		{"EXAMPLE-v\n\n", "EXAMPLE-v\n"},
		{"EXAMPLE-v\r\n\r\n", "EXAMPLE-v\r\n"},
		{"EXAMPLE-v", "EXAMPLE-v"},
		{"EXAMPLE-v\r", "EXAMPLE-v\r"},
		{"EXAMPLE-v\n\r", "EXAMPLE-v\n\r"},
		{"EXAMPLE\nv\n", "EXAMPLE\nv"},
	} {
		t.Run(fmt.Sprintf("%q", tc.content), func(t *testing.T) {
			t.Parallel()
			ref := writeSecret(t, t.TempDir(), "secret", []byte(tc.content), 0o600)
			v, _, err := secrets.NewFileDriver().Get(context.Background(), ref)
			if err != nil {
				t.Fatalf("Get: %v", err)
			}
			if got := string(v.Bytes()); got != tc.want {
				t.Fatalf("value = %q, want %q", got, tc.want)
			}
		})
	}
	for _, content := range []string{"\n", "\r\n"} {
		t.Run(fmt.Sprintf("%q alone", content), func(t *testing.T) {
			t.Parallel()
			ref := writeSecret(t, t.TempDir(), "secret", []byte(content), 0o600)
			_, _, err := secrets.NewFileDriver().Get(context.Background(), ref)
			if !errors.Is(err, secrets.ErrNotFound) {
				t.Fatalf("Get error = %v, want ErrNotFound for a file that is empty after the newline rule", err)
			}
		})
	}
}

func TestFileDriverFollowsSymlink(t *testing.T) {
	t.Parallel()
	canary := newCanary(t)
	dir := t.TempDir()

	target := writeSecret(t, dir, "target", []byte("EXAMPLE-linked"), 0o400)
	link := filepath.Join(dir, "link")
	if err := os.Symlink(target.Path, link); err != nil {
		t.Fatal(err)
	}
	v, _, err := secrets.NewFileDriver().Get(context.Background(), secrets.Ref{Scheme: "file", Path: link})
	if err != nil {
		t.Fatalf("Get through a symlink: %v", err)
	}
	if got := string(v.Bytes()); got != "EXAMPLE-linked" {
		t.Fatalf("value = %q, want EXAMPLE-linked", got)
	}

	open := writeSecret(t, dir, "open", []byte(canary), 0o644)
	openLink := filepath.Join(dir, "open-link")
	if err := os.Symlink(open.Path, openLink); err != nil {
		t.Fatal(err)
	}
	assertInsecure(t, secrets.NewFileDriver(), secrets.Ref{Scheme: "file", Path: openLink}, "mode 0644", canary)
}

func TestFileDriverMissingAndEmpty(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	empty := writeSecret(t, dir, "empty", nil, 0o600)
	for name, ref := range map[string]secrets.Ref{
		"missing file":          {Scheme: "file", Path: filepath.Join(dir, "missing")},
		"missing directory":     {Scheme: "file", Path: filepath.Join(dir, "missing", "secret")},
		"empty file":            empty,
		"dangling symlink link": danglingLink(t, dir),
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			v, _, err := secrets.NewFileDriver().Get(context.Background(), ref)
			if !errors.Is(err, secrets.ErrNotFound) {
				t.Fatalf("Get error = %v, want ErrNotFound", err)
			}
			if code := secrets.ErrorCode(err); code != "not_found" {
				t.Fatalf("ErrorCode = %q, want not_found", code)
			}
			if !strings.Contains(err.Error(), ref.String()) {
				t.Fatalf("message %q does not name the reference", err)
			}
			if v.Len() != 0 {
				t.Fatalf("Get returned a value with the error")
			}
		})
	}
}

func danglingLink(t *testing.T, dir string) secrets.Ref {
	t.Helper()
	p := filepath.Join(dir, "dangling")
	if err := os.Symlink(filepath.Join(dir, "nowhere"), p); err != nil {
		t.Fatal(err)
	}
	return secrets.Ref{Scheme: "file", Path: p}
}

func TestFileDriverTooLarge(t *testing.T) {
	t.Parallel()
	canary := newCanary(t)
	const limit = 1048576
	content := func(n int) []byte {
		return append([]byte(canary), bytes.Repeat([]byte("A"), n-len(canary))...)
	}
	dir := t.TempDir()

	big := writeSecret(t, dir, "big", content(limit+1), 0o600)
	_, _, err := secrets.NewFileDriver().Get(context.Background(), big)
	if !errors.Is(err, secrets.ErrReadFailed) {
		t.Fatalf("Get of %d bytes: error = %v, want ErrReadFailed", limit+1, err)
	}
	if code := secrets.ErrorCode(err); code != "read_failed" {
		t.Fatalf("ErrorCode = %q, want read_failed", code)
	}
	if !strings.Contains(err.Error(), big.String()) {
		t.Fatalf("message %q does not name the reference", err)
	}
	assertNoSecret(t, "error", err.Error(), canary)

	exact := writeSecret(t, dir, "exact", content(limit), 0o600)
	v, _, err := secrets.NewFileDriver().Get(context.Background(), exact)
	if err != nil {
		t.Fatalf("Get of %d bytes: %v", limit, err)
	}
	if v.Len() != limit {
		t.Fatalf("Len = %d, want %d", v.Len(), limit)
	}
}

func TestFileDriverReadFailures(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions, so the stat cannot fail here")
	}
	dir := t.TempDir()
	locked := filepath.Join(dir, "locked")
	if err := os.Mkdir(locked, 0o700); err != nil {
		t.Fatal(err)
	}
	ref := writeSecret(t, locked, "secret", []byte("EXAMPLE"), 0o600)
	if err := os.Chmod(locked, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o700) }) // #nosec G302 -- restores the test directory so t.TempDir can remove it.
	_, _, err := secrets.NewFileDriver().Get(context.Background(), ref)
	if !errors.Is(err, secrets.ErrReadFailed) {
		t.Fatalf("Get error = %v, want ErrReadFailed for a file behind a closed directory", err)
	}
	if !strings.Contains(err.Error(), ref.String()) || !strings.Contains(err.Error(), "permission denied") {
		t.Fatalf("message %q does not name the reference and the cause", err)
	}
}

func TestFileDriverRejectsBadInput(t *testing.T) {
	t.Parallel()
	d := secrets.NewFileDriver()
	ctx := context.Background()
	for _, tc := range []struct {
		ref  secrets.Ref
		want error
	}{
		{secrets.Ref{Scheme: "file", Path: "relative"}, secrets.ErrInvalidRef},
		{secrets.Ref{Scheme: "file", Path: "/a/../b"}, secrets.ErrInvalidRef},
		{secrets.Ref{Scheme: "file"}, secrets.ErrInvalidRef},
		{secrets.Ref{Scheme: "env", Path: "BOTHY_SECRET_X"}, secrets.ErrUnsupportedScheme},
	} {
		if _, _, err := d.Get(ctx, tc.ref); !errors.Is(err, tc.want) {
			t.Errorf("Get(%#v) error = %v, want %v", tc.ref, err, tc.want)
		}
	}

	ref := writeSecret(t, t.TempDir(), "secret", []byte("EXAMPLE"), 0o600)
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, _, err := d.Get(canceled, ref); !errors.Is(err, context.Canceled) {
		t.Fatalf("Get with a canceled context: error = %v, want context.Canceled", err)
	}

	// A reference of another scheme has no grammar the driver can check; its path is not
	// repeated, not even beside the context error.
	canary := newCanary(t)
	for _, other := range []secrets.Ref{{Scheme: "azkv", Path: canary}, {Scheme: "foo", Path: canary}} {
		for name, c := range map[string]context.Context{"live": ctx, "canceled": canceled} {
			if _, _, err := d.Get(c, other); err == nil {
				t.Errorf("Get(%s) with a %s context: no error", other.Scheme, name)
			} else {
				assertNoSecret(t, "Get "+other.Scheme+" with a "+name+" context", err.Error(), canary)
			}
		}
	}
}
