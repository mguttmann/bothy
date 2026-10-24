// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"fmt"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// purgeVariants are the four thumbnail variants of 12 "4.4 Key schema" that a purge deletes,
// whether a variant was ever written or not.
var purgeVariants = []Variant{VariantSmall, VariantMedium, VariantLarge, VariantPoster}

// ObjectKeysOf returns every key of the object store that may hold bytes of the file fileID
// whose row stores objectKey (12 section 12, the key schema of 12 4.4; docs/DECISIONS.md
// D-M3-08-4): the originals orig-1 to orig-(r+1), where r is the revision of objectKey, then the
// four variant keys. Revision r+1 covers an original that a processing run of files.thumbnail
// wrote but did not yet reference. Keys come only from the key schema and the row; nothing is
// listed (D-FI-11), and deleting a key that never existed is no error (12 4.1).
//
// The error wraps ErrInvalidKey when objectKey is not an original key or belongs to another file;
// the caller then deletes nothing.
//
// Example:
//
//	keys, err := domain.ObjectKeysOf(id, "f/5a/6b/<id>/orig-1")
//	// keys: orig-1, orig-2, small, medium, large, poster of id
func ObjectKeysOf(fileID ids.ID, objectKey string) ([]string, error) {
	owner, rev, err := ParseOriginalKey(objectKey)
	if err != nil {
		return nil, err
	}
	if owner != fileID {
		return nil, fmt.Errorf("%w: the original key belongs to another file", ErrInvalidKey)
	}
	keys := make([]string, 0, rev+1+len(purgeVariants))
	for n := 1; n <= rev+1; n++ {
		k, err := OriginalKey(fileID, n)
		if err != nil {
			return nil, err
		}
		keys = append(keys, k)
	}
	for _, v := range purgeVariants {
		k, err := VariantKey(fileID, v)
		if err != nil {
			return nil, err
		}
		keys = append(keys, k)
	}
	return keys, nil
}

// PurgeFacts are the columns of a files row that decide what files.purge_objects does, read
// without a transaction.
type PurgeFacts struct {
	// Exists reports that the row was found.
	Exists bool
	// Deleted reports deleted_at IS NOT NULL.
	Deleted                 bool
	UploadState, ScanStatus string
}

// PurgePlan is what one run of files.purge_objects does with a file.
type PurgePlan uint8

// The plans of PlanPurge.
const (
	// PurgeNothing leaves the file alone: no row, or a file that is live (upload running, scan
	// pending, deliverable).
	PurgeNothing PurgePlan = iota
	// PurgeObjects deletes the objects and parts of a failed or blocked file and keeps the row
	// for the uploader until file.failed_row_ttl.
	PurgeObjects
	// PurgeObjectsAndRow deletes the objects and parts of a deleted file and then its row
	// (03 principle 7, R-DM-46).
	PurgeObjectsAndRow
)

// PlanPurge decides what a run of files.purge_objects does (docs/DECISIONS.md D-M3-08-4): no row
// gives PurgeNothing, a deleted row PurgeObjectsAndRow in every state, a file that is final and
// not deliverable (upload failed, scan infected or error) PurgeObjects, and every other file
// PurgeNothing, so a stray job never touches a live file.
//
// Example:
//
//	domain.PlanPurge(domain.PurgeFacts{Exists: true, UploadState: "failed", ScanStatus: "pending"}) // PurgeObjects
func PlanPurge(f PurgeFacts) PurgePlan {
	switch {
	case !f.Exists:
		return PurgeNothing
	case f.Deleted:
		return PurgeObjectsAndRow
	case Final(f.UploadState, f.ScanStatus, false) && !IsDeliverable(f.UploadState, f.ScanStatus, false):
		return PurgeObjects
	default:
		return PurgeNothing
	}
}
