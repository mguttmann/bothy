// SPDX-License-Identifier: Apache-2.0

package migrations_test

import (
	"context"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/pressly/goose/v3"

	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
	"github.com/OWNER/bothy/services/server/migrations"
)

// TestSyncPersonalDataMigration_Schema holds read_states, drafts and user_settings against
// docs/spec/03-data-model.md section 4.10 column by column, with every key and index (M1-61).
func TestSyncPersonalDataMigration_Schema(t *testing.T) {
	t.Parallel()
	conn := pgtest.New(t).SuperConn(t)
	tz := "timestamp with time zone"
	for _, tc := range []struct {
		table string
		cols  []principalColumn
		cons  []string
		idx   []string
	}{
		{
			table: "read_states",
			cols: []principalColumn{
				{"user_id", "uuid", false, ""}, {"thread_id", "uuid", false, ""}, {"channel_id", "uuid", false, ""},
				{"last_read_seq", "bigint", false, "0"}, {"version", "bigint", false, "1"},
				{"updated_by_device_id", "uuid", true, ""}, {"updated_at", tz, false, "now()"},
			},
			cons: []string{
				"FOREIGN KEY (channel_id) REFERENCES channels(id) ON DELETE CASCADE",
				"FOREIGN KEY (thread_id) REFERENCES threads(id) ON DELETE CASCADE",
				"FOREIGN KEY (updated_by_device_id) REFERENCES devices(id) ON DELETE SET NULL",
				"FOREIGN KEY (user_id) REFERENCES users(principal_id) ON DELETE CASCADE",
				"PRIMARY KEY (user_id, thread_id)",
			},
			idx: []string{
				"CREATE INDEX read_states_user_channel_idx ON public.read_states USING btree (user_id, channel_id)",
				"CREATE UNIQUE INDEX read_states_pkey ON public.read_states USING btree (user_id, thread_id)",
			},
		},
		{
			table: "drafts",
			cols: []principalColumn{
				{"id", "uuid", false, ""}, {"user_id", "uuid", false, ""}, {"thread_id", "uuid", true, ""},
				{"channel_id", "uuid", false, ""}, {"content_md", "text", false, "''::text"},
				{"reply_to_message_id", "uuid", true, ""}, {"file_ids", "ARRAY", false, "'{}'::uuid[]"},
				{"version", "bigint", false, "1"}, {"updated_by_device_id", "uuid", true, ""}, {"updated_at", tz, false, "now()"},
			},
			cons: []string{
				"FOREIGN KEY (channel_id) REFERENCES channels(id) ON DELETE CASCADE",
				"FOREIGN KEY (reply_to_message_id) REFERENCES messages(id) ON DELETE SET NULL",
				"FOREIGN KEY (thread_id) REFERENCES threads(id) ON DELETE CASCADE",
				"FOREIGN KEY (updated_by_device_id) REFERENCES devices(id) ON DELETE SET NULL",
				"FOREIGN KEY (user_id) REFERENCES users(principal_id) ON DELETE CASCADE",
				"PRIMARY KEY (id)",
			},
			idx: []string{
				"CREATE UNIQUE INDEX drafts_pkey ON public.drafts USING btree (id)",
				"CREATE UNIQUE INDEX drafts_user_channel_key ON public.drafts USING btree (user_id, channel_id) WHERE (thread_id IS NULL)",
				"CREATE UNIQUE INDEX drafts_user_thread_key ON public.drafts USING btree (user_id, thread_id) WHERE (thread_id IS NOT NULL)",
			},
		},
		{
			table: "user_settings",
			cols: []principalColumn{
				{"user_id", "uuid", false, ""}, {"key", "text", false, ""}, {"value", "jsonb", false, ""},
				{"version", "bigint", false, "1"}, {"updated_by_device_id", "uuid", true, ""}, {"updated_at", tz, false, "now()"},
			},
			cons: []string{
				"FOREIGN KEY (updated_by_device_id) REFERENCES devices(id) ON DELETE SET NULL",
				"FOREIGN KEY (user_id) REFERENCES users(principal_id) ON DELETE CASCADE",
				"PRIMARY KEY (user_id, key)",
			},
			idx: []string{"CREATE UNIQUE INDEX user_settings_pkey ON public.user_settings USING btree (user_id, key)"},
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
}

// TestSyncPersonalDataMigration_Down: down to version 41 removes the three tables, up applies
// again and a second up changes nothing.
func TestSyncPersonalDataMigration_Down(t *testing.T) {
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
	if _, err := provider.DownTo(ctx, 41); err != nil {
		t.Fatalf("down to 41: %v", err)
	}
	var left int
	if err := sqlDB.QueryRowContext(ctx, `SELECT (to_regclass('read_states') IS NOT NULL)::int + (to_regclass('drafts') IS NOT NULL)::int
		+ (to_regclass('user_settings') IS NOT NULL)::int`).Scan(&left); err != nil {
		t.Fatal(err)
	}
	if left != 0 {
		t.Errorf("%d tables survive the down migration", left)
	}
	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("up again: %v", err)
	}
	res, err := provider.Up(ctx)
	if err != nil || len(res) != 0 {
		t.Fatalf("second up: %d results, %v", len(res), err)
	}
}
