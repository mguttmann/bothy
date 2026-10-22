// SPDX-License-Identifier: Apache-2.0

package importer_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/test/harness"
)

// Discord IDs of the fixture mapping (invented digit strings).
const (
	catNew      = "200" // category mapped to a new space
	catExisting = "201" // category mapped to the existing space
	catCollide  = "202" // category whose new space takes the name of the existing space
	catSkip     = "203" // category mapped to skip
	chText      = "300" // new text channel, lead discord, planned operator Lotse
	chForum     = "301" // new forum channel, lead discord
	chBothy     = "302" // new text channel, lead bothy
	chExisting  = "303" // the existing channel
	chSkip      = "304" // skipped channel
	chCollide   = "305" // new channel named like the existing channel in the existing space
	userAlex    = "400"
	userBot     = "401"
	userSam     = "500" // unmapped person: placeholder
	userHook    = "501" // webhook author: placeholder
	userOtherB  = "502" // unmapped bot: skipped
)

// iw is one harness server with Robin (owner), Alex (person), Lotse (agent), an existing space
// and channel the mapping adopts.
type iw struct {
	t                *testing.T
	s                *harness.Server
	robin, alex      *harness.Client
	lotse            *harness.Client
	ctx              context.Context
	existingSpace    string
	existingChannel  string
	existingMainThrd string
}

// newIW starts a server and seeds the world.
func newIW(t *testing.T) *iw {
	t.Helper()
	s := harness.Start(t, harness.Options{EgressDenyAll: true, Config: harness.RaisedRateLimits()})
	w := &iw{t: t, s: s, ctx: t.Context()}
	w.robin = s.NewUser(t, harness.UserSpec{Handle: "robin", DisplayName: "Robin", Email: "robin@example.com"})
	harness.MustDo(t, s.Owner, harness.Request{Method: http.MethodPost, Path: "/owners",
		Body: map[string]any{"user_id": w.robin.PrincipalID, "reason": "import owner"}}, http.StatusCreated)
	w.alex = s.NewUser(t, harness.UserSpec{Handle: "alex", DisplayName: "Alex", Email: "alex@example.com"})
	w.lotse = s.NewAgent(t, harness.AgentSpec{Handle: "lotse", DisplayName: "Lotse", BillingMode: "organization"})
	w.existingSpace = w.id(w.must(w.robin, http.MethodPost, "/spaces", map[string]any{"name": "Existing"}, nil, http.StatusCreated))
	var ch struct {
		ID           string `json:"id"`
		MainThreadID string `json:"main_thread_id"`
	}
	w.must(w.robin, http.MethodPost, "/spaces/"+w.existingSpace+"/channels", map[string]any{"kind": "text", "name": "old"}, nil,
		http.StatusCreated).JSON(t, &ch)
	w.existingChannel, w.existingMainThrd = ch.ID, ch.MainThreadID
	return w
}

// must sends one request with the step-up retry of the harness and fails unless want.
func (w *iw) must(c *harness.Client, method, path string, body any, h http.Header, want int) harness.Response {
	w.t.Helper()
	return harness.MustDo(w.t, c, harness.Request{Method: method, Path: path, Body: body, Header: h}, want)
}

// do sends one request without the step-up retry.
func (w *iw) do(c *harness.Client, method, path string, body any, h http.Header) harness.Response {
	w.t.Helper()
	return c.Do(w.ctx, harness.Request{Method: method, Path: path, Body: body, Header: h})
}

// id returns the member id of a JSON answer.
func (w *iw) id(r harness.Response) string {
	w.t.Helper()
	var b struct {
		ID string `json:"id"`
	}
	r.JSON(w.t, &b)
	if b.ID == "" {
		w.t.Fatalf("no id in %s", r.Body)
	}
	return b.ID
}

