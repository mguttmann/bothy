// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"slices"
	"strings"
	stdsync "sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/sync"
	"github.com/OWNER/bothy/services/server/internal/sync/app"
	"github.com/OWNER/bothy/services/server/internal/sync/domain"
)

// consumerConfig is a valid configuration for the unit tests.
func consumerConfig() app.ConsumerConfig {
	return app.ConsumerConfig{BatchSize: 3, PollInterval: time.Second, ReconnectBackoffCap: 10 * time.Second, StatesTimeout: time.Second}
}

// fakeCursors is a ConsumerCursorStore in memory.
type fakeCursors struct {
	mu       stdsync.Mutex
	rows     map[sync.ConsumerName]int64
	locked   map[sync.ConsumerName]bool
	ensured  []sync.ConsumerName
	advanced []int64
	lockErr  error
}

func newFakeCursors() *fakeCursors {
	return &fakeCursors{rows: map[sync.ConsumerName]int64{}, locked: map[sync.ConsumerName]bool{}}
}

func (c *fakeCursors) EnsureCursor(_ context.Context, name sync.ConsumerName) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ensured = append(c.ensured, name)
	if _, ok := c.rows[name]; !ok {
		c.rows[name] = 0
	}
	return nil
}

func (c *fakeCursors) LockCursor(_ context.Context, name sync.ConsumerName) (int64, bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.lockErr != nil {
		return 0, false, c.lockErr
	}
	last, ok := c.rows[name]
	if !ok || c.locked[name] {
		return 0, false, nil
	}
	return last, true, nil
}

func (c *fakeCursors) AdvanceCursor(_ context.Context, name sync.ConsumerName, lastSeq int64) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.advanced = append(c.advanced, lastSeq)
	c.rows[name] = lastSeq
	return nil
}

func (c *fakeCursors) ListCursors(context.Context) ([]app.CursorRow, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	var out []app.CursorRow
	for n, last := range c.rows {
		out = append(out, app.CursorRow{Name: n, LastSeq: last, HeadSeq: 9})
	}
	slices.SortFunc(out, func(a, b app.CursorRow) int { return strings.Compare(string(a.Name), string(b.Name)) })
	return out, nil
}

// fakeLog is a ConsumerLogReader over a list of seqs.
type fakeLog struct {
	seqs        []int64
	minRetained int64
	minReads    int
	at          time.Time
}

func (l *fakeLog) EventsAfter(_ context.Context, after int64, limit int) ([]sync.Event, error) {
	var out []sync.Event
	for _, s := range l.seqs {
		if s > after && len(out) < limit {
			out = append(out, sync.Event{Seq: s})
		}
	}
	return out, nil
}

func (l *fakeLog) MinRetainedSeq(context.Context) (int64, error) {
	l.minReads++
	return l.minRetained, nil
}

func (l *fakeLog) OldestEventAfter(_ context.Context, after int64) (time.Time, bool, error) {
	for _, s := range l.seqs {
		if s > after {
			return l.at.Add(time.Duration(s) * time.Second), true, nil
		}
	}
	return time.Time{}, false, nil
}

// recordingHandler records the seqs it handled and fails or panics at fail.
type recordingHandler struct {
	seen    []int64
	fail    int64
	panicAt int64
	before  func(ctx context.Context) error
}

func (h *recordingHandler) HandleEvent(ctx context.Context, ev sync.Event) error {
	if h.panicAt != 0 && ev.Seq == h.panicAt {
		panic("probe handler panic")
	}
	if h.fail != 0 && ev.Seq == h.fail {
		return errors.New("probe handler failure")
	}
	if h.before != nil {
		if err := h.before(ctx); err != nil {
			return err
		}
	}
	h.seen = append(h.seen, ev.Seq)
	return nil
}

