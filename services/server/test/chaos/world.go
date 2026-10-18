// SPDX-License-Identifier: Apache-2.0

//go:build perf

package chaos

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/wire"
	"github.com/OWNER/bothy/services/server/test/harness"
)

// The load of every chaos test (README.md).
const (
	// Writers and Receivers are the people writing into the channel and holding gateway
	// connections on it.
	Writers   = 4
	Receivers = 3
	// writePause is the pause of a writer between two acknowledged messages.
	writePause = 20 * time.Millisecond
	// retryPause is the first pause before a writer repeats an unacknowledged send; it doubles up
	// to retryPauseMax.
	retryPause    = 100 * time.Millisecond
	retryPauseMax = 2 * time.Second
	// finishWait bounds how long a stopped writer keeps repeating its last unacknowledged send.
	finishWait = 3 * time.Minute
	// reconnectPause is the pause of a receiver before it connects again.
	reconnectPause = 200 * time.Millisecond
)

// World is the load of one chaos test: a channel, writers that write into its main thread and
// repeat every unacknowledged send with the same client_msg_id and Idempotency-Key until it is
// acknowledged, and receivers that reconnect with the last seq they saw.
type World struct {
	P       *harness.Process
	Space   string
	Channel string
	Thread  string

	writers   []*harness.Client
	receivers []*Receiver

	mu        sync.Mutex
	acked     map[string]time.Time // client_msg_id -> acknowledgement
	attempted map[string]bool      // every client_msg_id a writer sent at least once
	stop      chan struct{}        // ends the writers
	recvStop  chan struct{}        // ends the receivers, after the writers
	wg        sync.WaitGroup
	recvWG    sync.WaitGroup
	errs      []error
}

