// SPDX-License-Identifier: Apache-2.0

package integration_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/pb33f/libopenapi"
	"github.com/pb33f/libopenapi-validator/schema_validation"

	"github.com/OWNER/bothy/services/server/api/rest"
	authzapp "github.com/OWNER/bothy/services/server/internal/authz/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
)

// The tests of thread.move (brief M1-62) run in the world of 07 sections 10 and 11.1: Support with
// Allgemein and Eskalation, both operated by Lotse (default operator of Support), and Marketing
// with Kampagnen, operated by Texter.

// moveMessageBody is a MessageCreate body with a fresh client_msg_id.
func moveMessageBody(content string) string {
	return `{"client_msg_id":"` + ids.New().String() + `","content_md":"` + content + `"}`
}

// send posts content as name into thread and returns the new message.
func (w *world) send(name string, thread ids.ID, content string) map[string]any {
	w.t.Helper()
	return w.want(w.do(name, http.MethodPost, "/api/v1/threads/"+thread.String()+"/messages", moveMessageBody(content)), http.StatusCreated, "")
}

// newThread creates a thread of kind thread in the text channel ch from a new message of the
// owner in its main thread and returns the thread; its operator is the effective operator of ch.
func (w *world) newThread(ch ids.ID) ids.ID {
	w.t.Helper()
	parent := w.send("owner", w.mainThread(ch), "Start")
	m := w.want(w.do("owner", http.MethodPost, "/api/v1/channels/"+ch.String()+"/threads", `{"parent_message_id":"`+parent["id"].(string)+`"}`, key()...),
		http.StatusCreated, "")
	th, _ := m["thread"].(map[string]any)
	return ids.MustParse(th["id"].(string))
}

// moveBody is the ThreadMove body to target with a reason.
func moveBody(target ids.ID) string {
	return `{"target_channel_id":"` + target.String() + `","reason":"Belongs elsewhere"}`
}

// move sends moveThread as name with the headers (a fresh Idempotency-Key when none).
func (w *world) move(name string, thread, target ids.ID, headers ...string) *httpservertest.Response {
	w.t.Helper()
	if headers == nil {
		headers = key()
	}
	return w.do(name, http.MethodPost, "/api/v1/threads/"+thread.String()+"/move", moveBody(target), headers...)
}

// channelOf reads threads.channel_id of thread.
func (w *world) channelOf(thread ids.ID) string {
	return w.text(`SELECT channel_id::text FROM threads WHERE id = $1`, thread.String())
}

// copiesElsewhere counts the rows of messages, read_states and drafts of thread whose channel_id
// is not ch.
func (w *world) copiesElsewhere(thread, ch ids.ID) int64 {
	return w.count(`SELECT (SELECT count(*) FROM messages WHERE thread_id = $1 AND channel_id <> $2)
		+ (SELECT count(*) FROM read_states WHERE thread_id = $1 AND channel_id <> $2)
		+ (SELECT count(*) FROM drafts WHERE thread_id = $1 AND channel_id <> $2)`, thread.String(), ch.String())
}

// sameNotFound checks that two answers are the same 404 apart from correlation_id and instance in
// the body and Correlation-Id and Date in the headers.
func sameNotFound(t *testing.T, a, b *httpservertest.Response) {
	t.Helper()
	if a.StatusCode != http.StatusNotFound || b.StatusCode != http.StatusNotFound {
		t.Fatalf("statuses %d %d: %s %s", a.StatusCode, b.StatusCode, a.Body, b.Body)
	}
	ha, hb := a.Header.Clone(), b.Header.Clone()
	for _, k := range []string{"Correlation-Id", "Date"} {
		ha.Del(k)
		hb.Del(k)
	}
	if fa, fb := fmt.Sprint(ha), fmt.Sprint(hb); fa != fb {
		t.Fatalf("404 headers differ:\n%s\n%s", fa, fb)
	}
	pa, pb := a.Problem(t), b.Problem(t)
	for _, k := range []string{"correlation_id", "instance"} {
		delete(pa, k)
		delete(pb, k)
	}
	ja, _ := json.Marshal(pa)
	jb, _ := json.Marshal(pb)
	if string(ja) != string(jb) {
		t.Fatalf("404 bodies differ:\n%s\n%s", ja, jb)
	}
}

