// SPDX-License-Identifier: Apache-2.0

package files

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// JournalKeyPrefix is the only prefix JournalObjects accepts: the objects of the deletion journal
// of docs/spec/20-privacy.md section 4.6 (M7-07). The key schema of 12 section 4.4 never produces
// it, so a journal object never shares a key with a file, a thumbnail or a part, and the object
// backup copies it with the files (docs/operations/backup-restore.md 1).
const JournalKeyPrefix = "deletion-journal/"

// JournalObjectMaxBytes bounds one journal object: an entry holds five short members. Get refuses a
// larger object instead of reading it into memory.
const JournalObjectMaxBytes = 4096

// JournalObjects is the inbound port through which the module privacy stores and reads the
// objects of the deletion journal in the object storage of files (brief M7-07 design note 2).
// Every key must start with JournalKeyPrefix and follow the key grammar of files
// (domain.ValidateKey); any other key is refused before a storage call. The port has no listing
// and no conditional write, like the object storage itself (12 section 4.1); privacy numbers its
// objects densely instead. Every method runs outside a unit of work, because it calls the storage
// (R-AR-07), except where privacy documents its exception. Errors are kernel/errs errors: internal
// for a refused key or an object above JournalObjectMaxBytes, unavailable for the storage.
//
// Example:
//
//	err := objs.Put(ctx, "deletion-journal/0000000000000000001", body)
//	body, found, err := objs.Get(ctx, "deletion-journal/0000000000000000001")
type JournalObjects interface {
	// Put writes the object key with body; an existing object is replaced, so the caller checks
	// with Get first (privacy never overwrites a journal object).
	Put(ctx context.Context, key string, body []byte) error
	// Get returns the bytes of the object key; found is false for a missing object.
	Get(ctx context.Context, key string) (body []byte, found bool, err error)
	// Delete removes the object key; a missing object counts as deleted (12 section 12).
	Delete(ctx context.Context, key string) error
}

// JournalFiles is the inbound port through which bothy restore finalize deletes a file again that
// the deletion journal names and that the restored database holds (SEC-BAK-3, brief M7-07 design
// note 3).
type JournalFiles interface {
	// DeleteForJournal deletes the file id through the deletion path of 20 section 4.3 without an
	// actor (deleted_at, redaction of its file.updated events, files.purge_objects queued) and
	// reports whether it did. An unknown or deleted file, an attached file (it goes with its
	// message) and a file whose purpose is not attachment (an avatar, a logo or an export package
	// goes through its own operation) are left alone and give false. It runs its own unit of
	// work.
	DeleteForJournal(ctx context.Context, id ids.ID) (bool, error)
}
