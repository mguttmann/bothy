// SPDX-License-Identifier: Apache-2.0

package secrets

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

// TempPrefix starts the name of every temporary file the file driver creates while it writes a
// secret (Put) and of the probe of CheckSigningKeyDir. A crash can leave such a file behind; it is
// never read as a secret, and bothy secrets export skips it (the same prefix as the temp files of
// bothy secrets init).
const TempPrefix = ".bothy-tmp-"

// Compile-time check: the file driver writes.
var _ Writer = (*fileDriver)(nil)

// Put stores v in the file ref names (Writer.Put, ADR 0066 rule 2): it creates a file with mode
// 0600 under a temporary name in the same directory, writes and syncs it, links it to the final
// name without overwriting (an existing name is ErrExists), removes the temporary name and syncs
// the directory, so the secret is complete and durable or absent after a crash. Errors name the
// reference, never the value.
func (d *fileDriver) Put(ctx context.Context, ref Ref, v Value) (Version, error) {
	if err := checkDriverRef(ctx, schemeFile, ref); err != nil {
		return "", err
	}
	name := ref.String()
	if platformProblem != "" {
		return "", &RefError{Ref: name, Err: ErrWriteFailed, Detail: platformProblem}
	}
	dir := filepath.Dir(ref.Path)
	content := v.content()
	tmp, err := os.CreateTemp(dir, TempPrefix+filepath.Base(ref.Path)+"-*")
	if err != nil {
		return "", writeError(name, err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	err = errors.Join(tmp.Chmod(modeOwnerReadWrite), write(tmp, content), tmp.Sync())
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return "", writeError(name, err)
	}
	// link fails with EEXIST instead of replacing the target, unlike rename.
	if err := os.Link(tmpName, ref.Path); err != nil {
		if errors.Is(err, fs.ErrExist) {
			return "", &RefError{Ref: name, Err: ErrExists, Detail: "file exists and is never overwritten"}
		}
		return "", writeError(name, err)
	}
	if err := os.Remove(tmpName); err != nil {
		return "", writeError(name, err)
	}
	if err := syncDir(dir); err != nil {
		return "", writeError(name, err)
	}
	return versionOf(content), nil
}

// Delete removes the file ref names and syncs its directory (Writer.Delete). A missing file is
// ErrNotFound.
func (d *fileDriver) Delete(ctx context.Context, ref Ref) error {
	if err := checkDriverRef(ctx, schemeFile, ref); err != nil {
		return err
	}
	name := ref.String()
	if platformProblem != "" {
		return &RefError{Ref: name, Err: ErrWriteFailed, Detail: platformProblem}
	}
	if err := os.Remove(ref.Path); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return &RefError{Ref: name, Err: ErrNotFound, Detail: "file does not exist"}
		}
		return writeError(name, err)
	}
	if err := syncDir(filepath.Dir(ref.Path)); err != nil {
		return writeError(name, err)
	}
	return nil
}

// write writes all of b to f.
func write(f *os.File, b []byte) error {
	_, err := f.Write(b)
	return err
}

// syncDir syncs the directory dir, so a created, linked or removed name survives a crash.
func syncDir(dir string) error {
	f, err := os.Open(dir) // #nosec G304 -- the directory of a checked absolute secret reference.
	if err != nil {
		return err
	}
	return errors.Join(f.Sync(), f.Close())
}

// writeError maps an error of a write step to ErrWriteFailed with the cause of the system call as
// detail, never a path or a value.
func writeError(name string, err error) *RefError {
	detail := "write error"
	var pathErr *fs.PathError
	var linkErr *os.LinkError
	switch {
	case errors.As(err, &pathErr):
		detail = pathErr.Err.Error()
	case errors.As(err, &linkErr):
		detail = linkErr.Err.Error()
	}
	return &RefError{Ref: name, Err: ErrWriteFailed, Detail: detail}
}
