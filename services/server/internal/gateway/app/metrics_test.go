// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"fmt"
	"slices"
	gosync "sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/gateway/app"
	"github.com/OWNER/bothy/services/server/internal/gateway/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// fakeMetrics records what the gateway measures as readable lines.
type fakeMetrics struct {
	mu    gosync.Mutex
	lines []string
}

func (m *fakeMetrics) add(format string, args ...any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lines = append(m.lines, fmt.Sprintf(format, args...))
}

func (m *fakeMetrics) ConnectionOpened(_ context.Context, kind string) { m.add("active +1 %s", kind) }
func (m *fakeMetrics) ConnectionClosed(_ context.Context, kind string) { m.add("active -1 %s", kind) }
func (m *fakeMetrics) Connected(_ context.Context, d time.Duration, outcome, kind string) {
	m.add("connect %s %s %s", outcome, kind, d)
}
func (m *fakeMetrics) CatchupDone(_ context.Context, d time.Duration, outcome string) {
	m.add("catchup %s %s", outcome, d)
}
func (m *fakeMetrics) Resync(_ context.Context, reason string) { m.add("resync %s", reason) }
func (m *fakeMetrics) Closed(_ context.Context, code int)      { m.add("close %d", code) }
func (m *fakeMetrics) Dropped(_ context.Context, reason string, n int) {
	m.add("dropped %s %d", reason, n)
}
func (m *fakeMetrics) Delivered(_ context.Context, _ time.Duration, durability, kind string) {
	m.add("delivered %s %s", durability, kind)
}

func (m *fakeMetrics) all() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return slices.Clone(m.lines)
}

// has reports whether every line of want was recorded.
func (m *fakeMetrics) has(t *testing.T, want ...string) {
	t.Helper()
	got := m.all()
	for _, w := range want {
		if !slices.Contains(got, w) {
			t.Errorf("no %q in %q", w, got)
		}
	}
}

// metricsEnv returns an env whose gateway measures into m.
func metricsEnv(t *testing.T, m *fakeMetrics, mod func(*app.Deps)) *env {
	return newEnv(t, func(_ *app.Config, d *app.Deps) {
		d.Metrics = m
		if mod != nil {
			mod(d)
		}
	})
}

// TestGatewayMetrics_ConnectOutcomes: bothy.gateway.connect.duration measures the upgrade to
// ready (ok, with the kind of the actor) or to the close before it: client_error for 4016 and a
// client that leaves, server_error for 4000; before identification the kind is anonymous.
func TestGatewayMetrics_ConnectOutcomes(t *testing.T) {
	t.Parallel()
	t.Run("ok", func(t *testing.T) {
		t.Parallel()
		m := &fakeMetrics{}
		e := metricsEnv(t, m, nil)
		c, fc := e.open()
		e.clk.Advance(1500 * time.Millisecond)
		e.identifyAlex(c)
		wantOpen(t, fc)
		m.has(t, "connect ok user 1.5s")
	})
	t.Run("device pending", func(t *testing.T) {
		t.Parallel()
		m := &fakeMetrics{}
		e := metricsEnv(t, m, nil)
		e.authz.set(e.alex.ID, authorize.Decision{Outcome: authorize.OutcomeForbidden, Reason: "device_pending"})
		c, fc := e.open()
		e.clk.Advance(time.Second)
		e.identifyAlex(c)
		wantClosed(t, fc, domain.CloseDevicePending)
		e.gw.Closed(c)
		m.has(t, "connect client_error anonymous 1s", "close 4016")
	})
	t.Run("server error", func(t *testing.T) {
		t.Parallel()
		m := &fakeMetrics{}
		e := metricsEnv(t, m, func(d *app.Deps) {
			d.Snapshot = fakeSnapshot{err: errs.New(errs.CodeUnavailable, ""), log: &callLog{}}
		})
		c, fc := e.open()
		e.identifyAlex(c)
		wantClosed(t, fc, domain.CloseUnknownError)
		e.gw.Closed(c)
		m.has(t, "connect server_error user 0s", "close 4000")
	})
	t.Run("client leaves", func(t *testing.T) {
		t.Parallel()
		m := &fakeMetrics{}
		e := metricsEnv(t, m, nil)
		c, _ := e.open()
		e.gw.Closed(c)
		m.has(t, "connect client_error anonymous 0s")
		if got := m.all(); len(got) != 1 {
			t.Errorf("recorded %q; a connection that ends without a close frame counts no close", got)
		}
	})
}

