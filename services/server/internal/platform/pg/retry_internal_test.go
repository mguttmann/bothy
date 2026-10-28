// SPDX-License-Identifier: Apache-2.0

package pg

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

// TestRetryUntilKeepsTheCauseOverADialTimeout: an attempt cut short by the end of the wait says
// nothing about the database, also when the dialer reports it as its own i/o timeout
// (os.ErrDeadlineExceeded, which is no context.DeadlineExceeded) because the socket deadline
// fired before the context noticed its end (PR run 40626720507: "dial error: timeout: dial tcp
// ...: i/o timeout" instead of the cause 3D000). The final error keeps the earlier cause.
func TestRetryUntilKeepsTheCauseOverADialTimeout(t *testing.T) {
	t.Parallel()
	cause := &pgconn.PgError{Code: "3D000", Message: `database "no_such_database" does not exist`}
	attempts := 0
	err := retryUntil(t.Context(), slog.New(slog.DiscardHandler), Config{Host: "127.0.0.1", Port: 5432}, 300*time.Millisecond,
		func(ctx context.Context) error {
			attempts++
			if attempts == 1 {
				return cause
			}
			<-ctx.Done()
			return &net.OpError{Op: "dial", Net: "tcp", Err: os.ErrDeadlineExceeded}
		})
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "3D000" {
		t.Fatalf("retryUntil after %d attempts: %v, want the cause 3D000", attempts, err)
	}
}

// TestErrorTypeOfADialTimeout: the log line of a failed attempt names a dial timeout of the
// socket deadline "timeout", like the end of the context.
func TestErrorTypeOfADialTimeout(t *testing.T) {
	t.Parallel()
	for _, err := range []error{
		context.DeadlineExceeded,
		&net.OpError{Op: "dial", Net: "tcp", Err: os.ErrDeadlineExceeded},
	} {
		if got := errorType(err); got != "timeout" {
			t.Errorf("errorType(%v) = %q, want timeout", err, got)
		}
	}
	if got := errorType(errors.New("refused")); got != "connect" {
		t.Errorf("errorType(refused) = %q, want connect", got)
	}
}
