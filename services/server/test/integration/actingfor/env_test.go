// SPDX-License-Identifier: Apache-2.0

package actingfor_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"maps"
	"net/http"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
	"github.com/OWNER/bothy/services/server/test/harness"
)

// headerName is the request header of the acting-for binding (brief M2-14 design note 1).
const headerName = "Invocation-Token"

// consumer is the REST consumer of the agents of a world (M2-07); its first poll creates it at
// the head of the log, so every invocation of a run created later reaches it.
const consumer = "sr9"

// waitLong bounds every polling wait of the tests; the servers share a loaded machine.
const waitLong = 90 * time.Second

// pollEvery is the pause between two polls of a waiting loop.
const pollEvery = 200 * time.Millisecond

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

// seenResponse is one answer a client of the world received.
type seenResponse struct {
	method, path string
	status       int
	body         []byte
}

// responses keeps every answer the clients of a world received, so the token check reads the
// problem bodies of every request of the tests.
type responses struct {
	mu   sync.Mutex
	list []seenResponse
}

// observe records one answer; it is the harness.Client.Observer of every client of the world.
func (r *responses) observe(req harness.Request, resp harness.Response) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.list = append(r.list, seenResponse{method: req.Method, path: req.Path, status: resp.Status, body: slices.Clone(resp.Body)})
}

// all returns a copy of the recorded answers.
func (r *responses) all() []seenResponse {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.list)
}

// env is one harness server with the matrix world, the read grant of Lotse on the space
// Engineering (created by the owner through createGrant, brief M2-14 "Integration") and the REST
// consumers of Lotse and Texter.
type env struct {
	t    *testing.T
	s    *harness.Server
	w    *harness.World
	db   *pgxpool.Pool
	logs *syncBuffer
	seen *responses
	// owner is the first owner; it plays the owner role of Robin in the brief.
	owner, alex, lotse, texter *harness.Client
	// lotseRead is the grant read of Lotse on the space Engineering.
	lotseRead string
	problems  *httpserver.SchemaValidator
	plain     *http.Client
	// handled are the invocation tokens the tests handled; failure messages redact them.
	handled *tokenList
}

// tokenList is a goroutine-safe list of tokens.
type tokenList struct {
	mu   sync.Mutex
	list []string
}

// in returns a copy of e that reports to t, the test of a subtest; the server, the world and the
// recorders stay shared.
func (e *env) in(t *testing.T) *env {
	c := *e
	c.t = t
	return &c
}

// newEnv starts a server with raised rate limits, a short deny aggregation window and cfg on top
// of opts, builds the matrix world, gives Lotse read on Engineering and creates the consumers.
func newEnv(t *testing.T, opts harness.Options, cfg map[string]string) *env {
	t.Helper()
	opts.Config = envConfig(cfg)
	opts.EgressDenyAll = true
	logs := &syncBuffer{}
	opts.LogWriter = logs
	s := harness.Start(t, opts)
	return attachEnv(t, s, harness.BuildMatrixWorld(t, s), logs)
}

// envConfig is the configuration of a test server: raised rate limits, a short deny aggregation
// window, then cfg.
func envConfig(cfg map[string]string) map[string]string {
	conf := harness.RaisedRateLimits()
	conf["audit.deny_aggregate_window"] = "50ms"
	maps.Copy(conf, cfg)
	return conf
}

