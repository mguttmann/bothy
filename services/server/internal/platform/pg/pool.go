// SPDX-License-Identifier: Apache-2.0

package pg

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"net"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/platform/secrets"
)

// The backoff of WaitReady and Connect: exponential from 100ms, capped at 2s, with full jitter
// (each pause is a random duration up to the current step), so that processes that start
// together do not knock in step.
const (
	backoffBase = 100 * time.Millisecond
	backoffCap  = 2 * time.Second
)

// notReadyMessage is the constant log line of a failed attempt in WaitReady and Connect.
const notReadyMessage = "database not reachable yet, retrying"

// The attribute keys of that line (docs/observability/signals.md "Logs"): the address and a
// class of the error, never its text, which can carry foreign data.
const (
	logKeyServerAddress = "server.address"
	logKeyServerPort    = "server.port"
	logKeyErrorType     = "error.type"
)

// errorType returns the class of a failed attempt for the log: the SQLSTATE of a PostgreSQL
// error (28P01 for a wrong password, 3D000 for a missing database), the code of a secret error
// (not_found, insecure_file), "timeout" or "connect".
func errorType(err error) string {
	var pgErr *pgconn.PgError
	var refErr *secrets.RefError
	switch {
	case errors.As(err, &pgErr):
		return pgErr.Code
	case errors.As(err, &refErr):
		return secrets.ErrorCode(err)
	case errors.Is(err, context.DeadlineExceeded):
		return "timeout"
	default:
		return "connect"
	}
}

// Pool is the connection pool of a process (one per process, docs/spec/02-architektur.md
// "Fristen"). It is safe for concurrent use. Callers go through Acquire, Begin and Ping and their
// deadline; only platform/jobs takes the pgx pool itself (PGX), because River's driver needs it.
type Pool struct {
	pool *pgxpool.Pool
	cfg  Config
	log  *slog.Logger
}

// Open returns the pool for cfg. It does not connect: the first connection is made by the first
// Acquire, Begin, Ping or WaitReady, so a start calls WaitReady next (02 "Start" step 3).
//
// Every new connection resolves cfg.PasswordRef through sp, so a rotated password takes effect
// after secrets.cache_ttl without a restart. Connections start with the session deadlines of cfg,
// application_name, READ COMMITTED and the time zone UTC; spans of the otelpgx tracer go to the
// global tracer provider and never carry query parameters. log receives the retry lines of
// WaitReady; nil means slog.Default().
//
// The error reports an invalid cfg (naming the configuration keys) or a nil sp.
func Open(ctx context.Context, cfg Config, sp *secrets.Provider, log *slog.Logger) (*Pool, error) {
	if sp == nil {
		return nil, errors.New("pg: Open needs a secret provider")
	}
	pc, err := poolConfig(cfg, passwordFunc(cfg, sp))
	if err != nil {
		return nil, err
	}
	p, err := pgxpool.NewWithConfig(ctx, pc)
	if err != nil {
		return nil, fmt.Errorf("pg: create the pool: %w", err)
	}
	if log == nil {
		log = slog.Default()
	}
	return &Pool{pool: p, cfg: cfg, log: log}, nil
}

// passwordFunc resolves the password of cfg for one new connection. The error names the login and
// the reference (secrets.RefError), never the value; the reference can come from
// database.password_ref or, for the bootstrap, migrate.bootstrap_password_ref.
func passwordFunc(cfg Config, sp *secrets.Provider) func(context.Context) (string, error) {
	return func(ctx context.Context) (string, error) {
		v, _, err := sp.Resolve(ctx, cfg.PasswordRef)
		if err != nil {
			return "", fmt.Errorf("pg: the password of %s: %w", cfg.User, err)
		}
		b := v.Bytes()
		defer clear(b)
		return string(b), nil
	}
}

// WaitReady pings the database until it answers or maxWait (startup.dependency_wait) has passed,
// pausing with exponential backoff and full jitter between attempts. It returns nil on the first
// successful ping. The error names host and port and wraps the last connection error, never the
// password; it also ends early with the context error when ctx is done.
func (p *Pool) WaitReady(ctx context.Context, maxWait time.Duration) error {
	return retryUntil(ctx, p.log, p.cfg, maxWait, p.ping)
}

