// SPDX-License-Identifier: Apache-2.0

package lifecycle_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/platform/lifecycle"
)

// recorder collects the names of started components and run stop steps in order.
type recorder struct {
	mu  sync.Mutex
	log []string
}

func (r *recorder) add(s string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.log = append(r.log, s)
}

func (r *recorder) got() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.log...)
}

// component is a test component: it records its start, optionally registers a stop step and
// returns err.
type component struct {
	name  string
	rec   *recorder
	err   error
	stop  *lifecycle.StopStep
	r     *lifecycle.Runner
	start func(ctx context.Context) error
}

func (c *component) Name() string { return c.name }

func (c *component) Start(ctx context.Context) error {
	c.rec.add("start " + c.name)
	if c.start != nil {
		if err := c.start(ctx); err != nil {
			return err
		}
	}
	if c.err != nil {
		return c.err
	}
	if c.stop != nil {
		c.r.OnStop(*c.stop)
	}
	return nil
}

func step(rec *recorder, name string, phase lifecycle.Phase, budget time.Duration) lifecycle.StopStep {
	return lifecycle.StopStep{Name: name, Phase: phase, Budget: budget, Run: func(context.Context) error {
		rec.add("stop " + name)
		return nil
	}}
}

type harness struct {
	r     *lifecycle.Runner
	sigs  chan os.Signal
	clk   *clock.Fake
	logs  *syncBuffer
	rec   *recorder
	total time.Duration
}

// syncBuffer is a bytes.Buffer safe for the concurrent writes of the runner's goroutines.
type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

func newHarness(t *testing.T, total time.Duration) *harness {
	t.Helper()
	h := &harness{
		sigs:  make(chan os.Signal, 2),
		clk:   clock.NewFake(time.Date(2026, 10, 12, 8, 0, 0, 0, time.UTC)),
		logs:  &syncBuffer{},
		rec:   &recorder{},
		total: total,
	}
	h.r = lifecycle.New(lifecycle.Options{
		Clock:        h.clk,
		Logger:       slog.New(slog.NewTextHandler(h.logs, nil)),
		TotalTimeout: total,
		Signals:      h.sigs,
	})
	return h
}

// runAsync runs the runner and returns the channel of its exit code.
func (h *harness) runAsync(ctx context.Context) <-chan int {
	done := make(chan int, 1)
	go func() { done <- h.r.Run(ctx) }()
	return done
}

func wait(t *testing.T, done <-chan int) int {
	t.Helper()
	select {
	case code := <-done:
		return code
	case <-time.After(10 * time.Second):
		t.Fatal("Run did not return")
		return -1
	}
}

// ready is a component that signals when the start reached it.
func ready(rec *recorder) (*component, <-chan struct{}) {
	ch := make(chan struct{})
	return &component{name: "ready", rec: rec, start: func(context.Context) error {
		close(ch)
		return nil
	}}, ch
}

func equal(t *testing.T, got, want []string) {
	t.Helper()
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("order = %q, want %q", got, want)
	}
}

func TestStartOrderAndRollback(t *testing.T) {
	t.Parallel()
	h := newHarness(t, time.Minute)
	// The first two components register stop steps of the same phase while they start; the
	// third fails, so the plan stops the second, then the first.
	a := &component{name: "a", rec: h.rec, r: h.r}
	sa := step(h.rec, "a", lifecycle.PhaseDatabase, time.Second)
	a.stop = &sa
	b := &component{name: "b", rec: h.rec, r: h.r}
	sb := step(h.rec, "b", lifecycle.PhaseDatabase, time.Second)
	b.stop = &sb
	c := &component{name: "c", rec: h.rec, r: h.r, err: errors.New("boom")}
	never := &component{name: "never", rec: h.rec, r: h.r}
	for _, comp := range []*component{a, b, c, never} {
		h.r.Add(comp)
	}
	code := wait(t, h.runAsync(context.Background()))
	if code != lifecycle.ExitFailure {
		t.Fatalf("exit = %d, want %d", code, lifecycle.ExitFailure)
	}
	equal(t, h.rec.got(), []string{"start a", "start b", "start c", "stop b", "stop a"})
	if err := h.r.Err(); err == nil || !strings.Contains(err.Error(), "c") || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("Err() = %v, want the error of component c", err)
	}
	if !strings.Contains(h.logs.String(), "start failed") {
		t.Fatalf("logs lack the start failure:\n%s", h.logs)
	}
}

