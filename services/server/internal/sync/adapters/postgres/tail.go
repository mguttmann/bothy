// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	stdsync "sync"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/OWNER/bothy/services/server/internal/platform/otel/attr"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/sync"
	"github.com/OWNER/bothy/services/server/internal/sync/domain"
)

// TailAppName is the application_name of the LISTEN connection of the tail, so an operator (and
// the chaos test) finds it in pg_stat_activity.
const TailAppName = "bothy-event-tail"

// The constant log lines of the tail (docs/observability/signals.md "Logs").
const (
	msgListenLost = "event log LISTEN connection lost, polling and reconnecting"
	msgReadFailed = "event log tail read failed, retrying at the next poll"
)

// TailConfig is the configuration of the log tail (02 "Configuration values of this chapter").
type TailConfig struct {
	// PollInterval is sync.log_poll_interval: the tail reads at least this often, and the
	// reconnect backoff starts here.
	PollInterval time.Duration `config:"sync.log_poll_interval"`
	// BatchSize is sync.tail_batch_size: events per read.
	BatchSize int `config:"sync.tail_batch_size"`
	// ReconnectBackoffCap is database.listen_reconnect_backoff_cap: the longest pause between two
	// attempts to reconnect the LISTEN connection.
	ReconnectBackoffCap time.Duration `config:"database.listen_reconnect_backoff_cap"`
}

// Tail implements sync.Tail: the one log reader of an api process (02 "Log tail per
// api process"). It listens on its own connection outside the pool, polls through the pool every
// PollInterval, reads seq > local_head ORDER BY seq in batches and hands each batch to the sinks
// in order, from the goroutine of Run.
type Tail struct {
	repo    *Repository
	cfg     TailConfig
	sinks   []sync.Sink
	log     *slog.Logger
	dial    func(ctx context.Context) (*pgx.Conn, error)
	head    atomic.Int64
	started chan struct{}
	once    stdsync.Once
}

// Compile-time check: Tail is the port.
var _ sync.Tail = (*Tail)(nil)

// NewTail returns the tail over the pool (for its LISTEN connection, application name
// TailAppName) and repo (for the reads), delivering to sinks in this order; log receives the
// lines of a lost connection and a failed read (nil discards them). A zero or negative field of
// cfg falls back to the defaults of 02 (one second, 1000 events, ten seconds), so a hand-built
// configuration never spins; a reconnect cap below the poll interval becomes the poll interval.
//
// Example:
//
//	tail := postgres.NewTail(pool, repo, cfg, []sync.Sink{rights.Observe, hub.Deliver}, logger)
//	go func() { runner.Fail(tail.Run(ctx)) }()
func NewTail(pool *pg.Pool, repo *Repository, cfg TailConfig, sinks []sync.Sink, log *slog.Logger) *Tail {
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = time.Second
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 1000
	}
	if cfg.ReconnectBackoffCap <= 0 {
		cfg.ReconnectBackoffCap = 10 * time.Second
	}
	// The backoff starts at the poll interval, so a smaller cap means the poll interval.
	cfg.ReconnectBackoffCap = max(cfg.ReconnectBackoffCap, cfg.PollInterval)
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	t := &Tail{repo: repo, cfg: cfg, sinks: append([]sync.Sink(nil), sinks...), log: log, started: make(chan struct{})}
	t.dial = func(ctx context.Context) (*pgx.Conn, error) { return pool.DialDedicated(ctx, TailAppName) }
	return t
}

// Head returns local_head: the highest seq the sinks received, or the head Run started at.
func (t *Tail) Head() int64 { return t.head.Load() }

// Started is closed once Run has read the head it starts at.
func (t *Tail) Started() <-chan struct{} { return t.started }

// fatalError marks an error that ends Run (a sink error or a gap).
type fatalError struct{ err error }

func (f fatalError) Error() string { return f.err.Error() }
func (f fatalError) Unwrap() error { return f.err }

