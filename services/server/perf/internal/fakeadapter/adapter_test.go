// SPDX-License-Identifier: Apache-2.0

package fakeadapter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"net/http"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/gateway/adapters/wstest"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/textcheck"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
	"github.com/OWNER/bothy/services/server/internal/runs/adapters/runstest"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
	"github.com/OWNER/bothy/services/server/perf/internal/agentpath"
)

// t0 is the fixed time of the fake clock.
var t0 = time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)

// events is the order of everything the fakes saw.
type events struct {
	mu   sync.Mutex
	list []string
}

func (e *events) add(s string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.list = append(e.list, s)
}

func (e *events) all() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return slices.Clone(e.list)
}

// fakeConn hands out queued frames and records the frames the adapter sends.
type fakeConn struct {
	ev     *events
	frames chan Frame
	once   sync.Once
}

func newFakeConn(ev *events) *fakeConn { return &fakeConn{ev: ev, frames: make(chan Frame, 64)} }

func (c *fakeConn) Next(ctx context.Context) (Frame, error) {
	select {
	case <-ctx.Done():
		return Frame{}, ctx.Err()
	case f, ok := <-c.frames:
		if !ok {
			return Frame{}, errors.New("closed")
		}
		return f, nil
	}
}

func (c *fakeConn) Send(op string, d any) error {
	b, err := json.Marshal(d)
	if err != nil {
		return err
	}
	c.ev.add(op + " " + string(b))
	return nil
}

func (c *fakeConn) Close() { c.once.Do(func() { close(c.frames) }) }

// fakeAPI answers the REST calls of the adapter and records them.
type fakeAPI struct {
	ev       *events
	interval atomic.Int64
	bodies   []rest.Heartbeat
	mu       sync.Mutex
}

func (f *fakeAPI) Do(_ context.Context, method, path string, body any) (int, []byte) {
	raw, _ := json.Marshal(body)
	switch {
	case path == heartbeatPath:
		var hb rest.Heartbeat
		if err := wstest.Strict(raw, &hb); err != nil {
			f.ev.add("heartbeat invalid " + err.Error())
			return http.StatusBadRequest, nil
		}
		f.mu.Lock()
		f.bodies = append(f.bodies, hb)
		f.mu.Unlock()
		f.ev.add("heartbeat")
		return http.StatusOK, []byte(fmt.Sprintf(`{"interval_ms": %d, "presence": {"state": "online"}}`, f.interval.Load()))
	case strings.HasSuffix(path, "/context"):
		f.ev.add("context")
		return http.StatusOK, []byte(`{}`)
	case strings.HasSuffix(path, "/start"):
		var b map[string]any
		_ = json.Unmarshal(raw, &b)
		f.ev.add(fmt.Sprintf("start %v %v", b["billing_mode_used"], b["tool_class_used"]))
		return http.StatusOK, []byte(`{"state": "running", "version": 7}`)
	case strings.HasSuffix(path, "/finish"):
		var b map[string]any
		_ = json.Unmarshal(raw, &b)
		f.ev.add(fmt.Sprintf("finish %v %v", b["state"], b["reason"]))
		return http.StatusOK, []byte(`{}`)
	}
	f.ev.add(method + " " + path)
	return http.StatusNotFound, []byte(`{"code": "not_found"}`)
}

// fakeVerifier records every check and rejects when reject is set.
type fakeVerifier struct {
	ev     *events
	reject error
	got    []runstest.ReferenceExpectation
	mu     sync.Mutex
}

func (v *fakeVerifier) Check(_ context.Context, _ string, e runstest.ReferenceExpectation) (runstest.ReferenceClaims, error) {
	v.mu.Lock()
	v.got = append(v.got, e)
	v.mu.Unlock()
	v.ev.add("check")
	return runstest.ReferenceClaims{}, v.reject
}

// world is one adapter over fakes.
type world struct {
	ev   *events
	conn *fakeConn
	api  *fakeAPI
	ver  *fakeVerifier
	a    *Adapter
}