// roleSteps registers the steps a role registers in wire and returns the expected order of
// 02 "Shutdown" for that role.
func roleSteps(rec *recorder, r *lifecycle.Runner, role string) []string {
	add := func(name string, p lifecycle.Phase) { r.OnStop(step(rec, name, p, time.Second)) }
	// Registered in an order unlike the phases: the phase decides, not the registration.
	add("pool", lifecycle.PhaseDatabase)
	add("internal", lifecycle.PhaseInternalListener)
	add("telemetry", lifecycle.PhaseTelemetry)
	add("not_ready", lifecycle.PhaseNotReady)
	api := role == "api" || role == "all"
	worker := role == "worker" || role == "all"
	if worker {
		add("river", lifecycle.PhaseJobs)
		add("consumers", lifecycle.PhaseConsumers)
	}
	if api {
		add("tail", lifecycle.PhaseEventTail)
		add("gateway", lifecycle.PhaseConnections)
		add("http", lifecycle.PhaseHTTP)
		add("drain", lifecycle.PhaseDrain)
	}
	want := []string{"stop not_ready"}
	if api {
		want = append(want, "stop drain", "stop http", "stop gateway", "stop tail")
	}
	if worker {
		want = append(want, "stop consumers", "stop river")
	}
	return append(want, "stop telemetry", "stop pool", "stop internal")
}

func TestStopPhasesPerRole(t *testing.T) {
	t.Parallel()
	for _, role := range []string{"api", "worker", "all"} {
		t.Run(role, func(t *testing.T) {
			t.Parallel()
			h := newHarness(t, time.Minute)
			want := roleSteps(h.rec, h.r, role)
			rc, readyCh := ready(h.rec)
			h.r.Add(rc)
			done := h.runAsync(context.Background())
			waitCh(t, readyCh)
			h.sigs <- syscall.SIGTERM
			if code := wait(t, done); code != lifecycle.ExitOK {
				t.Fatalf("exit = %d, want 0; logs:\n%s", code, h.logs)
			}
			equal(t, h.rec.got()[1:], want)
		})
	}
}

func TestPhaseNames(t *testing.T) {
	t.Parallel()
	want := []string{"not_ready", "drain", "http", "connections", "event_tail", "consumers", "jobs", "telemetry", "database", "internal_listener"}
	for i, p := range lifecycle.Phases() {
		if p.String() != want[i] {
			t.Errorf("phase %d = %q, want %q", i, p, want[i])
		}
	}
	if len(lifecycle.Phases()) != len(want) {
		t.Errorf("Phases() has %d entries, want %d", len(lifecycle.Phases()), len(want))
	}
	if got := lifecycle.Phase(99).String(); got != "phase_99" {
		t.Errorf("unknown phase = %q", got)
	}
}

func TestStepBudgetIsMinOfOwnAndRemaining(t *testing.T) {
	t.Parallel()
	h := newHarness(t, time.Minute)
	got := map[string]time.Duration{}
	var mu sync.Mutex
	record := func(name string, advance time.Duration) func(ctx context.Context) error {
		return func(ctx context.Context) error {
			deadline, ok := ctx.Deadline()
			if !ok {
				t.Errorf("%s: no deadline", name)
			}
			mu.Lock()
			got[name] = time.Until(deadline)
			mu.Unlock()
			h.clk.Advance(advance)
			return nil
		}
	}
	// 60s total: drain (own 40s) overruns to 50s, so http (own 15s) gets the remaining 10s.
	// The zero budget of not_ready means "the rest".
	h.r.OnStop(lifecycle.StopStep{Name: "not_ready", Phase: lifecycle.PhaseNotReady, Run: record("not_ready", 0)})
	h.r.OnStop(lifecycle.StopStep{Name: "drain", Phase: lifecycle.PhaseDrain, Budget: 40 * time.Second, Run: record("drain", 50*time.Second)})
	h.r.OnStop(lifecycle.StopStep{Name: "http", Phase: lifecycle.PhaseHTTP, Budget: 15 * time.Second, Run: record("http", 0)})
	rc, readyCh := ready(h.rec)
	h.r.Add(rc)
	done := h.runAsync(context.Background())
	waitCh(t, readyCh)
	h.sigs <- syscall.SIGTERM
	if code := wait(t, done); code != lifecycle.ExitOK {
		t.Fatalf("exit = %d, want 0", code)
	}
	near := func(name string, want time.Duration) {
		t.Helper()
		if d := got[name]; d > want || d < want-2*time.Second {
			t.Errorf("%s budget = %v, want about %v", name, d, want)
		}
	}
	near("not_ready", time.Minute)
	near("drain", 40*time.Second)
	near("http", 10*time.Second)
}

