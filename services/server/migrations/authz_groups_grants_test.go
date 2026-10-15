// SPDX-License-Identifier: Apache-2.0

package migrations_test

import (
	"context"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/pressly/goose/v3"

	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
	"github.com/OWNER/bothy/services/server/migrations"
)

// authzVersion is the version of 00005_authz_groups_grants.sql.
const authzVersion = 5

// namedConstraints returns "name: definition" of the constraints of table, sorted, without the
// NOT NULL constraints PostgreSQL 18 records. The name is part of the contract here, because
// the adapter maps violations by constraint name (brief M1-27 "Error mapping").
func namedConstraints(t *testing.T, conn *pgx.Conn, table string) []string {
	t.Helper()
	rows, err := conn.Query(context.Background(), `SELECT conname || ': ' || pg_get_constraintdef(oid)
		FROM pg_constraint WHERE conrelid = to_regclass($1) AND contype <> 'n' ORDER BY 1`, table)
	if err != nil {
		t.Fatal(err)
	}
	out, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// TestAuthzGroupsGrantsSchema holds acceptance criterion 1 of M1-27: groups,
// group_memberships and grants match docs/spec/03-data-model.md section 4.3 column by column,
// with every check, unique index, index and foreign key; groups.provider_id is added by the
// migration of identity_providers (M1-36). The only deviations are the deferred foreign keys
// grants.target_space_id, target_channel_id and target_host_id (M1-52, M1-53, M2-39).
func TestAuthzGroupsGrantsSchema(t *testing.T) {
	t.Parallel()
	conn := pgtest.New(t).SuperConn(t)

	tests := []struct {
		table       string
		columns     []principalColumn
		constraints []string
		indexes     []string
	}{
		{
			table: "groups",
			columns: []principalColumn{
				{"id", "uuid", false, ""},
				{"name", "text", false, ""},
				{"description", "text", false, "''::text"},
				{"source", "text", false, "'local'::text"},
				{"provider_id", "uuid", true, ""},
				{"external_ref", "text", true, ""},
				{"created_by", "uuid", false, ""},
				{"created_at", "timestamp with time zone", false, "now()"},
				{"deleted_at", "timestamp with time zone", true, ""},
			},
			constraints: []string{
				"groups_created_by_fkey: FOREIGN KEY (created_by) REFERENCES principals(id) ON DELETE RESTRICT",
				"groups_external_ref_shape_check: CHECK (((source = 'oidc_claim'::text) = (external_ref IS NOT NULL)))",
				"groups_pkey: PRIMARY KEY (id)",
				"groups_provider_id_fkey: FOREIGN KEY (provider_id) REFERENCES identity_providers(id) ON DELETE RESTRICT",
				"groups_provider_shape_check: CHECK (((source = 'oidc_claim'::text) = (provider_id IS NOT NULL)))",
				"groups_source_check: CHECK ((source = ANY (ARRAY['local'::text, 'oidc_claim'::text])))",
			},
			indexes: []string{
				"CREATE UNIQUE INDEX groups_claim_key ON public.groups USING btree (provider_id, external_ref) WHERE ((source = 'oidc_claim'::text) AND (deleted_at IS NULL))",
				"CREATE UNIQUE INDEX groups_live_name_key ON public.groups USING btree (lower(name)) WHERE (deleted_at IS NULL)",
				"CREATE UNIQUE INDEX groups_pkey ON public.groups USING btree (id)",
			},
		},
		{
			table: "group_memberships",
			columns: []principalColumn{
				{"id", "uuid", false, ""},
				{"group_id", "uuid", false, ""},
				{"user_id", "uuid", false, ""},
				{"added_by", "uuid", false, ""},
				{"added_at", "timestamp with time zone", false, "now()"},
				{"expires_at", "timestamp with time zone", true, ""},
				{"expiry_reminded_at", "timestamp with time zone", true, ""},
				{"expiry_reported_at", "timestamp with time zone", true, ""},
				{"removed_at", "timestamp with time zone", true, ""},
				{"removed_by", "uuid", true, ""},
			},
			constraints: []string{
				"group_memberships_added_by_fkey: FOREIGN KEY (added_by) REFERENCES principals(id) ON DELETE RESTRICT",
				"group_memberships_group_id_fkey: FOREIGN KEY (group_id) REFERENCES groups(id) ON DELETE RESTRICT",
				"group_memberships_pkey: PRIMARY KEY (id)",
				"group_memberships_removed_by_fkey: FOREIGN KEY (removed_by) REFERENCES principals(id) ON DELETE RESTRICT",
				"group_memberships_user_id_fkey: FOREIGN KEY (user_id) REFERENCES users(principal_id) ON DELETE RESTRICT",
			},
			indexes: []string{
				"CREATE INDEX group_memberships_active_user_idx ON public.group_memberships USING btree (user_id) WHERE (removed_at IS NULL)",
				"CREATE INDEX group_memberships_expiry_idx ON public.group_memberships USING btree (expires_at) WHERE ((removed_at IS NULL) AND (expires_at IS NOT NULL))",
				"CREATE UNIQUE INDEX group_memberships_active_key ON public.group_memberships USING btree (group_id, user_id) WHERE (removed_at IS NULL)",
				"CREATE UNIQUE INDEX group_memberships_pkey ON public.group_memberships USING btree (id)",
			},
		},
		{
			table: "grants",
			columns: []principalColumn{
				{"id", "uuid", false, ""},
				{"subject_kind", "text", false, ""},
				{"subject_principal_id", "uuid", true, ""},
				{"subject_group_id", "uuid", true, ""},
				{"target_kind", "text", false, ""},
				{"target_space_id", "uuid", true, ""},
				{"target_channel_id", "uuid", true, ""},
				{"target_host_id", "uuid", true, ""},
				{"level", "text", true, ""},
				{"capabilities", "ARRAY", false, "'{}'::text[]"},
				{"expires_at", "timestamp with time zone", true, ""},
				{"expiry_reminded_at", "timestamp with time zone", true, ""},
				{"expiry_reported_at", "timestamp with time zone", true, ""},
				{"reason", "text", false, ""},
				{"granted_by", "uuid", false, ""},
				{"granted_at", "timestamp with time zone", false, "now()"},
				{"revoked_at", "timestamp with time zone", true, ""},
				{"revoked_by", "uuid", true, ""},
				{"revoke_reason", "text", true, ""},
			},
			constraints: []string{
				"grants_bot_limits_check: CHECK (((subject_kind <> ALL (ARRAY['agent'::text, 'app'::text])) OR ((level <> 'trigger'::text) AND (capabilities <@ ARRAY['upload'::text]))))",
				"grants_capabilities_check: CHECK ((capabilities <@ ARRAY['approve_actions'::text, 'upload'::text, 'export'::text, 'manage'::text, 'terminal'::text]))",
				"grants_granted_by_fkey: FOREIGN KEY (granted_by) REFERENCES principals(id) ON DELETE RESTRICT",
				"grants_host_shape_check: CHECK (((target_kind <> 'host'::text) OR ((capabilities = ARRAY['terminal'::text]) AND (subject_kind = ANY (ARRAY['user'::text, 'group'::text])))))",
				"grants_level_check: CHECK (((level = ANY (ARRAY['read'::text, 'write'::text, 'trigger'::text])) AND ((target_kind = 'host'::text) = (level IS NULL))))",
				"grants_pkey: PRIMARY KEY (id)",
				"grants_revoked_by_fkey: FOREIGN KEY (revoked_by) REFERENCES principals(id) ON DELETE RESTRICT",
				"grants_subject_group_id_fkey: FOREIGN KEY (subject_group_id) REFERENCES groups(id) ON DELETE RESTRICT",
				"grants_subject_kind_check: CHECK ((subject_kind = ANY (ARRAY['user'::text, 'group'::text, 'agent'::text, 'app'::text])))",
				"grants_subject_principal_id_fkey: FOREIGN KEY (subject_principal_id) REFERENCES principals(id) ON DELETE RESTRICT",
				"grants_subject_shape_check: CHECK ((((subject_kind = 'group'::text) = ((subject_group_id IS NOT NULL) AND (subject_principal_id IS NULL))) AND ((subject_kind <> 'group'::text) = ((subject_principal_id IS NOT NULL) AND (subject_group_id IS NULL)))))",
				// The foreign key of the channel target comes with channels (00025, M1-53).
				"grants_target_channel_id_fkey: FOREIGN KEY (target_channel_id) REFERENCES channels(id) ON DELETE RESTRICT",
				"grants_target_kind_check: CHECK ((target_kind = ANY (ARRAY['instance'::text, 'space'::text, 'channel'::text, 'host'::text])))",
				"grants_target_shape_check: CHECK ((((target_space_id IS NOT NULL) = (target_kind = 'space'::text)) AND ((target_channel_id IS NOT NULL) = (target_kind = 'channel'::text)) AND ((target_host_id IS NOT NULL) = (target_kind = 'host'::text))))",
				// The foreign key of the space target comes with spaces (00023, M1-52).
				"grants_target_space_id_fkey: FOREIGN KEY (target_space_id) REFERENCES spaces(id) ON DELETE RESTRICT",
				"grants_terminal_only_on_host_check: CHECK (((target_kind = 'host'::text) OR (NOT ('terminal'::text = ANY (capabilities)))))",
			},
			indexes: []string{
				"CREATE INDEX grants_active_subject_group_idx ON public.grants USING btree (subject_group_id) WHERE (revoked_at IS NULL)",
				"CREATE INDEX grants_active_subject_principal_idx ON public.grants USING btree (subject_principal_id) WHERE (revoked_at IS NULL)",
				"CREATE INDEX grants_active_target_channel_idx ON public.grants USING btree (target_channel_id) WHERE (revoked_at IS NULL)",
				"CREATE INDEX grants_active_target_host_idx ON public.grants USING btree (target_host_id) WHERE (revoked_at IS NULL)",
				"CREATE INDEX grants_active_target_space_idx ON public.grants USING btree (target_space_id) WHERE (revoked_at IS NULL)",
				"CREATE INDEX grants_expiry_idx ON public.grants USING btree (expires_at) WHERE ((revoked_at IS NULL) AND (expires_at IS NOT NULL))",
				"CREATE UNIQUE INDEX grants_pkey ON public.grants USING btree (id)",
			},
		},
	}
	for _, tt := range tests {
		if got := principalColumns(t, conn, tt.table); !reflect.DeepEqual(got, tt.columns) {
			t.Errorf("%s columns:\n got %+v\nwant %+v", tt.table, got, tt.columns)
		}
		want := slices.Clone(tt.constraints)
		slices.Sort(want)
		if got := namedConstraints(t, conn, tt.table); !reflect.DeepEqual(got, want) {
			t.Errorf("%s constraints:\n got %q\nwant %q", tt.table, got, want)
		}
		want = slices.Clone(tt.indexes)
		slices.Sort(want)
		if got := indexes(t, conn, tt.table); !reflect.DeepEqual(got, want) {
			t.Errorf("%s indexes:\n got %q\nwant %q", tt.table, got, want)
		}
	}

	var triggers []string
	rows, err := conn.Query(context.Background(), `SELECT tgname || ' ' || pg_get_triggerdef(oid) FROM pg_trigger
		WHERE tgrelid = 'grants'::regclass AND NOT tgisinternal ORDER BY 1`)
	if err != nil {
		t.Fatal(err)
	}
	if triggers, err = pgx.CollectRows(rows, pgx.RowTo[string]); err != nil {
		t.Fatal(err)
	}
	wantTriggers := []string{"authz_grants_immutable CREATE TRIGGER authz_grants_immutable BEFORE UPDATE ON public.grants FOR EACH ROW EXECUTE FUNCTION authz_grants_immutable()"}
	if !reflect.DeepEqual(triggers, wantTriggers) {
		t.Errorf("triggers on grants:\n got %q\nwant %q", triggers, wantTriggers)
	}
}

// migrateProvider returns a goose provider over the embedded set for db, as bothy_owner.
func migrateProvider(t *testing.T, db *pgtest.DB) (*goose.Provider, func(string) bool) {
	t.Helper()
	pool := db.OpenPool(t, db.Config(pg.RoleOwner).ForMigrate(5*time.Second))
	sqlDB := pool.OpenSQL()
	t.Cleanup(func() { _ = sqlDB.Close() })
	provider, err := goose.NewProvider(goose.DialectPostgres, sqlDB, migrations.FS, goose.WithDisableGlobalRegistry(true))
	if err != nil {
		t.Fatal(err)
	}
	exists := func(rel string) bool {
		var ok bool
		if err := sqlDB.QueryRowContext(context.Background(), "SELECT to_regclass($1) IS NOT NULL", rel).Scan(&ok); err != nil {
			t.Fatal(err)
		}
		return ok
	}
	return provider, exists
}

// TestAuthzGroupsGrantsUpTwice: the migration applies after the identity migration (the
// template database is migrated with the whole set), and a second up changes nothing.
func TestAuthzGroupsGrantsUpTwice(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := pgtest.New(t)
	provider, exists := migrateProvider(t, db)
	version, err := provider.GetDBVersion(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if version < authzVersion {
		t.Fatalf("schema version %d, want at least %d", version, authzVersion)
	}
	results, err := provider.Up(ctx)
	if err != nil {
		t.Fatalf("second up: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("second up applied %d migrations, want none", len(results))
	}
	for _, rel := range []string{"groups", "group_memberships", "grants"} {
		if !exists(rel) {
			t.Errorf("table %s is missing", rel)
		}
	}
}

// TestAuthzGroupsGrantsDown holds the Down section: after a migration down to the identity
// version the three tables and the trigger function are gone, the identity tables stay, and up
// applies again.
func TestAuthzGroupsGrantsDown(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := pgtest.New(t)
	provider, exists := migrateProvider(t, db)
	if _, err := provider.DownTo(ctx, authzVersion-1); err != nil {
		t.Fatalf("down to %d: %v", authzVersion-1, err)
	}
	for _, rel := range []string{"groups", "group_memberships", "grants"} {
		if exists(rel) {
			t.Errorf("table %s survives the down migration", rel)
		}
	}
	if !exists("principals") {
		t.Error("the down migration removed principals")
	}
	conn := db.SuperConn(t)
	var functions int
	if err := conn.QueryRow(ctx, `SELECT count(*) FROM pg_proc p JOIN pg_namespace n ON n.oid = p.pronamespace
		WHERE n.nspname = 'public' AND p.proname LIKE 'authz_%'`).Scan(&functions); err != nil {
		t.Fatal(err)
	}
	if functions != 0 {
		t.Errorf("%d authz functions survive the down migration", functions)
	}
	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("up after down: %v", err)
	}
	if !exists("grants") {
		t.Error("grants missing after up again")
	}
}