// validateSchema checks payload against the schema name of docs/api/openapi.yaml.
func validateSchema(t *testing.T, name, payload string) {
	t.Helper()
	doc, err := libopenapi.NewDocument(rest.Spec)
	must(t, err)
	model, err := doc.BuildV3Model()
	must(t, err)
	proxy, ok := model.Model.Components.Schemas.Get(name)
	if !ok {
		t.Fatalf("no schema %s", name)
	}
	if valid, problems := schema_validation.NewSchemaValidator().ValidateSchemaString(proxy.Schema(), payload); !valid {
		for _, p := range problems {
			t.Errorf("schema %s: %s %s", name, p.Message, p.Reason)
		}
		t.Fatalf("payload does not match %s: %s", name, payload)
	}
}

// TestMoveThreadHappyPath: a thread with three messages, read states of Alex and Sam, a draft of
// Alex and two tags moves from Support/Allgemein to Support/Eskalation: the id stays, every copy
// follows, moved_at is set, the version rises by one and no tag is left (brief M1-62 notes 1e and
// 1f, 03 threads).
func TestMoveThreadHappyPath(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	th := w.newThread(w.allgemein)
	for i := range 3 {
		w.send("alex", th, "Message "+strconv.Itoa(i))
	}
	w.wantOK(w.do("alex", http.MethodPut, "/api/v1/threads/"+th.String()+"/read-state", `{"last_read_seq":2}`))
	w.wantOK(w.do("sam", http.MethodPut, "/api/v1/threads/"+th.String()+"/read-state", `{"last_read_seq":1}`))
	w.wantOK(w.do("alex", http.MethodPut, "/api/v1/threads/"+th.String()+"/draft", `{"content_md":"Draft of a reply"}`))
	// Two tags, inserted as fixtures: the API puts tags on forum posts only, and the move must
	// remove whatever thread_tags rows exist.
	for _, name := range []string{"Bug", "Billing"} {
		tag := w.want(w.do("owner", http.MethodPost, "/api/v1/channels/"+w.projektA.String()+"/tags", `{"name":"`+name+`"}`, key()...), http.StatusCreated, "")
		w.sql(`INSERT INTO thread_tags (thread_id, tag_id, added_by) VALUES ($1, $2, $3)`, th.String(), tag["id"].(string), w.owner.String())
	}
	before := w.count(`SELECT version FROM threads WHERE id = $1`, th.String())
	if n := w.count(`SELECT count(*) FROM read_states WHERE thread_id = $1`, th.String()); n != 2 {
		t.Fatalf("read states %d", n)
	}
	m := w.wantOK(w.move("owner", th, w.eskalation))
	if m["id"] != th.String() || m["channel_id"] != w.eskalation.String() || m["moved_at"] == nil || len(m["tag_ids"].([]any)) != 0 {
		t.Fatalf("answer %v", m)
	}
	if got := int64(m["version"].(float64)); got != before+1 || w.count(`SELECT version FROM threads WHERE id = $1`, th.String()) != before+1 {
		t.Fatalf("version %d, want %d", got, before+1)
	}
	if w.channelOf(th) != w.eskalation.String() || w.count(`SELECT count(*) FROM threads WHERE id = $1 AND moved_at IS NOT NULL`, th.String()) != 1 {
		t.Fatal("thread row not moved")
	}
	if n := w.copiesElsewhere(th, w.eskalation); n != 0 {
		t.Fatalf("%d copies outside the target", n)
	}
	if w.count(`SELECT count(*) FROM messages WHERE thread_id = $1`, th.String()) != 3 ||
		w.count(`SELECT count(*) FROM read_states WHERE thread_id = $1`, th.String()) != 2 ||
		w.count(`SELECT count(*) FROM drafts WHERE thread_id = $1`, th.String()) != 1 {
		t.Fatal("rows lost on the move")
	}
	if n := w.count(`SELECT count(*) FROM thread_tags WHERE thread_id = $1`, th.String()); n != 0 {
		t.Fatalf("%d tags left", n)
	}
	got := w.wantOK(w.do("owner", http.MethodGet, "/api/v1/threads/"+th.String(), ""))
	if got["channel_id"] != w.eskalation.String() {
		t.Fatalf("getThread %v", got)
	}
}

