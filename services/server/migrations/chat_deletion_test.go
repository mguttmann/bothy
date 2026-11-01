// SPDX-License-Identifier: Apache-2.0

package migrations_test

import (
	"context"
	"reflect"
	"slices"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
)

// TestChatDeletionMigration_Schema holds the deletion columns of 00086 against
// docs/spec/03-data-model.md (R-DM-57): every check and foreign key is validated (00087) and the
// partial indexes of the purge job and of the folder requests exist (00088).
func TestChatDeletionMigration_Schema(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	conn := pgtest.New(t).SuperConn(t)
	rows, err := conn.Query(ctx, `SELECT conrelid::regclass::text || '.' || conname, convalidated FROM pg_constraint
		WHERE conrelid IN ('spaces'::regclass, 'channels'::regclass, 'threads'::regclass, 'project_bindings'::regclass)
		AND (conname LIKE '%deleted%' OR conname LIKE '%purged%' OR conname LIKE 'project_bindings_trash%') ORDER BY 1`)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for rows.Next() {
		var name string
		var valid bool
		if err := rows.Scan(&name, &valid); err != nil {
			t.Fatal(err)
		}
		if !valid {
			t.Errorf("%s is not validated", name)
		}
		got = append(got, name)
	}
	rows.Close()
	want := []string{
		"channels.channels_deleted_archived_check", "channels.channels_deleted_by_fkey", "channels.channels_deleted_check",
		"channels.channels_purged_check",
		"project_bindings.project_bindings_trash_agent_id_fkey", "project_bindings.project_bindings_trash_check_check",
		"project_bindings.project_bindings_trash_check_reason_check", "project_bindings.project_bindings_trash_reason_check",
		"project_bindings.project_bindings_trash_request_check", "project_bindings.project_bindings_trash_state_check",
		"spaces.spaces_deleted_archived_check", "spaces.spaces_deleted_by_fkey", "spaces.spaces_deleted_check",
		"spaces.spaces_purged_check",
		"threads.threads_deleted_by_fkey", "threads.threads_deleted_check", "threads.threads_deleted_main_check",
		"threads.threads_purged_check",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("constraints:\n got %q\nwant %q", got, want)
	}
	for table, idx := range map[string]string{
		"spaces":           "CREATE INDEX spaces_purge_pending_idx ON public.spaces USING btree (deleted_at) WHERE ((deleted_at IS NOT NULL) AND (purged_at IS NULL))",
		"channels":         "CREATE INDEX channels_purge_pending_idx ON public.channels USING btree (deleted_at) WHERE ((deleted_at IS NOT NULL) AND (purged_at IS NULL))",
		"threads":          "CREATE INDEX threads_purge_pending_idx ON public.threads USING btree (deleted_at) WHERE ((deleted_at IS NOT NULL) AND (purged_at IS NULL))",
		"project_bindings": "CREATE INDEX project_bindings_trash_requested_idx ON public.project_bindings USING btree (trash_agent_id) WHERE (trash_state = 'requested'::text)",
	} {
		if !slices.Contains(indexes(t, conn, table), idx) {
			t.Errorf("%s lacks %q", table, idx)
		}
	}
}

