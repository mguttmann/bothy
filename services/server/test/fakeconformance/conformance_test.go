// SPDX-License-Identifier: Apache-2.0

// Package fakeconformance_test holds the server side of the behaviour conformance between the
// real server and the fake server of the client tests (packages/sync_engine/lib/src/testing):
// every scenario file of packages/sync_engine/test/fixtures/conformance names the requests of
// one rule and the answers and events the real server gives to them. This test sends the
// requests to the server of this tree (test/harness: the real wire graph on a disposable
// PostgreSQL) and holds the recorded part of each file; the Dart test
// packages/sync_engine/test/fake_server/conformance_test.dart sends the same requests to the
// fake and compares with the same file. So a rule the fake copies by hand fails a test when
// either side changes (AUD-pkg-sync-engine-01, README.md).
package fakeconformance_test

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/sync"
	"github.com/OWNER/bothy/services/server/test/harness"
)

func TestMain(m *testing.M) { os.Exit(harness.Main(m)) }

// scenarioDir holds the scenario files, next to the Dart test that reads them too.
const scenarioDir = "../../../../packages/sync_engine/test/fixtures/conformance"

// updateConformance rewrites the recorded part of every scenario file from the real server.
var updateConformance = flag.Bool("update-conformance", false, "rewrite the recorded answers of the conformance scenarios")

// scenario is one file: the requests of a rule and what the real server answers.
type scenario struct {
	// Rule names the rule; it is the file name without ".json".
	Rule string `json:"rule"`
	// Summary says what the rule is and where the specification states it.
	Summary string `json:"summary"`
	// Events are the event types of the rule: an observed step records every event of these
	// types it appended, in order.
	Events []string `json:"events"`
	// EventFields are the JSON Pointers into the envelope recorded for each such event.
	EventFields []string `json:"event_fields"`
	// Steps are the requests, in order.
	Steps []step `json:"steps"`
}

// step is one request. A step without Observe prepares the world and must answer 2xx.
type step struct {
	// Name says what the step does.
	Name string `json:"name"`
	// As names the person who sends it: alex or sam.
	As string `json:"as"`
	// Method and Path name the operation; Path is relative to /api/v1.
	Method string `json:"method"`
	Path   string `json:"path"`
	// Body is the JSON body, when the request has one.
	Body json.RawMessage `json:"body,omitempty"`
	// Observe makes the step an observed one and names what is recorded of its answer.
	Observe *observe `json:"observe,omitempty"`
	// Expect is the recorded answer of the real server.
	Expect *observation `json:"expect,omitempty"`
}

// observe names the JSON Pointers into the response body that an observed step records.
type observe struct {
	Body []string `json:"body"`
}

// observation is what an observed step showed: the status, the problem of a refusal, the
// picked members of the body and the events of the rule the step appended.
type observation struct {
	Status  int              `json:"status"`
	Problem *problem         `json:"problem,omitempty"`
	Body    map[string]any   `json:"body,omitempty"`
	Events  []map[string]any `json:"events"`
}

// problem is the part of an RFC 9457 answer that is a rule: its code and the field errors.
type problem struct {
	Code   string       `json:"code"`
	Errors []fieldError `json:"errors,omitempty"`
}

// fieldError is one entry of errors[].
type fieldError struct {
	Pointer string `json:"pointer"`
	Code    string `json:"code"`
}

// TestFakeConformance runs every scenario against the real server and holds its recorded part.
// Run with -update-conformance after a deliberate change of the server or of a scenario, then
// run the Dart test of the fake.
func TestFakeConformance(t *testing.T) {
	t.Parallel()
	files, err := filepath.Glob(filepath.Join(filepath.FromSlash(scenarioDir), "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatalf("no scenario in %s", scenarioDir)
	}
	for _, file := range files {
		t.Run(strings.TrimSuffix(filepath.Base(file), ".json"), func(t *testing.T) {
			t.Parallel()
			runScenario(t, file)
		})
	}
}

// runScenario sends the steps of the scenario in file to a fresh server and compares the file
// with what the server showed.
func runScenario(t *testing.T, file string) {
	t.Helper()
	raw, err := os.ReadFile(file) // #nosec G304 -- a scenario file of the repository
	if err != nil {
		t.Fatal(err)
	}
	var sc scenario
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&sc); err != nil {
		t.Fatalf("%s: %v", file, err)
	}
	if want := strings.TrimSuffix(filepath.Base(file), ".json"); sc.Rule != want {
		t.Fatalf("%s: rule %q, want the file name %q", file, sc.Rule, want)
	}
	w := newWorld(t)
	for i := range sc.Steps {
		st := &sc.Steps[i]
		c, ok := w.people[st.As]
		if !ok {
			t.Fatalf("step %q: unknown person %q", st.Name, st.As)
		}
		before := w.head(t)
		req := harness.Request{Method: st.Method, Path: w.fill(st.Path)}
		if len(st.Body) > 0 {
			req.Body = []byte(w.fill(string(st.Body)))
		}
		resp := c.Do(t.Context(), req)
		if st.Observe == nil {
			if resp.Status < 200 || resp.Status > 299 {
				t.Fatalf("step %q: %d %s", st.Name, resp.Status, resp.Body)
			}
			st.Expect = nil
			continue
		}
		st.Expect = w.observed(t, sc, *st.Observe, resp, before)
	}
	// The written form of a scenario file: indented, text as it is (no HTML escapes), one
	// newline at the end.
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(sc); err != nil {
		t.Fatal(err)
	}
	got := buf.Bytes()
	if *updateConformance {
		if err := os.WriteFile(file, got, 0o600); err != nil {
			t.Fatal(err)
		}
		return
	}
	if !bytes.Equal(got, raw) {
		t.Errorf("the real server no longer answers as %s records (or the file is not in its written form); "+
			"update it with -update-conformance and run the conformance test of the fake\ngot:\n%s", file, got)
	}
}

