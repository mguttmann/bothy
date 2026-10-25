// SPDX-License-Identifier: Apache-2.0

package migrations_test

import (
	"context"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
)

// providersVersion is the version of 00013_identity_providers.sql.
const providersVersion = 13

// TestProvidersMigration_Schema holds identity_providers and user_identities against
// docs/spec/03-data-model.md section 4.2 column by column, with every check, foreign key, unique
// and index, and the two foreign keys the migration adds to earlier tables (M1-36).
func TestProvidersMigration_Schema(t *testing.T) {
	t.Parallel()
	conn := pgtest.New(t).SuperConn(t)
	ts := "timestamp with time zone"
	tests := []struct {
		table       string
		columns     []principalColumn
		constraints []string
		indexes     []string
	}{
		{
			table: "identity_providers",
			columns: []principalColumn{
				{"id", "uuid", false, ""},
				{"kind", "text", false, ""},
				{"name", "text", false, ""},
				{"config", "jsonb", false, ""},
				{"enabled", "boolean", false, "true"},
				{"allow_provisioning", "boolean", false, "false"},
				{"created_at", ts, false, "now()"},
				{"updated_at", ts, false, "now()"},
			},
			constraints: []string{
				"CHECK ((kind = ANY (ARRAY['oidc'::text, 'trusted_header'::text, 'local'::text])))",
				"PRIMARY KEY (id)",
			},
			indexes: []string{
				"CREATE UNIQUE INDEX identity_providers_name_lower_key ON public.identity_providers USING btree (lower(name))",
				"CREATE UNIQUE INDEX identity_providers_pkey ON public.identity_providers USING btree (id)",
				"CREATE UNIQUE INDEX identity_providers_single_kind_key ON public.identity_providers USING btree (kind) WHERE (kind = ANY (ARRAY['trusted_header'::text, 'local'::text]))",
			},
		},
		{
			table: "user_identities",
			columns: []principalColumn{
				{"id", "uuid", false, ""},
				{"user_id", "uuid", false, ""},
				{"provider_id", "uuid", false, ""},
				{"subject", "text", false, ""},
				{"issuer", "text", true, ""},
				{"email_at_link", "text", true, ""},
				{"linked_at", ts, false, "now()"},
				{"last_login_at", ts, true, ""},
				{"unlinked_at", ts, true, ""},
			},
			constraints: []string{
				"FOREIGN KEY (provider_id) REFERENCES identity_providers(id) ON DELETE RESTRICT",
				"FOREIGN KEY (user_id) REFERENCES users(principal_id) ON DELETE RESTRICT",
				"PRIMARY KEY (id)",
			},
			indexes: []string{
				"CREATE INDEX user_identities_user_id_idx ON public.user_identities USING btree (user_id)",
				"CREATE UNIQUE INDEX user_identities_active_subject_key ON public.user_identities USING btree (provider_id, subject) WHERE (unlinked_at IS NULL)",
				"CREATE UNIQUE INDEX user_identities_pkey ON public.user_identities USING btree (id)",
			},
		},
	}
	for _, tt := range tests {
		if got := principalColumns(t, conn, tt.table); !reflect.DeepEqual(got, tt.columns) {
			t.Errorf("%s columns:\n got %+v\nwant %+v", tt.table, got, tt.columns)
		}
		want := slices.Clone(tt.constraints)
		slices.Sort(want)
		got := principalConstraints(t, conn, tt.table)
		slices.Sort(got)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s constraints:\n got %q\nwant %q", tt.table, got, want)
		}
		want = slices.Clone(tt.indexes)
		slices.Sort(want)
		got = indexes(t, conn, tt.table)
		slices.Sort(got)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s indexes:\n got %q\nwant %q", tt.table, got, want)
		}
	}
	for table, fk := range map[string]string{
		"auth_sessions": "auth_sessions_identity_id_fkey: FOREIGN KEY (identity_id) REFERENCES user_identities(id) ON DELETE SET NULL",
		"groups":        "groups_provider_id_fkey: FOREIGN KEY (provider_id) REFERENCES identity_providers(id) ON DELETE RESTRICT",
	} {
		if !slices.Contains(namedConstraints(t, conn, table), fk) {
			t.Errorf("%s lacks %s", table, fk)
		}
	}
}

