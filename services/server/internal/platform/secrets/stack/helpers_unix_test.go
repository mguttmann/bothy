// SPDX-License-Identifier: Apache-2.0

//go:build unix

package stack_test

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/platform/secrets/stack"
)

// me is the uid:gid the tests run as; init tests use it for every owner so they need no root.
func me() stack.Owner { return stack.Owner{UID: os.Geteuid(), GID: os.Getegid()} }

// owners gives server and migrate, the services that need one, the running user as owner.
func owners() map[string]stack.Owner {
	return map[string]stack.Owner{"server": me(), "migrate": me()}
}

// dirs returns a fresh target path (not created) and an empty source directory.
func dirs(t *testing.T) (target, source string) {
	t.Helper()
	base := t.TempDir()
	source = filepath.Join(base, "source")
	if err := os.Mkdir(source, 0o700); err != nil {
		t.Fatal(err)
	}
	target = filepath.Join(base, "target")
	// Service directories have mode 0500; the TempDir cleanup, registered before and so run after
	// this one, can only remove them once they are writable again.
	t.Cleanup(func() {
		_ = filepath.WalkDir(target, func(path string, d fs.DirEntry, err error) error {
			if err == nil && d.IsDir() {
				_ = os.Chmod(path, 0o700) //nolint:gosec // G302: a directory needs the x bit to be emptied.
			}
			return nil
		})
	})
	return target, source
}

func opts(target, source string) stack.InitOptions {
	return stack.InitOptions{Target: target, Source: source, Owners: owners()}
}

func mustInit(t *testing.T, o stack.InitOptions) stack.Report {
	t.Helper()
	r, err := stack.Init(t.Context(), o)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	return r
}

// put writes a file below root, creating parent directories; a read-only directory on the way
// (a service directory has mode 0500) is opened for the write and closed again, so tests can
// plant files like an intruder.
func put(t *testing.T, root, rel, content string, mode fs.FileMode) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	defer openDir(t, filepath.Dir(path))()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Lstat(path); err == nil && info.Mode().IsRegular() {
		chmod(t, path, 0o600)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	chmod(t, path, mode)
}

// openDir makes the nearest existing directory at or above dir writable and returns the function
// that restores its mode.
func openDir(t *testing.T, dir string) func() {
	t.Helper()
	for {
		info, err := os.Stat(dir)
		if errors.Is(err, fs.ErrNotExist) {
			dir = filepath.Dir(dir)
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm()&0o200 != 0 {
			return func() {}
		}
		chmod(t, dir, 0o700)
		return func() { chmod(t, dir, info.Mode().Perm()) }
	}
}

// remove deletes a file or tree below root, opening a read-only parent directory for it.
func remove(t *testing.T, root, rel string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	defer openDir(t, filepath.Dir(path))()
	if err := os.RemoveAll(path); err != nil {
		t.Fatal(err)
	}
}

func chmod(t *testing.T, path string, mode fs.FileMode) {
	t.Helper()
	if err := os.Chmod(path, mode); err != nil {
		t.Fatal(err)
	}
}

func read(t *testing.T, root, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Clean(filepath.Join(root, filepath.FromSlash(rel))))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func exists(t *testing.T, path string) bool {
	t.Helper()
	_, err := os.Lstat(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		t.Fatal(err)
	}
	return err == nil
}

// state is what TestInitSecondRunChangesNothing compares per path.
type state struct {
	mode     fs.FileMode
	uid, gid uint32
	size     int64
	mtime    int64
	sum      string
}

// snapshot records path, mode, owner, size, mtime in nanoseconds and the SHA-256 of every entry
// below dir.
func snapshot(t *testing.T, dir string) map[string]state {
	t.Helper()
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = root.Close() }()
	out := map[string]state{}
	err = fs.WalkDir(root.FS(), ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := root.Lstat(name)
		if err != nil {
			return err
		}
		st, ok := info.Sys().(*syscall.Stat_t)
		if !ok {
			t.Fatalf("%s: no syscall.Stat_t", name)
		}
		s := state{mode: info.Mode(), uid: st.Uid, gid: st.Gid, size: info.Size(), mtime: info.ModTime().UnixNano()}
		if info.Mode().IsRegular() {
			data, err := root.ReadFile(name)
			if err != nil {
				return err
			}
			s.sum = sha(string(data))
		}
		out[name] = s
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func diffSnapshots(t *testing.T, before, after map[string]state) {
	t.Helper()
	for _, p := range slices.Sorted(maps.Keys(before)) {
		a, ok := after[p]
		switch {
		case !ok:
			t.Errorf("%s vanished", p)
		case a != before[p]:
			t.Errorf("%s changed: before %+v, after %+v", p, before[p], a)
		}
	}
	for _, p := range slices.Sorted(maps.Keys(after)) {
		if _, ok := before[p]; !ok {
			t.Errorf("%s appeared", p)
		}
	}
}

// files lists the regular files below dir as relative slash paths, sorted.
func files(t *testing.T, dir string) []string {
	t.Helper()
	var out []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			rel, err := filepath.Rel(dir, path)
			if err != nil {
				return err
			}
			out = append(out, filepath.ToSlash(rel))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// owner returns uid and gid of path.
func owner(t *testing.T, path string) stack.Owner {
	t.Helper()
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		t.Fatalf("%s: no syscall.Stat_t", path)
	}
	return stack.Owner{UID: int(st.Uid), GID: int(st.Gid)}
}

func perm(t *testing.T, path string) fs.FileMode {
	t.Helper()
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	return info.Mode()
}

func sha(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// fingerprint is the documented form sha256: plus 12 hex characters, derived here without the
// code under test.
func fingerprint(content string) string { return "sha256:" + sha(content)[:12] }

// statuses maps line names to "<status> <fingerprint>" (or "<status>").
func statuses(r stack.Report) map[string]string {
	m := map[string]string{}
	for _, l := range r.Lines {
		m[l.Name] = strings.TrimSpace(string(l.Status) + " " + l.Fingerprint)
	}
	return m
}

func reportText(t *testing.T, r stack.Report) string {
	t.Helper()
	var b strings.Builder
	if _, err := r.WriteTo(&b); err != nil {
		t.Fatal(err)
	}
	return b.String()
}
