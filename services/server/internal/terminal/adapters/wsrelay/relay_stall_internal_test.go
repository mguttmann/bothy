// SPDX-License-Identifier: Apache-2.0

package wsrelay

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
	"github.com/OWNER/bothy/services/server/internal/terminal/app"
	"github.com/OWNER/bothy/services/server/internal/terminal/domain"
	"github.com/OWNER/bothy/services/server/test/terminal/fakehost"
)

// The relay of the stall tests: a queue of four stall messages, terminal.relay_stall_timeout on a
// fake clock.
const (
	stallBuffer  = 4 * stallMsg
	stallMsg     = 16 << 10
	stallTimeout = 10 * time.Second
)

// stallPair is a started, paired slot whose sides have real connections but no writers, so the test
// decides when the queue toward the device drains (pop); devCodes receives the close code the
// device's client reads.
type stallPair struct {
	r        *Relay
	timers   *fakehost.ClockTimers
	dev, hst *side
	devCodes chan int
}

func newStallPair(t *testing.T) stallPair {
	t.Helper()
	clk := clock.NewFake(time.Date(2026, 10, 25, 9, 0, 0, 0, time.UTC))
	timers := fakehost.NewClockTimers(clk)
	r, err := NewRelay(RelayConfig{BufferMaxBytes: stallBuffer, StallTimeout: stallTimeout, OpenTimeout: time.Minute}, &fakeRelayUC{}, nil,
		timers, clk, nil)
	if err != nil {
		t.Fatal(err)
	}
	id := ids.New()
	r.Expect(app.Expectation{SessionID: id, HostID: ids.New(), DeviceID: ids.New(), Deadline: clk.Now().Add(farDeadline), Started: true,
		Times: domain.SessionTimes{State: domain.SessionActive, OpenedAt: clk.Now(), StartedAt: clk.Now()}})
	devWS, devCodes := stallConn(t)
	hstWS, _ := stallConn(t)
	r.mu.Lock()
	s := r.slots[id]
	p := stallPair{r: r, timers: timers, dev: r.newSide(domain.SideClient, id, devWS), hst: r.newSide(domain.SideHost, id, hstWS), devCodes: devCodes}
	s.dev, s.hst, s.paired = p.dev, p.hst, true
	r.mu.Unlock()
	return p
}

// stallConn returns the server end of a WebSocket connection and the close codes its client reads.
func stallConn(t *testing.T) (*websocket.Conn, chan int) {
	t.Helper()
	conns := make(chan *websocket.Conn, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		ws, err := websocket.Accept(w, req, &websocket.AcceptOptions{CompressionMode: websocket.CompressionDisabled})
		if err != nil {
			return
		}
		conns <- ws
		<-req.Context().Done()
	}))
	t.Cleanup(srv.Close)
	ctx, cancel := context.WithTimeout(context.Background(), testkit.Patience)
	defer cancel()
	client, _, err := fakehost.Dial(ctx, "ws"+strings.TrimPrefix(srv.URL, "http"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.CloseNow() })
	codes := make(chan int, 1)
	go func() {
		for {
			if _, _, err := client.Read(context.Background()); err != nil {
				var ce websocket.CloseError
				if errors.As(err, &ce) {
					codes <- int(ce.Code)
				} else {
					codes <- 0
				}
				return
			}
		}
	}()
	select {
	case ws := <-conns:
		return ws, codes
	case <-ctx.Done():
		t.Fatal("no server end of the connection")
		return nil, nil
	}
}

// fill queues the host messages until the queue toward the device is full.
func (p stallPair) fill(t *testing.T) {
	t.Helper()
	for range stallBuffer / stallMsg {
		p.r.forward(p.hst, make([]byte, stallMsg))
	}
	if b := p.queued(); b != stallBuffer {
		t.Fatalf("queue toward the device %d bytes, want %d", b, stallBuffer)
	}
}

func (p stallPair) queued() int {
	p.r.mu.Lock()
	defer p.r.mu.Unlock()
	return p.r.slots[p.dev.sessionID].toDev.bytes
}

