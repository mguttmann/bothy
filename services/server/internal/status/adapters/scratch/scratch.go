// SPDX-License-Identifier: Apache-2.0

// Package scratch is the scratch area of the restore test (brief M7-03,
// docs/operations/backup-restore.md 4): it implements app.Scratch in a directory of the volume
// backup-scratch. It restores the backup with pgBackRest into <dir>/pgdata, starts a PostgreSQL
// there that listens only on a Unix socket in <dir>/socket (listen_addresses empty, no TCP),
// opens the copy over that socket, fetches sampled objects with restic into <dir>/objects, and
// stops the database and empties the directory again. It never connects to the production
// database.
//
// Example:
//
//	sc, err := scratch.New(scratch.Config{Dir: "/var/lib/bothy-backup/restore", Tools: exec, FilesRoot: root, Open: open})
//	// svc deps: app.Deps{Scratch: sc, ...}
package scratch

import (
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/status/adapters/postgres"
	"github.com/OWNER/bothy/services/server/internal/status/app"
)

// The defaults of Config.
const (
	defaultPostgres     = "postgres"
	defaultStartTimeout = 30 * time.Minute
	defaultStopTimeout  = time.Minute
	// pollInterval is the pause between two attempts to reach the starting copy.
	pollInterval = 250 * time.Millisecond
)

// The entries of the scratch directory.
const (
	dirPGData  = "pgdata"
	dirSocket  = "socket"
	dirObjects = "objects"
	fileHBA    = "pg_hba.conf"
	// objectsTag is the restic tag of the object backup (domain.KindObjects).
	objectsTag = "objects"
)

// hbaConf admits local socket connections only. The socket directory has mode 0700, so only the
// user of bothy-backupd reaches it; the copy's own pg_hba.conf of production is not used.
const hbaConf = "local all all trust\n"

// Error codes of this adapter (backup_runs and restore_test_checks.v1 reasons).
const (
	codeStartFailed   = "scratch_start_failed"
	codeStartTimeout  = "scratch_start_timeout"
	codeClearFailed   = "scratch_cleanup_failed"
	codeObjectsNotFS  = "objects_driver_s3"
	codeScratchInUse  = "scratch_in_use"
	codeDirInvalid    = "scratch_dir_invalid"
	codeRestoreFailed = "pgbackrest_restore_failed"
)

// Tools are the binaries the scratch area runs; *executor.Executor implements them.
type Tools interface {
	// PgBackRestRestore restores set label from repo1 into pgPath.
	PgBackRestRestore(ctx context.Context, label, pgPath string) error
	// PgBackRestVerify verifies repo1.
	PgBackRestVerify(ctx context.Context) error
	// ResticRestore restores includes of the newest snapshot with tag into target.
	ResticRestore(ctx context.Context, tag string, includes []string, target string) error
}

// Config is the configuration of the scratch area.
type Config struct {
	// Dir is the directory of the restore test in the volume backup-scratch; it must exist and
	// belong to the user of bothy-backupd. Required, absolute.
	Dir string
	// Tools runs pgBackRest and restic. Required.
	Tools Tools
	// Open opens the pool of the copy on the socket directory socketDir. Required.
	Open func(ctx context.Context, socketDir string) (*pg.Pool, error)
	// Postgres is the PostgreSQL server binary; default postgres from the PATH of the process.
	Postgres string
	// FilesRoot is file.fs.root, the path of the files in the restic snapshots of the object
	// backup; empty with the driver s3, where FetchObjects fails.
	FilesRoot string
	// Stderr receives the log of the scratch PostgreSQL (the container log); nil discards it.
	Stderr io.Writer
	// StartTimeout bounds the wait for the copy to accept queries; default 30m.
	StartTimeout time.Duration
}

// Scratch implements app.Scratch. One restore test uses it at a time (the slot of the restore
// test); its methods are safe for concurrent use.
type Scratch struct {
	cfg Config

	mu   sync.Mutex
	cmd  *exec.Cmd
	done chan struct{}
	pool *pg.Pool
}

// Compile-time check: Scratch is the scratch area of the restore test.
var _ app.Scratch = (*Scratch)(nil)

