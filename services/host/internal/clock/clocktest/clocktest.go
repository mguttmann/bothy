// SPDX-License-Identifier: Apache-2.0

// Package clocktest is the fake clock.Clock of the tests: time moves only with Advance, and a
// timer fires when Advance reaches its deadline.
package clocktest

import (
	"sync"
	"time"

	"github.com/OWNER/bothy/services/host/internal/clock"
)

// Fake is a clock.Clock whose time moves only with Advance. The zero value is not usable; call New.
type Fake struct {
	mu     sync.Mutex
	now    time.Time
	timers []*timer
}

// New returns a Fake that starts at start.
func New(start time.Time) *Fake { return &Fake{now: start} }

// Now implements clock.Clock.
func (f *Fake) Now() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.now
}

// NewTimer implements clock.Clock; the timer fires on the Advance that reaches now+d.
func (f *Fake) NewTimer(d time.Duration) clock.Timer {
	f.mu.Lock()
	defer f.mu.Unlock()
	t := &timer{f: f, at: f.now.Add(d), c: make(chan time.Time, 1)}
	if d <= 0 {
		t.fired = true
		t.c <- f.now
		return t
	}
	f.timers = append(f.timers, t)
	return t
}

// Advance moves the time forward by d and fires every timer whose deadline it reaches.
func (f *Fake) Advance(d time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.now = f.now.Add(d)
	kept := f.timers[:0]
	for _, t := range f.timers {
		if !t.at.After(f.now) {
			t.fired = true
			t.c <- f.now
			continue
		}
		kept = append(kept, t)
	}
	f.timers = kept
}

// Pending returns the number of timers that have neither fired nor stopped; tests wait for a
// timer to exist before they advance.
func (f *Fake) Pending() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.timers)
}

type timer struct {
	f     *Fake
	at    time.Time
	c     chan time.Time
	fired bool
}

func (t *timer) C() <-chan time.Time { return t.c }

func (t *timer) Stop() bool {
	t.f.mu.Lock()
	defer t.f.mu.Unlock()
	if t.fired {
		return false
	}
	for i, o := range t.f.timers {
		if o == t {
			t.f.timers = append(t.f.timers[:i], t.f.timers[i+1:]...)
			return true
		}
	}
	return false
}
