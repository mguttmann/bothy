// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/personaldata"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
	"github.com/OWNER/bothy/services/server/internal/terminal/adapters/postgres"
)

// pdFixture holds Alex and Sam with a device and a login session each, Alex's host (assigned to
// Alex, created by Sam, with Alex's LLM login), Sam's host, a session user and a closed terminal
// session of each person, and a computer session of each person on its host, each in a run of the
// person.
type pdFixture struct {
	alex, sam, alexDev, alexHost, alexHSU, alexTS ids.ID
	agent, alexRun, alexCS                        ids.ID
	created, closed                               time.Time
}

func newPDFixture(t *testing.T, super *pgx.Conn) pdFixture {
	t.Helper()
	ctx := context.Background()
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := super.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
	}
	f := pdFixture{
		alex: ids.New(), sam: ids.New(), alexDev: ids.New(), alexHost: ids.New(), alexHSU: ids.New(), alexTS: ids.New(),
		created: time.Date(2026, 10, 1, 8, 0, 0, 0, time.UTC), closed: time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC),
	}
	samDev, samHost, samHSU := ids.New(), ids.New(), ids.New()
	sess := map[ids.ID]ids.ID{}
	for _, p := range []struct {
		id, dev ids.ID
		handle  string
	}{{f.alex, f.alexDev, "alex"}, {f.sam, samDev, "sam"}} {
		exec(`INSERT INTO principals (id, kind, status, handle, display_name) VALUES ($1, 'user', 'active', $2, $2)`, p.id, p.handle)
		exec(`INSERT INTO users (principal_id, email) VALUES ($1, $2)`, p.id, p.handle+"@example.com")
		exec(`INSERT INTO devices (id, user_id, name, platform, state) VALUES ($1, $2, 'Laptop', 'macos', 'active')`, p.dev, p.id)
		sess[p.id] = ids.New()
		exec(`INSERT INTO auth_sessions (id, user_id, device_id, auth_method, expires_at) VALUES ($1, $2, $3, 'passkey', $4)`,
			sess[p.id], p.id, p.dev, f.closed.Add(time.Hour))
	}
	exec(`INSERT INTO hosts (id, name, state, platform, assigned_user_id, llm_credentials, credential_holder_id, pairing_code_hash, created_by, created_at)
		VALUES ($1, 'alex-workstation', 'pairing', 'linux', $2, 'personal', $2, '\x01', $3, $4)`, f.alexHost, f.alex, f.sam, f.created)
	exec(`INSERT INTO hosts (id, name, state, llm_credentials, pairing_code_hash, created_by) VALUES ($1, 'sam-box', 'pairing', 'none', '\x02', $2)`, samHost, f.sam)
	exec(`INSERT INTO host_session_users (id, host_id, user_id, os_user, created_by, created_at) VALUES ($1, $2, $3, 'alex', $4, $5)`,
		f.alexHSU, f.alexHost, f.alex, f.sam, f.created)
	exec(`INSERT INTO host_session_users (id, host_id, user_id, os_user, created_by) VALUES ($1, $2, $3, 'sam', $3)`, samHSU, samHost, f.sam)
	exec(`INSERT INTO terminal_sessions (id, host_id, user_id, host_session_user_id, opened_by_device_id, device_id, auth_session_id,
		profile, state, token_jti, key_generation, opened_at, closed_at, close_reason, close_detail, bytes_in, bytes_out)
		VALUES ($1, $2, $3, $4, $5, $5, $6, 'shell', 'closed', $7, 1, $8, $9, 'client_closed', 'detail text', 10, 2000)`,
		f.alexTS, f.alexHost, f.alex, f.alexHSU, f.alexDev, sess[f.alex], ids.New(), f.created, f.closed)
	exec(`INSERT INTO terminal_sessions (id, host_id, user_id, host_session_user_id, opened_by_device_id, device_id, auth_session_id,
		profile, state, token_jti, key_generation) VALUES ($1, $2, $3, $4, $5, $5, $6, 'shell', 'active', $7, 1)`,
		ids.New(), samHost, f.sam, samHSU, samDev, sess[f.sam], ids.New())
	f.computerSessions(t, exec, samHost)
	return f
}

