// SPDX-License-Identifier: Apache-2.0

package pg_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
)

// TestReadSnapshot: the statements of fn see one snapshot (a row committed by another session in
// between stays invisible), the transaction is read-only, and the error of fn comes back.
func TestReadSnapshot(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := pgtest.New(t, pgtest.WithoutMigrations())
	super := db.SuperConn(t)
	if _, err := super.Exec(ctx, "CREATE TABLE snap (id int); ALTER TABLE snap OWNER TO bothy_owner; INSERT INTO snap VALUES (1)"); err != nil {
		t.Fatal(err)
	}
	pool := db.Pool(t, pg.RoleOwner)
	var first, second int
	err := pg.ReadSnapshot(ctx, pool, func(ctx context.Context, q pg.DBTX) error {
		if err := q.QueryRow(ctx, "SELECT count(*) FROM snap").Scan(&first); err != nil {
			return err
		}
		if _, err := super.Exec(ctx, "INSERT INTO snap VALUES (2)"); err != nil {
			return err
		}
		if err := q.QueryRow(ctx, "SELECT count(*) FROM snap").Scan(&second); err != nil {
			return err
		}
		_, err := q.Exec(ctx, "INSERT INTO snap VALUES (3)")
		return err
	})
	if first != 1 || second != 1 {
		t.Errorf("counts %d and %d inside the snapshot, want 1 and 1", first, second)
	}
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "25006" {
		t.Errorf("insert inside the snapshot: %v, want read_only_sql_transaction (25006)", err)
	}
	var n int
	if err := super.QueryRow(ctx, "SELECT count(*) FROM snap").Scan(&n); err != nil || n != 2 {
		t.Errorf("rows after the snapshot %d (%v), want 2", n, err)
	}
}
