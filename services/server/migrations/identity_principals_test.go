// SPDX-License-Identifier: Apache-2.0

package migrations_test

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/pressly/goose/v3"

	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
	"github.com/OWNER/bothy/services/server/migrations"
)

// principalColumn is one row of information_schema.columns as 03 section 4.2 states it. The
// name differs from column of instance_test.go (M1-15) in the same package (M1-26/14).
type principalColumn struct {
	Name, Type string
	Nullable   bool
	Default    string // "" means no default
}

// principalColumns returns the columns of table in ordinal order.
func principalColumns(t *testing.T, conn *pgx.Conn, table string) []principalColumn {
	t.Helper()
	rows, err := conn.Query(context.Background(), `SELECT column_name, data_type, is_nullable = 'YES',
		coalesce(column_default, '') FROM information_schema.columns
		WHERE table_schema = 'public' AND table_name = $1 ORDER BY ordinal_position`, table)
	if err != nil {
		t.Fatal(err)
	}
	out, err := pgx.CollectRows(rows, pgx.RowToStructByPos[principalColumn])
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// principalConstraints returns the definitions (pg_get_constraintdef) of the constraints of table, sorted,
// without the NOT NULL constraints PostgreSQL 18 records (principalColumns holds nullability).
func principalConstraints(t *testing.T, conn *pgx.Conn, table string) []string {
	t.Helper()
	rows, err := conn.Query(context.Background(), `SELECT pg_get_constraintdef(oid) FROM pg_constraint
		WHERE conrelid = to_regclass($1) AND contype <> 'n' ORDER BY 1`, table)
	if err != nil {
		t.Fatal(err)
	}
	out, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// indexes returns the definitions (pg_get_indexdef) of the indexes of table, sorted, with the
// index name left in, so a rename shows.
func indexes(t *testing.T, conn *pgx.Conn, table string) []string {
	t.Helper()
	rows, err := conn.Query(context.Background(), `SELECT indexdef FROM pg_indexes
		WHERE schemaname = 'public' AND tablename = $1 ORDER BY 1`, table)
	if err != nil {
		t.Fatal(err)
	}
	out, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// TestIdentityPrincipalsSchema holds acceptance criterion 1 of M1-26: principals, users and
// owners match docs/spec/03-data-model.md section 4.2 column by column, with every check,
// unique index, index and foreign key, including the foreign key of avatar_file_id that M3-03
// adds with files.
func TestIdentityPrincipalsSchema(t *testing.T) {
	t.Parallel()
	conn := pgtest.New(t).SuperConn(t)

	tests := []struct {
		table       string
		columns     []principalColumn
		constraints []string
		indexes     []string
	}{
		{
			table: "principals",
			columns: []principalColumn{
				{"id", "uuid", false, ""},
				{"kind", "text", false, ""},
				{"status", "text", false, "'active'::text"},
				{"handle", "text", false, ""},
				{"display_name", "text", false, ""},
				{"avatar_file_id", "uuid", true, ""},
				{"created_at", "timestamp with time zone", false, "now()"},
				{"disabled_at", "timestamp with time zone", true, ""},
				{"deleted_at", "timestamp with time zone", true, ""},
			},
			constraints: []string{
				"CHECK ((handle ~ '^[a-z0-9][a-z0-9._-]*$'::text))",
				"CHECK ((kind = ANY (ARRAY['user'::text, 'agent'::text, 'app'::text])))",
				"CHECK ((status = ANY (ARRAY['pending'::text, 'active'::text, 'disabled'::text, 'deleted'::text])))",
				"FOREIGN KEY (avatar_file_id) REFERENCES files(id) ON DELETE SET NULL",
				"PRIMARY KEY (id)",
			},
			indexes: []string{
				"CREATE INDEX principals_kind_status_idx ON public.principals USING btree (kind, status)",
				"CREATE UNIQUE INDEX principals_handle_lower_key ON public.principals USING btree (lower(handle))",
				"CREATE UNIQUE INDEX principals_pkey ON public.principals USING btree (id)",
			},
		},
		{
			table: "users",
			columns: []principalColumn{
				{"principal_id", "uuid", false, ""},
				{"email", "text", true, ""},
				{"locale", "text", false, "'de'::text"},
				{"time_zone", "text", false, "'UTC'::text"},
				{"last_active_at", "timestamp with time zone", true, ""},
			},
			constraints: []string{
				"FOREIGN KEY (principal_id) REFERENCES principals(id) ON DELETE RESTRICT",
				"PRIMARY KEY (principal_id)",
			},
			indexes: []string{
				"CREATE UNIQUE INDEX users_email_lower_key ON public.users USING btree (lower(email)) WHERE (email IS NOT NULL)",
				"CREATE UNIQUE INDEX users_pkey ON public.users USING btree (principal_id)",
			},
		},
		{
			table: "owners",
			columns: []principalColumn{
				{"id", "uuid", false, ""},
				{"user_id", "uuid", false, ""},
				{"granted_by", "uuid", true, ""},
				{"granted_at", "timestamp with time zone", false, "now()"},
				{"revoked_at", "timestamp with time zone", true, ""},
				{"revoked_by", "uuid", true, ""},
				{"reason", "text", false, ""},
			},
			constraints: []string{
				"FOREIGN KEY (granted_by) REFERENCES principals(id) ON DELETE RESTRICT",
				"FOREIGN KEY (revoked_by) REFERENCES principals(id) ON DELETE RESTRICT",
				"FOREIGN KEY (user_id) REFERENCES users(principal_id) ON DELETE RESTRICT",
				"PRIMARY KEY (id)",
			},
			indexes: []string{
				"CREATE UNIQUE INDEX owners_active_user_key ON public.owners USING btree (user_id) WHERE (revoked_at IS NULL)",
				"CREATE UNIQUE INDEX owners_pkey ON public.owners USING btree (id)",
			},
		},
	}
	for _, tt := range tests {
		if got := principalColumns(t, conn, tt.table); !reflect.DeepEqual(got, tt.columns) {
			t.Errorf("%s columns:\n got %+v\nwant %+v", tt.table, got, tt.columns)
		}
		want := slices.Clone(tt.constraints)
		slices.Sort(want)
		if got := principalConstraints(t, conn, tt.table); !reflect.DeepEqual(got, want) {
			t.Errorf("%s constraints:\n got %q\nwant %q", tt.table, got, want)
		}
		want = slices.Clone(tt.indexes)
		slices.Sort(want)
		if got := indexes(t, conn, tt.table); !reflect.DeepEqual(got, want) {
			t.Errorf("%s indexes:\n got %q\nwant %q", tt.table, got, want)
		}
	}
}

// TestIdentityPrincipalsTriggers holds the two triggers of 03 section 4.2: a users row only for
// a principal of kind user, and principals.kind never changes. Both fail with SQLSTATE 23514 so
// the adapter maps them like a check violation; bothy_app runs into them as well.
func TestIdentityPrincipalsTriggers(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	app := pgtest.New(t).Pool(t, pg.RoleApp)
	const agent = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f5a6b"
	const person = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f5a6c"
	mustExec(t, app, `INSERT INTO principals (id, kind, handle, display_name) VALUES ($1, 'agent', 'helper', 'Helper')`, agent)
	mustExec(t, app, `INSERT INTO principals (id, kind, handle, display_name) VALUES ($1, 'user', 'alex', 'Alex')`, person)

	wantCheckViolation(t, exec(app, `INSERT INTO users (principal_id) VALUES ($1)`, agent), "users row for an agent")
	mustExec(t, app, `INSERT INTO users (principal_id) VALUES ($1)`, person)
	wantCheckViolation(t, exec(app, `UPDATE users SET principal_id = $1 WHERE principal_id = $2`, agent, person), "users row moved to an agent")
	wantCheckViolation(t, exec(app, `UPDATE principals SET kind = 'agent' WHERE id = $1`, person), "kind changed")
	wantCheckViolation(t, exec(app, `UPDATE principals SET kind = 'user' WHERE id = $1`, agent), "kind of an agent changed")
	// An update that keeps the kind passes.
	mustExec(t, app, `UPDATE principals SET kind = 'user', display_name = 'Alex B' WHERE id = $1`, person)
	if _, err := app.Exec(ctx, `DELETE FROM principals WHERE id = $1`, person); err == nil {
		t.Error("deleting a principal with a users row passed; want ON DELETE RESTRICT")
	}
}

// TestIdentityPrincipalsDown holds the Down section: after a migration down to version 1 the
// three tables and the trigger functions are gone, and up applies again.
func TestIdentityPrincipalsDown(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := pgtest.New(t)
	pool := db.OpenPool(t, db.Config(pg.RoleOwner).ForMigrate(5*time.Second))
	sqlDB := pool.OpenSQL()
	t.Cleanup(func() { _ = sqlDB.Close() })
	provider, err := goose.NewProvider(goose.DialectPostgres, sqlDB, migrations.FS, goose.WithDisableGlobalRegistry(true))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.DownTo(ctx, 1); err != nil {
		t.Fatalf("down to 1: %v", err)
	}
	for _, rel := range []string{"principals", "users", "owners"} {
		var exists bool
		if err := sqlDB.QueryRowContext(ctx, "SELECT to_regclass($1) IS NOT NULL", rel).Scan(&exists); err != nil {
			t.Fatal(err)
		}
		if exists {
			t.Errorf("table %s survives the down migration", rel)
		}
	}
	var functions int
	if err := sqlDB.QueryRowContext(ctx, `SELECT count(*) FROM pg_proc p JOIN pg_namespace n ON n.oid = p.pronamespace
		WHERE n.nspname = 'public' AND p.proname LIKE 'identity_%'`).Scan(&functions); err != nil {
		t.Fatal(err)
	}
	if functions != 0 {
		t.Errorf("%d identity trigger functions survive the down migration", functions)
	}
	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("up after down: %v", err)
	}
	if err := sqlDB.QueryRowContext(ctx, "SELECT 1 FROM principals LIMIT 1").Scan(new(int)); err != nil && !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("principals after up again: %v", err)
	}
}

func exec(p *pg.Pool, sql string, args ...any) error {
	_, err := p.Exec(context.Background(), sql, args...)
	return err
}

func mustExec(t *testing.T, p *pg.Pool, sql string, args ...any) {
	t.Helper()
	if err := exec(p, sql, args...); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
}

func wantCheckViolation(t *testing.T, err error, what string) {
	t.Helper()
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23514" {
		t.Errorf("%s: %v, want SQLSTATE 23514", what, err)
	}
}
