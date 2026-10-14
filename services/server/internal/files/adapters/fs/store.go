// SPDX-License-Identifier: Apache-2.0

package fs

import (
	"context"
	"errors"
	"fmt"
	iofs "io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/OWNER/bothy/services/server/internal/files/app"
	"github.com/OWNER/bothy/services/server/internal/files/domain"
)

// tmpDir holds the temporary files of writes. It starts with a dot, which no key may contain, so
// no key ever addresses a temporary file (docs/DECISIONS.md M3-01/7).
const tmpDir = ".tmp"

// rootState is the state Open expects of file.fs.root; errors name it instead of the path.
const rootState = "want a directory owned by the server user with mode 0700"

// expectedOwner returns the user ID that must own the root: the effective user of the process.
// Tests replace it (export_test.go), because they cannot create a directory of another user.
var expectedOwner = os.Geteuid

// errSymlink is the cause of a storage failure when a path component is a symbolic link. It is
// not ErrNotFound: callers answer 503 and the operator sees the cause.
var errSymlink = errors.New("symbolic link in the object path")

// Config is the configuration of the file system driver; internal/wire fills it from
// file.fs.root.
type Config struct {
	// Root is file.fs.root: an absolute, clean path of a directory used only by the server.
	Root string
}

// Store is the file system driver of app.ObjectStore (docs/spec/12-files.md "4.2"). Keys map
// to paths below the root with filepath.FromSlash; every operation goes through an os.Root of the
// root directory, so nothing outside it is ever read, written or deleted, and no symbolic link is
// followed at any level. It is safe for concurrent use, and several stores (the roles api and
// worker) may share one root, because every change is a rename or an unlink.
type Store struct {
	root *os.Root
	// hook is the test seam that sees the steps of every write; nil outside tests.
	hook func(event, name string)
}

var _ app.ObjectStore = (*Store)(nil)

// Open checks file.fs.root and opens the store on it (docs/DECISIONS.md M3-01/9, the same start
// check as a server-owned key directory): the root is an absolute, clean path; it exists; Lstat
// shows a directory and not a symbolic link; it is owned by the effective user of the process;
// and its permission bits are exactly 0700. Open creates <root>/.tmp with mode 0700 when missing,
// which also proves that the root is writable, and never removes entries of it: they may belong
// to another process on the same root. Open never creates the root itself; a missing root means
// a wrong mount.
//
// The error names file.fs.root and the expected state, never the path.
//
// Example:
//
//	store, err := fs.Open(fs.Config{Root: "/var/lib/bothy/files"})
//	if err != nil {
//		return err // "file.fs.root: does not exist (want a directory owned by ...)"
//	}
//	defer store.Close()
func Open(cfg Config) (*Store, error) {
	fail := func(what string) error { return fmt.Errorf("file.fs.root: %s (%s)", what, rootState) }
	r := cfg.Root
	if r == "" || !filepath.IsAbs(r) || filepath.Clean(r) != r {
		return nil, fail("must be an absolute path without . or .. elements and without a trailing slash")
	}
	fi, err := os.Lstat(r)
	switch {
	case errors.Is(err, iofs.ErrNotExist):
		return nil, fail("does not exist")
	case err != nil:
		return nil, fail("cannot be inspected: " + causeText(err))
	case fi.Mode()&iofs.ModeSymlink != 0:
		return nil, fail("is a symbolic link")
	case !fi.IsDir():
		return nil, fail("is not a directory")
	}
	if uid, ok := fileOwner(fi); !ok || uid != expectedOwner() {
		return nil, fail("is not owned by the server user")
	}
	if !serverDir(fi) {
		return nil, fail("has permission bits other than 0700")
	}
	root, err := os.OpenRoot(r)
	if err != nil {
		return nil, fail("cannot be opened: " + causeText(err))
	}
	// The directory may have been swapped between Lstat and OpenRoot.
	if st, err := root.Stat("."); err != nil || !os.SameFile(fi, st) {
		_ = root.Close()
		return nil, fail("changed while it was opened")
	}
	if err := root.Mkdir(tmpDir, 0o700); err != nil && !errors.Is(err, iofs.ErrExist) {
		_ = root.Close()
		return nil, fail("is not writable: " + causeText(err))
	}
	if ti, err := root.Lstat(tmpDir); err != nil || !serverDir(ti) {
		_ = root.Close()
		return nil, fail("holds a .tmp that is not a directory owned by the server user with mode 0700")
	}
	return &Store{root: root}, nil
}

// serverDir reports a directory (not a link) owned by the expected user whose permission bits
// are exactly 0700, without setuid, setgid or sticky bit.
func serverDir(fi iofs.FileInfo) bool {
	uid, ok := fileOwner(fi)
	return ok && uid == expectedOwner() && fi.IsDir() && fi.Mode()&iofs.ModeSymlink == 0 &&
		fi.Mode()&(iofs.ModePerm|iofs.ModeSetuid|iofs.ModeSetgid|iofs.ModeSticky) == 0o700
}

// Close releases the directory handle of the root. Operations after Close fail.
func (s *Store) Close() error { return s.root.Close() }

