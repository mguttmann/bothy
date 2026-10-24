// SPDX-License-Identifier: Apache-2.0

package reconcile_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	billingjobs "github.com/OWNER/bothy/services/server/internal/billing/adapters/jobs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
	"github.com/OWNER/bothy/services/server/internal/wire"
	"github.com/OWNER/bothy/services/server/test/harness"
)

// provider is the provider the runs of the world report, as the runtimes report it in
// cost_entries.provider (the example of CostReconcile in docs/api/openapi.yaml).
const provider = "anthropic"

// consumer is the REST consumer of the agents of a world; its first poll creates it at the head of
// the log, so every invocation of a run created later reaches it.
const consumer = "reconcile"

// syncBuffer is a goroutine-safe buffer for the JSON log lines of a server.
type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

// Write appends p; it implements io.Writer for harness.Options.LogWriter.
func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

// String returns every line written so far.
func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

// gate wraps the use case of billing.reconcile: it records every call and, while held, makes a job
// wait until release, so a test sees the state between the request and the job.
type gate struct {
	inner billingjobs.Applier
	mu    sync.Mutex
	held  chan struct{}
	calls [][]ids.ID
}

// ApplyReconciliation implements billingjobs.Applier.
func (g *gate) ApplyReconciliation(ctx context.Context, runIDs []ids.ID) error {
	g.mu.Lock()
	held := g.held
	g.mu.Unlock()
	if held != nil {
		select {
		case <-held:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	g.mu.Lock()
	g.calls = append(g.calls, append([]ids.ID(nil), runIDs...))
	g.mu.Unlock()
	return g.inner.ApplyReconciliation(ctx, runIDs)
}

// hold makes the following jobs wait until release.
func (g *gate) hold() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.held = make(chan struct{})
}

// release lets the waiting and the following jobs run.
func (g *gate) release() {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.held != nil {
		close(g.held)
		g.held = nil
	}
}

// callCount returns the number of applied jobs.
func (g *gate) callCount() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return len(g.calls)
}

// env is one harness server (role all, so River works billing.reconcile) with the matrix world:
// Lotse (A1) and Texter (A2) in mode organization, Assistent (A3) in mode personal with the owner
// as holder, Runner (A4) in mode organization; the owner plays the owner Alex of the brief.
type env struct {
	t    *testing.T
	s    *harness.Server
	w    *harness.World
	db   *pgxpool.Pool
	logs *syncBuffer
	gate *gate
	// owner is the first owner; alex and sam are people without owner rights.
	owner, alex, sam *harness.Client
	// lotse, texter, assistent and runner are the agents A1, A2, A3 and A4.
	lotse, texter, assistent, runner *harness.Client
	plain                            *http.Client
}