// TestChatDeletionMigration_Constraints exercises the rules as bothy_app: a deleted space or
// channel is archived, deleted_at and deleted_by go together, purged_at only on a deleted row, a
// main thread is never deleted, and the folder check and folder request columns of a binding are
// consistent.
func TestChatDeletionMigration_Constraints(t *testing.T) {
	t.Parallel()
	db := pgtest.New(t)
	app := db.Pool(t, pg.RoleApp)
	const (
		person  = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f8601"
		agent   = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f8602"
		space   = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f8611"
		channel = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f8621"
		main    = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f8631"
		chat    = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f8632"
	)
	mustExec(t, app, `INSERT INTO principals (id, kind, handle, display_name) VALUES ($1, 'user', 'alex', 'Alex')`, person)
	mustExec(t, app, `INSERT INTO users (principal_id) VALUES ($1)`, person)
	mustExec(t, app, `INSERT INTO principals (id, kind, handle, display_name) VALUES ($1, 'agent', 'robin', 'Robin')`, agent)
	mustExec(t, app, `INSERT INTO agents (principal_id, adapter_kind) VALUES ($1, 'custom')`, agent)
	mustExec(t, app, `INSERT INTO spaces (id, name, sort_key, created_by) VALUES ($1, 'Support', 'a0', $2)`, space, person)
	mustExec(t, app, `INSERT INTO channels (id, space_id, kind, name, sort_key, history_backfill_limit, created_by)
		VALUES ($1, $2, 'forum', 'website', 'a0', 20, $3)`, channel, space, person)
	mustExec(t, app, `INSERT INTO threads (id, channel_id, kind, created_by) VALUES ($1, $2, 'main', $3)`, main, channel, person)
	mustExec(t, app, `INSERT INTO threads (id, channel_id, kind, created_by) VALUES ($1, $2, 'thread', $3)`, chat, channel, person)
	mustExec(t, app, `INSERT INTO project_bindings (channel_id, workspace_ref, created_by) VALUES ($1, 'website', $2)`, channel, person)

	for _, table := range []string{"spaces", "channels"} {
		id := space
		if table == "channels" {
			id = channel
		}
		wantState(t, exec(app, `UPDATE `+table+` SET deleted_at = now(), deleted_by = $2 WHERE id = $1`, id, person),
			"23514", table+" deleted without the archive")
		wantState(t, exec(app, `UPDATE `+table+` SET archived_at = now(), deleted_at = now() WHERE id = $1`, id),
			"23514", table+" deleted without deleted_by")
		wantState(t, exec(app, `UPDATE `+table+` SET purged_at = now() WHERE id = $1`, id), "23514", table+" purged while live")
		mustExec(t, app, `UPDATE `+table+` SET archived_at = now(), deleted_at = now(), deleted_by = $2 WHERE id = $1`, id, person)
		mustExec(t, app, `UPDATE `+table+` SET purged_at = now() WHERE id = $1`, id)
	}
	wantState(t, exec(app, `UPDATE threads SET deleted_at = now(), deleted_by = $2 WHERE id = $1`, main, person),
		"23514", "a main thread deleted")
	wantState(t, exec(app, `UPDATE threads SET deleted_at = now() WHERE id = $1`, chat), "23514", "a thread deleted without deleted_by")
	wantState(t, exec(app, `UPDATE threads SET deleted_by = $2 WHERE id = $1`, chat, "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f86ff"),
		"23514", "deleted_by without deleted_at")
	mustExec(t, app, `UPDATE threads SET deleted_at = now(), deleted_by = $2 WHERE id = $1`, chat, person)
	mustExec(t, app, `UPDATE threads SET purged_at = now() WHERE id = $1`, chat)

	upd := `UPDATE project_bindings SET `
	for _, c := range []struct{ set, why string }{
		{"trash_check_ok = true", "a check without its time"},
		{"trash_checked_at = now()", "a check time without a result"},
		{"trash_check_ok = true, trash_checked_at = now(), trash_check_reason = 'symlink'", "a reason with an ok check"},
		{"trash_state = 'requested', trash_requested_at = now()", "a request without an agent"},
		{"trash_state = 'moved', trash_agent_id = '" + agent + "', trash_requested_at = now()", "an unknown state"},
		{"trash_state = 'trashed', trash_agent_id = '" + agent + "', trash_requested_at = now(), trash_reason = 'symlink'",
			"a reason without a refusal"},
	} {
		wantState(t, exec(app, upd+c.set+` WHERE channel_id = $1`, channel), "23514", c.why)
	}
	wantState(t, exec(app, upd+`trash_state = 'requested', trash_agent_id = $2, trash_requested_at = now() WHERE channel_id = $1`,
		channel, person), "23503", "a person as the serving agent")
	mustExec(t, app, upd+`trash_check_ok = false, trash_check_reason = 'symlink', trash_checked_at = now() WHERE channel_id = $1`, channel)
	mustExec(t, app, upd+`trash_state = 'requested', trash_agent_id = $2, trash_requested_at = now() WHERE channel_id = $1`, channel, agent)
	mustExec(t, app, upd+`trash_state = 'refused', trash_reason = 'uncommitted_changes' WHERE channel_id = $1`, channel)
}
