// SPDX-License-Identifier: Apache-2.0

package pg

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/jackc/pgx/v5"
)

// Timeouts are the deadlines of one transaction (docs/spec/02-architecture.md "Deadlines"): the
// pool sets them as session defaults from Config, and SetLocalTimeouts overrides them for a
// single transaction. A zero field keeps the session value.
type Timeouts struct {
	// Statement is statement_timeout: the longest a single statement may run.
	Statement time.Duration
	// Lock is lock_timeout: the longest a statement waits for a lock.
	Lock time.Duration
	// IdleInTx is idle_in_transaction_session_timeout: the longest an open transaction may
	// sit without work.
	IdleInTx time.Duration
}

// setLocalQuery sets the three deadlines for the current transaction only (set_config with
// is_local true is SET LOCAL as a function, so the values travel as parameters). A NULL
// parameter keeps the value the session has.
const setLocalQuery = `SELECT
	set_config('statement_timeout', coalesce($1::text, current_setting('statement_timeout')), true),
	set_config('lock_timeout', coalesce($2::text, current_setting('lock_timeout')), true),
	set_config('idle_in_transaction_session_timeout', coalesce($3::text, current_setting('idle_in_transaction_session_timeout')), true)`

// SetLocalTimeouts overrides the deadlines of t for the transaction tx (docs/spec/02-architecture.md
// "Deadlines": database.statement_timeout per transaction by SET LOCAL); when tx ends, the session
// values apply again. A zero field keeps the session value, so a zero Timeouts never switches a
// deadline off; values are whole milliseconds, rounded up.
//
// The error reports a negative field, a field above PostgreSQL's limit of 2147483647ms, or the
// error of the statement.
func SetLocalTimeouts(ctx context.Context, tx pgx.Tx, t Timeouts) error {
	args := make([]any, 0, 3)
	for _, d := range []struct {
		name string
		v    time.Duration
	}{{"statement", t.Statement}, {"lock", t.Lock}, {"idle in transaction", t.IdleInTx}} {
		switch {
		case d.v < 0 || d.v > maxTimeout:
			return fmt.Errorf("pg: the %s timeout must be between 0 and %dms", d.name, int64(math.MaxInt32))
		case d.v == 0:
			args = append(args, nil)
		default:
			args = append(args, millis(d.v))
		}
	}
	if _, err := tx.Exec(ctx, setLocalQuery, args...); err != nil {
		return fmt.Errorf("pg: set the transaction deadlines: %w", err)
	}
	return nil
}
