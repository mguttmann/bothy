// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"slices"
	"sync/atomic"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/agents"
	"github.com/OWNER/bothy/services/server/internal/gateway/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/sync"
)

// holdHook holds the event of seq at while held is set, the way runs.Delivery holds a queued
// run's invocation in maintenance mode (M7-08).
type holdHook struct {
	at   int64
	held *atomic.Bool
}

func (h holdHook) Prepare(_ context.Context, _ ids.ID, ev sync.Envelope) (sync.Envelope, bool, error) {
	if ev.Seq != nil && *ev.Seq == h.at && h.held.Load() {
		return sync.Envelope{}, false, app.ErrDeliveryHeld
	}
	return ev, true, nil
}

// TestConsumerRunner_HeldDeliveryWaits (M7-08): a held event stops the stream before it without
// closing the connection; once the hold ends it and the later events arrive in order.
func TestConsumerRunner_HeldDeliveryWaits(t *testing.T) {
	t.Parallel()
	held := &atomic.Bool{}
	held.Store(true)
	env := newEngineEnv(t, func(c *app.ConsumerConfig, d *app.ConsumerDeps) {
		c.MaxInflight = 10
		d.Hook = holdHook{at: 3, held: held}
	})
	_, _, q := env.attach("default", 0)
	env.publish(5)
	got := waitDispatches(t, q, count(2))
	if !slices.Equal(seqsOf(got), []int64{1, 2}) || q.closedWith() != 0 {
		t.Fatalf("while held: dispatched %v, closed with %d", seqsOf(got), q.closedWith())
	}
	held.Store(false)
	all := waitDispatches(t, q, count(5))
	if !slices.Equal(seqsOf(all), []int64{1, 2, 3, 4, 5}) || q.closedWith() != 0 {
		t.Fatalf("after the hold: dispatched %v, closed with %d", seqsOf(all), q.closedWith())
	}
}

// TestRestPoll_HeldDeliveryWaits (M7-08): a REST poll answers the events before a held event and
// a next_cursor before it; after the hold the poll goes on from there.
func TestRestPoll_HeldDeliveryWaits(t *testing.T) {
	t.Parallel()
	held := &atomic.Bool{}
	held.Store(true)
	env := newEngineEnv(t, func(c *app.ConsumerConfig, d *app.ConsumerDeps) {
		c.MaxInflight = 10
		d.Hook = holdHook{at: 3, held: held}
	})
	if _, err := env.e.Poll(env.pollCtx(), "default", 0); err != nil {
		t.Fatal(err)
	}
	env.publish(5)
	p, err := env.e.Poll(env.pollCtx(), "default", 0)
	if got := envelopes(t, p); err != nil || !slices.Equal(seqsOf(got), []int64{1, 2}) || p.NextCursor != 2 {
		t.Fatalf("while held: %v next %d, %v", seqsOf(got), p.NextCursor, err)
	}
	env.e.Acked(context.Background(), agents.ConsumerKey{AgentID: env.agent.ID, Consumer: "default"}, p.NextCursor)
	held.Store(false)
	p, err = env.e.Poll(env.pollCtx(), "default", 0)
	if got := envelopes(t, p); err != nil || !slices.Equal(seqsOf(got), []int64{3, 4, 5}) || p.NextCursor != 5 {
		t.Fatalf("after the hold: %v next %d, %v", seqsOf(got), p.NextCursor, err)
	}
}
