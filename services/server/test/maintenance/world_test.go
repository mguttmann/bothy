// SPDX-License-Identifier: Apache-2.0

// Package maintenance_test holds the integration tests of brief M7-08: maintenance mode
// server.maintenance_mode = read_only (ADR 0065) on the real server of internal/wire in role all
// through test/harness, PostgreSQL from pgtest. A world is built with the mode off, then the
// server restarts with the mode on behind a proxy that keeps server.public_url, the way an owner
// switches the mode with a restart (ADR 0065 item 1).
package maintenance_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
	"github.com/OWNER/bothy/services/server/test/harness"
)

func TestMain(m *testing.M) { os.Exit(harness.Main(m)) }

// retryAfter is server.maintenance_retry_after of the tests, in whole seconds as Retry-After.
const (
	retryAfter        = 37 * time.Second
	retryAfterSeconds = "37"
)

// switchProxy keeps the origin of server.public_url while the server behind it restarts.
type switchProxy struct {
	mu  sync.Mutex
	up  *url.URL
	ln  net.Listener
	srv *http.Server
}

// newSwitchProxy serves on 127.0.0.1 with a port the system picks until the test ends.
func newSwitchProxy(t *testing.T) *switchProxy {
	t.Helper()
	ln, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	p := &switchProxy{ln: ln}
	rp := &httputil.ReverseProxy{Rewrite: func(r *httputil.ProxyRequest) {
		p.mu.Lock()
		up := p.up
		p.mu.Unlock()
		r.SetURL(up)
		r.Out.Host = r.In.Host
	}}
	p.srv = &http.Server{Handler: rp, ReadHeaderTimeout: time.Minute}
	go func() { _ = p.srv.Serve(ln) }()
	t.Cleanup(func() { _ = p.srv.Close() })
	return p
}

func (p *switchProxy) origin() string { return "http://" + p.ln.Addr().String() }

func (p *switchProxy) set(upstream string) {
	u, _ := url.Parse(upstream)
	p.mu.Lock()
	p.up = u
	p.mu.Unlock()
}

// logs is the log output of every generation of the server.
type logs struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (l *logs) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.Write(p)
}

// String returns the log so far.
func (l *logs) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.String()
}

// world is one server with the matrix world of 07 and a running run of Lotse.
type world struct {
	t   *testing.T
	s   *harness.Server
	w   *harness.World
	clk *clock.Fake
	db  *pgxpool.Pool
	log *logs
	// run is the running run of Lotse in thread; channel is the channel of the thread.
	run, thread, channel string
}

// newWorld starts the server with the mode off, builds the matrix world and starts the queued run
// of Lotse in Support/Allgemein, so the world has a run in state running. extra goes into the
// configuration of every generation.
func newWorld(t *testing.T, extra map[string]string) *world {
	t.Helper()
	return newWorldWith(t, extra, nil)
}

// newWorldWith is newWorld with change applied to the options of the harness.
func newWorldWith(t *testing.T, extra map[string]string, change func(*harness.Options)) *world {
	t.Helper()
	p := newSwitchProxy(t)
	w := &world{t: t, clk: clock.NewFake(time.Now().UTC().Truncate(time.Millisecond)), log: &logs{}}
	conf := harness.RaisedRateLimits()
	conf["server.maintenance_retry_after"] = retryAfter.String()
	for k, v := range extra {
		conf[k] = v
	}
	opts := harness.Options{
		Config: conf, Clock: w.clk, ManualJobs: true, ProofsOnClock: true, PublicURL: p.origin(), OnListen: p.set,
		LogWriter: w.log,
	}
	if change != nil {
		change(&opts)
	}
	w.s = harness.Start(t, opts)
	w.db = w.s.AdminDB()
	w.w = harness.BuildMatrixWorld(t, w.s)
	lotse := w.w.Agents["Lotse"]
	// The cursor of the default consumer starts at the head of the log at the first poll, so
	// Lotse polls before the thread that opens its run exists.
	w.drain(lotse)
	w.channel = w.w.Channels["Support/Allgemein"]
	w.thread = w.newThread(w.s.Owner, w.channel, "Question for Lotse")
	w.run = w.text(`SELECT id::text FROM runs WHERE thread_id = $1 AND state = 'queued'`, w.thread)
	token := w.invocationToken(lotse, w.run)
	harness.MustDo(t, w.w.Agents["Lotse"], harness.Request{Method: http.MethodPost, Path: "/runs/" + w.run + "/start", Body: map[string]any{
		"invocation_token": token, "billing_mode_used": "organization", "tool_class_used": "chat",
	}}, http.StatusOK)
	if st := w.text(`SELECT state FROM runs WHERE id = $1`, w.run); st != "running" {
		t.Fatalf("run %s is %s, want running", w.run, st)
	}
	return w
}

// drain polls the default consumer of agent once and acknowledges what it got.
func (w *world) drain(agent *harness.Client) {
	w.t.Helper()
	var page struct {
		Events     []json.RawMessage `json:"events"`
		NextCursor int64             `json:"next_cursor"`
	}
	harness.MustDo(w.t, agent, harness.Request{Method: http.MethodGet, Path: "/agents/me/consumers/default/events"}, http.StatusOK).JSON(w.t, &page)
	if len(page.Events) > 0 {
		harness.MustDo(w.t, agent, harness.Request{Method: http.MethodPost, Path: "/agents/me/consumers/default/ack",
			Body: map[string]any{"seq": page.NextCursor}}, http.StatusOK)
	}
}

