// SPDX-License-Identifier: Apache-2.0

package migrations_test

import (
	"context"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
)

// terminalVersion is the version of 00025_terminal_hosts.sql.
const terminalVersion = 25

// The people and hosts of the terminal migration tests (white label: Alex, Sam, Robin).
const (
	tmRobin = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7a01"
	tmAlex  = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7a02"
	tmSam   = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7a03"
	tmR1    = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7b01"
	tmR2    = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7b02"
	tmDev   = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7c01"
	tmSess  = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7c02"
	tmAgent = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7a04"
)

// tmPeople inserts Robin, Alex and Sam as people and an agent principal.
func tmPeople(t *testing.T, app *pg.Pool) {
	t.Helper()
	for _, p := range []struct{ id, handle string }{{tmRobin, "robin"}, {tmAlex, "alex"}, {tmSam, "sam"}} {
		mustExec(t, app, `INSERT INTO principals (id, kind, handle, display_name) VALUES ($1, 'user', $2, $2)`, p.id, p.handle)
		mustExec(t, app, `INSERT INTO users (principal_id) VALUES ($1)`, p.id)
	}
	mustExec(t, app, `INSERT INTO principals (id, kind, handle, display_name) VALUES ($1, 'agent', 'lotse', 'Lotse')`, tmAgent)
}

// tmActiveHost inserts an active host with keys, confirmed by Robin.
func tmActiveHost(t *testing.T, app *pg.Pool, id, name string) {
	t.Helper()
	mustExec(t, app, `INSERT INTO hosts (id, name, state, noise_public_key, auth_public_jwk, auth_jkt, key_fingerprint,
		confirmed_by, confirmed_at, created_by) VALUES ($1, $2, 'active', $3, '{"kty":"OKP","crv":"Ed25519","x":"AA"}', $4, 'FP', $5, now(), $5)`,
		id, name, make([]byte, 32), "jkt-"+id, tmRobin)
}

