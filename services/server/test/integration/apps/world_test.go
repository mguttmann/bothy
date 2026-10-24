// SPDX-License-Identifier: Apache-2.0

// Package apps_test holds the integration tests of commands and interactions for apps (M2-25) on
// the shared harness of M1-71: the real server with PostgreSQL 18 through pgtest, the matrix world
// of BuildMatrixWorld with the app Tracker (interaction address on the fake app of
// test/apps/fakeapp) and the app Linker (gateway), installed through installApp.
package apps_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	stdsync "sync"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/egress"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
	"github.com/OWNER/bothy/services/server/test/apps/fakeapp"
	"github.com/OWNER/bothy/services/server/test/harness"
)

func TestMain(m *testing.M) { os.Exit(harness.Main(m)) }

// The hosts of the fake app: the interaction address of Tracker and the webhook receiver of the
// subscription of Linker.
const (
	trackerHost = fakeapp.DefaultHost
	hooksHost   = "hooks.example.com"
	// otherHost and privateHost exist only for TestInteractionsURLSecurity: a host the egress
	// client admits but webhook.allowed_hosts does not, and one webhook.allowed_hosts admits that
	// resolves to a private address.
	otherHost   = "other.example.com"
	privateHost = "private.example.com"
)

// logBuffer collects the JSON log lines of the server.
type logBuffer struct {
	mu stdsync.Mutex
	b  bytes.Buffer
}

func (l *logBuffer) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

// lines returns the log lines that contain every part.
func (l *logBuffer) lines(parts ...string) []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	var out []string
	for _, line := range strings.Split(l.b.String(), "\n") {
		keep := line != ""
		for _, p := range parts {
			keep = keep && strings.Contains(line, p)
		}
		if keep {
			out = append(out, line)
		}
	}
	return out
}

// blockedCounter is egress.Metrics: it counts bothy.egress.blocked per reason.
type blockedCounter struct {
	mu stdsync.Mutex
	n  map[egress.Reason]int
}

// Blocked implements egress.Metrics.
func (b *blockedCounter) Blocked(_ context.Context, r egress.Reason) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.n[r]++
}

// count returns the refused attempts with reason r.
func (b *blockedCounter) count(r egress.Reason) int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.n[r]
}

// world is the test world: the harness server with the matrix world, the fake app, Tracker and
// Linker with their clients and their commands.
type world struct {
	t        *testing.T
	s        *harness.Server
	m        *harness.World
	fake     *fakeapp.App
	resolver *fakeapp.Resolver
	blocked  *blockedCounter
	logs     *logBuffer

	tracker, linker   string
	trackerC, linkerC *harness.Client
	ticket, link      string
}

// newWorld starts a world. The fake app listens before the server starts, because the
// configuration and the manifest of Tracker name its port; the keys of getWebhookKeys exist only
// after the start and are handed to the fake then.
func newWorld(t *testing.T, cfg func(a *fakeapp.App) map[string]string) *world {
	t.Helper()
	w := &world{t: t, blocked: &blockedCounter{n: map[egress.Reason]int{}}, logs: &logBuffer{}}
	w.fake = fakeapp.Start(t, nil, trackerHost, hooksHost, otherHost, privateHost)
	w.resolver = fakeapp.NewResolver(trackerHost, hooksHost, otherHost, privateHost)
	conf := harness.RaisedRateLimits()
	conf["webhook.allowed_hosts"] = w.fake.HostPort(trackerHost) + "," + w.fake.HostPort(hooksHost)
	conf["egress.allowed_hosts"] = w.fake.HostPort(trackerHost) + ";allow_private," + w.fake.HostPort(hooksHost) + ";allow_private"
	// Generous bounds: an answer of the fake is never late on a loaded machine; the tests of the
	// deadlines use answers that never come (fakeapp.Response.Hang) or their own world.
	conf["app.interaction_timeout"] = "20s"
	conf["command.autocomplete_timeout"] = "20s"
	conf["http.request_timeout"] = "60s"
	// Each denial is written at once, so the tests wait for its entry without the aggregation
	// window of production.
	conf["audit.deny_aggregate_window"] = "1ms"
	if cfg != nil {
		for k, v := range cfg(w.fake) {
			conf[k] = v
		}
	}
	w.s = harness.Start(t, harness.Options{Config: conf, LogWriter: w.logs,
		AppsEgress: egress.Deps{Resolver: w.resolver, RootCAs: rootsOf(w.fake), Metrics: w.blocked}})
	t.Cleanup(func() {
		if t.Failed() {
			// The lines of the requests to apps and of the job carry codes only (SEC-LOG-2).
			for _, part := range []string{"app interaction request", "runs: app dispatch", `"level":"ERROR"`} {
				for _, line := range w.logs.lines(part) {
					t.Log(line)
				}
			}
		}
	})
	w.m = harness.BuildMatrixWorld(t, w.s)
	w.fake.SetKeys(w.webhookKeys())
	w.installLinker()
	w.installTracker()
	return w
}