// NewWorld creates the channel, the writers (write on the channel) and the receivers (read on the
// channel) through the public API of p, and connects the receivers.
func NewWorld(t *testing.T, p *harness.Process) *World {
	t.Helper()
	w := &World{P: p, acked: map[string]time.Time{}, attempted: map[string]bool{}, stop: make(chan struct{}), recvStop: make(chan struct{})}
	w.Space = created(t, harness.MustDo(t, p.Owner, harness.Request{Method: http.MethodPost, Path: "/spaces", Body: map[string]any{"name": "Chaos"}}, http.StatusCreated))
	var ch struct {
		ID           string `json:"id"`
		MainThreadID string `json:"main_thread_id"`
	}
	harness.MustDo(t, p.Owner, harness.Request{Method: http.MethodPost, Path: "/spaces/" + w.Space + "/channels",
		Body: map[string]any{"name": "chaos", "kind": "text"}}, http.StatusCreated).JSON(t, &ch)
	w.Channel, w.Thread = ch.ID, ch.MainThreadID
	for i := range Writers + Receivers {
		c := p.NewUser(t, harness.UserSpec{Handle: fmt.Sprintf("chaos-%d", i), DisplayName: fmt.Sprintf("Chaos %d", i)})
		level := "write"
		if i >= Writers {
			level = "read"
		}
		harness.MustDo(t, p.Owner, harness.Request{Method: http.MethodPost, Path: "/grants", Body: map[string]any{
			"subject_kind": "user", "subject_id": c.PrincipalID, "target_kind": "channel", "target_id": w.Channel, "level": level, "reason": "chaos",
		}}, http.StatusCreated)
		if i < Writers {
			w.writers = append(w.writers, c)
			continue
		}
		r := &Receiver{c: c, seen: map[int64]bool{}, firstSeen: map[string]time.Time{}}
		g, err := c.Gateway(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		r.start = g.Ready.HeadSeq
		r.last = r.start
		w.receivers = append(w.receivers, r)
		w.recvWG.Go(func() { r.run(w.recvStop, g) })
	}
	t.Cleanup(w.Stop)
	return w
}

// created returns the id of a created resource.
func created(t *testing.T, r harness.Response) string {
	t.Helper()
	var v struct {
		ID string `json:"id"`
	}
	r.JSON(t, &v)
	return v.ID
}

// StartWriters starts the writers; Stop ends them and the receivers.
func (w *World) StartWriters() {
	for _, c := range w.writers {
		w.wg.Go(func() { w.write(c) })
	}
}

// write sends messages until stop, each until it is acknowledged.
func (w *World) write(c *harness.Client) {
	for {
		select {
		case <-w.stop:
			return
		default:
		}
		if err := w.send(c); err != nil {
			w.fail(err)
			return
		}
		time.Sleep(writePause)
	}
}

// send sends one message and repeats it with the same client_msg_id and Idempotency-Key until the
// server acknowledges it (201 or 200); after stop it keeps trying for finishWait.
func (w *World) send(c *harness.Client) error {
	cm := ids.New().String()
	h := http.Header{}
	h.Set("Idempotency-Key", ids.New().String())
	req := harness.Request{Method: http.MethodPost, Path: "/threads/" + w.Thread + "/messages", Header: h,
		Body: map[string]any{"client_msg_id": cm, "content_md": "chaos " + cm}}
	w.mu.Lock()
	w.attempted[cm] = true
	w.mu.Unlock()
	pause := retryPause
	var deadline time.Time
	for {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		r := c.Do(ctx, req)
		cancel()
		if r.Status == http.StatusCreated || r.Status == http.StatusOK {
			w.mu.Lock()
			w.acked[cm] = time.Now()
			w.mu.Unlock()
			return nil
		}
		if r.Status >= 400 && r.Status < 500 && r.Status != http.StatusConflict && r.Status != http.StatusTooManyRequests && r.Status != http.StatusUnauthorized {
			return fmt.Errorf("send %s: %d %s", cm, r.Status, r.Body)
		}
		select {
		case <-w.stop:
			if deadline.IsZero() {
				deadline = time.Now().Add(finishWait)
			}
			if time.Now().After(deadline) {
				return fmt.Errorf("send %s not acknowledged within %s after the stop: last %d %s", cm, finishWait, r.Status, r.Body)
			}
		default:
		}
		time.Sleep(pause)
		pause = min(2*pause, retryPauseMax)
	}
}

// fail records an error of a writer or receiver.
func (w *World) fail(err error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.errs = append(w.errs, err)
}

// Stop ends the writers (each finishes its unacknowledged send) and the receivers.
func (w *World) Stop() {
	closeOnce(w.stop)
	w.wg.Wait()
	closeOnce(w.recvStop)
	w.recvWG.Wait()
}

// closeOnce closes c unless it is closed.
func closeOnce(c chan struct{}) {
	select {
	case <-c:
	default:
		close(c)
	}
}

// StopWriters ends the writers and fails t with their errors, then waits until every receiver saw
// every acknowledged message, at most settle, and ends the receivers. The receivers keep the
// connections they had (a receiver reconnects only when the server ended its connection), so what
// they saw came through the live path unless the fault itself broke the connection.
func (w *World) StopWriters(t *testing.T, settle time.Duration) {
	t.Helper()
	closeOnce(w.stop)
	w.wg.Wait()
	w.mu.Lock()
	errs := append([]error(nil), w.errs...)
	w.mu.Unlock()
	if err := errors.Join(errs...); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(settle)
	for time.Now().Before(deadline) && !w.allDelivered() {
		time.Sleep(100 * time.Millisecond)
	}
	closeOnce(w.recvStop)
	w.recvWG.Wait()
}

// CheckDeliveredWithin fails t for every message acknowledged after from that a receiver did not
// get, or got later than bound after its acknowledgement: delivery resumed within the poll and
// backoff bounds of 04 "Log tail" after the fault.
func (w *World) CheckDeliveredWithin(t *testing.T, from time.Time, bound time.Duration) {
	t.Helper()
	checked := 0
	for cm, acked := range w.Acked() {
		if acked.Before(from) {
			continue
		}
		checked++
		for i, r := range w.receivers {
			at, ok := r.FirstSeen(cm)
			switch {
			case !ok:
				t.Errorf("receiver %d never got %s, acknowledged %s after the fault", i, cm, acked.Sub(from))
			case at.Sub(acked) > bound:
				t.Errorf("receiver %d got %s %s after its acknowledgement, bound %s", i, cm, at.Sub(acked), bound)
			}
		}
	}
	if checked == 0 {
		t.Error("no message was acknowledged after the fault")
	}
}

// Reconnects returns how often each receiver connected again.
func (w *World) Reconnects() []int {
	out := make([]int, len(w.receivers))
	for i, r := range w.receivers {
		r.mu.Lock()
		out[i] = r.reconnects
		r.mu.Unlock()
	}
	return out
}

// allDelivered reports whether every receiver saw every acknowledged message.
func (w *World) allDelivered() bool {
	w.mu.Lock()
	acked := make([]string, 0, len(w.acked))
	for cm := range w.acked {
		acked = append(acked, cm)
	}
	w.mu.Unlock()
	for _, r := range w.receivers {
		r.mu.Lock()
		for _, cm := range acked {
			if _, ok := r.firstSeen[cm]; !ok {
				r.mu.Unlock()
				return false
			}
		}
		r.mu.Unlock()
	}
	return true
}

// Acked returns the acknowledged client_msg_ids with the time of their acknowledgement.
func (w *World) Acked() map[string]time.Time {
	w.mu.Lock()
	defer w.mu.Unlock()
	out := make(map[string]time.Time, len(w.acked))
	for k, v := range w.acked {
		out[k] = v
	}
	return out
}

// Attempted returns every client_msg_id a writer sent at least once.
func (w *World) Attempted() []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	out := make([]string, 0, len(w.attempted))
	for k := range w.attempted {
		out = append(out, k)
	}
	return out
}