// TestMoveThreadOperatorMismatch: Kampagnen in Marketing is operated by Texter, the thread by
// Lotse: 409 operator_mismatch, nothing changed, no event and no audit allow entry (01, 06).
func TestMoveThreadOperatorMismatch(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	th := w.newThread(w.allgemein)
	events := w.eventCount()
	version := w.count(`SELECT version FROM threads WHERE id = $1`, th.String())
	w.want(w.move("owner", th, w.kampagnen), http.StatusConflict, "operator_mismatch")
	if w.channelOf(th) != w.allgemein.String() || w.count(`SELECT version FROM threads WHERE id = $1`, th.String()) != version ||
		w.eventCount() != events || len(w.allows("thread.move")) != 0 {
		t.Fatal("a refused move changed something")
	}
}

// TestMoveThreadNeedsManageOnBoth: manage on the source only is 403; a target the caller cannot
// read is the same 404 as an unknown target id (brief M1-62 note 1a, R-AZ-01).
func TestMoveThreadNeedsManageOnBoth(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	th := w.newThread(w.allgemein)
	// Alex reads all of Support (G-1) and gets manage on Allgemein only; Robin gets write and
	// manage on Allgemein and cannot see Eskalation.
	w.grant(`"subject_kind":"user","subject_id":"` + w.alex.String() + `","target_kind":"channel","target_id":"` + w.allgemein.String() +
		`","level":"trigger","capabilities":["manage"],"reason":"Moderation"`)
	w.grant(`"subject_kind":"user","subject_id":"` + w.robin.String() + `","target_kind":"channel","target_id":"` + w.allgemein.String() +
		`","level":"write","capabilities":["manage"],"reason":"Moderation"`)
	p := w.want(w.move("alex", th, w.eskalation), http.StatusForbidden, "forbidden")
	if p["reason"] != "capability_missing" {
		t.Fatalf("reason %v", p)
	}
	sameNotFound(t, w.move("robin", th, w.eskalation), w.move("robin", th, ids.New()))
	sameNotFound(t, w.move("robin", th, w.eskalation), w.move("robin", ids.New(), w.eskalation))
	// The nil UUID as target names no channel: the 404 of an unknown target, for an invisible
	// and an unknown thread and for the owner alike (red team M1-62).
	sameNotFound(t, w.move("robin", th, ids.ID{}), w.move("robin", ids.New(), ids.ID{}))
	sameNotFound(t, w.move("owner", th, ids.ID{}), w.move("owner", th, ids.New()))
	if w.channelOf(th) != w.allgemein.String() {
		t.Fatal("moved")
	}
}

// TestMoveMainThreadRejected: the main thread belongs to its channel (R-DM-11): 409 conflict.
func TestMoveMainThreadRejected(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	w.want(w.move("owner", w.mainThread(w.allgemein), w.eskalation), http.StatusConflict, "conflict")
}

// TestMoveThreadSameChannel: the source as target is 400 validation_failed on /target_channel_id
// (docs/DECISIONS.md M1-62/3); a reason of blanks is 400 on /reason.
func TestMoveThreadSameChannel(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	th := w.newThread(w.allgemein)
	p := w.want(w.move("owner", th, w.allgemein), http.StatusBadRequest, "validation_failed")
	if !strings.Contains(fmt.Sprint(p["errors"]), "/target_channel_id") {
		t.Fatalf("errors %v", p["errors"])
	}
	r := w.do("owner", http.MethodPost, "/api/v1/threads/"+th.String()+"/move", `{"target_channel_id":"`+w.eskalation.String()+`","reason":"  "}`, key()...)
	p = w.want(r, http.StatusBadRequest, "validation_failed")
	if !strings.Contains(fmt.Sprint(p["errors"]), "/reason") {
		t.Fatalf("errors %v", p["errors"])
	}
}

