// SPDX-License-Identifier: Apache-2.0

package fakehost

import (
	"sort"
	gosync "sync"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
)

// ClockTimers is the deadline scheduler of the M2-46 tests: a fake of terminal/app.Timers on a fake
// clock. AfterFunc records the due instant (the clock's now plus d); Advance moves the clock and
// runs every timer whose instant is reached, in the order of their instants, one after the other in
// the caller's goroutine, so a test sees the effects when Advance returns. A timer whose duration is
// not positive runs at once in its own goroutine (its caller may hold a lock). No test sleeps for a
// window. Jump moves the clock without running timers, and until the next Advance a timer whose
// duration is not positive waits for that Advance too, so no deadline runs after a jump even when
// its timer is armed late. It is safe for concurrent use.
type ClockTimers struct {
	// Clock is the fake clock the server runs on.
	Clock *clock.Fake

	mu     gosync.Mutex
	list   []*clockTimer
	frozen bool
}

// clockTimer is one scheduled function.
type clockTimer struct {
	due     time.Time
	f       func()
	stopped bool
	fired   bool
}

// NewClockTimers returns a scheduler on c.
func NewClockTimers(c *clock.Fake) *ClockTimers { return &ClockTimers{Clock: c} }

// AfterFunc implements terminal/app.Timers.
func (t *ClockTimers) AfterFunc(d time.Duration, f func()) func() bool {
	t.mu.Lock()
	tm := &clockTimer{due: t.Clock.Now().Add(d), f: f}
	now := d <= 0 && !t.frozen
	if now {
		tm.fired = true
	} else {
		t.list = append(t.list, tm)
	}
	t.mu.Unlock()
	if now {
		go f()
	}
	return func() bool {
		t.mu.Lock()
		defer t.mu.Unlock()
		was := !tm.stopped && !tm.fired
		tm.stopped = true
		return was
	}
}

// Advance moves the clock by d and runs the due timers (also those a fired timer schedules within
// the new now); it returns how many ran.
func (t *ClockTimers) Advance(d time.Duration) int {
	t.mu.Lock()
	t.frozen = false
	t.mu.Unlock()
	t.Clock.Advance(d)
	n := 0
	for {
		tm := t.next()
		if tm == nil {
			return n
		}
		tm.f()
		n++
	}
}

// Jump moves the clock by d without running a timer; until the next Advance no timer runs, also
// none armed meanwhile with a duration that is not positive (a dead relay in the tests).
func (t *ClockTimers) Jump(d time.Duration) {
	t.Freeze()
	t.Clock.Advance(d)
}

// Freeze makes a timer armed with a duration that is not positive wait for the next Advance
// instead of running at once, for a scheduler on a clock another scheduler jumps.
func (t *ClockTimers) Freeze() {
	t.mu.Lock()
	t.frozen = true
	t.mu.Unlock()
}

// Thaw ends a Freeze without running a timer.
func (t *ClockTimers) Thaw() {
	t.mu.Lock()
	t.frozen = false
	t.mu.Unlock()
}

// next takes the earliest due timer that is reached, nil for none.
func (t *ClockTimers) next() *clockTimer {
	t.mu.Lock()
	defer t.mu.Unlock()
	now := t.Clock.Now()
	live := t.list[:0]
	for _, tm := range t.list {
		if !tm.stopped && !tm.fired {
			live = append(live, tm)
		}
	}
	t.list = live
	sort.SliceStable(t.list, func(i, j int) bool { return t.list[i].due.Before(t.list[j].due) })
	if len(t.list) == 0 || t.list[0].due.After(now) {
		return nil
	}
	tm := t.list[0]
	tm.fired = true
	return tm
}

// Pending returns the number of timers not yet fired or stopped.
func (t *ClockTimers) Pending() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	n := 0
	for _, tm := range t.list {
		if !tm.stopped && !tm.fired {
			n++
		}
	}
	return n
}
