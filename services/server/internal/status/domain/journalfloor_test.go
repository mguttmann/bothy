// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/status/domain"
)

// TestJournalFloor: the floor is the smallest journal number of the kept runs; logical and objects
// runs are kept for their retention, the newest full backups by count with the differential
// backups after the oldest of them; a kept run without the number keeps everything.
func TestJournalFloor(t *testing.T) {
	t.Parallel()
	const day = 24 * time.Hour
	now := time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)
	ago := func(d time.Duration) time.Time { return now.Add(-d) }
	keep := domain.Keep{Logical: 30 * day, Objects: 90 * day, FullCount: 2}
	run := func(k domain.Kind, age time.Duration, next int64) domain.KeptRun {
		return domain.KeptRun{Kind: k, StartedAt: ago(age), FinishedAt: ago(age - time.Hour), JournalNext: next}
	}
	cases := []struct {
		name    string
		runs    []domain.KeptRun
		floor   int64
		limited bool
	}{
		{"none", nil, 0, false},
		{"expired logical", []domain.KeptRun{run(domain.KindLogical, 31*day, 3)}, 0, false},
		{"kept logical", []domain.KeptRun{run(domain.KindLogical, 29*day, 3), run(domain.KindLogical, day, 9)}, 3, true},
		{"objects longer", []domain.KeptRun{run(domain.KindObjects, 89*day, 2), run(domain.KindLogical, 89*day, 1)}, 2, true},
		{"fulls by count", []domain.KeptRun{
			run(domain.KindPGFull, 30*day, 1), run(domain.KindPGFull, 20*day, 5), run(domain.KindPGFull, 10*day, 7),
		}, 5, true},
		{"diff of an expired full", []domain.KeptRun{
			run(domain.KindPGFull, 30*day, 1), run(domain.KindPGDiff, 25*day, 2), run(domain.KindPGFull, 20*day, 5),
			run(domain.KindPGDiff, 15*day, 6), run(domain.KindPGFull, 10*day, 7),
		}, 5, true},
		{"old full kept by count", []domain.KeptRun{run(domain.KindPGFull, 400*day, 4)}, 4, true},
		{"run before M7-07", []domain.KeptRun{run(domain.KindLogical, day, 0), run(domain.KindLogical, 2*day, 9)}, 1, true},
		{"restore test ignored", []domain.KeptRun{run(domain.KindRestoreTest, day, 1), run(domain.KindLogical, day, 9)}, 9, true},
	}
	for _, c := range cases {
		floor, limited := domain.JournalFloor(c.runs, keep, now)
		if floor != c.floor || limited != c.limited {
			t.Errorf("%s: JournalFloor = %d, %v; want %d, %v", c.name, floor, limited, c.floor, c.limited)
		}
	}
}