// Run implements sync.Tail.Run: it sets local_head to the head (02 "Start and graceful shutdown" step 8), starts the
// LISTEN connection and reads on every notification and every PollInterval until ctx ends, then
// returns nil. A read error is logged and retried at the next poll (the database may come back,
// 02 "Failure of a dependency"); a sink error or a hole in the sequence ends Run with that
// error (sync.ErrGap for a hole), and the lifecycle ends the process (02 "Panics").
func (t *Tail) Run(ctx context.Context) error {
	head, err := t.repo.Head(ctx)
	if err != nil {
		return fmt.Errorf("sync: read the head of event_log: %w", err)
	}
	t.head.Store(head)
	t.once.Do(func() { close(t.started) })
	ctx, cancel := context.WithCancel(ctx)
	wake := make(chan struct{}, 1)
	var wg stdsync.WaitGroup
	wg.Go(func() { t.listen(ctx, wake) })
	defer func() {
		cancel()
		wg.Wait()
	}()
	ticker := time.NewTicker(t.cfg.PollInterval)
	defer ticker.Stop()
	for {
		if err := t.drain(ctx); err != nil {
			if fatal, stop := t.failure(ctx, err); stop {
				return fatal
			}
		}
		select {
		case <-ctx.Done():
			return nil
		case <-wake:
		case <-ticker.C:
		}
	}
}

// failure sorts an error of drain: a fatal one ends Run (stop), with nil when ctx has ended,
// because a sink that honours the context of Run ends with it at shutdown and that is the orderly
// end, not a fault of the fan-out; a read error is logged and retried.
func (t *Tail) failure(ctx context.Context, err error) (error, bool) {
	var fatal fatalError
	if errors.As(err, &fatal) {
		if ctx.Err() != nil {
			return nil, true
		}
		return fatal.err, true
	}
	if ctx.Err() == nil {
		t.log.WarnContext(ctx, msgReadFailed, slog.String(attr.ErrorType, errorType(err)))
	}
	return nil, false
}

// drain reads batches after local_head until a short batch and delivers them.
func (t *Tail) drain(ctx context.Context) error {
	for ctx.Err() == nil {
		batch, err := t.repo.EventsAfter(ctx, t.head.Load(), t.cfg.BatchSize)
		if err != nil {
			return err
		}
		if len(batch) == 0 {
			return nil
		}
		if err := t.deliver(ctx, batch); err != nil {
			return err
		}
		if len(batch) < t.cfg.BatchSize {
			return nil
		}
	}
	return nil
}

// deliver checks that batch continues local_head without a hole, hands it to the sinks in order
// and advances local_head; a gap or a sink error is fatal.
func (t *Tail) deliver(ctx context.Context, batch []sync.Event) error {
	seqs := make([]int64, len(batch))
	for i, ev := range batch {
		seqs[i] = ev.Seq
	}
	next, err := domain.CheckContiguous(t.head.Load(), seqs)
	if err != nil {
		return fatalError{fmt.Errorf("%w: %w", sync.ErrGap, err)}
	}
	for _, sink := range t.sinks {
		if err := sink(ctx, batch); err != nil {
			return fatalError{err}
		}
	}
	t.head.Store(next)
	return nil
}

// listen holds the LISTEN connection of the tail (listener.run): a lost connection is
// reconnected with exponential backoff from PollInterval up to ReconnectBackoffCap; polling
// covers the gap, and the gapless seq makes the next read catch up.
func (t *Tail) listen(ctx context.Context, wake chan<- struct{}) {
	l := listener{dial: t.dial, poll: t.cfg.PollInterval, backoffCap: t.cfg.ReconnectBackoffCap,
		lost: func(ctx context.Context, typ string) {
			t.log.WarnContext(ctx, msgListenLost, slog.String(attr.ErrorType, typ))
		}}
	l.run(ctx, wake)
}

// errorType returns the class of err for the log: the SQLSTATE, "timeout" or "connect", never
// the text.
func errorType(err error) string {
	var pgErr interface{ SQLState() string }
	switch {
	case errors.As(err, &pgErr):
		return pgErr.SQLState()
	case errors.Is(err, context.DeadlineExceeded):
		return "timeout"
	default:
		return "connect"
	}
}
