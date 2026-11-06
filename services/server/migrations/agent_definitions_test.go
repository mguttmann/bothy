// SPDX-License-Identifier: Apache-2.0

package migrations_test

import (
	"reflect"
	"slices"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
)

// TestAgentDefinitionsMigration_Schema holds agent_definitions against
// docs/spec/03-data-model.md section 4.4 column by column, with the check, the primary key, the
// two foreign keys and no index besides the primary key (A1-02, R-DM-59).
func TestAgentDefinitionsMigration_Schema(t *testing.T) {
	t.Parallel()
	conn := pgtest.New(t).SuperConn(t)
	columns := []principalColumn{
		{"agent_id", "uuid", false, ""},
		{"version", "bigint", false, ""},
		{"definition", "jsonb", false, ""},
		{"origin_kind", "text", false, ""},
		{"created_at", "timestamp with time zone", false, "now()"},
		{"created_by", "uuid", false, ""},
	}
	constraints := []string{
		"CHECK ((origin_kind = ANY (ARRAY['template'::text, 'interview'::text, 'manual'::text])))",
		"FOREIGN KEY (agent_id) REFERENCES agents(principal_id) ON DELETE RESTRICT",
		"FOREIGN KEY (created_by) REFERENCES principals(id) ON DELETE RESTRICT",
		"PRIMARY KEY (agent_id, version)",
	}
	idx := []string{"CREATE UNIQUE INDEX agent_definitions_pkey ON public.agent_definitions USING btree (agent_id, version)"}
	if got := principalColumns(t, conn, "agent_definitions"); !reflect.DeepEqual(got, columns) {
		t.Errorf("columns:\n got %+v\nwant %+v", got, columns)
	}
	got := principalConstraints(t, conn, "agent_definitions")
	slices.Sort(got)
	slices.Sort(constraints)
	if !reflect.DeepEqual(got, constraints) {
		t.Errorf("constraints:\n got %q\nwant %q", got, constraints)
	}
	if got := indexes(t, conn, "agent_definitions"); !reflect.DeepEqual(got, idx) {
		t.Errorf("indexes:\n got %q\nwant %q", got, idx)
	}
}

// TestAgentDefinitionsMigration_Constraints exercises the rules as bothy_app: a version of an
// existing agent by an existing principal is stored, a second row of the same version, an
// unknown origin kind and an unknown agent are refused, and bothy_app may neither update nor
// delete a version (rows are never updated, R-DM-59).
func TestAgentDefinitionsMigration_Constraints(t *testing.T) {
	t.Parallel()
	db := pgtest.New(t)
	app := db.Pool(t, pg.RoleApp)
	const agent = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f9b01"
	const owner = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f9b02"
	const unknown = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f9b03"
	// A built-in agent names its hosting runner (A1-03, agents_hosted_by_check).
	const runner = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f9b04"
	mustExec(t, app, `INSERT INTO principals (id, kind, handle, display_name) VALUES ($1, 'agent', 'runner', 'Runner')`, runner)
	mustExec(t, app, `INSERT INTO agents (principal_id, adapter_kind) VALUES ($1, 'code_runner')`, runner)
	mustExec(t, app, `INSERT INTO principals (id, kind, handle, display_name) VALUES ($1, 'agent', 'writer', 'Writer')`, agent)
	mustExec(t, app, `INSERT INTO agents (principal_id, adapter_kind, hosted_by) VALUES ($1, 'claude_agent', $2)`, agent, runner)
	mustExec(t, app, `INSERT INTO principals (id, kind, handle, display_name) VALUES ($1, 'user', 'alex', 'Alex')`, owner)
	ins := `INSERT INTO agent_definitions (agent_id, version, definition, origin_kind, created_by) VALUES ($1, $2, '{}', $3, $4)`
	mustExec(t, app, ins, agent, 1, "manual", owner)
	mustExec(t, app, ins, agent, 2, "template", owner)
	wantState(t, exec(app, ins, agent, 2, "interview", owner), "23505", "the same version twice")
	wantState(t, exec(app, ins, agent, 3, "export", owner), "23514", "an unknown origin kind")
	wantState(t, exec(app, ins, unknown, 1, "manual", owner), "23503", "an unknown agent")
	wantState(t, exec(app, ins, agent, 3, "manual", unknown), "23503", "an unknown author")
	wantState(t, exec(app, `UPDATE agent_definitions SET origin_kind = 'manual' WHERE agent_id = $1`, agent), "42501", "bothy_app updates")
	wantState(t, exec(app, `DELETE FROM agent_definitions WHERE agent_id = $1`, agent), "42501", "bothy_app deletes")
}
