// SPDX-License-Identifier: Apache-2.0

// Test cases for the rule bothy-go-sql-string-building (run by `make semgrep`).
package fixtures

import (
	"context"
	"fmt"
)

type conn interface {
	Exec(ctx context.Context, sql string, args ...any) (any, error)
	Query(ctx context.Context, sql string, args ...any) (any, error)
}

func queries(ctx context.Context, db conn, table, name string) {
	// ruleid: bothy-go-sql-string-building
	db.Query(ctx, fmt.Sprintf("SELECT * FROM %s", table))
	// ruleid: bothy-go-sql-string-building
	db.Exec(ctx, "DELETE FROM threads WHERE title = '"+name)
	// ok: bothy-go-sql-string-building
	db.Query(ctx, "SELECT id FROM threads WHERE title = $1", name)
}