func TestShutdownOverBudgetExits1(t *testing.T) {
	t.Parallel()
	h := newHarness(t, time.Minute)
	h.r.OnStop(lifecycle.StopStep{Name: "drain", Phase: lifecycle.PhaseDrain, Budget: 30 * time.Second, Run: func(context.Context) error {
		h.clk.Advance(61 * time.Second) // the step overran the whole total
		return nil
	}})
	h.r.OnStop(step(h.rec, "telemetry", lifecycle.PhaseTelemetry, 5*time.Second))
	rc, readyCh := ready(h.rec)
	h.r.Add(rc)
	done := h.runAsync(context.Background())
	waitCh(t, readyCh)
	h.sigs <- syscall.SIGTERM
	if code := wait(t, done); code != lifecycle.ExitFailure {
		t.Fatalf("exit = %d, want 1", code)
	}
	equal(t, h.rec.got(), []string{"start ready"})
	logs := h.logs.String()
	if !strings.Contains(logs, "shutdown budget exceeded") || !strings.Contains(logs, "telemetry") {
		t.Fatalf("logs lack the budget line naming the phase:\n%s", logs)
	}
}

func TestLastStepOverBudgetExits1(t *testing.T) {
	t.Parallel()
	h := newHarness(t, time.Minute)
	h.r.OnStop(lifecycle.StopStep{Name: "pool", Phase: lifecycle.PhaseDatabase, Run: func(context.Context) error {
		h.clk.Advance(2 * time.Minute)
		return nil
	}})
	rc, readyCh := ready(h.rec)
	h.r.Add(rc)
	done := h.runAsync(context.Background())
	waitCh(t, readyCh)
	h.sigs <- syscall.SIGINT
	if code := wait(t, done); code != lifecycle.ExitFailure {
		t.Fatalf("exit = %d, want 1", code)
	}
	if !strings.Contains(h.logs.String(), "shutdown budget exceeded") {
		t.Fatalf("logs:\n%s", h.logs)
	}
}

func TestStepTimeoutMovesOn(t *testing.T) {
	t.Parallel()
	h := newHarness(t, time.Minute)
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	h.r.OnStop(lifecycle.StopStep{Name: "http", Phase: lifecycle.PhaseHTTP, Budget: 20 * time.Millisecond, Run: func(context.Context) error {
		<-release // ignores its context
		return nil
	}})
	h.r.OnStop(lifecycle.StopStep{Name: "pool", Phase: lifecycle.PhaseDatabase, Run: func(context.Context) error {
		return errors.New("close failed")
	}})
	h.r.OnStop(lifecycle.StopStep{Name: "panics", Phase: lifecycle.PhaseInternalListener, Run: func(context.Context) error {
		panic("listener")
	}})
	rc, readyCh := ready(h.rec)
	h.r.Add(rc)
	done := h.runAsync(context.Background())
	waitCh(t, readyCh)
	h.sigs <- syscall.SIGTERM
	if code := wait(t, done); code != lifecycle.ExitOK {
		t.Fatalf("exit = %d, want 0 (a slow or failing step is logged, the total held)", code)
	}
	logs := h.logs.String()
	for _, want := range []string{"shutdown step timed out", "shutdown step failed", "shutdown complete"} {
		if !strings.Contains(logs, want) {
			t.Errorf("logs lack %q:\n%s", want, logs)
		}
	}
}

// TestWaitStepReachingItsBudgetIsNoTimeout: a step that waits its budget by design (the drain
// delay) ends normally when the budget is spent; the runner logs no timeout for it.
func TestWaitStepReachingItsBudgetIsNoTimeout(t *testing.T) {
	t.Parallel()
	for range 20 {
		h := newHarness(t, time.Minute)
		h.r.OnStop(lifecycle.StopStep{Name: "drain", Phase: lifecycle.PhaseDrain, Budget: 5 * time.Millisecond, Wait: true, Run: func(ctx context.Context) error {
			<-ctx.Done()
			return nil
		}})
		rc, readyCh := ready(h.rec)
		h.r.Add(rc)
		done := h.runAsync(context.Background())
		waitCh(t, readyCh)
		h.sigs <- syscall.SIGTERM
		if code := wait(t, done); code != lifecycle.ExitOK {
			t.Fatalf("exit = %d, want 0", code)
		}
		if logs := h.logs.String(); strings.Contains(logs, "shutdown step timed out") {
			t.Fatalf("a wait step that used its budget was logged as timed out:\n%s", logs)
		}
	}
}

