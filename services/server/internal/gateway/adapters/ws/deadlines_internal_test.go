// SPDX-License-Identifier: Apache-2.0

package ws

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/gateway/adapters/wstest"
	"github.com/OWNER/bothy/services/server/internal/gateway/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
)

// Short deadlines of the clocked server; only the injected clock moves them.
const (
	clockedIdentify = 5 * time.Second
	clockedBeat     = time.Second
	// clockedDeadline is clockedBeat times (miss limit 1 + 1).
	clockedDeadline = 2 * clockedBeat
)

// tickClock is the clock of the handler: it reads the fake clock of the gateway and counts the
// reads, so a test knows when the deadline goroutine ticked at a moved time.
type tickClock struct {
	fake  *clock.Fake
	reads atomic.Int64
}

// Now returns the time of the fake clock.
func (c *tickClock) Now() time.Time {
	c.reads.Add(1)
	return c.fake.Now()
}

// advanceAndTick moves the fake clock by d and waits until the handler has ticked at the new
// time. The deadline goroutine reads the clock twice per round (the instant of Tick, then the
// timer after Tick returned), and an idle connection makes no other read. Of four reads after
// the count was taken, at most the first belongs to a round that read the old time, so the
// fourth ends a whole round that started at the new time.
func (c *tickClock) advanceAndTick(t *testing.T, d time.Duration) {
	t.Helper()
	before := c.reads.Load()
	c.fake.Advance(d)
	testkit.Until(t, "a tick of the handler at the moved clock (a close stops the ticks)", func() bool { return c.reads.Load() >= before+4 })
}

// newClockedServer starts a test server whose gateway and handler read a fake clock, with
// sync.identify_timeout clockedIdentify and a heartbeat deadline of clockedDeadline.
func newClockedServer(t *testing.T) (*testServer, *tickClock) {
	t.Helper()
	fake := clock.NewFake(time.Now())
	tc := &tickClock{fake: fake}
	s := newTestServerOpts(t, 16, nil, serverOpts{clock: fake, handlerClock: tc, windows: func(c *app.Config) {
		c.IdentifyTimeout, c.HeartbeatInterval, c.HeartbeatMissLimit = clockedIdentify, clockedBeat, 1 // nosemgrep: bothy-go-test-gateway-window -- the deadlines under test, moved only by the injected clock
	}})
	return s, tc
}

// expectCode waits for the close of c and requires code.
func expectCode(t *testing.T, c *wstest.Client, code int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), testkit.Patience)
	defer cancel()
	got, reason, err := c.ExpectClose(ctx)
	if err != nil {
		t.Fatalf("no close: %v", err)
	}
	if got != code {
		t.Fatalf("closed with %d %q, want %d", got, reason, code)
	}
}

// TestDeadlinesFollowTheInjectedClock: over a real WebSocket, the identify and heartbeat deadlines
// of the gateway hold to the instant on the clock of the gateway and the handler, not to how fast
// the host runs. A connection is still open one millisecond before each deadline after the handler
// ticked there, a first heartbeat after more than one interval (the jitter of 04 section 6.3 step
// 5) is answered, and the deadline itself closes with 4003 or 4009.
func TestDeadlinesFollowTheInjectedClock(t *testing.T) {
	t.Parallel()

	t.Run("heartbeat", func(t *testing.T) {
		t.Parallel()
		s, tc := newClockedServer(t)
		c := s.dial(t, wstest.WithoutHeartbeat())
		tc.advanceAndTick(t, clockedIdentify-time.Millisecond)
		ctx, cancel := context.WithTimeout(context.Background(), testkit.Patience)
		defer cancel()
		if _, err := c.Identify(ctx, wstest.IdentifyParams{APIToken: agentToken}); err != nil {
			t.Fatalf("identify one millisecond before sync.identify_timeout: %v", err)
		}
		tc.advanceAndTick(t, clockedDeadline-time.Millisecond)
		if err := c.Send("heartbeat", map[string]any{"last_seq": 0}); err != nil {
			t.Fatal(err)
		}
		op, _, err := c.Next(ctx)
		if err != nil || op != "heartbeat_ack" {
			t.Fatalf("a first heartbeat one millisecond before the deadline: %s, %v; want heartbeat_ack", op, err)
		}
		tc.advanceAndTick(t, clockedDeadline-time.Millisecond)
		if err := c.Send("heartbeat", map[string]any{"last_seq": 0}); err != nil {
			t.Fatal(err)
		}
		if op, _, err := c.Next(ctx); err != nil || op != "heartbeat_ack" {
			t.Fatalf("a heartbeat one millisecond before the moved deadline: %s, %v; want heartbeat_ack", op, err)
		}
		// The deadline goroutine closes at the deadline and reads the clock no more.
		tc.fake.Advance(clockedDeadline)
		expectCode(t, c, 4009)
	})

	t.Run("identify", func(t *testing.T) {
		t.Parallel()
		s, tc := newClockedServer(t)
		c := s.dial(t, wstest.WithoutHeartbeat())
		tc.advanceAndTick(t, clockedIdentify-time.Millisecond)
		tc.fake.Advance(time.Millisecond)
		expectCode(t, c, 4003)
	})
}