// newThread creates a thread in the text channel on a new message of its main thread, as c, and
// returns its id; with the operator of the channel's space the first message opens a run.
func (w *world) newThread(c *harness.Client, channel, text string) string {
	w.t.Helper()
	var ch struct {
		Channel struct {
			MainThreadID string `json:"main_thread_id"`
		} `json:"channel"`
		MainThreadID string `json:"main_thread_id"`
	}
	harness.MustDo(w.t, c, harness.Request{Method: http.MethodGet, Path: "/channels/" + channel}, http.StatusOK).JSON(w.t, &ch)
	main := ch.MainThreadID
	if main == "" {
		main = ch.Channel.MainThreadID
	}
	var msg struct {
		ID string `json:"id"`
	}
	harness.MustDo(w.t, c, harness.Request{Method: http.MethodPost, Path: "/threads/" + main + "/messages", Body: map[string]any{
		"client_msg_id": newID(), "content_md": text,
	}}, http.StatusCreated).JSON(w.t, &msg)
	var th struct {
		Thread struct {
			ID string `json:"id"`
		} `json:"thread"`
	}
	harness.MustDo(w.t, c, harness.Request{Method: http.MethodPost, Path: "/channels/" + channel + "/threads", Body: map[string]any{
		"title": text, "parent_message_id": msg.ID, "first_message": map[string]any{"client_msg_id": newID(), "content_md": text},
	}}, http.StatusCreated).JSON(w.t, &th)
	return th.Thread.ID
}

// invocationToken polls the default consumer of agent until agent.invocation of run arrives and
// returns its token; it acknowledges every page it reads.
func (w *world) invocationToken(agent *harness.Client, run string) string {
	w.t.Helper()
	token := ""
	testkit.Until(w.t, "agent.invocation of run "+run, func() bool {
		var page struct {
			Events []struct {
				Type            string `json:"type"`
				InvocationToken string `json:"invocation_token"`
				Resource        struct {
					ID string `json:"id"`
				} `json:"resource"`
			} `json:"events"`
			NextCursor int64 `json:"next_cursor"`
		}
		harness.MustDo(w.t, agent, harness.Request{Method: http.MethodGet, Path: "/agents/me/consumers/default/events"}, http.StatusOK).JSON(w.t, &page)
		for _, e := range page.Events {
			if e.Type == "agent.invocation" && e.Resource.ID == run && e.InvocationToken != "" {
				token = e.InvocationToken
			}
		}
		if len(page.Events) > 0 {
			harness.MustDo(w.t, agent, harness.Request{Method: http.MethodPost, Path: "/agents/me/consumers/default/ack",
				Body: map[string]any{"seq": page.NextCursor}}, http.StatusOK)
		}
		return token != ""
	})
	return token
}

// maintenance restarts the server with server.maintenance_mode = read_only.
func (w *world) maintenance() {
	w.t.Helper()
	w.s.Restart(w.t, harness.RestartOptions{Config: map[string]string{"server.maintenance_mode": "read_only"}})
}

// text reads one text value.
func (w *world) text(sql string, args ...any) string {
	w.t.Helper()
	var s string
	if err := w.db.QueryRow(context.Background(), sql, args...).Scan(&s); err != nil {
		w.t.Fatalf("%s: %v", sql, err)
	}
	return s
}

// count reads one number.
func (w *world) count(sql string, args ...any) int64 {
	w.t.Helper()
	var n int64
	if err := w.db.QueryRow(context.Background(), sql, args...).Scan(&n); err != nil {
		w.t.Fatalf("%s: %v", sql, err)
	}
	return n
}

// do sends r as c and returns the response.
func (w *world) do(c *harness.Client, r harness.Request) harness.Response {
	w.t.Helper()
	return c.Do(w.t.Context(), r)
}

// wantMaintenance fails the test unless resp is 503 maintenance with the configured Retry-After.
func (w *world) wantMaintenance(what string, resp harness.Response) {
	w.t.Helper()
	var p struct {
		Code string `json:"code"`
	}
	_ = json.Unmarshal(resp.Body, &p)
	if resp.Status != http.StatusServiceUnavailable || p.Code != "maintenance" || resp.Header.Get("Retry-After") != retryAfterSeconds {
		w.t.Fatalf("%s: status %d, Retry-After %q, body %s; want 503 maintenance with Retry-After %s",
			what, resp.Status, resp.Header.Get("Retry-After"), resp.Body, retryAfterSeconds)
	}
}

// tableState is a fingerprint of the rows of the business tables (every table of schema public
// except the delivery tables of ADR 0065 "Consequences"): each row as text, hashed per table.
func (w *world) tableState() map[string]string {
	w.t.Helper()
	rows, err := w.db.Query(context.Background(), `SELECT tablename FROM pg_tables WHERE schemaname = 'public' ORDER BY tablename`)
	if err != nil {
		w.t.Fatal(err)
	}
	var tables []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			w.t.Fatal(err)
		}
		tables = append(tables, n)
	}
	rows.Close()
	out := map[string]string{}
	for _, tbl := range tables {
		if skipTable(tbl) {
			continue
		}
		out[tbl] = w.text(`SELECT coalesce(md5(string_agg(t::text, '|' ORDER BY t::text)), '') FROM ` + pgxIdent(tbl) + ` t`)
	}
	return out
}

// skipTable reports the tables that may change in maintenance mode or that only the partition
// maintenance of the log touches: the partitions of event_log are compared through their parent.
func skipTable(name string) bool {
	return strings.HasPrefix(name, "event_log_p")
}

// pgxIdent quotes a table name.
func pgxIdent(name string) string { return `"` + strings.ReplaceAll(name, `"`, `""`) + `"` }

// newID returns a fresh UUIDv7 text.
func newID() string { return ids.New().String() }
