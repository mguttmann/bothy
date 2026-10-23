// SPDX-License-Identifier: Apache-2.0

package pg

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/OWNER/bothy/services/server/internal/platform/otel/attr"
)

// The constant log records of the listener: never a payload, an ID or an address.
const (
	msgListenerLost     = "ephemeral listener lost"
	msgListenerRestored = "ephemeral listener restored"
	errorListen         = "listen"
	errorWait           = "wait"
	errorDial           = "dial"
	// EphemeralApplicationName is the name of the listener's connection in pg_stat_activity.
	EphemeralApplicationName = "bothy-ephemeral"
	// EphemeralComponent is the name of the listener's start component and stop step.
	EphemeralComponent = "ephemeral_listener"
	// closeWait bounds the close of a broken LISTEN connection.
	closeWait = time.Second
)

// EphemeralRoute is one receiver of the channel ephemeral (03 section 6.6, 02 section 11.6): the
// terminal (M2-47) and the gateway (M7-17) each register one. Both methods run on the listener's
// goroutine and must not block on a connection.
type EphemeralRoute interface {
	// HandleEphemeral takes one payload in order of arrival; a route ignores the kinds of the
	// other routes (each module names its kinds with its own prefix).
	HandleEphemeral(ctx context.Context, payload []byte)
	// Reconcile runs after the LISTEN connection came back from a loss, never before LISTEN
	// succeeded again, so nothing committed in between is missed.
	Reconcile(ctx context.Context)
}

// EphemeralConfig holds the configuration of the listener.
type EphemeralConfig struct {
	// ReconnectBackoffCap is database.listen_reconnect_backoff_cap: after a lost connection the
	// listener tries again at once and then at this interval.
	ReconnectBackoffCap time.Duration
	// Gate, when set, runs before every dial (a test seam: a test keeps a node deaf); nil in
	// production.
	Gate func(ctx context.Context) error
}

// EphemeralListener is the one LISTEN ephemeral connection of a process (M7-17 Design note 1,
// replacing the terminal's own listener of M2-47): a lifecycle component named
// EphemeralComponent. It reads the notifications in order on one goroutine and hands every payload
// to every route. It is a soft dependency like the LISTEN of the log tail: no readiness check.
type EphemeralListener struct {
	dial   func(ctx context.Context) (*pgx.Conn, error)
	cfg    EphemeralConfig
	routes []EphemeralRoute
	fail   func(error)
	log    *slog.Logger

	mu     sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}
}

// NewEphemeralListener returns the listener over its own connection of pool (DialDedicated with
// EphemeralApplicationName) for routes. fail receives an unexpected end of its goroutine (a bug,
// not a lost connection; lifecycle.Runner.Fail); log nil means slog.Default(). Errors: a missing
// collaborator, no route or a backoff cap below 1.
func NewEphemeralListener(pool *Pool, cfg EphemeralConfig, routes []EphemeralRoute, fail func(error), log *slog.Logger) (*EphemeralListener, error) {
	if pool == nil || fail == nil || len(routes) == 0 {
		return nil, errors.New("pg: the ephemeral listener needs the pool, a route and the fail hook")
	}
	for _, r := range routes {
		if r == nil {
			return nil, errors.New("pg: the ephemeral listener got a nil route")
		}
	}
	if cfg.ReconnectBackoffCap <= 0 {
		return nil, errors.New("pg: database.listen_reconnect_backoff_cap must be positive")
	}
	if log == nil {
		log = slog.Default()
	}
	dial := func(ctx context.Context) (*pgx.Conn, error) {
		if cfg.Gate != nil {
			if err := cfg.Gate(ctx); err != nil {
				return nil, err
			}
		}
		return pool.DialDedicated(ctx, EphemeralApplicationName)
	}
	return &EphemeralListener{dial: dial, cfg: cfg, routes: append([]EphemeralRoute(nil), routes...), fail: fail, log: log}, nil
}

// Name implements lifecycle.Component.
func (*EphemeralListener) Name() string { return EphemeralComponent }

// Start implements lifecycle.Component: it starts the goroutine and returns; a database that is
// not reachable yet does not fail the start (soft dependency, 02 section 14.4).
func (l *EphemeralListener) Start(context.Context) error {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	l.mu.Lock()
	l.cancel, l.done = cancel, done
	l.mu.Unlock()
	go func() {
		defer close(done)
		defer func() {
			if p := recover(); p != nil {
				l.fail(fmt.Errorf("ephemeral listener: %v", p))
			}
		}()
		l.run(ctx)
		if ctx.Err() == nil {
			l.fail(errors.New("ephemeral listener stopped unexpectedly"))
		}
	}()
	return nil
}

// Stop is the stop step of the listener in lifecycle.PhaseEventTail (02 section 15.3 api step 4):
// it ends the goroutine and waits for it within ctx.
func (l *EphemeralListener) Stop(ctx context.Context) error {
	l.mu.Lock()
	cancel, done := l.cancel, l.done
	l.mu.Unlock()
	if cancel == nil {
		return nil
	}
	cancel()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// run holds the connection until ctx ends: on a loss it logs once, tries again at once and then
// every database.listen_reconnect_backoff_cap; after LISTEN succeeded again every route reconciles
// and the restore is logged.
func (l *EphemeralListener) run(ctx context.Context) {
	lost, first := false, true
	for ctx.Err() == nil {
		conn, errType := l.listen(ctx)
		if conn != nil {
			if lost {
				for _, r := range l.routes {
					r.Reconcile(ctx)
				}
				l.log.LogAttrs(ctx, slog.LevelInfo, msgListenerRestored)
			}
			lost, first = false, false
			errType = l.wait(ctx, conn)
			l.closeConn(ctx, conn)
		}
		if ctx.Err() != nil {
			return
		}
		if !lost {
			l.log.LogAttrs(ctx, slog.LevelWarn, msgListenerLost, slog.String(attr.ErrorType, errType))
			lost = true
			if !first {
				continue // try again at once
			}
		}
		first = false
		timer := time.NewTimer(l.cfg.ReconnectBackoffCap)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

// listen dials and executes LISTEN ephemeral; it returns the connection or the error.type.
func (l *EphemeralListener) listen(ctx context.Context) (*pgx.Conn, string) {
	conn, err := l.dial(ctx)
	if err != nil {
		return nil, errorDial
	}
	if _, err := conn.Exec(ctx, "LISTEN ephemeral"); err != nil {
		l.closeConn(ctx, conn)
		return nil, errorListen
	}
	return conn, ""
}

// wait hands every notification of conn to every route until conn fails or ctx ends.
func (l *EphemeralListener) wait(ctx context.Context, conn *pgx.Conn) string {
	for {
		n, err := conn.WaitForNotification(ctx)
		if err != nil {
			return errorWait
		}
		for _, r := range l.routes {
			r.HandleEphemeral(ctx, []byte(n.Payload))
		}
	}
}

// closeConn closes conn within closeWait.
func (l *EphemeralListener) closeConn(ctx context.Context, conn *pgx.Conn) {
	cctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), closeWait)
	defer cancel()
	_ = conn.Close(cctx)
}