// rootsOf returns the TLS roots that trust the fake.
func rootsOf(a *fakeapp.App) *x509.CertPool { return a.RootCAs }

// webhookKeys reads the public keys of getWebhookKeys.
func (w *world) webhookKeys() []ed25519.PublicKey {
	w.t.Helper()
	var body struct {
		Keys []struct {
			PublicKey string `json:"public_key"`
		} `json:"keys"`
	}
	harness.MustDo(w.t, w.s.Owner, harness.Request{Method: http.MethodGet, Path: "/instance/webhook-keys"}, http.StatusOK).JSON(w.t, &body)
	var out []ed25519.PublicKey
	for _, k := range body.Keys {
		der, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(k.PublicKey, "whpk_"))
		if err != nil {
			w.t.Fatal(err)
		}
		pub, err := x509.ParsePKIXPublicKey(der)
		if err != nil {
			w.t.Fatal(err)
		}
		out = append(out, pub.(ed25519.PublicKey))
	}
	if len(out) == 0 {
		w.t.Fatal("getWebhookKeys returned no key")
	}
	return out
}

// trackerManifest is the manifest of Tracker (07 11.1): the command ticket with the options title
// (required) and queue (autocomplete), required level trigger, and the interaction address on the
// fake app.
func (w *world) trackerManifest(version string, extra ...map[string]any) map[string]any {
	cmds := []any{map[string]any{
		"name": "ticket", "description": "Open a ticket", "required_level": "trigger",
		"options": []any{
			map[string]any{"name": "title", "description": "Title of the ticket", "type": "string", "required": true},
			map[string]any{"name": "queue", "description": "Queue of the ticket", "type": "string", "required": false, "autocomplete": true},
		},
	}}
	for _, e := range extra {
		cmds = append(cmds, e)
	}
	return map[string]any{
		"schema_version": 1, "slug": "tracker", "name": "Tracker", "description": "Tickets from the chat.",
		"publisher": "Example Org", "version": version, "min_api_version": "0.1",
		"scopes":   map[string]any{"required": []any{"messages:write", "commands:write", "components:write"}, "optional": []any{"events:subscribe"}},
		"delivery": map[string]any{"mode": "gateway", "interactions_url": w.fake.URLFor(trackerHost, "/hooks/interactions")},
		"commands": cmds, "components": []any{"button", "form"},
		"data_egress": "Commands, form values and autocomplete requests go to tracker.example.com.",
	}
}

// linkerManifest is the manifest of Linker: the command link with the option target
// (autocomplete), required level write, without an interaction address; it subscribes to
// command.invoked.
func (w *world) linkerManifest(version string, extra ...map[string]any) map[string]any {
	cmds := []any{map[string]any{
		"name": "link", "description": "Link an issue", "required_level": "write",
		"options": []any{map[string]any{"name": "target", "description": "Issue to link", "type": "string", "required": false, "autocomplete": true}},
	}}
	for _, e := range extra {
		cmds = append(cmds, e)
	}
	return map[string]any{
		"schema_version": 1, "slug": "linker", "name": "Linker", "description": "Links issues into the chat.",
		"publisher": "Example Org", "version": version, "min_api_version": "0.1",
		"scopes": map[string]any{
			"required": []any{"messages:write", "commands:write", "components:write", "events:subscribe"}, "optional": []any{},
		},
		"events":   []any{"command.invoked"},
		"delivery": map[string]any{"mode": "gateway"},
		"commands": cmds, "components": []any{"button"},
		"data_egress": "Command invocations go to the gateway connection of Linker.",
	}
}

