// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/importer"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// ItemRow is a row of import_items.
type ItemRow struct {
	ID, SourceID ids.ID
	// OriginKey and ItemKind name the source object; TargetKind and TargetID the created or
	// adopted object, empty while pending and for a skipped item.
	OriginKey, ItemKind, TargetKind string
	TargetID                        ids.ID
	// State is pending, posted, skipped or failed; Error "" for none.
	State, Error string
	// ContentSHA256 is the stored hash of the converted content; nil for none.
	ContentSHA256 []byte
	CreatedAt     time.Time
}

// NewItem is the pending row of a new origin key.
type NewItem struct {
	ID, SourceID        ids.ID
	OriginKey, ItemKind string
	ContentSHA256       []byte
	At                  time.Time
}

// SourceKey is the keyset position of listImports (created_at DESC, id DESC).
type SourceKey struct {
	At time.Time
	ID ids.ID
}

// Store is the outgoing port of the importer use cases over import_sources and import_items
// (implemented by adapters/postgres). Reads run in the ambient transaction of ctx when there is
// one and on the pool otherwise; writes and locks run only inside a unit of work and fail with
// kind internal outside one. Errors are kernel/errs errors without SQL text or values.
type Store interface {
	// InsertSource inserts the draft source s and returns the stored row.
	InsertSource(ctx context.Context, s importer.Source) (importer.Source, error)
	// Source returns the source id; found is false for an unknown id.
	Source(ctx context.Context, id ids.ID) (s importer.Source, found bool, err error)
	// LockSource returns the source id like Source and locks its row until the end of the unit.
	LockSource(ctx context.Context, id ids.ID) (s importer.Source, found bool, err error)
	// SaveSource writes state, config, stats, cursor, started_at and finished_at of s.
	SaveSource(ctx context.Context, s importer.Source) error
	// Sources returns at most limit sources after the position after (nil for the first page),
	// newest first.
	Sources(ctx context.Context, after *SourceKey, limit int) ([]importer.Source, error)
	// InsertPendingItem inserts the pending row of n; created is false when the key exists.
	InsertPendingItem(ctx context.Context, n NewItem) (row ItemRow, created bool, err error)
	// LockItem returns the item of the key and locks its row; found is false for none.
	LockItem(ctx context.Context, source ids.ID, originKey string) (row ItemRow, found bool, err error)
	// Item returns the item of the key; found is false for none.
	Item(ctx context.Context, source ids.ID, originKey string) (row ItemRow, found bool, err error)
	// FinishItem sets state, target and updated_at of the item id and returns the stored row.
	FinishItem(ctx context.Context, id ids.ID, state, targetKind string, targetID ids.ID, at time.Time) (ItemRow, error)
	// Items returns at most limit items of source after the id after (zero for the first page),
	// ascending, of one state ("" for every state).
	Items(ctx context.Context, source ids.ID, state string, after ids.ID, limit int) ([]ItemRow, error)
	// ItemsOfKind returns the items of kind in state of the source, ascending.
	ItemsOfKind(ctx context.Context, source ids.ID, kind, state string) ([]ItemRow, error)
	// ControlledChannel reports whether the channel is the target of a posted channel item of a
	// source that is neither completed nor failed.
	ControlledChannel(ctx context.Context, channelID ids.ID) (bool, error)
	// OriginOfTarget returns the oldest posted item of the target; found is false for none.
	OriginOfTarget(ctx context.Context, kind string, id ids.ID) (row ItemRow, found bool, err error)
}
