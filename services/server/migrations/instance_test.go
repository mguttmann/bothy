// SPDX-License-Identifier: Apache-2.0

package migrations_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
	"github.com/OWNER/bothy/services/server/migrations"
)

// column is one column as information_schema describes it.
type column struct {
	name, dataType, nullable, def string
}

// columns reads the columns of table in ordinal order.
func columns(t *testing.T, conn *pgx.Conn, table string) []column {
	t.Helper()
	rows, err := conn.Query(context.Background(), `SELECT column_name, data_type, is_nullable, coalesce(column_default, '')
		FROM information_schema.columns WHERE table_schema = 'public' AND table_name = $1 ORDER BY ordinal_position`, table)
	if err != nil {
		t.Fatal(err)
	}
	cols, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (column, error) {
		var c column
		err := r.Scan(&c.name, &c.dataType, &c.nullable, &c.def)
		return c, err
	})
	if err != nil {
		t.Fatal(err)
	}
	return cols
}

// constraints reads the constraints of table except NOT NULL (columns covers nullability) as "<type> <definition>", sorted.
func constraints(t *testing.T, conn *pgx.Conn, table string) []string {
	t.Helper()
	rows, err := conn.Query(context.Background(), `SELECT contype::text || ' ' || pg_get_constraintdef(oid)
		FROM pg_constraint WHERE conrelid = to_regclass('public.' || quote_ident($1)) AND contype <> 'n' ORDER BY 1`, table)
	if err != nil {
		t.Fatal(err)
	}
	out, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// TestMigration_InstanceTablesMatchSpec compares instance and sequence_counters with
// docs/spec/03-data-model.md 4.1: columns, types, nullability, defaults, checks and keys.
func TestMigration_InstanceTablesMatchSpec(t *testing.T) {
	t.Parallel()
	db := pgtest.New(t, pgtest.WithMigrations(migrations.FS))
	super := db.SuperConn(t)

	wantInstance := []column{
		{"id", "smallint", "NO", "1"},
		{"instance_id", "uuid", "NO", ""},
		{"setup_completed_at", "timestamp with time zone", "YES", ""},
		{"schema_version", "bigint", "NO", ""},
		{"min_server_version", "text", "YES", ""},
		{"sync_epoch", "uuid", "NO", ""},
		{"created_at", "timestamp with time zone", "NO", "now()"},
	}
	if got := columns(t, super, "instance"); !slices.Equal(got, wantInstance) {
		t.Errorf("instance columns\n got %v\nwant %v", got, wantInstance)
	}
	wantInstanceCons := []string{"c CHECK ((id = 1))", "p PRIMARY KEY (id)", "u UNIQUE (instance_id)"}
	if got := constraints(t, super, "instance"); !slices.Equal(got, wantInstanceCons) {
		t.Errorf("instance constraints\n got %v\nwant %v", got, wantInstanceCons)
	}

	wantCounters := []column{
		{"name", "text", "NO", ""},
		{"value", "bigint", "NO", "0"},
	}
	if got := columns(t, super, "sequence_counters"); !slices.Equal(got, wantCounters) {
		t.Errorf("sequence_counters columns\n got %v\nwant %v", got, wantCounters)
	}
	wantCounterCons := []string{
		"c CHECK ((name = ANY (ARRAY['event_seq'::text, 'audit_seq'::text, 'permission_version'::text])))",
		"p PRIMARY KEY (name)",
	}
	if got := constraints(t, super, "sequence_counters"); !slices.Equal(got, wantCounterCons) {
		t.Errorf("sequence_counters constraints\n got %v\nwant %v", got, wantCounterCons)
	}
	// No further indexes than the keys (03: "Indexes: no others").
	var extra int
	if err := super.QueryRow(context.Background(), `SELECT count(*) FROM pg_index i
		WHERE i.indrelid IN ('public.instance'::regclass, 'public.sequence_counters'::regclass)
		AND NOT EXISTS (SELECT 1 FROM pg_constraint c WHERE c.conindid = i.indexrelid)`).Scan(&extra); err != nil {
		t.Fatal(err)
	}
	if extra != 0 {
		t.Errorf("%d indexes besides the keys", extra)
	}
}

// TestMigration_CounterRowsCreated: the migration that creates sequence_counters inserts the
// three rows with value 0, and the migrations create no instance row (bothy migrate up does).
func TestMigration_CounterRowsCreated(t *testing.T) {
	t.Parallel()
	db := pgtest.New(t, pgtest.WithMigrations(migrations.FS))
	app := db.Pool(t, pg.RoleApp)
	rows, err := app.Query(context.Background(), "SELECT name || '=' || value FROM sequence_counters ORDER BY name")
	if err != nil {
		t.Fatal(err)
	}
	got, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"audit_seq=0", "event_seq=0", "permission_version=0"}; !slices.Equal(got, want) {
		t.Errorf("counter rows %v, want %v", got, want)
	}
	var n int
	if err := app.QueryRow(context.Background(), "SELECT count(*) FROM instance").Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("the migrations created %d instance rows; bothy migrate up creates it", n)
	}
}