// installApp installs manifest with the scopes and event subscriptions as the owner and returns
// the app ID.
func (w *world) installApp(manifest map[string]any, scopes, events []string) string {
	w.t.Helper()
	body := map[string]any{"manifest": manifest, "granted_scopes": scopes, "reason": "test"}
	if events != nil {
		body["event_subscriptions"] = events
	}
	var app struct {
		ID string `json:"id"`
	}
	harness.MustDo(w.t, w.s.Owner, harness.Request{Method: http.MethodPost, Path: "/apps", Body: body}, http.StatusCreated).JSON(w.t, &app)
	return app.ID
}

// installLinker installs Linker with every scope, write on Support/Allgemein and an API token.
func (w *world) installLinker() {
	w.linker = w.installApp(w.linkerManifest("1.0.0"), []string{"commands:write", "components:write", "events:subscribe", "messages:write"},
		[]string{"command.invoked"})
	w.grant(w.linker, "Support/Allgemein")
	w.linkerC = w.s.NewTokenClient(w.t, &harness.Client{PrincipalID: w.linker})
	w.link = w.commandID(w.linker, "link")
}

// installTracker installs Tracker with messages:write, commands:write and components:write, the
// grants G-6 (write on Support/Eskalation) and G-7 (write on Research/Notizen) and an API token.
func (w *world) installTracker() {
	w.tracker = w.installApp(w.trackerManifest("1.0.0"), []string{"commands:write", "components:write", "messages:write"}, nil)
	w.grant(w.tracker, "Support/Eskalation")
	w.grant(w.tracker, "Research/Notizen")
	w.trackerC = w.s.NewTokenClient(w.t, &harness.Client{PrincipalID: w.tracker})
	w.ticket = w.commandID(w.tracker, "ticket")
}

// grant gives the app subject write on the channel key ("Space/Channel").
func (w *world) grant(subject, channel string) {
	w.t.Helper()
	harness.MustDo(w.t, w.s.Owner, harness.Request{Method: http.MethodPost, Path: "/grants", Body: map[string]any{
		"subject_kind": "app", "subject_id": subject, "target_kind": "channel", "target_id": w.m.Channels[channel], "level": "write", "reason": "test",
	}}, http.StatusCreated)
}

// commandID reads the ID of the live command name of owner.
func (w *world) commandID(owner, name string) string {
	w.t.Helper()
	var id string
	if err := w.s.AdminDB().QueryRow(w.t.Context(), `SELECT id::text FROM commands WHERE owner_principal_id = $1 AND name = $2 AND removed_at IS NULL`,
		owner, name).Scan(&id); err != nil {
		w.t.Fatalf("command %s of %s: %v", name, owner, err)
	}
	return id
}

// count runs a count query on the admin connection.
func (w *world) count(q string, args ...any) int {
	w.t.Helper()
	var n int
	if err := w.s.AdminDB().QueryRow(w.t.Context(), q, args...).Scan(&n); err != nil {
		w.t.Fatal(err)
	}
	return n
}

// text runs a query of one text column on the admin connection; "" when there is no row.
func (w *world) text(q string, args ...any) string {
	w.t.Helper()
	var s string
	_ = w.s.AdminDB().QueryRow(w.t.Context(), q, args...).Scan(&s)
	return s
}

// do sends r as c.
func (w *world) do(c *harness.Client, r harness.Request) harness.Response {
	w.t.Helper()
	resp, err := harness.DoWithStepUp(w.t.Context(), c, r)
	if err != nil {
		w.t.Fatal(err)
	}
	return resp
}

// invoke runs the command on the channel key with options as c; thread is "" for none.
func (w *world) invoke(c *harness.Client, channel, command, thread string, options map[string]string) harness.Response {
	w.t.Helper()
	body := map[string]any{}
	if thread != "" {
		body["thread_id"] = thread
	}
	if len(options) > 0 {
		var list []any
		for k, v := range options {
			list = append(list, map[string]any{"name": k, "value": v})
		}
		body["options"] = list
	}
	return w.do(c, harness.Request{Method: http.MethodPost, Path: "/channels/" + w.m.Channels[channel] + "/commands/" + command + "/invoke", Body: body})
}