// computerSessions adds an agent, a run of each person and a computer session in each run: Alex's
// closed on Alex's host, Sam's open on Sam's host. Alex's row carries the columns the export
// leaves out (token ID, generations, close detail, stopped_by).
func (f *pdFixture) computerSessions(t *testing.T, exec func(string, ...any), samHost ids.ID) {
	t.Helper()
	f.agent, f.alexRun, f.alexCS = ids.New(), ids.New(), ids.New()
	space, channel, thread, samRun := ids.New(), ids.New(), ids.New(), ids.New()
	exec(`INSERT INTO principals (id, kind, handle, display_name) VALUES ($1, 'agent', 'runner', 'Runner')`, f.agent)
	exec(`INSERT INTO agents (principal_id, adapter_kind) VALUES ($1, 'custom')`, f.agent)
	exec(`INSERT INTO spaces (id, name, sort_key, created_by) VALUES ($1, 'Support', 'a0', $2)`, space, f.alex)
	exec(`INSERT INTO channels (id, space_id, kind, name, sort_key, history_backfill_limit, created_by)
		VALUES ($1, $2, 'text', 'general', 'a0', 20, $3)`, channel, space, f.alex)
	exec(`INSERT INTO threads (id, channel_id, kind, created_by) VALUES ($1, $2, 'main', $3)`, thread, channel, f.alex)
	for i, r := range []struct {
		run, by ids.ID
		state   string
	}{{f.alexRun, f.alex, "completed"}, {samRun, f.sam, "running"}} {
		msg := ids.New()
		exec(`INSERT INTO messages (id, thread_id, channel_id, thread_seq, author_principal_id, client_msg_id,
			client_payload_sha256, origin, content_md) VALUES ($1, $2, $3, $4, $5, $1, '\x00', 'human', 'hello')`,
			msg, thread, channel, i+1, r.by)
		exec(`INSERT INTO runs (id, thread_id, agent_id, trigger_kind, triggering_message_id, triggered_by, state,
			billing_mode_expected, invocation_jti, queued_at, started_at, finished_at)
			VALUES ($1, $2, $3, 'message', $4, $5, $6, 'organization', $7, $8, $8, CASE WHEN $6 = 'completed' THEN $9::timestamptz END)`,
			r.run, thread, f.agent, msg, r.by, r.state, ids.New(), f.created, f.closed)
	}
	exec(`INSERT INTO computer_sessions (id, host_id, user_id, agent_id, run_id, state, token_jti, generation, key_generation,
		opened_at, started_at, paused_ms, closed_at, stopped_by, close_reason, close_detail, action_count, front_app)
		VALUES ($1, $2, $3, $4, $5, 'closed', $6, 7, 1, $7, $7, 1500, $8, $3, 'stopped_by_user', 'detail text', 12, 'com.example.editor')`,
		f.alexCS, f.alexHost, f.alex, f.agent, f.alexRun, ids.New(), f.created, f.closed)
	exec(`INSERT INTO computer_sessions (id, host_id, user_id, agent_id, run_id, token_jti, generation, key_generation)
		VALUES ($1, $2, $3, $4, $5, $6, 1, 1)`, ids.New(), samHost, f.sam, f.agent, samRun, ids.New())
}

// validateSection validates raw against the section schema name of the export package.
func validateSection(t *testing.T, name string, raw json.RawMessage) {
	t.Helper()
	path, err := filepath.Abs(filepath.Join("..", "..", "..", "..", "..", "..", "docs", "api", "gdpr-export", "v1", "sections", name))
	if err != nil {
		t.Fatal(err)
	}
	c := jsonschema.NewCompiler()
	c.AssertFormat()
	sch, err := c.Compile(path)
	if err != nil {
		t.Fatalf("schema %s: %v", name, err)
	}
	inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	if err := sch.Validate(inst); err != nil {
		t.Errorf("%s: %s does not match: %v", name, raw, err)
	}
}

