// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/searchkit"
)

// TestSearchReads_MapStatementTimeout (brief M3-14 design note 6): the error mapping of the search
// read operation SearchMessages. SQLSTATE 57014 while the context is alive is
// searchkit.ErrTimeout; the same SQLSTATE after the caller cancelled is the context error (pgx
// cancels the statement for the caller); any other error is an internal kernel/errs error.
func TestSearchReads_MapStatementTimeout(t *testing.T) {
	t.Parallel()
	timeout := fmt.Errorf("query: %w", &pgconn.PgError{Code: "57014", Message: "canceling statement due to statement timeout"})
	other := &pgconn.PgError{Code: "42601", Message: "syntax error"}
	live := context.Background()
	cancelled, cancel := context.WithCancel(live)
	cancel()

	for name, read := range map[string]func(context.Context, error) error{
		"SearchMessages": func(ctx context.Context, err error) error { return searchError(ctx, err, "searching messages") },
	} {
		if got := read(live, timeout); !errors.Is(got, searchkit.ErrTimeout) {
			t.Errorf("%s: 57014 with a live context = %v, want searchkit.ErrTimeout", name, got)
		}
		if got := read(cancelled, timeout); !errors.Is(got, context.Canceled) || errors.Is(got, searchkit.ErrTimeout) {
			t.Errorf("%s: 57014 with a cancelled context = %v, want context.Canceled", name, got)
		}
		if got := read(live, other); errors.Is(got, searchkit.ErrTimeout) || errs.KindOf(got) != errs.KindInternal {
			t.Errorf("%s: another database error = %v, want an internal error", name, got)
		}
	}
}