func newService(t *testing.T, cursors app.ConsumerCursorStore, log app.ConsumerLogReader, logger *slog.Logger) *app.ConsumerService {
	t.Helper()
	s, err := app.NewConsumerService(consumerConfig(), &fakeUoW{}, cursors, log, logger)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestNewConsumerService_RejectsBadConfig(t *testing.T) {
	t.Parallel()
	for _, bad := range []app.ConsumerConfig{
		{BatchSize: 0, PollInterval: time.Second, ReconnectBackoffCap: time.Second, StatesTimeout: time.Second},
		{BatchSize: 1, PollInterval: 0, ReconnectBackoffCap: time.Second, StatesTimeout: time.Second},
		{BatchSize: 1, PollInterval: time.Second, ReconnectBackoffCap: 0, StatesTimeout: time.Second},
		{BatchSize: 1, PollInterval: time.Second, ReconnectBackoffCap: time.Second, StatesTimeout: 0},
	} {
		if _, err := app.NewConsumerService(bad, &fakeUoW{}, newFakeCursors(), &fakeLog{}, nil); err == nil {
			t.Errorf("config %+v was accepted", bad)
		}
	}
}

// TestConsumerRegistration: a duplicate name, an invalid name, a nil handler and a registration
// after the runner started fail (design note 2 of M2-01).
func TestConsumerRegistration(t *testing.T) {
	t.Parallel()
	s := newService(t, newFakeCursors(), &fakeLog{}, nil)
	if err := s.Register(sync.ConsumerWebhooks, &recordingHandler{}); err != nil {
		t.Fatal(err)
	}
	if err := s.Register(sync.ConsumerWebhooks, &recordingHandler{}); err == nil {
		t.Error("a second registration of webhooks was accepted")
	}
	if err := s.Register("agents", &recordingHandler{}); err == nil {
		t.Error("an invalid name was accepted")
	}
	if err := s.Register(sync.ConsumerPush, nil); err == nil {
		t.Error("a nil handler was accepted")
	}
	r, err := app.NewConsumerRunner(s, newFakeWaker(), consumerConfig(), app.RunnerOptions{After: newFakeAfter().after})
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = r.Stop(context.Background()) })
	if err := s.Register(sync.ConsumerPush, &recordingHandler{}); err == nil {
		t.Error("a registration after Start was accepted")
	}
	if got := s.Names(); !slices.Equal(got, []sync.ConsumerName{sync.ConsumerWebhooks}) {
		t.Errorf("Names = %v", got)
	}
	if err := r.Start(context.Background()); err == nil {
		t.Error("a second Start was accepted")
	}
}