// Receivers returns the receivers.
func (w *World) Receivers() []*Receiver { return w.receivers }

// Receiver is one person's gateway connection that reconnects with the last seq it saw.
type Receiver struct {
	c *harness.Client

	mu         sync.Mutex
	start      int64 // head_seq of the first ready
	last       int64 // highest seq seen
	order      []int64
	seen       map[int64]bool
	dupes      int
	outOfOrder []string
	firstSeen  map[string]time.Time // client_msg_id of message.created -> first receipt
	reconnects int
}

// run reads g (or a new connection when g is nil) until stop, reconnecting from the last seq.
func (r *Receiver) run(stop chan struct{}, g *harness.GatewayConn) {
	for {
		if g == nil {
			r.mu.Lock()
			after := r.last
			r.mu.Unlock()
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			conn, err := r.c.GatewayFrom(ctx, after)
			cancel()
			if err != nil {
				select {
				case <-stop:
					return
				case <-time.After(reconnectPause):
				}
				continue
			}
			g = conn
			r.mu.Lock()
			r.reconnects++
			r.mu.Unlock()
		}
		r.read(stop, g)
		g.Close()
		g = nil
		select {
		case <-stop:
			return
		default:
		}
	}
}

// read records the dispatches of g until stop or the end of the connection.
func (r *Receiver) read(stop chan struct{}, g *harness.GatewayConn) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		select {
		case <-stop:
			cancel()
		case <-ctx.Done():
		}
	}()
	for {
		f, err := g.Next(ctx)
		if err != nil {
			return
		}
		if f.Op != "dispatch" {
			continue
		}
		var env struct {
			Seq         int64  `json:"seq"`
			Type        string `json:"type"`
			ClientMsgID string `json:"client_msg_id"`
		}
		if json.Unmarshal(f.D, &env) != nil || env.Seq == 0 {
			continue
		}
		r.record(env.Seq, env.Type, env.ClientMsgID, f.Received)
	}
}

// record adds one dispatch: a seq seen before is a duplicate (dropped by seq dedupe), a new seq
// below the highest is out of order.
func (r *Receiver) record(seq int64, typ, cm string, at time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.seen[seq] {
		r.dupes++
		return
	}
	if seq < r.last {
		r.outOfOrder = append(r.outOfOrder, fmt.Sprintf("seq %d after %d", seq, r.last))
	}
	r.seen[seq] = true
	r.order = append(r.order, seq)
	r.last = max(r.last, seq)
	if typ == "message.created" && cm != "" {
		if _, ok := r.firstSeen[cm]; !ok {
			r.firstSeen[cm] = at
		}
	}
}

// FirstSeen returns when the receiver first saw the message.created of cm.
func (r *Receiver) FirstSeen(cm string) (time.Time, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	at, ok := r.firstSeen[cm]
	return at, ok
}

// DefaultDuration returns the default of a duration key of the server's configuration schema, the
// value a chaos server runs with.
func DefaultDuration(key string) (time.Duration, error) {
	schema, err := wire.ConfigSchema()
	if err != nil {
		return 0, err
	}
	values, err := config.Load(schema, config.Sources{Env: []string{config.EnvName("server.public_url") + "=http://127.0.0.1:1"}})
	if err != nil {
		return 0, err
	}
	return time.ParseDuration(values.Text(key))
}
