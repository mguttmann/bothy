// SPDX-License-Identifier: Apache-2.0

package lifecycle

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"slices"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/platform/otel/attr"
)

// Exit codes of Run (docs/spec/02-architecture.md "Commands of the binary"). cmd/bothy uses the
// same values for every command; ExitForced is specific to serve.
const (
	// ExitOK is the graceful end within shutdown.total_timeout.
	ExitOK = 0
	// ExitFailure is a failed start, a fatal runtime error (Fail) or a shutdown over
	// shutdown.total_timeout.
	ExitFailure = 1
	// ExitUsage is a usage or configuration error, here stop step budgets that add up to more
	// than shutdown.total_timeout.
	ExitUsage = 2
	// ExitForced is the immediate end on a second signal.
	ExitForced = 3
)

// Log messages of the runner; constant text (signals.md "Logs").
const (
	msgStartFailed     = "start failed"
	msgFatal           = "fatal runtime error"
	msgShutdownStarted = "shutdown started"
	msgShutdownDone    = "shutdown complete"
	msgBudgetExceeded  = "shutdown budget exceeded"
	msgStepTimedOut    = "shutdown step timed out"
	msgStepFailed      = "shutdown step failed"
	msgForced          = "forced shutdown"
	msgBudgetTooLarge  = "shutdown budgets exceed shutdown.total_timeout"
)

// Values of error.type in the runner's log lines.
const (
	errTypeStart   = "start"
	errTypeFatal   = "fatal"
	errTypeStep    = "step"
	errTypeTimeout = "timeout"
	errTypeBudget  = "budget"
)

// Component is one part of the start: telemetry, the internal listener, the pool, the public
// listener. Start runs once, in registration order; it returns when the part is started and
// leaves long-running work to its own goroutines, which must not depend on ctx: ctx ends when
// the start is cut short by a signal, a Fail or the end of the context of Run, and the
// component then returns promptly with an error. A component registers the stop step of what it
// started with Runner.OnStop, before or during its Start.
type Component interface {
	// Name is a stable snake_case name for logs and errors, for example "database".
	Name() string
	// Start starts the part; an error aborts the start (exit code ExitFailure).
	Start(ctx context.Context) error
}

// StopStep is one step of the shutdown plan.
type StopStep struct {
	// Name says what the step stops, for example "pool"; it appears in errors.
	Name string
	// Phase places the step in the fixed order; steps of the same phase run in reverse
	// registration order, as a rollback would.
	Phase Phase
	// Budget is the step's own limit (shutdown.drain_delay, shutdown.http_grace, ...). The step
	// runs with the smaller of Budget and what is left of the total; 0 means only the rest of
	// the total bounds it (instant steps such as turning readiness off). The sum of all
	// budgets must not exceed Options.TotalTimeout.
	Budget time.Duration
	// Run stops the part. Its context ends when the budget is spent; a step that ignores it is
	// left behind and the plan moves on. An error is logged and does not change the exit code.
	Run func(ctx context.Context) error
}

// Options configure a Runner.
type Options struct {
	// Clock measures the total budget; nil means clock.System().
	Clock clock.Clock
	// Logger receives the start and shutdown lines; nil means slog.Default(). It is read on
	// every line, so a Logger whose handler switches outputs (the server logger after
	// telemetry starts) works.
	Logger *slog.Logger
	// TotalTimeout is shutdown.total_timeout; it must be positive.
	TotalTimeout time.Duration
	// Signals delivers the shutdown signals; nil means os/signal for SIGTERM and SIGINT with a
	// buffer of two. Tests inject their own channel.
	Signals <-chan os.Signal
}

// Runner runs the start and shutdown of one process. Add and OnStop may be called before and
// during Run (OnStop from a component's Start); Fail from any goroutine at any time. Run runs
// once.
type Runner struct {
	opts       Options
	mu         sync.Mutex
	components []Component
	steps      []StopStep
	err        error
	fail       chan struct{}
	failOnce   sync.Once
	ran        atomic.Bool
}

// New returns a Runner for opts. It panics when opts.TotalTimeout is not positive, because a
// shutdown without a budget is a programming error (the configuration schema bounds the key).
func New(opts Options) *Runner {
	if opts.TotalTimeout <= 0 {
		panic("lifecycle: New needs a positive TotalTimeout")
	}
	if opts.Clock == nil {
		opts.Clock = clock.System()
	}
	return &Runner{opts: opts, fail: make(chan struct{})}
}

// Add appends c to the start sequence.
func (r *Runner) Add(c Component) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.components = append(r.components, c)
}

// OnStop adds s to the shutdown plan.
func (r *Runner) OnStop(s StopStep) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.steps = append(r.steps, s)
}