// TestMoveThreadEvents: exactly thread.moved_out and then thread.moved with consecutive seq; the
// thread.moved_out row names the target nowhere; both payloads validate against their schemas
// and hold no JSON null (brief M1-62 note 1g).
func TestMoveThreadEvents(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	th := w.newThread(w.allgemein)
	last := w.count(`SELECT coalesce(max(seq), 0) FROM event_log`)
	m := w.wantOK(w.move("owner", th, w.eskalation))
	rows, err := w.db.Query(context.Background(), `SELECT seq, type, row_to_json(e)::text, payload::text, coalesce(resource_version, 0),
		resource_kind, resource_id::text, channel_id::text, coalesce(thread_id::text, ''), receive_action FROM event_log e WHERE seq > $1 ORDER BY seq`, last)
	must(t, err)
	defer rows.Close()
	type row struct {
		seq, version                                       int64
		typ, raw, payload, kind, resource, channel, thread string
		receive                                            string
	}
	var got []row
	for rows.Next() {
		var r row
		must(t, rows.Scan(&r.seq, &r.typ, &r.raw, &r.payload, &r.version, &r.kind, &r.resource, &r.channel, &r.thread, &r.receive))
		got = append(got, r)
	}
	must(t, rows.Err())
	if len(got) != 2 || got[0].typ != "thread.moved_out" || got[1].typ != "thread.moved" || got[1].seq != got[0].seq+1 {
		t.Fatalf("events %+v", got)
	}
	out, in := got[0], got[1]
	if strings.Contains(out.raw, w.eskalation.String()) {
		t.Fatalf("thread.moved_out names the target: %s", out.raw)
	}
	if out.kind != "channel" || out.resource != w.allgemein.String() || out.version != 0 || out.channel != w.allgemein.String() ||
		out.thread != "" || out.receive != "channel.read" {
		t.Fatalf("thread.moved_out %+v", out)
	}
	if in.kind != "thread" || in.resource != th.String() || in.version != int64(m["version"].(float64)) || in.channel != w.eskalation.String() ||
		in.thread != th.String() || in.receive != "thread.read" {
		t.Fatalf("thread.moved %+v", in)
	}
	for _, r := range got {
		if strings.Contains(r.payload, "null") {
			t.Fatalf("null in %s", r.payload)
		}
	}
	validateSchema(t, "ThreadMovedOutPayload", out.payload)
	validateSchema(t, "ThreadMovedPayload", in.payload)
	var p struct {
		Thread        map[string]any `json:"thread"`
		FromChannelID string         `json:"from_channel_id"`
	}
	must(t, json.Unmarshal([]byte(in.payload), &p))
	if p.FromChannelID != w.allgemein.String() || p.Thread["channel_id"] != w.eskalation.String() {
		t.Fatalf("thread.moved payload %s", in.payload)
	}
}