// attachEnv wraps a started server s with its world w: the observers, the grant of Lotse and the
// consumers.
func attachEnv(t *testing.T, s *harness.Server, w *harness.World, logs *syncBuffer) *env {
	t.Helper()
	pv, err := httpserver.NewSchemaValidator(rest.Spec, []string{"Problem"})
	if err != nil {
		t.Fatal(err)
	}
	e := &env{t: t, s: s, w: w, db: s.AdminDB(), logs: logs, seen: &responses{}, owner: w.Owner, alex: w.People["Alex"],
		lotse: w.Agents["Lotse"], texter: w.Agents["Texter"], problems: pv, plain: &http.Client{Timeout: time.Minute}, handled: &tokenList{}}
	for _, c := range append(slices.Collect(maps.Values(w.People)), append(slices.Collect(maps.Values(w.Agents)), w.Owner)...) {
		c.Observer = e.seen.observe
	}
	e.lotseRead = e.grant("agent", e.lotse.PrincipalID, "space", w.Spaces["Engineering"], "read")
	for _, a := range []*harness.Client{e.lotse, e.texter} {
		e.page(a, consumer)
	}
	return e
}

// remember adds tokens to the redaction list of failure messages.
func (e *env) remember(tokens ...string) {
	e.handled.mu.Lock()
	defer e.handled.mu.Unlock()
	e.handled.list = append(e.handled.list, tokens...)
}

// secrets returns the remembered tokens.
func (e *env) secrets() []string {
	e.handled.mu.Lock()
	defer e.handled.mu.Unlock()
	return slices.Clone(e.handled.list)
}

// redact returns b as text with every remembered token and its signature and payload segments
// replaced, so a failure message never repeats a token.
func (e *env) redact(b []byte) string {
	s := string(b)
	for _, tok := range e.secrets() {
		for _, part := range tokenParts(tok) {
			s = strings.ReplaceAll(s, part, "<redacted>")
		}
	}
	return s
}

// tokenParts are the strings of a compact token that must never be stored or logged: the token,
// its payload segment and its signature segment (brief M2-14 "TestSR9_TokenNeverLogged").
func tokenParts(tok string) []string {
	out := []string{tok}
	if parts := strings.Split(tok, "."); len(parts) == 3 {
		out = append(out, parts[1], parts[2])
	}
	return out
}

// invocation returns the header with the values of the Invocation-Token header; no value gives a
// header without it, an empty string an empty value.
func invocation(tokens ...string) http.Header {
	h := http.Header{}
	if len(tokens) > 0 {
		h[headerName] = slices.Clone(tokens)
	}
	return h
}

// do sends one request as c with the Invocation-Token values tokens, without a step-up retry.
func (e *env) do(c *harness.Client, method, path string, body any, tokens ...string) harness.Response {
	e.t.Helper()
	return c.Do(e.t.Context(), harness.Request{Method: method, Path: path, Body: body, Header: invocation(tokens...)})
}

// must sends one request as c with tokens and the step-up retry of the harness and fails unless
// the status is want.
func (e *env) must(c *harness.Client, method, path string, body any, want int, tokens ...string) harness.Response {
	e.t.Helper()
	r, err := harness.DoWithStepUp(e.t.Context(), c, harness.Request{Method: method, Path: path, Body: body, Header: invocation(tokens...)})
	if err != nil {
		e.t.Fatalf("%s %s: %v", method, path, err)
	}
	if r.Status != want {
		e.t.Fatalf("%s %s = %d %s, want %d", method, path, r.Status, e.redact(r.Body), want)
	}
	return r
}

// ok requires a 2xx answer with status want, a JSON body without null and returns it decoded.
func (e *env) ok(r harness.Response, want int, label string) map[string]any {
	e.t.Helper()
	if r.Status != want {
		e.t.Fatalf("%s = %d %s, want %d", label, r.Status, e.redact(r.Body), want)
	}
	var m map[string]any
	if err := json.Unmarshal(r.Body, &m); err != nil {
		e.t.Fatalf("%s: body is no JSON object: %v", label, err)
	}
	if p := nullPath(m, ""); p != "" {
		e.t.Fatalf("%s: JSON null at %s", label, p)
	}
	return m
}

// nullPath returns the path of the first JSON null in v, or "" (the contract has no null).
func nullPath(v any, at string) string {
	switch x := v.(type) {
	case nil:
		return at + "(null)"
	case map[string]any:
		for _, k := range slices.Sorted(maps.Keys(x)) {
			if p := nullPath(x[k], at+"."+k); p != "" {
				return p
			}
		}
	case []any:
		for i, el := range x {
			if p := nullPath(el, at+"["+itoa(i)+"]"); p != "" {
				return p
			}
		}
	}
	return ""
}