func newWorld(t *testing.T, mode agentpath.Mode) *world {
	t.Helper()
	ev := &events{}
	w := &world{ev: ev, conn: newFakeConn(ev), api: &fakeAPI{ev: ev}, ver: &fakeVerifier{ev: ev}}
	w.api.interval.Store(30000)
	a, err := New(Config{Mode: mode, MaxInflight: 100, AckPersistInterval: time.Hour, RunUpdateMinInterval: 500 * time.Millisecond,
		Clock: clock.NewFake(t0)}, w.api, w.ver)
	if err != nil {
		t.Fatal(err)
	}
	w.a = a
	if err := a.Start(t.Context(), func(context.Context) (Conn, error) {
		ev.add("dial")
		return w.conn, nil
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(a.Stop)
	return w
}

// invocationFrame returns a dispatch of agent.invocation for run, thread and message with seq.
func invocationFrame(t *testing.T, seq int64, run, thread, message string, redelivered bool, received time.Time) Frame {
	t.Helper()
	env := map[string]any{
		"id": ids.New().String(), "seq": seq, "type": "agent.invocation", "schema_version": 1,
		"occurred_at": received.Add(-30 * time.Millisecond).Format(time.RFC3339Nano), "invocation_token": "header.claims.signature",
		"resource": map[string]any{"kind": "run", "id": run}, "scope": map[string]any{"thread_id": thread},
		"payload": map[string]any{"run_id": run, "thread_id": thread, "channel_id": ids.New().String(), "message_id": message,
			"billing_mode": "organization", "trigger_kind": "message", "triggered_by": map[string]any{"id": ids.New().String(), "kind": "user", "display_name": "Alex"}},
	}
	if redelivered {
		env["redelivered"] = true
	}
	b, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	return Frame{Op: "dispatch", D: b, Received: received}
}

// wait waits until the adapter's report satisfies done.
func wait(t *testing.T, a *Adapter, done func(Report) bool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), testkit.Patience)
	defer cancel()
	if err := a.Wait(ctx, done); err != nil {
		t.Fatalf("waiting for the adapter: %v (%+v)", err, a.Report())
	}
}