// TestProvidersMigration_Constraints exercises the rules of 03 as bothy_app: names unique without
// regard to case, one trusted_header and one local provider and any number of oidc, one active
// row per (provider_id, subject) while unlinked rows may repeat it, the restrict from identities
// and groups to their provider, and ON DELETE SET NULL from an identity to its sessions.
func TestProvidersMigration_Constraints(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := pgtest.New(t)
	app := db.Pool(t, pg.RoleApp)
	owner := db.Pool(t, pg.RoleOwner)
	id := func(n byte) string {
		return "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7c" + string("0123456789abcdef"[n>>4]) + string("0123456789abcdef"[n&15])
	}
	ins := `INSERT INTO identity_providers (id, kind, name, config) VALUES ($1, $2, $3, '{}')`
	prov := func(n byte, kind, name string) error { return exec(app, ins, id(n), kind, name) }
	for n, p := range []struct{ kind, name string }{{"oidc", "Company SSO"}, {"oidc", "Partner SSO"}, {"trusted_header", "Proxy"}, {"local", "Local"}} {
		if err := prov(byte(n+1), p.kind, p.name); err != nil {
			t.Fatalf("provider %s: %v", p.name, err)
		}
	}
	wantState(t, prov(10, "oidc", "company sso"), "23505", "name taken without regard to case")
	wantState(t, prov(11, "trusted_header", "Second proxy"), "23505", "second trusted_header")
	wantState(t, prov(12, "local", "Second local"), "23505", "second local")
	wantState(t, prov(13, "saml", "SAML"), "23514", "unknown kind")

	const person = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7d01"
	const dev = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7d02"
	const sess = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7d03"
	mustExec(t, app, `INSERT INTO principals (id, kind, handle, display_name) VALUES ($1, 'user', 'alex', 'Alex')`, person)
	mustExec(t, app, `INSERT INTO users (principal_id) VALUES ($1)`, person)
	link := `INSERT INTO user_identities (id, user_id, provider_id, subject, issuer) VALUES ($1, $2, $3, $4, $5)`
	if err := exec(app, link, id(30), person, id(1), "alex", "https://login.example.com"); err != nil {
		t.Fatal(err)
	}
	if err := exec(app, link, id(31), person, id(2), "alex", "https://partner.example.com"); err != nil {
		t.Fatalf("same subject at another provider: %v", err)
	}
	wantState(t, exec(app, link, id(32), person, id(1), "alex", "https://login.example.com"), "23505", "active subject twice")
	mustExec(t, app, `UPDATE user_identities SET unlinked_at = now() WHERE id = $1`, id(30))
	if err := exec(app, link, id(33), person, id(1), "alex", "https://login.example.com"); err != nil {
		t.Fatalf("relink after unlink: %v", err)
	}
	wantState(t, exec(app, link, id(34), person, id(9), "sam", nil), "23503", "unknown provider")

	mustExec(t, app, `INSERT INTO devices (id, user_id, name, platform) VALUES ($1, $2, 'Phone', 'ios')`, dev, person)
	mustExec(t, app, `INSERT INTO auth_sessions (id, user_id, device_id, identity_id, auth_method, expires_at) VALUES ($1, $2, $3, $4, 'oidc', $5)`,
		sess, person, dev, id(33), time.Now().Add(time.Hour))
	wantState(t, exec(app, `INSERT INTO auth_sessions (id, user_id, device_id, identity_id, auth_method, expires_at) VALUES ($1, $2, $3, $4, 'oidc', $5)`,
		id(40), person, dev, id(99), time.Now().Add(time.Hour)), "23503", "session with an unknown identity")

	mustExec(t, app, `INSERT INTO groups (id, name, source, provider_id, external_ref, created_by) VALUES ($1, 'Support', 'oidc_claim', $2, 'support', $3)`,
		id(50), id(2), person)
	wantState(t, exec(app, `DELETE FROM identity_providers WHERE id = $1`, id(1)), "23001", "delete a provider with identities")
	mustExec(t, owner, `DELETE FROM user_identities WHERE provider_id = $1`, id(2))
	wantState(t, exec(app, `DELETE FROM identity_providers WHERE id = $1`, id(2)), "23001", "delete a provider with claim groups")
	mustExec(t, app, `DELETE FROM identity_providers WHERE id = $1`, id(4))

	mustExec(t, owner, `DELETE FROM user_identities WHERE id = $1`, id(33))
	var identity *string
	if err := owner.QueryRow(ctx, `SELECT identity_id::text FROM auth_sessions WHERE id = $1`, sess).Scan(&identity); err != nil || identity != nil {
		t.Errorf("identity_id after deleting the identity = %v, %v (want NULL through ON DELETE SET NULL)", identity, err)
	}
	// The account deletion deletes the identities of the person (M7-06, migration 00079).
	mustExec(t, app, `DELETE FROM user_identities WHERE id = $1`, id(31))
}

// TestProvidersMigration_Down holds the Down section: down to version 12 removes both tables and
// the two foreign keys, up applies again.
func TestProvidersMigration_Down(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := pgtest.New(t)
	provider, exists := migrateProvider(t, db)
	if _, err := provider.DownTo(ctx, providersVersion-1); err != nil {
		t.Fatalf("down to %d: %v", providersVersion-1, err)
	}
	for _, rel := range []string{"identity_providers", "user_identities"} {
		if exists(rel) {
			t.Errorf("table %s survives the down migration", rel)
		}
	}
	if !exists("auth_sessions") || !exists("groups") {
		t.Error("the down migration removed an earlier table")
	}
	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("up after down: %v", err)
	}
	if !exists("user_identities") {
		t.Error("user_identities missing after up again")
	}
}
