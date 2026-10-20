// SPDX-License-Identifier: Apache-2.0

// Package integrations_test holds the end-to-end tests of the incoming webhooks (M2-27) and of
// the reference integrations that use them (11 section 10.3 monitoring alerts, 10.4 cron
// reports): the assembled server of internal/wire with role all on the shared harness (M1-71),
// PostgreSQL 18 through pgtest, the matrix world of 07 section 11.1 built through the public API,
// and the standard library HTTP client as the sender. No third-party service is called; the
// Alertmanager payloads are fixtures of internal/apps/domain/incoming/testdata.
package integrations_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/test/harness"
)

func TestMain(m *testing.M) { os.Exit(harness.Main(m)) }

// scopeMessagesWrite is the one scope the app Tracker is granted (07 section 11.1).
const scopeMessagesWrite = "messages:write"

// hw is one harness server with the matrix world, the app Tracker installed with scope
// messages:write only, its grant G-6 write on Support/Eskalation and a forum channel
// Support/Meldungen where Tracker writes too.
type hw struct {
	t   *testing.T
	s   *harness.Server
	w   *harness.World
	ctx context.Context
	// tracker is the app principal; g6 its grant on Support/Eskalation.
	tracker, g6 string
	// eskalation and its main thread; forum is Support/Meldungen.
	eskalation, eskalationMain, forum string
	http                              *http.Client
}

// options are the knobs of a world.
type options struct {
	cfg  map[string]string
	opts func(*harness.Options)
}

// newHW starts a server with raised rate limits (unless cfg sets a class) and builds the world.
func newHW(t *testing.T, o options) *hw {
	t.Helper()
	c := harness.RaisedRateLimits()
	for k, v := range o.cfg {
		c[k] = v
	}
	ho := harness.Options{EgressDenyAll: true, Config: c}
	if o.opts != nil {
		o.opts(&ho)
	}
	s := harness.Start(t, ho)
	h := &hw{t: t, s: s, w: harness.BuildMatrixWorld(t, s), ctx: t.Context(), http: &http.Client{Timeout: time.Minute}}
	h.eskalation = h.w.Channels["Support/Eskalation"]
	var ch struct {
		Channel struct {
			MainThreadID string `json:"main_thread_id"`
		} `json:"channel"`
	}
	h.must(h.w.Owner, http.MethodGet, "/channels/"+h.eskalation, nil, http.StatusOK).JSON(t, &ch)
	h.eskalationMain = ch.Channel.MainThreadID
	h.tracker = h.install("tracker", scopeMessagesWrite)
	h.g6 = h.grantApp(h.tracker, h.eskalation)
	h.forum = created(t, h.must(h.w.Owner, http.MethodPost, "/spaces/"+h.w.Spaces["Support"]+"/channels",
		map[string]any{"kind": "forum", "name": "Meldungen"}, http.StatusCreated))
	h.grantApp(h.tracker, h.forum)
	return h
}

// must sends one request as c with the step-up retry of the harness and fails unless want.
func (h *hw) must(c *harness.Client, method, path string, body any, want int) harness.Response {
	h.t.Helper()
	return harness.MustDo(h.t, c, harness.Request{Method: method, Path: path, Body: body}, want)
}

// install installs an app with slug and the granted scopes as the owner and returns its id.
func (h *hw) install(slug string, granted ...string) string {
	h.t.Helper()
	m := map[string]any{
		"schema_version": 1, "slug": slug, "name": "App " + slug, "description": "Posts monitoring alerts and reports.",
		"publisher": "Example Org", "version": "1.0.0", "min_api_version": "0.1",
		"scopes": map[string]any{"required": granted, "optional": []string{}},
	}
	return created(h.t, h.must(h.w.Owner, http.MethodPost, "/apps", map[string]any{
		"manifest": m, "granted_scopes": granted, "reason": "Alerts and reports of the stack",
	}, http.StatusCreated))
}

// grantApp grants app write on channel as the owner and returns the grant id.
func (h *hw) grantApp(app, channel string) string {
	h.t.Helper()
	return created(h.t, h.must(h.w.Owner, http.MethodPost, "/grants", map[string]any{
		"subject_kind": "app", "subject_id": app, "target_kind": "channel", "target_id": channel, "level": "write",
		"reason": "Posts of the app",
	}, http.StatusCreated))
}

// hook is a created incoming webhook: its id, the URL shown once and the token in it.
type hook struct {
	id, url, token string
}

// create creates an incoming webhook of app on channel (optionally with a fixed thread) as the
// owner.
func (h *hw) create(app, channel, thread string) hook {
	h.t.Helper()
	body := map[string]any{"app_id": app, "name": "Monitoring alerts"}
	if thread != "" {
		body["thread_id"] = thread
	}
	var res struct {
		Webhook struct {
			ID string `json:"id"`
		} `json:"webhook"`
		URL string `json:"url"`
	}
	h.must(h.w.Owner, http.MethodPost, "/channels/"+channel+"/incoming-webhooks", body, http.StatusCreated).JSON(h.t, &res)
	i := strings.LastIndex(res.URL, "/")
	return hook{id: res.Webhook.ID, url: res.URL, token: res.URL[i+1:]}
}

