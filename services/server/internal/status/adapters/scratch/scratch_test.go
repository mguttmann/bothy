// SPDX-License-Identifier: Apache-2.0

package scratch_test

import (
	"context"
	"crypto/sha256"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/secrets"
	"github.com/OWNER/bothy/services/server/internal/status/adapters/scratch"
	"github.com/OWNER/bothy/services/server/internal/status/app"
)

// fakeTools records the calls and writes what the real tools would write.
type fakeTools struct {
	restored []string
	includes [][]string
	// objects maps a restored path below the target (as stored, absolute) to its content.
	objects    map[string]string
	restoreErr error
	noData     bool
}

func (f *fakeTools) PgBackRestRestore(_ context.Context, label, pgPath string) error {
	f.restored = append(f.restored, label)
	if f.restoreErr != nil {
		return f.restoreErr
	}
	if f.noData {
		return nil
	}
	if err := os.MkdirAll(pgPath, 0o700); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(pgPath, "PG_VERSION"), []byte("18\n"), 0o600)
}

func (f *fakeTools) PgBackRestVerify(context.Context) error { return nil }

func (f *fakeTools) ResticRestore(_ context.Context, tag, path string, includes []string, target string) error {
	if tag != "objects" || path != "/var/lib/bothy/files" {
		return errors.New("wrong snapshot")
	}
	f.includes = append(f.includes, slices.Clone(includes))
	for _, p := range includes {
		content, ok := f.objects[p]
		if !ok {
			continue
		}
		dst := filepath.Join(target, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
			return err
		}
		if err := os.WriteFile(dst, []byte(content), 0o600); err != nil {
			return err
		}
	}
	return nil
}

func newScratch(t *testing.T, tools *fakeTools, mutate func(*scratch.Config)) (*scratch.Scratch, string) {
	t.Helper()
	dir := t.TempDir()
	cfg := scratch.Config{Dir: dir, Tools: tools, FilesRoot: "/var/lib/bothy/files",
		Open: func(context.Context, string) (*pg.Pool, error) { return nil, errors.New("no database in this test") }}
	if mutate != nil {
		mutate(&cfg)
	}
	s, err := scratch.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return s, dir
}

