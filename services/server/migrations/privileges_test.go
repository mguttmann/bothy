// SPDX-License-Identifier: Apache-2.0

package migrations_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
	"github.com/OWNER/bothy/services/server/migrations"
)

// appPrivileges is the expectation list of docs/spec/16-betrieb.md 7.1 rule 3: per table in
// schema public exactly the rights of bothy_app that 03 and 16 provide. A migration that creates
// a table adds its row here.
var appPrivileges = map[string][]string{
	"goose_db_version": {"SELECT"},
	// instance: bothy migrate writes it as bothy_owner; the server reads it (M1-15), plus the
	// column right UPDATE (setup_completed_at) of bothy setup (M1-37), which has_table_privilege
	// does not show; TestEnrollmentCodesMigration_InstanceSetupGrant holds it.
	"instance": {"SELECT"},
	// sequence_counters: plus the column right UPDATE (value), which has_table_privilege does
	// not show; TestCounters_ValueNeverDecreases holds it.
	"sequence_counters": {"SELECT"},
	// 03 Grundsatz 7: principals are never hard deleted, owners are revoked, a users row lives
	// with its principal (M1-26).
	"principals": {"SELECT", "INSERT", "UPDATE"},
	"users":      {"SELECT", "INSERT", "UPDATE"},
	"owners":     {"SELECT", "INSERT", "UPDATE"},
	// 19 SEC-AUD-1: the audit log is append-only for bothy_app (M1-24).
	"audit_events": {"SELECT", "INSERT"},
	// plus the column right UPDATE (key_ciphertext, destroyed_at) for the destruction of a key,
	// which has_table_privilege does not show; TestSECT16_AppRoleCannotUpdateOrDelete holds it.
	"audit_ip_keys": {"SELECT", "INSERT"},
	// 03 audit_checkpoints (M1-50): rights as audit_events, plus the column right UPDATE
	// (exported_at) of the export, which has_table_privilege does not show;
	// TestSECT16_CheckpointsAppendOnly holds it.
	"audit_checkpoints": {"SELECT", "INSERT"},
	// 03 section 4.3, R-DM-09: groups are soft deleted, memberships removed by removed_at and
	// grants revoked by revoked_at; the rows are history (M1-27).
	"groups":            {"SELECT", "INSERT", "UPDATE"},
	"group_memberships": {"SELECT", "INSERT", "UPDATE"},
	"grants":            {"SELECT", "INSERT", "UPDATE"},
	// 03 event_log, R-DM-18: the envelope is append-only for bothy_app; redaction has the column
	// right UPDATE (payload, redacted_at) since M1-58, which has_table_privilege does not show
	// (TestEventLogRedactionGrant holds it), and partitions change only through the SECURITY
	// DEFINER functions (M1-16).
	"event_log": {"SELECT", "INSERT"},
	// 03 idempotency_keys (M1-20): the stage inserts, completes and replaces expired rows, the
	// cleanup job deletes them.
	"idempotency_keys": {"SELECT", "INSERT", "UPDATE", "DELETE"},
	// 03 section 4.2: devices are revoked by state, keys by revoked_at; the rows are security
	// history, deleted only by the retention job of class security, which grants it (M1-33).
	"devices":     {"SELECT", "INSERT", "UPDATE"},
	"device_keys": {"SELECT", "INSERT", "UPDATE"},
	// 03 event_consumer_cursors: a cursor row is never deleted in operation; restore finalize
	// moves it (M2-01, M7-04).
	"event_consumer_cursors": {"SELECT", "INSERT", "UPDATE"},
	// 03 server_signing_keys (M1-32): keys change by state and not_after; a retired row stays as
	// the record of a key that signed, deleted only by the retention job of class security.
	"server_signing_keys": {"SELECT", "INSERT", "UPDATE"},
	// 03 instance_settings (M1-46): a key gets a row on its first write and is changed with
	// version + 1; the writer never deletes a row, because that would lower the branding version.
	"instance_settings": {"SELECT", "INSERT", "UPDATE"},
	// 03 feature_flags (M1-47): a flag gets a row on its first write and is changed with
	// version + 1; a row is deleted only by the migration that removes the flag (ADR 0064).
	"feature_flags": {"SELECT", "INSERT", "UPDATE"},
	// 03 section 4.2: sessions and families are revoked by revoked_at, tokens marked by used_at;
	// the rows are security history, deleted only by the cleanup and retention jobs, which grant
	// their delete (M1-34).
	"auth_sessions":          {"SELECT", "INSERT", "UPDATE"},
	"refresh_token_families": {"SELECT", "INSERT", "UPDATE"},
	"refresh_tokens":         {"SELECT", "INSERT", "UPDATE"},
	// 03 auth_challenges (M1-35): transient single-use challenges; the job identity.auth_cleanup
	// deletes the expired rows.
	"auth_challenges": {"SELECT", "INSERT", "UPDATE", "DELETE"},
	// 03 section 4.2 (M1-36): a provider without identities may be deleted (08 section 3.3); an
	// identity is unlinked by unlinked_at and stays as history.
	"identity_providers": {"SELECT", "INSERT", "UPDATE", "DELETE"},
	"user_identities":    {"SELECT", "INSERT", "UPDATE"},
	// 03 api_tokens (M1-44): revocation and last_used_at are updates, the rows are security
	// history.
	"api_tokens": {"SELECT", "INSERT", "UPDATE"},
	// 03 enrollment_codes (M1-37): a code ends by used_at, expires_at or attempts; the rows are
	// security history, deleted only by the retention job, which grants its delete.
	"enrollment_codes": {"SELECT", "INSERT", "UPDATE"},
	// 03 device_pairings (M1-38): a pairing ends by its status; deleting old rows belongs to the
	// retention job.
	"device_pairings": {"SELECT", "INSERT", "UPDATE"},
	// 03 passkey_credentials (M1-40): a passkey ends by revoked_at; the rows are security history.
	"passkey_credentials": {"SELECT", "INSERT", "UPDATE"},
	// 03 auth_login_flows (M1-41): transient OIDC login flows; the job identity.auth_cleanup
	// deletes the expired ones.
	"auth_login_flows": {"SELECT", "INSERT", "UPDATE", "DELETE"},
	// 03 agents (M1-51): an agent principal is never hard deleted, its record lives with it.
	"agents": {"SELECT", "INSERT", "UPDATE"},
	// 03 agent_consumer_cursors (M2-07): the rows end with their agent through ON DELETE CASCADE.
	"agent_consumer_cursors": {"SELECT", "INSERT", "UPDATE"},
	// 03 spaces (M1-52): archiving is final and the data stays for retention and export, so a
	// space is never deleted; a sidebar row is personal data that erasure deletes (M7-06).
	"spaces":            {"SELECT", "INSERT", "UPDATE"},
	"user_space_states": {"SELECT", "INSERT", "UPDATE", "DELETE"},
	// 03 channels, forum_tags and threads (M1-53): archived_at and deleted_at, never a hard
	// delete (03 Grundsatz 7); the ordered skill list of a channel is replaced as a whole (M2-02).
	"channels":               {"SELECT", "INSERT", "UPDATE"},
	"channel_skill_bindings": {"SELECT", "INSERT", "UPDATE", "DELETE"},
	"forum_tags":             {"SELECT", "INSERT", "UPDATE"},
	"threads":                {"SELECT", "INSERT", "UPDATE"},
	// 03 project_bindings (M2-03): deleteProjectBinding removes the row; a binding has no
	// tombstone (06 section 7).
	"project_bindings": {"SELECT", "INSERT", "UPDATE", "DELETE"},
	// 03 thread_tags (M1-55): setThreadTags replaces the set and deleteForumTag drops the rows of
	// its tag; a row is never updated.
	"thread_tags": {"SELECT", "INSERT", "DELETE"},
	// 03 messages and mentions (M1-56): a deleted message stays as a tombstone (03 Grundsatz 7);
	// editing replaces the mentions of a message and deleting removes them.
	"messages": {"SELECT", "INSERT", "UPDATE"},
	"mentions": {"SELECT", "INSERT", "DELETE"},
	// M1-60: templates are deleted softly (deleted_at); no DELETE.
	"message_templates": {"SELECT", "INSERT", "UPDATE"},
	// 03 message_revisions (M1-58): an edit inserts the previous content, a deletion removes every
	// revision of the message (03 section 6.5).
	"message_revisions": {"SELECT", "INSERT", "DELETE"},
	// 03 reactions and pins (M1-59): a reaction or a pin is added or removed, never changed; a
	// deletion of the message removes both (03 messages).
	"reactions": {"SELECT", "INSERT", "DELETE"},
	"pins":      {"SELECT", "INSERT", "DELETE"},
	// M1-61: the personal sync tables of sync. A read state is never deleted by the use cases
	// (it goes with its person, thread or channel by ON DELETE CASCADE); an empty draft and a
	// reset setting are deleted.
	"read_states":   {"SELECT", "INSERT", "UPDATE"},
	"drafts":        {"SELECT", "INSERT", "UPDATE", "DELETE"},
	"user_settings": {"SELECT", "INSERT", "UPDATE", "DELETE"},
	// M3-18: the notification levels and quiet hours of a person.
	"notification_preferences": {"SELECT", "INSERT", "UPDATE", "DELETE"},
	"quiet_hours":              {"SELECT", "INSERT", "UPDATE", "DELETE"},
	// 03 section 4.7 (M3-03): the cleanup of M3-08 deletes sessions, parts and files.
	"files":                {"SELECT", "INSERT", "UPDATE", "DELETE"},
	"file_upload_sessions": {"SELECT", "INSERT", "UPDATE", "DELETE"},
	"file_upload_parts":    {"SELECT", "INSERT", "UPDATE", "DELETE"},
	// 03 file_thumbnails (M3-06): a variant row is inserted once and never changed, so its object
	// is never written again (12 4.4); the deletion of a file removes its rows (ON DELETE CASCADE).
	"file_thumbnails": {"SELECT", "INSERT", "DELETE"},
	// 03 section 4.14 (M2-39): hosts and terminal sessions are security history; a session user
	// row is never deleted while its host is not retired (ruling T12, R-DM-39). The retention
	// job of class security grants its own DELETE (M7).
	"hosts":              {"SELECT", "INSERT", "UPDATE"},
	"host_session_users": {"SELECT", "INSERT", "UPDATE"},
	"terminal_sessions":  {"SELECT", "INSERT", "UPDATE"},
	// 03 local_credentials (M1-39): one row per local identity, changed in place; it goes with
	// its identity (ON DELETE CASCADE).
	"local_credentials": {"SELECT", "INSERT", "UPDATE"},
	// 03 section 4.4 (M2-24): apps and installations are never deleted; an uninstall sets the
	// state, a reinstall adds an installation.
	"apps":              {"SELECT", "INSERT", "UPDATE"},
	"app_installations": {"SELECT", "INSERT", "UPDATE"},
	// 03 section 4.11 (M2-26): an owner deletes a subscription with its deliveries
	// (deleteWebhookSubscription); the retention job of class transient deletes finished
	// deliveries (M7-05).
	"webhook_subscriptions": {"SELECT", "INSERT", "UPDATE", "DELETE"},
	"webhook_deliveries":    {"SELECT", "INSERT", "UPDATE", "DELETE"},
	// 03 incoming_webhooks (M2-27): a revoke sets revoked_at; the retention job of class config
	// deletes revoked rows (M7-05).
	"incoming_webhooks": {"SELECT", "INSERT", "UPDATE", "DELETE"},
	// 03 backup_runs (M7-02): the console and the status page read the runs; only bothy_backup
	// writes them (TestMigrationBackupRuns holds its rights).
	"backup_runs": {"SELECT"},
	// 03 runs and run_steps (M2-05): a run ends with an end state, never with a delete; steps
	// are deleted when the server folds older finished steps into one (run.max_steps_persisted,
	// M2-10).
	"runs":      {"SELECT", "INSERT", "UPDATE"},
	"run_steps": {"SELECT", "INSERT", "UPDATE", "DELETE"},
	// 03 agent_sessions (M2-12): a session ends with ended_at and end_reason, never with a
	// delete; the rows are the session history of the thread.
	"agent_sessions": {"SELECT", "INSERT", "UPDATE"},
	// 03 message_components and interactions (M2-15): runs deletes the components of a deleted
	// message (its tombstone keeps the row, so the cascade never fires); interactions go only with
	// their component or message (cascade).
	"message_components": {"SELECT", "INSERT", "UPDATE", "DELETE"},
	"interactions":       {"SELECT", "INSERT", "UPDATE"},
}

