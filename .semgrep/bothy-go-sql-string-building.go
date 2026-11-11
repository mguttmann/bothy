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

// The statement is built first and reaches the call through a variable.
func built(ctx context.Context, db conn, table, name, order string) {
	formatted := fmt.Sprintf("SELECT * FROM %s", table)
	// ruleid: bothy-go-sql-string-building
	db.Query(ctx, formatted)

	joined := "DELETE FROM threads WHERE title = '" + name
	// ruleid: bothy-go-sql-string-building
	db.Exec(ctx, joined)

	// A clause appended between two literals.
	sorted := "SELECT id FROM threads WHERE title = $1 " + order + " LIMIT 1"
	// ruleid: bothy-go-sql-string-building
	db.Query(ctx, sorted, name)

	// The literal on the right.
	prefixed := table + " SET title = $1"
	// ruleid: bothy-go-sql-string-building
	db.Exec(ctx, prefixed, name)

	// Built statements in a list.
	statements := []string{
		"ALTER DATABASE " + table + " OWNER TO app",
		"REVOKE ALL ON DATABASE " + table + " FROM PUBLIC",
	}
	for _, s := range statements {
		// ruleid: bothy-go-sql-string-building
		db.Exec(ctx, s)
	}

	// Built outside a function literal, run inside it.
	counted := "SELECT count(*) FROM " + table
	run := func() {
		// ruleid: bothy-go-sql-string-building
		db.Query(ctx, counted)
	}
	run()
}

// A constant statement may pass through a variable, and a built text may go anywhere else.
func constant(ctx context.Context, db conn, name string) error {
	const byTitle = "SELECT id FROM threads WHERE title = $1"
	statement := byTitle
	// ok: bothy-go-sql-string-building
	db.Query(ctx, statement, name)

	text := "read the thread " + name
	// ok: bothy-go-sql-string-building
	_, err := db.Query(ctx, byTitle, name)
	if err != nil {
		return fmt.Errorf("%s: %w", text, err)
	}
	return nil
}
