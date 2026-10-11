// SPDX-License-Identifier: Apache-2.0

package clock_test

import (
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
)

func TestSystemReturnsUTC(t *testing.T) {
	before := time.Now()
	got := clock.System().Now()
	after := time.Now()

	if got.Location() != time.UTC {
		t.Fatalf("System().Now() location = %v, want UTC", got.Location())
	}
	if got.Before(before.Truncate(time.Microsecond)) || got.After(after.Add(time.Microsecond)) {
		t.Fatalf("System().Now() = %v, want between %v and %v", got, before, after)
	}
}

func TestFakeSetAndAdvance(t *testing.T) {
	plusTwo := time.FixedZone("UTC+2", 2*60*60)
	start := time.Date(2026, 10, 11, 10, 0, 0, 0, plusTwo)
	f := clock.NewFake(start)

	if got := f.Now(); !got.Equal(start) || got.Location() != time.UTC {
		t.Fatalf("NewFake(%v).Now() = %v, want the same instant in UTC", start, got)
	}

	f.Advance(90 * time.Second)
	if want := start.Add(90 * time.Second); !f.Now().Equal(want) {
		t.Fatalf("after Advance(90s) Now() = %v, want %v", f.Now(), want)
	}

	f.Advance(-30 * time.Second)
	if want := start.Add(60 * time.Second); !f.Now().Equal(want) {
		t.Fatalf("after Advance(-30s) Now() = %v, want %v", f.Now(), want)
	}

	earlier := time.Date(2020, 1, 2, 3, 4, 5, 6, plusTwo)
	f.Set(earlier)
	if got := f.Now(); !got.Equal(earlier) || got.Location() != time.UTC {
		t.Fatalf("after Set(%v) Now() = %v, want the same instant in UTC", earlier, got)
	}

	var c clock.Clock = f
	if !c.Now().Equal(earlier) {
		t.Fatalf("Fake used as Clock returns %v, want %v", c.Now(), earlier)
	}
}

func TestFakeConcurrentUse(t *testing.T) {
	start := time.Date(2026, 10, 11, 0, 0, 0, 0, time.UTC)
	f := clock.NewFake(start)

	const goroutines, steps = 16, 500
	var wg sync.WaitGroup
	for range goroutines {
		wg.Go(func() {
			for range steps {
				f.Advance(time.Millisecond)
				_ = f.Now()
			}
		})
	}
	wg.Wait()

	want := start.Add(goroutines * steps * time.Millisecond)
	if got := f.Now(); !got.Equal(want) {
		t.Fatalf("after %d concurrent Advance calls Now() = %v, want %v", goroutines*steps, got, want)
	}
}

func TestFakeConcurrentSetAdvanceNow(t *testing.T) {
	// Set, Advance and Now from many goroutines at once; run with -race, which reports a write
	// in Set or Advance that the mutex does not guard.
	start := time.Date(2026, 10, 11, 0, 0, 0, 0, time.UTC)
	plusTwo := time.FixedZone("UTC+2", 2*60*60)
	f := clock.NewFake(start)

	const goroutines, steps = 8, 500
	latest := start.Add(goroutines * time.Hour)
	var wg sync.WaitGroup
	for g := range goroutines {
		wg.Go(func() {
			// Each setter stores its own instant, given in another zone.
			at := start.Add(time.Duration(g) * time.Hour).In(plusTwo)
			for range steps {
				f.Set(at)
			}
		})
		wg.Go(func() {
			for range steps {
				f.Advance(time.Millisecond)
				f.Advance(-time.Millisecond)
			}
		})
		wg.Go(func() {
			for range steps {
				// Advance moves by at most goroutines milliseconds away from a stored instant.
				now := f.Now()
				if now.Location() != time.UTC || now.Before(start.Add(-goroutines*time.Millisecond)) ||
					now.After(latest) {
					t.Errorf("concurrent Now() = %v, want a UTC instant between %v and %v", now, start, latest)
					return
				}
			}
		})
	}
	wg.Wait()

	f.Set(start)
	if got := f.Now(); !got.Equal(start) {
		t.Fatalf("after the concurrent calls Set(%v) gave Now() = %v", start, got)
	}
}