// Fail reports a fatal runtime error, for example a panic in the log tail (02 "Panics"): the
// Runner shuts down in order and Run returns ExitFailure. Only the first report counts; later
// ones are ignored. Fail never blocks.
func (r *Runner) Fail(err error) {
	if err == nil {
		err = errors.New("lifecycle: fatal runtime error")
	}
	r.failOnce.Do(func() {
		r.setErr(err)
		close(r.fail)
	})
}

// FailOn reports the first error that failed delivers as a fatal runtime error (Fail), wrapped
// with name; a channel closed without a value is an orderly end and reports nothing. It is for
// parts that run on their own after their start and whose loss leaves the process useless, for
// example the River client of the worker (docs/spec/02-architecture.md "Health", "Panics"). It
// returns at once; one goroutine waits on failed.
func (r *Runner) FailOn(name string, failed <-chan error) {
	go func() {
		if err, ok := <-failed; ok {
			if err == nil {
				err = errors.New("stopped unexpectedly")
			}
			r.Fail(fmt.Errorf("%s: %w", name, err))
		}
	}()
}

// Err returns the error that made Run return ExitFailure or ExitUsage: the error of the
// failed component (wrapped with its name), the error given to Fail, the spent shutdown total
// with its phase, or the error of step budgets above the total. The first cause wins. It
// is nil after a graceful end. cmd/bothy prints it on stderr, because the log handlers keep no
// error text (signals.md "Logs").
func (r *Runner) Err() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.err
}

func (r *Runner) setErr(err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err == nil {
		r.err = err
	}
}

func (r *Runner) log() *slog.Logger {
	if r.opts.Logger != nil {
		return r.opts.Logger
	}
	return slog.Default()
}

// Run starts the components, waits for the end and shuts down; it returns the exit code:
// ExitOK after a graceful end (a signal, or the end of ctx, also during the start), ExitFailure
// after a failed start, a Fail or a shutdown over the total, ExitUsage when the budgets of the
// steps registered before Run exceed the total (nothing starts then), and ExitForced on a second
// signal. Run panics when called twice.
func (r *Runner) Run(ctx context.Context) int {
	if !r.ran.CompareAndSwap(false, true) {
		panic("lifecycle: Run called twice")
	}
	sigs, stop := r.signals()
	defer stop()
	if err := r.checkBudgets(); err != nil {
		r.setErr(err)
		r.log().ErrorContext(ctx, msgBudgetTooLarge, slog.String(attr.ErrorType, errTypeBudget))
		return ExitUsage
	}
	startCtx, cancelStart := context.WithCancel(ctx)
	defer cancelStart()
	started := make(chan error, 1)
	go func() { started <- r.start(startCtx) }()

	code := ExitOK
	// The total budget counts from the moment the shutdown is requested, so a component that is
	// slow to give up its start cannot stretch the end beyond shutdown.total_timeout.
	var requested time.Time
	select {
	case err := <-started:
		requested = r.opts.Clock.Now()
		switch {
		case err == nil:
			code = r.awaitEnd(ctx, sigs)
			requested = r.opts.Clock.Now()
		case ctx.Err() == nil:
			code = r.startFailed(ctx, err)
		}
		// Otherwise the end of ctx cut the start short: an orderly end, as in the case below.
	case <-sigs:
		requested = r.opts.Clock.Now()
		cancelStart()
		if forced := r.awaitStart(started, sigs); forced {
			return r.forced(ctx)
		}
	case <-r.fail:
		requested = r.opts.Clock.Now()
		cancelStart()
		if forced := r.awaitStart(started, sigs); forced {
			return r.forced(ctx)
		}
		code = r.fatal(ctx)
	case <-ctx.Done():
		requested = r.opts.Clock.Now()
		cancelStart()
		if forced := r.awaitStart(started, sigs); forced {
			return r.forced(ctx)
		}
	}
	return r.shutdown(ctx, sigs, code, requested)
}

// awaitEnd waits after a complete start for a signal, a Fail or the end of ctx.
func (r *Runner) awaitEnd(ctx context.Context, sigs <-chan os.Signal) int {
	select {
	case <-sigs:
	case <-ctx.Done():
	case <-r.fail:
		return r.fatal(ctx)
	}
	return ExitOK
}

// awaitStart waits for the cut-short start to return; it reports true on a second signal.
func (r *Runner) awaitStart(started <-chan error, sigs <-chan os.Signal) bool {
	select {
	case <-started:
		return false
	case <-sigs:
		return true
	}
}

func (r *Runner) startFailed(ctx context.Context, err error) int {
	r.setErr(err)
	r.log().ErrorContext(ctx, msgStartFailed, slog.String(attr.ErrorType, errTypeStart))
	return ExitFailure
}

