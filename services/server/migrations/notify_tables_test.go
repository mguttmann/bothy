// SPDX-License-Identifier: Apache-2.0

package migrations_test

import (
	"context"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/pressly/goose/v3"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
	"github.com/OWNER/bothy/services/server/migrations"
)

// TestMigrationNotifyTables holds notification_preferences and quiet_hours against
// docs/spec/03-data-model.md section 4.10 column by column, with every key, check and index
// (M3-18): no foreign key on target_id (R-DM-16), both tables cascade from users, a second up
// changes nothing and deleting a person removes the person's rows.
func TestMigrationNotifyTables(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := pgtest.New(t)
	conn := db.SuperConn(t)
	tz := "timestamp with time zone"
	for _, tc := range []struct {
		table string
		cols  []principalColumn
		cons  []string
		idx   []string
	}{
		{
			table: "notification_preferences",
			cols: []principalColumn{
				{"user_id", "uuid", false, ""}, {"target_kind", "text", false, ""}, {"target_id", "uuid", false, ""},
				{"level", "text", false, ""}, {"muted_until", tz, true, ""}, {"version", "bigint", false, "1"},
				{"updated_at", tz, false, "now()"},
			},
			cons: []string{
				"CHECK ((level = ANY (ARRAY['all'::text, 'mentions'::text, 'mute'::text])))",
				"CHECK ((target_kind = ANY (ARRAY['space'::text, 'channel'::text, 'thread'::text])))",
				"FOREIGN KEY (user_id) REFERENCES users(principal_id) ON DELETE CASCADE",
				"PRIMARY KEY (user_id, target_kind, target_id)",
			},
			idx: []string{
				"CREATE INDEX notification_preferences_target_idx ON public.notification_preferences USING btree (target_kind, target_id)",
				"CREATE UNIQUE INDEX notification_preferences_pkey ON public.notification_preferences USING btree (user_id, target_kind, target_id)",
			},
		},
		{
			table: "quiet_hours",
			cols: []principalColumn{
				{"id", "uuid", false, ""}, {"user_id", "uuid", false, ""}, {"weekdays", "smallint", false, ""},
				{"start_local", "time without time zone", false, ""}, {"end_local", "time without time zone", false, ""},
				{"time_zone", "text", true, ""}, {"enabled", "boolean", false, "true"}, {"version", "bigint", false, "1"},
			},
			cons: []string{
				"CHECK (((weekdays >= 1) AND (weekdays <= 127)))",
				"FOREIGN KEY (user_id) REFERENCES users(principal_id) ON DELETE CASCADE",
				"PRIMARY KEY (id)",
			},
			idx: []string{
				"CREATE INDEX quiet_hours_user_enabled_idx ON public.quiet_hours USING btree (user_id) WHERE enabled",
				"CREATE UNIQUE INDEX quiet_hours_pkey ON public.quiet_hours USING btree (id)",
			},
		},
	} {
		if got := principalColumns(t, conn, tc.table); !reflect.DeepEqual(got, tc.cols) {
			t.Errorf("%s columns:\n got %+v\nwant %+v", tc.table, got, tc.cols)
		}
		cons := principalConstraints(t, conn, tc.table)
		slices.Sort(cons)
		slices.Sort(tc.cons)
		if !reflect.DeepEqual(cons, tc.cons) {
			t.Errorf("%s constraints:\n got %q\nwant %q", tc.table, cons, tc.cons)
		}
		if got := indexes(t, conn, tc.table); !reflect.DeepEqual(got, tc.idx) {
			t.Errorf("%s indexes:\n got %q\nwant %q", tc.table, got, tc.idx)
		}
	}

	pool := db.OpenPool(t, db.Config(pg.RoleOwner).ForMigrate(5*time.Second))
	sqlDB := pool.OpenSQL()
	t.Cleanup(func() { _ = sqlDB.Close() })
	provider, err := goose.NewProvider(goose.DialectPostgres, sqlDB, migrations.FS, goose.WithDisableGlobalRegistry(true))
	if err != nil {
		t.Fatal(err)
	}
	if res, err := provider.Up(ctx); err != nil || len(res) != 0 {
		t.Fatalf("second up: %d results, %v", len(res), err)
	}

	person := ids.New().String()
	for _, stmt := range []string{
		`INSERT INTO principals (id, kind, status, handle, display_name) VALUES ($1, 'user', 'active', 'alex', 'Alex')`,
		`INSERT INTO users (principal_id) VALUES ($1)`,
		`INSERT INTO notification_preferences (user_id, target_kind, target_id, level) VALUES ($1, 'space', gen_random_uuid(), 'mute')`,
		`INSERT INTO quiet_hours (id, user_id, weekdays, start_local, end_local) VALUES (gen_random_uuid(), $1, 31, '22:00', '07:00')`,
		`DELETE FROM users WHERE principal_id = $1`,
	} {
		if _, err := conn.Exec(ctx, stmt, person); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	var left int
	if err := conn.QueryRow(ctx, `SELECT (SELECT count(*) FROM notification_preferences WHERE user_id = $1)
		+ (SELECT count(*) FROM quiet_hours WHERE user_id = $1)`, person).Scan(&left); err != nil {
		t.Fatal(err)
	}
	if left != 0 {
		t.Fatalf("%d rows survive the deletion of the person", left)
	}
}