// mapping is the fixture mapping of import_source.discord.v1.
func (w *iw) mapping() map[string]any {
	create := func(name string) map[string]any { return map[string]any{"create": map[string]any{"name": name}} }
	return map[string]any{
		"version": 1, "guild_id": "100",
		"defaults": map[string]any{"unmapped_human": "placeholder", "unmapped_bot": "skip", "uncategorized_space": catNew},
		"spaces": []any{
			map[string]any{"discord_category": catNew, "target": create("Imported")},
			map[string]any{"discord_category": catExisting, "target": map[string]any{"existing": w.existingSpace}},
			map[string]any{"discord_category": catCollide, "target": create("existing")},
			map[string]any{"discord_category": catSkip, "target": "skip"},
		},
		"channels": []any{
			map[string]any{"discord_channel": chText, "target": create("tickets"), "planned_operator": "agent:lotse"},
			map[string]any{"discord_channel": chForum, "target": create("forum")},
			map[string]any{"discord_channel": chBothy, "target": create("led-here"), "lead": "bothy"},
			map[string]any{"discord_channel": chExisting, "target": map[string]any{"existing": w.existingChannel}},
			map[string]any{"discord_channel": chSkip, "target": "skip"},
			map[string]any{"discord_channel": chCollide, "target": create("OLD")},
		},
		"users": []any{
			map[string]any{"discord_user": userAlex, "target": "user:alex"},
			map[string]any{"discord_user": userBot, "target": "agent:lotse"},
		},
	}
}

// source is the schema ImportSource as the tests read it.
type source struct {
	ID         string         `json:"id"`
	State      string         `json:"state"`
	Config     map[string]any `json:"config"`
	Stats      map[string]any `json:"stats"`
	Cursor     map[string]any `json:"cursor"`
	StartedAt  string         `json:"started_at"`
	FinishedAt string         `json:"finished_at"`
}

// create creates the fixture source as Robin with step-up and returns its id.
func (w *iw) create() string {
	w.t.Helper()
	return w.id(w.must(w.robin, http.MethodPost, "/imports", map[string]any{"kind": "discord", "name": "Old community server",
		"config": w.mapping()}, nil, http.StatusCreated))
}

// running creates and starts the fixture source.
func (w *iw) running() string {
	w.t.Helper()
	id := w.create()
	w.must(w.robin, http.MethodPost, "/imports/"+id+"/start", nil, nil, http.StatusAccepted)
	return id
}

// get reads a source as Robin.
func (w *iw) get(id string) source {
	w.t.Helper()
	var s source
	w.must(w.robin, http.MethodGet, "/imports/"+id, nil, nil, http.StatusOK).JSON(w.t, &s)
	return s
}

// item is one entry of ImportBatchResult.items.
type item struct {
	ID            string `json:"id"`
	OriginKey     string `json:"origin_key"`
	ItemKind      string `json:"item_kind"`
	TargetKind    string `json:"target_kind"`
	TargetID      string `json:"target_id"`
	State         string `json:"state"`
	SourceChanged bool   `json:"source_changed"`
}

// obj is one ImportObject.
func obj(key, kind string, data map[string]any) map[string]any {
	return map[string]any{"origin_key": key, "item_kind": kind, "data": data}
}

// batch posts items to the source as Robin (import.apply needs no step-up).
func (w *iw) batch(id string, items ...map[string]any) harness.Response {
	w.t.Helper()
	return w.do(w.robin, http.MethodPost, "/imports/"+id+"/batches", map[string]any{"items": items}, nil)
}

// mustBatch posts items and returns the items of a 200 answer.
func (w *iw) mustBatch(id string, items ...map[string]any) []item {
	w.t.Helper()
	r := w.batch(id, items...)
	if r.Status != http.StatusOK {
		w.t.Fatalf("batch = %d %s", r.Status, r.Body)
	}
	var b struct {
		Items []item `json:"items"`
	}
	r.JSON(w.t, &b)
	return b.Items
}

// structure is the structure batch of the fixture: the new space, two new channels, a channel
// led here, a forum tag and the authors.
func structure() []map[string]any {
	return []map[string]any{
		obj("space:"+catNew, "space", map[string]any{"name": "Imported", "sort_key": "a0"}),
		obj("channel:"+chText, "channel", map[string]any{"space_origin_key": "space:" + catNew, "name": "tickets", "kind": "text", "sort_key": "a0"}),
		obj("channel:"+chForum, "channel", map[string]any{"space_origin_key": "space:" + catNew, "name": "forum", "kind": "forum", "sort_key": "a1"}),
		obj("channel:"+chBothy, "channel", map[string]any{"name": "led-here", "kind": "text", "sort_key": "a2"}),
		obj("forum_tag:"+chForum+":9", "forum_tag", map[string]any{"channel_origin_key": "channel:" + chForum, "name": "bug", "emoji": "x"}),
		obj("user:"+userAlex, "user", map[string]any{"discord_kind": "user", "username": "alex"}),
		obj("user:"+userBot, "user", map[string]any{"discord_kind": "bot", "username": "lotse"}),
		obj("user:"+userSam, "user", map[string]any{"discord_kind": "user", "username": "sam.example", "global_name": "Sam"}),
		obj("user:"+userHook, "user", map[string]any{"discord_kind": "webhook", "username": "Deploy Hook"}),
		obj("user:"+userOtherB, "user", map[string]any{"discord_kind": "bot", "username": "other"}),
	}
}

