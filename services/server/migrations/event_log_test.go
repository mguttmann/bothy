// SPDX-License-Identifier: Apache-2.0

package migrations_test

import (
	"context"
	"fmt"
	"math"
	"slices"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
	"github.com/OWNER/bothy/services/server/migrations"
)

// eventLogVersion is the version of 00006_sync_event_log.sql.
const eventLogVersion = 6

// partitionName is the name sync_event_log_ensure_partitions gives the partition with the lower
// bound lower (zero-padded to 19 digits).
func partitionName(lower int64) string { return fmt.Sprintf("event_log_from_%019d", lower) }

// eventInsert inserts one row with seq $1 through a connection of any role.
const eventInsert = `INSERT INTO event_log (seq, id, type, resource_kind, receive_action, payload, permission_version)
	VALUES ($1::bigint, gen_random_uuid(), 'thread.closed', 'thread', 'thread.read', '{}'::jsonb, 0)`

// TestMigration_EventLogMatchesSpec compares event_log with docs/spec/03-datenmodell.md 4.10:
// columns, types, nullability, defaults, checks, keys, indexes, partitioning by range of seq
// without a default partition (R-DM-15), and no foreign key.
func TestMigration_EventLogMatchesSpec(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := pgtest.New(t, pgtest.WithMigrations(migrations.FS))
	super := db.SuperConn(t)

	want := []column{
		{"seq", "bigint", "NO", ""},
		{"id", "uuid", "NO", ""},
		{"type", "text", "NO", ""},
		{"schema_version", "smallint", "NO", "1"},
		{"occurred_at", "timestamp with time zone", "NO", "now()"},
		{"actor_kind", "text", "YES", ""},
		{"actor_principal_id", "uuid", "YES", ""},
		{"space_id", "uuid", "YES", ""},
		{"channel_id", "uuid", "YES", ""},
		{"thread_id", "uuid", "YES", ""},
		{"resource_kind", "text", "NO", ""},
		{"resource_id", "uuid", "YES", ""},
		{"resource_version", "bigint", "YES", ""},
		{"audience_principal_id", "uuid", "YES", ""},
		{"receive_action", "text", "NO", ""},
		{"payload", "jsonb", "NO", ""},
		{"truncated", "boolean", "NO", "false"},
		{"redacted_at", "timestamp with time zone", "YES", ""},
		{"correlation_id", "uuid", "YES", ""},
		{"trace_context", "text", "YES", ""},
		{"client_msg_id", "uuid", "YES", ""},
		{"permission_version", "bigint", "NO", ""},
	}
	if got := columns(t, super, "event_log"); !slices.Equal(got, want) {
		t.Errorf("event_log columns\n got %v\nwant %v", got, want)
	}
	wantCons := []string{
		"c CHECK ((actor_kind = ANY (ARRAY['user'::text, 'agent'::text, 'app'::text, 'system'::text])))",
		"c CHECK ((COALESCE((actor_kind = ANY (ARRAY['user'::text, 'agent'::text, 'app'::text])), false) = (actor_principal_id IS NOT NULL)))",
		"c CHECK (((receive_action <> 'self'::text) OR (audience_principal_id IS NOT NULL)))",
		"p PRIMARY KEY (seq)",
		"u UNIQUE (id, seq)",
	}
	if got := constraints(t, super, "event_log"); !slices.Equal(got, wantCons) {
		t.Errorf("event_log constraints\n got %v\nwant %v", got, wantCons)
	}
	wantIdx := []string{
		"CREATE INDEX event_log_audience_seq_idx ON ONLY public.event_log USING btree (audience_principal_id, seq) WHERE (audience_principal_id IS NOT NULL)",
		"CREATE INDEX event_log_channel_seq_idx ON ONLY public.event_log USING btree (channel_id, seq)",
		"CREATE INDEX event_log_occurred_at_brin ON ONLY public.event_log USING brin (occurred_at)",
		"CREATE INDEX event_log_resource_idx ON ONLY public.event_log USING btree (resource_kind, resource_id)",
		"CREATE INDEX event_log_space_seq_idx ON ONLY public.event_log USING btree (space_id, seq)",
	}
	rows, err := super.Query(ctx, `SELECT indexdef FROM pg_indexes
		WHERE tablename = 'event_log' AND indexdef NOT LIKE 'CREATE UNIQUE%' ORDER BY 1`)
	if err != nil {
		t.Fatal(err)
	}
	idx, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(idx, wantIdx) {
		t.Errorf("event_log indexes\n got %v\nwant %v", idx, wantIdx)
	}
	var kind, key string
	var partitions int
	if err := super.QueryRow(ctx, `SELECT c.relkind::text, pg_get_partkeydef(c.oid),
		(SELECT count(*) FROM pg_inherits WHERE inhparent = c.oid)
		FROM pg_class c WHERE c.oid = 'public.event_log'::regclass`).Scan(&kind, &key, &partitions); err != nil {
		t.Fatal(err)
	}
	if kind != "p" || key != "RANGE (seq)" || partitions != 0 {
		t.Errorf("event_log: relkind %s, key %q, %d partitions; want a range partitioned table on seq without partitions (bothy migrate up creates them)", kind, key, partitions)
	}
	if _, err := super.Exec(ctx, eventInsert, 1); err == nil || sqlstate(err) != "23514" {
		t.Errorf("insert without a partition: %v, want 23514 (no default partition)", err)
	}
}