func TestSecondSignalForcesExit3(t *testing.T) {
	t.Parallel()
	h := newHarness(t, time.Minute)
	entered := make(chan struct{})
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	h.r.OnStop(lifecycle.StopStep{Name: "drain", Phase: lifecycle.PhaseDrain, Budget: 30 * time.Second, Run: func(ctx context.Context) error {
		close(entered)
		<-release
		return nil
	}})
	h.r.OnStop(step(h.rec, "pool", lifecycle.PhaseDatabase, time.Second))
	rc, readyCh := ready(h.rec)
	h.r.Add(rc)
	done := h.runAsync(context.Background())
	waitCh(t, readyCh)
	h.sigs <- syscall.SIGTERM
	waitCh(t, entered)
	h.sigs <- syscall.SIGTERM
	if code := wait(t, done); code != lifecycle.ExitForced {
		t.Fatalf("exit = %d, want 3", code)
	}
	equal(t, h.rec.got(), []string{"start ready"})
	if !strings.Contains(h.logs.String(), "forced shutdown") {
		t.Fatalf("logs:\n%s", h.logs)
	}
}

func TestSignalDuringStartStopsStarted(t *testing.T) {
	t.Parallel()
	h := newHarness(t, time.Minute)
	blocking := make(chan struct{})
	a := &component{name: "a", rec: h.rec, r: h.r}
	sa := step(h.rec, "a", lifecycle.PhaseDatabase, time.Second)
	a.stop = &sa
	slow := &component{name: "slow", rec: h.rec, r: h.r, start: func(ctx context.Context) error {
		close(blocking)
		<-ctx.Done() // a dependency wait ends with the start context
		return ctx.Err()
	}}
	after := &component{name: "after", rec: h.rec, r: h.r}
	h.r.Add(a)
	h.r.Add(slow)
	h.r.Add(after)
	done := h.runAsync(context.Background())
	waitCh(t, blocking)
	h.sigs <- syscall.SIGTERM
	if code := wait(t, done); code != lifecycle.ExitOK {
		t.Fatalf("exit = %d, want 0 (orderly end on request)", code)
	}
	equal(t, h.rec.got(), []string{"start a", "start slow", "stop a"})
}

func TestSecondSignalDuringStartForcesExit3(t *testing.T) {
	t.Parallel()
	h := newHarness(t, time.Minute)
	blocking := make(chan struct{})
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	h.r.Add(&component{name: "stuck", rec: h.rec, r: h.r, start: func(context.Context) error {
		close(blocking)
		<-release // ignores the start context
		return nil
	}})
	done := h.runAsync(context.Background())
	waitCh(t, blocking)
	h.sigs <- syscall.SIGTERM
	h.sigs <- syscall.SIGTERM
	if code := wait(t, done); code != lifecycle.ExitForced {
		t.Fatalf("exit = %d, want 3", code)
	}
}

func TestFailTriggersShutdownExit1(t *testing.T) {
	t.Parallel()
	h := newHarness(t, time.Minute)
	h.r.OnStop(step(h.rec, "pool", lifecycle.PhaseDatabase, time.Second))
	rc, readyCh := ready(h.rec)
	h.r.Add(rc)
	done := h.runAsync(context.Background())
	waitCh(t, readyCh)
	h.r.Fail(errors.New("log tail panicked"))
	h.r.Fail(errors.New("second report is ignored"))
	if code := wait(t, done); code != lifecycle.ExitFailure {
		t.Fatalf("exit = %d, want 1", code)
	}
	equal(t, h.rec.got(), []string{"start ready", "stop pool"})
	if err := h.r.Err(); err == nil || !strings.Contains(err.Error(), "log tail panicked") {
		t.Fatalf("Err() = %v", err)
	}
	if !strings.Contains(h.logs.String(), "fatal runtime error") {
		t.Fatalf("logs:\n%s", h.logs)
	}
}

func TestFailDuringStartExit1(t *testing.T) {
	t.Parallel()
	h := newHarness(t, time.Minute)
	blocking := make(chan struct{})
	h.r.Add(&component{name: "waits", rec: h.rec, r: h.r, start: func(ctx context.Context) error {
		close(blocking)
		<-ctx.Done()
		return ctx.Err()
	}})
	done := h.runAsync(context.Background())
	waitCh(t, blocking)
	h.r.Fail(errors.New("hub panicked"))
	if code := wait(t, done); code != lifecycle.ExitFailure {
		t.Fatalf("exit = %d, want 1", code)
	}
}

