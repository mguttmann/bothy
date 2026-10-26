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

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/personaldata"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
	"github.com/OWNER/bothy/services/server/internal/runs/adapters/postgres"
)

// pdWorld is the fixture of the participant tests: the people Alex and Sam, one agent, one thread
// with a message of each, a run triggered by each and an interaction of each on a component of
// Alex's message.
type pdWorld struct {
	alex, sam                      ids.ID
	alexRun, samRun                ids.ID
	alexInteraction, component     ids.ID
	alexMsg, thread, agent         ids.ID
	queued, started, finished, sub time.Time
}

func newPDWorld(t *testing.T, exec func(sql string, args ...any)) pdWorld {
	t.Helper()
	w := pdWorld{
		alex: ids.New(), sam: ids.New(), alexRun: ids.New(), samRun: ids.New(), alexInteraction: ids.New(),
		component: ids.New(), alexMsg: ids.New(), thread: ids.New(), agent: ids.New(),
		queued:   time.Date(2026, 10, 20, 9, 0, 0, 0, time.UTC),
		started:  time.Date(2026, 10, 20, 9, 0, 5, 0, time.UTC),
		finished: time.Date(2026, 10, 20, 9, 1, 0, 0, time.UTC),
		sub:      time.Date(2026, 10, 20, 9, 2, 0, 0, time.UTC),
	}
	space, channel, samMsg := ids.New(), ids.New(), ids.New()
	for handle, id := range map[string]ids.ID{"alex": w.alex, "sam": w.sam} {
		exec(`INSERT INTO principals (id, kind, handle, display_name) VALUES ($1, 'user', $2, $2)`, id, handle)
		exec(`INSERT INTO users (principal_id, email) VALUES ($1, $2)`, id, handle+"@example.com")
	}
	exec(`INSERT INTO principals (id, kind, handle, display_name) VALUES ($1, 'agent', 'helper', 'Helper')`, w.agent)
	exec(`INSERT INTO agents (principal_id, adapter_kind) VALUES ($1, 'custom')`, w.agent)
	exec(`INSERT INTO spaces (id, name, sort_key, created_by) VALUES ($1, 'Support', 'a0', $2)`, space, w.alex)
	exec(`INSERT INTO channels (id, space_id, kind, name, sort_key, history_backfill_limit, created_by)
		VALUES ($1, $2, 'text', 'general', 'a0', 20, $3)`, channel, space, w.alex)
	exec(`INSERT INTO threads (id, channel_id, kind, created_by) VALUES ($1, $2, 'main', $3)`, w.thread, channel, w.alex)
	for i, m := range []struct{ id, author ids.ID }{{w.alexMsg, w.alex}, {samMsg, w.sam}} {
		exec(`INSERT INTO messages (id, thread_id, channel_id, thread_seq, author_principal_id, client_msg_id,
			client_payload_sha256, origin, content_md) VALUES ($1, $2, $3, $4, $5, $1, '\x00', 'human', 'hello')`,
			m.id, w.thread, channel, i+1, m.author)
	}
	// Alex's run carries every column the export must leave out: agent texts, plan, summary and
	// a last retry with its error text.
	exec(`INSERT INTO runs (id, thread_id, agent_id, trigger_kind, triggering_message_id, triggered_by, state,
		billing_mode_expected, invocation_jti, status_line, last_text_excerpt, summary, plan, metrics,
		cost_micros, cost_is_estimate, queued_at, started_at, finished_at, state_reason)
		VALUES ($1, $2, $3, 'message', $4, $5, 'completed', 'organization', $6, 'secret status of Sam',
		'excerpt about Sam', 'summary about Sam', '{"steps": ["read the notes of Sam"]}',
		'{"tokens_in": 120, "tokens_out": 30, "context_pct": 12.5, "last_api_retry": {"attempt": 1, "error": "text of Sam"}}',
		4200, false, $7, $8, $9, 'done')`,
		w.alexRun, w.thread, w.agent, w.alexMsg, w.alex, ids.New(), w.queued, w.started, w.finished)
	exec(`INSERT INTO runs (id, thread_id, agent_id, trigger_kind, triggering_message_id, triggered_by, state,
		billing_mode_expected, invocation_jti, queued_at, started_at, finished_at)
		VALUES ($1, $2, $3, 'message', $4, $5, 'failed', 'organization', $6, $7, $7, $7)`,
		w.samRun, w.thread, w.agent, samMsg, w.sam, ids.New(), w.queued)
	exec(`INSERT INTO message_components (id, message_id, custom_id, type, definition, position)
		VALUES ($1, $2, 'pick', 'select', '{"options": ["yes", "no"]}', 0)`, w.component, w.alexMsg)
	exec(`INSERT INTO interactions (id, component_id, message_id, principal_id, client_interaction_id, "values", state, response, created_at)
		VALUES ($1, $2, $3, $4, $1, '{"pick": "yes"}', 'acknowledged', '{"text": "answer for Alex"}', $5)`,
		w.alexInteraction, w.component, w.alexMsg, w.alex, w.sub)
	samInteraction := ids.New()
	exec(`INSERT INTO interactions (id, component_id, message_id, principal_id, client_interaction_id, "values", created_at)
		VALUES ($1, $2, $3, $4, $1, '{"pick": "no"}', $5)`, samInteraction, w.component, w.alexMsg, w.sam, w.sub)
	exec(`INSERT INTO run_followups (run_id, message_id) VALUES ($1, $2)`, w.alexRun, samMsg)
	return w
}

