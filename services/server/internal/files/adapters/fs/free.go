// SPDX-License-Identifier: Apache-2.0

package fs

import (
	"context"
	"errors"
)

// FreeBytes returns the free space of the file system of file.fs.root (statfs; a local system
// call, not a network call), for the reserve file.fs.min_free_bytes of createUpload
// (docs/spec/12-files.md 3.1). It is not part of app.ObjectStore; the files module reads it
// through its port app.FreeSpace. The error names no path.
//
// Example:
//
//	free, err := store.FreeBytes(ctx)
func (s *Store) FreeBytes(ctx context.Context) (int64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	n, err := freeBytes(s.root.Name())
	if err != nil {
		return 0, errors.New("files/fs: free space of file.fs.root unavailable")
	}
	return n, nil
}