// allPrivileges are the table rights PostgreSQL 18 knows (MAINTAIN since 17).
var allPrivileges = []string{"SELECT", "INSERT", "UPDATE", "DELETE", "TRUNCATE", "REFERENCES", "TRIGGER", "MAINTAIN"}

// TestPrivilegesAfterAllMigrations holds the grant rule of README.md for every migration, also
// the ones later tickets add: after all migrations every table in schema public belongs to
// bothy_owner, bothy_app may read it but holds none of TRUNCATE, REFERENCES and TRIGGER, PUBLIC
// holds nothing, and the version table of goose is read-only for bothy_app. bothy_app cannot
// create tables. A table whose migration forgot its grant fails here.
func TestPrivilegesAfterAllMigrations(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := pgtest.New(t, pgtest.WithMigrations(migrations.FS))
	super := db.SuperConn(t)

	rows, err := super.Query(ctx, `SELECT c.relname, pg_get_userbyid(c.relowner), c.relkind::text
		FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = 'public' AND c.relkind IN ('r', 'p', 'v', 'm', 'S', 'f') ORDER BY c.relname`)
	if err != nil {
		t.Fatal(err)
	}
	type relation struct{ name, owner, kind string }
	var relations []relation
	for rows.Next() {
		var r relation
		if err := rows.Scan(&r.name, &r.owner, &r.kind); err != nil {
			t.Fatal(err)
		}
		relations = append(relations, r)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}

	has := func(role, table, privilege string) bool {
		t.Helper()
		var ok bool
		if err := super.QueryRow(ctx, "SELECT has_table_privilege($1, quote_ident($2), $3)", role, table, privilege).Scan(&ok); err != nil {
			t.Fatalf("has_table_privilege(%s, %s, %s): %v", role, table, privilege, err)
		}
		return ok
	}
	sawVersionTable := false
	for _, r := range relations {
		if r.owner != string(pg.RoleOwner) {
			t.Errorf("%s belongs to %s, want %s", r.name, r.owner, pg.RoleOwner)
		}
		var public bool
		if err := super.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pg_class c, aclexplode(c.relacl) a
			WHERE c.oid = to_regclass(quote_ident($1)) AND a.grantee = 0)`, r.name).Scan(&public); err != nil {
			t.Fatal(err)
		}
		if public {
			t.Errorf("PUBLIC holds a right on %s", r.name)
		}
		if r.kind == "S" {
			continue // sequences: the rule covers tables
		}
		sawVersionTable = sawVersionTable || r.name == "goose_db_version"
		want, listed := appPrivileges[r.name]
		if !listed {
			t.Errorf("table %s has no row in appPrivileges; add the rights of bothy_app that 03 and 16 provide", r.name)
			continue
		}
		if !has("bothy_app", r.name, "SELECT") {
			t.Errorf("bothy_app cannot read %s: its migration lacks the grant", r.name)
		}
		for _, p := range allPrivileges {
			if got, expected := has("bothy_app", r.name, p), slices.Contains(want, p); got != expected {
				t.Errorf("bothy_app on %s: %s is %v, want %v", r.name, p, got, expected)
			}
		}
		for _, p := range []string{"TRUNCATE", "REFERENCES", "TRIGGER", "MAINTAIN"} {
			if slices.Contains(want, p) {
				t.Errorf("appPrivileges grants %s on %s; bothy_app never holds it", p, r.name)
			}
		}
	}
	if !sawVersionTable {
		t.Fatal("goose_db_version is missing from schema public; the test did not see the migrated schema")
	}

	app := db.Pool(t, pg.RoleApp)
	c, err := app.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Release()
	_, err = c.Exec(ctx, "CREATE TABLE app_may_not_create (id int)")
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "42501" {
		t.Errorf("CREATE TABLE as bothy_app: %v, want insufficient_privilege (42501)", err)
	}
}

func TestEmbeddedMigrationsValidate(t *testing.T) {
	t.Parallel()
	if err := pg.Validate(migrations.FS); err != nil {
		t.Fatal(err)
	}
}
