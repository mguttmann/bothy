// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	stdsync "sync"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/sync"
	"github.com/OWNER/bothy/services/server/internal/sync/domain"
)

// The constant log lines of the runner (docs/observability/signals.md "Logs").
const (
	msgRoundFailed = "event consumer round failed"
	msgRecovered   = "event consumer recovered"
)

// Waker wakes the worker consumers on NOTIFY event_log (adapters/postgres): one LISTEN
// connection per process for every consumer, apart from the log tail (D-M2-01-7).
type Waker interface {
	// Run holds the LISTEN connection until ctx ends and then returns nil; a lost connection is
	// reconnected with backoff. Any other return is an unexpected end.
	Run(ctx context.Context) error
	// Wake delivers one signal per notification and after every (re)connect.
	Wake() <-chan struct{}
}

// Rounds is what the runner drives; ConsumerService implements it.
type Rounds interface {
	// Seal ends the registration and returns the registered consumer names.
	Seal() []sync.ConsumerName
	// Round runs one round of name (ConsumerService.Round).
	Round(ctx context.Context, name sync.ConsumerName) (RoundResult, error)
}

// Compile-time check: ConsumerService drives the runner.
var _ Rounds = (*ConsumerService)(nil)

// RunnerOptions are the injected dependencies of the runner.
type RunnerOptions struct {
	// After returns a channel that receives once after d; nil means time.After. Unit tests
	// inject their own timer.
	After func(d time.Duration) <-chan time.Time
	// Logger receives the lines of a failed and a recovered round; nil discards them.
	Logger *slog.Logger
}

// ConsumerRunner runs the rounds of the worker consumers in the roles worker and all (02
// "Worker consumers" item 3): one goroutine per registered consumer, woken by the shared
// Waker and every consumer.poll_interval, draining at once after a full batch (D-M2-01-8). A
// failing round of one consumer never holds back another one. It is safe for concurrent use.
type ConsumerRunner struct {
	rounds Rounds
	waker  Waker
	poll   time.Duration
	after  func(d time.Duration) <-chan time.Time
	log    *slog.Logger

	mu           stdsync.Mutex
	started      bool
	ended        bool
	stopLoops    context.CancelFunc
	cancelRounds context.CancelFunc
	done         chan struct{}
	failed       chan error
}

// NewConsumerRunner returns the runner over rounds and waker with consumer.poll_interval of cfg.
//
// Errors: a configuration ConsumerConfig refuses, or a nil rounds or waker.
func NewConsumerRunner(rounds Rounds, waker Waker, cfg ConsumerConfig, opts RunnerOptions) (*ConsumerRunner, error) {
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	if rounds == nil || waker == nil {
		return nil, errors.New("sync: the consumer runner needs its rounds and a waker")
	}
	if opts.After == nil {
		opts.After = time.After
	}
	if opts.Logger == nil {
		opts.Logger = slog.New(slog.DiscardHandler)
	}
	return &ConsumerRunner{rounds: rounds, waker: waker, poll: cfg.PollInterval, after: opts.After, log: opts.Logger,
		done: make(chan struct{}), failed: make(chan error, 1)}, nil
}

// Start seals the registration and starts the waker and one loop per consumer; each loop runs
// its first round at once. Without a registered consumer it starts nothing and opens no
// connection. The goroutines do not depend on ctx (lifecycle.Component); Stop ends them.
//
// Errors: a second Start.
func (r *ConsumerRunner) Start(context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.started {
		return errors.New("sync: the consumer runner started twice")
	}
	r.started = true
	names := r.rounds.Seal()
	if len(names) == 0 {
		r.end()
		return nil
	}
	loops, stopLoops := context.WithCancel(context.Background())
	rounds, cancelRounds := context.WithCancel(context.Background())
	r.stopLoops, r.cancelRounds = stopLoops, cancelRounds
	wakes := make([]chan struct{}, len(names))
	for i := range wakes {
		wakes[i] = make(chan struct{}, 1)
	}
	var wg stdsync.WaitGroup
	wg.Go(func() { r.runWaker(loops) })
	wg.Go(func() { r.dispatch(loops, wakes) })
	for i, name := range names {
		wg.Go(func() { r.loop(loops, rounds, name, wakes[i]) })
	}
	go func() {
		wg.Wait()
		cancelRounds()
		r.mu.Lock()
		defer r.mu.Unlock()
		r.end()
	}()
	return nil
}

// end closes done and failed once; the caller holds r.mu.
func (r *ConsumerRunner) end() {
	if !r.ended {
		r.ended = true
		close(r.done)
		close(r.failed)
	}
}