// TestMigration_Hosts holds hosts against 03 section 4.14 (M2-39): columns with types,
// nullability and defaults, every check, the partial uniques and the foreign keys with ON DELETE
// RESTRICT, and exercises the checks as bothy_app.
func TestMigration_Hosts(t *testing.T) {
	t.Parallel()
	db := pgtest.New(t)
	conn := db.SuperConn(t)
	ts := "timestamp with time zone"
	columns := []principalColumn{
		{"id", "uuid", false, ""},
		{"name", "text", false, ""},
		{"state", "text", false, "'pairing'::text"},
		{"platform", "text", true, ""},
		{"assigned_user_id", "uuid", true, ""},
		{"dev_environment_id", "uuid", true, ""},
		{"llm_credentials", "text", false, "'none'::text"},
		{"credential_holder_id", "uuid", true, ""},
		{"llm_credentials_declared_by", "uuid", true, ""},
		{"llm_credentials_declared_at", ts, true, ""},
		{"declared_server_host", "boolean", false, "false"},
		{"pairing_code_hash", "bytea", true, ""},
		{"pairing_expires_at", ts, true, ""},
		{"noise_public_key", "bytea", true, ""},
		{"auth_public_jwk", "jsonb", true, ""},
		{"auth_jkt", "text", true, ""},
		{"key_fingerprint", "text", true, ""},
		{"key_generation", "integer", false, "0"},
		{"fingerprint_failures", "integer", false, "0"},
		{"paired_at", ts, true, ""},
		{"confirmed_by", "uuid", true, ""},
		{"confirmed_at", ts, true, ""},
		{"reported_profiles", "jsonb", false, "'[]'::jsonb"},
		{"reported_session_users", "jsonb", false, "'[]'::jsonb"},
		{"reported_doctor", "jsonb", false, "'[]'::jsonb"},
		{"reported_is_server_host", "boolean", true, ""},
		{"reported_require_local_confirm", "boolean", true, ""},
		{"reported_os_version", "text", true, ""},
		{"reported_host_version", "text", true, ""},
		{"last_seen_at", ts, true, ""},
		{"created_by", "uuid", false, ""},
		{"created_at", ts, false, "now()"},
		{"disabled_at", ts, true, ""},
		{"disabled_reason", "text", true, ""},
		{"retired_at", ts, true, ""},
		{"version", "bigint", false, "1"},
	}
	if got := principalColumns(t, conn, "hosts"); !reflect.DeepEqual(got, columns) {
		t.Errorf("columns:\n got %+v\nwant %+v", got, columns)
	}
	constraints := []string{
		"hosts_assigned_user_id_fkey: FOREIGN KEY (assigned_user_id) REFERENCES users(principal_id) ON DELETE RESTRICT",
		"hosts_confirmed_by_fkey: FOREIGN KEY (confirmed_by) REFERENCES principals(id) ON DELETE RESTRICT",
		"hosts_confirmed_check: CHECK (((state <> ALL (ARRAY['active'::text, 'disabled'::text])) OR (confirmed_at IS NOT NULL)))",
		"hosts_created_by_fkey: FOREIGN KEY (created_by) REFERENCES principals(id) ON DELETE RESTRICT",
		"hosts_credential_holder_id_fkey: FOREIGN KEY (credential_holder_id) REFERENCES users(principal_id) ON DELETE RESTRICT",
		"hosts_disabled_check: CHECK (((state = 'disabled'::text) = (disabled_reason IS NOT NULL)))",
		"hosts_disabled_reason_check: CHECK ((disabled_reason = ANY (ARRAY['owner'::text, 'duplicate_connection'::text])))",
		"hosts_keys_check: CHECK (((state <> ALL (ARRAY['confirming'::text, 'active'::text, 'disabled'::text])) OR ((noise_public_key IS NOT NULL) AND (auth_public_jwk IS NOT NULL) AND (auth_jkt IS NOT NULL) AND (key_fingerprint IS NOT NULL))))",
		"hosts_llm_credentials_check: CHECK ((llm_credentials = ANY (ARRAY['none'::text, 'personal'::text, 'organization'::text])))",
		"hosts_llm_credentials_declared_by_fkey: FOREIGN KEY (llm_credentials_declared_by) REFERENCES principals(id) ON DELETE RESTRICT",
		"hosts_noise_public_key_length_check: CHECK ((octet_length(noise_public_key) = 32))",
		"hosts_pairing_hash_check: CHECK (((state = 'pairing'::text) = (pairing_code_hash IS NOT NULL)))",
		"hosts_personal_holder_check: CHECK (((llm_credentials = 'personal'::text) = (credential_holder_id IS NOT NULL)))",
		"hosts_pkey: PRIMARY KEY (id)",
		"hosts_platform_check: CHECK ((platform = ANY (ARRAY['macos'::text, 'linux'::text, 'windows'::text])))",
		"hosts_retired_check: CHECK (((state = 'retired'::text) = (retired_at IS NOT NULL)))",
		"hosts_state_check: CHECK ((state = ANY (ARRAY['pairing'::text, 'confirming'::text, 'active'::text, 'disabled'::text, 'retired'::text])))",
	}
	if got := namedConstraints(t, conn, "hosts"); !reflect.DeepEqual(got, constraints) {
		t.Errorf("constraints:\n got %q\nwant %q", got, constraints)
	}
	idx := []string{
		"CREATE INDEX hosts_assigned_user_active_idx ON public.hosts USING btree (assigned_user_id) WHERE (state = 'active'::text)",
		"CREATE INDEX hosts_credential_holder_personal_idx ON public.hosts USING btree (credential_holder_id) WHERE (llm_credentials = 'personal'::text)",
		"CREATE INDEX hosts_pairing_expires_idx ON public.hosts USING btree (pairing_expires_at) WHERE (state = 'pairing'::text)",
		"CREATE UNIQUE INDEX hosts_auth_jkt_key ON public.hosts USING btree (auth_jkt) WHERE (auth_jkt IS NOT NULL)",
		"CREATE UNIQUE INDEX hosts_name_live_key ON public.hosts USING btree (lower(name)) WHERE (state <> 'retired'::text)",
		"CREATE UNIQUE INDEX hosts_pairing_code_hash_key ON public.hosts USING btree (pairing_code_hash) WHERE (pairing_code_hash IS NOT NULL)",
		"CREATE UNIQUE INDEX hosts_pkey ON public.hosts USING btree (id)",
	}
	got := indexes(t, conn, "hosts")
	slices.Sort(idx)
	slices.Sort(got)
	if !reflect.DeepEqual(got, idx) {
		t.Errorf("indexes:\n got %q\nwant %q", got, idx)
	}

	app := db.Pool(t, pg.RoleApp)
	tmPeople(t, app)
	tmActiveHost(t, app, tmR1, "Build server")
	ins := `INSERT INTO hosts (id, name, state, pairing_code_hash, created_by) VALUES ($1, $2, $3, $4, $5)`
	host := func(n string) string { return "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7d" + n }
	// A pairing host needs its hash, and only a pairing host has one.
	wantCheckViolation(t, exec(app, ins, host("01"), "Lab", "pairing", nil, tmRobin), "pairing without a hash")
	mustExec(t, app, ins, host("02"), "Lab", "pairing", []byte{1}, tmRobin)
	wantCheckViolation(t, exec(app, `UPDATE hosts SET pairing_code_hash = $2 WHERE id = $1`, tmR1, []byte{2}), "active with a hash")
	wantState(t, exec(app, ins, host("03"), "Lab two", "pairing", []byte{1}, tmRobin), "23505", "pairing hash taken")
	// Keys in confirming, active and disabled; confirmed_at in active and disabled.
	wantCheckViolation(t, exec(app, ins, host("04"), "Lab three", "confirming", nil, tmRobin), "confirming without keys")
	wantCheckViolation(t, exec(app, `UPDATE hosts SET key_fingerprint = NULL WHERE id = $1`, tmR1), "active without a fingerprint")
	wantCheckViolation(t, exec(app, `UPDATE hosts SET confirmed_at = NULL WHERE id = $1`, tmR1), "active without confirmed_at")
	// retired_at exactly with retired, disabled_reason exactly with disabled.
	wantCheckViolation(t, exec(app, `UPDATE hosts SET state = 'retired' WHERE id = $1`, tmR1), "retired without retired_at")
	wantCheckViolation(t, exec(app, `UPDATE hosts SET retired_at = now() WHERE id = $1`, tmR1), "retired_at on an active host")
	wantCheckViolation(t, exec(app, `UPDATE hosts SET state = 'disabled' WHERE id = $1`, tmR1), "disabled without a reason")
	wantCheckViolation(t, exec(app, `UPDATE hosts SET disabled_reason = 'owner' WHERE id = $1`, tmR1), "reason on an active host")
	wantCheckViolation(t, exec(app, `UPDATE hosts SET state = 'disabled', disabled_reason = 'stolen' WHERE id = $1`, tmR1), "unknown reason")
	mustExec(t, app, `UPDATE hosts SET state = 'disabled', disabled_reason = 'duplicate_connection', disabled_at = now() WHERE id = $1`, tmR1)
	// personal exactly with a holder, the holder a person.
	wantCheckViolation(t, exec(app, `UPDATE hosts SET llm_credentials = 'personal' WHERE id = $1`, tmR1), "personal without holder")
	wantCheckViolation(t, exec(app, `UPDATE hosts SET credential_holder_id = $2 WHERE id = $1`, tmR1, tmAlex), "holder without personal")
	wantState(t, exec(app, `UPDATE hosts SET llm_credentials = 'personal', credential_holder_id = $2 WHERE id = $1`, tmR1, tmAgent),
		"23503", "an agent as credential holder")
	wantState(t, exec(app, `UPDATE hosts SET assigned_user_id = $2 WHERE id = $1`, tmR1, tmAgent), "23503", "an agent as holder")
	mustExec(t, app, `UPDATE hosts SET llm_credentials = 'personal', credential_holder_id = $2, assigned_user_id = $2 WHERE id = $1`, tmR1, tmAlex)
	// A 32-byte noise key only.
	wantCheckViolation(t, exec(app, `UPDATE hosts SET noise_public_key = $2 WHERE id = $1`, tmR1, make([]byte, 31)), "31-byte noise key")
	wantCheckViolation(t, exec(app, `UPDATE hosts SET platform = 'plan9' WHERE id = $1`, tmR1), "unknown platform")
	// The name is unique among live hosts without regard to case; a retired host frees it.
	wantState(t, exec(app, ins, host("05"), "build SERVER", "pairing", []byte{3}, tmRobin), "23505", "live name taken in another case")
	mustExec(t, app, `UPDATE hosts SET state = 'retired', retired_at = now(), disabled_reason = NULL WHERE id = $1`, tmR1)
	mustExec(t, app, ins, host("05"), "build SERVER", "pairing", []byte{3}, tmRobin)
	// Foreign keys: a holder with a host cannot be removed; bothy_app never deletes.
	owner := db.Pool(t, pg.RoleOwner)
	wantState(t, exec(owner, `DELETE FROM users WHERE principal_id = $1`, tmAlex), "23001", "delete the holder of a host")
	wantState(t, exec(app, `DELETE FROM hosts WHERE id = $1`, tmR1), "42501", "bothy_app deletes a host")
}