// TestInvocationHandling_Order: the receive time is taken before decoding (the transport's);
// then token check, ack, getRunContext, startRun with organization and chat, then finishRun with
// completed in mode invocation or the patch schedule in mode patch; a rejected token, a
// redelivered envelope and a second invocation for one run are recorded as errors and start
// nothing; the rejected token leads to finishRun failed with invalid_invocation_token, then the
// ack.
func TestInvocationHandling_Order(t *testing.T) {
	t.Parallel()
	t.Run("invocation", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t, agentpath.ModeInvocation)
		run, thread, message := ids.New().String(), ids.New().String(), ids.New().String()
		recv := t0.Add(5 * time.Second)
		w.conn.frames <- invocationFrame(t, 41, run, thread, message, false, recv)
		wait(t, w.a, func(r Report) bool { return r.Finished == 1 })
		want := []string{"heartbeat", "dial", "check", `ack {"seq":41}`, "context", "start organization chat", "finish completed <nil>"}
		if got := w.ev.all(); !slices.Equal(got, want) {
			t.Errorf("order\n got %q\nwant %q", got, want)
		}
		rep := w.a.Report()
		if len(rep.Invocations) != 1 || !rep.Invocations[0].Received.Equal(recv) || rep.Invocations[0].MessageID != message || rep.Invocations[0].RunID != run {
			t.Errorf("invocation %+v, want the transport's receive time %s", rep.Invocations, recv)
		}
		if rep.Invocations[0].OccurredAt.IsZero() {
			t.Error("occurred_at not kept")
		}
		e := w.ver.got[0]
		if e.RunID.String() != run || e.ThreadID.String() != thread || e.MessageID.String() != message || e.ActualBillingMode != "organization" || e.ActualToolClass != "chat" {
			t.Errorf("expectation %+v", e)
		}
		if err := rep.Err(); err != nil {
			t.Errorf("clean run reported %v", err)
		}
		if len(rep.StartRTT) != 1 || len(rep.FinishRTT) != 1 {
			t.Errorf("round trips %v %v", rep.StartRTT, rep.FinishRTT)
		}
	})
	t.Run("patch", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t, agentpath.ModePatch)
		run, thread := ids.New().String(), ids.New().String()
		w.conn.frames <- invocationFrame(t, 9, run, thread, ids.New().String(), false, t0)
		wait(t, w.a, func(r Report) bool { return r.Running == 1 })
		want := []string{"heartbeat", "dial", "check", `ack {"seq":9}`, "context", "start organization chat"}
		if got := w.ev.all(); !slices.Equal(got, want) {
			t.Errorf("order\n got %q\nwant %q", got, want)
		}
		if got := w.a.Running(); len(got) != 1 || got[0] != (RunStart{ID: run, Thread: thread, Version: 7}) {
			t.Errorf("running %+v, want the run at the version of the startRun answer", got)
		}
	})
	t.Run("rejected", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t, agentpath.ModeInvocation)
		w.ver.reject = runstest.ErrRefInvalid
		run := ids.New().String()
		w.conn.frames <- invocationFrame(t, 3, run, ids.New().String(), ids.New().String(), false, t0)
		wait(t, w.a, func(r Report) bool { return r.Failed == 1 })
		want := []string{"heartbeat", "dial", "check", "finish failed invalid_invocation_token", `ack {"seq":3}`}
		if got := w.ev.all(); !slices.Equal(got, want) {
			t.Errorf("order\n got %q\nwant %q", got, want)
		}
		if err := w.a.Report().Err(); err == nil || !strings.Contains(err.Error(), run) {
			t.Errorf("rejected token: %v", err)
		}
	})
	t.Run("redelivered and second", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t, agentpath.ModeInvocation)
		run, thread, message := ids.New().String(), ids.New().String(), ids.New().String()
		w.conn.frames <- invocationFrame(t, 1, ids.New().String(), thread, message, true, t0)
		w.conn.frames <- invocationFrame(t, 2, run, thread, message, false, t0)
		wait(t, w.a, func(r Report) bool { return r.Finished == 1 })
		w.conn.frames <- invocationFrame(t, 3, run, thread, message, false, t0)
		wait(t, w.a, func(r Report) bool { return len(r.Errors) >= 2 })
		rep := w.a.Report()
		if len(rep.Errors) != 2 || !strings.Contains(rep.Errors[0], "redelivered") || !strings.Contains(rep.Errors[1], "second") {
			t.Fatalf("errors %q", rep.Errors)
		}
		checks, starts := 0, 0
		for _, e := range w.ev.all() {
			switch {
			case e == "check":
				checks++
			case strings.HasPrefix(e, "start"):
				starts++
			}
		}
		if checks != 1 || starts != 1 {
			t.Errorf("%d checks and %d starts, want one each: only the first clean invocation starts", checks, starts)
		}
	})
}