// TestGatewayMetrics_CloseCodeForeignCodeIsOther: the close code a client sends is counted as it
// came; the adapter maps codes outside 04 to other (adapters/otelmetrics). A server close wins over
// the code the client answers with.
func TestGatewayMetrics_CloseCodeForeignCodeIsOther(t *testing.T) {
	t.Parallel()
	m := &fakeMetrics{}
	e := metricsEnv(t, m, nil)
	c, _ := e.open()
	e.identifyAlex(c)
	c.PeerClosed(4999)
	e.gw.Closed(c)
	m.has(t, "close 4999", "active +1 user", "active -1 user")
	c2, _ := e.open()
	c2.Close(domain.CloseHeartbeatTimeout, "")
	c2.PeerClosed(1000)
	e.gw.Closed(c2)
	m.has(t, "close 4009")
	if slices.Contains(m.all(), "close 1000") {
		t.Error("the client's answer to a server close was counted as well")
	}
}

// TestGatewayMetrics_CatchupAndResync: resync_required counts its reason and ends the catch-up
// with outcome resync_required; catchup.complete ends it with complete.
func TestGatewayMetrics_CatchupAndResync(t *testing.T) {
	t.Parallel()
	m := &fakeMetrics{}
	e := metricsEnv(t, m, nil)
	c, fc := e.open()
	e.clk.Advance(time.Second)
	e.send(c, "identify", map[string]any{"protocol_version": 1, "access_token": alexToken, "dpop": "p", "device_id": e.alex.DeviceID,
		"cursor": 7, "sync_epoch": ids.New()})
	wantOpen(t, fc)
	m.has(t, "resync epoch_changed", "catchup resync_required 0s")

	hm := &fakeMetrics{}
	w, alex, _, _, _ := catchUpWorld()
	h, err := app.NewHub(hubCfg, app.HubDeps{Authenticator: w, Authorizer: w, CatchUp: w, Metrics: hm})
	if err != nil {
		t.Fatal(err)
	}
	qc := connect(t, h, alex, 100, 0, 10)
	waitComplete(t, qc)
	waitFor(t, "catchup complete", func() bool {
		return slices.ContainsFunc(hm.all(), func(l string) bool { return len(l) > 16 && l[:16] == "catchup complete" })
	})
}

// TestGatewayMetrics_SlowConsumerDroppedFrames: a 4014 close of the hub counts the frames of the
// live buffer it discards, and the frame that did not fit the queue.
func TestGatewayMetrics_SlowConsumerDroppedFrames(t *testing.T) {
	t.Parallel()
	m := &fakeMetrics{}
	w := newHubWorld()
	support := ids.New()
	allgemein := w.channel(support)
	alex := person()
	w.grantChannel(alex.ID, allgemein)
	h, err := app.NewHub(hubCfg, app.HubDeps{Authenticator: w, Authorizer: w, CatchUp: w, Metrics: m})
	if err != nil {
		t.Fatal(err)
	}
	qc := connect(t, h, alex, 3, 0, 0)
	for seq := int64(1); seq <= 4; seq++ {
		h.Dispatch(context.Background(), msg(seq, support, allgemein))
	}
	if code := qc.closedWith(); code != domain.CloseSlowConsumer {
		t.Fatalf("closed with %d", code)
	}
	m.has(t, "dropped slow_consumer 1")

	// A connection in catch-up whose live buffer overflows: every buffered event is dropped.
	bm := &fakeMetrics{}
	cfg := hubCfg
	cfg.QueueMax = 2
	bh, err := app.NewHub(cfg, app.HubDeps{Authenticator: w, Authorizer: w, CatchUp: w, Metrics: bm})
	if err != nil {
		t.Fatal(err)
	}
	bq := newQConn(100)
	c, s := app.NewTestConnection(bq, alex)
	bh.Register(context.Background(), c, s)
	for seq := int64(1); seq <= 3; seq++ {
		bh.Dispatch(context.Background(), msg(seq, support, allgemein))
	}
	if code := bq.closedWith(); code != domain.CloseSlowConsumer {
		t.Fatalf("buffer overflow closed with %d", code)
	}
	bm.has(t, "dropped slow_consumer 3")
}