// byKey indexes batch items by origin key.
func byKey(items []item) map[string]item {
	m := map[string]item{}
	for _, it := range items {
		m[it.OriginKey] = it
	}
	return m
}

// maxSeq returns the newest seq of the event log.
func (w *iw) maxSeq() int64 {
	w.t.Helper()
	var n int64
	if err := w.s.AdminDB().QueryRow(w.ctx, "SELECT coalesce(max(seq), 0) FROM event_log").Scan(&n); err != nil {
		w.t.Fatal(err)
	}
	return n
}

// event is one row of the event log.
type event struct {
	Seq        int64
	Type       string
	ResourceID string
	Commit     string
	Payload    json.RawMessage
}

// events returns the events after seq in order; Commit is the transaction time, equal for the
// events of one commit.
func (w *iw) events(after int64) []event {
	w.t.Helper()
	rows, err := w.s.AdminDB().Query(w.ctx, `SELECT seq, type, coalesce(resource_id::text, ''), occurred_at::text, payload
		FROM event_log WHERE seq > $1 ORDER BY seq`, after)
	if err != nil {
		w.t.Fatal(err)
	}
	defer rows.Close()
	var out []event
	for rows.Next() {
		var e event
		if err := rows.Scan(&e.Seq, &e.Type, &e.ResourceID, &e.Commit, &e.Payload); err != nil {
			w.t.Fatal(err)
		}
		out = append(out, e)
	}
	return out
}

// count runs a count query on the admin pool.
func (w *iw) count(query string, args ...any) int {
	w.t.Helper()
	var n int
	if err := w.s.AdminDB().QueryRow(w.ctx, query, args...).Scan(&n); err != nil {
		w.t.Fatal(err)
	}
	return n
}

// problem is the part of application/problem+json the tests check.
type problem struct {
	Code   string `json:"code"`
	Reason string `json:"reason"`
	Errors []struct {
		Pointer string `json:"pointer"`
		Code    string `json:"code"`
	} `json:"errors"`
}

// wantProblem fails unless r has status and code (and reason when not empty).
func (w *iw) wantProblem(r harness.Response, status int, code, reason string) problem {
	w.t.Helper()
	var p problem
	_ = json.Unmarshal(r.Body, &p)
	if r.Status != status || p.Code != code || (reason != "" && p.Reason != reason) {
		w.t.Fatalf("got %d %s, want %d %s %s", r.Status, r.Body, status, code, reason)
	}
	return p
}

// hasError reports an error entry with pointer and code.
func (p problem) hasError(pointer, code string) bool {
	for _, e := range p.Errors {
		if e.Pointer == pointer && e.Code == code {
			return true
		}
	}
	return false
}

// channel reads a channel as c.
func (w *iw) channel(c *harness.Client, id string) (int, map[string]any, string) {
	w.t.Helper()
	r := w.do(c, http.MethodGet, "/channels/"+id, nil, nil)
	var v struct {
		Channel map[string]any `json:"channel"`
	}
	_ = json.Unmarshal(r.Body, &v)
	return r.Status, v.Channel, r.Header.Get("ETag")
}

// ifMatch returns an If-Match header for an ETag value.
func ifMatch(etag string) http.Header {
	h := http.Header{}
	h.Set("If-Match", etag)
	return h
}

// message posts a message into a thread as c.
func (w *iw) message(c *harness.Client, thread string) harness.Response {
	w.t.Helper()
	return w.do(c, http.MethodPost, "/threads/"+thread+"/messages", map[string]any{"client_msg_id": ids.New().String(), "content_md": "hello"}, nil)
}
