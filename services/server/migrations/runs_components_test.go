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

// TestMigration_ComponentsAndInteractions holds message_components and interactions against
// docs/spec/03-data-model.md section 4.8 column by column, with every check, key and index, and
// the foreign key runs.triggering_interaction_id that 00051 left out (M2-15). As bothy_app it
// then shows the rules: the checks of type, restricted_to and the two states, the unique
// custom_id per message and client_interaction_id per principal, and the cascades: deleting a
// component deletes its interactions, which sets runs.triggering_interaction_id to NULL without
// breaking the triggering_* check of runs; deleting the message deletes its components.
func TestMigration_ComponentsAndInteractions(t *testing.T) {
	t.Parallel()
	db := pgtest.New(t)
	conn := db.SuperConn(t)
	tz := "timestamp with time zone"
	wantComponents := []principalColumn{
		{"id", "uuid", false, ""},
		{"message_id", "uuid", false, ""},
		{"custom_id", "text", false, ""},
		{"type", "text", false, ""},
		{"definition", "jsonb", false, ""},
		{"position", "smallint", false, ""},
		{"single_use", "boolean", false, "true"},
		{"required_capability", "text", true, ""},
		{"restricted_to", "text", false, "'any'::text"},
		{"run_id", "uuid", true, ""},
		{"state", "text", false, "'active'::text"},
		{"expires_at", tz, true, ""},
		{"used_by", "uuid", true, ""},
		{"used_at", tz, true, ""},
		{"version", "bigint", false, "1"},
	}
	if got := principalColumns(t, conn, "message_components"); !reflect.DeepEqual(got, wantComponents) {
		t.Errorf("message_components columns:\n got %+v\nwant %+v", got, wantComponents)
	}
	cons := principalConstraints(t, conn, "message_components")
	slices.Sort(cons)
	wantCons := []string{
		"CHECK ((restricted_to = ANY (ARRAY['any'::text, 'run_starter'::text, 'owner'::text])))",
		"CHECK ((state = ANY (ARRAY['active'::text, 'used'::text, 'expired'::text, 'disabled'::text])))",
		"CHECK ((type = ANY (ARRAY['button'::text, 'select'::text, 'form'::text])))",
		"FOREIGN KEY (message_id) REFERENCES messages(id) ON DELETE CASCADE",
		"FOREIGN KEY (run_id) REFERENCES runs(id) ON DELETE SET NULL",
		"FOREIGN KEY (used_by) REFERENCES principals(id) ON DELETE RESTRICT",
		"PRIMARY KEY (id)",
		"UNIQUE (message_id, custom_id)",
	}
	slices.Sort(wantCons)
	if !reflect.DeepEqual(cons, wantCons) {
		t.Errorf("message_components constraints:\n got %q\nwant %q", cons, wantCons)
	}
	wantIdx := []string{
		"CREATE INDEX message_components_expires_active_idx ON public.message_components USING btree (expires_at) WHERE (state = 'active'::text)",
		"CREATE INDEX message_components_run_active_idx ON public.message_components USING btree (run_id) WHERE (state = 'active'::text)",
		"CREATE UNIQUE INDEX message_components_message_id_custom_id_key ON public.message_components USING btree (message_id, custom_id)",
		"CREATE UNIQUE INDEX message_components_pkey ON public.message_components USING btree (id)",
	}
	if got := indexes(t, conn, "message_components"); !reflect.DeepEqual(got, wantIdx) {
		t.Errorf("message_components indexes:\n got %q\nwant %q", got, wantIdx)
	}

	wantInteractions := []principalColumn{
		{"id", "uuid", false, ""},
		{"component_id", "uuid", false, ""},
		{"message_id", "uuid", false, ""},
		{"principal_id", "uuid", false, ""},
		{"client_interaction_id", "uuid", false, ""},
		{"values", "jsonb", false, "'{}'::jsonb"},
		{"state", "text", false, "'received'::text"},
		{"response", "jsonb", true, ""},
		{"created_at", tz, false, "now()"},
		{"updated_at", tz, false, "now()"},
	}
	if got := principalColumns(t, conn, "interactions"); !reflect.DeepEqual(got, wantInteractions) {
		t.Errorf("interactions columns:\n got %+v\nwant %+v", got, wantInteractions)
	}
	icons := principalConstraints(t, conn, "interactions")
	slices.Sort(icons)
	wantICons := []string{
		"CHECK ((state = ANY (ARRAY['received'::text, 'dispatched'::text, 'acknowledged'::text, 'failed'::text, 'rejected'::text])))",
		"FOREIGN KEY (component_id) REFERENCES message_components(id) ON DELETE CASCADE",
		"FOREIGN KEY (message_id) REFERENCES messages(id) ON DELETE CASCADE",
		"FOREIGN KEY (principal_id) REFERENCES principals(id) ON DELETE RESTRICT",
		"PRIMARY KEY (id)",
		"UNIQUE (principal_id, client_interaction_id)",
	}
	slices.Sort(wantICons)
	if !reflect.DeepEqual(icons, wantICons) {
		t.Errorf("interactions constraints:\n got %q\nwant %q", icons, wantICons)
	}
	wantIIdx := []string{
		"CREATE INDEX interactions_component_id_idx ON public.interactions USING btree (component_id)",
		"CREATE INDEX interactions_open_state_idx ON public.interactions USING btree (state) WHERE (state = ANY (ARRAY['received'::text, 'dispatched'::text]))",
		"CREATE UNIQUE INDEX interactions_pkey ON public.interactions USING btree (id)",
		"CREATE UNIQUE INDEX interactions_principal_id_client_interaction_id_key ON public.interactions USING btree (principal_id, client_interaction_id)",
	}
	if got := indexes(t, conn, "interactions"); !reflect.DeepEqual(got, wantIIdx) {
		t.Errorf("interactions indexes:\n got %q\nwant %q", got, wantIIdx)
	}
	var def string
	if err := conn.QueryRow(context.Background(), `SELECT pg_get_constraintdef(oid) FROM pg_constraint
		WHERE conname = 'runs_triggering_interaction_id_fkey'`).Scan(&def); err != nil {
		t.Fatalf("runs_triggering_interaction_id_fkey: %v", err)
	}
	if def != "FOREIGN KEY (triggering_interaction_id) REFERENCES interactions(id) ON DELETE SET NULL" {
		t.Errorf("runs triggering interaction key %q", def)
	}

	app := db.Pool(t, pg.RoleApp)
	const (
		person  = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f6601"
		agent   = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f6602"
		space   = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f6611"
		channel = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f6621"
		main    = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f6631"
		card    = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f6641"
		comp1   = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f6651"
		comp2   = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f6652"
		comp3   = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f6653"
		inter1  = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f6661"
		inter2  = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f6662"
		client1 = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f6671"
		run1    = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f6681"
	)
	mustExec(t, app, `INSERT INTO principals (id, kind, handle, display_name) VALUES ($1, 'user', 'alex', 'Alex')`, person)
	mustExec(t, app, `INSERT INTO users (principal_id) VALUES ($1)`, person)
	mustExec(t, app, `INSERT INTO principals (id, kind, handle, display_name) VALUES ($1, 'agent', 'lotse', 'Lotse')`, agent)
	mustExec(t, app, `INSERT INTO agents (principal_id, adapter_kind) VALUES ($1, 'custom')`, agent)
	mustExec(t, app, `INSERT INTO spaces (id, name, sort_key, created_by) VALUES ($1, 'Support', 'a0', $2)`, space, person)
	mustExec(t, app, `INSERT INTO channels (id, space_id, kind, name, sort_key, history_backfill_limit, created_by)
		VALUES ($1, $2, 'text', 'general', 'a0', 20, $3)`, channel, space, person)
	mustExec(t, app, `INSERT INTO threads (id, channel_id, kind, created_by) VALUES ($1, $2, 'main', $3)`, main, channel, person)
	mustExec(t, app, `INSERT INTO messages (id, thread_id, channel_id, thread_seq, author_principal_id, client_msg_id,
		client_payload_sha256, origin, kind, content_md) VALUES ($1, $2, $3, 1, $4, $1, '\x00', 'agent', 'card', 'Approve the refund?')`,
		card, main, channel, agent)

	comp := `INSERT INTO message_components (id, message_id, custom_id, type, definition, position, restricted_to, state)
		VALUES ($1, $2, $3, $4, '{}', $5, $6, $7)`
	mustExec(t, app, comp, comp1, card, "approve", "button", 1, "any", "active")
	mustExec(t, app, comp, comp2, card, "deny", "button", 2, "run_starter", "active")
	wantState(t, exec(app, comp, comp3, card, "approve", "button", 3, "any", "active"), "23505", "a custom_id twice in one card")
	wantCheckViolation(t, exec(app, comp, comp3, card, "c3", "slider", 3, "any", "active"), "an unknown component type")
	wantCheckViolation(t, exec(app, comp, comp3, card, "c3", "button", 3, "everyone", "active"), "an unknown restriction")
	wantCheckViolation(t, exec(app, comp, comp3, card, "c3", "button", 3, "any", "gone"), "an unknown component state")

	inter := `INSERT INTO interactions (id, component_id, message_id, principal_id, client_interaction_id, state)
		VALUES ($1, $2, $3, $4, $5, $6)`
	mustExec(t, app, inter, inter1, comp1, card, person, client1, "received")
	wantState(t, exec(app, inter, inter2, comp2, card, person, client1, "received"), "23505", "a client_interaction_id twice for one principal")
	wantCheckViolation(t, exec(app, inter, inter2, comp2, card, person, run1, "done"), "an unknown interaction state")

	// A run started by the interaction; deleting the component cascades to the interaction, and
	// the run keeps trigger_kind interaction with a NULL triggering_interaction_id.
	mustExec(t, app, `INSERT INTO runs (id, thread_id, agent_id, trigger_kind, triggering_interaction_id, triggered_by,
		billing_mode_expected, invocation_jti) VALUES ($1, $2, $3, 'interaction', $4, $5, 'organization', $1)`, run1, main, agent, inter1, person)
	mustExec(t, app, `UPDATE message_components SET run_id = $1 WHERE id = $2`, run1, comp2)
	mustExec(t, app, `DELETE FROM message_components WHERE id = $1`, comp1)
	var interactions int
	var trigger *string
	if err := app.QueryRow(context.Background(), `SELECT (SELECT count(*)::int FROM interactions), triggering_interaction_id::text
		FROM runs WHERE id = $1`, run1).Scan(&interactions, &trigger); err != nil {
		t.Fatal(err)
	}
	if interactions != 0 || trigger != nil {
		t.Errorf("after deleting the component: %d interactions, triggering_interaction_id %v", interactions, trigger)
	}
	// Deleting the message deletes its components (03 messages "Deletion ... deletes components").
	mustExec(t, app, inter, inter2, comp2, card, person, run1, "received")
	if _, err := db.Pool(t, pg.RoleOwner).Exec(context.Background(), `DELETE FROM messages WHERE id = $1`, card); err != nil {
		t.Fatalf("delete of the card: %v", err)
	}
	var left int
	if err := app.QueryRow(context.Background(), `SELECT (SELECT count(*)::int FROM message_components)
		+ (SELECT count(*)::int FROM interactions)`).Scan(&left); err != nil {
		t.Fatal(err)
	}
	if left != 0 {
		t.Errorf("%d components and interactions left after the card was deleted", left)
	}
}

// TestMigration_ComponentsDown: down to version 56 removes interactions, message_components and
// the foreign key of runs.triggering_interaction_id; up applies again.
func TestMigration_ComponentsDown(t *testing.T) {
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
	if _, err := provider.DownTo(ctx, 60); err != nil {
		t.Fatalf("down to 60: %v", err)
	}
	var left int
	if err := sqlDB.QueryRowContext(ctx, `SELECT (to_regclass('interactions') IS NOT NULL)::int
		+ (to_regclass('message_components') IS NOT NULL)::int
		+ (SELECT count(*)::int FROM pg_constraint WHERE conname = 'runs_triggering_interaction_id_fkey')`).Scan(&left); err != nil {
		t.Fatal(err)
	}
	if left != 0 {
		t.Errorf("%d of interactions, message_components and the run key left after down", left)
	}
	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("up after down: %v", err)
	}
}
