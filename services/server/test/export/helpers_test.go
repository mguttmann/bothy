// SPDX-License-Identifier: Apache-2.0

package export_test

import (
	"bufio"
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/jose"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
	"github.com/OWNER/bothy/services/server/test/harness"
)

func TestMain(m *testing.M) { os.Exit(harness.Main(m)) }

// The fixed values of the export tests. Every wait for something that must happen is bounded by
// testkit.Patience (PL/8), never by a number of seconds of its own.
const (
	mediaNDJSON  = "application/x-ndjson"
	tsLayout     = "2006-01-02T15:04:05.000Z"
	streamWait   = testkit.Patience
	blockedWait  = testkit.Patience
	exportAction = "export.channel"
)

// steerClock is the steerable clock of a test server: the system clock plus an offset that only
// grows (Advance), or a pinned instant while a test posts a message at an exact created_at (Pin).
// A frozen clock.Fake would let the DPoP proofs of the harness clients (iat from the system clock)
// age past auth.dpop_proof_max_age while a test runs, so the offset stays small and real time
// keeps moving.
type steerClock struct {
	mu     sync.Mutex
	offset time.Duration
	pinned time.Time
}

// Now implements clock.Clock.
func (c *steerClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.pinned.IsZero() {
		return c.pinned
	}
	return time.Now().UTC().Add(c.offset)
}

// Pin makes Now return t until Unpin.
func (c *steerClock) Pin(t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.pinned = t.UTC()
}

// Unpin lets the clock run again.
func (c *steerClock) Unpin() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.pinned = time.Time{}
}

// Advance moves the running clock forward by d.
func (c *steerClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.offset += d
}

// safeBuffer collects the log lines of a server.
type safeBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *safeBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *safeBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

// env is one test server with its world.
type env struct {
	s     *harness.Server
	w     *harness.World
	db    *pgxpool.Pool
	logs  *safeBuffer
	clk   *steerClock
	plain *http.Client

	mu      sync.Mutex
	callers map[*harness.Client]*caller
}

// start runs a server in role all with the steerable clock, the matrix world and extra
// configuration on top of the raised rate limits.
func start(t *testing.T, extra map[string]string) *env {
	t.Helper()
	cfg := harness.RaisedRateLimits()
	for k, v := range extra {
		cfg[k] = v
	}
	logs := &safeBuffer{}
	clk := &steerClock{}
	s := harness.Start(t, harness.Options{Config: cfg, LogWriter: logs, Clock: clk})
	w := harness.BuildMatrixWorld(t, s)
	return &env{s: s, w: w, db: s.AdminDB(), logs: logs, clk: clk, plain: &http.Client{}, callers: map[*harness.Client]*caller{}}
}

// duration reads a duration key of the server.
func (e *env) duration(t *testing.T, key string) time.Duration {
	t.Helper()
	d, err := time.ParseDuration(e.s.ConfigValue(key))
	if err != nil {
		t.Fatalf("%s = %q: %v", key, e.s.ConfigValue(key), err)
	}
	return d
}

// ts formats t as a Timestamp of the contract (UTC, milliseconds).
func ts(t time.Time) string { return t.UTC().Format(tsLayout) }

// exportPath is the path of exportChannel below /api/v1 for channel and the range.
func exportPath(channel string, from, to time.Time) string {
	q := url.Values{}
	q.Set("from", ts(from))
	q.Set("to", ts(to))
	return "/channels/" + channel + "/export?" + q.Encode()
}

// caller sends raw requests as a harness client (DPoP-bound access or API token with the
// client's key) or with a bearer API token, so a test can read a streamed body line by line; the
// harness Client reads every body to its end.
type caller struct {
	c      *harness.Client
	bearer string
	key    *ecdsa.PrivateKey
	jwk    json.RawMessage

	mu    sync.Mutex
	nonce string
}