// Ping checks that the database answers, waiting at most database.acquire_timeout for a
// connection (readiness check of /readyz, M1-11). Its errors are those of Acquire.
func (p *Pool) Ping(ctx context.Context) error {
	if err := p.ping(ctx); err != nil {
		return p.unavailable(ctx, err, "ping")
	}
	return nil
}

// ping is Ping without the mapping of its error.
func (p *Pool) ping(ctx context.Context) error {
	actx, cancel := context.WithTimeout(ctx, p.cfg.AcquireTimeout)
	defer cancel()
	return p.pool.Ping(actx)
}

// Acquire returns a connection of the pool, waiting at most database.acquire_timeout. The caller
// must Release it.
//
// Errors: when the deadline passes while every connection is in use, the kernel/errs error of
// code unavailable with the internal cause db_pool_exhausted (02 "Fehlerarten": 503 for the
// client, the cause only in log and metric); when the database cannot be reached or refuses the
// login, code unavailable with cause dependency_unavailable and the pgx error wrapped for the log;
// when ctx itself ends, code unavailable with cause deadline_exceeded for its deadline, or the
// context error unchanged when the caller canceled.
func (p *Pool) Acquire(ctx context.Context) (*pgxpool.Conn, error) {
	actx, cancel := context.WithTimeout(ctx, p.cfg.AcquireTimeout)
	defer cancel()
	c, err := p.pool.Acquire(actx)
	if err != nil {
		return nil, p.unavailable(ctx, err, "acquire")
	}
	return c, nil
}

// Begin starts a transaction on a connection of the pool; the connection goes back when the
// transaction commits or rolls back. Acquiring the connection and BEGIN together wait at most
// database.acquire_timeout; the errors are those of Acquire. An empty isolation level in opts
// becomes READ COMMITTED (03 Grundsatz 11), so a transaction never depends on a server default.
func (p *Pool) Begin(ctx context.Context, opts pgx.TxOptions) (pgx.Tx, error) {
	if opts.IsoLevel == "" {
		opts.IsoLevel = pgx.ReadCommitted
	}
	actx, cancel := context.WithTimeout(ctx, p.cfg.AcquireTimeout)
	defer cancel()
	tx, err := p.pool.BeginTx(actx, opts)
	if err != nil {
		return nil, p.unavailable(ctx, err, "begin")
	}
	return tx, nil
}

// PGX returns the pgx pool inside p for River's driver (platform/jobs, M1-14): River runs its
// own queries, LISTEN and transactions on it, so they bypass the deadline of Acquire and Begin
// but keep the session settings of every connection. It is meant for platform/jobs only; no
// gate enforces that (the architecture gate limits the import of River, not this call), so a
// new caller needs a review of why Acquire and Begin do not suffice.
func (p *Pool) PGX() *pgxpool.Pool { return p.pool }

// Stat returns the statistics of the pool (acquired, idle and total connections, waits).
func (p *Pool) Stat() *pgxpool.Stat { return p.pool.Stat() }

// Close closes all connections of the pool; it waits until every acquired connection is back.
func (p *Pool) Close() { p.pool.Close() }

// OpenSQL returns a database/sql handle over the pool for code that needs one (goose, River
// migrations). Closing it does not close the pool, and the pool's deadline of Acquire does not
// apply to it.
func (p *Pool) OpenSQL() *sql.DB { return stdlib.OpenDBFromPool(p.pool) }

// unavailable maps an error of the pool for the operation op to the error of Acquire.
func (p *Pool) unavailable(ctx context.Context, err error, op string) error {
	if ctxErr := ctx.Err(); ctxErr != nil {
		if errors.Is(ctxErr, context.Canceled) {
			return fmt.Errorf("pg: %s: %w", op, ctxErr)
		}
		return errs.Wrap(errs.CodeUnavailable, err, "the request deadline ended while waiting for the database").
			WithCause(errs.CauseDeadlineExceeded)
	}
	if errors.Is(err, context.DeadlineExceeded) {
		if st := p.pool.Stat(); st.AcquiredConns() >= st.MaxConns() {
			return errs.Wrap(errs.CodeUnavailable, err, "no database connection was free within database.acquire_timeout").
				WithCause(errs.CauseDBPoolExhausted)
		}
	}
	return errs.Wrap(errs.CodeUnavailable, err, "the database is not reachable").
		WithCause(errs.CauseDependencyUnavailable)
}