// TestAckPolicy: an ack right after an invocation-bearing event; at half of
// agent.max_inflight_events; after agent.ack_persist_interval with something unacknowledged; over
// a random stream never more than agent.max_inflight_events unacknowledged; every ack names the
// highest received seq.
func TestAckPolicy(t *testing.T) {
	t.Parallel()
	const maxInflight, persist = 10, time.Second
	p := NewAckPolicy(maxInflight, persist, t0)
	if _, ok := p.Received(1, true, t0); ok {
		t.Fatal("an invocation was acknowledged before its ack step")
	}
	if _, ok := p.Tick(t0.Add(time.Hour)); ok {
		t.Fatal("the time rule acknowledged an invocation before its ack step")
	}
	if seq, ok := p.Invoked(t0); !ok || seq != 1 {
		t.Fatalf("after the invocation: %d %v", seq, ok)
	}
	for s := int64(2); s < 6; s++ {
		if _, ok := p.Received(s, false, t0); ok {
			t.Fatalf("ack at %d before half the window", s)
		}
	}
	if seq, ok := p.Received(6, false, t0); !ok || seq != 6 {
		t.Fatalf("at half the window: %d %v", seq, ok)
	}
	p.Received(7, false, t0)
	if _, ok := p.Tick(t0.Add(persist - time.Nanosecond)); ok {
		t.Fatal("ack before agent.ack_persist_interval")
	}
	if seq, ok := p.Tick(t0.Add(persist)); !ok || seq != 7 {
		t.Fatalf("after agent.ack_persist_interval: %d %v", seq, ok)
	}
	if _, ok := p.Tick(t0.Add(time.Hour)); ok {
		t.Fatal("ack without anything unacknowledged")
	}

	// A random stream: seqs rise with gaps, a tenth are invocations acknowledged at once, the clock
	// moves by random steps. The seed is printed and kept below as a regression case.
	for _, seed := range []uint64{1, 20261025, rand.Uint64()} { // #nosec G404 -- test input, not a secret
		r := rand.New(rand.NewPCG(seed, seed^0x5eed)) // #nosec G404 -- test input, not a secret
		p := NewAckPolicy(maxInflight, persist, t0)
		now, seq, highest := t0, int64(0), int64(0)
		for range 10000 {
			seq += 1 + r.Int64N(3)
			now = now.Add(time.Duration(r.Int64N(int64(300 * time.Millisecond))))
			inv := r.IntN(10) == 0
			acked, ok := p.Received(seq, inv, now)
			highest = seq
			if inv {
				acked, ok = p.Invoked(now)
			}
			if !ok {
				acked, ok = p.Tick(now)
			}
			if ok && acked != highest {
				t.Fatalf("seed %d: ack %d, highest received %d", seed, acked, highest)
			}
			if p.Unacked() > maxInflight/2 {
				t.Fatalf("seed %d: %d unacknowledged, window %d", seed, p.Unacked(), maxInflight)
			}
		}
	}
}

// TestPatchSchedule: one patch per run per agent.run_update_min_interval; the runs of one adapter
// phase-shifted evenly; base versions v, v+1, v+2; a lagging writer keeps the scheduled time as
// send time.
func TestPatchSchedule(t *testing.T) {
	t.Parallel()
	interval := 600 * time.Millisecond
	runs := []RunStart{{ID: "a", Version: 10}, {ID: "b", Version: 20}, {ID: "c", Version: 30}}
	s := NewSchedule(runs, t0, t0.Add(3*interval), interval)
	var got []Patch
	for {
		p, ok := s.Next()
		if !ok {
			break
		}
		got = append(got, p)
	}
	if len(got) != 9 {
		t.Fatalf("%d patches, want three per run", len(got))
	}
	for i, p := range got {
		j, k := i%3, i/3
		wantAt := t0.Add(time.Duration(k)*interval + time.Duration(j)*interval/3)
		if p.RunID != runs[j].ID || p.BaseVersion != runs[j].Version+int64(k) || !p.Scheduled.Equal(wantAt) {
			t.Errorf("patch %d = %s base %d at %s, want %s base %d at %s", i, p.RunID, p.BaseVersion, p.Scheduled, runs[j].ID, runs[j].Version+int64(k), wantAt)
		}
	}

	// A writer that lags (every wait returns at once, long after the due time) records the
	// scheduled times, not its own.
	s = NewSchedule(runs, t0, t0.Add(2*interval), interval)
	var recorded []Patch
	sends := 0
	err := runWriter(t.Context(), s, func(context.Context, time.Time) error { return nil },
		func(Patch) error { sends++; return nil }, func(p Patch) { recorded = append(recorded, p) })
	if err != nil || sends != 6 || len(recorded) != 6 {
		t.Fatalf("writer: %v, %d sends, %d recorded", err, sends, len(recorded))
	}
	for i, p := range recorded {
		if !p.Scheduled.Equal(got[i].Scheduled) {
			t.Errorf("recorded %s, want the scheduled %s", p.Scheduled, got[i].Scheduled)
		}
	}
	if _, ok := NewSchedule(nil, t0, t0.Add(time.Hour), interval).Next(); ok {
		t.Error("a schedule without runs gave a patch")
	}
}

