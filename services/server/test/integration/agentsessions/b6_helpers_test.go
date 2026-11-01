// SPDX-License-Identifier: Apache-2.0

package agentsessions_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"path/filepath"
	"sync"
	"testing"

	"github.com/OWNER/bothy/services/server/api/specschema"
	"github.com/OWNER/bothy/services/server/internal/agents/adapters/agentstest"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
	gosync "github.com/OWNER/bothy/services/server/internal/sync"
	syncpg "github.com/OWNER/bothy/services/server/internal/sync/adapters/postgres"
)

// b6Post sends a text message as name into thread and returns the run it opened ("" for none).
func (w *world) b6Post(name string, thread ids.ID, text string) string {
	w.t.Helper()
	m := w.post(name, thread, text)
	if r, ok := w.runOf(m["id"].(string)); ok {
		return r.ID
	}
	return ""
}

// b6Started opens a run of Lotse in thread through a message of Alex, delivers it and starts it;
// extra are the members of the start body (for example the reference). It returns the run.
func (w *world) b6Started(thread ids.ID, extra ...string) string {
	w.t.Helper()
	run := w.trigger(thread)
	w.wantOK(w.start("lotse", run, w.deliver(run), extra...))
	return run
}

// b6Import stores an import row of agent with ref in thread (22 section 9 step 1) through the
// helper of agents and returns its id.
func (w *world) b6Import(thread, agent ids.ID, ref string, baseline int64) string {
	w.t.Helper()
	return agentstest.InsertImportedSession(w.t, w.db, agentstest.ImportedSession{
		ThreadID: thread, AgentID: agent, ExternalSessionRef: ref, CostBaselineMicros: baseline,
	}).String()
}

// b6Line is one system line of a thread.
type b6Line struct {
	ID, Author, Origin, Code, Content, TriggerState, Run string
	Silent                                               bool
}

// b6SystemLines returns the messages of kind system of thread, oldest first.
func (w *world) b6SystemLines(thread ids.ID) []b6Line {
	w.t.Helper()
	rs, err := w.db.Query(context.Background(), `SELECT id::text, author_principal_id::text, origin, coalesce(system_code, ''), content_md,
		coalesce(trigger_state, ''), coalesce(run_id::text, ''), silent FROM messages WHERE thread_id = $1 AND kind = 'system' ORDER BY created_at, id`,
		thread.String())
	must(w.t, err)
	defer rs.Close()
	var out []b6Line
	for rs.Next() {
		var l b6Line
		must(w.t, rs.Scan(&l.ID, &l.Author, &l.Origin, &l.Code, &l.Content, &l.TriggerState, &l.Run, &l.Silent))
		out = append(out, l)
	}
	return out
}

// b6BindAudit is one allow entry of agent_session.bind with its actor.
type b6BindAudit struct {
	Actor, Resource string
	Details         map[string]any
	Raw             string
}

// b6BindAudits returns the allow entries of agent_session.bind on thread, oldest first.
func (w *world) b6BindAudits(thread ids.ID) []b6BindAudit {
	w.t.Helper()
	rs, err := w.db.Query(context.Background(), `SELECT coalesce(actor_principal_id::text, ''), coalesce(resource_id::text, ''), details::text
		FROM audit_events WHERE action = 'agent_session.bind' AND decision = 'allow' AND resource_id = $1 ORDER BY seq`, thread.String())
	must(w.t, err)
	defer rs.Close()
	var out []b6BindAudit
	for rs.Next() {
		var a b6BindAudit
		must(w.t, rs.Scan(&a.Actor, &a.Resource, &a.Raw))
		must(w.t, json.Unmarshal([]byte(a.Raw), &a.Details))
		out = append(out, a)
	}
	return out
}

// b6Count counts the rows of agent_sessions of thread.
func (w *world) b6Count(thread ids.ID) int {
	return len(w.sessionRows(thread))
}

// b6Run reads the run id.
func (w *world) b6Run(id string) runRow {
	w.t.Helper()
	rows := w.runsWhere(`id = $1`, id)
	if len(rows) != 1 {
		w.t.Fatalf("run %s: %d rows", id, len(rows))
	}
	return rows[0]
}

// b6Pointer fails unless the problem p has a field error at pointer.
func (w *world) b6Pointer(p map[string]any, pointer string) {
	w.t.Helper()
	list, _ := p["errors"].([]any)
	for _, f := range list {
		if m, ok := f.(map[string]any); ok && m["pointer"] == pointer {
			return
		}
	}
	w.t.Fatalf("no field error at %s: %v", pointer, p)
}

// b6Concurrent sends the requests of fns at the same time behind a start barrier and returns
// their responses in the order of fns.
func b6Concurrent(fns ...func() *httpservertest.Response) []*httpservertest.Response {
	out := make([]*httpservertest.Response, len(fns))
	gate := make(chan struct{})
	var wg sync.WaitGroup
	for i, fn := range fns {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-gate
			out[i] = fn()
		}()
	}
	close(gate)
	wg.Wait()
	return out
}

// b6Statuses returns the status codes of rs.
func b6Statuses(rs []*httpservertest.Response) []int {
	out := make([]int, len(rs))
	for i, r := range rs {
		out[i] = r.StatusCode
	}
	return out
}

// b6Decode decodes the body of r.
func (w *world) b6Decode(r *httpservertest.Response) map[string]any {
	w.t.Helper()
	var m map[string]any
	if err := json.Unmarshal(r.Body, &m); err != nil {
		w.t.Fatalf("body: %v: %s", err, r.Body)
	}
	return m
}

// b6StoredEvents reads every stored event of the world's log through the repository of sync, as
// the gateway and the catch-up read them.
func (w *world) b6StoredEvents() []gosync.Event {
	w.t.Helper()
	list, err := syncpg.NewRepository(w.pool, slog.New(w.log)).EventsAfter(context.Background(), 0, 100000)
	must(w.t, err)
	return list
}

// b6Contract loads the contracts of the repository (openapi.yaml, asyncapi.yaml, actions.yaml).
func b6Contract(t *testing.T) *specschema.Contract {
	t.Helper()
	c, err := specschema.Load(filepath.Join("..", "..", "..", "..", ".."))
	must(t, err)
	return c
}

// b6ValidFrame checks env as a dispatch frame of the gateway against asyncapi.yaml and
// openapi.yaml (the strict event schema, the envelope rules, no null).
func b6ValidFrame(t *testing.T, c *specschema.Contract, env gosync.Envelope) {
	t.Helper()
	raw, err := json.Marshal(map[string]any{"op": "dispatch", "d": env})
	must(t, err)
	if testkit.HasJSONNull(raw) {
		t.Fatalf("null in %s", raw)
	}
	if err := c.ValidateServerFrame(raw); err != nil {
		t.Fatalf("%s: %v", raw, err)
	}
}

// b6ThreadState reads threads.state of thread.
func (w *world) b6ThreadState(thread ids.ID) string {
	return w.text(`SELECT state FROM threads WHERE id = $1`, thread.String())
}

// b6Close closes thread as name (closeThread).
func (w *world) b6Close(name string, thread ids.ID) {
	w.t.Helper()
	w.want(w.do(name, http.MethodPost, "/api/v1/threads/"+thread.String()+"/close", "", key()...), http.StatusOK, "")
}