func TestConsumerEnsure_OneRowPerRegisteredName(t *testing.T) {
	t.Parallel()
	cursors := newFakeCursors()
	s := newService(t, cursors, &fakeLog{}, nil)
	for _, n := range []sync.ConsumerName{sync.ConsumerWebhooks, sync.ConsumerPush} {
		if err := s.Register(n, &recordingHandler{}); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.Ensure(context.Background()); err != nil {
		t.Fatal(err)
	}
	if want := []sync.ConsumerName{sync.ConsumerPush, sync.ConsumerWebhooks}; !slices.Equal(cursors.ensured, want) {
		t.Fatalf("ensured %v, want %v", cursors.ensured, want)
	}
}

func TestConsumerRound_Outcomes(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	cursors := newFakeCursors()
	log := &fakeLog{seqs: []int64{1, 2, 3, 4}}
	h := &recordingHandler{}
	s := newService(t, cursors, log, nil)
	if err := s.Register(sync.ConsumerWebhooks, h); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Round(ctx, sync.ConsumerPush); errs.KindOf(err) != errs.KindInternal {
		t.Errorf("Round of an unregistered consumer = %v, want kind internal", err)
	}
	if res, err := s.Round(ctx, sync.ConsumerWebhooks); err != nil || res.Outcome != app.RoundLocked {
		t.Fatalf("Round without a row = %+v, %v; want RoundLocked", res, err)
	}
	if err := s.Ensure(ctx); err != nil {
		t.Fatal(err)
	}
	res, err := s.Round(ctx, sync.ConsumerWebhooks)
	if err != nil || res != (app.RoundResult{Outcome: app.RoundProcessed, Processed: 3, Full: true}) {
		t.Fatalf("first round = %+v, %v; want 3 processed, full", res, err)
	}
	res, err = s.Round(ctx, sync.ConsumerWebhooks)
	if err != nil || res != (app.RoundResult{Outcome: app.RoundProcessed, Processed: 1}) {
		t.Fatalf("second round = %+v, %v; want 1 processed, short", res, err)
	}
	res, err = s.Round(ctx, sync.ConsumerWebhooks)
	if err != nil || res.Outcome != app.RoundIdle {
		t.Fatalf("third round = %+v, %v; want RoundIdle", res, err)
	}
	if !slices.Equal(h.seen, []int64{1, 2, 3, 4}) || !slices.Equal(cursors.advanced, []int64{3, 4}) {
		t.Fatalf("handled %v, advanced %v", h.seen, cursors.advanced)
	}
	if log.minReads != 0 {
		t.Errorf("the minimum was read %d times for contiguous batches", log.minReads)
	}
	cursors.locked[sync.ConsumerWebhooks] = true
	if res, err := s.Round(ctx, sync.ConsumerWebhooks); err != nil || res.Outcome != app.RoundLocked {
		t.Fatalf("round on a locked row = %+v, %v", res, err)
	}
}

// TestConsumerRound_FailuresRollBackWhole: a handler error, a panic, a gap and a durable event
// appended by the handler fail the round before the cursor moves (design notes 5 to 7).
func TestConsumerRound_FailuresRollBackWhole(t *testing.T) {
	t.Parallel()
	appender := newAppender(t, fixedCorrelation{})
	cases := []struct {
		name  string
		seqs  []int64
		h     *recordingHandler
		check func(error) bool
	}{
		{"handler error", []int64{1, 2}, &recordingHandler{fail: 2}, func(err error) bool { return err != nil }},
		{"handler panic", []int64{1, 2}, &recordingHandler{panicAt: 2}, func(err error) bool { return errs.KindOf(err) == errs.KindInternal }},
		{"hole", []int64{1, 3}, &recordingHandler{}, func(err error) bool { return errors.Is(err, domain.ErrConsumerGap) }},
		{"jump", []int64{2, 3}, &recordingHandler{}, func(err error) bool { return errors.Is(err, domain.ErrConsumerGap) }},
		{"appended event", []int64{1}, &recordingHandler{before: func(ctx context.Context) error {
			return appender.Append(ctx, event())
		}}, func(err error) bool { return errs.KindOf(err) == errs.KindInternal }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			cursors := newFakeCursors()
			s := newService(t, cursors, &fakeLog{seqs: c.seqs, minRetained: 1}, nil)
			if err := s.Register(sync.ConsumerWebhooks, c.h); err != nil {
				t.Fatal(err)
			}
			if err := s.Ensure(context.Background()); err != nil {
				t.Fatal(err)
			}
			res, err := s.Round(context.Background(), sync.ConsumerWebhooks)
			if !c.check(err) || res != (app.RoundResult{}) {
				t.Fatalf("Round = %+v, %v", res, err)
			}
			if len(cursors.advanced) != 0 {
				t.Fatalf("the cursor moved to %v", cursors.advanced)
			}
		})
	}
}

// TestConsumerRound_RetentionLossIsLogged: events removed by retention are skipped, and one
// error line names the consumer and the first seq processed, never a payload.
func TestConsumerRound_RetentionLossIsLogged(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	cursors := newFakeCursors()
	log := &fakeLog{seqs: []int64{5, 6}, minRetained: 5}
	h := &recordingHandler{}
	s := newService(t, cursors, log, logger)
	if err := s.Register(sync.ConsumerPush, h); err != nil {
		t.Fatal(err)
	}
	if err := s.Ensure(context.Background()); err != nil {
		t.Fatal(err)
	}
	res, err := s.Round(context.Background(), sync.ConsumerPush)
	if err != nil || res.Processed != 2 || !slices.Equal(h.seen, []int64{5, 6}) || log.minReads != 1 {
		t.Fatalf("Round = %+v, %v; handled %v, minimum read %d times", res, err, h.seen, log.minReads)
	}
	line := buf.String()
	for _, want := range []string{`"level":"ERROR"`, `"msg":"event consumer lost events to retention"`, `"bothy.consumer.name":"push"`, `"bothy.event.seq":5`} {
		if !strings.Contains(line, want) {
			t.Errorf("log %q lacks %s", line, want)
		}
	}
}

func TestConsumerStates(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)
	cursors := newFakeCursors()
	cursors.rows[sync.ConsumerWebhooks] = 7
	cursors.rows[sync.ConsumerPush] = 9
	s := newService(t, cursors, &fakeLog{seqs: []int64{8, 9}, at: at}, nil)
	got, err := s.States(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := []sync.ConsumerState{
		{Name: sync.ConsumerPush, LastSeq: 9, HeadSeq: 9},
		{Name: sync.ConsumerWebhooks, LastSeq: 7, HeadSeq: 9, OldestPendingAt: at.Add(8 * time.Second)},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("States = %+v, want %+v", got, want)
	}
}