// TestMigration_HostSessionUsers holds host_session_users (M2-39, ruling T12, R-DM-39): a name is
// unique per host over all rows without regard to case, ended rows included; a person has at
// most one valid assignment per host; an ended row stays with its name; the same name on another
// host is fine; a host with rows cannot be deleted.
func TestMigration_HostSessionUsers(t *testing.T) {
	t.Parallel()
	db := pgtest.New(t)
	conn := db.SuperConn(t)
	constraints := []string{
		"host_session_users_created_by_fkey: FOREIGN KEY (created_by) REFERENCES principals(id) ON DELETE RESTRICT",
		"host_session_users_host_id_fkey: FOREIGN KEY (host_id) REFERENCES hosts(id) ON DELETE RESTRICT",
		"host_session_users_id_host_user_key: UNIQUE (id, host_id, user_id)",
		"host_session_users_os_user_check: CHECK ((os_user ~ '^[A-Za-z0-9_][A-Za-z0-9_.-]*$'::text))",
		"host_session_users_pkey: PRIMARY KEY (id)",
		"host_session_users_revoked_by_fkey: FOREIGN KEY (revoked_by) REFERENCES principals(id) ON DELETE RESTRICT",
		"host_session_users_revoked_check: CHECK (((revoked_at IS NULL) = (revoked_by IS NULL)))",
		"host_session_users_user_id_fkey: FOREIGN KEY (user_id) REFERENCES users(principal_id) ON DELETE RESTRICT",
	}
	if got := namedConstraints(t, conn, "host_session_users"); !reflect.DeepEqual(got, constraints) {
		t.Errorf("constraints:\n got %q\nwant %q", got, constraints)
	}
	idx := []string{
		"CREATE INDEX host_session_users_user_valid_idx ON public.host_session_users USING btree (user_id) WHERE (revoked_at IS NULL)",
		"CREATE UNIQUE INDEX host_session_users_host_os_user_key ON public.host_session_users USING btree (host_id, lower(os_user))",
		"CREATE UNIQUE INDEX host_session_users_host_user_valid_key ON public.host_session_users USING btree (host_id, user_id) WHERE (revoked_at IS NULL)",
		"CREATE UNIQUE INDEX host_session_users_id_host_user_key ON public.host_session_users USING btree (id, host_id, user_id)",
		"CREATE UNIQUE INDEX host_session_users_pkey ON public.host_session_users USING btree (id)",
	}
	got := indexes(t, conn, "host_session_users")
	slices.Sort(idx)
	slices.Sort(got)
	if !reflect.DeepEqual(got, idx) {
		t.Errorf("indexes:\n got %q\nwant %q", got, idx)
	}

	app := db.Pool(t, pg.RoleApp)
	tmPeople(t, app)
	tmActiveHost(t, app, tmR1, "Build server")
	tmActiveHost(t, app, tmR2, "Lab machine")
	row := func(n string) string { return "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7e" + n }
	ins := `INSERT INTO host_session_users (id, host_id, user_id, os_user, created_by) VALUES ($1, $2, $3, $4, $5)`
	mustExec(t, app, ins, row("01"), tmR2, tmSam, "bothy-s-sam", tmRobin)
	wantState(t, exec(app, ins, row("02"), tmR2, tmAlex, "BOTHY-S-SAM", tmRobin), "23505", "the name of Sam in another case")
	end := `UPDATE host_session_users SET revoked_at = now(), revoked_by = $2 WHERE id = $1`
	wantCheckViolation(t, exec(app, `UPDATE host_session_users SET revoked_at = now() WHERE id = $1`, row("01")), "revoked_at without revoked_by")
	mustExec(t, app, end, row("01"), tmRobin)
	wantState(t, exec(app, ins, row("03"), tmR2, tmAlex, "BOTHY-S-SAM", tmRobin), "23505", "the name of an ended row")
	mustExec(t, app, ins, row("04"), tmR2, tmSam, "bothy-s-sam2", tmRobin)
	wantState(t, exec(app, ins, row("05"), tmR2, tmSam, "bothy-s-sam3", tmRobin), "23505", "a second valid row of Sam")
	mustExec(t, app, end, row("04"), tmRobin)
	mustExec(t, app, ins, row("05"), tmR2, tmSam, "bothy-s-sam3", tmRobin)
	var ended int
	if err := app.QueryRow(context.Background(), `SELECT count(*) FROM host_session_users WHERE host_id = $1 AND revoked_at IS NOT NULL`, tmR2).
		Scan(&ended); err != nil || ended != 2 {
		t.Fatalf("ended rows %d, %v", ended, err)
	}
	mustExec(t, app, ins, row("06"), tmR1, tmAlex, "bothy-s-sam", tmRobin)
	for _, bad := range []string{"../x", "a/b", ".hidden", "", "with space"} {
		wantCheckViolation(t, exec(app, ins, row("07"), tmR1, tmSam, bad, tmRobin), "os_user "+bad)
	}
	wantState(t, exec(app, ins, row("08"), tmR1, tmAgent, "bothy-s-agent", tmRobin), "23503", "an agent as person")
	owner := db.Pool(t, pg.RoleOwner)
	wantState(t, exec(owner, `DELETE FROM hosts WHERE id = $1`, tmR2), "23001", "delete a host with session users")
	wantState(t, exec(app, `DELETE FROM host_session_users WHERE id = $1`, row("01")), "42501", "bothy_app deletes a row")
}

