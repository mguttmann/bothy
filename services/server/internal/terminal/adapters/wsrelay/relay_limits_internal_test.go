// SPDX-License-Identifier: Apache-2.0

package wsrelay

import (
	"slices"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
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
	r.writes.Wait() // the writes run in their own goroutines
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
		timers.Settle()
		timers.Advance(time.Second)
		timers.Settle()
		if e := uc.ends(); !slices.Equal(e, []string{"end:idle_timeout"}) {
			t.Fatalf("ends %v after a late active row, want one idle_timeout", e)
		}
	}
}

// TestRelayLimits_InputAfterOutputOnlyWriteIsStored (red team M2-46, M1): a progress write that
// carries only host output must not hold back the next device input; otherwise the stored
// last_input_at lags the true one by more than terminal.idle_grace and terminal.sweep ends a
// live session early (Design note 7 d, D-M2-46-7).
func TestRelayLimits_InputAfterOutputOnlyWriteIsStored(t *testing.T) {
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
	r.forward(dev, []byte("handshake-1"))
	r.forward(hst, []byte("handshake-2"))
	// The host prints for a while without input: output-only writes, the last one just now.
	for range 25 {
		r.forward(hst, []byte("out"))
		timers.Advance(time.Second)
	}
	r.forward(hst, []byte("out"))
	timers.Advance(time.Second)
	typed := timers.Clock.Now()
	r.forward(dev, []byte("k"))
	// Then silence: nothing else would carry the input to the store.
	r.writes.Wait() // the writes run in their own goroutines
	uc.mu.Lock()
	defer uc.mu.Unlock()
	var stored time.Time
	for _, c := range uc.written {
		if c.LastInput.After(stored) {
			stored = c.LastInput
		}
	}
	if typed.Sub(stored) >= limitWindows.IdleGrace {
		t.Fatalf("stored last_input_at %v lags the input at %v by %v, want less than the idle grace %v",
			stored.Sub(t0), typed.Sub(t0), typed.Sub(stored), limitWindows.IdleGrace)
	}
}

// TestRelayLimits_FailedWriteRetriesOncePerGrace (red team M2-46, M3): while the progress write
// keeps failing, the relay retries at most once per terminal.idle_grace, not on every frame.
func TestRelayLimits_FailedWriteRetriesOncePerGrace(t *testing.T) {
	t.Parallel()
	r, uc, timers := limitRelay(t)
	uc.failCounters = true
	t0 := timers.Clock.Now()
	id := ids.New()
	r.Expect(app.Expectation{SessionID: id, HostID: ids.New(), DeviceID: ids.New(), Deadline: t0.Add(farDeadline), Started: true,
		Times: domain.SessionTimes{State: domain.SessionActive, OpenedAt: t0, StartedAt: t0}})
	r.mu.Lock()
	dev, hst := r.newSide(domain.SideClient, id, nil), r.newSide(domain.SideHost, id, nil)
	r.slots[id].dev, r.slots[id].hst, r.slots[id].paired = dev, hst, true
	r.hold(r.slots[id])
	r.mu.Unlock()
	r.forward(dev, []byte("handshake-1"))
	r.forward(hst, []byte("handshake-2"))
	r.forward(dev, []byte("k"))
	// 200 host frames within 2 seconds, far less than one grace.
	for range 200 {
		r.writes.Wait() // let a failed write give its counts back
		r.forward(hst, []byte("out"))
		timers.Advance(10 * time.Millisecond)
	}
	r.writes.Wait()
	uc.mu.Lock()
	n := len(uc.written)
	uc.mu.Unlock()
	if n > 2 {
		t.Fatalf("%d progress writes within one grace while the database fails, want at most 2", n)
	}
}

// TestRelayLimits_FailedWriteThenSilenceIsRetried (M2-46, D-M2-46-22): a progress write with
// device input fails and then no frame arrives; the flush timer writes the input once the grace
// passed, so terminal.sweep never decides from the older stored last_input_at.
func TestRelayLimits_FailedWriteThenSilenceIsRetried(t *testing.T) {
	t.Parallel()
	r, uc, timers := limitRelay(t)
	uc.mu.Lock()
	uc.failCounters = true
	uc.mu.Unlock()
	t0 := timers.Clock.Now()
	id := ids.New()
	r.Expect(app.Expectation{SessionID: id, HostID: ids.New(), DeviceID: ids.New(), Deadline: t0.Add(farDeadline), Started: true,
		Times: domain.SessionTimes{State: domain.SessionActive, OpenedAt: t0, StartedAt: t0}})
	r.mu.Lock()
	dev, hst := r.newSide(domain.SideClient, id, nil), r.newSide(domain.SideHost, id, nil)
	r.slots[id].dev, r.slots[id].hst, r.slots[id].paired = dev, hst, true
	r.hold(r.slots[id])
	r.mu.Unlock()
	r.forward(dev, []byte("handshake-1"))
	r.forward(hst, []byte("handshake-2"))
	typed := timers.Clock.Now()
	r.forward(dev, []byte("k"))
	r.writes.Wait()
	uc.mu.Lock()
	uc.failCounters = false
	uc.mu.Unlock()
	timers.Advance(limitWindows.IdleGrace)
	r.writes.Wait()
	uc.mu.Lock()
	defer uc.mu.Unlock()
	if len(uc.okWritten) != 1 || !uc.okWritten[0].LastInput.Equal(typed) || uc.okWritten[0].In != 1 {
		t.Fatalf("written after the grace %v, want the input at %v once", uc.okWritten, typed.Sub(t0))
	}
}