// itoa formats i in decimal.
func itoa(i int) string {
	b, _ := json.Marshal(i)
	return string(b)
}

// problem is the part of a Problem body the tests compare.
type problem struct {
	Type          string `json:"type"`
	Title         string `json:"title"`
	Status        int    `json:"status"`
	Code          string `json:"code"`
	Reason        string `json:"reason"`
	CorrelationID string `json:"correlation_id"`
	Instance      string `json:"instance"`
}

// wantProblem requires status, a body that validates against the schema Problem of the contract,
// the code and the reason (empty: no reason member).
func (e *env) wantProblem(r harness.Response, status int, code, reason, label string) {
	e.t.Helper()
	if r.Status != status {
		e.t.Fatalf("%s = %d %s, want %d %s", label, r.Status, e.redact(r.Body), status, code)
	}
	if err := e.problems.Validate("Problem", r.Body); err != nil {
		e.t.Fatalf("%s: the body is no Problem: %v: %s", label, err, e.redact(r.Body))
	}
	var p problem
	if err := json.Unmarshal(r.Body, &p); err != nil {
		e.t.Fatalf("%s: %v", label, err)
	}
	var raw map[string]any
	_ = json.Unmarshal(r.Body, &raw)
	if p.Code != code || p.Status != status || p.Reason != reason {
		e.t.Fatalf("%s = %d %s reason %q, want %d %s reason %q", label, p.Status, p.Code, p.Reason, status, code, reason)
	}
	if _, has := raw["reason"]; reason == "" && has {
		e.t.Fatalf("%s carries a reason: %s", label, e.redact(r.Body))
	}
	if p := nullPath(raw, ""); p != "" {
		e.t.Fatalf("%s: JSON null at %s", label, p)
	}
}

// volatileHeaders differ between two otherwise equal answers: time, correlation, the DPoP nonce
// and the rate-limit counters.
var volatileHeaders = []string{"Date", "Correlation-Id", "Dpop-Nonce", "Ratelimit", "Ratelimit-Policy", "Ratelimit-Remaining",
	"Ratelimit-Reset", "Ratelimit-Limit", "Traceparent", "Server-Timing"}

// sameAnswer requires that got answers exactly like want (status, body apart from correlation_id
// and instance, headers apart from volatileHeaders): an invisible target answers like an unknown
// ID (R-AZ-01, AZ-M44).
func (e *env) sameAnswer(got, want harness.Response, label string) {
	e.t.Helper()
	if got.Status != want.Status {
		e.t.Fatalf("%s: %d %s, but the unknown ID answers %d %s", label, got.Status, e.redact(got.Body), want.Status, e.redact(want.Body))
	}
	strip := func(b []byte) map[string]any {
		var m map[string]any
		if err := json.Unmarshal(b, &m); err != nil {
			e.t.Fatalf("%s: %v", label, err)
		}
		delete(m, "correlation_id")
		delete(m, "instance")
		return m
	}
	if g, w := strip(got.Body), strip(want.Body); !jsonEqual(g, w) {
		e.t.Fatalf("%s: body %v differs from the unknown ID's %v", label, g, w)
	}
	headers := func(h http.Header) map[string]string {
		out := map[string]string{}
		for k, v := range h {
			if !slices.Contains(volatileHeaders, k) {
				out[k] = strings.Join(v, ", ")
			}
		}
		return out
	}
	if g, w := headers(got.Header), headers(want.Header); !maps.Equal(g, w) {
		e.t.Fatalf("%s: headers %v differ from the unknown ID's %v", label, g, w)
	}
}

// jsonEqual compares two decoded JSON values.
func jsonEqual(a, b any) bool {
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	return bytes.Equal(x, y)
}

