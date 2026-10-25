// SPDX-License-Identifier: Apache-2.0

package fakeadapter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/gateway/adapters/wstest"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/runs/adapters/runstest"
	"github.com/OWNER/bothy/services/server/perf/internal/agentpath"
)

// Config is the configuration of one adapter: its mode and the values of the keys it follows.
type Config struct {
	// Mode is agentpath.ModeInvocation (start and finish every run) or agentpath.ModePatch
	// (start every run and patch it after StartPatches).
	Mode agentpath.Mode
	// MaxInflight is agent.max_inflight_events, AckPersistInterval agent.ack_persist_interval,
	// RunUpdateMinInterval agent.run_update_min_interval.
	MaxInflight          int
	AckPersistInterval   time.Duration
	RunUpdateMinInterval time.Duration
	// Clock is the time of the ack policy and of the call counter; nil means the system clock.
	Clock clock.Clock
}

// The states of a run in the adapter.
const (
	runReceived = "received"
	runRunning  = "running"
	runFinished = "finished"
	runFailed   = "failed"
)

// runState is one run the adapter was invoked for.
type runState struct {
	state   string
	version int64
	started time.Time
}

// Adapter is the fake adapter of one agent (package comment). Safe for concurrent use.
type Adapter struct {
	cfg Config
	api API
	ver Verifier
	clk clock.Clock

	conn    Conn
	cancel  context.CancelFunc
	closing atomic.Bool
	wg      sync.WaitGroup

	// ackMu orders the ack decisions and their frames, so a lower cumulative ack never follows a
	// higher one.
	ackMu sync.Mutex
	ack   *AckPolicy

	mu          sync.Mutex
	changed     chan struct{} // closed and replaced on every change of a run state
	invocations []agentpath.Invocation
	runs        map[string]*runState
	errs        []string
	startRTT    []time.Duration
	finishRTT   []time.Duration
	calls       map[int64]int
	sent        []PatchSent
	patching    bool
}

// New returns an adapter over api and ver. The error reports a mode other than the two of
// agentpath or a value that is not positive.
func New(cfg Config, api API, ver Verifier) (*Adapter, error) {
	if cfg.Mode != agentpath.ModeInvocation && cfg.Mode != agentpath.ModePatch {
		return nil, fmt.Errorf("fakeadapter: mode %q", cfg.Mode)
	}
	if cfg.MaxInflight <= 0 || cfg.AckPersistInterval <= 0 || cfg.RunUpdateMinInterval <= 0 {
		return nil, errors.New("fakeadapter: agent.max_inflight_events, agent.ack_persist_interval and agent.run_update_min_interval must be positive")
	}
	clk := cfg.Clock
	if clk == nil {
		clk = clock.System()
	}
	return &Adapter{cfg: cfg, api: api, ver: ver, clk: clk, ack: NewAckPolicy(cfg.MaxInflight, cfg.AckPersistInterval, clk.Now()),
		changed: make(chan struct{}), runs: map[string]*runState{}, calls: map[int64]int{}}, nil
}

// Start sends the first heartbeat, then connects through dial (09 section 4.8: the agent reports
// before its consumer connects) and starts the read loop, the heartbeats and the ack timer. The
// adapter runs until Stop or the end of ctx. The error reports a failed heartbeat or dial.
func (a *Adapter) Start(ctx context.Context, dial func(context.Context) (Conn, error)) error {
	interval, err := a.heartbeat(ctx)
	if err != nil {
		return err
	}
	conn, err := dial(ctx)
	if err != nil {
		return err
	}
	ctx, a.cancel = context.WithCancel(ctx)
	a.conn = conn
	a.wg.Add(3)
	go a.readLoop(ctx)
	go a.heartbeatLoop(ctx, interval)
	go a.ackLoop(ctx)
	return nil
}

// Stop ends the adapter: it closes the connection (an end the adapter caused is no error) and
// waits for every goroutine, open calls included.
func (a *Adapter) Stop() {
	if a.closing.Swap(true) || a.cancel == nil {
		return
	}
	a.cancel()
	a.conn.Close()
	a.wg.Wait()
}

// readLoop handles every frame in order until the connection ends.
func (a *Adapter) readLoop(ctx context.Context) {
	defer a.wg.Done()
	for {
		f, err := a.conn.Next(ctx)
		if err != nil {
			if !a.closing.Load() && ctx.Err() == nil {
				a.fail("the connection ended: %v", err)
			}
			return
		}
		a.handle(ctx, f)
	}
}