// TestMoveThreadAudit: one allow entry with from, to and the reason; a refused attempt by Sam
// (read on Allgemein, no manage) writes a deny entry (R-AZ-04).
func TestMoveThreadAudit(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	th := w.newThread(w.allgemein)
	w.want(w.move("sam", th, w.eskalation), http.StatusNotFound, "not_found")
	other := w.newThread(w.allgemein)
	w.grant(`"subject_kind":"user","subject_id":"` + w.sam.String() + `","target_kind":"channel","target_id":"` + w.eskalation.String() +
		`","level":"read","reason":"Escalations"`)
	w.want(w.move("sam", other, w.eskalation), http.StatusForbidden, "forbidden")
	w.wantOK(w.move("owner", th, w.eskalation))
	allows := w.allows("thread.move")
	if len(allows) != 1 || allows[0].ResourceID != th.String() {
		t.Fatalf("allows %+v", allows)
	}
	// The recorder adds the keyed IP hash of every entry (ip_hmac, ip_key_id); the use case
	// writes from, to and reason and nothing else.
	d := allows[0].Details
	delete(d, "ip_hmac")
	delete(d, "ip_key_id")
	if d["from_channel_id"] != w.allgemein.String() || d["to_channel_id"] != w.eskalation.String() || d["reason"] != "Belongs elsewhere" || len(d) != 3 {
		t.Fatalf("details %v", d)
	}
	if len(w.denies("thread.move", "capability_missing")) != 1 || len(w.denies("thread.move", "no_grant")) != 1 {
		t.Fatalf("denies %+v", w.audits("thread.move"))
	}
}

// newAuthorizer builds a second Authorizer over the same sources, the Authorizer of another
// process instance with its own snapshot cache.
func (w *world) newAuthorizer() *authzapp.Authorizer {
	w.t.Helper()
	az, err := authzapp.NewAuthorizer(w.azConfig, w.azDeps)
	must(w.t, err)
	return az
}

// TestMoveThreadAuthorizeFollowsMove: two Authorizer instances with warm snapshot caches (REST
// mode, no log events fed) decide on the place of the thread at decision time (07 4.2): after the
// move a person who reads only the target gets allow for thread.read and a person who reads only
// the source gets not_found, in both instances (brief M1-62 note 4).
func TestMoveThreadAuthorizeFollowsMove(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	th := w.newThread(w.allgemein)
	// Robin reads only Eskalation; Sam reads only Allgemein in Support (G-4).
	w.grant(`"subject_kind":"user","subject_id":"` + w.robin.String() + `","target_kind":"channel","target_id":"` + w.eskalation.String() +
		`","level":"read","reason":"Escalations"`)
	second := w.newAuthorizer()
	res := authorize.Resource{Kind: authorize.ResourceThread, ID: th}
	decide := func(az *authzapp.Authorizer, name string) authorize.Outcome {
		d, err := az.Authorize(context.Background(), w.actor(name), authorize.ActionThreadRead, res)
		must(t, err)
		return d.Outcome
	}
	for _, az := range []*authzapp.Authorizer{w.az, second} {
		if decide(az, "robin") != authorize.OutcomeNotFound || decide(az, "sam") != authorize.OutcomeAllow {
			t.Fatal("before the move")
		}
	}
	w.wantOK(w.move("owner", th, w.eskalation))
	for i, az := range []*authzapp.Authorizer{w.az, second} {
		if r, s := decide(az, "robin"), decide(az, "sam"); r != authorize.OutcomeAllow || s != authorize.OutcomeNotFound {
			t.Fatalf("instance %d after the move: robin %s, sam %s", i, r, s)
		}
	}
	w.wantOK(w.do("robin", http.MethodGet, "/api/v1/threads/"+th.String(), ""))
	w.want(w.do("sam", http.MethodGet, "/api/v1/threads/"+th.String(), ""), http.StatusNotFound, "not_found")
}

// TestMoveThreadConcurrentReadState: putReadState of Alex in parallel with the move; afterwards
// read_states.channel_id equals threads.channel_id (the thread lock serializes both, M1-61).
func TestMoveThreadConcurrentReadState(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	for round := range 5 {
		th := w.newThread(w.allgemein)
		w.send("alex", th, "Message "+strconv.Itoa(round))
		var wg sync.WaitGroup
		var moved, read *httpservertest.Response
		wg.Add(2)
		go func() { defer wg.Done(); moved = w.move("owner", th, w.eskalation) }()
		go func() {
			defer wg.Done()
			read = w.do("alex", http.MethodPut, "/api/v1/threads/"+th.String()+"/read-state", `{"last_read_seq":1}`)
		}()
		wg.Wait()
		if moved.StatusCode != http.StatusOK || read.StatusCode != http.StatusOK {
			t.Fatalf("round %d: move %d %s, read state %d %s", round, moved.StatusCode, moved.Body, read.StatusCode, read.Body)
		}
		if n := w.copiesElsewhere(th, w.eskalation); n != 0 || w.channelOf(th) != w.eskalation.String() {
			t.Fatalf("round %d: %d copies outside the thread's channel", round, n)
		}
		if w.count(`SELECT count(*) FROM read_states WHERE thread_id = $1`, th.String()) != 1 {
			t.Fatalf("round %d: read state missing", round)
		}
	}
}