// callerOf returns the raw caller of c.
func (e *env) callerOf(t *testing.T, c *harness.Client) *caller {
	t.Helper()
	e.mu.Lock()
	defer e.mu.Unlock()
	if cl, ok := e.callers[c]; ok {
		return cl
	}
	var k struct {
		D string `json:"d"`
	}
	if err := json.Unmarshal(c.Key(), &k); err != nil {
		t.Fatal(err)
	}
	d, err := base64.RawURLEncoding.DecodeString(k.D)
	if err != nil {
		t.Fatal(err)
	}
	priv, err := ecdsa.ParseRawPrivateKey(elliptic.P256(), d)
	if err != nil {
		t.Fatal(err)
	}
	pub, err := jose.JWKFromPublicKey(&priv.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(pub)
	if err != nil {
		t.Fatal(err)
	}
	cl := &caller{c: c, key: priv, jwk: raw}
	e.callers[c] = cl
	return cl
}

// proof returns a DPoP proof (RFC 9449) for GET target with the newest nonce and ath of access.
func (cl *caller) proof(t *testing.T, method, target, access string) string {
	t.Helper()
	u, err := url.Parse(target)
	if err != nil {
		t.Fatal(err)
	}
	u.RawQuery, u.Fragment = "", ""
	cl.mu.Lock()
	nonce := cl.nonce
	cl.mu.Unlock()
	sum := sha256.Sum256([]byte(access))
	claims := map[string]any{"jti": ids.New().String(), "htm": method, "htu": u.String(), "iat": json.Number(strconv.FormatInt(time.Now().Unix(), 10)),
		"ath": base64.RawURLEncoding.EncodeToString(sum[:])}
	if nonce != "" {
		claims["nonce"] = nonce
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		t.Fatal(err)
	}
	s, err := jose.SignCompact(jose.ES256Signer{Key: cl.key}, jose.Header{Typ: "dpop+jwt", JWK: cl.jwk}, payload)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// open sends GET path (below /api/v1) and returns the request and the response with its body
// unread, after the DPoP nonce dance and one token renewal. The caller closes the body.
func (e *env) open(t *testing.T, ctx context.Context, cl *caller, path string) (*http.Request, *http.Response) {
	t.Helper()
	target := e.s.APIBase + path
	for attempt := 0; ; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
		if err != nil {
			t.Fatal(err)
		}
		if cl.bearer != "" {
			req.Header.Set("Authorization", "Bearer "+cl.bearer)
		} else {
			access, _ := cl.c.Tokens()
			req.Header.Set("Authorization", "DPoP "+access)
			req.Header.Set("DPoP", cl.proof(t, http.MethodGet, target, access))
		}
		resp, err := e.plain.Do(req)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		if n := resp.Header.Get("DPoP-Nonce"); n != "" {
			cl.mu.Lock()
			cl.nonce = n
			cl.mu.Unlock()
		}
		if resp.StatusCode != http.StatusUnauthorized {
			return req, resp
		}
		b, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			t.Fatal(err)
		}
		resp.Body = io.NopCloser(bytes.NewReader(b))
		code := problemCode(b)
		if attempt < 3 && code == "use_dpop_nonce" {
			continue
		}
		if attempt < 3 && code == "token_expired" && cl.c != nil {
			// The harness client renews its token pair on token_expired.
			cl.c.Do(ctx, harness.Request{Method: http.MethodGet, Path: "/me"})
			continue
		}
		return req, resp
	}
}

// result is a finished export response.
type result struct {
	req    *http.Request
	status int
	header http.Header
	body   []byte
	// err is the error of reading the body (io.ErrUnexpectedEOF for a cut stream).
	err error
}

// get sends GET path as cl and reads the whole body.
func (e *env) get(t *testing.T, cl *caller, path string) result {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), streamWait)
	defer cancel()
	req, resp := e.open(t, ctx, cl, path)
	defer func() { _ = resp.Body.Close() }()
	b, err := io.ReadAll(resp.Body)
	return result{req: req, status: resp.StatusCode, header: resp.Header, body: b, err: err}
}

// export performs a fresh step-up as c and exports channel for [from, to).
func (e *env) export(t *testing.T, c *harness.Client, channel string, from, to time.Time) result {
	t.Helper()
	stepUp(t, c)
	return e.get(t, e.callerOf(t, c), exportPath(channel, from, to))
}

// stepUp performs a fresh step-up as c.
func stepUp(t *testing.T, c *harness.Client) {
	t.Helper()
	if err := c.StepUp(t.Context()); err != nil {
		t.Fatal(err)
	}
}

