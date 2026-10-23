// SPDX-License-Identifier: Apache-2.0

package migrations_test

import (
	"context"
	"reflect"
	"slices"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
)

// TestMigrationGatewayPresence holds gateway_presence against docs/spec/03-data-model.md section
// 4.16 (R-DM-51, M7-17): the columns with their defaults, the primary key, no foreign key, the two
// indexes and the persistence UNLOGGED.
func TestMigrationGatewayPresence(t *testing.T) {
	t.Parallel()
	db := pgtest.New(t)
	conn := db.SuperConn(t)
	cols := []principalColumn{
		{"gateway_session_id", "uuid", false, ""}, {"node_id", "uuid", false, ""}, {"principal_id", "uuid", false, ""},
		{"device_id", "uuid", false, ""}, {"focus_thread_ids", "ARRAY", false, "'{}'::uuid[]"}, {"idle", "boolean", false, "false"},
		{"updated_at", "timestamp with time zone", false, "now()"},
	}
	if got := principalColumns(t, conn, "gateway_presence"); !reflect.DeepEqual(got, cols) {
		t.Errorf("columns:\n got %+v\nwant %+v", got, cols)
	}
	cons := principalConstraints(t, conn, "gateway_presence")
	slices.Sort(cons)
	if want := []string{"PRIMARY KEY (gateway_session_id)"}; !reflect.DeepEqual(cons, want) {
		t.Errorf("constraints: %q", cons)
	}
	idx := []string{
		"CREATE INDEX gateway_presence_node_id_idx ON public.gateway_presence USING btree (node_id)",
		"CREATE INDEX gateway_presence_principal_id_idx ON public.gateway_presence USING btree (principal_id)",
		"CREATE UNIQUE INDEX gateway_presence_pkey ON public.gateway_presence USING btree (gateway_session_id)",
	}
	if got := indexes(t, conn, "gateway_presence"); !reflect.DeepEqual(got, idx) {
		t.Errorf("indexes:\n got %q\nwant %q", got, idx)
	}
	var persistence string
	if err := conn.QueryRow(context.Background(), `SELECT relpersistence::text FROM pg_class WHERE relname = 'gateway_presence'`).Scan(&persistence); err != nil {
		t.Fatal(err)
	}
	if persistence != "u" {
		t.Fatalf("relpersistence %q, want u (UNLOGGED)", persistence)
	}
}
