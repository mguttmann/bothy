// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
)

// closeTimeout bounds the close of a broken LISTEN connection.
const closeTimeout = time.Second

// listener holds one dedicated LISTEN event_log connection and turns its notifications into
// wake signals; the log tail and the waker of the worker consumers share it, each with its own
// connection (docs/DECISIONS.md D-M2-01-7).
type listener struct {
	// dial opens the connection outside the pool (pg.Pool.DialDedicated).
	dial func(ctx context.Context) (*pgx.Conn, error)
	// poll is the first pause after a lost connection; the reader polls at this interval.
	poll time.Duration
	// backoffCap is database.listen_reconnect_backoff_cap, the longest pause.
	backoffCap time.Duration
	// lost is called with the error.type of every lost connection; each user logs its own
	// constant line (signals.md "Logs").
	lost func(ctx context.Context, errorType string)
}

// run holds the connection until ctx ends and signals wake on every notification and after
// every (re)connect, because notifications sent while it was down are lost. A lost connection is
// reconnected with exponential backoff from poll up to backoffCap; polling of the reader covers
// the gap, and the gapless seq makes the next read catch up.
func (l listener) run(ctx context.Context, wake chan<- struct{}) {
	backoff := l.poll
	for ctx.Err() == nil {
		conn, err := l.dial(ctx)
		if err == nil {
			if _, err = conn.Exec(ctx, "LISTEN event_log"); err == nil {
				backoff = l.poll
				signal(wake)
				err = wait(ctx, conn, wake)
			}
		}
		if conn != nil {
			cctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), closeTimeout)
			_ = conn.Close(cctx)
			cancel()
		}
		if ctx.Err() != nil {
			return
		}
		l.lost(ctx, errorType(err))
		timer := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
		backoff = min(2*backoff, l.backoffCap)
	}
}

// wait forwards notifications of conn to wake until conn fails or ctx ends.
func wait(ctx context.Context, conn *pgx.Conn, wake chan<- struct{}) error {
	for {
		if _, err := conn.WaitForNotification(ctx); err != nil {
			return err
		}
		signal(wake)
	}
}

// signal wakes the reader without blocking; one pending wake is enough.
func signal(wake chan<- struct{}) {
	select {
	case wake <- struct{}{}:
	default:
	}
}