// validate checks r against the contract of exportChannel (G8 of M1-71); an NDJSON body line by
// line against Message.
func validate(t *testing.T, r result) {
	t.Helper()
	rv, err := httpservertest.SharedResponseValidator(rest.Spec, []string{"exportChannel"})
	if err != nil {
		t.Fatal(err)
	}
	resp := &http.Response{StatusCode: r.status, Header: r.header, Request: r.req, Proto: "HTTP/1.1", ProtoMajor: 1, ProtoMinor: 1}
	if err := rv.Validate(r.req, &httpservertest.Response{Response: resp, Body: r.body}); err != nil {
		t.Errorf("contract of exportChannel %d: %v", r.status, err)
	}
}

// line is one decoded export line.
type line map[string]any

func (l line) str(k string) string {
	s, _ := l[k].(string)
	return s
}

func (l line) seq(t *testing.T) int64 {
	t.Helper()
	n, ok := l["thread_seq"].(json.Number)
	if !ok {
		t.Fatalf("thread_seq of %v", l)
	}
	v, err := n.Int64()
	if err != nil {
		t.Fatal(err)
	}
	return v
}

// lines checks a complete 200 export (status, media type, contract, one JSON object per line
// decoded with UseNumber, no null anywhere, no raw newline inside a line) and returns the lines.
func lines(t *testing.T, r result) []line {
	t.Helper()
	if r.status != http.StatusOK || r.err != nil {
		t.Fatalf("export: %d %v %.300s", r.status, r.err, r.body)
	}
	if ct := r.header.Get("Content-Type"); ct != mediaNDJSON {
		t.Errorf("Content-Type = %q", ct)
	}
	validate(t, r)
	return decodeLines(t, r.body)
}

// decodeLines decodes complete NDJSON lines; body must be empty or end with a newline.
func decodeLines(t *testing.T, body []byte) []line {
	t.Helper()
	if len(body) == 0 {
		return nil
	}
	if body[len(body)-1] != '\n' {
		t.Fatalf("body does not end with a newline: %.200q", body[max(0, len(body)-200):])
	}
	var out []line
	for _, raw := range bytes.Split(body[:len(body)-1], []byte("\n")) {
		dec := json.NewDecoder(bytes.NewReader(raw))
		dec.UseNumber()
		var l line
		if err := dec.Decode(&l); err != nil {
			t.Fatalf("line %d: %v: %.200q", len(out), err, raw)
		}
		if dec.More() {
			t.Fatalf("line %d holds more than one value", len(out))
		}
		if path := nullAt(map[string]any(l), ""); path != "" {
			t.Errorf("line %d has null at %s", len(out), path)
		}
		out = append(out, l)
	}
	return out
}

// nullAt returns the path of the first JSON null in v, or "".
func nullAt(v any, path string) string {
	switch x := v.(type) {
	case nil:
		return path + "/"
	case map[string]any:
		for k, m := range x {
			if p := nullAt(m, path+"/"+k); p != "" {
				return p
			}
		}
	case []any:
		for i, m := range x {
			if p := nullAt(m, path+"/"+strconv.Itoa(i)); p != "" {
				return p
			}
		}
	}
	return ""
}

// idsOf returns the message ids of the lines in order.
func idsOf(ls []line) []string {
	out := make([]string, 0, len(ls))
	for _, l := range ls {
		out = append(out, l.str("id"))
	}
	return out
}

// checkOrder checks the export order: threads ascending by id, thread_seq ascending in a thread,
// every thread in one run.
func checkOrder(t *testing.T, ls []line) {
	t.Helper()
	seen := map[string]bool{}
	for i, l := range ls {
		th := l.str("thread_id")
		if i == 0 || ls[i-1].str("thread_id") != th {
			if seen[th] {
				t.Errorf("thread %s appears in two runs", th)
			}
			seen[th] = true
			if i > 0 && ls[i-1].str("thread_id") > th {
				t.Errorf("thread %s after %s", th, ls[i-1].str("thread_id"))
			}
			continue
		}
		if ls[i-1].seq(t) >= l.seq(t) {
			t.Errorf("thread %s: thread_seq %d after %d", th, l.seq(t), ls[i-1].seq(t))
		}
	}
}

