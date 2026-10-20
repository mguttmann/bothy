// SPDX-License-Identifier: Apache-2.0

package wsrelay

import (
	"slices"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/terminal/app"
	"github.com/OWNER/bothy/services/server/internal/terminal/domain"
	"github.com/OWNER/bothy/services/server/test/terminal/fakehost"
)

// The windows of the limit tests: short test values, not the product defaults.
var limitWindows = domain.Windows{Idle: time.Minute, IdleGrace: 10 * time.Second, MaxDuration: time.Hour, DetachGrace: 30 * time.Second,
	OpenTimeout: 20 * time.Second}

// tick is the step one instant before a deadline; farDeadline keeps the opening deadline of M2-44
// out of the way (the sides of these tests have no connection).
const (
	tick        = time.Millisecond
	farDeadline = 48 * time.Hour
)

// limitRelay is a relay with the limits on a fake clock and its scheduler.
func limitRelay(t *testing.T) (*Relay, *fakeRelayUC, *fakehost.ClockTimers) {
	t.Helper()
	clk := clock.NewFake(time.Date(2026, 10, 20, 9, 0, 0, 0, time.UTC))
	timers := fakehost.NewClockTimers(clk)
	uc := &fakeRelayUC{}
	r, err := NewRelay(RelayConfig{BufferMaxBytes: 1 << 16, OpenTimeout: limitWindows.OpenTimeout, Windows: limitWindows}, uc, nil, timers, clk, nil)
	if err != nil {
		t.Fatal(err)
	}
	return r, uc, timers
}

// ends returns the RelayEnd calls so far.
func (f *fakeRelayUC) ends() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for _, c := range f.calls {
		if len(c) > 4 && c[:4] == "end:" {
			out = append(out, c)
		}
	}
	return out
}

// TestRelayLimits_IdleAtInstant (M2-46 Design note 7 a and c, TM-T11): without device bytes the
// relay ends a held session at the last reference plus terminal.idle_timeout plus
// terminal.idle_grace, not one tick before; a device message moves the deadline.
func TestRelayLimits_IdleAtInstant(t *testing.T) {
	t.Parallel()
	r, uc, timers := limitRelay(t)
	t0 := timers.Clock.Now()
	id := ids.New()
	r.Expect(app.Expectation{SessionID: id, HostID: ids.New(), DeviceID: ids.New(), Deadline: t0.Add(farDeadline), Started: true,
		Times: domain.SessionTimes{State: domain.SessionActive, OpenedAt: t0, StartedAt: t0}})
	r.mu.Lock()
	sd := r.newSide(domain.SideClient, id, nil)
	r.slots[id].dev = sd
	r.hold(r.slots[id])
	r.mu.Unlock()

	timers.Advance(30 * time.Second)
	r.forward(sd, []byte("keys"))
	idle := limitWindows.Idle + limitWindows.IdleGrace
	timers.Advance(idle - tick)
	if e := uc.ends(); len(e) != 0 {
		t.Fatalf("ended one tick before the moved deadline: %v", e)
	}
	timers.Advance(tick)
	if e := uc.ends(); !slices.Equal(e, []string{"end:idle_timeout"}) {
		t.Fatalf("ends %v, want idle_timeout at the instant", e)
	}
}