// grant creates a grant through createGrant as the owner and returns its ID.
func (e *env) grant(subjectKind, subject, targetKind, target, level string) string {
	e.t.Helper()
	r := e.must(e.owner, http.MethodPost, "/grants", map[string]any{"subject_kind": subjectKind, "subject_id": subject,
		"target_kind": targetKind, "target_id": target, "level": level, "reason": "acting-for test"}, http.StatusCreated)
	return str(e.ok(r, http.StatusCreated, "createGrant")["id"])
}

// revoke revokes a grant through revokeGrant as the owner.
func (e *env) revoke(grant string) {
	e.t.Helper()
	e.must(e.owner, http.MethodPost, "/grants/"+grant+"/revoke", map[string]any{"reason": "acting-for test"}, http.StatusOK)
}

// str returns v as a string, or "".
func str(v any) string {
	s, _ := v.(string)
	return s
}

// post writes text as c into thread (createMessage) and returns the message ID.
func (e *env) post(c *harness.Client, thread, text string) string {
	e.t.Helper()
	r := e.must(c, http.MethodPost, "/threads/"+thread+"/messages", map[string]any{"client_msg_id": ids.New().String(), "content_md": text},
		http.StatusCreated)
	return str(e.ok(r, http.StatusCreated, "createMessage")["id"])
}

// newThread opens a thread in the text channel key ("Space/Channel") as c: a message in the main
// thread and a thread on it with a first message (createThread); it returns the thread and the
// ID of its first message.
func (e *env) newThread(c *harness.Client, key, text string) (string, string) {
	e.t.Helper()
	ch := e.ok(e.must(e.owner, http.MethodGet, "/channels/"+e.w.Channels[key], nil, http.StatusOK), http.StatusOK, "getChannel")
	main := str(ch["channel"].(map[string]any)["main_thread_id"])
	parent := e.post(c, main, "Frage: "+text)
	r := e.must(c, http.MethodPost, "/channels/"+e.w.Channels[key]+"/threads", map[string]any{"title": text, "parent_message_id": parent,
		"first_message": map[string]any{"client_msg_id": ids.New().String(), "content_md": text}}, http.StatusCreated)
	var body struct {
		Thread struct {
			ID string `json:"id"`
		} `json:"thread"`
	}
	r.JSON(e.t, &body)
	first := e.text(`SELECT id::text FROM messages WHERE thread_id = $1 AND kind <> 'run' ORDER BY created_at, id LIMIT 1`, body.Thread.ID)
	return body.Thread.ID, first
}

// run is a run with the invocation token its agent got through the REST poll.
type run struct {
	id, thread, first, token string
}

// newRun opens a thread in the channel key as author, waits for the run of agent there and
// returns it with the token of its agent.invocation; the run stays queued.
func (e *env) newRun(author, agent *harness.Client, key, text string) run {
	e.t.Helper()
	thread, first := e.newThread(author, key, text)
	var id string
	e.eventually("the run of "+text, func() bool {
		id = e.textOr(``, `SELECT id::text FROM runs WHERE thread_id = $1 AND triggered_by = $2`, thread, author.PrincipalID)
		return id != ""
	})
	return run{id: id, thread: thread, first: first, token: e.invocationToken(agent, id)}
}

// startedRun is newRun followed by startRun with the delivered token.
func (e *env) startedRun(author, agent *harness.Client, key, text string) run {
	e.t.Helper()
	r := e.newRun(author, agent, key, text)
	e.must(agent, http.MethodPost, "/runs/"+r.id+"/start", startBody(r.token), http.StatusOK)
	return r
}

// startBody is the RunStart body with the token, billing mode organization and tool class chat.
func startBody(token string) map[string]any {
	return map[string]any{"invocation_token": token, "billing_mode_used": "organization", "tool_class_used": "chat"}
}