// ---- runner ---------------------------------------------------------------------------------

// fakeWaker is a Waker whose signals the test sends.
type fakeWaker struct {
	wake   chan struct{}
	runErr error
	ran    chan struct{}
}

func newFakeWaker() *fakeWaker {
	return &fakeWaker{wake: make(chan struct{}), ran: make(chan struct{}, 1)}
}

func (w *fakeWaker) Run(ctx context.Context) error {
	w.ran <- struct{}{}
	if w.runErr != nil {
		return w.runErr
	}
	<-ctx.Done()
	return nil
}

func (w *fakeWaker) Wake() <-chan struct{} { return w.wake }

// fakeAfter is the injected timer: every call records its duration and returns a channel the
// test fires.
type fakeAfter struct {
	mu    stdsync.Mutex
	calls []time.Duration
	chans []chan time.Time
	asked chan struct{}
}

func newFakeAfter() *fakeAfter { return &fakeAfter{asked: make(chan struct{}, 100)} }

func (a *fakeAfter) after(d time.Duration) <-chan time.Time {
	a.mu.Lock()
	defer a.mu.Unlock()
	ch := make(chan time.Time, 1)
	a.calls = append(a.calls, d)
	a.chans = append(a.chans, ch)
	a.asked <- struct{}{}
	return ch
}

// fire fires the newest timer.
func (a *fakeAfter) fire() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.chans[len(a.chans)-1] <- time.Time{}
}

// fakeRounds returns scripted round results and counts the rounds.
type fakeRounds struct {
	mu      stdsync.Mutex
	names   []sync.ConsumerName
	script  []roundStep
	rounds  int
	started chan struct{}
	sealed  bool
}

// roundStep is one scripted round: block until release (when set), then return res and err;
// with honourCtx the round ends with the context error when its context ends first.
type roundStep struct {
	res       app.RoundResult
	err       error
	release   chan struct{}
	honourCtx bool
}

func (f *fakeRounds) Seal() []sync.ConsumerName {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sealed = true
	return f.names
}

func (f *fakeRounds) Round(ctx context.Context, _ sync.ConsumerName) (app.RoundResult, error) {
	f.mu.Lock()
	step := roundStep{res: app.RoundResult{Outcome: app.RoundIdle}}
	if len(f.script) > 0 {
		step, f.script = f.script[0], f.script[1:]
	}
	f.rounds++
	f.mu.Unlock()
	if f.started != nil {
		f.started <- struct{}{}
	}
	if step.release != nil {
		if step.honourCtx {
			select {
			case <-step.release:
			case <-ctx.Done():
				return app.RoundResult{}, ctx.Err()
			}
		} else {
			<-step.release
		}
	}
	return step.res, step.err
}

func (f *fakeRounds) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.rounds
}

// startRunner starts a runner over rounds with the fake waker and timer.
func startRunner(t *testing.T, rounds app.Rounds, w app.Waker, a *fakeAfter, logger *slog.Logger) *app.ConsumerRunner {
	t.Helper()
	r, err := app.NewConsumerRunner(rounds, w, consumerConfig(), app.RunnerOptions{After: a.after, Logger: logger})
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = r.Stop(context.Background()) })
	return r
}

// waitAsked waits until the runner asked for its next timer.
func waitAsked(t *testing.T, a *fakeAfter) {
	t.Helper()
	select {
	case <-a.asked:
	case <-time.After(5 * time.Second):
		t.Fatal("the runner did not start to wait")
	}
}

// settle gives the runner goroutines time to do what they should not.
func settle() { time.Sleep(50 * time.Millisecond) }

func processed(n int, full bool) app.RoundResult {
	return app.RoundResult{Outcome: app.RoundProcessed, Processed: n, Full: full}
}

// TestRunnerDrainsFullBatches: after a full batch the next round starts at once; only after the
// short batch the runner waits (D-M2-01-8).
func TestRunnerDrainsFullBatches(t *testing.T) {
	t.Parallel()
	rounds := &fakeRounds{names: []sync.ConsumerName{sync.ConsumerWebhooks},
		script: []roundStep{{res: processed(3, true)}, {res: processed(3, true)}, {res: processed(1, false)}}}
	a := newFakeAfter()
	startRunner(t, rounds, newFakeWaker(), a, nil)
	waitAsked(t, a)
	if got := rounds.count(); got != 3 {
		t.Fatalf("%d rounds before the first wait, want 3", got)
	}
	if a.calls[0] != time.Second {
		t.Fatalf("waited %s, want consumer.poll_interval", a.calls[0])
	}
	settle()
	if got := rounds.count(); got != 3 {
		t.Fatalf("%d rounds while waiting, want 3", got)
	}
}