// pdEnv is a test database with the participant and a fixture runner as bothy_owner.
func pdEnv(t *testing.T) (*postgres.PersonalData, func(sql string, args ...any), *pg.Pool) {
	t.Helper()
	db := pgtest.New(t)
	owner := db.Pool(t, pg.RoleOwner)
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := owner.Exec(context.Background(), sql, args...); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
	}
	return postgres.NewPersonalData(db.Pool(t, pg.RoleApp)), exec, owner
}

// validateLine validates raw against the line schema of the export package (docs/api/gdpr-export).
func validateLine(t *testing.T, name string, raw json.RawMessage) {
	t.Helper()
	path, err := filepath.Abs(filepath.Join("..", "..", "..", "..", "..", "..", "docs", "api", "gdpr-export", "v1", "lines", name))
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

// members returns the member names of the JSON object raw, sorted.
func members(t *testing.T, raw json.RawMessage) []string {
	t.Helper()
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	var out []string
	for k := range m {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}

// TestPersonalDataExport (M7-06, 20 section 5.2): runs.jsonl holds Alex's run with its state,
// times, costs and numeric metrics, never the agent texts, plan, summary or the last retry, and
// no run of Sam; interactions.jsonl holds Alex's interaction with the submitted values, never the
// response and never Sam's interaction. Both validate against their schemas.
func TestPersonalDataExport(t *testing.T) {
	t.Parallel()
	p, exec, _ := pdEnv(t)
	w := newPDWorld(t, exec)
	mw := personaldata.NewMemoryWriter()
	if err := p.Export(context.Background(), personaldata.Export{Subject: w.alex}, mw); err != nil {
		t.Fatal(err)
	}
	if len(mw.Sections) != 0 || len(mw.Contents) != 0 {
		t.Errorf("sections %v, contents %d; want none", mw.Sections, len(mw.Contents))
	}
	runs := mw.Lines[personaldata.FileRuns]
	if len(runs) != 1 {
		t.Fatalf("runs.jsonl has %d lines, want 1: %s", len(runs), runs)
	}
	validateLine(t, "runs.schema.json", runs[0])
	want := []string{"agent_id", "cost_is_estimate", "cost_micros", "currency", "finished_at", "id", "metrics", "queued_at", "started_at", "state", "thread_id", "trigger_kind"}
	if got := members(t, runs[0]); !slices.Equal(got, want) {
		t.Errorf("run members %v, want %v", got, want)
	}
	var run struct {
		ID             string         `json:"id"`
		ThreadID       string         `json:"thread_id"`
		AgentID        string         `json:"agent_id"`
		TriggerKind    string         `json:"trigger_kind"`
		State          string         `json:"state"`
		Currency       string         `json:"currency"`
		QueuedAt       time.Time      `json:"queued_at"`
		StartedAt      time.Time      `json:"started_at"`
		FinishedAt     time.Time      `json:"finished_at"`
		CostMicros     int64          `json:"cost_micros"`
		CostIsEstimate bool           `json:"cost_is_estimate"`
		Metrics        map[string]any `json:"metrics"`
	}
	if err := json.Unmarshal(runs[0], &run); err != nil {
		t.Fatal(err)
	}
	if run.ID != w.alexRun.String() || run.ThreadID != w.thread.String() || run.AgentID != w.agent.String() ||
		run.TriggerKind != "message" || run.State != "completed" || run.CostMicros != 4200 || run.CostIsEstimate ||
		run.Currency != "USD" || !run.QueuedAt.Equal(w.queued) || !run.StartedAt.Equal(w.started) || !run.FinishedAt.Equal(w.finished) {
		t.Errorf("run %+v", run)
	}
	if len(run.Metrics) != 3 || run.Metrics["tokens_in"] != float64(120) || run.Metrics["context_pct"] != 12.5 {
		t.Errorf("metrics %v, want tokens_in, tokens_out and context_pct only", run.Metrics)
	}
	inter := mw.Lines[personaldata.FileInteractions]
	if len(inter) != 1 {
		t.Fatalf("interactions.jsonl has %d lines, want 1: %s", len(inter), inter)
	}
	validateLine(t, "interactions.schema.json", inter[0])
	if got, want := members(t, inter[0]), []string{"component_id", "created_at", "id", "message_id", "state", "values"}; !slices.Equal(got, want) {
		t.Errorf("interaction members %v, want %v", got, want)
	}
	var in struct {
		ID          string            `json:"id"`
		ComponentID string            `json:"component_id"`
		MessageID   string            `json:"message_id"`
		State       string            `json:"state"`
		CreatedAt   time.Time         `json:"created_at"`
		Values      map[string]string `json:"values"`
	}
	if err := json.Unmarshal(inter[0], &in); err != nil {
		t.Fatal(err)
	}
	if in.ID != w.alexInteraction.String() || in.ComponentID != w.component.String() || in.MessageID != w.alexMsg.String() ||
		in.State != "acknowledged" || !in.CreatedAt.Equal(w.sub) || in.Values["pick"] != "yes" || len(in.Values) != 1 {
		t.Errorf("interaction %+v", in)
	}
	for _, f := range []personaldata.File{personaldata.FileRuns, personaldata.FileInteractions} {
		for _, l := range mw.Lines[f] {
			if strings.Contains(string(l), "Sam") || strings.Contains(string(l), w.sam.String()) || strings.Contains(string(l), w.samRun.String()) {
				t.Errorf("%s leaks data of Sam or agent texts: %s", f, l)
			}
		}
	}

	// A person without runs and interactions gets no line at all.
	empty := personaldata.NewMemoryWriter()
	if err := p.Export(context.Background(), personaldata.Export{Subject: ids.New()}, empty); err != nil {
		t.Fatal(err)
	}
	if len(empty.Lines) != 0 {
		t.Errorf("lines for a stranger: %v", empty.Lines)
	}
}

// TestPersonalDataErase (M7-06, 20 sections 2 and 4.4): runs, run steps, follow-ups, components
// and interactions go with their thread or message, so no step changes a row of the module, in
// both content modes and on the repeated run after a crash.
func TestPersonalDataErase(t *testing.T) {
	t.Parallel()
	p, exec, owner := pdEnv(t)
	w := newPDWorld(t, exec)
	ctx := context.Background()
	tables := p.Tables()
	before := snapshot(t, owner, tables)
	for _, mode := range []personaldata.ContentMode{personaldata.KeepPseudonymized, personaldata.DeleteContent} {
		for range 2 {
			for _, step := range personaldata.ParticipantSteps() {
				if err := p.Erase(ctx, step, personaldata.Erasure{Subject: w.alex, RequestID: ids.New(), Content: mode}); err != nil {
					t.Fatalf("%s %s: %v", mode, step, err)
				}
			}
		}
	}
	if after := snapshot(t, owner, tables); !slices.Equal(after, before) {
		t.Errorf("erase changed the tables of runs:\nbefore %v\nafter  %v", before, after)
	}
	if want := []string{"runs", "run_steps", "run_followups", "message_components", "interactions"}; !slices.Equal(tables, want) || p.Module() != "runs" {
		t.Errorf("module %s, tables %v", p.Module(), tables)
	}
}

// snapshot returns every row of tables as canonical JSON text, per table in a stable order.
func snapshot(t *testing.T, q *pg.Pool, tables []string) []string {
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