// envelope is the part of an EventEnvelope of the REST poll the tests read.
type envelope struct {
	ID              string `json:"id"`
	Seq             int64  `json:"seq"`
	Type            string `json:"type"`
	InvocationToken string `json:"invocation_token"`
	Resource        struct {
		Kind string `json:"kind"`
		ID   string `json:"id"`
	} `json:"resource"`
}

// eventPage is the body of pollAgentEvents.
type eventPage struct {
	Events     []envelope `json:"events"`
	NextCursor int64      `json:"next_cursor"`
	HeadSeq    int64      `json:"head_seq"`
}

// page polls one page of the consumer name of agent with tokens, acknowledges it and returns it.
func (e *env) page(agent *harness.Client, name string, tokens ...string) eventPage {
	e.t.Helper()
	r := e.do(agent, http.MethodGet, "/agents/me/consumers/"+name+"/events?limit=100", nil, tokens...)
	e.ok(r, http.StatusOK, "pollAgentEvents")
	var p eventPage
	r.JSON(e.t, &p)
	for _, ev := range p.Events {
		if ev.InvocationToken != "" {
			e.remember(ev.InvocationToken)
		}
	}
	if len(p.Events) > 0 {
		e.must(agent, http.MethodPost, "/agents/me/consumers/"+name+"/ack", map[string]any{"seq": p.NextCursor}, http.StatusOK, tokens...)
	}
	return p
}

// invocationToken polls the consumer of agent until the agent.invocation of the run arrives and
// returns its token.
func (e *env) invocationToken(agent *harness.Client, runID string) string {
	e.t.Helper()
	var tok string
	e.eventually("the agent.invocation of run "+runID, func() bool {
		for _, ev := range e.page(agent, consumer).Events {
			if ev.Type == "agent.invocation" && ev.Resource.ID == runID && ev.InvocationToken != "" {
				tok = ev.InvocationToken
			}
		}
		return tok != ""
	})
	return tok
}