func TestContextEndIsOrderlyShutdown(t *testing.T) {
	t.Parallel()
	h := newHarness(t, time.Minute)
	h.r.OnStop(step(h.rec, "pool", lifecycle.PhaseDatabase, time.Second))
	rc, readyCh := ready(h.rec)
	h.r.Add(rc)
	ctx, cancel := context.WithCancel(context.Background())
	done := h.runAsync(ctx)
	waitCh(t, readyCh)
	cancel()
	if code := wait(t, done); code != lifecycle.ExitOK {
		t.Fatalf("exit = %d, want 0", code)
	}
	equal(t, h.rec.got(), []string{"start ready", "stop pool"})
}

func TestBudgetSumAboveTotalFailsAtWiring(t *testing.T) {
	t.Parallel()
	h := newHarness(t, 10*time.Second)
	h.r.OnStop(step(h.rec, "drain", lifecycle.PhaseDrain, 6*time.Second))
	h.r.OnStop(step(h.rec, "http", lifecycle.PhaseHTTP, 5*time.Second))
	h.r.Add(&component{name: "never", rec: h.rec, r: h.r})
	if code := wait(t, h.runAsync(context.Background())); code != lifecycle.ExitUsage {
		t.Fatalf("exit = %d, want 2", code)
	}
	if len(h.rec.got()) != 0 {
		t.Fatalf("something ran: %q", h.rec.got())
	}
	err := h.r.Err()
	if err == nil || !strings.Contains(err.Error(), "11s") || !strings.Contains(err.Error(), "10s") || !strings.Contains(err.Error(), "shutdown.total_timeout") {
		t.Fatalf("Err() = %v, want both values and the key", err)
	}
}

func TestRunTwicePanics(t *testing.T) {
	t.Parallel()
	h := newHarness(t, time.Minute)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if code := h.r.Run(ctx); code != lifecycle.ExitOK {
		t.Fatalf("first Run = %d", code)
	}
	defer func() {
		if recover() == nil {
			t.Fatal("second Run did not panic")
		}
	}()
	h.r.Run(ctx)
}

func TestNewDefaults(t *testing.T) {
	t.Parallel()
	// Nil clock and logger fall back to the system clock and slog.Default; a zero total is a
	// programming error.
	r := lifecycle.New(lifecycle.Options{TotalTimeout: time.Second, Signals: make(chan os.Signal, 2)})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if code := r.Run(ctx); code != lifecycle.ExitOK {
		t.Fatalf("Run = %d", code)
	}
	defer func() {
		if recover() == nil {
			t.Fatal("New with a zero total did not panic")
		}
	}()
	lifecycle.New(lifecycle.Options{Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
}

func TestOSSignals(t *testing.T) {
	// Not parallel: it signals the test process itself. With nil Signals the runner listens
	// on os/signal for SIGTERM and SIGINT.
	logs := &syncBuffer{}
	r := lifecycle.New(lifecycle.Options{TotalTimeout: time.Minute, Logger: slog.New(slog.NewTextHandler(logs, nil))})
	rec := &recorder{}
	rc, readyCh := ready(rec)
	r.Add(rc)
	done := make(chan int, 1)
	go func() { done <- r.Run(context.Background()) }()
	waitCh(t, readyCh)
	p, err := os.FindProcess(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	if code := wait(t, done); code != lifecycle.ExitOK {
		t.Fatalf("exit = %d", code)
	}
}

// waitCh fails the test when ch is not closed within ten seconds, so a broken runner cannot
// hang the test binary.
func waitCh(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for the start to reach the component")
	}
}

func TestTotalCountsFromTheShutdownRequest(t *testing.T) {
	t.Parallel()
	h := newHarness(t, time.Minute)
	h.r.OnStop(step(h.rec, "pool", lifecycle.PhaseDatabase, time.Second))
	blocking := make(chan struct{})
	h.r.Add(&component{name: "slow", rec: h.rec, r: h.r, start: func(ctx context.Context) error {
		close(blocking)
		<-ctx.Done()
		// The component needs longer than the whole total to give up its wait.
		h.clk.Advance(61 * time.Second)
		return ctx.Err()
	}})
	done := h.runAsync(context.Background())
	waitCh(t, blocking)
	h.sigs <- syscall.SIGTERM
	if code := wait(t, done); code != lifecycle.ExitFailure {
		t.Fatalf("exit = %d, want 1: the total starts with the signal, not after the start returned", code)
	}
	equal(t, h.rec.got(), []string{"start slow"})
	if !strings.Contains(h.logs.String(), "shutdown budget exceeded") {
		t.Fatalf("logs:\n%s", h.logs)
	}
	if err := h.r.Err(); err == nil || !strings.Contains(err.Error(), "shutdown.total_timeout") || !strings.Contains(err.Error(), "database") {
		t.Fatalf("Err() = %v, want the spent total and the skipped phase", err)
	}
}