// TestMoveThreadConcurrentOperatorChange: setChannelOperator on the target to Texter in parallel
// with the move: both end with a defined status (no deadlock reaches the client), a move that
// committed keeps the operator of the thread, and VerifyInvariants of authz passes (03 "I1
// check", brief M1-62 note 1d).
func TestMoveThreadConcurrentOperatorChange(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	for round := range 4 {
		th := w.newThread(w.allgemein)
		agent, back := w.texter, `{"mode":"inherit","reason":"Back to the space"}`
		var wg sync.WaitGroup
		var moved, set *httpservertest.Response
		version := w.channelVersion(w.eskalation)
		wg.Add(2)
		go func() { defer wg.Done(); moved = w.move("owner", th, w.eskalation) }()
		go func() {
			defer wg.Done()
			set = w.setChannelOperatorAs("owner", w.eskalation, agentBody(agent), ifMatch(version)...)
		}()
		wg.Wait()
		if set.StatusCode != http.StatusOK {
			t.Fatalf("round %d: operator change %d %s", round, set.StatusCode, set.Body)
		}
		switch moved.StatusCode {
		case http.StatusOK:
			if w.channelOf(th) != w.eskalation.String() {
				t.Fatalf("round %d: answered 200 without the move", round)
			}
		case http.StatusConflict:
			if moved.Problem(t)["code"] != "operator_mismatch" || w.channelOf(th) != w.allgemein.String() {
				t.Fatalf("round %d: %s", round, moved.Body)
			}
		default:
			t.Fatalf("round %d: move %d %s", round, moved.StatusCode, moved.Body)
		}
		v, err := w.hook.VerifyInvariants(context.Background(), w.clk.Now())
		if err != nil || !v.Empty() {
			t.Fatalf("round %d: invariants %+v %v", round, v, err)
		}
		w.wantOK(w.setChannelOperator(w.eskalation, back))
	}
}

// TestMoveThreadIdempotencyKey: a replay with the same key returns the stored answer with
// Idempotent-Replay: true and appends no second pair of events (M1-20).
func TestMoveThreadIdempotencyKey(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	th := w.newThread(w.allgemein)
	k := key()
	first := w.move("owner", th, w.eskalation, k...)
	w.wantOK(first)
	events := w.eventCount()
	replay := w.move("owner", th, w.eskalation, k...)
	w.wantOK(replay)
	var a, b map[string]any
	must(t, json.Unmarshal(first.Body, &a))
	must(t, json.Unmarshal(replay.Body, &b))
	if replay.Header.Get("Idempotent-Replay") != "true" || fmt.Sprint(a) != fmt.Sprint(b) {
		t.Fatalf("replay %v %s", replay.Header, replay.Body)
	}
	if w.eventCount() != events || len(w.events("thread.moved")) != 1 || len(w.events("thread.moved_out")) != 1 {
		t.Fatal("the replay appended events")
	}
}

// TestSendAfterMove: createMessage to the moved thread id succeeds and the new message has the
// target channel_id (04 "Editing and deleting", row "Thread moved").
func TestSendAfterMove(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	th := w.newThread(w.allgemein)
	w.send("alex", th, "Before")
	w.wantOK(w.move("owner", th, w.eskalation))
	m := w.send("alex", th, "After")
	if w.text(`SELECT channel_id::text FROM messages WHERE id = $1`, m["id"].(string)) != w.eskalation.String() {
		t.Fatal("new message outside the target")
	}
}