// handle handles one frame: a dispatch counts for the ack policy and an agent.invocation runs its
// handling; an error frame is recorded; other frames (cursor, catchup.complete) change nothing.
func (a *Adapter) handle(ctx context.Context, f Frame) {
	switch f.Op {
	case "dispatch":
		var env struct {
			Seq  *int64 `json:"seq"`
			Type string `json:"type"`
		}
		if err := json.Unmarshal(f.D, &env); err != nil {
			a.fail("a dispatch does not decode: %v", err)
			return
		}
		invocation := env.Type == "agent.invocation"
		if env.Seq != nil {
			a.ackMu.Lock()
			seq, ok := a.ack.Received(*env.Seq, invocation, a.clk.Now())
			a.sendAck(seq, ok)
			a.ackMu.Unlock()
		}
		if invocation {
			a.invocation(ctx, f)
		}
	case "error":
		var e rest.GatewayError
		if err := wstest.Strict(f.D, &e); err != nil {
			a.fail("an error frame does not decode strictly: %v", err)
			return
		}
		a.fail("error frame: op %s, code %s, ref %s", e.Op, e.Code, deref(e.Ref))
	}
}

// invocation is the handling of design note 3 d, in the order of 09 section 4.9, all after the
// receive time: a redelivered envelope and a second invocation of a run are errors and start
// nothing; the token is checked against the payload, a billing mode organization and the tool
// class chat; a rejected token finishes the run failed with invalid_invocation_token, then the
// event is acknowledged; an accepted one is acknowledged, then the run is started in its own
// goroutine, so the read loop goes on.
func (a *Adapter) invocation(ctx context.Context, f Frame) {
	var ev rest.AgentInvocationEvent
	if err := wstest.Strict(f.D, &ev); err != nil {
		a.fail("an agent.invocation does not decode strictly: %v", err)
		return
	}
	p := ev.Payload
	run, thread := p.RunId.String(), p.ThreadId.String()
	message := ""
	if p.MessageId != nil {
		message = p.MessageId.String()
	}
	redelivered := ev.Redelivered != nil && *ev.Redelivered
	a.mu.Lock()
	a.invocations = append(a.invocations, agentpath.Invocation{MessageID: message, RunID: run, OccurredAt: ev.OccurredAt, Received: f.Received, Redelivered: redelivered})
	_, seen := a.runs[run]
	if !seen && !redelivered {
		a.runs[run] = &runState{state: runReceived}
	}
	a.mu.Unlock()
	switch {
	case redelivered:
		a.fail("redelivered agent.invocation of run %s (message %s)", run, message)
		return
	case seen:
		a.fail("second agent.invocation of run %s (message %s)", run, message)
		return
	case ev.InvocationToken == nil:
		a.fail("agent.invocation of run %s without an invocation token", run)
		return
	}
	token := *ev.InvocationToken
	if _, err := a.ver.Check(ctx, token, expectation(run, thread, message)); err != nil {
		a.fail("the reference verifier rejected the token of run %s: %v", run, err)
		a.finish(ctx, run, "failed", "invalid_invocation_token")
		a.invoked()
		return
	}
	a.invoked()
	a.wg.Add(1)
	go a.startRun(ctx, run, token) //nolint:gosec // G118: the start belongs to the adapter, Stop waits for it
}

// expectation is the check of the token against the event and the runtime (09 4.5 points 6, 8 and
// 10). An ID that does not parse stays zero, is not compared and makes the token fail on its own
// claims instead.
func expectation(run, thread, message string) runstest.ReferenceExpectation {
	parse := func(s string) ids.ID {
		id, _ := ids.Parse(s)
		return id
	}
	return runstest.ReferenceExpectation{RunID: parse(run), ThreadID: parse(thread), MessageID: parse(message),
		ActualBillingMode: billingMode, ActualToolClass: toolClass}
}

// invoked sends the ack of an invocation-bearing event that reached its ack step.
func (a *Adapter) invoked() {
	a.ackMu.Lock()
	defer a.ackMu.Unlock()
	seq, ok := a.ack.Invoked(a.clk.Now())
	a.sendAck(seq, ok)
}

// sendAck sends ack {seq} when ok; the caller holds ackMu.
func (a *Adapter) sendAck(seq int64, ok bool) {
	if !ok {
		return
	}
	a.count()
	if err := a.conn.Send("ack", rest.GatewayAck{Seq: seq}); err != nil && !a.closing.Load() {
		a.fail("ack %d: %v", seq, err)
	}
}

// ackLoop applies the time rule of the ack policy a few times per agent.ack_persist_interval.
func (a *Adapter) ackLoop(ctx context.Context) {
	defer a.wg.Done()
	tick := time.NewTicker(a.cfg.AckPersistInterval / 4)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
		a.ackMu.Lock()
		seq, ok := a.ack.Tick(a.clk.Now())
		a.sendAck(seq, ok)
		a.ackMu.Unlock()
	}
}