// world is the world of a scenario on one server: Alex and Sam, both with write on the text
// channel of one space, and the main thread of that channel. names maps the placeholders of the
// scenario files to its IDs.
type world struct {
	srv    *harness.Server
	people map[string]*harness.Client
	names  map[string]string
}

// newWorld starts a server and builds the world through the public API as the owner.
func newWorld(t *testing.T) *world {
	t.Helper()
	s := harness.Start(t, harness.Options{EgressDenyAll: true, Config: harness.RaisedRateLimits()})
	o := s.Owner
	var space struct {
		ID string `json:"id"`
	}
	harness.MustDo(t, o, harness.Request{Method: http.MethodPost, Path: "/spaces", Body: map[string]any{"name": "Team"}}, http.StatusCreated).JSON(t, &space)
	var channel struct {
		ID           string `json:"id"`
		MainThreadID string `json:"main_thread_id"`
	}
	harness.MustDo(t, o, harness.Request{Method: http.MethodPost, Path: "/spaces/" + space.ID + "/channels",
		Body: map[string]any{"kind": "text", "name": "general"}}, http.StatusCreated).JSON(t, &channel)
	w := &world{srv: s, people: map[string]*harness.Client{}, names: map[string]string{
		"space": space.ID, "channel": channel.ID, "thread": channel.MainThreadID,
	}}
	for _, p := range []harness.UserSpec{
		{Handle: "alex", DisplayName: "Alex", Email: "alex@example.com"},
		{Handle: "sam", DisplayName: "Sam", Email: "sam@example.com"},
	} {
		c := s.NewUser(t, p)
		w.people[p.Handle], w.names[p.Handle] = c, c.PrincipalID
		harness.MustDo(t, o, harness.Request{Method: http.MethodPost, Path: "/grants", Body: map[string]any{
			"subject_kind": "user", "subject_id": c.PrincipalID, "target_kind": "channel", "target_id": channel.ID,
			"level": "write", "reason": "conformance world",
		}}, http.StatusCreated)
	}
	return w
}

// placeholder matches {name} and {uuid:name} in a path or a body.
var placeholder = regexp.MustCompile(`\{(uuid:)?([a-z0-9_]+)\}`)

// fill replaces the placeholders of text: {name} by the ID of the world, {uuid:name} by a UUID
// that is new in this run and the same for every use of the name.
func (w *world) fill(text string) string {
	return placeholder.ReplaceAllStringFunc(text, func(m string) string {
		key := strings.Trim(m, "{}")
		if id, ok := w.names[key]; ok {
			return id
		}
		if strings.HasPrefix(key, "uuid:") {
			w.names[key] = ids.New().String()
			return w.names[key]
		}
		return m
	})
}

// named replaces every string of v that is an ID of the world by its placeholder, so a recorded
// value is the same in every run and on both servers.
func (w *world) named(v any) any {
	switch x := v.(type) {
	case string:
		for name, id := range w.names {
			if x == id {
				return "{" + name + "}"
			}
		}
		return x
	case []any:
		for i := range x {
			x[i] = w.named(x[i])
		}
	case map[string]any:
		for k := range x {
			x[k] = w.named(x[k])
		}
	}
	return v
}

// head returns the seq of the newest event.
func (w *world) head(t *testing.T) int64 {
	t.Helper()
	var seq int64
	if err := w.srv.AdminDB().QueryRow(t.Context(), `SELECT COALESCE(max(seq), 0) FROM event_log`).Scan(&seq); err != nil {
		t.Fatal(err)
	}
	return seq
}