// Stop is the stop step event_consumers (02 "Shutdown", worker step 2): it starts no new round
// and waits until the running rounds have committed or rolled back. When ctx ends first, it
// cancels the context of the running rounds and returns ctx.Err() at once, without waiting for
// them: a cancelled round can no longer commit, its transaction rolls back (in the unit of work
// or, when the pool closes first, in PostgreSQL), so either way no job is lost or doubled. Stop
// may be called again, also before Start; a later call waits for the rounds to end.
func (r *ConsumerRunner) Stop(ctx context.Context) error {
	r.mu.Lock()
	if !r.started {
		r.started = true
		r.end()
	}
	stopLoops, cancelRounds := r.stopLoops, r.cancelRounds
	r.mu.Unlock()
	if stopLoops != nil {
		stopLoops()
	}
	select {
	case <-r.done:
		return nil
	case <-ctx.Done():
		if cancelRounds != nil {
			cancelRounds()
		}
		return ctx.Err()
	}
}

// Failed delivers an unexpected end of the runner (an end of the waker or a panic outside a
// handler, both bugs, never a failing round); internal/wire passes it to lifecycle.Runner.FailOn
// so the process exits 1 (D-M2-01-9). It is closed without a value after an orderly stop.
func (r *ConsumerRunner) Failed() <-chan error { return r.failed }

// fail reports err once on Failed, unless the runner has ended.
func (r *ConsumerRunner) fail(err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.ended {
		return
	}
	select {
	case r.failed <- err:
	default:
	}
}

// runWaker runs the waker until loops ends; any earlier return is an unexpected end.
func (r *ConsumerRunner) runWaker(loops context.Context) {
	defer r.recoverPanic("waker")
	err := r.waker.Run(loops)
	if loops.Err() != nil {
		return
	}
	if err == nil {
		err = errors.New("ended before the stop")
	}
	r.fail(fmt.Errorf("sync: consumer waker: %w", err))
}

// dispatch forwards every signal of the waker to every consumer with a non-blocking send into a
// channel of capacity one, so signals during a round coalesce into one.
func (r *ConsumerRunner) dispatch(loops context.Context, wakes []chan struct{}) {
	for {
		select {
		case <-loops.Done():
			return
		case <-r.waker.Wake():
			for _, w := range wakes {
				select {
				case w <- struct{}{}:
				default:
				}
			}
		}
	}
}

// loop runs the rounds of one consumer until loops ends. rounds is the context of the rounds
// themselves; Stop cancels it only when its budget is spent.
func (r *ConsumerRunner) loop(loops, rounds context.Context, name sync.ConsumerName, wake <-chan struct{}) {
	defer r.recoverPanic(string(name))
	failing := false
	for loops.Err() == nil {
		res, err := r.rounds.Round(rounds, name)
		if err != nil && loops.Err() != nil {
			// A round cut short by the stop rolled back; that is no failure of the consumer.
			return
		}
		failing = r.note(rounds, name, failing, err)
		if err == nil && res.Outcome == RoundProcessed && res.Full {
			continue
		}
		select {
		case <-loops.Done():
			return
		case <-wake:
		case <-r.after(r.poll):
		}
	}
}

// note logs the first failure after a success at error level and the first success after a
// failure at info level, so an outage of the database does not flood the log (D-M2-01-3); it
// returns whether the consumer is failing now.
func (r *ConsumerRunner) note(ctx context.Context, name sync.ConsumerName, failing bool, err error) bool {
	switch {
	case err != nil && !failing:
		r.log.ErrorContext(ctx, msgRoundFailed, slog.String(keyConsumerName, string(name)), slog.String(keyErrorType, errorType(err)))
		return true
	case err != nil:
		return true
	case failing:
		r.log.InfoContext(ctx, msgRecovered, slog.String(keyConsumerName, string(name)))
		return false
	default:
		return false
	}
}

// recoverPanic turns a panic of a runner goroutine outside a handler (a bug) into an unexpected
// end; the panic value is not kept.
func (r *ConsumerRunner) recoverPanic(what string) {
	if p := recover(); p != nil {
		r.fail(fmt.Errorf("sync: consumer runner %s panicked", what))
	}
}

// errorType returns the class of a round error for the log, never its text: the SQLSTATE of a
// database error, consumer_gap, timeout, canceled, or the kind of a kernel/errs error.
func errorType(err error) string {
	var pgErr interface{ SQLState() string }
	switch {
	case errors.As(err, &pgErr):
		return pgErr.SQLState()
	case errors.Is(err, domain.ErrConsumerGap):
		return "consumer_gap"
	case errors.Is(err, context.DeadlineExceeded):
		return "timeout"
	case errors.Is(err, context.Canceled):
		return "canceled"
	default:
		return errs.KindOf(err).String()
	}
}