// TestMigration_TerminalSessions_RDM40 holds terminal_sessions (M2-39, R-DM-36, R-DM-40): the
// session user of a session belongs to its host and its person through the composite foreign
// key; the checks of state, close_reason, close_detail and detached_at; the time columns the
// limits of M2-46 rely on.
func TestMigration_TerminalSessions_RDM40(t *testing.T) {
	t.Parallel()
	db := pgtest.New(t)
	conn := db.SuperConn(t)
	ts := "timestamp with time zone"
	columns := []principalColumn{
		{"id", "uuid", false, ""},
		{"host_id", "uuid", false, ""},
		{"user_id", "uuid", false, ""},
		{"host_session_user_id", "uuid", false, ""},
		{"opened_by_device_id", "uuid", false, ""},
		{"device_id", "uuid", false, ""},
		{"auth_session_id", "uuid", false, ""},
		{"profile", "text", false, ""},
		{"state", "text", false, "'opening'::text"},
		{"token_jti", "uuid", false, ""},
		{"key_generation", "integer", false, ""},
		{"opened_at", ts, false, "now()"},
		{"started_at", ts, true, ""},
		{"detached_at", ts, true, ""},
		{"last_input_at", ts, true, ""},
		{"closed_at", ts, true, ""},
		{"closed_by", "uuid", true, ""},
		{"close_reason", "text", true, ""},
		{"close_detail", "text", true, ""},
		{"exit_code", "integer", true, ""},
		{"bytes_in", "bigint", false, "0"},
		{"bytes_out", "bigint", false, "0"},
		{"version", "bigint", false, "1"},
	}
	if got := principalColumns(t, conn, "terminal_sessions"); !reflect.DeepEqual(got, columns) {
		t.Errorf("columns:\n got %+v\nwant %+v", got, columns)
	}
	got := namedConstraints(t, conn, "terminal_sessions")
	for _, want := range []string{
		"terminal_sessions_session_user_fkey: FOREIGN KEY (host_session_user_id, host_id, user_id) REFERENCES host_session_users(id, host_id, user_id) ON DELETE RESTRICT",
		"terminal_sessions_token_jti_key: UNIQUE (token_jti)",
		"terminal_sessions_closed_check: CHECK (((state = 'closed'::text) = (closed_at IS NOT NULL)))",
		"terminal_sessions_reason_check: CHECK (((state = 'closed'::text) = (close_reason IS NOT NULL)))",
		"terminal_sessions_detail_check: CHECK (((close_detail IS NULL) OR (state = 'closed'::text)))",
		"terminal_sessions_detached_check: CHECK (((state = 'detached'::text) = (detached_at IS NOT NULL)))",
		"terminal_sessions_state_check: CHECK ((state = ANY (ARRAY['opening'::text, 'active'::text, 'detached'::text, 'closed'::text])))",
	} {
		if !slices.Contains(got, want) {
			t.Errorf("constraint missing: %s\n in %q", want, got)
		}
	}
	reasons := ""
	for _, c := range got {
		if strings.HasPrefix(c, "terminal_sessions_close_reason_check:") {
			reasons = c
		}
	}
	for _, r := range []string{"access_revoked", "host_disabled", "host_retired", "host_restarted", "idle_timeout", "max_duration", "detach_expired"} {
		if !strings.Contains(reasons, "'"+r+"'") {
			t.Errorf("close_reason check lacks %s: %s", r, reasons)
		}
	}
	idx := indexes(t, conn, "terminal_sessions")
	for _, want := range []string{
		"CREATE INDEX terminal_sessions_opened_at_brin ON public.terminal_sessions USING brin (opened_at)",
		"CREATE INDEX terminal_sessions_user_open_idx ON public.terminal_sessions USING btree (user_id) WHERE (state <> 'closed'::text)",
		"CREATE INDEX terminal_sessions_host_open_idx ON public.terminal_sessions USING btree (host_id) WHERE (state <> 'closed'::text)",
		"CREATE INDEX terminal_sessions_auth_session_open_idx ON public.terminal_sessions USING btree (auth_session_id) WHERE (state <> 'closed'::text)",
		"CREATE INDEX terminal_sessions_device_open_idx ON public.terminal_sessions USING btree (device_id) WHERE (state <> 'closed'::text)",
	} {
		if !slices.Contains(idx, want) {
			t.Errorf("index missing: %s", want)
		}
	}

	app := db.Pool(t, pg.RoleApp)
	tmPeople(t, app)
	tmActiveHost(t, app, tmR1, "Build server")
	tmActiveHost(t, app, tmR2, "Lab machine")
	mustExec(t, app, `INSERT INTO devices (id, user_id, name, platform) VALUES ($1, $2, 'Phone', 'ios')`, tmDev, tmAlex)
	mustExec(t, app, `INSERT INTO auth_sessions (id, user_id, device_id, auth_method, expires_at) VALUES ($1, $2, $3, 'oidc', $4)`,
		tmSess, tmAlex, tmDev, time.Now().Add(time.Hour))
	const alexOnR1 = "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7f01"
	mustExec(t, app, `INSERT INTO host_session_users (id, host_id, user_id, os_user, created_by) VALUES ($1, $2, $3, 'bothy-s-alex', $4)`,
		alexOnR1, tmR1, tmAlex, tmRobin)
	sid := func(n string) string { return "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7f" + n }
	ins := `INSERT INTO terminal_sessions (id, host_id, user_id, host_session_user_id, opened_by_device_id, device_id,
		auth_session_id, profile, token_jti, key_generation) VALUES ($1, $2, $3, $4, $5, $5, $6, 'shell', $7, 1)`
	mustExec(t, app, ins, sid("10"), tmR1, tmAlex, alexOnR1, tmDev, tmSess, sid("90"))
	wantState(t, exec(app, ins, sid("11"), tmR2, tmAlex, alexOnR1, tmDev, tmSess, sid("91")), "23503", "the session user of another host")
	wantState(t, exec(app, ins, sid("12"), tmR1, tmSam, alexOnR1, tmDev, tmSess, sid("92")), "23503", "the session user of another person")
	wantState(t, exec(app, ins, sid("13"), tmR1, tmAlex, alexOnR1, tmDev, tmSess, sid("90")), "23505", "token_jti taken")
	var opened time.Time
	var state string
	if err := app.QueryRow(context.Background(), `SELECT opened_at, state FROM terminal_sessions WHERE id = $1`, sid("10")).
		Scan(&opened, &state); err != nil || opened.IsZero() || state != "opening" {
		t.Fatalf("defaults %v %q, %v", opened, state, err)
	}
	upd := func(set string) error {
		return exec(app, `UPDATE terminal_sessions SET `+set+` WHERE id = $1`, sid("10"))
	}
	wantCheckViolation(t, upd(`state = 'parked'`), "unknown state")
	wantCheckViolation(t, upd(`state = 'detached'`), "detached without detached_at")
	wantCheckViolation(t, upd(`detached_at = now()`), "detached_at while opening")
	wantCheckViolation(t, upd(`state = 'closed', closed_at = now()`), "closed without a reason")
	wantCheckViolation(t, upd(`state = 'closed', close_reason = 'access_revoked'`), "closed without closed_at")
	wantCheckViolation(t, upd(`close_detail = 'pty_failed'`), "close_detail while open")
	wantCheckViolation(t, upd(`state = 'closed', closed_at = now(), close_reason = 'bored'`), "unknown close reason")
	mustExec(t, app, `UPDATE terminal_sessions SET state = 'active', last_input_at = now(), started_at = now() WHERE id = $1`, sid("10"))
	mustExec(t, app, `UPDATE terminal_sessions SET state = 'detached', detached_at = now() WHERE id = $1`, sid("10"))
	mustExec(t, app, `UPDATE terminal_sessions SET state = 'closed', closed_at = now(), close_reason = 'open_failed',
		close_detail = 'pty_failed', detached_at = NULL WHERE id = $1`, sid("10"))
	wantState(t, exec(app, `DELETE FROM terminal_sessions WHERE id = $1`, sid("10")), "42501", "bothy_app deletes a session")
}

