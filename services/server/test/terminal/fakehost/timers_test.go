// SPDX-License-Identifier: Apache-2.0

package fakehost

import (
	"testing"
	"time"
)

// TestTimers_StartedSinceCountsTimersOfOneDurationAfterTheMark (PL/47): a timer of the duration
// started after the mark counts, also when it stopped again and when a timer started before the
// mark stopped meanwhile, which leaves Pending at its old value; timers of other durations and
// timers started before the mark do not count; Fire keeps the record.
func TestTimers_StartedSinceCountsTimersOfOneDurationAfterTheMark(t *testing.T) {
	const stall, deadline = 10 * time.Second, 20 * time.Second
	var tm Timers
	stopOld := tm.AfterFunc(stall, func() {})
	before, mark := tm.Pending(), tm.Mark()
	stopOld()
	stopNew := tm.AfterFunc(stall, func() {})
	if got := tm.Pending(); got != before {
		t.Fatalf("Pending %d, want %d (one stopped, one started)", got, before)
	}
	stopNew()
	tm.AfterFunc(deadline, func() {})
	if got := tm.StartedSince(mark, stall); got != 1 {
		t.Fatalf("StartedSince %d, want 1 (the stopped timer started after the mark)", got)
	}
	tm.AfterFunc(stall, func() {})
	if n := tm.Fire(); n != 2 {
		t.Fatalf("Fire ran %d, want 2", n)
	}
	if got := tm.StartedSince(mark, stall); got != 2 {
		t.Fatalf("StartedSince %d after Fire, want 2", got)
	}
	if got := tm.StartedSince(tm.Mark(), stall); got != 0 {
		t.Fatalf("StartedSince of a fresh mark %d, want 0", got)
	}
}
