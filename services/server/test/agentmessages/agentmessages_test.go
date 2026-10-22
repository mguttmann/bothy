// SPDX-License-Identifier: Apache-2.0

// Package agentmessages_test holds the end-to-end tests of agent messages (M2-19): the assembled
// server of internal/wire with role all on the shared harness (M1-71), PostgreSQL 18 through
// pgtest and the principals of the matrix world of 07 section 11.1 built through the public API.
// TestAgentMessage_OriginEverywhere proves 19 SEC-OUT-3 (origin and author of an agent message in
// every representation), TestAgentMessage_RateLimitClass that agent messages count against
// rl.message of the agent principal.
package agentmessages_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"testing"

	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"github.com/OWNER/bothy/services/server/api/specschema"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
	"github.com/OWNER/bothy/services/server/test/harness"
)

func TestMain(m *testing.M) { os.Exit(harness.Main(m)) }

// message is the part of Message (openapi.yaml) the tests check.
type message struct {
	ID     string `json:"id"`
	Origin string `json:"origin"`
	Author struct {
		Kind   string `json:"kind"`
		Handle string `json:"handle"`
	} `json:"author"`
}

// event is the part of an event envelope (listEvents, gateway dispatch) the tests check.
type event struct {
	Seq     int64           `json:"seq"`
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

// payloadMessage decodes the message of a message.created payload (MessagePayload).
func payloadMessage(t *testing.T, what string, payload []byte) message {
	t.Helper()
	var p struct {
		Message message `json:"message"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		t.Fatalf("%s: decode payload %s: %v", what, payload, err)
	}
	return p.Message
}

// agentOrigin fails unless m is message id with origin agent and author {kind agent, handle
// lotse}, the representation SEC-OUT-3 requires everywhere.
func agentOrigin(t *testing.T, what, id string, m message) {
	t.Helper()
	if m.ID != id || m.Origin != "agent" || m.Author.Kind != "agent" || m.Author.Handle != "lotse" {
		t.Errorf("%s: id %q origin %q author {kind %q, handle %q}; want id %s origin agent author {kind agent, handle lotse}",
			what, m.ID, m.Origin, m.Author.Kind, m.Author.Handle, id)
	}
}

// noNull fails when the JSON document b holds a null at any depth (ADR 0053: absent instead of
// null). It walks the decoded value, so a string containing the text null does not count.
func noNull(t *testing.T, what string, b []byte) {
	t.Helper()
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatalf("%s: decode %s: %v", what, b, err)
	}
	if p := nullAt(v, ""); p != "" {
		t.Errorf("%s: JSON null at %s in %s", what, p, b)
	}
}

// nullAt returns the JSON pointer of the first null in v ("/" for v itself), "" when there is none.
func nullAt(v any, ptr string) string {
	switch x := v.(type) {
	case nil:
		if ptr == "" {
			return "/"
		}
		return ptr
	case map[string]any:
		for k, e := range x {
			if p := nullAt(e, ptr+"/"+k); p != "" {
				return p
			}
		}
	case []any:
		for i, e := range x {
			if p := nullAt(e, ptr+"/"+strconv.Itoa(i)); p != "" {
				return p
			}
		}
	}
	return ""
}

// validateEvent checks the envelope ev as the d of a dispatch frame against the contract
// (specschema: event.<type> of asyncapi.yaml, for message.created the MessageCreatedEvent of
// openapi.yaml), which is the schema check the gateway client applies to every frame it receives.
func validateEvent(t *testing.T, what string, ev []byte) {
	t.Helper()
	c, err := specschema.Default()
	if err != nil {
		t.Fatal(err)
	}
	frame := []byte(`{"op":"dispatch","d":` + string(ev) + `}`)
	if err := c.ValidateServerFrame(frame); err != nil {
		t.Errorf("%s: event does not validate against MessageCreatedEvent: %v", what, err)
	}
}

// createdCount returns the sum of bothy.message.created with bothy.message.origin origin from
// the manual metric reader of ts.
func createdCount(t *testing.T, ts *harness.TelemetrySinks, origin string) int64 {
	t.Helper()
	var rm metricdata.ResourceMetrics
	if err := ts.Metrics.Collect(context.Background(), &rm); err != nil {
		t.Fatal(err)
	}
	var n int64
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name != "bothy.message.created" {
				continue
			}
			sum, ok := m.Data.(metricdata.Sum[int64])
			if !ok {
				t.Fatalf("bothy.message.created is %T, want an int64 sum", m.Data)
			}
			for _, p := range sum.DataPoints {
				if v, ok := p.Attributes.Value("bothy.message.origin"); ok && v.AsString() == origin {
					n += p.Value
				}
			}
		}
	}
	return n
}

// post sends createMessage as c into thread with the fixed client_msg_id and Idempotency-Key, so
// a second call with the same arguments is a replay of the same request.
func post(ctx context.Context, c *harness.Client, thread, clientMsgID, key, text string) harness.Response {
	h := http.Header{}
	h.Set("Idempotency-Key", key)
	return c.Do(ctx, harness.Request{Method: http.MethodPost, Path: "/threads/" + thread + "/messages", Header: h,
		Body: map[string]any{"client_msg_id": clientMsgID, "content_md": text}})
}

// findEvent pages listEvents as c from the start of the log until it finds the message.created
// event of message id, and returns that event as raw JSON. Catch-up is how a client that missed
// the gateway learns of the message, so it must carry the same origin.
func findEvent(t *testing.T, ctx context.Context, c *harness.Client, id string) []byte {
	t.Helper()
	after := int64(0)
	for range 1000 {
		var page struct {
			Events     []json.RawMessage `json:"events"`
			NextCursor int64             `json:"next_cursor"`
			HeadSeq    int64             `json:"head_seq"`
		}
		r := harness.MustDo(t, c, harness.Request{Method: http.MethodGet, Path: fmt.Sprintf("/events?after=%d", after)}, http.StatusOK)
		noNull(t, "listEvents", r.Body)
		r.JSON(t, &page)
		for _, raw := range page.Events {
			var ev event
			if err := json.Unmarshal(raw, &ev); err != nil {
				t.Fatalf("listEvents: decode %s: %v", raw, err)
			}
			if ev.Type == "message.created" && payloadMessage(t, "listEvents", ev.Payload).ID == id {
				return raw
			}
		}
		if page.NextCursor >= page.HeadSeq || page.NextCursor <= after {
			break
		}
		after = page.NextCursor
	}
	t.Fatalf("listEvents: no message.created event for message %s", id)
	return nil
}

// TestAgentMessage_OriginEverywhere proves 19 SEC-OUT-3 for one answer of Lotse in
// Support/Allgemein (write through G-5 and as operator of Support): origin agent and author
// {kind agent, handle lotse} in the 201, in the replay 200, in the message.created row of
// event_log, in listEvents read by Alex, in the gateway dispatch to Alex's connection, in
// getMessage and listThreadMessages; no JSON null anywhere; the event validates against
// MessageCreatedEvent; bothy.message.created counts it once with bothy.message.origin agent.
// It sets Options.Telemetry, so it never runs in parallel (otel.Setup installs globals).
func TestAgentMessage_OriginEverywhere(t *testing.T) {
	ts := harness.NewTelemetrySinks()
	s := harness.Start(t, harness.Options{EgressDenyAll: true, Config: harness.RaisedRateLimits(), Telemetry: ts})
	w := harness.BuildMatrixWorld(t, s)
	ctx := t.Context()
	alex, lotse := w.People["Alex"], w.Agents["Lotse"]
	thread := w.Threads["Support/Allgemein"]

	// The connection is open before the message exists, so the frame below is a live dispatch,
	// not a catch-up of the identify.
	conn, err := alex.Gateway(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	before := createdCount(t, ts, "agent")

	clientMsgID, key := ids.New().String(), ids.New().String()
	first := post(ctx, lotse, thread, clientMsgID, key, "Report: the invoice export runs again.")
	if first.Status != http.StatusCreated {
		t.Fatalf("createMessage as Lotse = %d %s, want 201", first.Status, first.Body)
	}
	noNull(t, "201", first.Body)
	var created message
	first.JSON(t, &created)
	id := created.ID
	agentOrigin(t, "201", id, created)

	// The replay sends the same body with the same keys: the original with 200 and
	// Idempotent-Replay, not a second message.
	replay := post(ctx, lotse, thread, clientMsgID, key, "Report: the invoice export runs again.")
	if replay.Status != http.StatusOK || replay.Header.Get("Idempotent-Replay") != "true" {
		t.Fatalf("replay = %d (Idempotent-Replay %q) %s, want 200 with Idempotent-Replay true",
			replay.Status, replay.Header.Get("Idempotent-Replay"), replay.Body)
	}
	noNull(t, "replay", replay.Body)
	var replayed message
	replay.JSON(t, &replayed)
	agentOrigin(t, "replay", id, replayed)

	// The gateway dispatch to Alex: skip every other frame until the message.created of this
	// message arrives.
	// testkit.Patience bounds the wait: a missing frame fails the test instead of hanging it.
	dctx, cancel := context.WithTimeout(ctx, testkit.Patience)
	defer cancel()
	var dispatched []byte
	for dispatched == nil {
		f, err := conn.Next(dctx)
		if err != nil {
			t.Fatalf("gateway: no message.created for %s: %v", id, err)
		}
		var ev event
		if f.Op != "dispatch" || json.Unmarshal(f.D, &ev) != nil || ev.Type != "message.created" {
			continue
		}
		if payloadMessage(t, "gateway", ev.Payload).ID == id {
			dispatched = f.D
		}
	}
	noNull(t, "gateway", dispatched)
	var gw event
	if err := json.Unmarshal(dispatched, &gw); err != nil {
		t.Fatal(err)
	}
	agentOrigin(t, "gateway", id, payloadMessage(t, "gateway", gw.Payload))
	validateEvent(t, "gateway", dispatched)

	// The stored row is the source of every later catch-up; it must carry the origin itself.
	var stored []byte
	var actorKind *string
	if err := s.AdminDB().QueryRow(ctx, `SELECT payload, actor_kind FROM event_log
		WHERE type = 'message.created' AND resource_id = $1`, id).Scan(&stored, &actorKind); err != nil {
		t.Fatalf("event_log: message.created row of %s: %v", id, err)
	}
	noNull(t, "event_log", stored)
	agentOrigin(t, "event_log", id, payloadMessage(t, "event_log", stored))
	if actorKind == nil || *actorKind != "agent" {
		t.Errorf("event_log: actor_kind %v, want agent", actorKind)
	}

	listed := findEvent(t, ctx, alex, id)
	var le event
	if err := json.Unmarshal(listed, &le); err != nil {
		t.Fatal(err)
	}
	agentOrigin(t, "listEvents", id, payloadMessage(t, "listEvents", le.Payload))
	validateEvent(t, "listEvents", listed)
	// The stored payload in the envelope of the listed event: the row itself validates against
	// MessagePayload, the payload schema of MessageCreatedEvent.
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(listed, &envelope); err != nil {
		t.Fatal(err)
	}
	envelope["payload"] = stored
	withStored, err := json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	validateEvent(t, "event_log", withStored)

	got := harness.MustDo(t, alex, harness.Request{Method: http.MethodGet, Path: "/messages/" + id}, http.StatusOK)
	noNull(t, "getMessage", got.Body)
	var fetched message
	got.JSON(t, &fetched)
	agentOrigin(t, "getMessage", id, fetched)

	list := harness.MustDo(t, alex, harness.Request{Method: http.MethodGet, Path: "/threads/" + thread + "/messages"}, http.StatusOK)
	noNull(t, "listThreadMessages", list.Body)
	var page struct {
		Items []json.RawMessage `json:"items"`
	}
	list.JSON(t, &page)
	found := false
	for _, raw := range page.Items {
		var m message
		if err := json.Unmarshal(raw, &m); err != nil {
			t.Fatal(err)
		}
		if m.ID == id {
			found = true
			agentOrigin(t, "listThreadMessages", id, m)
		}
	}
	if !found {
		t.Errorf("listThreadMessages: message %s not among %d items", id, len(page.Items))
	}

	// The counter is added after commit; wait until it grew so a late add is not a false failure.
	// The replay must not count, so the difference is exactly one.
	testkit.Until(t, "bothy.message.created{bothy.message.origin=agent} grew", func() bool {
		return createdCount(t, ts, "agent")-before >= 1
	})
	if d := createdCount(t, ts, "agent") - before; d != 1 {
		t.Errorf("bothy.message.created{bothy.message.origin=agent} grew by %d, want 1", d)
	}
}

// The rate limit of the class rl.message in TestAgentMessage_RateLimitClass: a burst of two and
// one token per minute, so no token comes back while the test runs.
const (
	messageBurst     = 2
	messagePerMinute = 1
)

// TestAgentMessage_RateLimitClass proves that agent messages count against rl.message of the
// agent principal (05 4.7, 09 14 point 4): Lotse's message beyond the burst gives 429
// rate_limited with Retry-After, and a message of Alex at the same moment succeeds because Alex
// has an own bucket.
//
// The matrix world cannot be built here: its owner writes far more messages than the low burst
// allows, and the owner's bucket would refuse the build. The test builds the part of the world
// it needs through the same public API instead: space Support with channel Allgemein, Alex with
// trigger on Support (G-1) and Lotse with write on Support (G-5). Neither the owner nor anyone
// else writes a message before Lotse, so both buckets start full.
func TestAgentMessage_RateLimitClass(t *testing.T) {
	cfg := harness.RaisedRateLimits()
	cfg["ratelimit.rl.message.burst"] = strconv.Itoa(messageBurst)
	cfg["ratelimit.rl.message.per_minute"] = strconv.Itoa(messagePerMinute)
	s := harness.Start(t, harness.Options{EgressDenyAll: true, Config: cfg})
	ctx := t.Context()
	o := s.Owner

	var created struct {
		ID string `json:"id"`
	}
	harness.MustDo(t, o, harness.Request{Method: http.MethodPost, Path: "/spaces", Body: map[string]any{"name": "Support"}}, http.StatusCreated).JSON(t, &created)
	space := created.ID
	var ch struct {
		MainThreadID string `json:"main_thread_id"`
	}
	harness.MustDo(t, o, harness.Request{Method: http.MethodPost, Path: "/spaces/" + space + "/channels",
		Body: map[string]any{"kind": "text", "name": "Allgemein"}}, http.StatusCreated).JSON(t, &ch)
	thread := ch.MainThreadID
	if thread == "" {
		t.Fatal("createChannel: no main_thread_id")
	}
	alex := s.NewUser(t, harness.UserSpec{Handle: "alex", DisplayName: "Alex", Email: "alex@example.com"})
	lotse := s.NewAgent(t, harness.AgentSpec{Handle: "lotse", DisplayName: "Lotse", BillingMode: "organization"})
	for _, g := range []struct{ subjectKind, subject, level string }{
		{"user", alex.PrincipalID, "trigger"},
		{"agent", lotse.PrincipalID, "write"},
	} {
		harness.MustDo(t, o, harness.Request{Method: http.MethodPost, Path: "/grants", Body: map[string]any{
			"subject_kind": g.subjectKind, "subject_id": g.subject, "target_kind": "space", "target_id": space,
			"level": g.level, "reason": "rate limit test",
		}}, http.StatusCreated)
	}

	// Exactly the burst succeeds; the next message is refused by the bucket of Lotse.
	for i := range messageBurst {
		r := post(ctx, lotse, thread, ids.New().String(), ids.New().String(), fmt.Sprintf("Report %d of Lotse.", i+1))
		if r.Status != http.StatusCreated {
			t.Fatalf("message %d of Lotse = %d %s, want 201 within the burst of %d", i+1, r.Status, r.Body, messageBurst)
		}
	}
	limited := post(ctx, lotse, thread, ids.New().String(), ids.New().String(), "Report beyond the burst.")
	var p struct {
		Code string `json:"code"`
	}
	_ = json.Unmarshal(limited.Body, &p)
	retry, err := strconv.Atoi(limited.Header.Get("Retry-After"))
	if limited.Status != http.StatusTooManyRequests || p.Code != "rate_limited" || err != nil || retry < 1 {
		t.Fatalf("message %d of Lotse = %d %s, Retry-After %q; want 429 rate_limited with Retry-After of at least one second",
			messageBurst+1, limited.Status, limited.Body, limited.Header.Get("Retry-After"))
	}

	// Alex at once: a full own bucket, so the message is created although Lotse's is empty.
	r := post(ctx, alex, thread, ids.New().String(), ids.New().String(), "Thanks, Lotse.")
	if r.Status != http.StatusCreated {
		t.Fatalf("message of Alex right after Lotse's 429 = %d %s, want 201 (own bucket)", r.Status, r.Body)
	}
	if bytes.Contains(r.Body, []byte(`"origin":"agent"`)) {
		t.Errorf("message of Alex has origin agent: %s", r.Body)
	}
}