// TestScratchRestoreFetchClear holds the scratch area without a database: Restore empties the
// directory first and restores into <dir>/pgdata; FetchObjects asks restic for the paths below
// file.fs.root and hashes what it restored, a missing object and a key that leaves the directory
// are absent; Clear empties the directory and keeps it.
func TestScratchRestoreFetchClear(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	tools := &fakeTools{objects: map[string]string{
		"/var/lib/bothy/files/f/ab/1": "alpha-EXAMPLE",
		"/var/lib/bothy/files/f/cd/2": "beta-EXAMPLE",
	}}
	s, dir := newScratch(t, tools, nil)
	if err := os.WriteFile(filepath.Join(dir, "left-over"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := s.Restore(ctx, "20261018-020000F"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "left-over")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("Restore kept a file of an earlier run: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "pgdata", "PG_VERSION")); err != nil || !slices.Equal(tools.restored, []string{"20261018-020000F"}) {
		t.Errorf("restore into pgdata: %v, labels %v", err, tools.restored)
	}

	keys := []string{"f/ab/1", "f/cd/2", "f/ef/3", "../../../../etc/passwd"}
	sums, err := s.FetchObjects(ctx, keys)
	if err != nil {
		t.Fatal(err)
	}
	if len(sums) != 2 || sums["f/ab/1"] != sha256.Sum256([]byte("alpha-EXAMPLE")) || sums["f/cd/2"] != sha256.Sum256([]byte("beta-EXAMPLE")) {
		t.Errorf("sums %v, want the two restored objects", sums)
	}
	if want := []string{"/var/lib/bothy/files/f/ab/1", "/var/lib/bothy/files/f/cd/2", "/var/lib/bothy/files/f/ef/3", "/etc/passwd"}; !slices.Equal(tools.includes[0], want) {
		t.Errorf("includes %q, want %q", tools.includes[0], want)
	}

	if err := s.Clear(ctx); err != nil {
		t.Fatal(err)
	}
	if entries, err := os.ReadDir(dir); err != nil || len(entries) != 0 {
		t.Errorf("after Clear: %v entries, %v", len(entries), err)
	}
	if err := s.Clear(ctx); err != nil {
		t.Errorf("second Clear: %v", err)
	}
}

// TestScratchFailures: a restore that fails or leaves no data directory fails with a code; a
// server that exits during the start is scratch_start_failed and its directory is cleared after;
// the driver s3 has no restic snapshot to fetch from.
func TestScratchFailures(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	failing := &fakeTools{restoreErr: &app.ExecError{Code: "pgbackrest_exit_62"}}
	s, _ := newScratch(t, failing, nil)
	if err := s.Restore(ctx, "20261018-020000F"); !hasCode(err, "pgbackrest_exit_62") {
		t.Errorf("failed restore: %v", err)
	}
	empty, _ := newScratch(t, &fakeTools{noData: true}, nil)
	if err := empty.Restore(ctx, "20261018-020000F"); !hasCode(err, "pgbackrest_restore_failed") {
		t.Errorf("restore without data: %v", err)
	}

	bin := filepath.Join(t.TempDir(), "postgres")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\nexit 1\n"), 0o700); err != nil { // #nosec G306 -- an executable test fixture
		t.Fatal(err)
	}
	opened := ""
	starts, dir := newScratch(t, &fakeTools{}, func(c *scratch.Config) {
		c.Postgres, c.StartTimeout = bin, time.Minute
		c.Open = func(_ context.Context, socket string) (*pg.Pool, error) {
			opened = socket
			secretDir := t.TempDir()
			if err := os.WriteFile(filepath.Join(secretDir, "pw"), []byte("EXAMPLE"), 0o600); err != nil {
				return nil, err
			}
			sp, err := secrets.NewProvider(secrets.Options{Dir: secretDir})
			if err != nil {
				return nil, err
			}
			cfg := pg.Config{Host: socket, Port: 5432, Name: "bothy", User: "postgres", PasswordRef: "file:" + filepath.Join(secretDir, "pw"),
				SSLMode: pg.SSLModeDisable, MaxConns: 1, AcquireTimeout: time.Second}
			return pg.Open(ctx, cfg, sp, nil)
		}
	})
	if err := starts.Restore(ctx, "20261018-020000F"); err != nil {
		t.Fatal(err)
	}
	if _, err := starts.Start(ctx); !hasCode(err, "scratch_start_failed") {
		t.Errorf("start of an exiting server: %v", err)
	}
	if opened != filepath.Join(dir, "socket") {
		t.Errorf("pool opened on %q, want the socket directory", opened)
	}
	if st, err := os.Stat(filepath.Join(dir, "socket")); err != nil || st.Mode().Perm() != 0o700 {
		t.Errorf("socket directory %v, %v; want mode 0700", st, err)
	}
	if b, err := os.ReadFile(filepath.Join(dir, "pg_hba.conf")); err != nil || !strings.HasPrefix(string(b), "local all all trust") {
		t.Errorf("pg_hba.conf %q, %v", b, err)
	}
	if err := starts.Clear(ctx); err != nil {
		t.Fatal(err)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Errorf("%d entries after Clear", len(entries))
	}

	s3, _ := newScratch(t, &fakeTools{}, func(c *scratch.Config) { c.FilesRoot = "" })
	if _, err := s3.FetchObjects(ctx, []string{"f/ab/1"}); !hasCode(err, "objects_driver_s3") {
		t.Errorf("FetchObjects without file.fs.root: %v", err)
	}
	if _, err := scratch.New(scratch.Config{Dir: "relative", Tools: &fakeTools{}, Open: func(context.Context, string) (*pg.Pool, error) { return nil, nil }}); err == nil {
		t.Error("New accepted a relative directory")
	}
}

// hasCode reports whether err is an *app.ExecError with code.
func hasCode(err error, code string) bool {
	var ee *app.ExecError
	return errors.As(err, &ee) && ee.Code == code
}
