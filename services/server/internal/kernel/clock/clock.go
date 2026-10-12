// SPDX-License-Identifier: Apache-2.0

// Package clock is the single source of the current time for the server.
//
// Code that needs "now" takes a [Clock] instead of calling [time.Now], so tests can pin and move
// time with [Fake] and every timestamp the server produces is in UTC (docs/spec/03-data-model.md,
// "Principles" item 2: every instant is stored in UTC). The fake lives in this package rather than
// in a test helper package so that the kernel-core component of the architecture gate keeps exact
// package paths.
//
// Example:
//
//	c := clock.NewFake(time.Date(2026, 10, 11, 9, 0, 0, 0, time.UTC))
//	c.Advance(time.Minute)
//	_ = c.Now() // 2026-10-11 09:01:00 UTC
package clock

import (
	"sync"
	"time"
)

// Clock reports the current time.
//
// Implementations return UTC and are safe for concurrent use. The name stays Clock rather than
// the -er form of its one method (docs/DECISIONS.md M1-05/26).
type Clock interface { // NOSONAR: a clock is a thing, not an action; see M1-05/26.
	// Now returns the current instant in UTC.
	Now() time.Time
}

// systemClock reads the operating system clock.
type systemClock struct{}

// Now returns the wall clock time in UTC.
func (systemClock) Now() time.Time { return time.Now().UTC() }

// System returns the clock of the operating system.
//
// Its Now converts to UTC so that callers never format or store a local zone by accident. The
// returned value is stateless and safe for concurrent use.
func System() Clock { return systemClock{} }

// Fake is a [Clock] whose time only changes through [Fake.Set] and [Fake.Advance].
//
// It is meant for tests and is safe for concurrent use: a mutex guards the stored instant, so
// one goroutine may advance the fake while others read it.
type Fake struct {
	mu  sync.Mutex
	now time.Time
}

// NewFake returns a fake clock that reports t, converted to UTC, until it is changed.
func NewFake(t time.Time) *Fake {
	return &Fake{now: t.UTC()}
}

// Now returns the stored instant in UTC.
func (f *Fake) Now() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.now
}

// Set replaces the stored instant with t, converted to UTC.
//
// Set may move the clock backwards; tests use that to check code that must cope with a system
// clock that jumps back.
func (f *Fake) Set(t time.Time) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.now = t.UTC()
}

// Advance moves the stored instant by d; a negative d moves it backwards.
func (f *Fake) Advance(d time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.now = f.now.Add(d)
}