// New returns the scratch area of cfg with its defaults filled in. The error names a field.
func New(cfg Config) (*Scratch, error) {
	switch {
	case cfg.Dir == "" || !filepath.IsAbs(cfg.Dir) || filepath.Clean(cfg.Dir) != cfg.Dir:
		return nil, errors.New("scratch: Config.Dir must be a clean absolute path")
	case cfg.Tools == nil:
		return nil, errors.New("scratch: Config.Tools is nil")
	case cfg.Open == nil:
		return nil, errors.New("scratch: Config.Open is nil")
	}
	if cfg.Postgres == "" {
		cfg.Postgres = defaultPostgres
	}
	if cfg.StartTimeout <= 0 {
		cfg.StartTimeout = defaultStartTimeout
	}
	if cfg.Stderr == nil {
		cfg.Stderr = io.Discard
	}
	return &Scratch{cfg: cfg}, nil
}

// execError returns the *app.ExecError of code.
func execError(code string) error { return &app.ExecError{Code: code} }

// Restore implements app.Scratch: it empties the directory and restores label into
// <dir>/pgdata.
func (s *Scratch) Restore(ctx context.Context, label string) error {
	if err := s.Clear(ctx); err != nil {
		return err
	}
	pgdata := filepath.Join(s.cfg.Dir, dirPGData)
	if err := s.cfg.Tools.PgBackRestRestore(ctx, label, pgdata); err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(pgdata, "PG_VERSION")); err != nil {
		// pgBackRest exited 0 without a data directory: nothing was restored.
		return execError(codeRestoreFailed)
	}
	return nil
}

// Verify implements app.Scratch with pgbackrest verify on repo1.
func (s *Scratch) Verify(ctx context.Context) error { return s.cfg.Tools.PgBackRestVerify(ctx) }

// Start implements app.Scratch: it writes pg_hba.conf, creates the socket directory with mode
// 0700, starts the server with listen_addresses empty, the socket in that directory, archive_mode
// off and the own pg_hba.conf, and waits until the copy answers outside recovery. A server that
// exits during the start is scratch_start_failed, one that does not answer within StartTimeout
// scratch_start_timeout.
func (s *Scratch) Start(ctx context.Context) (app.ScratchDB, error) {
	s.mu.Lock()
	if s.cmd != nil {
		s.mu.Unlock()
		return nil, execError(codeScratchInUse)
	}
	s.mu.Unlock()
	socket := filepath.Join(s.cfg.Dir, dirSocket)
	hba := filepath.Join(s.cfg.Dir, fileHBA)
	if err := os.Mkdir(socket, 0o700); err != nil && !errors.Is(err, fs.ErrExist) {
		return nil, execError(codeStartFailed)
	}
	if err := os.WriteFile(hba, []byte(hbaConf), 0o600); err != nil {
		return nil, execError(codeStartFailed)
	}
	// #nosec G204 -- the configured server binary with a fixed argument list, no shell
	cmd := exec.Command(s.cfg.Postgres, serverArgs(filepath.Join(s.cfg.Dir, dirPGData), socket, hba)...)
	cmd.Env = []string{"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin", "LANG=C.UTF-8", "HOME=/tmp"}
	cmd.Stdout, cmd.Stderr = s.cfg.Stderr, s.cfg.Stderr
	if err := cmd.Start(); err != nil {
		return nil, execError(codeStartFailed)
	}
	done := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(done)
	}()
	s.mu.Lock()
	s.cmd, s.done = cmd, done
	s.mu.Unlock()
	pool, err := s.waitReady(ctx, socket, done)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	s.pool = pool
	s.mu.Unlock()
	if err := afterStart(ctx, s.cfg.Dir, pool); err != nil {
		return nil, execError(codeStartFailed)
	}
	return postgres.NewScratchDB(pool), nil
}

// serverArgs are the arguments of the scratch PostgreSQL. The command line wins over
// postgresql.conf and postgresql.auto.conf of the restored directory: no TCP listener, the socket
// in the scratch directory only for its owner, no WAL archiving (the copy must never push its own
// timeline into the repository), no TLS (the certificates of production are not part of the
// copy), and the log on stderr.
func serverArgs(pgdata, socket, hba string) []string {
	return []string{
		"-D", pgdata,
		"-c", "listen_addresses=",
		"-c", "unix_socket_directories=" + socket,
		"-c", "unix_socket_permissions=0700",
		"-c", "hba_file=" + hba,
		"-c", "archive_mode=off",
		"-c", "ssl=off",
		"-c", "logging_collector=off",
		"-c", "log_destination=stderr",
		"-c", "max_connections=20",
	}
}

