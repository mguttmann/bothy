// SPDX-License-Identifier: Apache-2.0

package migrations_test

import (
	"reflect"
	"slices"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
)

// TestInstanceSettingsMigration_Schema holds instance_settings against
// docs/spec/03-datenmodell.md section 4.1 column by column, with its key and foreign key (M1-46).
func TestInstanceSettingsMigration_Schema(t *testing.T) {
	t.Parallel()
	conn := pgtest.New(t).SuperConn(t)
	ts := "timestamp with time zone"
	wantCols := []principalColumn{
		{"key", "text", false, ""},
		{"value", "jsonb", false, ""},
		{"version", "bigint", false, "1"},
		{"updated_by", "uuid", true, ""},
		{"updated_at", ts, false, "now()"},
	}
	if got := principalColumns(t, conn, "instance_settings"); !reflect.DeepEqual(got, wantCols) {
		t.Errorf("columns:\n got %+v\nwant %+v", got, wantCols)
	}
	wantCons := []string{
		"FOREIGN KEY (updated_by) REFERENCES principals(id) ON DELETE RESTRICT",
		"PRIMARY KEY (key)",
	}
	gotCons := principalConstraints(t, conn, "instance_settings")
	slices.Sort(gotCons)
	if !reflect.DeepEqual(gotCons, wantCons) {
		t.Errorf("constraints:\n got %q\nwant %q", gotCons, wantCons)
	}
	wantIdx := []string{"CREATE UNIQUE INDEX instance_settings_pkey ON public.instance_settings USING btree (key)"}
	if got := indexes(t, conn, "instance_settings"); !reflect.DeepEqual(got, wantIdx) {
		t.Errorf("indexes:\n got %q\nwant %q", got, wantIdx)
	}
}
