// SPDX-License-Identifier: Apache-2.0

package migrations_test

import (
	"reflect"
	"slices"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
)

// TestFeatureFlagsMigration_Schema holds feature_flags against docs/spec/03-data-model.md section
// 4.1 column by column, with its key and foreign key (M1-47).
func TestFeatureFlagsMigration_Schema(t *testing.T) {
	t.Parallel()
	conn := pgtest.New(t).SuperConn(t)
	wantCols := []principalColumn{
		{"key", "text", false, ""},
		{"enabled", "boolean", false, "false"},
		{"rollout", "jsonb", false, "'{}'::jsonb"},
		{"version", "bigint", false, "1"},
		{"updated_by", "uuid", true, ""},
		{"updated_at", "timestamp with time zone", false, "now()"},
	}
	if got := principalColumns(t, conn, "feature_flags"); !reflect.DeepEqual(got, wantCols) {
		t.Errorf("columns:\n got %+v\nwant %+v", got, wantCols)
	}
	wantCons := []string{
		"FOREIGN KEY (updated_by) REFERENCES principals(id) ON DELETE RESTRICT",
		"PRIMARY KEY (key)",
	}
	gotCons := principalConstraints(t, conn, "feature_flags")
	slices.Sort(gotCons)
	if !reflect.DeepEqual(gotCons, wantCons) {
		t.Errorf("constraints:\n got %q\nwant %q", gotCons, wantCons)
	}
	wantIdx := []string{"CREATE UNIQUE INDEX feature_flags_pkey ON public.feature_flags USING btree (key)"}
	if got := indexes(t, conn, "feature_flags"); !reflect.DeepEqual(got, wantIdx) {
		t.Errorf("indexes:\n got %q\nwant %q", got, wantIdx)
	}
}
