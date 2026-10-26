// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"slices"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/OWNER/bothy/services/server/internal/agents/adapters/postgres"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/personaldata"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
)

// pdFixture holds the people Alex and Sam, an agent whose credentials each of them holds, an
// agent of the organization and one session of Alex's agent.
type pdFixture struct {
	alex, sam, alexAgent ids.ID
}

func newPDFixture(t *testing.T, owner *pg.Pool) pdFixture {
	t.Helper()
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := owner.Exec(context.Background(), sql, args...); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
	}
	f := pdFixture{alex: ids.New(), sam: ids.New(), alexAgent: ids.New()}
	samAgent, orgAgent, space, channel, thread := ids.New(), ids.New(), ids.New(), ids.New(), ids.New()
	for handle, id := range map[string]ids.ID{"alex": f.alex, "sam": f.sam} {
		exec(`INSERT INTO principals (id, kind, handle, display_name) VALUES ($1, 'user', $2, $2)`, id, handle)
		exec(`INSERT INTO users (principal_id, email) VALUES ($1, $2)`, id, handle+"@example.com")
	}
	for _, a := range []struct {
		id     ids.ID
		handle string
		holder *ids.ID
	}{{f.alexAgent, "alex-helper", &f.alex}, {samAgent, "sam-helper", &f.sam}, {orgAgent, "team-helper", nil}} {
		exec(`INSERT INTO principals (id, kind, handle, display_name) VALUES ($1, 'agent', $2, $2)`, a.id, a.handle)
		mode := "organization"
		if a.holder != nil {
			mode = "personal"
		}
		exec(`INSERT INTO agents (principal_id, adapter_kind, billing_mode, credential_holder_id) VALUES ($1, 'custom', $2, $3)`, a.id, mode, a.holder)
	}
	exec(`INSERT INTO spaces (id, name, sort_key, created_by) VALUES ($1, 'Support', 'a0', $2)`, space, f.alex)
	exec(`INSERT INTO channels (id, space_id, kind, name, sort_key, history_backfill_limit, created_by)
		VALUES ($1, $2, 'text', 'general', 'a0', 20, $3)`, channel, space, f.alex)
	exec(`INSERT INTO threads (id, channel_id, kind, created_by) VALUES ($1, $2, 'main', $3)`, thread, channel, f.alex)
	exec(`INSERT INTO agent_sessions (id, thread_id, agent_id, external_session_ref) VALUES ($1, $2, $3, 'ref-1')`, ids.New(), thread, f.alexAgent)
	return f
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

// TestPersonalDataExport (M7-06, 20 section 5.2): access.json agents_held names the agent whose
// credentials Alex holds and nothing else; Sam's agent and the organization's agent are absent.
// A person without such an agent gets an empty array.
func TestPersonalDataExport(t *testing.T) {
	t.Parallel()
	db := pgtest.New(t)
	f := newPDFixture(t, db.Pool(t, pg.RoleOwner))
	p := postgres.NewPersonalData(db.Pool(t, pg.RoleApp))
	w := personaldata.NewMemoryWriter()
	if err := p.Export(context.Background(), personaldata.Export{Subject: f.alex}, w); err != nil {
		t.Fatal(err)
	}
	raw := w.Sections[personaldata.FileAccess]["agents_held"]
	validateSection(t, "access.agents_held.schema.json", raw)
	if want := `[{"agent_id":"` + f.alexAgent.String() + `"}]`; string(raw) != want {
		t.Errorf("agents_held = %s, want %s", raw, want)
	}
	if len(w.Sections) != 1 || len(w.Sections[personaldata.FileAccess]) != 1 || len(w.Lines) != 0 || len(w.Contents) != 0 {
		t.Errorf("export wrote more than agents_held: %v %v", w.Sections, w.Lines)
	}
	empty := personaldata.NewMemoryWriter()
	if err := p.Export(context.Background(), personaldata.Export{Subject: ids.New()}, empty); err != nil {
		t.Fatal(err)
	}
	if got := string(empty.Sections[personaldata.FileAccess]["agents_held"]); got != "[]" {
		t.Errorf("agents_held of a stranger = %s, want []", got)
	}
}

// TestPersonalDataErase (M7-06, 20 sections 2 and 4.4): agents stay ("stays"; the credential
// holder becomes pseudonymous with the principal) and agent sessions go with their thread, so no
// step changes a row of the module, in both content modes and on the repeated run after a crash.
func TestPersonalDataErase(t *testing.T) {
	t.Parallel()
	db := pgtest.New(t)
	owner := db.Pool(t, pg.RoleOwner)
	f := newPDFixture(t, owner)
	p := postgres.NewPersonalData(db.Pool(t, pg.RoleApp))
	before := pdSnapshot(t, owner, p.Tables())
	for _, mode := range []personaldata.ContentMode{personaldata.KeepPseudonymized, personaldata.DeleteContent} {
		for range 2 {
			for _, step := range personaldata.ParticipantSteps() {
				if err := p.Erase(context.Background(), step, personaldata.Erasure{Subject: f.alex, RequestID: ids.New(), Content: mode}); err != nil {
					t.Fatalf("%s %s: %v", mode, step, err)
				}
			}
		}
	}
	if after := pdSnapshot(t, owner, p.Tables()); !slices.Equal(after, before) {
		t.Errorf("erase changed the tables of agents:\nbefore %v\nafter  %v", before, after)
	}
	if want := []string{"agents", "agent_sessions"}; !slices.Equal(p.Tables(), want) || p.Module() != "agents" {
		t.Errorf("module %s, tables %v", p.Module(), p.Tables())
	}
}

// pdSnapshot returns every row of tables as canonical JSON text, per table in a stable order.
func pdSnapshot(t *testing.T, q *pg.Pool, tables []string) []string {
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