// TestMigration_GrantsTargetHostFK holds the foreign key M1-27 deferred to M2-39: a host grant
// with a random target_host_id fails, with a real host it succeeds, and the host cannot be
// deleted while the grant exists.
func TestMigration_GrantsTargetHostFK(t *testing.T) {
	t.Parallel()
	db := pgtest.New(t)
	app := db.Pool(t, pg.RoleApp)
	tmPeople(t, app)
	tmActiveHost(t, app, tmR2, "Lab machine")
	ins := `INSERT INTO grants (id, subject_kind, subject_principal_id, target_kind, target_host_id, capabilities, reason, granted_by)
		VALUES ($1, 'user', $2, 'host', $3, '{terminal}', 'fixture', $4)`
	wantState(t, exec(app, ins, "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7a81", tmSam, "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7aff", tmRobin), "23503", "a random host")
	mustExec(t, app, ins, "0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f7a82", tmSam, tmR2, tmRobin)
	owner := db.Pool(t, pg.RoleOwner)
	wantState(t, exec(owner, `DELETE FROM hosts WHERE id = $1`, tmR2), "23001", "delete a host with a grant")
	var def string
	if err := db.SuperConn(t).QueryRow(context.Background(), `SELECT pg_get_constraintdef(oid) FROM pg_constraint
		WHERE conname = 'grants_target_host_id_fkey'`).Scan(&def); err != nil ||
		def != "FOREIGN KEY (target_host_id) REFERENCES hosts(id) ON DELETE RESTRICT" {
		t.Fatalf("grants_target_host_id_fkey = %q, %v", def, err)
	}
}

// TestMigration_TerminalHostsDown holds the Down section: down below version 25 removes the three
// tables and the foreign key of grants, up applies again.
func TestMigration_TerminalHostsDown(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := pgtest.New(t)
	provider, _ := migrateProvider(t, db)
	if _, err := provider.DownTo(ctx, terminalVersion-1); err != nil {
		t.Fatalf("down to %d: %v", terminalVersion-1, err)
	}
	conn := db.SuperConn(t)
	var left bool
	if err := conn.QueryRow(ctx, `SELECT to_regclass('hosts') IS NOT NULL OR to_regclass('host_session_users') IS NOT NULL
		OR to_regclass('terminal_sessions') IS NOT NULL OR EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'grants_target_host_id_fkey')`).
		Scan(&left); err != nil || left {
		t.Fatalf("terminal objects after down: %v, %v", left, err)
	}
	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("up after down: %v", err)
	}
}