// selection returns the ids of the messages of channel in [from, to) in the order of the export
// (threads by id, then thread_seq), read with the superuser.
func (e *env) selection(t *testing.T, channel string, from, to time.Time) []string {
	t.Helper()
	rows, err := e.db.Query(t.Context(), `SELECT id::text FROM messages WHERE channel_id = $1 AND created_at >= $2 AND created_at < $3
		ORDER BY thread_id, thread_seq`, channel, from, to)
	if err != nil {
		t.Fatal(err)
	}
	out, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// problemCode returns the member code of a problem body, or "".
func problemCode(body []byte) string {
	var p struct {
		Code string `json:"code"`
	}
	_ = json.Unmarshal(body, &p)
	return p.Code
}

// problem decodes a problem body.
func problem(t *testing.T, r result) map[string]any {
	t.Helper()
	if ct := r.header.Get("Content-Type"); ct != "application/problem+json" {
		t.Errorf("Content-Type of %d = %q", r.status, ct)
	}
	var p map[string]any
	if err := json.Unmarshal(r.body, &p); err != nil {
		t.Fatalf("problem %d: %v: %s", r.status, err, r.body)
	}
	return p
}

// wantProblem checks status, code and (when not empty) reason of a problem answer and its contract.
func wantProblem(t *testing.T, r result, status int, code, reason string) map[string]any {
	t.Helper()
	if r.status != status {
		t.Fatalf("status %d, want %d: %s", r.status, status, r.body)
	}
	p := problem(t, r)
	if p["code"] != code || reason != "" && p["reason"] != reason {
		t.Errorf("problem %v, want code %s reason %s", p, code, reason)
	}
	validate(t, r)
	return p
}

// fieldErrors returns the pointer and code of every errors[] entry of a problem.
func fieldErrors(p map[string]any) []string {
	list, _ := p["errors"].([]any)
	var out []string
	for _, e := range list {
		f, _ := e.(map[string]any)
		out = append(out, fmt.Sprint(f["pointer"])+" "+fmt.Sprint(f["code"]))
	}
	return out
}

// problemOf is the problem body without its per-request members.
func problemOf(t *testing.T, r result) string {
	t.Helper()
	p := problem(t, r)
	delete(p, "correlation_id")
	delete(p, "instance")
	b, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// headerSet is the response headers without the per-request ones.
func headerSet(r result) string {
	var out []string
	for k, v := range r.header {
		switch k {
		case "Correlation-Id", "Date", "Ratelimit", "Ratelimit-Policy", "Content-Length", "Dpop-Nonce":
			continue
		}
		out = append(out, k+"="+strings.Join(v, ","))
	}
	slices.Sort(out)
	return strings.Join(out, "\n")
}

// auditRow is one row of audit_events.
type auditRow struct {
	Seq                                            int64
	Decision, DenyReason, ResourceKind, ResourceID string
	Actor, Session, Device, Correlation            string
	Details                                        map[string]any
}

// audits returns the rows of action by actor, oldest first.
func (e *env) audits(t *testing.T, action, actor string) []auditRow {
	t.Helper()
	rows, err := e.db.Query(t.Context(), `SELECT seq, decision, coalesce(deny_reason, ''), coalesce(resource_kind, ''), coalesce(resource_id::text, ''),
		coalesce(actor_principal_id::text, ''), coalesce(actor_session_id::text, ''), coalesce(actor_device_id::text, ''),
		coalesce(correlation_id::text, ''), details::text FROM audit_events WHERE action = $1 AND actor_principal_id = $2 ORDER BY seq`, action, actor)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []auditRow
	for rows.Next() {
		var r auditRow
		var raw string
		if err := rows.Scan(&r.Seq, &r.Decision, &r.DenyReason, &r.ResourceKind, &r.ResourceID, &r.Actor, &r.Session, &r.Device, &r.Correlation, &raw); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal([]byte(raw), &r.Details); err != nil {
			t.Fatal(err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

// allows returns the allow rows of export.channel by actor.
func (e *env) allows(t *testing.T, actor string) []auditRow {
	t.Helper()
	var out []auditRow
	for _, r := range e.audits(t, exportAction, actor) {
		if r.Decision == "allow" {
			out = append(out, r)
		}
	}
	return out
}

// gate holds an ACCESS EXCLUSIVE lock on messages in a superuser transaction, so the statements
// of an export wait at a known point (database.lock_timeout and database.statement_timeout of the
// server are raised for it). step lets exactly one waiting statement run: it queues a second lock
// behind the waiting statement and then releases the first, so the export proceeds to its next
// statement on messages and waits there again.
type gate struct {
	e  *env
	tx pgx.Tx
}

// lockMessages takes the gate.
func (e *env) lockMessages(t *testing.T) *gate {
	t.Helper()
	g := &gate{e: e}
	tx, err := e.db.Begin(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(t.Context(), `SET LOCAL lock_timeout = 0`); err != nil {
		t.Fatal(err)
	}
	slow := time.AfterFunc(3*time.Second, func() { g.dump(t) })
	_, err = tx.Exec(t.Context(), `LOCK TABLE messages IN ACCESS EXCLUSIVE MODE`)
	slow.Stop()
	if err != nil {
		t.Fatal(err)
	}
	g.tx = tx
	t.Cleanup(func() { _ = g.tx.Rollback(context.WithoutCancel(t.Context())) })
	return g
}

// isWaiting reports whether an export statement of the server waits for the gate now.
func (g *gate) isWaiting(t *testing.T) bool {
	t.Helper()
	for _, q := range g.waiting(t) {
		if strings.Contains(q, "ListExportThread") {
			return true
		}
	}
	return false
}

// waiting returns the text of the statements of the server role that wait for a lock.
func (g *gate) waiting(t *testing.T) []string {
	t.Helper()
	rows, err := g.e.db.Query(t.Context(), `SELECT query FROM pg_stat_activity WHERE usename = 'bothy_app' AND wait_event_type = 'Lock'`)
	if err != nil {
		t.Fatal(err)
	}
	out, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// waitFor waits until an export statement of the server waits for the gate and returns its name
// (ListExportThreadIDs or ListExportThreadMessages).
func (g *gate) waitFor(t *testing.T) string {
	t.Helper()
	var found string
	testkit.Until(t, "an export statement waits for the gate", func() bool {
		for _, q := range g.waiting(t) {
			for _, name := range []string{"ListExportThreadIDs", "ListExportThreadMessages"} {
				if strings.Contains(q, name) {
					found = name
					return true
				}
			}
		}
		return false
	})
	return found
}

// step lets the waiting export statement run and takes the gate again behind it.
func (g *gate) step(t *testing.T) {
	t.Helper()
	g.waitFor(t)
	next, err := g.e.db.Begin(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	var pid int
	if _, err := next.Exec(t.Context(), `SET LOCAL lock_timeout = 0`); err != nil {
		t.Fatal(err)
	}
	if err := next.QueryRow(t.Context(), `SELECT pg_backend_pid()`).Scan(&pid); err != nil {
		t.Fatal(err)
	}
	locked := make(chan error, 1)
	go func() {
		_, err := next.Exec(context.WithoutCancel(t.Context()), `LOCK TABLE messages IN ACCESS EXCLUSIVE MODE`)
		locked <- err
	}()
	testkit.Until(t, "the second lock queues behind the gate", func() bool {
		var n int
		if err := g.e.db.QueryRow(t.Context(), `SELECT count(*) FROM pg_stat_activity WHERE pid = $1 AND wait_event_type = 'Lock'`, pid).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n == 1
	})
	if err := g.tx.Commit(t.Context()); err != nil {
		t.Fatal(err)
	}
	g.tx = next
	select {
	case err := <-locked:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(blockedWait):
		g.dump(t)
		t.Fatal("the gate was not taken again")
	}
}

// dump logs the server connections and the locks on messages (diagnosis of a stuck gate).
func (g *gate) dump(t *testing.T) {
	t.Helper()
	rows, err := g.e.db.Query(context.WithoutCancel(t.Context()), `SELECT a.pid, coalesce(a.usename, ''), coalesce(a.state, ''), coalesce(a.wait_event_type, ''),
		coalesce((SELECT string_agg(l.mode || ':' || l.granted::text, ',') FROM pg_locks l WHERE l.pid = a.pid AND l.relation = 'messages'::regclass), ''),
		left(regexp_replace(coalesce(a.query, ''), '\s+', ' ', 'g'), 160), coalesce(current_setting('lock_timeout'), '')
		FROM pg_stat_activity a WHERE a.datname = current_database() ORDER BY a.pid`)
	if err != nil {
		t.Log(err)
		return
	}
	defer rows.Close()
	for rows.Next() {
		var pid int
		var user, state, wait, locks, query, lt string
		if err := rows.Scan(&pid, &user, &state, &wait, &locks, &query, &lt); err != nil {
			t.Log(err)
			return
		}
		t.Logf("pid %d %s state=%q wait=%q messages=%q lock_timeout=%s %s", pid, user, state, wait, locks, lt, query)
	}
}

// release lets the export run free.
func (g *gate) release(t *testing.T) {
	t.Helper()
	if err := g.tx.Commit(t.Context()); err != nil {
		t.Fatal(err)
	}
}

// lineReader reads the lines of a streamed body in the background.
type lineReader struct {
	mu    sync.Mutex
	lines [][]byte
	got   chan struct{}
	done  chan struct{}
	err   error
	tail  []byte
	// hold, when not nil, stops the reading after the first complete line until it is closed.
	hold chan struct{}
}

// readStream starts reading body.
func readStream(body io.Reader) *lineReader { return readStreamHold(body, nil) }

// readStreamHold starts reading body; with hold it stops after the first line until hold is
// closed, so the server meets a client that does not read.
func readStreamHold(body io.Reader, hold chan struct{}) *lineReader {
	lr := &lineReader{got: make(chan struct{}, 1), done: make(chan struct{}), hold: hold}
	go func() {
		defer close(lr.done)
		br := bufio.NewReader(body)
		for {
			b, err := br.ReadBytes('\n')
			if err == nil {
				lr.mu.Lock()
				lr.lines = append(lr.lines, b)
				n := len(lr.lines)
				lr.mu.Unlock()
				select {
				case lr.got <- struct{}{}:
				default:
				}
				if n == 1 && lr.hold != nil {
					<-lr.hold
				}
				continue
			}
			lr.mu.Lock()
			lr.tail, lr.err = b, err
			lr.mu.Unlock()
			return
		}
	}()
	return lr
}

// count returns the number of complete lines read so far.
func (lr *lineReader) count() int {
	lr.mu.Lock()
	defer lr.mu.Unlock()
	return len(lr.lines)
}

// settle gives the reader a moment to take what the server flushed.
func (lr *lineReader) settle() {
	select {
	case <-lr.got:
	case <-lr.done:
	case <-time.After(300 * time.Millisecond):
	}
}

// waitLines waits until n complete lines are read.
func (lr *lineReader) waitLines(t *testing.T, n int) {
	t.Helper()
	deadline := time.After(blockedWait)
	for lr.count() < n {
		select {
		case <-lr.got:
		case <-lr.done:
			if lr.count() < n {
				t.Fatalf("stream ended after %d lines (%v), want %d", lr.count(), lr.err, n)
			}
		case <-deadline:
			t.Fatalf("%d lines after %v, want %d", lr.count(), blockedWait, n)
		}
	}
}

// finish waits for the end of the body and returns the complete lines, the bytes after the last
// newline and the read error (io.EOF for a complete body).
func (lr *lineReader) finish(t *testing.T) ([]byte, []byte, error) {
	t.Helper()
	select {
	case <-lr.done:
	case <-time.After(streamWait):
		t.Fatal("the stream did not end")
	}
	lr.mu.Lock()
	defer lr.mu.Unlock()
	return bytes.Join(lr.lines, nil), lr.tail, lr.err
}

// grant creates a grant of level read with the capability export on target as the owner and
// returns its id.
func (e *env) grantExport(t *testing.T, subject *harness.Client, space string) string {
	t.Helper()
	var g struct {
		ID string `json:"id"`
	}
	harness.MustDo(t, e.w.Owner, harness.Request{Method: http.MethodPost, Path: "/grants", Body: map[string]any{
		"subject_kind": "user", "subject_id": subject.PrincipalID, "target_kind": "space", "target_id": space,
		"level": "read", "capabilities": []string{"export"}, "reason": "export test",
	}}, http.StatusCreated).JSON(t, &g)
	if g.ID == "" {
		t.Fatal("no grant id")
	}
	return g.ID
}

// msg is a posted message.
type msg struct {
	ID, Thread, Content string
	Seq                 int64
	Created             time.Time
}

// decodeMsg reads a Message body.
func decodeMsg(t *testing.T, b []byte) msg {
	t.Helper()
	var m struct {
		ID        string    `json:"id"`
		ThreadID  string    `json:"thread_id"`
		ThreadSeq int64     `json:"thread_seq"`
		ContentMD string    `json:"content_md"`
		CreatedAt time.Time `json:"created_at"`
	}
	if err := json.Unmarshal(b, &m); err != nil || m.ID == "" {
		t.Fatalf("message %s: %v", b, err)
	}
	return msg{ID: m.ID, Thread: m.ThreadID, Content: m.ContentMD, Seq: m.ThreadSeq, Created: m.CreatedAt}
}

// post posts content into thread as c at the pinned instant at (zero: the running clock).
func (e *env) post(t *testing.T, c *harness.Client, thread, content string, at time.Time) msg {
	t.Helper()
	if !at.IsZero() {
		e.clk.Pin(at)
		defer e.clk.Unpin()
	}
	r := harness.MustDo(t, c, harness.Request{Method: http.MethodPost, Path: "/threads/" + thread + "/messages", Body: map[string]any{
		"client_msg_id": ids.New().String(), "content_md": content,
	}}, http.StatusCreated)
	return decodeMsg(t, r.Body)
}

// newThread creates a thread in the text channel on a new parent message of mainThread, both at
// the pinned instant at, and returns the parent, the thread id and its first message.
func (e *env) newThread(t *testing.T, c *harness.Client, channel, mainThread, parent, first string, at time.Time) (msg, string, msg) {
	t.Helper()
	p := e.post(t, c, mainThread, parent, at)
	e.clk.Pin(at)
	defer e.clk.Unpin()
	var out struct {
		Thread struct {
			ID string `json:"id"`
		} `json:"thread"`
		FirstMessage json.RawMessage `json:"first_message"`
	}
	harness.MustDo(t, c, harness.Request{Method: http.MethodPost, Path: "/channels/" + channel + "/threads", Body: map[string]any{
		"title": "Topic " + first[:min(len(first), 12)], "parent_message_id": p.ID,
		"first_message": map[string]any{"client_msg_id": ids.New().String(), "content_md": first},
	}}, http.StatusCreated).JSON(t, &out)
	return p, out.Thread.ID, decodeMsg(t, out.FirstMessage)
}

// runAnchors returns the run anchors (kind = run, M2-05) of a thread in thread order.
func (e *env) runAnchors(t *testing.T, thread string) []msg {
	t.Helper()
	rows, err := e.db.Query(t.Context(), `SELECT id::text, thread_seq, created_at FROM messages WHERE thread_id = $1 AND kind = 'run' ORDER BY thread_seq`, thread)
	if err != nil {
		t.Fatalf("run anchors of %s: %v", thread, err)
	}
	defer rows.Close()
	var out []msg
	for rows.Next() {
		m := msg{Thread: thread}
		if err := rows.Scan(&m.ID, &m.Seq, &m.Created); err != nil {
			t.Fatalf("run anchor of %s: %v", thread, err)
		}
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("run anchors of %s: %v", thread, err)
	}
	return out
}

// mainThread returns the main thread of a text channel (getChannel does not name it).
func (e *env) mainThread(t *testing.T, channel string) string {
	t.Helper()
	var id string
	if err := e.db.QueryRow(t.Context(), `SELECT id::text FROM threads WHERE channel_id = $1 AND kind = 'main'`, channel).Scan(&id); err != nil {
		t.Fatalf("main thread of %s: %v", channel, err)
	}
	return id
}

// newChannel creates a text channel in space and returns its id and main thread.
func (e *env) newChannel(t *testing.T, space, name string) (string, string) {
	t.Helper()
	var ch struct {
		ID           string `json:"id"`
		MainThreadID string `json:"main_thread_id"`
	}
	harness.MustDo(t, e.w.Owner, harness.Request{Method: http.MethodPost, Path: "/spaces/" + e.w.Spaces[space] + "/channels",
		Body: map[string]any{"kind": "text", "name": name}}, http.StatusCreated).JSON(t, &ch)
	return ch.ID, ch.MainThreadID
}

// isUnexpectedEOF reports whether err is the cut of a chunked body.
func isUnexpectedEOF(err error) bool { return errors.Is(err, io.ErrUnexpectedEOF) }
