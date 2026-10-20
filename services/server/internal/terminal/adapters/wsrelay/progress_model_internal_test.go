// SPDX-License-Identifier: Apache-2.0

package wsrelay

import (
	"fmt"
	"log/slog"
	"math/rand/v2"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/terminal/app"
	"github.com/OWNER/bothy/services/server/internal/terminal/domain"
	"github.com/OWNER/bothy/services/server/test/terminal/fakehost"
)

// progressWindows keep the limits of the relay out of the model: only the progress write runs.
var progressWindows = domain.Windows{Idle: 1000 * time.Hour, IdleGrace: 10 * time.Second, MaxDuration: 10000 * time.Hour,
	DetachGrace: time.Hour, OpenTimeout: 20 * time.Second}

// TestRelayProgress_Model (red team M2-46, M1, M3 and the failed write followed by silence, D-M2-46-22)
// drives one paired session of a real relay on a fake clock with seeded random device input, host
// output, pauses and store outages, and checks the three rules of the progress write in every
// quiet state: no byte is lost or counted twice; once the store has worked for a full
// terminal.idle_grace, the stored last_input_at lags the true last input by less than the grace,
// whether or not another frame arrives; and the relay writes at most twice per grace, also while
// every write fails.
func TestRelayProgress_Model(t *testing.T) {
	t.Parallel()
	for seed := range uint64(200) {
		if err := progressRun(t, seed); err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
	}
}

// progressRun is one random sequence; it returns the first broken rule.
func progressRun(t *testing.T, seed uint64) error {
	t.Helper()
	rng := rand.New(rand.NewPCG(seed, 46)) //nolint:gosec // G404: reproducible test sequences from a fixed seed, not security
	grace := progressWindows.IdleGrace
	clk := clock.NewFake(time.Date(2026, 10, 20, 9, 0, 0, 0, time.UTC))
	timers := fakehost.NewClockTimers(clk)
	uc := &fakeRelayUC{}
	r, err := NewRelay(RelayConfig{BufferMaxBytes: 1 << 20, OpenTimeout: progressWindows.OpenTimeout, Windows: progressWindows},
		uc, nil, timers, clk, slog.New(slog.DiscardHandler))
	if err != nil {
		return err
	}
	t0 := clk.Now()
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

	var sentIn, sentOut int64
	var trueInput time.Time
	healthySince, failing := t0, false
	for step := range 400 {
		switch k := rng.IntN(10); {
		case k < 3:
			msg := make([]byte, 1+rng.IntN(8))
			r.forward(dev, msg)
			sentIn += int64(len(msg))
			trueInput = clk.Now()
		case k < 6:
			msg := make([]byte, 1+rng.IntN(64))
			r.forward(hst, msg)
			sentOut += int64(len(msg))
		case k < 9:
			timers.Advance(time.Duration(rng.IntN(int(4*grace/time.Millisecond))) * time.Millisecond)
		default:
			failing = !failing
			uc.mu.Lock()
			uc.failCounters = failing
			uc.mu.Unlock()
			if !failing {
				healthySince = clk.Now()
			}
		}
		if !settled(r) {
			return fmt.Errorf("step %d: the progress writes do not settle (a write storm)", step)
		}

		now := clk.Now()
		var storedIn, storedOut int64
		var stored time.Time
		uc.mu.Lock()
		calls := len(uc.written)
		for _, c := range uc.okWritten {
			storedIn += c.In
			storedOut += c.Out
			if c.LastInput.After(stored) {
				stored = c.LastInput
			}
		}
		uc.mu.Unlock()
		r.mu.Lock()
		s := r.slots[id]
		pendIn, pendOut := s.in, s.out
		r.mu.Unlock()
		where := fmt.Sprintf("step %d at %v", step, now.Sub(t0))
		if storedIn+pendIn != sentIn || storedOut+pendOut != sentOut {
			return fmt.Errorf("%s: stored %d/%d plus pending %d/%d, sent %d/%d", where, storedIn, storedOut, pendIn, pendOut, sentIn, sentOut)
		}
		if !failing && now.Sub(healthySince) >= grace && !trueInput.IsZero() && trueInput.Sub(stored) >= grace {
			return fmt.Errorf("%s: stored last_input_at %v lags the input at %v by %v with a store healthy since %v",
				where, stored.Sub(t0), trueInput.Sub(t0), trueInput.Sub(stored), healthySince.Sub(t0))
		}
		if limit := 2*int(now.Sub(t0)/grace) + 2; calls > limit {
			return fmt.Errorf("%s: %d progress writes, want at most %d", where, calls, limit)
		}
	}
	return nil
}

// settled waits until no progress write is in flight; false when that takes longer than a few
// seconds of real time, which only a write that keeps starting the next one does.
func settled(r *Relay) bool {
	done := make(chan struct{})
	go func() {
		r.writes.Wait()
		close(done)
	}()
	select {
	case <-done:
		return true
	case <-time.After(5 * time.Second):
		return false
	}
}