// TestPatchOps_Allowed: every patch of a long generated sequence passes ParseOps of
// runs/domain without a field error, and its display texts pass textcheck.DisplayText.
func TestPatchOps_Allowed(t *testing.T) {
	t.Parallel()
	g := &OpGen{}
	steps := 0
	for k := range 1000 {
		ops := g.Next()
		raw := make([]domain.RawOp, len(ops))
		for i, op := range ops {
			v, err := json.Marshal(op.Value)
			if err != nil {
				t.Fatal(err)
			}
			raw[i] = domain.RawOp{Op: op.Op, Path: op.Path, Value: v}
			switch {
			case op.Path == "/status_line":
				if err := textcheck.DisplayText(op.Value.(string)); err != nil {
					t.Errorf("patch %d status_line: %v", k, err)
				}
			case op.Op == "add" && strings.HasPrefix(op.Path, "/steps/"):
				steps++
				m := op.Value.(map[string]any)
				for _, f := range []string{"name", "title"} {
					if err := textcheck.DisplayText(m[f].(string)); err != nil {
						t.Errorf("patch %d step %s: %v", k, f, err)
					}
				}
			}
		}
		if _, fieldErrs := domain.ParseOps(raw); len(fieldErrs) > 0 {
			t.Fatalf("patch %d: %+v", k, fieldErrs)
		}
	}
	if steps != 1000/agentpath.StepEvery {
		t.Errorf("%d step adds, want one per StepEvery patches", steps)
	}
}

// TestHeartbeat: the first heartbeat goes out before the consumer connects; the interval comes
// from HeartbeatAck.interval_ms; details.active_runs lists exactly the running runs of the
// adapter; the body validates against Heartbeat.
func TestHeartbeat(t *testing.T) {
	t.Parallel()
	w := newWorld(t, agentpath.ModePatch)
	if got := w.ev.all(); len(got) < 2 || got[0] != "heartbeat" || got[1] != "dial" {
		t.Fatalf("start order %q, want the heartbeat before the dial", got)
	}
	w.api.interval.Store(1234)
	run := ids.New().String()
	w.conn.frames <- invocationFrame(t, 1, run, ids.New().String(), ids.New().String(), false, t0)
	wait(t, w.a, func(r Report) bool { return r.Running == 1 })
	d, err := w.a.heartbeat(t.Context())
	if err != nil || d != 1234*time.Millisecond {
		t.Fatalf("interval %s, %v; want interval_ms of the answer", d, err)
	}
	w.api.mu.Lock()
	first, last := w.api.bodies[0], w.api.bodies[len(w.api.bodies)-1]
	w.api.mu.Unlock()
	if runs := (*first.Details)["active_runs"].([]any); len(runs) != 0 {
		t.Errorf("first heartbeat lists %v, want no run", runs)
	}
	if runs := (*last.Details)["active_runs"].([]any); len(runs) != 1 || runs[0] != run {
		t.Errorf("heartbeat lists %v, want exactly the running run %s", runs, run)
	}
	if last.State != rest.HeartbeatStateOnline || (*last.BillingModesSupported)[0] != "organization" || *last.ReportedToolClass != "chat" {
		t.Errorf("heartbeat %+v", last)
	}
	w.a.FinishRuns(t.Context())
	if got := w.a.Active(); len(got) != 0 {
		t.Errorf("after finishing: active %v", got)
	}
}

// TestErrorFrameFails: an error frame for run.update (codes rate_limited and
// base_version_conflict) is recorded with op, code and ref, and the adapter's report fails.
func TestErrorFrameFails(t *testing.T) {
	t.Parallel()
	for _, code := range []string{"rate_limited", "base_version_conflict"} {
		t.Run(code, func(t *testing.T) {
			t.Parallel()
			w := newWorld(t, agentpath.ModePatch)
			w.conn.frames <- Frame{Op: "error", D: []byte(`{"op": "run.update", "code": "` + code + `", "ref": "p7"}`), Received: t0}
			wait(t, w.a, func(r Report) bool { return r.Err() != nil })
			err := w.a.Report().Err()
			if err == nil || !strings.Contains(err.Error(), "run.update") || !strings.Contains(err.Error(), code) || !strings.Contains(err.Error(), "p7") {
				t.Errorf("report %v, want a finding with op, code and ref", err)
			}
		})
	}
}