// TestRelayLimits_EndWaitsForWriteInFlight (M2-46, D-M2-46-24): a limit end of the relay waits for
// the progress write of the session that is still in flight, so that write reaches the row before
// the end commits; otherwise the write would find the session closed and its counts would be lost
// for the row and for terminal.terminate.
func TestRelayLimits_EndWaitsForWriteInFlight(t *testing.T) {
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
	r.forward(dev, []byte("handshake-1"))
	r.forward(hst, []byte("handshake-2"))
	uc.entered, uc.hold = make(chan struct{}, 8), make(chan struct{})
	r.forward(dev, []byte("abc")) // the first input is written at once
	testkit.Receive(t, "the first progress write", uc.entered)
	advanced := make(chan struct{})
	go func() {
		defer close(advanced)
		timers.Advance(limitWindows.Idle + limitWindows.IdleGrace)
	}()
	select {
	case <-advanced:
		t.Fatalf("the idle end ran while a progress write was in flight: %v", uc.ends())
	case <-time.After(100 * time.Millisecond): // must not happen; a slow host can only pass this
	}
	close(uc.hold)
	testkit.Receive(t, "the idle end", advanced)
	r.writes.Wait()
	uc.mu.Lock()
	defer uc.mu.Unlock()
	done, end := slices.Index(uc.calls, "counters_done"), slices.Index(uc.calls, "end:idle_timeout")
	if done < 0 || end < 0 || done > end {
		t.Fatalf("calls %v: the write must finish before the end", uc.calls)
	}
	var in int64
	for _, c := range uc.okWritten {
		in += c.In
	}
	for _, p := range uc.progress {
		in += p.BytesIn
	}
	if in != int64(len("abc")) {
		t.Fatalf("bytes_in %d over the writes and the end, want %d", in, len("abc"))
	}
}

// TestRelayLimits_FailedEndKeepsProgress: a limit end that waited for a write in flight and then
// failed (the session stays open) gives its counts back and lets the progress writes go on, so the
// input typed afterwards still reaches the store and terminal.sweep does not see a stale
// last_input_at (D-M2-46-25).
func TestRelayLimits_FailedEndKeepsProgress(t *testing.T) {
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
	r.forward(dev, []byte("handshake-1"))
	r.forward(hst, []byte("handshake-2"))
	uc.mu.Lock()
	uc.failEnd = true
	uc.entered, uc.hold = make(chan struct{}, 64), make(chan struct{})
	uc.mu.Unlock()
	r.forward(dev, []byte("abc")) // the first input is written at once and held in flight
	testkit.Receive(t, "the first progress write", uc.entered)
	advanced := make(chan struct{})
	go func() {
		defer close(advanced)
		timers.Advance(limitWindows.Idle + limitWindows.IdleGrace)
	}()
	testkit.Until(t, "the idle end waits for the write in flight", func() bool {
		r.mu.Lock()
		defer r.mu.Unlock()
		return r.slots[id].ending
	})
	close(uc.hold)
	testkit.Receive(t, "the idle end", advanced)
	r.writes.Wait()
	if e := uc.ends(); !slices.Equal(e, []string{"end:idle_timeout"}) {
		t.Fatalf("ends %v, want one idle_timeout", e)
	}
	uc.mu.Lock()
	uc.hold = nil
	uc.mu.Unlock()
	// The end failed, the session stays open and the user types on.
	for range 6 {
		timers.Advance(limitWindows.IdleGrace)
		r.forward(dev, []byte("more"))
		r.writes.Wait()
	}
	timers.Advance(limitWindows.IdleGrace)
	r.writes.Wait()
	uc.mu.Lock()
	defer uc.mu.Unlock()
	var in int64
	for _, c := range uc.okWritten {
		in += c.In
	}
	if want := int64(len("abc") + 6*len("more")); in != want {
		t.Fatalf("bytes_in %d over the progress writes after a failed end, want %d (writes %v)", in, want, uc.okWritten)
	}
}
