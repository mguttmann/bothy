// SPDX-License-Identifier: Apache-2.0

package migrations_test

import (
	"context"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/pressly/goose/v3"

	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
	"github.com/OWNER/bothy/services/server/migrations"
)

// TestMigrationStatusPage holds the four status tables against docs/spec/03-data-model.md 4.12
// column by column with their checks, keys and indexes (M7-09): the unique lower(name) of the
// components, the closed states, kinds and impacts, the cascades of the incident children and of a
// removed component, the agent link that becomes NULL with its agent, and the Down section, which
// drops the tables again. The rights of bothy_app are held by appPrivileges.
func TestMigrationStatusPage(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := pgtest.New(t)
	conn := db.SuperConn(t)
	tz := "timestamp with time zone"

	cols := map[string][]principalColumn{
		"status_components": {
			{"id", "uuid", false, ""}, {"name", "text", false, ""}, {"description", "text", false, "''::text"},
			{"sort_key", "text", false, ""}, {"state", "text", false, "'operational'::text"},
			{"health_source", "text", true, ""}, {"agent_id", "uuid", true, ""}, {"updated_at", tz, false, "now()"},
		},
		"status_incidents": {
			{"id", "uuid", false, ""}, {"title", "text", false, ""}, {"kind", "text", false, "'incident'::text"},
			{"impact", "text", false, ""}, {"state", "text", false, "'investigating'::text"},
			{"started_at", tz, false, ""}, {"resolved_at", tz, true, ""}, {"created_by", "uuid", true, ""},
			{"hidden", "boolean", false, "false"},
		},
		"status_incident_updates": {
			{"id", "uuid", false, ""}, {"incident_id", "uuid", false, ""}, {"state", "text", false, ""},
			{"body_md", "text", false, ""}, {"created_by", "uuid", true, ""}, {"created_at", tz, false, "now()"},
		},
		"status_incident_components": {{"incident_id", "uuid", false, ""}, {"component_id", "uuid", false, ""}},
	}
	for table, want := range cols {
		if got := principalColumns(t, conn, table); !reflect.DeepEqual(got, want) {
			t.Errorf("%s columns:\n got %+v\nwant %+v", table, got, want)
		}
	}
	idx := map[string][]string{
		"status_components": {
			"CREATE INDEX status_components_sort_key_idx ON public.status_components USING btree (sort_key)",
			"CREATE UNIQUE INDEX status_components_name_lower_key ON public.status_components USING btree (lower(name))",
			"CREATE UNIQUE INDEX status_components_pkey ON public.status_components USING btree (id)",
		},
		"status_incidents": {
			"CREATE INDEX status_incidents_state_started_at_idx ON public.status_incidents USING btree (state, started_at DESC)",
			"CREATE UNIQUE INDEX status_incidents_pkey ON public.status_incidents USING btree (id)",
		},
		"status_incident_updates": {
			"CREATE INDEX status_incident_updates_incident_id_created_at_idx ON public.status_incident_updates USING btree (incident_id, created_at)",
			"CREATE UNIQUE INDEX status_incident_updates_pkey ON public.status_incident_updates USING btree (id)",
		},
		"status_incident_components": {
			"CREATE INDEX status_incident_components_component_id_idx ON public.status_incident_components USING btree (component_id)",
			"CREATE UNIQUE INDEX status_incident_components_pkey ON public.status_incident_components USING btree (incident_id, component_id)",
		},
	}
	for table, want := range idx {
		if got := indexes(t, conn, table); !reflect.DeepEqual(got, want) {
			t.Errorf("%s indexes:\n got %q\nwant %q", table, got, want)
		}
	}
	cons := principalConstraints(t, conn, "status_components")
	for _, want := range []string{
		"FOREIGN KEY (agent_id) REFERENCES agents(principal_id) ON DELETE SET NULL",
		"CHECK ((state = ANY (ARRAY['operational'::text, 'degraded'::text, 'partial_outage'::text, 'major_outage'::text, 'maintenance'::text])))",
	} {
		if !slices.Contains(cons, want) {
			t.Errorf("status_components constraints %q lack %q", cons, want)
		}
	}
	cons = principalConstraints(t, conn, "status_incident_components")
	for _, want := range []string{
		"FOREIGN KEY (component_id) REFERENCES status_components(id) ON DELETE CASCADE",
		"FOREIGN KEY (incident_id) REFERENCES status_incidents(id) ON DELETE CASCADE",
	} {
		if !slices.Contains(cons, want) {
			t.Errorf("status_incident_components constraints %q lack %q", cons, want)
		}
	}

	// The closed values and the unique name, case-insensitive.
	insert := `INSERT INTO status_components (id, name, sort_key, state) VALUES (gen_random_uuid(), $1, 'a0', $2)`
	if _, err := conn.Exec(ctx, insert, "Messages", "operational"); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ name, state, check string }{
		{"MESSAGES", "operational", "status_components_name_lower_key"},
		{"Push", "down", "status_components_state_check"},
	} {
		if _, err := conn.Exec(ctx, insert, c.name, c.state); err == nil || !strings.Contains(err.Error(), c.check) {
			t.Errorf("insert %s %s: %v, want %s", c.name, c.state, err, c.check)
		}
	}
	incident := `INSERT INTO status_incidents (id, title, kind, impact, state, started_at) VALUES (gen_random_uuid(), 'x', $1, $2, $3, now())`
	for _, c := range []struct{ kind, impact, state, check string }{
		{"outage", "none", "investigating", "status_incidents_kind_check"},
		{"incident", "huge", "investigating", "status_incidents_impact_check"},
		{"incident", "none", "open", "status_incidents_state_check"},
	} {
		if _, err := conn.Exec(ctx, incident, c.kind, c.impact, c.state); err == nil || !strings.Contains(err.Error(), c.check) {
			t.Errorf("incident %v: %v, want %s", c, err, c.check)
		}
	}

	// Removing a component removes its links; removing an incident its updates and links.
	var comp, inc string
	if err := conn.QueryRow(ctx, `SELECT id FROM status_components`).Scan(&comp); err != nil {
		t.Fatal(err)
	}
	if err := conn.QueryRow(ctx, `INSERT INTO status_incidents (id, title, impact, started_at) VALUES (gen_random_uuid(), 'x', 'major', now()) RETURNING id`).Scan(&inc); err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{
		`INSERT INTO status_incident_components VALUES ($1, $2)`,
		`INSERT INTO status_incident_updates (id, incident_id, state, body_md) VALUES (gen_random_uuid(), $1, 'investigating', 'x')`,
	} {
		args := []any{inc}
		if strings.Contains(q, "$2") {
			args = append(args, comp)
		}
		if _, err := conn.Exec(ctx, q, args...); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	if _, err := conn.Exec(ctx, `DELETE FROM status_components WHERE id = $1`, comp); err != nil {
		t.Fatal(err)
	}
	var links, updates int
	if err := conn.QueryRow(ctx, `SELECT (SELECT count(*) FROM status_incident_components), (SELECT count(*) FROM status_incident_updates)`).Scan(&links, &updates); err != nil {
		t.Fatal(err)
	}
	if links != 0 || updates != 1 {
		t.Errorf("after removing the component: links %d updates %d, want 0 and 1", links, updates)
	}
	if _, err := conn.Exec(ctx, `DELETE FROM status_incidents WHERE id = $1`, inc); err != nil {
		t.Fatal(err)
	}
	if err := conn.QueryRow(ctx, `SELECT count(*) FROM status_incident_updates`).Scan(&updates); err != nil || updates != 0 {
		t.Errorf("updates after removing the incident: %d, %v", updates, err)
	}

	// Down drops the four tables and takes DELETE on backup_runs back; Up creates them again.
	pool := db.OpenPool(t, db.Config(pg.RoleOwner).ForMigrate(5*time.Second))
	sqlDB := pool.OpenSQL()
	t.Cleanup(func() { _ = sqlDB.Close() })
	provider, err := goose.NewProvider(goose.DialectPostgres, sqlDB, migrations.FS, goose.WithDisableGlobalRegistry(true))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.DownTo(ctx, 76); err != nil {
		t.Fatalf("down to 76: %v", err)
	}
	var left int
	var canDelete bool
	if err := conn.QueryRow(ctx, `SELECT count(*) FROM pg_tables WHERE schemaname = 'public' AND tablename LIKE 'status\_%'`).Scan(&left); err != nil || left != 0 {
		t.Errorf("status tables after down: %d, %v", left, err)
	}
	if err := conn.QueryRow(ctx, `SELECT has_table_privilege('bothy_app', 'backup_runs', 'DELETE')`).Scan(&canDelete); err != nil || canDelete {
		t.Errorf("bothy_app DELETE on backup_runs after down: %v, %v", canDelete, err)
	}
	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("up again: %v", err)
	}
}