// newEnv starts a server with raised rate limits and a short deny aggregation window, builds the
// matrix world and creates the REST consumers of the four agents.
func newEnv(t *testing.T) *env {
	t.Helper()
	cfg := harness.RaisedRateLimits()
	cfg["audit.deny_aggregate_window"] = "50ms"
	logs := &syncBuffer{}
	g := &gate{}
	s := harness.Start(t, harness.Options{Config: cfg, EgressDenyAll: true, LogWriter: logs, JobModules: func(m *wire.JobModules) {
		g.inner = m.Reconciliation
		m.Reconciliation = g
	}})
	w := harness.BuildMatrixWorld(t, s)
	e := &env{t: t, s: s, w: w, db: s.AdminDB(), logs: logs, gate: g, owner: w.Owner, alex: w.People["Alex"], sam: w.People["Sam"],
		lotse: w.Agents["Lotse"], texter: w.Agents["Texter"], assistent: w.Agents["Assistent"], runner: w.Agents["Runner"],
		plain: &http.Client{Timeout: testkit.Patience, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
	for _, a := range []*harness.Client{e.lotse, e.texter, e.assistent, e.runner} {
		e.page(a)
	}
	return e
}

// in returns a copy of e that reports to t; the server and the world stay shared.
func (e *env) in(t *testing.T) *env {
	c := *e
	c.t = t
	return &c
}

// report is one cost report of a finish: the model, the adapter's amount, the currency and the
// token count of every unit.
type report struct {
	model    string
	amount   int64
	currency string
	tokens   int64
}

// usd is a report of model-large with amount micro units in USD.
func usd(amount int64) report {
	return report{model: "model-large", amount: amount, currency: "USD", tokens: 100}
}

// runSpec names who triggers a run of which agent where, the billing mode the agent reports and
// whether the run starts with an external session.
type runSpec struct {
	author, agent *harness.Client
	channel       string
	mode          string
	session       bool
}

// a1, a2, a3 and a4 are the runs of Lotse (Alex in Support/Allgemein), Texter (Sam in
// Marketing/Kampagnen), Assistent (the owner, its holder, in Research/Notizen) and Runner (the
// owner in Engineering/Werkzeuge).
func (e *env) a1() runSpec {
	return runSpec{author: e.alex, agent: e.lotse, channel: "Support/Allgemein", mode: "organization"}
}

func (e *env) a2() runSpec {
	return runSpec{author: e.sam, agent: e.texter, channel: "Marketing/Kampagnen", mode: "organization"}
}

func (e *env) a3() runSpec {
	return runSpec{author: e.owner, agent: e.assistent, channel: "Research/Notizen", mode: "personal"}
}

func (e *env) a4() runSpec {
	return runSpec{author: e.owner, agent: e.runner, channel: "Engineering/Werkzeuge", mode: "organization"}
}

// withSession is r starting with an external session reference.
func (r runSpec) withSession() runSpec {
	r.session = true
	return r
}

// run opens a thread in the channel of r as its author, waits for the run of its agent, starts it
// with the delivered token and finishes it with the reports (booked as estimates by M2-22); it
// returns the run ID.
func (e *env) run(r runSpec, reports ...report) string {
	e.t.Helper()
	thread := e.newThread(r.author, r.channel)
	var id string
	testkit.Until(e.t, "the run in "+r.channel, func() bool {
		id = e.textOr(`SELECT id::text FROM runs WHERE thread_id = $1 AND agent_id = $2`, thread, r.agent.PrincipalID)
		return id != ""
	})
	token := e.invocationToken(r.agent, id)
	start := map[string]any{"invocation_token": token, "billing_mode_used": r.mode, "tool_class_used": "chat"}
	if r.session {
		start["external_session_ref"] = "sess-" + ids.New().String()
	}
	e.must(r.agent, http.MethodPost, "/runs/"+id+"/start", start, http.StatusOK)
	costs := make([]any, len(reports))
	for i, rep := range reports {
		n := rep.tokens
		costs[i] = map[string]any{"provider": provider, "model": rep.model, "tokens_in": n, "tokens_out": n, "tokens_cache_read": n,
			"tokens_cache_write": n, "tokens_cache_write_1h": n, "amount_micros": rep.amount, "currency": rep.currency}
	}
	e.must(r.agent, http.MethodPost, "/runs/"+id+"/finish", map[string]any{"state": "completed", "costs": costs}, http.StatusOK)
	return id
}

// newThread opens a thread in the text channel key as c: a message in the main thread and a
// thread on it with a first message, which triggers the operator of the channel.
func (e *env) newThread(c *harness.Client, key string) string {
	e.t.Helper()
	var ch struct {
		Channel struct {
			MainThreadID string `json:"main_thread_id"`
		} `json:"channel"`
	}
	e.must(e.owner, http.MethodGet, "/channels/"+e.w.Channels[key], nil, http.StatusOK).JSON(e.t, &ch)
	var parent struct {
		ID string `json:"id"`
	}
	e.must(c, http.MethodPost, "/threads/"+ch.Channel.MainThreadID+"/messages",
		map[string]any{"client_msg_id": ids.New().String(), "content_md": "Question"}, http.StatusCreated).JSON(e.t, &parent)
	var th struct {
		Thread struct {
			ID string `json:"id"`
		} `json:"thread"`
	}
	e.must(c, http.MethodPost, "/channels/"+e.w.Channels[key]+"/threads", map[string]any{"title": "Costs", "parent_message_id": parent.ID,
		"first_message": map[string]any{"client_msg_id": ids.New().String(), "content_md": "Please help"}}, http.StatusCreated).JSON(e.t, &th)
	return th.Thread.ID
}

// envelope is the part of an EventEnvelope of the REST poll the tests read.
type envelope struct {
	Type            string `json:"type"`
	InvocationToken string `json:"invocation_token"`
	Resource        struct {
		ID string `json:"id"`
	} `json:"resource"`
}

// page polls one page of the consumer of agent, acknowledges it and returns its events.
func (e *env) page(agent *harness.Client) []envelope {
	e.t.Helper()
	var p struct {
		Events     []envelope `json:"events"`
		NextCursor int64      `json:"next_cursor"`
	}
	e.must(agent, http.MethodGet, "/agents/me/consumers/"+consumer+"/events?limit=100", nil, http.StatusOK).JSON(e.t, &p)
	if len(p.Events) > 0 {
		e.must(agent, http.MethodPost, "/agents/me/consumers/"+consumer+"/ack", map[string]any{"seq": p.NextCursor}, http.StatusOK)
	}
	return p.Events
}

// invocationToken polls the consumer of agent until the agent.invocation of the run arrives.
func (e *env) invocationToken(agent *harness.Client, runID string) string {
	e.t.Helper()
	var tok string
	testkit.Until(e.t, "the agent.invocation of run "+runID, func() bool {
		for _, ev := range e.page(agent) {
			if ev.Type == "agent.invocation" && ev.Resource.ID == runID && ev.InvocationToken != "" {
				tok = ev.InvocationToken
			}
		}
		return tok != ""
	})
	return tok
}

// must sends a request as c and fails unless the status is want; a 401 step_up_required makes c
// confirm and repeat (harness.MustDo).
func (e *env) must(c *harness.Client, method, path string, body any, want int) harness.Response {
	e.t.Helper()
	return harness.MustDo(e.t, c, harness.Request{Method: method, Path: path, Body: body}, want)
}

// do sends a request as c with the headers (name, value pairs) without a step-up retry.
func (e *env) do(c *harness.Client, method, path string, body any, headers ...string) harness.Response {
	e.t.Helper()
	h := http.Header{}
	for i := 0; i+1 < len(headers); i += 2 {
		h.Set(headers[i], headers[i+1])
	}
	return c.Do(e.t.Context(), harness.Request{Method: method, Path: path, Body: body, Header: h})
}

// bearer sends a request with an API token as Bearer, without DPoP (the reconciliation app).
func (e *env) bearer(token, method, path string, body any, headers ...string) harness.Response {
	e.t.Helper()
	var r io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			e.t.Fatal(err)
		}
		r = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(e.t.Context(), method, e.s.PublicURL+"/api/v1"+path, r)
	if err != nil {
		e.t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	resp, err := e.plain.Do(req)
	if err != nil {
		e.t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		e.t.Fatal(err)
	}
	return harness.Response{Status: resp.StatusCode, Header: resp.Header, Body: raw}
}

// manifest is the manifest of the reconciliation app in plugins/cost-reconciler.
func manifest(t *testing.T) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(repoRoot(t), "plugins", "cost-reconciler", "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

// repoRoot is the root of the repository, four levels above this file.
func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("no caller")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", ".."))
}

// installApp installs the reconciliation app with its manifest as the owner (step-up on demand),
// granting costs:reconcile, and returns its ID and a new app token (createToken, shown once).
func (e *env) installApp() (app, token string) {
	e.t.Helper()
	m := manifest(e.t)
	m["slug"] = "cost-reconciler-" + strconv.FormatInt(time.Now().UnixNano()%1_000_000_000, 36)
	var out struct {
		App struct {
			ID string `json:"id"`
		} `json:"app"`
		ID string `json:"id"`
	}
	e.must(e.owner, http.MethodPost, "/apps", map[string]any{"manifest": m, "granted_scopes": []string{"costs:reconcile"},
		"reason": "Reconcile the estimated costs"}, http.StatusCreated).JSON(e.t, &out)
	app = out.App.ID
	if app == "" {
		app = out.ID
	}
	return app, e.token(app)
}

// token creates an API token of the principal id as the owner and returns its secret.
func (e *env) token(id string) string {
	e.t.Helper()
	var tok struct {
		Secret string `json:"secret"`
	}
	e.must(e.owner, http.MethodPost, "/principals/"+id+"/tokens", map[string]any{"name": "Reconciler",
		"expires_at": e.s.Clock().Now().Add(24 * time.Hour).UTC().Format("2006-01-02T15:04:05.000Z")}, http.StatusCreated).JSON(e.t, &tok)
	return tok.Secret
}

// stamp is t in the Timestamp form of the API.
func stamp(t time.Time) string { return t.UTC().Format("2006-01-02T15:04:05.000Z") }

// entry is one row of cost_entries as the tests read it.
type entry struct {
	ID, Run, Agent, Thread, Channel, Space, Session, Mode, Provider, Model, Currency, Source, Supersedes string
	Tokens                                                                                               [5]int64
	Amount                                                                                               int64
	Estimate                                                                                             bool
	Occurred                                                                                             time.Time
	Reconciled                                                                                           *time.Time
}

// entries reads the cost entries of run in ascending id.
func (e *env) entries(run string) []entry {
	e.t.Helper()
	rows, err := e.db.Query(context.Background(), `SELECT id::text, run_id::text, agent_id::text, thread_id::text, channel_id::text,
		coalesce(space_id::text, ''), coalesce(agent_session_id::text, ''), billing_mode, provider, model, currency, source,
		coalesce(supersedes_id::text, ''), tokens_in, tokens_out, tokens_cache_read, tokens_cache_write, tokens_cache_write_1h,
		amount_micros, is_estimate, occurred_at, reconciled_at FROM cost_entries WHERE run_id = $1 ORDER BY id`, run)
	if err != nil {
		e.t.Fatal(err)
	}
	defer rows.Close()
	var out []entry
	for rows.Next() {
		var x entry
		if err := rows.Scan(&x.ID, &x.Run, &x.Agent, &x.Thread, &x.Channel, &x.Space, &x.Session, &x.Mode, &x.Provider, &x.Model, &x.Currency,
			&x.Source, &x.Supersedes, &x.Tokens[0], &x.Tokens[1], &x.Tokens[2], &x.Tokens[3], &x.Tokens[4], &x.Amount, &x.Estimate, &x.Occurred,
			&x.Reconciled); err != nil {
			e.t.Fatal(err)
		}
		out = append(out, x)
	}
	if err := rows.Err(); err != nil {
		e.t.Fatal(err)
	}
	return out
}

// valid returns the entries of es that no entry of es supersedes.
func valid(es []entry) []entry {
	superseded := map[string]bool{}
	for _, x := range es {
		if x.Supersedes != "" {
			superseded[x.Supersedes] = true
		}
	}
	var out []entry
	for _, x := range es {
		if !superseded[x.ID] {
			out = append(out, x)
		}
	}
	return out
}

// day is the period of a reconciliation of runs: from the start of the UTC day of their earliest
// entry to the end of the UTC day of their latest, so runs of one test that cross midnight still
// share one period.
func (e *env) day(runs ...string) (time.Time, time.Time) {
	e.t.Helper()
	var first, last time.Time
	for _, run := range runs {
		es := e.entries(run)
		if len(es) == 0 {
			e.t.Fatalf("run %s has no cost entry", run)
		}
		for _, x := range es {
			if first.IsZero() || x.Occurred.Before(first) {
				first = x.Occurred
			}
			if x.Occurred.After(last) {
				last = x.Occurred
			}
		}
	}
	return first.UTC().Truncate(24 * time.Hour), last.UTC().Truncate(24 * time.Hour).Add(24 * time.Hour)
}

// runCost is the cost columns and the version of a run.
type runCost struct {
	Micros   int64
	Currency string
	Estimate bool
	Version  int64
}

// runCostOf reads the cost columns of run.
func (e *env) runCostOf(run string) runCost {
	e.t.Helper()
	var c runCost
	if err := e.db.QueryRow(context.Background(), `SELECT cost_micros, currency, cost_is_estimate, version FROM runs WHERE id = $1`, run).
		Scan(&c.Micros, &c.Currency, &c.Estimate, &c.Version); err != nil {
		e.t.Fatal(err)
	}
	return c
}

// sessionCost is the cost columns and the version of an agent session.
type sessionCost struct {
	Micros, Baseline int64
	Estimate         bool
	Version          int64
}

// sessionOf reads the cost columns of the session of run.
func (e *env) sessionOf(run string) (string, sessionCost) {
	e.t.Helper()
	var id string
	var c sessionCost
	if err := e.db.QueryRow(context.Background(), `SELECT s.id::text, s.cumulative_cost_micros, s.cost_baseline_micros, s.cost_is_estimate,
		s.version FROM agent_sessions s JOIN runs r ON r.agent_session_id = s.id WHERE r.id = $1`, run).
		Scan(&id, &c.Micros, &c.Baseline, &c.Estimate, &c.Version); err != nil {
		e.t.Fatal(err)
	}
	return id, c
}

// count returns the integer result of query.
func (e *env) count(query string, args ...any) int64 {
	e.t.Helper()
	var n int64
	if err := e.db.QueryRow(context.Background(), query, args...).Scan(&n); err != nil {
		e.t.Fatalf("query %q: %v", query, err)
	}
	return n
}

// textOr runs a query with one text column; "" without a row.
func (e *env) textOr(query string, args ...any) string {
	var s string
	if err := e.db.QueryRow(context.Background(), query, args...).Scan(&s); err != nil {
		return ""
	}
	return s
}

// jobsOf returns the number of billing.reconcile jobs whose arguments name run.
func (e *env) jobsOf(run string) int64 {
	e.t.Helper()
	return e.count(`SELECT count(*) FROM jobs.river_job WHERE kind = $1 AND args->'run_ids' ? $2`, billingjobs.KindReconcile, run)
}

// settle waits until no billing.reconcile job is pending or running.
func (e *env) settle() {
	e.t.Helper()
	testkit.Until(e.t, "the billing.reconcile jobs", func() bool {
		return e.count(`SELECT count(*) FROM jobs.river_job WHERE kind = $1 AND state NOT IN ('completed', 'discarded', 'cancelled')`,
			billingjobs.KindReconcile) == 0
	})
}

// body is a CostReconcile body of the period with the entries (run, amount, currency triples).
func body(from, to time.Time, entries ...any) map[string]any {
	list := []any{}
	for i := 0; i+2 < len(entries); i += 3 {
		list = append(list, map[string]any{"run_id": entries[i], "amount_micros": entries[i+1], "currency": entries[i+2]})
	}
	return map[string]any{"provider": provider, "period_start": stamp(from), "period_end": stamp(to), "entries": list}
}

// result is the decoded CostReconcileResult.
type result struct {
	Reconciled int      `json:"reconciled"`
	Unmatched  []string `json:"unmatched_run_ids"`
}

// decode decodes a 200 body into result and fails on another status.
func (e *env) decode(r harness.Response) result {
	e.t.Helper()
	if r.Status != http.StatusOK {
		e.t.Fatalf("status %d %s", r.Status, r.Body)
	}
	var res result
	r.JSON(e.t, &res)
	return res
}

// problem is the part of a problem the tests read.
type problem struct {
	Code   string `json:"code"`
	Reason string `json:"reason"`
	Errors []struct {
		Pointer string `json:"pointer"`
		Code    string `json:"code"`
	} `json:"errors"`
}

// wantProblem checks status, code and reason of a problem answer.
func (e *env) wantProblem(r harness.Response, status int, code, reason string) problem {
	e.t.Helper()
	var p problem
	_ = json.Unmarshal(r.Body, &p)
	if r.Status != status || p.Code != code || p.Reason != reason {
		e.t.Fatalf("got %d %s/%s (%s), want %d %s/%s", r.Status, p.Code, p.Reason, r.Body, status, code, reason)
	}
	return p
}

// auditRow is one row of audit_events.
type auditRow struct {
	Decision, Reason, Actor string
	Details                 map[string]any
}

// audits returns the audit rows of action after seq, oldest first.
func (e *env) audits(action string, after int64) []auditRow {
	e.t.Helper()
	rows, err := e.db.Query(context.Background(), `SELECT decision, coalesce(deny_reason, ''), coalesce(actor_principal_id::text, ''), details::text
		FROM audit_events WHERE action = $1 AND seq > $2 ORDER BY seq`, action, after)
	if err != nil {
		e.t.Fatal(err)
	}
	defer rows.Close()
	var out []auditRow
	for rows.Next() {
		var a auditRow
		var details string
		if err := rows.Scan(&a.Decision, &a.Reason, &a.Actor, &details); err != nil {
			e.t.Fatal(err)
		}
		if err := json.Unmarshal([]byte(details), &a.Details); err != nil {
			e.t.Fatal(err)
		}
		out = append(out, a)
	}
	return out
}

// auditSeq is the highest seq of audit_events.
func (e *env) auditSeq() int64 {
	e.t.Helper()
	return e.count(`SELECT coalesce(max(seq), 0) FROM audit_events`)
}

// waitAudits waits until action has n rows of decision after seq (denials are aggregated per
// audit.deny_aggregate_window and flushed later) and returns them.
func (e *env) waitAudits(action, decision string, after int64, n int) []auditRow {
	e.t.Helper()
	var got []auditRow
	testkit.Until(e.t, strconv.Itoa(n)+" audit rows "+action+" "+decision, func() bool {
		got = got[:0]
		for _, a := range e.audits(action, after) {
			if a.Decision == decision {
				got = append(got, a)
			}
		}
		return len(got) >= n
	})
	return got
}

// detailKeys returns the attribute keys of an audit entry without the members of the recorder.
func detailKeys(d map[string]any) []string {
	var out []string
	for k := range maps.Keys(d) {
		if !strings.HasPrefix(k, "ip_") && k != "aggregate" && k != "schema" {
			out = append(out, k)
		}
	}
	return out
}
