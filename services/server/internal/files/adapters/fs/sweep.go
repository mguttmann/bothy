// SPDX-License-Identifier: Apache-2.0

package fs

import (
	"context"
	"errors"
	iofs "io/fs"
	"path"
	"time"

	"github.com/OWNER/bothy/services/server/internal/files/app"
)

// Compile-time check: the store sweeps its temporary files for the cleanup (M3-08).
var _ app.TempSweeper = (*Store)(nil)

// SweepTemp implements app.TempSweeper (docs/spec/12-files.md 4.2, brief M3-08 design note 6 e):
// it removes the entries of <root>/.tmp whose modification time is before olderThan and returns
// how many it removed. A writer changes its temporary file with every chunk and ends with a rename
// or a removal, and no write outlives an upload session, so an entry older than
// file.upload_session_ttl belongs to a crashed process (docs/decisions.d/M3-08.md). The sweep
// reads only .tmp, never the key space (12 4.1), goes through the os.Root of the store and never
// follows a symbolic link: a link is removed as an entry. An entry another process removed
// meanwhile is skipped.
//
// Errors: a cancelled ctx, and the first failure to read .tmp or to remove an entry (the other
// entries are still tried).
//
// Example:
//
//	n, err := store.SweepTemp(ctx, now.Add(-sessionTTL))
func (s *Store) SweepTemp(ctx context.Context, olderThan time.Time) (int, error) {
	dir, err := s.root.Open(tmpDir)
	if err != nil {
		return 0, opErr("sweep temporary files", "", err)
	}
	entries, err := dir.ReadDir(-1)
	_ = dir.Close()
	if err != nil {
		return 0, opErr("sweep temporary files", "", err)
	}
	n := 0
	var first error
	for _, e := range entries {
		if err := ctx.Err(); err != nil {
			return n, err
		}
		name := path.Join(tmpDir, e.Name())
		fi, err := s.root.Lstat(osPath(name))
		if errors.Is(err, iofs.ErrNotExist) {
			continue
		}
		if err == nil && !fi.ModTime().Before(olderThan) {
			continue
		}
		if err == nil {
			err = s.root.Remove(osPath(name))
		}
		switch {
		case err == nil:
			n++
		case errors.Is(err, iofs.ErrNotExist):
		case first == nil:
			first = opErr("sweep temporary files", "", err)
		}
	}
	return n, first
}