func (r *Runner) fatal(ctx context.Context) int {
	r.log().ErrorContext(ctx, msgFatal, slog.String(attr.ErrorType, errTypeFatal))
	return ExitFailure
}

func (r *Runner) forced(ctx context.Context) int {
	r.log().ErrorContext(ctx, msgForced)
	return ExitForced
}

// signals returns the signal channel and the function that stops the delivery.
func (r *Runner) signals() (<-chan os.Signal, func()) {
	if r.opts.Signals != nil {
		return r.opts.Signals, func() {}
	}
	ch := make(chan os.Signal, 2)
	signal.Notify(ch, syscall.SIGTERM, syscall.SIGINT)
	return ch, func() { signal.Stop(ch) }
}

// checkBudgets compares the sum of the registered step budgets with the total (02 "Shutdown").
func (r *Runner) checkBudgets() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	var sum time.Duration
	for _, s := range r.steps {
		sum += s.Budget
	}
	if sum > r.opts.TotalTimeout {
		return fmt.Errorf("lifecycle: the stop step budgets add up to %s, more than shutdown.total_timeout %s", sum, r.opts.TotalTimeout)
	}
	return nil
}

// start runs the components in order and stops at the first error or at the end of ctx.
func (r *Runner) start(ctx context.Context) error {
	for i := 0; ; i++ {
		r.mu.Lock()
		if i >= len(r.components) {
			r.mu.Unlock()
			return nil
		}
		c := r.components[i]
		r.mu.Unlock()
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := guard(func() error { return c.Start(ctx) }); err != nil {
			return fmt.Errorf("start %s: %w", c.Name(), err)
		}
	}
}

// plan returns the registered steps in phase order, steps of one phase in reverse registration
// order.
func (r *Runner) plan() []StopStep {
	r.mu.Lock()
	steps := slices.Clone(r.steps)
	r.mu.Unlock()
	slices.Reverse(steps)
	slices.SortStableFunc(steps, func(a, b StopStep) int { return int(a.Phase) - int(b.Phase) })
	return steps
}

// shutdown runs the plan within the total budget, counted from requested, and returns the exit
// code: code, or ExitFailure when the total was exceeded, or ExitForced on a signal.
func (r *Runner) shutdown(ctx context.Context, sigs <-chan os.Signal, code int, requested time.Time) int {
	log := r.log()
	// The parent context may be the reason for the shutdown; the lines still carry its trace.
	lctx := context.WithoutCancel(ctx)
	log.InfoContext(lctx, msgShutdownStarted)
	deadline := requested.Add(r.opts.TotalTimeout)
	var last Phase
	for _, s := range r.plan() {
		remaining := deadline.Sub(r.opts.Clock.Now())
		if remaining <= 0 {
			return r.overBudget(lctx, s.Phase)
		}
		budget := remaining
		if s.Budget > 0 && s.Budget < remaining {
			budget = s.Budget
		}
		if forced := r.runStep(lctx, s, budget, sigs); forced {
			return r.forced(lctx)
		}
		last = s.Phase
	}
	if r.opts.Clock.Now().After(deadline) {
		return r.overBudget(lctx, last)
	}
	log.InfoContext(lctx, msgShutdownDone)
	return code
}

// overBudget records and logs a spent total at phase p and returns ExitFailure; Err then names
// the total and the phase, so cmd/bothy prints a cause on stderr.
func (r *Runner) overBudget(ctx context.Context, p Phase) int {
	r.setErr(fmt.Errorf("lifecycle: shutdown.total_timeout %s spent at phase %s", r.opts.TotalTimeout, p))
	r.log().ErrorContext(ctx, msgBudgetExceeded, slog.String(attr.Reason, p.String()))
	return ExitFailure
}

// runStep runs s with budget and reports true when a signal arrived meanwhile.
func (r *Runner) runStep(ctx context.Context, s StopStep, budget time.Duration, sigs <-chan os.Signal) bool {
	sctx, cancel := context.WithTimeout(context.Background(), budget)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- guard(func() error { return s.Run(sctx) }) }()
	select {
	case err := <-done:
		if err != nil {
			r.log().WarnContext(ctx, msgStepFailed, slog.String(attr.Reason, s.Phase.String()), slog.String(attr.ErrorType, errTypeStep))
		}
	case <-sctx.Done():
		r.log().WarnContext(ctx, msgStepTimedOut, slog.String(attr.Reason, s.Phase.String()), slog.String(attr.ErrorType, errTypeTimeout))
	case <-sigs:
		return true
	}
	return false
}

// guard runs f and turns a panic into an error, so a broken component or step cannot take the
// shutdown plan down with it.
func guard(f func() error) (err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("panic: %v", p)
		}
	}()
	return f()
}
