// SPDX-License-Identifier: Apache-2.0

package fs

import (
	"context"
	"errors"
	"io"
	iofs "io/fs"
	"os"
	"path"
	"strings"

	"github.com/OWNER/bothy/services/server/internal/files/app"
	"github.com/OWNER/bothy/services/server/internal/files/domain"
)

// objectReader streams one object or one range of it through ReadAt; Close closes the file.
type objectReader struct {
	*io.SectionReader
	f *os.File
}

// Close closes the file of the object.
func (r *objectReader) Close() error { return r.f.Close() }

// Get opens key for reading, rng nil for the whole object or one byte range
// (docs/spec/12-files.md "4.2": Get with a range reads through ReadAt; the server keeps no
// cache of objects). A range whose end lies beyond the object ends at the last byte.
//
// Errors: app.ErrInvalidKey before any file system call; app.ErrInvalidRange for Offset < 0,
// Length < 1 or Offset at or beyond the size; app.ErrNotFound when no regular file is at the key
// (a directory counts as missing); a symbolic link anywhere in the path is a storage failure.
func (s *Store) Get(ctx context.Context, key string, rng *app.ByteRange) (io.ReadCloser, error) {
	if err := domain.ValidateKey(key); err != nil {
		return nil, opErr("get", "", err)
	}
	if rng != nil && (rng.Offset < 0 || rng.Length < 1) {
		return nil, opErr("get", key, app.ErrInvalidRange)
	}
	if err := ctx.Err(); err != nil {
		return nil, opErr("get", key, err)
	}
	f, size, err := s.openRegular(key)
	if err != nil {
		return nil, opErr("get", key, err)
	}
	off, n := int64(0), size
	if rng != nil {
		if rng.Offset >= size {
			_ = f.Close()
			return nil, opErr("get", key, app.ErrInvalidRange)
		}
		off, n = rng.Offset, min(rng.Length, size-rng.Offset)
	}
	return &objectReader{SectionReader: io.NewSectionReader(f, off, n), f: f}, nil
}

// openRegular opens the regular file at name and returns it with its size; anything else at the
// path is app.ErrNotFound. os.Root follows a symbolic link inside the root even with O_NOFOLLOW,
// so the open is bracketed: inspect checks every component with Lstat before, and when the
// opened file is not the one inspect saw, the final component is checked again; a link there now
// is a storage failure, while another regular file means a concurrent Put replaced the object,
// which is a valid read of the new object.
func (s *Store) openRegular(name string) (*os.File, int64, error) {
	fi, err := s.inspect(name)
	if err != nil {
		return nil, 0, err
	}
	if fi == nil || !fi.Mode().IsRegular() {
		return nil, 0, app.ErrNotFound
	}
	s.event("before-open", name)
	f, err := s.root.OpenFile(osPath(name), os.O_RDONLY|oNonBlock, 0)
	switch {
	case errors.Is(err, iofs.ErrNotExist) || notDir(err):
		return nil, 0, app.ErrNotFound
	case err != nil:
		return nil, 0, err
	}
	st, err := f.Stat()
	if err == nil && !os.SameFile(fi, st) {
		err = s.stillRegular(name)
	}
	if err == nil && !st.Mode().IsRegular() {
		err = app.ErrNotFound
	}
	if err != nil {
		_ = f.Close()
		return nil, 0, err
	}
	return f, st.Size(), nil
}

// stillRegular checks the path of an object again after the open met another file than the
// check before: a symbolic link anywhere is errSymlink, a missing path app.ErrNotFound.
func (s *Store) stillRegular(name string) error {
	fi, err := s.inspect(name)
	switch {
	case err != nil:
		return err
	case fi == nil:
		return app.ErrNotFound
	}
	return nil
}

// Head returns the size of the regular file at key.
//
// Errors: app.ErrInvalidKey before any file system call; app.ErrNotFound when no regular file is
// at the key; a symbolic link anywhere in the path is a storage failure.
func (s *Store) Head(ctx context.Context, key string) (app.ObjectInfo, error) {
	if err := domain.ValidateKey(key); err != nil {
		return app.ObjectInfo{}, opErr("head", "", err)
	}
	if err := ctx.Err(); err != nil {
		return app.ObjectInfo{}, opErr("head", key, err)
	}
	fi, err := s.inspect(key)
	if err != nil {
		return app.ObjectInfo{}, opErr("head", key, err)
	}
	if fi == nil || !fi.Mode().IsRegular() {
		return app.ObjectInfo{}, opErr("head", key, app.ErrNotFound)
	}
	return app.ObjectInfo{Size: fi.Size()}, nil
}

// Delete removes the regular file at key, syncs its directory and then removes the directory
// f/<aa>/<bb>/<file_id> or uploads/<file_id> if it is empty now; the fan-out levels stay
// (docs/DECISIONS.md M3-01/11). For an original key it also removes uploads/<file_id> with any
// parts a completion could not remove, so deleting a file removes all its bytes (12 "12",
// M3-01/18). A missing object, a directory at the key and a directory that is not empty or
// already gone are no error (12 "12": a missing object counts as deleted).
//
// Errors: app.ErrInvalidKey before any file system call; a symbolic link anywhere in the path,
// the final component included, is a storage failure and nothing is removed.
func (s *Store) Delete(ctx context.Context, key string) error {
	if err := domain.ValidateKey(key); err != nil {
		return opErr("delete", "", err)
	}
	if err := ctx.Err(); err != nil {
		return opErr("delete", key, err)
	}
	fi, err := s.inspect(key)
	if err != nil {
		return opErr("delete", key, err)
	}
	if fi != nil && fi.Mode().IsRegular() {
		if err := s.removeObject(key); err != nil {
			return opErr("delete", key, err)
		}
	}
	if id, _, perr := domain.ParseOriginalKey(key); perr == nil {
		if err := s.dropUpload(id); err != nil {
			return opErr("delete", key, err)
		}
	}
	return nil
}

// removeObject unlinks the file at key, syncs its directory and removes the file directory when
// it is empty now.
func (s *Store) removeObject(key string) error {
	if err := s.root.Remove(osPath(key)); err != nil && !errors.Is(err, iofs.ErrNotExist) {
		return err
	}
	dir := path.Dir(key)
	if err := s.syncDir(dir); err != nil && !errors.Is(err, iofs.ErrNotExist) {
		return err
	}
	if fileDirectory(dir) {
		if err := s.root.Remove(osPath(dir)); err != nil && !errors.Is(err, iofs.ErrNotExist) && !notEmpty(err) {
			return err
		}
	}
	return nil
}

// fileDirectory reports whether dir is the directory of one file, f/<aa>/<bb>/<file_id> or
// uploads/<file_id>, which Delete removes when it is empty.
func fileDirectory(dir string) bool {
	segs := strings.Split(dir, "/")
	return (len(segs) == 4 && segs[0] == "f") || (len(segs) == 2 && segs[0] == "uploads")
}
