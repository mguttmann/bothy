// SPDX-License-Identifier: Apache-2.0

package ws

import (
	"context"
	"fmt"
	"slices"
	gosync "sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/gateway/adapters/wstest"
	"github.com/OWNER/bothy/services/server/internal/gateway/app"
	"github.com/OWNER/bothy/services/server/internal/gateway/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
)

// recMetrics records what the adapter measures.
type recMetrics struct {
	app.NopMetrics
	mu      gosync.Mutex
	dropped int
	lines   []string
}

func (m *recMetrics) Dropped(_ context.Context, reason string, n int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if reason == app.ReasonSlowConsumer {
		m.dropped += n
	}
}

func (m *recMetrics) Delivered(_ context.Context, d time.Duration, durability, kind string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lines = append(m.lines, fmt.Sprintf("delivered %s %s %s", durability, kind, d))
}

func (m *recMetrics) Closed(_ context.Context, code int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lines = append(m.lines, fmt.Sprintf("close %d", code))
}

func (m *recMetrics) snapshot() (int, []string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.dropped, slices.Clone(m.lines)
}

// TestGatewayMetrics_QueueDroppedAt4014: at the 4014 close of a full outbound queue the adapter
// counts the queued frames and the one that did not fit in bothy.gateway.outbound.dropped.
//
// The writer must hold the first heartbeat_ack before the next ones arrive: if it took that frame
// out of the queue only after the overflow, one queued frame fewer is left at the close and the
// count is 2 (the flake of issue #238). The test therefore sends the further heartbeats only once
// the writer blocks with the first frame.
func TestGatewayMetrics_QueueDroppedAt4014(t *testing.T) {
	t.Parallel()
	m := &recMetrics{}
	release := make(chan struct{})
	blocked := make(chan struct{}, 1)
	var block bool
	var mu gosync.Mutex
	s := newTestServer(t, 2, func(h *Handler) {
		h.d.Metrics = m
		h.beforeWrite = func() {
			mu.Lock()
			b := block
			mu.Unlock()
			if b {
				select {
				case blocked <- struct{}{}:
				default:
				}
				<-release
			}
		}
	})
	t.Cleanup(func() { close(release) })
	c := s.dial(t, wstest.WithoutHeartbeat())
	ctx, cancel := context.WithTimeout(context.Background(), testkit.Patience)
	defer cancel()
	if _, err := c.Identify(ctx, wstest.IdentifyParams{APIToken: agentToken}); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	block = true
	mu.Unlock()
	// The first heartbeat_ack goes to the writer, which blocks with it; the next two fill the
	// queue of two, the fourth does not fit.
	if err := c.Send("heartbeat", map[string]any{"last_seq": 0}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-blocked:
	case <-ctx.Done():
		t.Fatal("the writer did not take the first heartbeat_ack")
	}
	for i := 0; i < 3; i++ {
		if err := c.Send("heartbeat", map[string]any{"last_seq": 0}); err != nil {
			break
		}
	}
	if code, _, err := c.ExpectClose(ctx); err != nil || code != int(domain.CloseSlowConsumer) {
		t.Fatalf("close %d %v", code, err)
	}
	if n, _ := m.snapshot(); n != 3 {
		t.Errorf("dropped %d, want 3 (two queued and the one that did not fit)", n)
	}
}

// TestGatewayMetrics_PeerCloseCounted: the close frame of a client reaches bothy.gateway.close
// with its code.
func TestGatewayMetrics_PeerCloseCounted(t *testing.T) {
	t.Parallel()
	m := &recMetrics{}
	s := newTestServerWith(t, 16, nil, m)
	c := s.dial(t)
	ctx, cancel := context.WithTimeout(context.Background(), testkit.Patience)
	defer cancel()
	if _, err := c.Identify(ctx, wstest.IdentifyParams{APIToken: agentToken}); err != nil {
		t.Fatal(err)
	}
	c.Close()
	deadline := testkit.Deadline()
	for {
		if _, lines := m.snapshot(); slices.Contains(lines, "close 1000") {
			return
		}
		if time.Now().After(deadline) {
			_, lines := m.snapshot()
			t.Fatalf("recorded %q, want close 1000", lines)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// TestGatewayMetrics_DeliveredDurability: a live frame records occurred_at to now with its
// durability and the receiver's kind.
func TestGatewayMetrics_DeliveredDurability(t *testing.T) {
	t.Parallel()
	m := &recMetrics{}
	clk := clock.NewFake(time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC))
	w := newConn(nil, 4, nil, nil)
	w.metrics, w.clock = m, clk
	w.delivered(domain.Entry{Live: true, OccurredAt: clk.Now().Add(-250 * time.Millisecond)})
	w.delivered(domain.Entry{Live: true, Ephemeral: true, OccurredAt: clk.Now()})
	_, lines := m.snapshot()
	want := []string{"delivered durable  250ms", "delivered ephemeral  0s"}
	if !slices.Equal(lines, want) {
		t.Errorf("recorded %q, want %q", lines, want)
	}
}
