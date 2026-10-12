// SPDX-License-Identifier: Apache-2.0

package fs

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	iofs "io/fs"
	"os"
	"path"

	"github.com/OWNER/bothy/services/server/internal/files/app"
	"github.com/OWNER/bothy/services/server/internal/files/domain"
)

// copyChunk bounds the buffer of one write; the context is checked between chunks.
const copyChunk = 64 << 10

// errParentGone reports that the directory of a write without directory creation vanished
// before the rename: for a part, the upload was aborted or completed meanwhile.
var errParentGone = errors.New("parent directory vanished")

// Put writes key from exactly size bytes of body (docs/spec/12-dateien.md "4.2"): a temporary
// file in <root>/.tmp with mode 0600, a copy with a bounded buffer, Sync, the missing directories
// with mode 0700, rename to the key and Sync of the directory. A reader never sees half an
// object. Put on an existing key replaces it atomically (docs/DECISIONS.md M3-01/10).
//
// Errors: app.ErrInvalidKey before any file system call, app.ErrSizeMismatch for size < 0 or a
// body of another length, the context error when ctx ends; otherwise a storage failure. On every
// error the temporary file is removed and nothing is stored.
func (s *Store) Put(ctx context.Context, key string, body io.Reader, size int64) error {
	if err := domain.ValidateKey(key); err != nil {
		return opErr("put", "", err)
	}
	if size < 0 {
		return opErr("put", key, app.ErrSizeMismatch)
	}
	err := s.commit(ctx, key, true, func(w io.Writer) error { return copyExactly(ctx, w, body, size) })
	if err != nil {
		return opErr("put", key, err)
	}
	return nil
}

// commit writes the object key through a temporary file that fill writes, then syncs, renames
// and syncs the directory, in this order. With makeDirs it creates the missing directories of
// the key and creates them again when a concurrent Delete removed the emptied directory before
// the rename (place, docs/DECISIONS.md M3-01/11); without makeDirs a missing directory gives
// errParentGone. On every error the temporary file is removed.
func (s *Store) commit(ctx context.Context, key string, makeDirs bool, fill func(io.Writer) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if _, err := s.inspect(key); err != nil {
		return err
	}
	tmp, err := s.writeTemp(key, fill)
	if err != nil {
		return err
	}
	if err := s.place(ctx, tmp, key, makeDirs); err != nil {
		_ = s.root.Remove(osPath(tmp))
		return err
	}
	s.event("rename", key)
	// A concurrent Delete of this very key may have removed the directory again; the write
	// itself is complete then.
	if err := s.syncDir(path.Dir(key)); err != nil && !errors.Is(err, iofs.ErrNotExist) {
		return err
	}
	s.event("sync-dir", key)
	return nil
}

// writeTemp creates a temporary file with mode 0600 in .tmp, lets fill write it, syncs and
// closes it, and returns its name. On every error the file is closed and removed.
func (s *Store) writeTemp(key string, fill func(io.Writer) error) (tmp string, err error) {
	if ti, err := s.root.Lstat(tmpDir); err != nil || !ti.IsDir() || ti.Mode()&iofs.ModeSymlink != 0 {
		return "", fmt.Errorf("%s is missing or not a directory", tmpDir)
	}
	tmp, err = tempName()
	if err != nil {
		return "", err
	}
	f, err := s.root.OpenFile(osPath(tmp), os.O_WRONLY|os.O_CREATE|os.O_EXCL|oNoFollow, 0o600)
	if err != nil {
		return "", err
	}
	err = fill(f)
	if err == nil {
		s.event("write", key)
		err = f.Sync()
	}
	if err == nil {
		s.event("sync-file", key)
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		_ = s.root.Remove(osPath(tmp))
		return "", err
	}
	return tmp, nil
}

// rename moves the temporary file to the key after checking again that no component of the key
// is a symbolic link.
func (s *Store) rename(tmp, key string) error {
	if _, err := s.inspect(path.Dir(key)); err != nil {
		return err
	}
	return s.root.Rename(osPath(tmp), osPath(key))
}