// Connect opens a single connection for cfg outside a pool, retrying like WaitReady until
// maxWait has passed. bothy migrate up --bootstrap uses it for the superuser
// (migrate.bootstrap_user), and tests for their own sessions. The password comes from
// cfg.PasswordRef through sp; the caller closes the connection. log receives the retry lines; nil
// means slog.Default().
//
// The error reports an invalid cfg, a nil sp, or, after maxWait, the last connection error with
// host and port, never the password.
func Connect(ctx context.Context, cfg Config, sp *secrets.Provider, maxWait time.Duration, log *slog.Logger) (*pgx.Conn, error) {
	if sp == nil {
		return nil, errors.New("pg: Connect needs a secret provider")
	}
	cc, err := connConfig(cfg)
	if err != nil {
		return nil, err
	}
	password := passwordFunc(cfg, sp)
	var conn *pgx.Conn
	if log == nil {
		log = slog.Default()
	}
	err = retryUntil(ctx, log, cfg, maxWait, func(ctx context.Context) error {
		pw, err := password(ctx)
		if err != nil {
			return err
		}
		attempt := cc.Copy()
		attempt.Password = pw
		actx, cancel := context.WithTimeout(ctx, cfg.AcquireTimeout)
		defer cancel()
		conn, err = pgx.ConnectConfig(actx, attempt)
		return err
	})
	if err != nil {
		return nil, err
	}
	return conn, nil
}

// retryUntil runs attempt until it succeeds or maxWait has passed, with the backoff of
// backoffBase and backoffCap between attempts; each failed attempt logs one line. The final
// error names the address of cfg and wraps the last error of attempt.
func retryUntil(ctx context.Context, log *slog.Logger, cfg Config, maxWait time.Duration, attempt func(context.Context) error) error {
	deadline := time.Now().Add(maxWait)
	step := backoffBase
	var last error
	for {
		err := attemptBefore(ctx, deadline, maxWait, attempt)
		if err == nil {
			return nil
		}
		if ctxErr := ctx.Err(); ctxErr != nil {
			return fmt.Errorf("pg: waiting for the database at %s: %w", address(cfg), ctxErr)
		}
		remaining := time.Until(deadline)
		// An attempt cut short by the end of the wait says nothing about the database; the
		// operator needs the cause before it (a wrong password, a missing database).
		if last == nil || remaining > 0 || !errors.Is(err, context.DeadlineExceeded) {
			last = err
		}
		if remaining <= 0 {
			return fmt.Errorf("pg: the database at %s is not reachable within %s: %w", address(cfg), maxWait, last)
		}
		log.WarnContext(ctx, notReadyMessage,
			slog.String(logKeyServerAddress, cfg.Host), slog.Int(logKeyServerPort, cfg.Port),
			slog.String(logKeyErrorType, errorType(err)))
		pause := min(jitter(step), remaining)
		t := time.NewTimer(pause)
		select {
		case <-ctx.Done():
			t.Stop()
			return fmt.Errorf("pg: waiting for the database at %s: %w", address(cfg), ctx.Err())
		case <-t.C:
		}
		step = min(2*step, backoffCap)
	}
}

// attemptBefore runs attempt with a context that ends at deadline, so that one slow attempt
// cannot outlast maxWait; with maxWait 0 the single attempt keeps the context of the caller.
func attemptBefore(ctx context.Context, deadline time.Time, maxWait time.Duration, attempt func(context.Context) error) error {
	if maxWait <= 0 {
		return attempt(ctx)
	}
	actx, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()
	return attempt(actx)
}

// jitter returns a random duration in [0, d] (full jitter).
func jitter(d time.Duration) time.Duration {
	return rand.N(d + 1) // #nosec G404 -- spreads retries; not a secret.
}

// address renders host and port of cfg for errors and logs; a socket directory stays a path.
func address(cfg Config) string {
	if isSocket(cfg.Host) {
		return cfg.Host + " (port " + strconv.Itoa(cfg.Port) + ")"
	}
	return net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))
}