// TestRunnerWaitsAfterShortLockedOrFailedRound: a short batch, an idle round, a locked row and a
// failed round each wait for the next wake or poll; the poll and the wake each start one round.
func TestRunnerWaitsAfterShortLockedOrFailedRound(t *testing.T) {
	t.Parallel()
	for name, step := range map[string]roundStep{
		"short":  {res: processed(1, false)},
		"idle":   {res: app.RoundResult{Outcome: app.RoundIdle}},
		"locked": {res: app.RoundResult{Outcome: app.RoundLocked}},
		"failed": {err: errors.New("round failed")},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			rounds := &fakeRounds{names: []sync.ConsumerName{sync.ConsumerPush}, script: []roundStep{step, step, step}}
			a := newFakeAfter()
			w := newFakeWaker()
			startRunner(t, rounds, w, a, nil)
			waitAsked(t, a)
			settle()
			if got := rounds.count(); got != 1 {
				t.Fatalf("%d rounds before a wake or poll, want 1", got)
			}
			a.fire()
			waitAsked(t, a)
			if got := rounds.count(); got != 2 {
				t.Fatalf("%d rounds after the poll, want 2", got)
			}
			w.wake <- struct{}{}
			waitAsked(t, a)
			if got := rounds.count(); got != 3 {
				t.Fatalf("%d rounds after the wake, want 3", got)
			}
		})
	}
}

// TestRunnerWakeSignalsCoalesce: notifications that arrive during a round wake the consumer once
// (capacity one, non-blocking send, design note 8), and one notification wakes every consumer.
func TestRunnerWakeSignalsCoalesce(t *testing.T) {
	t.Parallel()
	release := make(chan struct{})
	rounds := &fakeRounds{names: []sync.ConsumerName{sync.ConsumerWebhooks},
		script: []roundStep{{res: processed(1, false), release: release}}, started: make(chan struct{}, 4)}
	a := newFakeAfter()
	w := newFakeWaker()
	startRunner(t, rounds, w, a, nil)
	// The wakes must arrive while the first round runs, and dispatch must have forwarded the
	// last of them before the round ends: a forward that lands after the loop took the first
	// wake would start a third round (the send of a wake returns before dispatch forwards it).
	select {
	case <-rounds.started:
	case <-time.After(5 * time.Second):
		t.Fatal("the first round did not start")
	}
	for range 5 {
		w.wake <- struct{}{}
	}
	settle()
	close(release)
	waitAsked(t, a)
	waitAsked(t, a)
	settle()
	if got := rounds.count(); got != 2 {
		t.Fatalf("%d rounds after five wakes during one round, want 2", got)
	}

	two := &fakeRounds{names: []sync.ConsumerName{sync.ConsumerPush, sync.ConsumerWebhooks}}
	a2 := newFakeAfter()
	w2 := newFakeWaker()
	startRunner(t, two, w2, a2, nil)
	waitAsked(t, a2)
	waitAsked(t, a2)
	w2.wake <- struct{}{}
	waitAsked(t, a2)
	waitAsked(t, a2)
	if got := two.count(); got != 4 {
		t.Fatalf("%d rounds of two consumers after one wake, want 4", got)
	}
}

