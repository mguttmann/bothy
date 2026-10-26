// SPDX-License-Identifier: Apache-2.0

package app

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"

	"github.com/OWNER/bothy/services/server/internal/files"
	"github.com/OWNER/bothy/services/server/internal/files/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Journal implements files.JournalObjects and files.JournalFiles over the files use cases (brief
// M7-07): the objects of the deletion journal of privacy in the object storage of the module, and
// the deletion of a file again that the journal names after a restore. It is safe for concurrent
// use.
type Journal struct {
	s *Service
}

// Compile-time checks: Journal is both ports privacy uses.
var (
	_ files.JournalObjects = (*Journal)(nil)
	_ files.JournalFiles   = (*Journal)(nil)
)

// NewJournal returns the ports over the files use cases s, which must hold the object store.
// Errors: a missing collaborator.
//
// Example:
//
//	j, err := app.NewJournal(svc)
func NewJournal(s *Service) (*Journal, error) {
	if s == nil || s.d.Store == nil || s.d.Cleanup == nil || s.d.UoW == nil {
		return nil, errors.New("files: NewJournal needs the use cases with their object store, cleanup store and unit of work")
	}
	return &Journal{s: s}, nil
}

// journalKey refuses every key outside the prefix of the journal or the key grammar.
func journalKey(key string) error {
	if !strings.HasPrefix(key, files.JournalKeyPrefix) || domain.ValidateKey(key) != nil {
		return errs.New(errs.CodeInternalError, "files: a key outside the deletion journal")
	}
	return nil
}

// Put implements files.JournalObjects.
func (j *Journal) Put(ctx context.Context, key string, body []byte) error {
	if err := journalKey(key); err != nil {
		return err
	}
	if len(body) > files.JournalObjectMaxBytes {
		return errs.New(errs.CodeInternalError, "files: a journal object above its bound")
	}
	if err := j.s.d.Store.Put(ctx, key, bytes.NewReader(body), int64(len(body))); err != nil {
		return storageError(err)
	}
	return nil
}

// Get implements files.JournalObjects: it reads at most one byte more than the bound, so a larger
// object is refused without being held.
func (j *Journal) Get(ctx context.Context, key string) ([]byte, bool, error) {
	if err := journalKey(key); err != nil {
		return nil, false, err
	}
	rc, err := j.s.d.Store.Get(ctx, key, nil)
	switch {
	case errors.Is(err, ErrNotFound):
		return nil, false, nil
	case err != nil:
		return nil, false, storageError(err)
	}
	defer func() { _ = rc.Close() }()
	body, err := io.ReadAll(io.LimitReader(rc, files.JournalObjectMaxBytes+1))
	if err != nil {
		return nil, false, storageError(err)
	}
	if len(body) > files.JournalObjectMaxBytes {
		return nil, false, errs.New(errs.CodeInternalError, "files: a journal object above its bound")
	}
	return body, true, nil
}

// Delete implements files.JournalObjects.
func (j *Journal) Delete(ctx context.Context, key string) error {
	if err := journalKey(key); err != nil {
		return err
	}
	if err := j.s.d.Store.Delete(ctx, key); err != nil {
		return storageError(err)
	}
	return nil
}

// DeleteForJournal implements files.JournalFiles: in one unit of work the row FOR UPDATE, then
// deletion, attachment and purpose read again and markDeleted, the path of deleteFile without an
// actor (05 file.delete writes no event and no audit).
func (j *Journal) DeleteForJournal(ctx context.Context, id ids.ID) (bool, error) {
	done := false
	err := j.s.d.UoW.Do(ctx, func(ctx context.Context) error {
		done = false
		found, err := j.s.d.Cleanup.LockFileForDelete(ctx, id)
		if err != nil || !found {
			return err
		}
		cur, found, err := j.s.d.Cleanup.CleanupRecord(ctx, id)
		if err != nil || !found || cur.Deleted || cur.Attached || cur.Purpose != purposeAttachment {
			return err
		}
		if err := j.s.markDeleted(ctx, Deletion{FileIDs: []ids.ID{id}}); err != nil {
			return err
		}
		done = true
		return nil
	})
	if err != nil {
		return false, err
	}
	return done, nil
}