// forwardBlocked starts one more host message and returns when forward waits for room (its stall
// timer is armed): the number of pending timers before, and a channel that closes when forward
// returns.
func (p stallPair) forwardBlocked(t *testing.T, msg []byte) (int, chan struct{}) {
	t.Helper()
	before := p.timers.Pending()
	done := make(chan struct{})
	go func() {
		defer close(done)
		p.r.forward(p.hst, msg)
	}()
	testkit.Until(t, "forward waits for room or returns", func() bool {
		select {
		case <-done:
			return true
		default:
			return p.timers.Pending() > before
		}
	})
	select {
	case <-done:
		t.Fatal("forward returned on a full queue instead of waiting for room")
	default:
	}
	return before, done
}

// TestRelayStall_FullQueueHoldsTheSender (PL/40): a message that does not fit into the full queue
// toward the device waits for room instead of closing the device with 4014; once the writer took a
// message it is queued in order. A reattach replays the host's scrollback at once, more than
// terminal.relay_buffer_max_bytes, and closed the device whenever its link lagged.
func TestRelayStall_FullQueueHoldsTheSender(t *testing.T) {
	t.Parallel()
	p := newStallPair(t)
	p.fill(t)
	last := []byte("last message of the replay")
	before, done := p.forwardBlocked(t, last)
	if m := p.r.pop(p.dev); len(m) != stallMsg {
		t.Fatalf("popped %d bytes, want %d", len(m), stallMsg)
	}
	testkit.Receive(t, "forward after room", done)
	if p.dev.isClosed() || p.hst.isClosed() {
		t.Fatal("a side was closed although the queue drained")
	}
	if b := p.queued(); b != stallBuffer-stallMsg+len(last) {
		t.Fatalf("queue toward the device %d bytes, want %d", b, stallBuffer-stallMsg+len(last))
	}
	for range stallBuffer/stallMsg - 1 {
		p.r.pop(p.dev)
	}
	if m := p.r.pop(p.dev); string(m) != string(last) {
		t.Fatalf("last message %q, want %q", m, last)
	}
	if n := p.timers.Pending(); n != before {
		t.Fatalf("%d pending timers after forward returned, want %d (the stall timer stops)", n, before)
	}
}

// TestRelayStall_SilentReceiverIsTheSlowEnd (PL/40, 24 6.3): a queue that gets no room within
// terminal.relay_stall_timeout closes the device as the slow end with 4014, the host side with
// 1000; one tick before, nothing closes.
func TestRelayStall_SilentReceiverIsTheSlowEnd(t *testing.T) {
	t.Parallel()
	p := newStallPair(t)
	p.fill(t)
	_, done := p.forwardBlocked(t, make([]byte, stallMsg))
	p.timers.Advance(stallTimeout - tick)
	p.timers.Settle()
	if p.dev.isClosed() {
		t.Fatal("device closed one tick before the stall timeout")
	}
	p.timers.Advance(tick)
	testkit.Receive(t, "forward at the stall timeout", done)
	if !p.dev.isClosed() || !p.hst.isClosed() {
		t.Fatal("the sides stayed open after the stall timeout")
	}
	if code := testkit.Receive(t, "close code of the device", p.devCodes); code != int(domain.StreamSlowConsumer) {
		t.Fatalf("device closed with %d, want 4014", code)
	}
}

// TestRelayStall_MessageAboveTheBufferClosesAtOnce: a message larger than the whole queue can never
// fit, so it closes the receiver without waiting (unchanged from M2-44).
func TestRelayStall_MessageAboveTheBufferClosesAtOnce(t *testing.T) {
	t.Parallel()
	p := newStallPair(t)
	p.r.forward(p.hst, make([]byte, stallBuffer+1))
	if !p.dev.isClosed() {
		t.Fatal("device open after a message above the buffer")
	}
}

// isClosed reports a side whose close started.
func (sd *side) isClosed() bool {
	select {
	case <-sd.done:
		return true
	default:
		return false
	}
}