// TestRelayLimits_MaxAcrossGrant: a new grant (reattach) keeps opened_at, so the maximum duration
// counts from the opening.
func TestRelayLimits_MaxAcrossGrant(t *testing.T) {
	t.Parallel()
	r, uc, timers := limitRelay(t)
	t0 := timers.Clock.Now()
	id, host, dev := ids.New(), ids.New(), ids.New()
	times := domain.SessionTimes{State: domain.SessionActive, OpenedAt: t0, StartedAt: t0}
	r.Expect(app.Expectation{SessionID: id, HostID: host, DeviceID: dev, Deadline: t0.Add(farDeadline), Started: true, Times: times})
	r.mu.Lock()
	sd := r.newSide(domain.SideClient, id, nil)
	r.slots[id].dev = sd
	r.hold(r.slots[id])
	r.mu.Unlock()
	// Input every half idle window keeps idle away; a reattach in between renews the grant.
	for step := time.Duration(0); step < limitWindows.MaxDuration-limitWindows.Idle/2; step += limitWindows.Idle / 2 {
		timers.Advance(limitWindows.Idle / 2)
		r.mu.Lock()
		if r.slots[id] != nil && r.slots[id].dev != nil {
			sd = r.slots[id].dev
		}
		r.mu.Unlock()
		r.forward(sd, []byte("k"))
		if step == limitWindows.Idle*5 {
			// The device left (no connection in this test) and attaches again with a new grant.
			r.mu.Lock()
			r.slots[id].dev = nil
			r.mu.Unlock()
			times.GrantedAt = timers.Clock.Now()
			r.Expect(app.Expectation{SessionID: id, HostID: host, DeviceID: dev, Deadline: timers.Clock.Now().Add(farDeadline),
				Started: true, Times: times, Grant: ids.New()})
			r.mu.Lock()
			sd = r.newSide(domain.SideClient, id, nil)
			r.slots[id].dev = sd
			r.hold(r.slots[id])
			r.mu.Unlock()
		}
	}
	if e := uc.ends(); len(e) != 0 {
		t.Fatalf("ended before the maximum: %v", e)
	}
	timers.Advance(t0.Add(limitWindows.MaxDuration).Sub(timers.Clock.Now()))
	if e := uc.ends(); !slices.Equal(e, []string{"end:max_duration"}) {
		t.Fatalf("ends %v, want max_duration at opened_at plus the maximum", e)
	}
}

// TestRelayLimits_DetachGrace: a detached session ends with detach_expired at detached_at plus
// terminal.detach_grace; the times come from the log tail.
func TestRelayLimits_DetachGrace(t *testing.T) {
	t.Parallel()
	r, uc, timers := limitRelay(t)
	t0 := timers.Clock.Now()
	id, host := ids.New(), ids.New()
	r.Expect(app.Expectation{SessionID: id, HostID: host, DeviceID: ids.New(), Deadline: t0.Add(farDeadline), Started: true,
		Times: domain.SessionTimes{State: domain.SessionActive, OpenedAt: t0, StartedAt: t0}})
	// The device side was here and left: the node holds the session.
	r.mu.Lock()
	r.hold(r.slots[id])
	r.mu.Unlock()
	timers.Advance(10 * time.Second)
	detached := timers.Clock.Now()
	payload := `{"host_id":"` + host.String() + `","state":"detached","opened_at":"` + t0.Format(time.RFC3339Nano) +
		`","started_at":"` + t0.Format(time.RFC3339Nano) + `","detached_at":"` + detached.Format(time.RFC3339Nano) + `"}`
	r.ObserveLogEvent(t.Context(), eventSessionChg, id, []byte(payload))
	timers.Advance(limitWindows.DetachGrace - tick)
	if e := uc.ends(); len(e) != 0 {
		t.Fatalf("ended one tick before the detach deadline: %v", e)
	}
	timers.Advance(tick)
	if e := uc.ends(); !slices.Equal(e, []string{"end:detach_expired"}) {
		t.Fatalf("ends %v, want detach_expired", e)
	}
}