// waitReady opens the pool and polls until the copy answers outside recovery, the server exits,
// StartTimeout passes or ctx ends.
func (s *Scratch) waitReady(ctx context.Context, socket string, exited <-chan struct{}) (*pg.Pool, error) {
	ctx, cancel := context.WithTimeout(ctx, s.cfg.StartTimeout)
	defer cancel()
	pool, err := s.cfg.Open(ctx, socket)
	if err != nil {
		return nil, execError(codeStartFailed)
	}
	for {
		var recovering bool
		if err := pool.QueryRow(ctx, `SELECT pg_is_in_recovery()`).Scan(&recovering); err == nil && !recovering {
			return pool, nil
		}
		t := time.NewTimer(pollInterval)
		select {
		case <-exited:
			t.Stop()
			pool.Close()
			return nil, execError(codeStartFailed)
		case <-ctx.Done():
			t.Stop()
			pool.Close()
			return nil, execError(codeStartTimeout)
		case <-t.C:
		}
	}
}

// FetchObjects implements app.Scratch: it restores the objects of keys from the newest restic
// snapshot of the object backup into <dir>/objects and hashes each one found. The files are read
// through an os.Root of that directory, so a key of the copy that points outside it is not read
// and counts as missing.
func (s *Scratch) FetchObjects(ctx context.Context, keys []string) (map[string][sha256.Size]byte, error) {
	if s.cfg.FilesRoot == "" {
		return nil, execError(codeObjectsNotFS)
	}
	target := filepath.Join(s.cfg.Dir, dirObjects)
	if err := os.MkdirAll(target, 0o700); err != nil {
		return nil, execError(codeDirInvalid)
	}
	includes := make([]string, 0, len(keys))
	for _, k := range keys {
		includes = append(includes, path.Join(filepath.ToSlash(s.cfg.FilesRoot), k))
	}
	if err := s.cfg.Tools.ResticRestore(ctx, objectsTag, includes, target); err != nil {
		return nil, err
	}
	if err := afterFetch(ctx, target); err != nil {
		return nil, execError(codeDirInvalid)
	}
	root, err := os.OpenRoot(target)
	if err != nil {
		return nil, execError(codeDirInvalid)
	}
	defer func() { _ = root.Close() }()
	out := make(map[string][sha256.Size]byte, len(keys))
	for i, k := range keys {
		rel := strings.TrimPrefix(includes[i], "/")
		sum, ok := hashFile(root, filepath.FromSlash(rel))
		if ok {
			out[k] = sum
		}
	}
	return out, nil
}

// hashFile returns the SHA-256 of the regular file name below root, false when it is missing,
// not a regular file or outside root.
func hashFile(root *os.Root, name string) ([sha256.Size]byte, bool) {
	f, err := root.Open(name)
	if err != nil {
		return [sha256.Size]byte{}, false
	}
	defer func() { _ = f.Close() }()
	if st, err := f.Stat(); err != nil || !st.Mode().IsRegular() {
		return [sha256.Size]byte{}, false
	}
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return [sha256.Size]byte{}, false
	}
	var sum [sha256.Size]byte
	copy(sum[:], h.Sum(nil))
	return sum, true
}

// Clear implements app.Scratch: it closes the pool of the copy, stops the server (SIGINT, a fast
// shutdown, then SIGKILL after a minute) and removes every entry of the directory, which itself
// stays. It returns scratch_cleanup_failed when an entry cannot be removed.
func (s *Scratch) Clear(ctx context.Context) error {
	s.mu.Lock()
	pool, cmd, done := s.pool, s.cmd, s.done
	s.pool, s.cmd, s.done = nil, nil, nil
	s.mu.Unlock()
	if pool != nil {
		pool.Close()
	}
	if cmd != nil {
		stop(ctx, cmd, done)
	}
	entries, err := os.ReadDir(s.cfg.Dir)
	if err != nil {
		return execError(codeClearFailed)
	}
	var failed bool
	for _, e := range entries {
		if err := os.RemoveAll(filepath.Join(s.cfg.Dir, e.Name())); err != nil {
			failed = true
		}
	}
	if failed {
		return execError(codeClearFailed)
	}
	return nil
}

// stop ends the server of cmd and waits for it: SIGINT is the fast shutdown of PostgreSQL;
// after defaultStopTimeout, or when ctx ends, it is killed.
func stop(ctx context.Context, cmd *exec.Cmd, done <-chan struct{}) {
	if cmd.Process == nil {
		return
	}
	_ = cmd.Process.Signal(os.Interrupt)
	t := time.NewTimer(defaultStopTimeout)
	defer t.Stop()
	select {
	case <-done:
		return
	case <-t.C:
	case <-ctx.Done():
	}
	_ = cmd.Process.Kill()
	<-done
}
