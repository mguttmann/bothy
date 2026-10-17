// SPDX-License-Identifier: Apache-2.0

// Package clock is the time source of the host service. Production code takes a Clock instead of
// calling time.Now or time.NewTimer, so tests drive every window with clocktest.Fake and never
// sleep for it.
package clock

import "time"

// Clock tells the time and makes timers.
type Clock interface {
	// Now returns the current time.
	Now() time.Time
	// NewTimer returns a timer that fires once after d.
	NewTimer(d time.Duration) Timer
}

// Timer is a one-shot timer; C delivers the time once it fired.
type Timer interface {
	// C returns the channel the timer sends on when it fires.
	C() <-chan time.Time
	// Stop prevents the timer from firing; it reports whether it stopped a pending timer.
	Stop() bool
}

// Real returns the Clock of the operating system.
func Real() Clock { return realClock{} }

type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }

func (realClock) NewTimer(d time.Duration) Timer { return realTimer{time.NewTimer(d)} }

type realTimer struct{ t *time.Timer }

func (r realTimer) C() <-chan time.Time { return r.t.C }
func (r realTimer) Stop() bool          { return r.t.Stop() }