// TestMigration_EventLogFunctionsAreNarrowDDL holds 16 "Rollen" (R-OP-17): the two partition
// functions are SECURITY DEFINER of bothy_owner with a pinned search_path, PUBLIC cannot run
// them, bothy_app can; bothy_app itself has no DDL right and only SELECT and INSERT on the
// table (R-DM-18: the envelope is append-only, redaction adds its column grant in M1-58).
func TestMigration_EventLogFunctionsAreNarrowDDL(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := pgtest.New(t, pgtest.WithMigrations(migrations.FS))
	super := db.SuperConn(t)
	rows, err := super.Query(ctx, `SELECT p.proname, pg_get_userbyid(p.proowner), p.prosecdef,
		array_to_string(p.proconfig, ','), has_function_privilege('bothy_app', p.oid, 'EXECUTE'),
		EXISTS (SELECT 1 FROM aclexplode(p.proacl) a WHERE a.grantee = 0)
		FROM pg_proc p JOIN pg_namespace n ON n.oid = p.pronamespace
		WHERE n.nspname = 'public' AND p.proname LIKE 'sync_event_log_%' ORDER BY 1`)
	if err != nil {
		t.Fatal(err)
	}
	type fn struct {
		name, owner string
		definer     bool
		config      string
		app, public bool
	}
	fns, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (fn, error) {
		var f fn
		err := r.Scan(&f.name, &f.owner, &f.definer, &f.config, &f.app, &f.public)
		return f, err
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(fns) != 2 || fns[0].name != "sync_event_log_drop_expired" || fns[1].name != "sync_event_log_ensure_partitions" {
		t.Fatalf("functions = %+v, want drop_expired and ensure_partitions", fns)
	}
	for _, f := range fns {
		if f.owner != string(pg.RoleOwner) || !f.definer || f.config != "search_path=public, pg_temp" || !f.app || f.public {
			t.Errorf("%s: %+v; want owner bothy_owner, SECURITY DEFINER, search_path public then pg_temp, EXECUTE only for bothy_app", f.name, f)
		}
	}

	app := db.Pool(t, pg.RoleApp)
	var created int
	if err := app.QueryRow(ctx, "SELECT sync_event_log_ensure_partitions(10, 1, 0)").Scan(&created); err != nil || created != 1 {
		t.Fatalf("ensure as bothy_app = %d, %v; want 1 partition", created, err)
	}
	if _, err := app.Exec(ctx, eventInsert, 1); err != nil {
		t.Fatalf("bothy_app cannot insert: %v", err)
	}
	for _, q := range []string{
		"UPDATE event_log SET type = 'x.y'",
		"DELETE FROM event_log",
		"TRUNCATE event_log",
		"ALTER TABLE event_log DETACH PARTITION " + partitionName(1),
		"CREATE TABLE " + partitionName(11) + " (LIKE event_log)",
		"SELECT * FROM " + partitionName(1),
	} {
		if _, err := app.Exec(ctx, q); sqlstate(err) != "42501" {
			t.Errorf("%s as bothy_app: %v, want permission denied (42501)", q, err)
		}
	}
	for _, q := range []string{
		"SELECT sync_event_log_ensure_partitions(0, 1, 0)",
		"SELECT sync_event_log_ensure_partitions(10, -1, 0)",
		"SELECT sync_event_log_ensure_partitions(10, 1, -1)",
		"SELECT sync_event_log_drop_expired(now(), -1, '1s')",
		"SELECT sync_event_log_drop_expired(now(), 0, '0s')",
		"SELECT sync_event_log_ensure_partitions(1000000000001, 1, 0)",
		"SELECT sync_event_log_ensure_partitions(10, 1001, 0)",
	} {
		if _, err := app.Exec(ctx, q); sqlstate(err) != "22023" {
			t.Errorf("%s: %v, want invalid_parameter_value (22023)", q, err)
		}
	}
}

// TestMigration_EventLogDown: down removes the table with its partitions and both functions;
// up applies again.
func TestMigration_EventLogDown(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := pgtest.New(t)
	owner := db.Pool(t, pg.RoleOwner)
	if _, err := owner.Exec(ctx, "SELECT sync_event_log_ensure_partitions(10, 2, 0)"); err != nil {
		t.Fatal(err)
	}
	provider, exists := migrateProvider(t, db)
	if _, err := provider.DownTo(ctx, eventLogVersion-1); err != nil {
		t.Fatalf("down: %v", err)
	}
	for _, rel := range []string{"event_log", partitionName(1)} {
		if exists(rel) {
			t.Errorf("%s survives the down migration", rel)
		}
	}
	var n int
	if err := db.SuperConn(t).QueryRow(ctx, "SELECT count(*) FROM pg_proc WHERE proname LIKE 'sync_event_log_%'").Scan(&n); err != nil || n != 0 {
		t.Errorf("%d partition functions survive the down migration (%v)", n, err)
	}
	if _, err := provider.Up(ctx); err != nil || !exists("event_log") {
		t.Fatalf("up after down: %v", err)
	}
}

// TestMigration_DropExpiredTrustsOnlyTheTable: bothy_app may call sync_event_log_drop_expired with
// any arguments, but the function clamps the head to max(seq) and the cutoff to now(), so the
// partition holding the head always survives (16 "Rollen": the definer function is the only DDL
// of the application role and must not widen it).
func TestMigration_DropExpiredTrustsOnlyTheTable(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := pgtest.New(t, pgtest.WithMigrations(migrations.FS))
	app := db.Pool(t, pg.RoleApp)
	if _, err := app.Exec(ctx, "SELECT sync_event_log_ensure_partitions(10, 2, 0)"); err != nil {
		t.Fatal(err)
	}
	for seq := 1; seq <= 12; seq++ {
		if _, err := app.Exec(ctx, eventInsert, seq); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.SuperConn(t).Exec(ctx, "UPDATE event_log SET occurred_at = now() - interval '400 days'"); err != nil {
		t.Fatal(err)
	}
	var dropped int
	if err := app.QueryRow(ctx, "SELECT sync_event_log_drop_expired('infinity', $1, '1s')", int64(math.MaxInt64)).Scan(&dropped); err != nil {
		t.Fatal(err)
	}
	var n, minSeq int64
	if err := app.QueryRow(ctx, "SELECT count(*), min(seq) FROM event_log").Scan(&n, &minSeq); err != nil {
		t.Fatal(err)
	}
	if dropped != 1 || n != 2 || minSeq != 11 {
		t.Fatalf("dropped %d, %d rows from %d remain; want only the partition below the head dropped", dropped, n, minSeq)
	}
}