// place renames the temporary file to the key. With makeDirs it first creates the missing
// directories of the key, and when the rename fails because a concurrent Delete removed the
// emptied file directory in between (docs/DECISIONS.md M3-01/11), it creates the directory again
// and renames again, until the rename succeeds, fails for another reason, the temporary file is
// gone or ctx ends. Each repetition needs another Delete that empties and removes the directory
// in that window, so the loop ends as soon as the deletes stop. Without makeDirs a vanished
// directory gives errParentGone.
func (s *Store) place(ctx context.Context, tmp, key string, makeDirs bool) error {
	dir := path.Dir(key)
	for {
		if makeDirs {
			if err := s.mkdirs(ctx, dir); err != nil {
				return err
			}
		}
		s.event("before-rename", key)
		err := s.rename(tmp, key)
		if err == nil {
			return nil
		}
		if err := s.retryable(ctx, tmp, dir, err, makeDirs); err != nil {
			return err
		}
	}
}

// retryable returns nil when place should create the directory again and repeat the rename
// that failed with err, and otherwise the error to return.
func (s *Store) retryable(ctx context.Context, tmp, dir string, err error, makeDirs bool) error {
	switch {
	case !s.vanished(dir, err):
		return err
	case !makeDirs:
		return errParentGone
	}
	if _, terr := s.root.Lstat(osPath(tmp)); terr != nil {
		return err
	}
	return ctx.Err()
}

// vanished reports whether a failed rename failed because the directory of the target is gone:
// the error is "not exist", the error is EINVAL (macOS renameat returns it instead of ENOENT when
// a concurrent rmdir removes the target directory during the call; the source is always a
// regular temporary file, so EINVAL has no other cause here), or the directory is missing now.
func (s *Store) vanished(dir string, err error) bool {
	if errors.Is(err, iofs.ErrNotExist) || renameRaced(err) {
		return true
	}
	fi, ierr := s.inspect(dir)
	return ierr == nil && fi == nil
}

// tempName returns a fresh name in .tmp; random, so two processes on one root never collide.
func tempName() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return tmpDir + "/w-" + hex.EncodeToString(b[:]), nil
}

// copyExactly copies exactly size bytes from body to w with a bounded buffer, checking ctx
// between chunks, then reads once more to detect a longer body. A shorter or longer body gives
// app.ErrSizeMismatch; an error of body is returned wrapped.
func copyExactly(ctx context.Context, w io.Writer, body io.Reader, size int64) error {
	buf := make([]byte, copyChunk)
	var done int64
	for empty := 0; done < size; {
		if err := ctx.Err(); err != nil {
			return err
		}
		n, err := body.Read(buf[:min(int64(len(buf)), size-done)])
		if n > 0 {
			empty = 0
			if _, werr := w.Write(buf[:n]); werr != nil {
				return werr
			}
			done += int64(n)
		} else if empty++; empty > maxEmptyReads {
			return io.ErrNoProgress
		}
		if err := readEnd(err, done < size); err != nil {
			return err
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return expectEOF(body)
}

// maxEmptyReads bounds reads that return neither bytes nor an error, as io.Reader allows but
// discourages; more of them in a row is io.ErrNoProgress.
const maxEmptyReads = 100

// readEnd maps the error of one read of the body: io.EOF before size bytes is
// app.ErrSizeMismatch, io.EOF after them is no error, anything else is the body's error.
func readEnd(err error, short bool) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, io.EOF):
		if short {
			return app.ErrSizeMismatch
		}
		return nil
	default:
		return fmt.Errorf("read body: %w", err)
	}
}

// expectEOF reads once more after size bytes: one more byte is app.ErrSizeMismatch.
func expectEOF(body io.Reader) error {
	var one [1]byte
	for range maxEmptyReads {
		n, err := body.Read(one[:])
		if n > 0 {
			return app.ErrSizeMismatch
		}
		if err != nil {
			return readEnd(err, false)
		}
	}
	return io.ErrNoProgress
}