// TestRunnerStopStartsNoNewRound: Stop waits for the running round and starts no new one, even
// after a full batch; with an expired budget it cancels the round's context and returns.
func TestRunnerStopStartsNoNewRound(t *testing.T) {
	t.Parallel()
	t.Run("round finishes within the budget", func(t *testing.T) {
		t.Parallel()
		release := make(chan struct{})
		rounds := &fakeRounds{names: []sync.ConsumerName{sync.ConsumerWebhooks}, started: make(chan struct{}, 10),
			script: []roundStep{{res: processed(3, true), release: release}}}
		r := startRunner(t, rounds, newFakeWaker(), newFakeAfter(), nil)
		<-rounds.started
		stopped := make(chan error, 1)
		go func() { stopped <- r.Stop(context.Background()) }()
		select {
		case err := <-stopped:
			t.Fatalf("Stop returned %v while the round runs", err)
		case <-time.After(50 * time.Millisecond):
		}
		close(release)
		select {
		case err := <-stopped:
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("Stop did not return after the round ended")
		}
		settle()
		if got := rounds.count(); got != 1 {
			t.Fatalf("%d rounds, want 1: a full batch must not start a round after Stop", got)
		}
	})
	t.Run("budget expires", func(t *testing.T) {
		t.Parallel()
		rounds := &fakeRounds{names: []sync.ConsumerName{sync.ConsumerWebhooks}, started: make(chan struct{}, 10),
			script: []roundStep{{res: processed(3, true), release: make(chan struct{}), honourCtx: true}}}
		r := startRunner(t, rounds, newFakeWaker(), newFakeAfter(), nil)
		<-rounds.started
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()
		if err := r.Stop(ctx); !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("Stop = %v, want the deadline", err)
		}
		if err := r.Stop(context.Background()); err != nil {
			t.Fatalf("Stop after the cancelled round = %v", err)
		}
		if got := rounds.count(); got != 1 {
			t.Fatalf("%d rounds, want 1", got)
		}
	})
}

// TestRunnerLogsFailuresOnEdges: the first failure after a success is one error line, repeated
// failures add none, and the next success is one info line (design note 7).
func TestRunnerLogsFailuresOnEdges(t *testing.T) {
	t.Parallel()
	var mu stdsync.Mutex
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(writerFunc(func(p []byte) (int, error) {
		mu.Lock()
		defer mu.Unlock()
		return buf.Write(p)
	}), nil))
	boom := errors.New("round failed")
	rounds := &fakeRounds{names: []sync.ConsumerName{sync.ConsumerPush},
		script: []roundStep{{err: boom}, {err: boom}, {err: boom}, {res: processed(1, false)}}}
	a := newFakeAfter()
	startRunner(t, rounds, newFakeWaker(), a, logger)
	for range 3 {
		waitAsked(t, a)
		a.fire()
	}
	waitAsked(t, a)
	mu.Lock()
	out := buf.String()
	mu.Unlock()
	if n := strings.Count(out, `"msg":"event consumer round failed"`); n != 1 {
		t.Errorf("%d failure lines for three failed rounds, want 1:\n%s", n, out)
	}
	if n := strings.Count(out, `"msg":"event consumer recovered"`); n != 1 {
		t.Errorf("%d recovery lines, want 1:\n%s", n, out)
	}
	for _, want := range []string{`"bothy.consumer.name":"push"`, `"error.type":"`, `"level":"ERROR"`, `"level":"INFO"`} {
		if !strings.Contains(out, want) {
			t.Errorf("log lacks %s:\n%s", want, out)
		}
	}
	if strings.Contains(out, "round failed\"") && strings.Contains(out, `"error.type":"round failed"`) {
		t.Errorf("the log carries the error text:\n%s", out)
	}
}

// TestRunnerFailsWhenTheWakerEnds: a waker that ends with an error while the runner runs is an
// unexpected end and reaches Failed (D-M2-01-9).
func TestRunnerFailsWhenTheWakerEnds(t *testing.T) {
	t.Parallel()
	w := newFakeWaker()
	w.runErr = errors.New("listen loop ended")
	r := startRunner(t, &fakeRounds{names: []sync.ConsumerName{sync.ConsumerPush}}, w, newFakeAfter(), nil)
	select {
	case err := <-r.Failed():
		if err == nil {
			t.Fatal("Failed delivered nil")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the end of the waker did not reach Failed")
	}
}

// TestRunnerWithoutConsumersStartsNothing: with no registered consumer the runner opens no wake
// connection and starts no round (the production list is empty until M2-26 and M3-19).
func TestRunnerWithoutConsumersStartsNothing(t *testing.T) {
	t.Parallel()
	w := newFakeWaker()
	rounds := &fakeRounds{}
	r := startRunner(t, rounds, w, newFakeAfter(), nil)
	settle()
	select {
	case <-w.ran:
		t.Fatal("the waker runs without consumers")
	default:
	}
	if rounds.count() != 0 || !rounds.sealed {
		t.Fatalf("rounds %d, sealed %v", rounds.count(), rounds.sealed)
	}
	if err := r.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
}

// writerFunc adapts a function to io.Writer.
type writerFunc func(p []byte) (int, error)

func (f writerFunc) Write(p []byte) (int, error) { return f(p) }
