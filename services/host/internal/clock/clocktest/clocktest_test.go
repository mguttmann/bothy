// SPDX-License-Identifier: Apache-2.0

package clocktest

import (
	"testing"
	"time"
)

func TestFakeTimer(t *testing.T) {
	start := time.Date(2026, 10, 15, 9, 0, 0, 0, time.UTC)
	f := New(start)
	a := f.NewTimer(time.Minute)
	b := f.NewTimer(2 * time.Minute)
	if f.Pending() != 2 {
		t.Fatalf("Pending = %d", f.Pending())
	}
	f.Advance(time.Minute)
	select {
	case got := <-a.C():
		if !got.Equal(start.Add(time.Minute)) {
			t.Errorf("fired at %v", got)
		}
	default:
		t.Fatal("timer a did not fire")
	}
	if !b.Stop() || b.Stop() {
		t.Error("Stop of a pending timer must report true once")
	}
	if a.Stop() {
		t.Error("Stop of a fired timer reported true")
	}
	z := f.NewTimer(0)
	<-z.C()
	if !f.Now().Equal(start.Add(time.Minute)) {
		t.Errorf("Now = %v", f.Now())
	}
}
