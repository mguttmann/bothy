// SPDX-License-Identifier: Apache-2.0

package migrations_test

import (
	"reflect"
	"slices"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
)

// TestChatProjectBindingsMigration_Schema holds project_bindings against
// docs/spec/03-data-model.md section 4.5 column by column, with the primary key, both foreign
// keys and the unique index on lower(workspace_ref) (M2-03), and the columns provision and
// ready_reason with the check of provision (M4-43, R-DM-55).
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
		{"provision", "text", false, "'none'::text"},
		{"ready_reason", "text", true, ""},
		// FEAT-delete-spaces-chats (migration 00086).
		{"trash_check_ok", "boolean", true, ""},
		{"trash_check_reason", "text", true, ""},
		{"trash_checked_at", ts, true, ""},
		{"trash_state", "text", true, ""},
		{"trash_reason", "text", true, ""},
		{"trash_agent_id", "uuid", true, ""},
		{"trash_requested_at", ts, true, ""},
	}
	constraints := []string{
		"CHECK ((provision = ANY (ARRAY['none'::text, 'init'::text, 'clone'::text])))",
		"CHECK ((((trash_state IS NULL) = (trash_agent_id IS NULL)) AND ((trash_state IS NULL) = (trash_requested_at IS NULL))))",
		"CHECK (((trash_check_ok IS NULL) = (trash_checked_at IS NULL)))",
		"CHECK (((trash_check_reason IS NULL) OR (trash_check_ok = false)))",
		"CHECK (((trash_reason IS NULL) OR (trash_state = 'refused'::text)))",
		"CHECK ((trash_state = ANY (ARRAY['requested'::text, 'trashed'::text, 'refused'::text])))",
		"FOREIGN KEY (trash_agent_id) REFERENCES agents(principal_id) ON DELETE RESTRICT",
		"FOREIGN KEY (channel_id) REFERENCES channels(id) ON DELETE CASCADE",
		"FOREIGN KEY (created_by) REFERENCES principals(id) ON DELETE RESTRICT",
		"PRIMARY KEY (channel_id)",
	}
	idx := []string{
		"CREATE UNIQUE INDEX project_bindings_pkey ON public.project_bindings USING btree (channel_id)",
		"CREATE INDEX project_bindings_trash_requested_idx ON public.project_bindings USING btree (trash_agent_id) WHERE (trash_state = 'requested'::text)",
		"CREATE UNIQUE INDEX project_bindings_workspace_ref_key ON public.project_bindings USING btree (lower(workspace_ref))",
	}
	if got := principalColumns(t, conn, "project_bindings"); !reflect.DeepEqual(got, columns) {
		t.Errorf("columns:\n got %+v\nwant %+v", got, columns)
	}
	got := principalConstraints(t, conn, "project_bindings")
	slices.Sort(got)
	slices.Sort(constraints)
	slices.Sort(idx)
	if !reflect.DeepEqual(got, constraints) {
		t.Errorf("constraints:\n got %q\nwant %q", got, constraints)
	}
	gotIdx := indexes(t, conn, "project_bindings")
	slices.Sort(gotIdx)
	if !reflect.DeepEqual(gotIdx, idx) {
		t.Errorf("indexes:\n got %q\nwant %q", gotIdx, idx)
	}
}
