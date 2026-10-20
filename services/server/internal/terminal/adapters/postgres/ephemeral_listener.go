// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/OWNER/bothy/services/server/internal/platform/otel/attr"
	"github.com/OWNER/bothy/services/server/internal/terminal/app"
	"github.com/OWNER/bothy/services/server/internal/terminal/domain"
)

// The constant log records of the listener (M2-47 Design note 14): never a payload, an ID or an
// address.
const (
	msgListenerLost     = "terminal ephemeral listener lost"
	msgListenerRestored = "terminal ephemeral listener restored"
	msgSignalDropped    = "terminal signal dropped"
	reasonMalformed     = "malformed"
	errorListen         = "listen"
	errorWait           = "wait"
	errorDial           = "dial"
	// ApplicationName is the name of the listener's connection in pg_stat_activity.
	ApplicationName = "bothy-terminal-ephemeral"
	// closeWait bounds the close of a broken LISTEN connection.
	closeWait = time.Second
)

// Reconciler runs the reconciliation of a node after its LISTEN connection came back (M2-47
// Design note 9); the wsrelay hub implements it.
type Reconciler interface {
	Reconcile(ctx context.Context)
}

// ListenerConfig holds the configuration of the listener.
type ListenerConfig struct {
	// ReconnectBackoffCap is database.listen_reconnect_backoff_cap: after a lost connection the
	// listener tries again at once and then at this interval.
	ReconnectBackoffCap time.Duration
}

// EphemeralListener is the LISTEN ephemeral connection of an api process (M2-47 Design note 5): a
// lifecycle.Component named terminal_ephemeral. It reads the notifications in order on one
// goroutine, decodes them strictly and hands each terminal signal to the handler; a malformed
// terminal payload is dropped with one log record, a foreign kind is ignored. It is a soft
// dependency like the LISTEN of the log tail: no readiness check. One listener per process; M7-17
// may merge it with its own listener of the same channel.
type EphemeralListener struct {
	dial      func(ctx context.Context) (*pgx.Conn, error)
	cfg       ListenerConfig
	handler   app.SignalHandler
	reconcile Reconciler
	fail      func(error)
	log       *slog.Logger

	mu     sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}
}

// Dialer opens the dedicated connection of the listener (pg.Pool.DialDedicated with
// ApplicationName).
type Dialer func(ctx context.Context) (*pgx.Conn, error)

// NewEphemeralListener returns the listener. fail receives an unexpected end of its goroutine (a
// bug, not a lost connection; lifecycle.Runner.Fail); log nil means slog.Default(). Errors: a
// missing collaborator or a backoff cap below 1.
func NewEphemeralListener(dial Dialer, cfg ListenerConfig, handler app.SignalHandler, reconcile Reconciler, fail func(error),
	log *slog.Logger) (*EphemeralListener, error) {
	if dial == nil || handler == nil || reconcile == nil || fail == nil {
		return nil, errors.New("postgres: the ephemeral listener needs the dialer, the handler, the reconciler and the fail hook")
	}
	if cfg.ReconnectBackoffCap <= 0 {
		return nil, errors.New("postgres: database.listen_reconnect_backoff_cap must be positive")
	}
	if log == nil {
		log = slog.Default()
	}
	return &EphemeralListener{dial: dial, cfg: cfg, handler: handler, reconcile: reconcile, fail: fail, log: log}, nil
}

// Name implements lifecycle.Component.
func (*EphemeralListener) Name() string { return "terminal_ephemeral" }

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
				l.fail(fmt.Errorf("terminal ephemeral listener: %v", p))
			}
		}()
		l.run(ctx)
		if ctx.Err() == nil {
			l.fail(errors.New("terminal ephemeral listener stopped unexpectedly"))
		}
	}()
	return nil
}

// Stop is the stop step terminal_ephemeral in lifecycle.PhaseEventTail (02 section 15.3 api step
// 4): it ends the goroutine and waits for it within ctx.
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
// every database.listen_reconnect_backoff_cap; after LISTEN succeeded again it reconciles (never
// before, so nothing committed in between is missed) and logs the restore.
func (l *EphemeralListener) run(ctx context.Context) {
	lost, first := false, true
	for ctx.Err() == nil {
		conn, errType := l.listen(ctx)
		if conn != nil {
			if lost {
				l.reconcile.Reconcile(ctx)
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

// wait hands every notification of conn to the handler until conn fails or ctx ends.
func (l *EphemeralListener) wait(ctx context.Context, conn *pgx.Conn) string {
	for {
		n, err := conn.WaitForNotification(ctx)
		if err != nil {
			return errorWait
		}
		l.deliver(ctx, []byte(n.Payload))
	}
}

// deliver decodes one payload and hands a terminal signal to the handler.
func (l *EphemeralListener) deliver(ctx context.Context, payload []byte) {
	s, ours, err := domain.DecodeSignal(payload)
	switch {
	case err != nil:
		l.log.LogAttrs(ctx, slog.LevelWarn, msgSignalDropped, slog.String(attr.Reason, reasonMalformed))
	case ours:
		l.handler.HandleSignal(ctx, s)
	}
}

// closeConn closes conn within closeWait.
func (l *EphemeralListener) closeConn(ctx context.Context, conn *pgx.Conn) {
	cctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), closeWait)
	defer cancel()
	_ = conn.Close(cctx)
}