// wantSQLState fails t unless err carries code.
func wantSQLState(t *testing.T, what string, err error, code string) {
	t.Helper()
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != code {
		t.Errorf("%s: %v, want SQLSTATE %s", what, err, code)
	}
}

// TestCounters_ValueNeverDecreases: the trigger refuses a lower value and a rename even for the
// owner; bothy_app may only update value, never insert or delete rows, and never write instance.
func TestCounters_ValueNeverDecreases(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := pgtest.New(t, pgtest.WithMigrations(migrations.FS))
	owner := db.Pool(t, pg.RoleOwner)
	if _, err := owner.Exec(ctx, "UPDATE sequence_counters SET value = 5 WHERE name = 'event_seq'"); err != nil {
		t.Fatal(err)
	}
	_, err := owner.Exec(ctx, "UPDATE sequence_counters SET value = 4 WHERE name = 'event_seq'")
	if err == nil || !strings.Contains(err.Error(), "never decrease") {
		t.Errorf("lowering a counter as bothy_owner: %v, want the trigger error", err)
	}
	_, err = owner.Exec(ctx, "UPDATE sequence_counters SET name = 'audit_seq' WHERE name = 'event_seq'")
	if err == nil || !strings.Contains(err.Error(), "name") {
		t.Errorf("renaming a counter as bothy_owner: %v, want the trigger error", err)
	}
	if _, err := owner.Exec(ctx, "UPDATE sequence_counters SET value = value WHERE name = 'event_seq'"); err != nil {
		t.Errorf("an unchanged value is no decrease: %v", err)
	}

	app := db.Pool(t, pg.RoleApp)
	if _, err := app.Exec(ctx, "UPDATE sequence_counters SET value = value + 1 WHERE name = 'audit_seq'"); err != nil {
		t.Errorf("bothy_app raises a counter: %v", err)
	}
	_, err = app.Exec(ctx, "UPDATE sequence_counters SET value = 0 WHERE name = 'event_seq'")
	wantSQLState(t, "bothy_app lowers a counter", err, "P0001")
	_, err = app.Exec(ctx, "INSERT INTO sequence_counters (name, value) VALUES ('event_seq', 0)")
	wantSQLState(t, "bothy_app inserts a counter row", err, "42501")
	_, err = app.Exec(ctx, "DELETE FROM sequence_counters WHERE name = 'event_seq'")
	wantSQLState(t, "bothy_app deletes a counter row", err, "42501")
	_, err = app.Exec(ctx, "UPDATE sequence_counters SET name = name WHERE name = 'event_seq'")
	wantSQLState(t, "bothy_app updates the name column", err, "42501")
	_, err = app.Exec(ctx, "INSERT INTO instance (instance_id, schema_version, sync_epoch) VALUES (gen_random_uuid(), 1, gen_random_uuid())")
	wantSQLState(t, "bothy_app inserts the instance row", err, "42501")
	_, err = app.Exec(ctx, "UPDATE instance SET schema_version = 9")
	wantSQLState(t, "bothy_app updates the instance row", err, "42501")
}