// TestRelayLimits_ProgressWrittenAtMostOncePerGrace (Design note 7 d): the counts of a held
// session are written at most once per terminal.idle_grace while bytes flow, as additions; the end
// at a limit carries the counts not written yet.
func TestRelayLimits_ProgressWrittenAtMostOncePerGrace(t *testing.T) {
	t.Parallel()
	r, uc, timers := limitRelay(t)
	t0 := timers.Clock.Now()
	id := ids.New()
	r.Expect(app.Expectation{SessionID: id, HostID: ids.New(), DeviceID: ids.New(), Deadline: t0.Add(farDeadline), Started: true,
		Times: domain.SessionTimes{State: domain.SessionActive, OpenedAt: t0, StartedAt: t0}})
	r.mu.Lock()
	dev, hst := r.newSide(domain.SideClient, id, nil), r.newSide(domain.SideHost, id, nil)
	r.slots[id].dev, r.slots[id].hst, r.slots[id].paired = dev, hst, true
	r.hold(r.slots[id])
	r.mu.Unlock()
	counters := func() int {
		uc.mu.Lock()
		defer uc.mu.Unlock()
		n := 0
		for _, c := range uc.calls {
			if c == "counters" {
				n++
			}
		}
		return n
	}
	r.forward(dev, []byte("handshake-1"))
	r.forward(hst, []byte("handshake-2"))
	for range 20 {
		r.forward(dev, []byte("abc"))
		timers.Advance(time.Second)
	}
	// 20 seconds of input with a grace of 10 seconds: at most three writes (the first at once).
	time.Sleep(50 * time.Millisecond) // the writes run in their own goroutines
	if n := counters(); n < 1 || n > 3 {
		t.Fatalf("%d progress writes in two grace windows", n)
	}
	timers.Advance(limitWindows.Idle + limitWindows.IdleGrace)
	if e := uc.ends(); !slices.Equal(e, []string{"end:idle_timeout"}) {
		t.Fatalf("ends %v", e)
	}
}

// TestRelayLimits_OnlyTheHoldingNode (code review M2-46, M1): with several nodes every node expects
// the session, but only the node that holds a side runs the limits; a node without a side never
// ends a session whose device types on another node, and a new grant clears the mark.
func TestRelayLimits_OnlyTheHoldingNode(t *testing.T) {
	t.Parallel()
	clk := clock.NewFake(time.Date(2026, 10, 20, 9, 0, 0, 0, time.UTC))
	timers := fakehost.NewClockTimers(clk)
	cfg := RelayConfig{BufferMaxBytes: 1 << 16, OpenTimeout: limitWindows.OpenTimeout, Windows: limitWindows, Multi: true}
	ucA, ucB := &fakeRelayUC{}, &fakeRelayUC{}
	a, err := NewRelay(cfg, ucA, nil, timers, clk, nil)
	if err != nil {
		t.Fatal(err)
	}
	b, err := NewRelay(cfg, ucB, nil, timers, clk, nil)
	if err != nil {
		t.Fatal(err)
	}
	t0 := clk.Now()
	id, host, dev := ids.New(), ids.New(), ids.New()
	e := app.Expectation{SessionID: id, HostID: host, DeviceID: dev, Deadline: t0.Add(farDeadline), Started: true, Grant: ids.New(),
		Times: domain.SessionTimes{State: domain.SessionActive, OpenedAt: t0, StartedAt: t0}}
	a.Expect(e)
	b.Expect(e)
	a.mu.Lock()
	sd := a.newSide(domain.SideClient, id, nil)
	a.slots[id].dev = sd
	a.hold(a.slots[id])
	a.mu.Unlock()
	for range 20 {
		timers.Advance(limitWindows.Idle / 2)
		a.forward(sd, []byte("k"))
	}
	if ea, eb := ucA.ends(), ucB.ends(); len(ea) != 0 || len(eb) != 0 {
		t.Fatalf("ends A %v, B %v during steady input on A", ea, eb)
	}
	// A new grant: the pairing may now live on B; A's mark is cleared, B holds nothing yet.
	e.Grant = ids.New()
	a.mu.Lock()
	a.slots[id].dev = nil
	a.mu.Unlock()
	a.Expect(e)
	b.Expect(e)
	timers.Advance(limitWindows.Idle + limitWindows.IdleGrace)
	if ea, eb := ucA.ends(), ucB.ends(); len(ea) != 0 || len(eb) != 0 {
		t.Fatalf("ends A %v, B %v after a new grant on nodes without a side", ea, eb)
	}
}

