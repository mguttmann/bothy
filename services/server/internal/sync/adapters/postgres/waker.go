// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"log/slog"

	"github.com/jackc/pgx/v5"

	"github.com/OWNER/bothy/services/server/internal/platform/otel/attr"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/sync/app"
)

// WakerAppName is the application_name of the LISTEN connection of the worker consumers, so an
// operator (and the chaos test) finds it in pg_stat_activity apart from the log tail.
const WakerAppName = "bothy-consumer-wake"

// msgWakeLost is the constant log line of a lost wake connection.
const msgWakeLost = "event consumer LISTEN connection lost, polling and reconnecting"

// Waker implements app.Waker: one LISTEN event_log connection per process for every worker
// consumer, outside the pool and apart from the tail's connection, so in role all the ordered
// loop of the tail never depends on the consumers (docs/DECISIONS.md D-M2-01-7). The payload of
// a notification (the highest seq) is not needed: a signal only starts a round.
type Waker struct {
	l    listener
	wake chan struct{}
}

// Compile-time check: Waker is the outgoing port of the runner.
var _ app.Waker = (*Waker)(nil)

// NewWaker returns the waker over pool with consumer.poll_interval and
// database.listen_reconnect_backoff_cap of cfg (a cap below the poll interval becomes the poll
// interval, because the backoff starts there); log receives the line of a lost connection (nil
// discards it).
func NewWaker(pool *pg.Pool, cfg app.ConsumerConfig, log *slog.Logger) *Waker {
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	w := &Waker{wake: make(chan struct{}, 1)}
	w.l = listener{
		dial:       func(ctx context.Context) (*pgx.Conn, error) { return pool.DialDedicated(ctx, WakerAppName) },
		poll:       cfg.PollInterval,
		backoffCap: max(cfg.ReconnectBackoffCap, cfg.PollInterval),
		lost: func(ctx context.Context, typ string) {
			log.WarnContext(ctx, msgWakeLost, slog.String(attr.ErrorType, typ))
		},
	}
	return w
}

// Run implements app.Waker.Run: it holds the connection until ctx ends, reconnecting with
// exponential backoff from consumer.poll_interval up to database.listen_reconnect_backoff_cap,
// and returns nil. Polling of the runner covers the time without a connection, and nothing is
// lost because the cursor lives in the database (02 "Failure of a dependency").
func (w *Waker) Run(ctx context.Context) error {
	w.l.run(ctx, w.wake)
	return nil
}

// Wake implements app.Waker.Wake: one signal per notification and after every (re)connect;
// signals coalesce while nobody reads.
func (w *Waker) Wake() <-chan struct{} { return w.wake }
