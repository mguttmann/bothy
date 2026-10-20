// SPDX-License-Identifier: Apache-2.0

package migrations_test

import (
	"reflect"
	"slices"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
)

// TestChatProjectBindingsMigration_Schema holds project_bindings against
// docs/spec/03-datenmodell.md section 4.5 column by column, with the primary key, both foreign
// keys and the unique index on lower(workspace_ref) (M2-03).
func TestChatProjectBindingsMigration_Schema(t *testing.T) {
	t.Parallel()
	conn := pgtest.New(t).SuperConn(t)
	ts := "timestamp with time zone"
	columns := []principalColumn{
		{"channel_id", "uuid", false, ""},
		{"workspace_ref", "text", false, ""},
		{"repository_url", "text", true, ""},
		{"default_branch", "text", true, ""},
		{"runner_config", "jsonb", false, "'{}'::jsonb"},
		{"ready", "boolean", false, "false"},
		{"created_by", "uuid", false, ""},
		{"created_at", ts, false, "now()"},
		{"version", "bigint", false, "1"},
	}
	constraints := []string{
		"FOREIGN KEY (channel_id) REFERENCES channels(id) ON DELETE CASCADE",
		"FOREIGN KEY (created_by) REFERENCES principals(id) ON DELETE RESTRICT",
		"PRIMARY KEY (channel_id)",
	}
	idx := []string{
		"CREATE UNIQUE INDEX project_bindings_pkey ON public.project_bindings USING btree (channel_id)",
		"CREATE UNIQUE INDEX project_bindings_workspace_ref_key ON public.project_bindings USING btree (lower(workspace_ref))",
	}
	if got := principalColumns(t, conn, "project_bindings"); !reflect.DeepEqual(got, columns) {
		t.Errorf("columns:\n got %+v\nwant %+v", got, columns)
	}
	got := principalConstraints(t, conn, "project_bindings")
	slices.Sort(got)
	if !reflect.DeepEqual(got, constraints) {
		t.Errorf("constraints:\n got %q\nwant %q", got, constraints)
	}
	gotIdx := indexes(t, conn, "project_bindings")
	slices.Sort(gotIdx)
	if !reflect.DeepEqual(gotIdx, idx) {
		t.Errorf("indexes:\n got %q\nwant %q", gotIdx, idx)
	}
}