// startRun reads the context of the run and starts it with the delivered token, billing mode
// organization and tool class chat; in mode invocation it then finishes the run completed, in mode
// patch the run waits for StartPatches.
func (a *Adapter) startRun(ctx context.Context, run, token string) {
	defer a.wg.Done()
	if !a.call(ctx, http.MethodGet, "/runs/"+run+"/context", nil, "getRunContext of run "+run) {
		a.setState(run, runFailed, 0)
		return
	}
	began := time.Now()
	status, body := a.api.Do(ctx, http.MethodPost, "/runs/"+run+"/start", map[string]any{
		"invocation_token": token, "billing_mode_used": billingMode, "tool_class_used": toolClass,
	})
	rtt := time.Since(began)
	a.count()
	if status != http.StatusOK {
		a.failCall(ctx, "startRun of run "+run, status, body)
		a.setState(run, runFailed, 0)
		return
	}
	var started struct {
		Version int64  `json:"version"`
		State   string `json:"state"`
	}
	if err := json.Unmarshal(body, &started); err != nil || started.State != runRunning {
		a.fail("startRun of run %s answered state %q (%v)", run, started.State, err)
		a.setState(run, runFailed, 0)
		return
	}
	a.mu.Lock()
	a.startRTT = append(a.startRTT, rtt)
	a.mu.Unlock()
	a.setState(run, runRunning, started.Version)
	if a.cfg.Mode == agentpath.ModeInvocation {
		a.finish(ctx, run, "completed", "")
	}
}

// finish sends finishRun with state and reason (empty for none) and records the round trip of a
// completed finish.
func (a *Adapter) finish(ctx context.Context, run, state, reason string) {
	body := map[string]any{"state": state}
	if reason != "" {
		body["reason"] = reason
	}
	began := time.Now()
	status, b := a.api.Do(ctx, http.MethodPost, "/runs/"+run+"/finish", body)
	rtt := time.Since(began)
	a.count()
	if status != http.StatusOK {
		a.failCall(ctx, "finishRun "+state+" of run "+run, status, b)
		a.setState(run, runFailed, 0)
		return
	}
	if state != "completed" {
		a.setState(run, runFailed, 0)
		return
	}
	a.mu.Lock()
	a.finishRTT = append(a.finishRTT, rtt)
	a.mu.Unlock()
	a.setState(run, runFinished, 0)
}

// FinishRuns finishes every running run completed (perf/run_patch after Drain); the patches must
// have stopped (StopPatches).
func (a *Adapter) FinishRuns(ctx context.Context) {
	for _, r := range a.Running() {
		a.finish(ctx, r.ID, "completed", "")
	}
}

// call sends one request without a body interest and records a status other than 200.
func (a *Adapter) call(ctx context.Context, method, path string, body any, what string) bool {
	status, b := a.api.Do(ctx, method, path, body)
	a.count()
	if status != http.StatusOK {
		a.failCall(ctx, what, status, b)
		return false
	}
	return true
}

// failCall records an unexpected status with the problem code of its body; a call cut by the end
// of ctx is no finding.
func (a *Adapter) failCall(ctx context.Context, what string, status int, body []byte) {
	if ctx.Err() != nil && a.closing.Load() {
		return
	}
	a.fail("%s = %d %s", what, status, problemCode(body))
}

// setState sets the state of run (and its version when running) and wakes Wait.
func (a *Adapter) setState(run, state string, version int64) {
	a.mu.Lock()
	defer a.mu.Unlock()
	r, ok := a.runs[run]
	if !ok {
		r = &runState{}
		a.runs[run] = r
	}
	r.state = state
	if state == runRunning {
		r.version, r.started = version, a.clk.Now()
	}
	a.wake()
}

// wake wakes every Wait; the caller holds mu.
func (a *Adapter) wake() {
	close(a.changed)
	a.changed = make(chan struct{})
}

// Active returns the IDs of the runs in running, for details.active_runs.
func (a *Adapter) Active() []string {
	var out []string
	for _, r := range a.Running() {
		out = append(out, r.ID)
	}
	return out
}

// Running returns the running runs with the version of their startRun answer, in start order
// (ties by ID).
func (a *Adapter) Running() []RunStart {
	a.mu.Lock()
	defer a.mu.Unlock()
	type entry struct {
		RunStart
		at time.Time
	}
	var es []entry
	for id, r := range a.runs {
		if r.state == runRunning {
			es = append(es, entry{RunStart{ID: id, Version: r.version}, r.started})
		}
	}
	slices.SortFunc(es, func(x, y entry) int {
		if c := x.at.Compare(y.at); c != 0 {
			return c
		}
		if x.ID < y.ID {
			return -1
		}
		return 1
	})
	out := make([]RunStart, len(es))
	for i, e := range es {
		out[i] = e.RunStart
	}
	return out
}