// PresignUploadPart refuses with app.ErrNotSupported after the key check and the argument
// checks of the port (app.ErrInvalidPart for n < 1, app.ErrSizeMismatch for size < 1) and
// without any file system call: presigned uploads exist only with the S3 driver (12 "4.1",
// "3.3").
func (s *Store) PresignUploadPart(_ context.Context, key, _ string, n int, size int64, _ time.Duration) (app.PresignedPart, error) {
	if err := domain.ValidateKey(key); err != nil {
		return app.PresignedPart{}, opErr("presign upload part", "", err)
	}
	if n < 1 {
		return app.PresignedPart{}, opErr("presign upload part", key, app.ErrInvalidPart)
	}
	if size < 1 {
		return app.PresignedPart{}, opErr("presign upload part", key, app.ErrSizeMismatch)
	}
	return app.PresignedPart{}, opErr("presign upload part", key, app.ErrNotSupported)
}

// event reports one step of a write to the test seam.
func (s *Store) event(e, name string) {
	if s.hook != nil {
		s.hook(e, name)
	}
}

// opErr wraps err with the operation and the key. A refused key is not repeated, because it may
// carry anything a caller passed.
func opErr(op, key string, err error) error {
	if key == "" {
		return fmt.Errorf("files/fs: %s: %w", op, err)
	}
	return fmt.Errorf("files/fs: %s %s: %w", op, key, err)
}

// causeText returns the system error of err without the path a PathError carries.
func causeText(err error) string {
	var pe *iofs.PathError
	if errors.As(err, &pe) {
		return pe.Err.Error()
	}
	var le *os.LinkError
	if errors.As(err, &le) {
		return le.Err.Error()
	}
	return err.Error()
}

// osPath maps a key or a slash path below the root to the name os.Root expects.
func osPath(name string) string { return filepath.FromSlash(name) }

// inspect checks every existing component of name with Lstat through the root, so no symbolic
// link is followed at any level (docs/DECISIONS.md M3-01/8). It returns the FileInfo of the final
// component, or nil when a component is missing or a parent is not a directory; a symbolic link
// anywhere gives errSymlink.
func (s *Store) inspect(name string) (iofs.FileInfo, error) {
	segs := strings.Split(name, "/")
	for i := range segs {
		p := strings.Join(segs[:i+1], "/")
		fi, err := s.root.Lstat(osPath(p))
		switch {
		case errors.Is(err, iofs.ErrNotExist) || notDir(err):
			return nil, nil
		case err != nil:
			return nil, err
		case fi.Mode()&iofs.ModeSymlink != 0:
			return nil, errSymlink
		case i < len(segs)-1 && !fi.IsDir():
			return nil, nil
		case i == len(segs)-1:
			return fi, nil
		}
	}
	return nil, nil
}

// mkdirs creates the missing directories of dir with mode 0700, refusing an existing component
// that is a symbolic link or not a directory. A component that a concurrent Delete removes
// between Mkdir and Lstat is created again, until ctx ends.
func (s *Store) mkdirs(ctx context.Context, dir string) error {
	if dir == "." || dir == "" {
		return nil
	}
	segs := strings.Split(dir, "/")
	for i := 0; i < len(segs); {
		again, err := s.mkdir(strings.Join(segs[:i+1], "/"))
		if err != nil {
			return err
		}
		if again {
			if err := ctx.Err(); err != nil {
				return err
			}
			continue
		}
		i++
	}
	return nil
}

// mkdir creates one directory p (a slash path) with mode 0700 and syncs its parent, so the whole
// path of an object survives a crash, not only the last directory. An existing directory is
// accepted; a symbolic link or another entry is refused. again reports that p vanished between
// Mkdir and Lstat and must be created again.
func (s *Store) mkdir(p string) (again bool, err error) {
	err = s.root.Mkdir(osPath(p), 0o700)
	if err == nil {
		if err := s.syncDir(path.Dir(p)); err != nil && !errors.Is(err, iofs.ErrNotExist) {
			return false, err
		}
		s.event("sync-new-dir", p)
		return false, nil
	}
	if !errors.Is(err, iofs.ErrExist) {
		return false, err
	}
	fi, err := s.root.Lstat(osPath(p))
	switch {
	case errors.Is(err, iofs.ErrNotExist):
		return true, nil
	case err != nil:
		return false, err
	case fi.Mode()&iofs.ModeSymlink != 0:
		return false, errSymlink
	case !fi.IsDir():
		return false, fmt.Errorf("%s is not a directory", path.Base(p))
	}
	return false, nil
}

// syncDir syncs directory dir, so a rename or unlink in it survives a crash. Only the error
// "not supported" counts as success, for platforms that cannot sync a directory.
func (s *Store) syncDir(dir string) error {
	d, err := s.root.OpenFile(osPath(dir), os.O_RDONLY, 0)
	if err != nil {
		return err
	}
	err = d.Sync()
	cerr := d.Close()
	if err != nil && !syncUnsupported(err) {
		return err
	}
	return cerr
}