// eventually polls cond until it holds or waitLong passed.
func (e *env) eventually(what string, cond func() bool) {
	e.t.Helper()
	deadline := time.Now().Add(waitLong)
	for !cond() {
		if time.Now().After(deadline) {
			e.t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(pollEvery)
	}
}

// text runs a query with one text column and one row.
func (e *env) text(query string, args ...any) string {
	e.t.Helper()
	var s string
	if err := e.db.QueryRow(context.Background(), query, args...).Scan(&s); err != nil {
		e.t.Fatalf("query %q: %v", query, err)
	}
	return s
}

// textOr is text with def for a query without a row.
func (e *env) textOr(def, query string, args ...any) string {
	e.t.Helper()
	var s string
	if err := e.db.QueryRow(context.Background(), query, args...).Scan(&s); err != nil {
		return def
	}
	return s
}

// texts runs a query with one text column.
func (e *env) texts(query string, args ...any) []string {
	e.t.Helper()
	rows, err := e.db.Query(context.Background(), query, args...)
	if err != nil {
		e.t.Fatalf("query %q: %v", query, err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			e.t.Fatal(err)
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		e.t.Fatal(err)
	}
	return out
}

// engineeringIDs returns every ID of the space Engineering as stored now: the space, its
// channels, their threads and messages, and the runs of those threads.
func (e *env) engineeringIDs() []string {
	e.t.Helper()
	return e.texts(`WITH ch AS (SELECT id FROM channels WHERE space_id = $1), th AS (SELECT id FROM threads WHERE channel_id IN (SELECT id FROM ch))
		SELECT $1::text UNION SELECT id::text FROM ch UNION SELECT id::text FROM th
		UNION SELECT id::text FROM messages WHERE channel_id IN (SELECT id FROM ch)
		UNION SELECT id::text FROM runs WHERE thread_id IN (SELECT id FROM th)`, e.w.Spaces["Engineering"])
}

// firstID returns the first of ids that body contains, or "".
func firstID(body []byte, list []string) string {
	for _, id := range list {
		if bytes.Contains(body, []byte(id)) {
			return id
		}
	}
	return ""
}

// denial is one deny entry of audit_events.
type denial struct {
	reason  string
	details map[string]any
	raw     string
}

// denials waits until at least one deny entry of action by actor with reason exists and returns
// every such entry (the aggregator writes a bucket after audit.deny_aggregate_window). resource
// "" matches an entry without resource: a denial of status invalid or run_inactive is decided
// before the path is resolved (brief design note 4b, 4c) and names none.
func (e *env) denials(action, resource, actor, reason string) []denial {
	e.t.Helper()
	var out []denial
	e.eventually("a deny entry "+action+" "+reason, func() bool {
		out = nil
		rows, err := e.db.Query(context.Background(), `SELECT coalesce(deny_reason, ''), coalesce(details::text, '{}'), row_to_json(a)::text
			FROM audit_events a WHERE action = $1 AND decision = 'deny' AND coalesce(resource_id::text, '') = $2 AND actor_principal_id::text = $3
			AND deny_reason = $4 ORDER BY occurred_at`, action, resource, actor, reason)
		if err != nil {
			e.t.Fatal(err)
		}
		defer rows.Close()
		for rows.Next() {
			var d denial
			var details string
			if err := rows.Scan(&d.reason, &details, &d.raw); err != nil {
				e.t.Fatal(err)
			}
			if err := json.Unmarshal([]byte(details), &d.details); err != nil {
				e.t.Fatal(err)
			}
			out = append(out, d)
		}
		return len(out) > 0
	})
	return out
}

// noTokenLeak fails when a token, its payload or its signature segment appears in the server log,
// a problem body any client received, or a row of audit_events (brief M2-14
// "TestSR9_TokenNeverLogged"). Spans and OTLP log records are checked where telemetry is on.
func (e *env) noTokenLeak(tokens []string) {
	e.t.Helper()
	if len(tokens) == 0 {
		e.t.Fatal("no token to check")
	}
	logs := e.logs.String()
	seen := e.seen.all()
	for i, tok := range tokens {
		for j, part := range tokenParts(tok) {
			where := "token " + itoa(i) + " part " + itoa(j)
			if strings.Contains(logs, part) {
				e.t.Fatalf("%s is in the server log", where)
			}
			for _, r := range seen {
				if r.status >= 400 && bytes.Contains(r.body, []byte(part)) {
					e.t.Fatalf("%s is in the problem body of %s %s", where, r.method, r.path)
				}
			}
			var n int
			if err := e.db.QueryRow(context.Background(), `SELECT count(*) FROM audit_events a WHERE strpos(row_to_json(a)::text, $1) > 0`, part).
				Scan(&n); err != nil {
				e.t.Fatal(err)
			}
			if n != 0 {
				e.t.Fatalf("%s is in %d rows of audit_events", where, n)
			}
		}
	}
}

// noNullInSuccess fails when a 2xx JSON body any client of the world received holds a JSON null
// (brief M2-14 "Contract": no JSON null in any success body).
func (e *env) noNullInSuccess() {
	e.t.Helper()
	n := 0
	for _, r := range e.seen.all() {
		if r.status < 200 || r.status > 299 || len(r.body) == 0 {
			continue
		}
		var v any
		if json.Unmarshal(r.body, &v) != nil {
			continue
		}
		n++
		if p := nullPath(v, ""); p != "" {
			e.t.Errorf("%s %s answered %d with a JSON null at %s", r.method, r.path, r.status, p)
		}
	}
	if n == 0 {
		e.t.Fatal("no success body was recorded; the check would be vacuous")
	}
}

// claims decodes the payload of a compact token without verifying it.
func claims(t *testing.T, tok string) map[string]any {
	t.Helper()
	parts := strings.Split(tok, ".")
	if len(parts) != 3 {
		t.Fatal("the token is no compact JWS")
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	return m
}