// reply is the answer to a webhook call.
type reply struct {
	status int
	header http.Header
	body   []byte
}

// problem returns code, the first errors[].code and pointer and detail of a problem answer.
func (r reply) problem() (code, fieldCode, pointer, detail, instance string) {
	var p struct {
		Code     string `json:"code"`
		Detail   string `json:"detail"`
		Instance string `json:"instance"`
		Errors   []struct {
			Code    string `json:"code"`
			Pointer string `json:"pointer"`
		} `json:"errors"`
	}
	_ = json.Unmarshal(r.body, &p)
	if len(p.Errors) > 0 {
		fieldCode, pointer = p.Errors[0].Code, p.Errors[0].Pointer
	}
	return p.Code, fieldCode, pointer, p.Detail, p.Instance
}

// result decodes IncomingWebhookResult.
func (r reply) result(t *testing.T) (message, thread string) {
	t.Helper()
	var v struct {
		MessageID string `json:"message_id"`
		ThreadID  string `json:"thread_id"`
	}
	if err := json.Unmarshal(r.body, &v); err != nil || v.MessageID == "" || v.ThreadID == "" {
		t.Fatalf("result %d %s: %v", r.status, r.body, err)
	}
	return v.MessageID, v.ThreadID
}

// send posts raw to url with the headers hdr (nil for none) through the standard library client.
func (h *hw) send(url string, raw []byte, hdr http.Header) reply {
	h.t.Helper()
	req, err := http.NewRequestWithContext(h.ctx, http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		h.t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, vs := range hdr {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}
	resp, err := h.http.Do(req)
	if err != nil {
		h.t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		h.t.Fatal(err)
	}
	return reply{status: resp.StatusCode, header: resp.Header, body: b}
}

// postPath posts body (marshalled) to the path URL of k on the public listener.
func (h *hw) postPath(k hook, body any) reply {
	h.t.Helper()
	return h.send(h.s.APIBase+"/hooks/"+k.id+"/"+k.token, marshal(h.t, body), nil)
}

// postBearer posts body to /hooks/{id} with Authorization: Bearer <token>.
func (h *hw) postBearer(k hook, body any) reply {
	h.t.Helper()
	hdr := http.Header{}
	hdr.Set("Authorization", "Bearer "+k.token)
	return h.send(h.s.APIBase+"/hooks/"+k.id, marshal(h.t, body), hdr)
}

// marshal returns the JSON of v; a []byte stays as it is.
func marshal(t *testing.T, v any) []byte {
	t.Helper()
	if b, ok := v.([]byte); ok {
		return b
	}
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// native is a native body with content and the optional members in kv (name, value pairs).
func native(content string, kv ...string) map[string]any {
	m := map[string]any{"content_md": content}
	for i := 0; i+1 < len(kv); i += 2 {
		m[kv[i]] = kv[i+1]
	}
	return m
}

// created returns the id of a created resource.
func created(t *testing.T, r harness.Response) string {
	t.Helper()
	var v struct {
		ID string `json:"id"`
	}
	r.JSON(t, &v)
	if v.ID == "" {
		t.Fatalf("no id in %s", r.Body)
	}
	return v.ID
}

// row reads one row of query with args through the superuser pool into dst.
func (h *hw) row(query string, args []any, dst ...any) {
	h.t.Helper()
	if err := h.s.AdminDB().QueryRow(h.ctx, query, args...).Scan(dst...); err != nil {
		h.t.Fatalf("query %q: %v", query, err)
	}
}

// count returns the number of rows of query.
func (h *hw) count(query string, args ...any) int {
	h.t.Helper()
	var n int
	h.row("SELECT count(*) FROM ("+query+") q", args, &n)
	return n
}

// message is a stored message row.
type message struct {
	threadID, channelID, authorID, origin, kind, content string
	silent                                               bool
	triggerState                                         *string
}

// message reads the stored message id.
func (h *hw) message(id string) message {
	h.t.Helper()
	var m message
	h.row(`SELECT thread_id::text, channel_id::text, author_principal_id::text, origin, kind, content_md, silent, trigger_state
		FROM messages WHERE id = $1`, []any{id}, &m.threadID, &m.channelID, &m.authorID, &m.origin, &m.kind, &m.content, &m.silent, &m.triggerState)
	return m
}

// events counts the events of type typ about the resource id written by an actor of kind
// actorKind ("" for any).
func (h *hw) events(typ, resourceID, actorKind string) int {
	h.t.Helper()
	return h.count(`SELECT 1 FROM event_log WHERE type = $1 AND resource_id = $2 AND ($3 = '' OR actor_kind = $3)`, typ, resourceID, actorKind)
}

// newID returns a new UUIDv7 as text.
func newID() string { return ids.New().String() }