// TestPersonalDataExport (M7-06, 20 section 5.2): devices.json gets the sections hosts,
// host_session_users, terminal_sessions and computer_sessions (CU-04) with Alex's rows only, each
// with exactly the documented members: no key, hash, fingerprint, reported data, token ID,
// generation or close detail, and nothing of Sam.
func TestPersonalDataExport(t *testing.T) {
	t.Parallel()
	db := pgtest.New(t)
	f := newPDFixture(t, db.SuperConn(t))
	p := postgres.NewPersonalData(db.Pool(t, pg.RoleApp))
	w := personaldata.NewMemoryWriter()
	if err := p.Export(context.Background(), personaldata.Export{Subject: f.alex}, w); err != nil {
		t.Fatal(err)
	}
	sec := w.Sections[personaldata.FileDevices]
	if len(w.Sections) != 1 || len(sec) != 4 || len(w.Lines) != 0 {
		t.Fatalf("sections %v, lines %v; want the four sections of devices.json", w.Sections, w.Lines)
	}
	ts := func(tm time.Time) string { return tm.Format(time.RFC3339) }
	want := map[string]string{
		"hosts": `[{"id":"` + f.alexHost.String() + `","name":"alex-workstation","state":"pairing","platform":"linux",` +
			`"assignee":true,"creator":false,"credential_holder":true,"created_at":"` + ts(f.created) + `"}]`,
		"host_session_users": `[{"id":"` + f.alexHSU.String() + `","host_id":"` + f.alexHost.String() + `","os_user":"alex","created_at":"` + ts(f.created) + `"}]`,
		"terminal_sessions": `[{"id":"` + f.alexTS.String() + `","host_id":"` + f.alexHost.String() + `","device_id":"` + f.alexDev.String() +
			`","profile":"shell","state":"closed","opened_at":"` + ts(f.created) + `","closed_at":"` + ts(f.closed) +
			`","close_reason":"client_closed","bytes_in":10,"bytes_out":2000}]`,
		"computer_sessions": `[{"id":"` + f.alexCS.String() + `","host_id":"` + f.alexHost.String() + `","agent_id":"` + f.agent.String() +
			`","run_id":"` + f.alexRun.String() + `","state":"closed","opened_at":"` + ts(f.created) + `","started_at":"` + ts(f.created) +
			`","closed_at":"` + ts(f.closed) + `","close_reason":"stopped_by_user","paused_ms":1500,"action_count":12,` +
			`"front_app":"com.example.editor"}]`,
	}
	for key, w := range want {
		validateSection(t, "devices."+key+".schema.json", sec[key])
		if string(sec[key]) != w {
			t.Errorf("%s =\n%s\nwant\n%s", key, sec[key], w)
		}
		if strings.Contains(string(sec[key]), f.sam.String()) || strings.Contains(string(sec[key]), "sam") {
			t.Errorf("%s names Sam: %s", key, sec[key])
		}
	}

	// Sam created Alex's host but is neither its assignee nor its credential holder: Sam's export
	// holds Sam's own host only, never the name or times of Alex's host.
	samW := personaldata.NewMemoryWriter()
	if err := p.Export(context.Background(), personaldata.Export{Subject: f.sam}, samW); err != nil {
		t.Fatal(err)
	}
	if got := string(samW.Sections[personaldata.FileDevices]["hosts"]); strings.Contains(got, f.alexHost.String()) || strings.Contains(got, "alex") {
		t.Errorf("Sam's hosts name Alex's host: %s", got)
	}

	empty := personaldata.NewMemoryWriter()
	if err := p.Export(context.Background(), personaldata.Export{Subject: ids.New()}, empty); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"hosts", "host_session_users", "terminal_sessions", "computer_sessions"} {
		if got := string(empty.Sections[personaldata.FileDevices][key]); got != "[]" {
			t.Errorf("%s of a stranger = %s, want []", key, got)
		}
	}
}

// TestPersonalDataErase (M7-06, 20 sections 2 and 4.4): hosts, session users and terminal
// sessions stay with the pseudonymous ID until their retention, and the open sessions of the
// person were ended by the user.disable of step 1 (the subscribers of terminal), so no step changes
// a row of the module, in both content modes and on the repeated run after a crash.
func TestPersonalDataErase(t *testing.T) {
	t.Parallel()
	db := pgtest.New(t)
	super := db.SuperConn(t)
	f := newPDFixture(t, super)
	p := postgres.NewPersonalData(db.Pool(t, pg.RoleApp))
	before := pdSnapshot(t, super, p.Tables())
	for _, mode := range []personaldata.ContentMode{personaldata.KeepPseudonymized, personaldata.DeleteContent} {
		for range 2 {
			for _, step := range personaldata.ParticipantSteps() {
				if err := p.Erase(context.Background(), step, personaldata.Erasure{Subject: f.alex, RequestID: ids.New(), Content: mode}); err != nil {
					t.Fatalf("%s %s: %v", mode, step, err)
				}
			}
		}
	}
	if after := pdSnapshot(t, super, p.Tables()); !slices.Equal(after, before) {
		t.Errorf("erase changed the tables of terminal:\nbefore %v\nafter  %v", before, after)
	}
	if want := []string{"hosts", "host_session_users", "terminal_sessions", "computer_sessions"}; !slices.Equal(p.Tables(), want) || p.Module() != "terminal" {
		t.Errorf("module %s, tables %v", p.Module(), p.Tables())
	}
}

// pdSnapshot returns every row of tables as canonical JSON text, per table in a stable order.
func pdSnapshot(t *testing.T, q *pgx.Conn, tables []string) []string {
	t.Helper()
	out := make([]string, 0, len(tables))
	for _, table := range tables {
		var s string
		if err := q.QueryRow(context.Background(),
			"SELECT coalesce(jsonb_agg(to_jsonb(t) ORDER BY to_jsonb(t)::text), '[]')::text FROM "+table+" t").Scan(&s); err != nil {
			t.Fatal(err)
		}
		out = append(out, table+"="+s)
	}
	return out
}