// TestRelayLimits_ActivateClearsDetach (red team M2-46, B1): a reattach that pairs just before
// detached_at plus terminal.detach_grace makes the session active on this node at once; the limit
// timer must not end it with detach_expired before the log tail delivered the active row, and idle
// still applies.
func TestRelayLimits_ActivateClearsDetach(t *testing.T) {
	t.Parallel()
	r, uc, timers := limitRelay(t)
	t0 := timers.Clock.Now()
	id, host := ids.New(), ids.New()
	r.Expect(app.Expectation{SessionID: id, HostID: host, DeviceID: ids.New(), Deadline: t0.Add(farDeadline), Started: true,
		Times: domain.SessionTimes{State: domain.SessionActive, OpenedAt: t0, StartedAt: t0}})
	r.mu.Lock()
	r.hold(r.slots[id])
	r.mu.Unlock()
	timers.Advance(10 * time.Second)
	detached := timers.Clock.Now()
	payload := `{"host_id":"` + host.String() + `","state":"detached","opened_at":"` + t0.Format(time.RFC3339Nano) +
		`","started_at":"` + t0.Format(time.RFC3339Nano) + `","detached_at":"` + detached.Format(time.RFC3339Nano) + `"}`
	r.ObserveLogEvent(t.Context(), eventSessionChg, id, []byte(payload))
	timers.Advance(limitWindows.DetachGrace - time.Second)
	r.mu.Lock()
	call := r.applyOne(r.slots[id], domain.RelayAction{Kind: domain.ActActivate})
	r.mu.Unlock()
	call(t.Context())
	timers.Advance(2 * time.Second)
	if e := uc.ends(); len(e) != 0 {
		t.Fatalf("ends %v after a reattach before the detach deadline", e)
	}
	timers.Advance(t0.Add(limitWindows.Idle + limitWindows.IdleGrace).Sub(timers.Clock.Now()))
	if e := uc.ends(); !slices.Equal(e, []string{"end:idle_timeout"}) {
		t.Fatalf("ends %v, want idle_timeout", e)
	}
}

// TestRelayLimits_EndsOnce (M2-46): once the limit timer ended a session with the counts of this
// node, a row of the log tail that arrives before the closed row (the active row of a reattach)
// must not start the timer again: a second end would carry no counts and race the first.
func TestRelayLimits_EndsOnce(t *testing.T) {
	t.Parallel()
	r, uc, timers := limitRelay(t)
	t0 := timers.Clock.Now()
	id, host := ids.New(), ids.New()
	r.Expect(app.Expectation{SessionID: id, HostID: host, DeviceID: ids.New(), Deadline: t0.Add(farDeadline), Started: true,
		Times: domain.SessionTimes{State: domain.SessionActive, OpenedAt: t0, StartedAt: t0}})
	r.mu.Lock()
	r.hold(r.slots[id])
	r.mu.Unlock()
	timers.Advance(limitWindows.Idle + limitWindows.IdleGrace)
	if e := uc.ends(); !slices.Equal(e, []string{"end:idle_timeout"}) {
		t.Fatalf("ends %v, want idle_timeout", e)
	}
	payload := `{"host_id":"` + host.String() + `","state":"active","opened_at":"` + t0.Format(time.RFC3339Nano) +
		`","started_at":"` + t0.Format(time.RFC3339Nano) + `"}`
	r.ObserveLogEvent(t.Context(), eventSessionChg, id, []byte(payload))
	// A reached deadline runs its timer at once in its own goroutine (fakehost.ClockTimers).
	for range 50 {
		timers.Advance(time.Second)
		if e := uc.ends(); !slices.Equal(e, []string{"end:idle_timeout"}) {
			t.Fatalf("ends %v after a late active row, want one idle_timeout", e)
		}
		time.Sleep(2 * time.Millisecond)
	}
}