// TestGatewayMetrics_ConnectionsActiveUpDown: a connection counts +1 at ready and -1 when it
// closes; one that never reached ready changes nothing.
func TestGatewayMetrics_ConnectionsActiveUpDown(t *testing.T) {
	t.Parallel()
	m := &fakeMetrics{}
	e := metricsEnv(t, m, nil)
	c, _ := e.open()
	e.send(c, "identify", map[string]any{"protocol_version": 1, "api_token": agentToken})
	m.has(t, "active +1 agent")
	e.gw.Closed(c)
	m.has(t, "active -1 agent")
	never, _ := e.open()
	e.gw.Closed(never)
	n := 0
	for _, l := range m.all() {
		if len(l) > 6 && l[:6] == "active" {
			n++
		}
	}
	if n != 2 {
		t.Errorf("active changes %q, want exactly +1 and -1", m.all())
	}
}

// TestGatewayMetrics_DeliveryDurationLiveOnly: the hub marks the entries of live delivery (durable
// and ephemeral) as live; entries of catch-up and of the flush after it are not, so the writer
// measures bothy.delivery.duration for live delivery only.
func TestGatewayMetrics_DeliveryDurationLiveOnly(t *testing.T) {
	t.Parallel()
	w, alex, space, visible, _ := catchUpWorld()
	h := newHub(t, w, hubCfg)
	qc := connect(t, h, alex, 100, 0, 10)
	waitComplete(t, qc)
	h.Dispatch(context.Background(), msg(11, space, visible))
	var live, caught int
	for _, e := range qc.drain() {
		switch {
		case e.Class != domain.ClassDispatch:
		case e.Live && e.Seq == 11 && !e.Ephemeral:
			live++
		case !e.Live:
			caught++
		default:
			t.Errorf("entry %d live=%v ephemeral=%v", e.Seq, e.Live, e.Ephemeral)
		}
	}
	if live != 1 || caught != 5 {
		t.Errorf("live %d caught up %d, want 1 and 5", live, caught)
	}
}

// TestFanoutLag_ZeroWhenIdle: without events the tail knows of and the hub has not distributed,
// the lag is 0, also long after the last event.
func TestFanoutLag_ZeroWhenIdle(t *testing.T) {
	t.Parallel()
	clk := clock.NewFake(t0)
	w := newHubWorld()
	h, err := app.NewHub(hubCfg, app.HubDeps{Authenticator: w, Authorizer: w, CatchUp: w, Clock: clk})
	if err != nil {
		t.Fatal(err)
	}
	if lag := h.FanoutLag(); lag != 0 {
		t.Fatalf("fresh hub lag %v", lag)
	}
	support := ids.New()
	ch := w.channel(support)
	h.Known(2)
	h.Dispatch(context.Background(), msg(1, support, ch))
	h.Dispatch(context.Background(), msg(2, support, ch))
	clk.Advance(time.Hour)
	if lag := h.FanoutLag(); lag != 0 {
		t.Errorf("idle lag after an hour %v, want 0", lag)
	}
}

// TestFanoutLag_GrowsWhenHubStuck: events the tail knows of but the hub has not distributed age
// with the clock from the moment they became known; distributing them brings the lag back to 0.
func TestFanoutLag_GrowsWhenHubStuck(t *testing.T) {
	t.Parallel()
	clk := clock.NewFake(t0)
	w := newHubWorld()
	h, err := app.NewHub(hubCfg, app.HubDeps{Authenticator: w, Authorizer: w, CatchUp: w, Clock: clk})
	if err != nil {
		t.Fatal(err)
	}
	support := ids.New()
	ch := w.channel(support)
	h.Known(3)
	clk.Advance(2 * time.Second)
	h.Known(5)
	clk.Advance(3 * time.Second)
	if lag := h.FanoutLag(); lag != 5*time.Second {
		t.Fatalf("stuck lag %v, want 5s", lag)
	}
	for seq := int64(1); seq <= 3; seq++ {
		h.Dispatch(context.Background(), msg(seq, support, ch))
	}
	if lag := h.FanoutLag(); lag != 3*time.Second {
		t.Errorf("lag after the first batch %v, want 3s (the second batch became known later)", lag)
	}
	h.Dispatch(context.Background(), msg(4, support, ch))
	h.Dispatch(context.Background(), msg(5, support, ch))
	if lag := h.FanoutLag(); lag != 0 {
		t.Errorf("lag after everything was distributed %v", lag)
	}
}