// observed returns what resp and the events after the seq before show of the rule of sc.
func (w *world) observed(t *testing.T, sc scenario, o observe, resp harness.Response, before int64) *observation {
	t.Helper()
	out := &observation{Status: resp.Status, Events: []map[string]any{}}
	var body any
	if len(resp.Body) > 0 {
		if err := json.Unmarshal(resp.Body, &body); err != nil {
			t.Fatalf("a body that is not JSON: %d %s", resp.Status, resp.Body)
		}
	}
	if resp.Status >= 400 {
		out.Problem = problemOf(t, resp.Body)
	} else if len(o.Body) > 0 {
		out.Body = w.picked(body, o.Body)
	}
	for _, env := range w.envelopes(t, before, sc.Events) {
		picked := w.picked(env, sc.EventFields)
		picked["type"] = env.(map[string]any)["type"]
		out.Events = append(out.Events, picked)
	}
	return out
}

// problemOf returns the code and the field errors of a problem body, the errors in the order of
// their pointers.
func problemOf(t *testing.T, body []byte) *problem {
	t.Helper()
	var p problem
	if err := json.Unmarshal(body, &p); err != nil || p.Code == "" {
		t.Fatalf("a refusal without a problem code: %s", body)
	}
	slices.SortFunc(p.Errors, func(a, b fieldError) int {
		return strings.Compare(a.Pointer+" "+a.Code, b.Pointer+" "+b.Code)
	})
	return &p
}

// picked returns the members of doc at the JSON Pointers, by pointer, with the IDs of the world
// replaced by their placeholders; a pointer that leads nowhere is left out.
func (w *world) picked(doc any, pointers []string) map[string]any {
	out := map[string]any{}
	for _, p := range pointers {
		if v, ok := at(doc, p); ok {
			out[p] = w.named(v)
		}
	}
	return out
}

// at resolves the JSON Pointer (RFC 6901) p in doc.
func at(doc any, p string) (any, bool) {
	if p == "" {
		return doc, true
	}
	cur := doc
	for _, token := range strings.Split(strings.TrimPrefix(p, "/"), "/") {
		token = strings.ReplaceAll(strings.ReplaceAll(token, "~1", "/"), "~0", "~")
		switch x := cur.(type) {
		case map[string]any:
			v, ok := x[token]
			if !ok {
				return nil, false
			}
			cur = v
		case []any:
			i, err := strconv.Atoi(token)
			if err != nil || i < 0 || i >= len(x) {
				return nil, false
			}
			cur = x[i]
		default:
			return nil, false
		}
	}
	return cur, true
}

// envelopes returns the events of the types after the seq before, each as the envelope the
// gateway delivers (sync.Event.Envelope), decoded.
func (w *world) envelopes(t *testing.T, before int64, types []string) []any {
	t.Helper()
	rows, err := w.srv.AdminDB().Query(t.Context(), `
		SELECT seq, id::text, type, schema_version, occurred_at, COALESCE(actor_kind, ''), COALESCE(actor_principal_id::text, ''),
		       COALESCE(space_id::text, ''), COALESCE(channel_id::text, ''), COALESCE(thread_id::text, ''),
		       resource_kind, COALESCE(resource_id::text, ''), COALESCE(resource_version, 0),
		       COALESCE(audience_principal_id::text, ''), receive_action, payload, truncated, permission_version
		FROM event_log WHERE seq > $1 AND type = ANY($2) ORDER BY seq`, before, types)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []any
	for rows.Next() {
		var (
			ev                                                                 sync.Event
			id, actor, space, channel, thread, resource, audience, payloadText string
		)
		if err := rows.Scan(&ev.Seq, &id, &ev.Type, &ev.SchemaVersion, &ev.OccurredAt, &ev.ActorKind, &actor, &space, &channel, &thread,
			&ev.ResourceKind, &resource, &ev.ResourceVersion, &audience, &ev.ReceiveAction, &payloadText, &ev.Truncated, &ev.PermissionVersion); err != nil {
			t.Fatal(err)
		}
		ev.ID, ev.ActorPrincipalID, ev.SpaceID, ev.ChannelID, ev.ThreadID = parse(t, id), parse(t, actor), parse(t, space), parse(t, channel), parse(t, thread)
		ev.ResourceID, ev.AudiencePrincipalID, ev.Payload = parse(t, resource), parse(t, audience), json.RawMessage(payloadText)
		encoded, err := json.Marshal(ev.Envelope(sync.DeliveryFields{}))
		if err != nil {
			t.Fatal(err)
		}
		var env any
		if err := json.Unmarshal(encoded, &env); err != nil {
			t.Fatal(err)
		}
		out = append(out, env)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

// parse returns the ID of text, the zero ID for an empty text.
func parse(t *testing.T, text string) ids.ID {
	t.Helper()
	if text == "" {
		return ids.ID{}
	}
	id, err := ids.Parse(text)
	if err != nil {
		t.Fatal(fmt.Errorf("event_log: %w", err))
	}
	return id
}
