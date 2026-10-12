// SPDX-License-Identifier: Apache-2.0

package pg

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
)

// The queries for the schema version: goose's table goose_db_version, unqualified as goose
// creates it, so it resolves through search_path in schema public (03 "Technical tables").
// A database without the table has version 0; rows that a down migration marked as not applied
// do not count.
const (
	versionTableQuery = "SELECT to_regclass('goose_db_version') IS NOT NULL"
	versionQuery      = "SELECT coalesce(max(version_id), 0) FROM goose_db_version WHERE is_applied"
)

// SchemaTooOldError reports a database whose schema is older than the binary needs
// (docs/spec/02-architecture.md "Start" step 4): the start aborts and the operator runs the
// migrations first.
type SchemaTooOldError struct {
	// Have is the version of the database, 0 when it was never migrated.
	Have int64
	// Need is the version the binary requires (RequiredSchemaVersion).
	Need int64
}

// Error returns the message with the way out, for example
// `database schema version 3 is older than required 4; run "bothy migrate up"`.
func (e *SchemaTooOldError) Error() string {
	return fmt.Sprintf("database schema version %d is older than required %d; run \"bothy migrate up\"", e.Have, e.Need)
}

// RequiredSchemaVersion returns the schema version a binary with the migrations fsys needs: the
// highest embedded version. Each binary needs all of its own migrations, while older binaries
// run against newer schemas by the expand and contract rule (docs/operations/upgrade.md).
//
// The error reports a set that Validate's naming and numbering rules refuse.
func RequiredSchemaVersion(fsys fs.FS) (int64, error) {
	files, err := listMigrations(fsys)
	if err != nil {
		return 0, err
	}
	return files[len(files)-1].version, nil
}

// CheckSchema compares the schema version of the database behind p with required and returns
// *SchemaTooOldError when the database is older. An equal or newer schema returns nil: a newer
// one belongs to a newer binary, which this one may run against during an upgrade and after a
// rollback by one version (upgrade.md "Rollback"). The role of p needs SELECT on goose_db_version
// (migration 00001 grants it to bothy_app).
//
// Other errors are those of Acquire (kind unavailable) or of the query.
func CheckSchema(ctx context.Context, p *Pool, required int64) error {
	c, err := p.Acquire(ctx)
	if err != nil {
		return err
	}
	defer c.Release()
	have, err := SchemaVersion(ctx, c.Conn())
	if err != nil {
		return fmt.Errorf("pg: read the schema version: %w", err)
	}
	if have < required {
		return &SchemaTooOldError{Have: have, Need: required}
	}
	return nil
}

// SchemaVersion returns the schema version of the database behind q: the highest applied goose
// version, 0 when the version table does not exist. q is the pool or the transaction of a unit
// of work (Querier); bothy migrate up reads it inside the unit that records the instance row
// (M1-15). The role needs SELECT on goose_db_version. The error is the database error.
func SchemaVersion(ctx context.Context, q DBTX) (int64, error) {
	var exists bool
	if err := q.QueryRow(ctx, versionTableQuery).Scan(&exists); err != nil || !exists {
		return 0, err
	}
	var v int64
	err := q.QueryRow(ctx, versionQuery).Scan(&v)
	return v, err
}

// sqlSchemaVersion returns the schema version over database/sql (the migrator).
func sqlSchemaVersion(ctx context.Context, db *sql.DB) (int64, error) {
	var exists bool
	if err := db.QueryRowContext(ctx, versionTableQuery).Scan(&exists); err != nil || !exists {
		return 0, err
	}
	var v int64
	err := db.QueryRowContext(ctx, versionQuery).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return v, err
}
