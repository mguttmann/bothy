// SPDX-License-Identifier: Apache-2.0

package domain

import "time"

// DueKinds returns the kinds whose schedule has a matching minute in the window (from, to], in
// the order of Kinds(); a kind without a schedule in schedules is never due. When pg_full is due,
// pg_diff is dropped: a full and a differential due together, or within one window, run only the
// full, because the full already contains what the differential would add (brief M7-02 design
// note 1).
//
// The window is how missed times are skipped: the scheduler passes the start of the process as
// the first from, so a time that passed while the process was down is never caught up.
//
// Example:
//
//	due := domain.DueKinds(schedules, prev, now) // for example [pg_full logical]
func DueKinds(schedules map[Kind]Schedule, from, to time.Time) []Kind {
	if !to.After(from) {
		return nil
	}
	var due []Kind
	full := false
	for _, k := range Kinds() {
		s, ok := schedules[k]
		if !ok {
			continue
		}
		next := s.Next(from)
		if next.IsZero() || next.After(to) {
			continue
		}
		if k == KindPGFull {
			full = true
		}
		if k == KindPGDiff && full {
			continue
		}
		due = append(due, k)
	}
	return due
}