// Counts returns how many runs are running, finished completed and failed.
func (a *Adapter) Counts() (running, finished, failed int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, r := range a.runs {
		switch r.state {
		case runRunning:
			running++
		case runFinished:
			finished++
		case runFailed:
			failed++
		}
	}
	return running, finished, failed
}

// Wait waits until done reports true for the current Report or ctx ends; the adapter wakes it on
// every change of a run state and every finding.
func (a *Adapter) Wait(ctx context.Context, done func(Report) bool) error {
	for {
		a.mu.Lock()
		ch := a.changed
		a.mu.Unlock()
		if done(a.Report()) {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ch:
		}
	}
}

// StartPatches starts the patch writer over the running runs: one patch per run every
// agent.run_update_min_interval from start, phase-shifted, until the scheduled time reaches
// stop. Only in mode patch and only once; the error reports anything else.
func (a *Adapter) StartPatches(ctx context.Context, start, stop time.Time) error {
	a.mu.Lock()
	if a.cfg.Mode != agentpath.ModePatch || a.patching {
		a.mu.Unlock()
		return errors.New("fakeadapter: StartPatches needs mode patch and runs once")
	}
	a.patching = true
	a.mu.Unlock()
	s := NewSchedule(a.Running(), start, stop, a.cfg.RunUpdateMinInterval)
	a.wg.Add(1)
	go func() { //nolint:gosec // G118: the writer belongs to the adapter, Stop waits for it
		defer a.wg.Done()
		err := runWriter(ctx, s, waitUntil, a.sendPatch, a.recordPatch)
		if err != nil && ctx.Err() == nil && !a.closing.Load() {
			a.fail("patch writer: %v", err)
		}
	}()
	return nil
}

// sendPatch sends one run.update.
func (a *Adapter) sendPatch(p Patch) error {
	a.count()
	return a.conn.Send("run.update", map[string]any{"run_id": p.RunID, "base_version": p.BaseVersion, "ops": p.Ops, "ref": p.Ref})
}

// recordPatch records a sent patch with the version it produces and its scheduled time.
func (a *Adapter) recordPatch(p Patch) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.sent = append(a.sent, PatchSent{RunID: p.RunID, Version: p.BaseVersion + 1, Scheduled: p.Scheduled})
}

// count counts one rl.agent_runtime call in the second of now.
func (a *Adapter) count() {
	sec := a.clk.Now().Unix()
	a.mu.Lock()
	defer a.mu.Unlock()
	a.calls[sec]++
}

// fail records one finding; findings never contain a token.
func (a *Adapter) fail(format string, args ...any) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.errs = append(a.errs, fmt.Sprintf(format, args...))
	a.wake()
}

// Report is what one adapter observed.
type Report struct {
	// Invocations are every agent.invocation received, with receive times.
	Invocations []agentpath.Invocation
	// Sent are the patches sent, with the version each produces and its scheduled time.
	Sent []PatchSent
	// StartRTT and FinishRTT are the round trips of startRun and of finishRun completed.
	StartRTT, FinishRTT []time.Duration
	// Errors are the findings: error and close frames, rejected tokens, redelivered and second
	// invocations, unexpected statuses.
	Errors []string
	// MaxCallsPerSecond is the highest number of rl.agent_runtime calls in one second.
	MaxCallsPerSecond int
	// Running, Finished and Failed count the runs by state.
	Running, Finished, Failed int
}

// Report returns a copy of what the adapter observed so far.
func (a *Adapter) Report() Report {
	running, finished, failed := a.Counts()
	a.mu.Lock()
	defer a.mu.Unlock()
	r := Report{
		Invocations: slices.Clone(a.invocations), Sent: slices.Clone(a.sent), StartRTT: slices.Clone(a.startRTT),
		FinishRTT: slices.Clone(a.finishRTT), Errors: slices.Clone(a.errs), Running: running, Finished: finished, Failed: failed,
	}
	for _, n := range a.calls {
		r.MaxCallsPerSecond = max(r.MaxCallsPerSecond, n)
	}
	return r
}

// Err returns an error with the count and the first findings, or nil without any.
func (r Report) Err() error {
	if len(r.Errors) == 0 {
		return nil
	}
	return fmt.Errorf("%d findings, first: %s", len(r.Errors), r.Errors[0])
}

// problemCode returns the code member of a problem body, or "" (the body itself is never echoed:
// it could carry content).
func problemCode(body []byte) string {
	var p struct {
		Code string `json:"code"`
	}
	_ = json.Unmarshal(body, &p)
	return p.Code
}

// deref returns *s or "".
func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
