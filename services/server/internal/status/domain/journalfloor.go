// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"slices"
	"time"
)

// KeptRun is a succeeded backup run as the journal floor reads it (M7-07): its kind, its times and
// the deletion_journal.next_number of its manifest, 0 when the manifest has none (a run before
// M7-07).
type KeptRun struct {
	Kind       Kind
	StartedAt  time.Time
	FinishedAt time.Time
	// JournalNext is deletion_journal.next_number of the manifest; 0 for none.
	JournalNext int64
}

// Keep are the retention rules of the backups (docs/spec/16-operations.md 8.1).
type Keep struct {
	// Logical is backup.logical.retention, Objects backup.objects.retention: a snapshot is kept
	// while its run finished less than this long ago.
	Logical, Objects time.Duration
	// FullCount is the number of full pgBackRest backups kept (the larger of
	// backup.pg.retention_full and backup.pg.local_retention_full); a differential backup is
	// kept with the oldest kept full backup it follows.
	FullCount int
}

// JournalFloor returns the smallest deletion_journal.next_number among the runs of runs that are
// still kept at now under k (03 4.12, brief M7-07 requirement 16): the retention of the deletion
// journal removes no object at or above it, so a restore of any kept backup finds its first
// object. A kept run without the number gives 1, which keeps every object until that run expires.
// limited is false when no run is kept: then no backup needs the journal.
//
// Example:
//
//	floor, limited := domain.JournalFloor(runs, domain.Keep{Logical: 30 * day, Objects: 90 * day, FullCount: 4}, now)
func JournalFloor(runs []KeptRun, k Keep, now time.Time) (floor int64, limited bool) {
	consider := func(r KeptRun) {
		n := r.JournalNext
		if n < 1 {
			n = 1
		}
		if !limited || n < floor {
			floor = n
		}
		limited = true
	}
	var fulls []KeptRun
	var diffs []KeptRun
	for _, r := range runs {
		switch r.Kind {
		case KindLogical:
			if now.Sub(r.FinishedAt) < k.Logical {
				consider(r)
			}
		case KindObjects:
			if now.Sub(r.FinishedAt) < k.Objects {
				consider(r)
			}
		case KindPGFull:
			fulls = append(fulls, r)
		case KindPGDiff:
			diffs = append(diffs, r)
		}
	}
	slices.SortFunc(fulls, func(a, b KeptRun) int { return b.StartedAt.Compare(a.StartedAt) })
	if len(fulls) > k.FullCount {
		fulls = fulls[:max(k.FullCount, 0)]
	}
	for _, r := range fulls {
		consider(r)
	}
	if len(fulls) > 0 {
		oldest := fulls[len(fulls)-1].StartedAt
		for _, r := range diffs {
			if r.StartedAt.After(oldest) {
				consider(r)
			}
		}
	}
	return floor, limited
}