// mainThread reads the main thread of the channel key through getChannel.
func (w *world) mainThread(channel string) string {
	w.t.Helper()
	var view struct {
		Channel struct {
			MainThreadID string `json:"main_thread_id"`
		} `json:"channel"`
	}
	harness.MustDo(w.t, w.s.Owner, harness.Request{Method: http.MethodGet, Path: "/channels/" + w.m.Channels[channel]}, http.StatusOK).JSON(w.t, &view)
	return view.Channel.MainThreadID
}

// postCard posts a card with components as c into thread and returns the message and its
// components by custom_id.
func (w *world) postCard(c *harness.Client, thread, content, components string) (string, map[string]string, harness.Response) {
	w.t.Helper()
	text, _ := json.Marshal(content)
	body := `{"client_msg_id":"` + ids.New().String() + `","kind":"card","content_md":` + string(text) + `,"components":` + components + `}`
	resp := w.do(c, harness.Request{Method: http.MethodPost, Path: "/threads/" + thread + "/messages", Body: []byte(body)})
	if resp.Status != http.StatusCreated {
		return "", nil, resp
	}
	var m struct {
		ID         string `json:"id"`
		Components []struct {
			ID       string `json:"id"`
			CustomID string `json:"custom_id"`
		} `json:"components"`
	}
	resp.JSON(w.t, &m)
	out := map[string]string{}
	for _, comp := range m.Components {
		out[comp.CustomID] = comp.ID
	}
	return m.ID, out, resp
}

// click operates the component as c with values (JSON, "" for none).
func (w *world) click(c *harness.Client, component, values string) harness.Response {
	w.t.Helper()
	body := `{"client_interaction_id":"` + ids.New().String() + `"`
	if values != "" {
		body += `,"values":` + values
	}
	return w.do(c, harness.Request{Method: http.MethodPost, Path: "/components/" + component + "/interactions", Body: []byte(body + "}")})
}

// buttonAndForm are the components of a Tracker card: a button and a form with a text field.
const buttonAndForm = `[{"custom_id":"ok","type":"button","single_use":false,"definition":{"label":"OK","value":"ok"}},` +
	`{"custom_id":"details","type":"form","single_use":false,"definition":{"title":"Details","fields":[{"name":"note","label":"Note","type":"text"}]}}]`

// oneButton is a card with one button.
const oneButton = `[{"custom_id":"go","type":"button","definition":{"label":"Go","value":"go"}}]`

// want fails the test unless resp has the status.
func (w *world) want(resp harness.Response, status int) {
	w.t.Helper()
	if resp.Status != status {
		w.t.Fatalf("status %d %s, want %d", resp.Status, resp.Body, status)
	}
}

// wantProblem fails the test unless resp is a problem with status, code and reason.
func (w *world) wantProblem(resp harness.Response, status int, code, reason string) {
	w.t.Helper()
	var p struct {
		Code   string `json:"code"`
		Reason string `json:"reason"`
	}
	_ = json.Unmarshal(resp.Body, &p)
	if resp.Status != status || p.Code != code || p.Reason != reason {
		w.t.Fatalf("problem %d %s, want %d %s %s", resp.Status, resp.Body, status, code, reason)
	}
}

// until waits for cond with testkit.Patience as the only bound.
func (w *world) until(what string, cond func() bool) {
	w.t.Helper()
	testkit.Until(w.t, what, cond)
}

// patience returns a context bounded by testkit.Patience.
func (w *world) patience() (context.Context, context.CancelFunc) {
	return context.WithTimeout(w.t.Context(), testkit.Patience)
}

// interactionState reads interactions.state.
func (w *world) interactionState(id string) string {
	return w.text(`SELECT state FROM interactions WHERE id = $1`, id)
}

// stripCorrelation removes correlation_id and instance from a problem body for comparisons
// (AZ-P4).
func stripCorrelation(b []byte) map[string]any {
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	delete(m, "correlation_id")
	delete(m, "instance")
	return m
}
